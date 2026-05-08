// localcluster_state persists the manager overlay's last-seen view of
// local clusters to $XDG_STATE_HOME/lfk/local-clusters.yaml. The cache
// drives the cluster picker's status icon (filled vs hollow) so stopped
// clusters don't disappear from the picker between manager refreshes.
// Mirrors cluster_colors.go and discovery_cache.go: schema-versioned,
// atomic write, graceful degradation on missing/corrupt/future-schema
// files.
package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/logger"
)

const localClusterStateSchemaVersion = 1

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
	stateDir := os.Getenv("XDG_STATE_HOME")
	if stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		stateDir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateDir, "lfk", "local-clusters.yaml")
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

// saveLocalClusterState writes the cache atomically (tmp + rename).
// Each error path is wrapped with %w + a short context so callers
// surfacing the error see "mkdir state dir: permission denied" rather
// than a bare "permission denied" with no provenance. Failures are
// also logged at the storage layer (mirroring loadLocalClusterState's
// pattern) so a silent disk problem still leaves a trace even if the
// caller discards the error.
func saveLocalClusterState(entries []localClusterCacheEntry) error {
	path := localClusterStateFilePath()
	if path == "" {
		err := errors.New("no state path: HOME and XDG_STATE_HOME both unset")
		logger.Warn("local cluster state save failed", "error", err)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		wrapped := fmt.Errorf("mkdir local-cluster state dir: %w", err)
		logger.Warn("local cluster state save failed", "path", path, "error", wrapped)
		return wrapped
	}
	body, err := yaml.Marshal(localClusterStateFile{
		SchemaVersion: localClusterStateSchemaVersion,
		Clusters:      entries,
	})
	if err != nil {
		wrapped := fmt.Errorf("marshal local-cluster state: %w", err)
		logger.Warn("local cluster state save failed", "path", path, "error", wrapped)
		return wrapped
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		wrapped := fmt.Errorf("write local-cluster state tmp: %w", err)
		logger.Warn("local cluster state save failed", "path", path, "error", wrapped)
		return wrapped
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		wrapped := fmt.Errorf("rename local-cluster state into place: %w", err)
		logger.Warn("local cluster state save failed", "path", path, "error", wrapped)
		return wrapped
	}
	return nil
}
