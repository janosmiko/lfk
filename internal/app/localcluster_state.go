// localcluster_state persists the manager overlay's last-seen view of
// local clusters to the lfk state directory (local-clusters.yaml). The cache
// drives the cluster picker's status icon (filled vs hollow) so stopped
// clusters don't disappear from the picker between manager refreshes.
// Mirrors cluster_colors.go and discovery_cache.go: schema-versioned,
// atomic write, graceful degradation on missing/corrupt/future-schema
// files.
package app

import (
	"errors"
	"os"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/logger"
)

const (
	localClusterStateFileName      = "local-clusters.yaml"
	localClusterStateSchemaVersion = 1
)

// localClusterCacheEntry is one row in local-clusters.yaml.
type localClusterCacheEntry struct {
	Provider    string    `json:"provider"`
	Name        string    `json:"name"`
	ContextName string    `json:"context_name"`
	Status      string    `json:"status"`
	K8sVersion  string    `json:"k8s_version,omitempty"`
	Nodes       int       `json:"nodes,omitempty"`
	Age         string    `json:"age,omitempty"`
	LastSeen    time.Time `json:"last_seen"`
}

type localClusterStateFile struct {
	SchemaVersion int                      `json:"schema_version"`
	Clusters      []localClusterCacheEntry `json:"clusters"`
}

func localClusterStateFilePath() string {
	return stateFilePath(localClusterStateFileName)
}

// loadLocalClusterState reads the cache from disk. Returns an empty
// (non-nil) map on any failure so callers can treat it as "no entries
// yet". Keyed by ContextName for O(1) picker-row lookup.
func loadLocalClusterState() map[string]localClusterCacheEntry {
	out := make(map[string]localClusterCacheEntry)
	path := localClusterStateFilePath()
	if path == "" {
		return out
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			logger.Warn("local cluster state read failed", "path", path, "error", err)
		}
		return out
	}
	var f localClusterStateFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		logger.Warn("local cluster state file is corrupt; ignoring", "path", path, "error", err)
		return out
	}
	if f.SchemaVersion != localClusterStateSchemaVersion {
		logger.Info("local cluster state schema mismatch; ignoring",
			"path", path, "got", f.SchemaVersion, "want", localClusterStateSchemaVersion)
		return out
	}
	for _, e := range f.Clusters {
		if e.ContextName == "" || e.Provider == "" {
			continue
		}
		out[e.ContextName] = e
	}
	return out
}

// saveLocalClusterState writes the cache durably (fsynced tmp + rename).
// Failures are logged at the storage layer so a silent disk problem still
// leaves a trace even if the caller discards the error.
func saveLocalClusterState(entries []localClusterCacheEntry) error {
	err := saveStateFile(localClusterStateFileName, localClusterStateFile{
		SchemaVersion: localClusterStateSchemaVersion,
		Clusters:      entries,
	})
	if err != nil {
		logger.Warn("local cluster state save failed", "error", err)
	}
	return err
}
