// Package app — security_findings_cache.go
// Persists scanned security findings per (host, namespace) so a fresh session
// paints SEC badges instantly from disk, then revalidates in the background
// (stale-while-revalidate). Distinct from security_availability_cache.go,
// which only records WHICH sources exist; this stores the findings themselves.
// Same per-host cache dir as the discovery/availability caches, so kubectl/k9s
// cache invalidation also wipes it.
package app

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/security"
)

// securityFindingsCacheMu serializes the findings-cache read-modify-write.
// Persistence runs on background goroutines (off the Bubble Tea Update loop,
// where the YAML marshal of a large findings set would freeze the UI), so two
// scan completions for different namespaces of the same host could otherwise
// interleave their read/merge/rename and drop an entry. Process-wide is fine —
// writes are infrequent (scan completions), and the cross-process race is
// already accepted (see saveSecurityFindingsCacheForHost).
var securityFindingsCacheMu sync.Mutex

// securityFindingsCacheSchemaVersion bumps when the on-disk shape changes.
// A mismatch is treated as a miss so an older binary never decodes a
// forward-incompatible file into garbage findings. Version 1 was the YAML
// format (issue #387); version 2 is JSON with raw per-namespace entries.
const securityFindingsCacheSchemaVersion = 2

// securityFindingsCacheFilename is the per-host basename. Distinct from the
// availability cache so both can live in the same per-host directory. JSON, not
// YAML: the file can be tens of MB and sigs.k8s.io/yaml's YAML->JSON->struct
// decode of the whole thing was the dominant startup memory spike (issue #387).
const securityFindingsCacheFilename = "lfk-security-findings.json"

// securityFindingsCacheLegacyFilename is the pre-#387 YAML basename. New code
// never reads it; saveSecurityFindingsCacheForHost removes it so the large
// legacy file does not linger.
const securityFindingsCacheLegacyFilename = "lfk-security-findings.yaml"

// securityFindingsCacheTTL bounds how stale a disk-seeded badge paint may be.
// Findings older than this are not painted at startup (a miss), so a cluster
// left unopened for a long time doesn't show badges that may be far out of
// date before the background rescan replaces them. The live scan always runs
// regardless; this only gates the instant pre-scan paint.
const securityFindingsCacheTTL = time.Hour

// securityFindingsCacheReleaseThreshold: when the on-disk cache exceeds this,
// seeding it reads and tokenizes the whole file, briefly allocating well beyond
// the retained badge index. Go's background scavenger returns that memory to the
// OS only slowly — on Windows it left the process RSS pinned at the spike for the
// rest of the session (issue #387) — so the seed proactively frees it.
const securityFindingsCacheReleaseThreshold = 4 << 20 // 4 MiB

// securityFindingsCacheFile is the on-disk envelope: one file per cluster API
// host, holding findings keyed by namespace ("" = all-namespaces). Per-namespace
// entries are kept as raw JSON so a load (or a save touching one namespace)
// never decodes the findings of the other namespaces — the reflect-heavy decode
// of every namespace's findings was the dominant allocation in the startup
// memory spike (issue #387).
type securityFindingsCacheFile struct {
	SchemaVersion int                        `json:"schema_version"`
	Namespaces    map[string]json.RawMessage `json:"namespaces"`
}

// securityFindingsEntry is one namespace's payload; each carries its own
// scannedAt so per-namespace TTL is independent.
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

// securityFindingsCacheLegacyPathForHost returns the pre-#387 YAML cache path,
// or "" when it can't be resolved. Only used to clean up the legacy file.
func securityFindingsCacheLegacyPathForHost(host string) string {
	if host == "" {
		return ""
	}
	base := k8s.DiscoveryCacheBaseDir()
	if base == "" {
		return ""
	}
	return filepath.Join(base, "discovery", k8s.CacheHostDir(host), securityFindingsCacheLegacyFilename)
}

// readFindingsCacheFile loads and validates the per-host envelope. Returns nil
// on any failure (missing, corrupt, schema mismatch) so callers fall through to
// a live scan. Per-namespace findings are NOT decoded here — they stay as raw
// JSON until a specific namespace is requested (see loadSecurityFindingsCacheForHost).
func readFindingsCacheFile(host string) *securityFindingsCacheFile {
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
	var f securityFindingsCacheFile
	if err := json.Unmarshal(data, &f); err != nil {
		logger.Warn("Security findings cache is corrupt; ignoring", "host", k8s.CacheHostDir(host), "error", err)
		return nil
	}
	if f.SchemaVersion != securityFindingsCacheSchemaVersion {
		logger.Info("Security findings cache schema mismatch; ignoring",
			"host", k8s.CacheHostDir(host), "got", f.SchemaVersion, "want", securityFindingsCacheSchemaVersion)
		return nil
	}
	return &f
}

// loadSecurityFindingsCacheForHost returns the cached findings for (host,
// namespace) when present and newer than ttl, else nil (a miss). A nil return
// is the signal to fall through to a live scan. Only the requested namespace's
// entry is decoded; a corrupt sibling namespace cannot block a healthy one.
func loadSecurityFindingsCacheForHost(host, namespace string, ttl time.Duration) []security.Finding {
	f := readFindingsCacheFile(host)
	if f == nil {
		return nil
	}
	raw, ok := f.Namespaces[namespace]
	if !ok {
		return nil
	}
	var entry securityFindingsEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		logger.Warn("Security findings cache namespace entry corrupt; ignoring",
			"host", k8s.CacheHostDir(host), "namespace", namespace, "error", err)
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
	entryBytes, err := json.Marshal(securityFindingsEntry{
		ScannedAt: scannedAt.UTC(),
		Findings:  findings,
	})
	if err != nil {
		return err
	}
	// Merge into any existing file so a save for one namespace doesn't wipe the
	// others. Other namespaces are carried as raw JSON, never re-decoded. A
	// corrupt/missing file just starts fresh. The read-modify-write is
	// serialized within this process by securityFindingsCacheMu (saves run on
	// background goroutines, no longer the Update loop). Two lfk instances
	// saving different namespaces of the same host concurrently can still lose
	// one entry (last rename wins) — acceptable for a best-effort, TTL-bounded
	// cache; the lost entry is re-derived by the next scan.
	securityFindingsCacheMu.Lock()
	defer securityFindingsCacheMu.Unlock()
	file := readFindingsCacheFile(host)
	if file == nil {
		file = &securityFindingsCacheFile{SchemaVersion: securityFindingsCacheSchemaVersion}
	}
	if file.Namespaces == nil {
		file.Namespaces = make(map[string]json.RawMessage)
	}
	file.Namespaces[namespace] = json.RawMessage(entryBytes)

	data, err := json.Marshal(file)
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
	if err := writeFileDurable(path, data, 0o600); err != nil {
		return err
	}
	// Best-effort: drop a pre-#387 YAML cache so the large legacy file does not
	// linger on disk. Ignored when absent.
	if legacy := securityFindingsCacheLegacyPathForHost(host); legacy != "" {
		_ = os.Remove(legacy)
	}
	return nil
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

// securityFindingsCacheFileSizeForContext returns the on-disk size of the
// per-host findings cache, or 0 when it can't be resolved or stat'd. Used to
// decide whether a seed parse was large enough to warrant returning the
// transient memory to the OS (issue #387).
func securityFindingsCacheFileSizeForContext(client *k8s.Client, contextName string) int64 {
	if client == nil || contextName == "" {
		return 0
	}
	host := client.HostForContext(contextName)
	if host == "" {
		return 0
	}
	path := securityFindingsCacheFilePathForHost(host)
	if path == "" {
		return 0
	}
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
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
