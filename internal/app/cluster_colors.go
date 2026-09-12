package app

import (
	"errors"
	"fmt"
	"os"

	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/ui"
)

const clusterColorsFileName = "cluster-colors.yaml"

// clusterColorsSchemaVersion bumps whenever the on-disk shape changes.
// loadClusterColors rejects unknown versions so older binaries don't trip on
// a forward-incompat write from a newer one — the worst case is the user
// loses their colour assignments until they re-set them.
const clusterColorsSchemaVersion = 1

// clusterColorsState is the on-disk shape: schema-versioned map of context
// name → colour name. The colour name must be one of ui.ClusterColorNames.
type clusterColorsState struct {
	SchemaVersion int               `json:"schema_version"`
	Contexts      map[string]string `json:"contexts"`
}

// clusterColorsFilePath returns the path to the cluster-colors state file.
func clusterColorsFilePath() string {
	return stateFilePath(clusterColorsFileName)
}

// loadClusterColors reads the cluster-colours map from disk. Returns an
// empty (non-nil) map on any failure — missing file, corrupt YAML, schema
// mismatch — so callers can treat it as "no colours assigned yet". Unknown
// colour names are dropped silently; a typo on one entry must not poison
// the rest of the file.
func loadClusterColors() map[string]string {
	out := make(map[string]string)
	path := clusterColorsFilePath()
	if path == "" {
		return out
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			logger.Warn("Cluster colors read failed", "path", path, "error", err)
		}
		return out
	}
	var s clusterColorsState
	if err := yaml.Unmarshal(data, &s); err != nil {
		logger.Warn("Cluster colors file is corrupt; ignoring", "path", path, "error", err)
		return out
	}
	if s.SchemaVersion != clusterColorsSchemaVersion {
		logger.Info("Cluster colors schema version mismatch; ignoring",
			"path", path, "got", s.SchemaVersion, "want", clusterColorsSchemaVersion)
		return out
	}
	for ctx, color := range s.Contexts {
		if !ui.IsValidClusterColor(color) {
			logger.Warn("Cluster colors: dropping unknown color", "context", ctx, "color", color)
			continue
		}
		out[ctx] = color
	}
	return out
}

// saveClusterColors writes the cluster-colours map to disk. Rejects unknown
// colour names at the boundary so the on-disk file is always valid.
func saveClusterColors(colors map[string]string) error {
	for ctx, color := range colors {
		if !ui.IsValidClusterColor(color) {
			return fmt.Errorf("cluster colors: unknown color %q for context %q", color, ctx)
		}
	}
	if clusterColorsFilePath() == "" {
		return errors.New("cluster colors: cannot resolve state file path")
	}
	return saveStateFile(clusterColorsFileName, clusterColorsState{
		SchemaVersion: clusterColorsSchemaVersion,
		Contexts:      colors,
	})
}
