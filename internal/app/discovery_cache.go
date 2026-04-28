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
	"github.com/janosmiko/lfk/internal/model"
)

// discoveryCacheSchemaVersion bumps whenever the on-disk shape changes.
// loadDiscoveryCacheForHost rejects unknown versions so older binaries don't
// trip on a forward-incompat write from a newer one — the worst case is one
// extra discovery roundtrip on first launch after upgrade.
//
// v2: per-host file layout (one yaml per ~/.kube/cache/discovery/<host>/)
// replacing the v1 single-file XDG-state design — shares lifecycle with
// kubectl/k9s so `kubectl api-resources --invalidate-cache` wipes lfk's
// cache too. v1 files in the old XDG location are abandoned (the previous
// build was unreleased).
const discoveryCacheSchemaVersion = 2

// discoveryCacheFilename is the basename of lfk's enriched cache inside each
// per-host kubectl-cache dir. Distinct from kubectl's per-group/version JSON
// files so the two formats can coexist in the same directory.
const discoveryCacheFilename = "lfk-enriched.yaml"

// DiscoveryCacheEntry is the serialized form of a single ResourceTypeEntry.
// It mirrors model.ResourceTypeEntry but carries explicit JSON tags so the
// YAML shape stays stable across renames in the domain type.
type DiscoveryCacheEntry struct {
	DisplayName    string              `json:"display_name,omitempty"`
	Kind           string              `json:"kind"`
	APIGroup       string              `json:"api_group"`
	APIVersion     string              `json:"api_version"`
	Resource       string              `json:"resource"`
	Namespaced     bool                `json:"namespaced,omitempty"`
	RequiresCRD    bool                `json:"requires_crd,omitempty"`
	Deprecated     bool                `json:"deprecated,omitempty"`
	DeprecationMsg string              `json:"deprecation_msg,omitempty"`
	Verbs          []string            `json:"verbs,omitempty"`
	PrinterColumns []DiscoveryCacheCol `json:"printer_columns,omitempty"`
	Icon           DiscoveryCacheIcon  `json:"icon,omitzero"`
}

// DiscoveryCacheCol mirrors model.PrinterColumn with explicit JSON tags.
type DiscoveryCacheCol struct {
	Name     string `json:"name"`
	Type     string `json:"type,omitempty"`
	JSONPath string `json:"json_path,omitempty"`
}

// DiscoveryCacheIcon mirrors model.Icon with explicit JSON tags.
type DiscoveryCacheIcon struct {
	Unicode  string `json:"unicode,omitempty"`
	Simple   string `json:"simple,omitempty"`
	Emoji    string `json:"emoji,omitempty"`
	NerdFont string `json:"nerd_font,omitempty"`
}

// DiscoveryCacheHostState is the on-disk shape: one file per cluster API
// host. Multiple kubeconfig contexts pointing at the same host share one
// file because cluster-level discovery output is host-keyed, not
// context-keyed.
type DiscoveryCacheHostState struct {
	SchemaVersion int                   `json:"schema_version"`
	Host          string                `json:"host"`
	UpdatedAt     time.Time             `json:"updated_at"`
	Entries       []DiscoveryCacheEntry `json:"entries"`
}

// discoveryCacheFilePathForHost returns the per-host cache file path:
// $KUBECACHEDIR/discovery/<host>/lfk-enriched.yaml (with $KUBECACHEDIR
// defaulting to ~/.kube/cache). Returns "" for an empty host or an
// unresolvable home dir; callers treat that as "skip caching".
func discoveryCacheFilePathForHost(host string) string {
	if host == "" {
		return ""
	}
	base := k8s.DiscoveryCacheBaseDir()
	if base == "" {
		return ""
	}
	return filepath.Join(base, "discovery", k8s.CacheHostDir(host), discoveryCacheFilename)
}

// loadDiscoveryCacheForHost reads one host's enriched cache. Returns nil on
// any failure — missing file, corrupt YAML, schema mismatch — so callers
// can treat a nil result as "fall through to live discovery".
func loadDiscoveryCacheForHost(host string) *DiscoveryCacheHostState {
	path := discoveryCacheFilePathForHost(host)
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			logger.Warn("Discovery cache read failed", "host", host, "error", err)
		}
		return nil
	}
	var s DiscoveryCacheHostState
	if err := yaml.Unmarshal(data, &s); err != nil {
		logger.Warn("Discovery cache is corrupt; ignoring", "host", host, "error", err)
		return nil
	}
	if s.SchemaVersion != discoveryCacheSchemaVersion {
		logger.Info("Discovery cache schema version mismatch; ignoring",
			"host", host, "got", s.SchemaVersion, "want", discoveryCacheSchemaVersion)
		return nil
	}
	return &s
}

