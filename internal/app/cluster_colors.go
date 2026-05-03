package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/ui"
)

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
// Uses $XDG_STATE_HOME/lfk/ (defaults to ~/.local/state/lfk/) per XDG spec —
// same convention as bookmarks.yaml and the input histories.
func clusterColorsFilePath() string {
	stateDir := os.Getenv("XDG_STATE_HOME")
	if stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		stateDir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateDir, "lfk", "cluster-colors.yaml")
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

// saveClusterColors writes the cluster-colours map to disk atomically
// (sibling .tmp + rename) so a crash mid-write can't leave a half-written
// file that loadClusterColors would discard. Rejects unknown colour names
// at the boundary so the on-disk file is always valid.
func saveClusterColors(colors map[string]string) error {
	for ctx, color := range colors {
		if !ui.IsValidClusterColor(color) {
			return fmt.Errorf("cluster colors: unknown color %q for context %q", color, ctx)
		}
	}
	path := clusterColorsFilePath()
	if path == "" {
		return errors.New("cluster colors: cannot resolve state file path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	state := clusterColorsState{
		SchemaVersion: clusterColorsSchemaVersion,
		Contexts:      colors,
	}
	data, err := yaml.Marshal(state)
	if err != nil {
		return err
	}
	// Use os.CreateTemp with a unique suffix instead of a fixed ".tmp"
	// sibling so two concurrent saves can't collide on the temp filename
	// — a fixed suffix would race in the (admittedly unusual) case of
	// two lfk instances saving to the same XDG state dir at the same
	// moment, leaving one with a half-written file or a botched rename.
	tmpFile, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp.*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	// Best-effort cleanup if anything below fails: the temp file is
	// orphaned and will not be retried on next save.
	cleanup := func() { _ = os.Remove(tmpPath) }
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		cleanup()
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		cleanup()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return err
	}
	return nil
}
