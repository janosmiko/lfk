// Package app — security_availability_cache.go
// Persists the per-host security source availability probe so a fresh
// session can populate the sidebar Security category before the live
// probe completes — same lifecycle as the discovery cache (one yaml per
// $KUBECACHEDIR/discovery/<host>/), so kubectl/k9s cache invalidation
// also wipes lfk's security cache.
package app

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/logger"
)

// securityAvailabilityCacheSchemaVersion bumps when the on-disk shape
// changes. Older binaries reject unknown versions so a forward-incompat
// write doesn't crash the loader; the worst case is one extra probe on
// next launch.
const securityAvailabilityCacheSchemaVersion = 1

// securityAvailabilityCacheFilename is the basename inside the per-host
// kubectl-cache dir. Distinct from the discovery cache's filename so the
// two formats can coexist in the same directory.
const securityAvailabilityCacheFilename = "lfk-security-availability.yaml"

// SecurityAvailabilityCacheState is the on-disk shape: one file per
// cluster API host. The Availability map mirrors
// Model.securityAvailabilityByName.
type SecurityAvailabilityCacheState struct {
	SchemaVersion int             `json:"schema_version"`
	UpdatedAt     time.Time       `json:"updated_at"`
	Availability  map[string]bool `json:"availability"`
}

// securityAvailabilityCacheFilePathForHost returns the per-host cache file
// path. Returns "" when the host is empty or KUBECACHEDIR can't be
// resolved; callers treat that as "skip caching".
func securityAvailabilityCacheFilePathForHost(host string) string {
	if host == "" {
		return ""
	}
	base := k8s.DiscoveryCacheBaseDir()
	if base == "" {
		return ""
	}
	return filepath.Join(base, "discovery", k8s.CacheHostDir(host), securityAvailabilityCacheFilename)
}

// loadSecurityAvailabilityCacheForHost reads one host's cache. Returns
// nil on any failure (missing file, corrupt YAML, schema mismatch) so
// callers can treat nil as "fall through to the live probe".
func loadSecurityAvailabilityCacheForHost(host string) *SecurityAvailabilityCacheState {
	path := securityAvailabilityCacheFilePathForHost(host)
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			logger.Warn("Security availability cache read failed", "host", k8s.CacheHostDir(host), "error", err)
		}
		return nil
	}
	var s SecurityAvailabilityCacheState
	if err := yaml.Unmarshal(data, &s); err != nil {
		logger.Warn("Security availability cache is corrupt; ignoring", "host", k8s.CacheHostDir(host), "error", err)
		return nil
	}
	if s.SchemaVersion != securityAvailabilityCacheSchemaVersion {
		logger.Info("Security availability cache schema version mismatch; ignoring",
			"host", k8s.CacheHostDir(host), "got", s.SchemaVersion, "want", securityAvailabilityCacheSchemaVersion)
		return nil
	}
	return &s
}

// saveSecurityAvailabilityCacheForHost writes the host's cache atomically
// (sibling .tmp + rename) so a crash mid-write can't leave a half-written
// file that the loader would discard.
func saveSecurityAvailabilityCacheForHost(host string, availability map[string]bool) error {
	path := securityAvailabilityCacheFilePathForHost(host)
	if path == "" {
		return nil
	}
	// 0700 dir to match the findings cache that shares this per-host
	// directory: whichever cache creates it first sets the mode, so both use
	// 0700 to keep the dir non-traversable on multi-user hosts. The
	// availability file itself holds only booleans (which sources exist), so
	// 0644 on the file is fine.
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	// Copy to insulate the on-disk snapshot from later mutations.
	avail := make(map[string]bool, len(availability))
	maps.Copy(avail, availability)
	state := SecurityAvailabilityCacheState{
		SchemaVersion: securityAvailabilityCacheSchemaVersion,
		UpdatedAt:     time.Now().UTC(),
		Availability:  avail,
	}
	data, err := yaml.Marshal(state)
	if err != nil {
		return err
	}
	return writeFileDurable(path, data, 0o644)
}

// loadSecurityAvailabilityCacheForContext is the convenience accessor used
// by refreshSecuritySources: resolve the active context's host, read the
// cache, return the availability map (or nil on miss).
func loadSecurityAvailabilityCacheForContext(client *k8s.Client, contextName string) map[string]bool {
	if client == nil || contextName == "" {
		return nil
	}
	host := client.HostForContext(contextName)
	if host == "" {
		return nil
	}
	snap := loadSecurityAvailabilityCacheForHost(host)
	if snap == nil {
		return nil
	}
	return snap.Availability
}

// updateSecurityAvailabilityCacheForContext is the single mutator used by
// the probe-success path: resolve the context to its host, write the
// host's snapshot. No-op when the host can't be resolved.
func updateSecurityAvailabilityCacheForContext(client *k8s.Client, contextName string, availability map[string]bool) error {
	if client == nil || contextName == "" {
		return nil
	}
	host := client.HostForContext(contextName)
	if host == "" {
		return nil
	}
	return saveSecurityAvailabilityCacheForHost(host, availability)
}
