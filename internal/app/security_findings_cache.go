// Package app — security_findings_cache.go
// Persists scanned security findings per (host, namespace) so a fresh session
// paints SEC badges instantly from disk, then revalidates in the background
// (stale-while-revalidate). Distinct from security_availability_cache.go,
// which only records WHICH sources exist; this stores the findings themselves.
// Same per-host cache dir as the discovery/availability caches, so kubectl/k9s
// cache invalidation also wipes it.
package app

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/security"
)

// securityFindingsCacheSchemaVersion bumps when the on-disk shape changes.
// A mismatch is treated as a miss so an older binary never decodes a
// forward-incompatible file into garbage findings.
const securityFindingsCacheSchemaVersion = 1

// securityFindingsCacheFilename is the per-host basename. Distinct from the
// availability cache so both can live in the same per-host directory.
const securityFindingsCacheFilename = "lfk-security-findings.yaml"

// securityFindingsCacheTTL bounds how stale a disk-seeded badge paint may be.
// Findings older than this are not painted at startup (a miss), so a cluster
// left unopened for a long time doesn't show badges that may be far out of
// date before the background rescan replaces them. The live scan always runs
// regardless; this only gates the instant pre-scan paint.
const securityFindingsCacheTTL = time.Hour

// securityFindingsCacheState is the on-disk shape: one file per cluster API
// host, holding findings keyed by namespace ("" = all-namespaces). Each entry
// carries its own scannedAt so per-namespace TTL is independent.
type securityFindingsCacheState struct {
	SchemaVersion int                              `json:"schema_version"`
	Namespaces    map[string]securityFindingsEntry `json:"namespaces"`
}

type securityFindingsEntry struct {
	ScannedAt time.Time          `json:"scanned_at"`
	Findings  []security.Finding `json:"findings"`
}

// securityFindingsCacheFilePathForHost returns the per-host cache file path,
// or "" when the host or cache base dir can't be resolved (treated as "skip").
func securityFindingsCacheFilePathForHost(host string) string {
	if host == "" {
		return ""
	}
	base := k8s.DiscoveryCacheBaseDir()
	if base == "" {
		return ""
	}
	return filepath.Join(base, "discovery", k8s.CacheHostDir(host), securityFindingsCacheFilename)
}

// readFindingsCacheFile loads and validates the whole per-host state. Returns
// nil on any failure (missing, corrupt, schema mismatch) so callers fall
// through to a live scan.
func readFindingsCacheFile(host string) *securityFindingsCacheState {
	path := securityFindingsCacheFilePathForHost(host)
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			logger.Warn("Security findings cache read failed", "host", k8s.CacheHostDir(host), "error", err)
		}
		return nil
	}
	var s securityFindingsCacheState
	if err := yaml.Unmarshal(data, &s); err != nil {
		logger.Warn("Security findings cache is corrupt; ignoring", "host", k8s.CacheHostDir(host), "error", err)
		return nil
	}
	if s.SchemaVersion != securityFindingsCacheSchemaVersion {
		logger.Info("Security findings cache schema mismatch; ignoring",
			"host", k8s.CacheHostDir(host), "got", s.SchemaVersion, "want", securityFindingsCacheSchemaVersion)
		return nil
	}
	return &s
}

// loadSecurityFindingsCacheForHost returns the cached findings for (host,
// namespace) when present and newer than ttl, else nil (a miss). A nil return
// is the signal to fall through to a live scan.
func loadSecurityFindingsCacheForHost(host, namespace string, ttl time.Duration) []security.Finding {
	s := readFindingsCacheFile(host)
	if s == nil {
		return nil
	}
	entry, ok := s.Namespaces[namespace]
	if !ok {
		return nil
	}
	if ttl > 0 && time.Since(entry.ScannedAt) > ttl {
		return nil
	}
	return entry.Findings
}

// saveSecurityFindingsCacheForHost writes findings for (host, namespace),
// merging into the existing per-host file so other namespaces' entries
// survive. scannedAt stamps the entry's freshness. Atomic (tmp + rename +
// fsync), matching the availability cache's durability. No-op for an empty
// host.
func saveSecurityFindingsCacheForHost(host, namespace string, findings []security.Finding, scannedAt time.Time) error {
	path := securityFindingsCacheFilePathForHost(host)
	if path == "" {
		return nil
	}
	// Normalize nil to an empty slice so a clean scan with zero findings
	// round-trips as `findings: []` (a real cached "nothing found") rather
	// than `findings: null`, which would unmarshal to nil and be
	// indistinguishable from a cache miss on the next load.
	if findings == nil {
		findings = []security.Finding{}
	}
	// Merge into any existing state so a save for one namespace doesn't wipe
	// the others. A corrupt/missing file just starts fresh. NOTE: the
	// read-modify-write is only atomic within one process (saves run on the
	// Bubble Tea Update loop). Two lfk instances saving different namespaces
	// of the same host concurrently can lose one entry (last rename wins) —
	// acceptable for a best-effort, TTL-bounded cache; the lost entry is
	// re-derived by the next scan.
	state := readFindingsCacheFile(host)
	if state == nil {
		state = &securityFindingsCacheState{SchemaVersion: securityFindingsCacheSchemaVersion}
	}
	if state.Namespaces == nil {
		state.Namespaces = make(map[string]securityFindingsEntry)
	}
	state.Namespaces[namespace] = securityFindingsEntry{
		ScannedAt: scannedAt.UTC(),
		Findings:  findings,
	}

	data, err := yaml.Marshal(state)
	if err != nil {
		return err
	}
	// 0700 dir / 0600 file: findings carry CVE IDs, resource and container
	// names, policy text, and labels — more sensitive than the availability
	// cache's booleans, so they are not world-readable even when lfk is the
	// first to create the per-host cache directory on a multi-user host.
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return writeFileDurable(path, data, 0o600)
}

// updateSecurityFindingsCacheForContext resolves the context to its host and
// persists the findings. No-op when the host can't be resolved.
func updateSecurityFindingsCacheForContext(client *k8s.Client, contextName, namespace string, findings []security.Finding) error {
	if client == nil || contextName == "" {
		return nil
	}
	host := client.HostForContext(contextName)
	if host == "" {
		return nil
	}
	return saveSecurityFindingsCacheForHost(host, namespace, findings, time.Now().UTC())
}

// loadSecurityFindingsCacheForContext resolves the context's host and returns
// the cached findings within ttl, or nil on a miss.
func loadSecurityFindingsCacheForContext(client *k8s.Client, contextName, namespace string, ttl time.Duration) []security.Finding {
	if client == nil || contextName == "" {
		return nil
	}
	host := client.HostForContext(contextName)
	if host == "" {
		return nil
	}
	return loadSecurityFindingsCacheForHost(host, namespace, ttl)
}