// saveDiscoveryCacheForHost writes one host's enriched cache atomically
// (sibling .tmp + rename) so a crash mid-write can't leave a half-written
// file that loadDiscoveryCacheForHost would discard.
func saveDiscoveryCacheForHost(host string, entries []model.ResourceTypeEntry) error {
	path := discoveryCacheFilePathForHost(host)
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	state := DiscoveryCacheHostState{
		SchemaVersion: discoveryCacheSchemaVersion,
		Host:          host,
		UpdatedAt:     time.Now().UTC(),
		Entries:       discoveryCacheEntriesFromModel(entries),
	}
	data, err := yaml.Marshal(state)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// loadAllDiscoveryCaches enumerates every kubeconfig context known to client,
// resolves each to a host, reads each unique host's cache file once, and
// returns a map from context display name to cached resource entries ready
// to plug into m.discoveredResources. Contexts with unresolvable hosts or
// missing cache files are silently skipped — they fall through to live
// discovery on first interaction.
func loadAllDiscoveryCaches(client *k8s.Client) map[string][]model.ResourceTypeEntry {
	if client == nil {
		return nil
	}
	contexts, err := client.GetContexts()
	if err != nil {
		return nil
	}
	out := make(map[string][]model.ResourceTypeEntry)
	hostCache := make(map[string]*DiscoveryCacheHostState)
	for _, ctx := range contexts {
		host := client.HostForContext(ctx.Name)
		if host == "" {
			continue
		}
		snap, ok := hostCache[host]
		if !ok {
			snap = loadDiscoveryCacheForHost(host)
			hostCache[host] = snap
		}
		if snap == nil {
			continue
		}
		out[ctx.Name] = modelEntriesFromDiscoveryCache(snap.Entries)
	}
	return out
}

// updateDiscoveryCacheForContext is the single mutator used by the discovery
// success path: resolve the context to its host, write the host's enriched
// snapshot. No-op when the host can't be resolved — the live data is still
// authoritative for this session.
func updateDiscoveryCacheForContext(client *k8s.Client, contextName string, entries []model.ResourceTypeEntry) error {
	if client == nil || contextName == "" {
		return nil
	}
	host := client.HostForContext(contextName)
	if host == "" {
		return nil
	}
	return saveDiscoveryCacheForHost(host, entries)
}

// discoveryCacheEntriesFromModel converts the model's domain types to the
// cache's serialization types. The cache deliberately does not store
// pseudo-resources (helm releases, port forwards) — those are prepended at
// runtime by updateAPIResourceDiscovery and may evolve independently of
// disk format.
func discoveryCacheEntriesFromModel(entries []model.ResourceTypeEntry) []DiscoveryCacheEntry {
	out := make([]DiscoveryCacheEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, DiscoveryCacheEntry{
			DisplayName:    e.DisplayName,
			Kind:           e.Kind,
			APIGroup:       e.APIGroup,
			APIVersion:     e.APIVersion,
			Resource:       e.Resource,
			Namespaced:     e.Namespaced,
			RequiresCRD:    e.RequiresCRD,
			Deprecated:     e.Deprecated,
			DeprecationMsg: e.DeprecationMsg,
			Verbs:          append([]string(nil), e.Verbs...),
			PrinterColumns: discoveryCacheColsFromModel(e.PrinterColumns),
			Icon: DiscoveryCacheIcon{
				Unicode:  e.Icon.Unicode,
				Simple:   e.Icon.Simple,
				Emoji:    e.Icon.Emoji,
				NerdFont: e.Icon.NerdFont,
			},
		})
	}
	return out
}

func discoveryCacheColsFromModel(cols []model.PrinterColumn) []DiscoveryCacheCol {
	if len(cols) == 0 {
		return nil
	}
	out := make([]DiscoveryCacheCol, 0, len(cols))
	for _, c := range cols {
		out = append(out, DiscoveryCacheCol{Name: c.Name, Type: c.Type, JSONPath: c.JSONPath})
	}
	return out
}

// modelEntriesFromDiscoveryCache is the inverse: turn a cached snapshot back
// into model types ready to plug into m.discoveredResources.
func modelEntriesFromDiscoveryCache(entries []DiscoveryCacheEntry) []model.ResourceTypeEntry {
	out := make([]model.ResourceTypeEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, model.ResourceTypeEntry{
			DisplayName:    e.DisplayName,
			Kind:           e.Kind,
			APIGroup:       e.APIGroup,
			APIVersion:     e.APIVersion,
			Resource:       e.Resource,
			Namespaced:     e.Namespaced,
			RequiresCRD:    e.RequiresCRD,
			Deprecated:     e.Deprecated,
			DeprecationMsg: e.DeprecationMsg,
			Verbs:          append([]string(nil), e.Verbs...),
			PrinterColumns: modelColsFromDiscoveryCache(e.PrinterColumns),
			Icon: model.Icon{
				Unicode:  e.Icon.Unicode,
				Simple:   e.Icon.Simple,
				Emoji:    e.Icon.Emoji,
				NerdFont: e.Icon.NerdFont,
			},
		})
	}
	return out
}

func modelColsFromDiscoveryCache(cols []DiscoveryCacheCol) []model.PrinterColumn {
	if len(cols) == 0 {
		return nil
	}
	out := make([]model.PrinterColumn, 0, len(cols))
	for _, c := range cols {
		out = append(out, model.PrinterColumn{Name: c.Name, Type: c.Type, JSONPath: c.JSONPath})
	}
	return out
}

// shouldFireDiscoveryFor reports whether a fresh discovery call should be
// kicked off for contextName. Returns false when (a) a discovery is already
// in flight for that context — deduplication — or (b) discovery has already
// completed during this session, in which case re-fetching would just hit
// the cluster API for nothing.
//
// This is the gate that makes stale-while-revalidate work: NewModel prefills
// m.discoveredResources from disk, but the first hover/navigate of a context
// returns true here so a live refresh kicks off behind the cached view.
func (m *Model) shouldFireDiscoveryFor(contextName string) bool {
	if contextName == "" {
		return false
	}
	if m.discoveringContexts[contextName] {
		return false
	}
	if m.discoveryRefreshedContexts[contextName] {
		return false
	}
	return true
}

// markDiscoveryStarted records that a discovery call is now in flight for
// contextName so subsequent shouldFireDiscoveryFor calls deduplicate. Safe
// to call when discoveringContexts is nil — the function is the canonical
// place to lazily allocate the map.
func (m *Model) markDiscoveryStarted(contextName string) {
	if contextName == "" {
		return
	}
	if m.discoveringContexts == nil {
		m.discoveringContexts = make(map[string]bool)
	}
	m.discoveringContexts[contextName] = true
}
