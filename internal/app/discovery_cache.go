package app

import (
	"os"
	"path/filepath"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// discoveryCacheSchemaVersion bumps whenever the on-disk shape changes.
// loadDiscoveryCache rejects unknown versions so older binaries don't trip
// on a forward-incompat write from a newer one — the worst case is one
// extra discovery roundtrip on first launch after upgrade.
const discoveryCacheSchemaVersion = 1

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

// DiscoveryCacheContext is the per-context block — a snapshot of the API
// resources the cluster reported, plus the wall-clock time of capture.
type DiscoveryCacheContext struct {
	UpdatedAt time.Time             `json:"updated_at"`
	Entries   []DiscoveryCacheEntry `json:"entries"`
}

// DiscoveryCacheState is the top-level on-disk document.
type DiscoveryCacheState struct {
	SchemaVersion int                              `json:"schema_version"`
	Contexts      map[string]DiscoveryCacheContext `json:"contexts,omitempty"`
}

// discoveryCacheFilePath returns the path to the discovery cache file.
// Uses the same XDG state directory as session.yaml so cleanup tools that
// already understand lfk's state dir cover it without extra config.
func discoveryCacheFilePath() string {
	stateDir := os.Getenv("XDG_STATE_HOME")
	if stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		stateDir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateDir, "lfk", "discovery-cache.yaml")
}

// loadDiscoveryCache reads the cache file. Returns nil on any failure —
// missing file, corrupt YAML, unknown schema. Callers fall through to a
// live discovery, so a nil result is recoverable.
func loadDiscoveryCache() *DiscoveryCacheState {
	path := discoveryCacheFilePath()
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var s DiscoveryCacheState
	if err := yaml.Unmarshal(data, &s); err != nil {
		logger.Warn("Discovery cache is corrupt; ignoring", "error", err, "path", path)
		return nil
	}
	if s.SchemaVersion != discoveryCacheSchemaVersion {
		logger.Info("Discovery cache schema version mismatch; ignoring",
			"got", s.SchemaVersion, "want", discoveryCacheSchemaVersion)
		return nil
	}
	if s.Contexts == nil {
		s.Contexts = make(map[string]DiscoveryCacheContext)
	}
	return &s
}

// saveDiscoveryCache writes the full cache document atomically (write to a
// sibling .tmp then rename) so a crash mid-write can't leave a half-written
// file that loadDiscoveryCache would discard.
func saveDiscoveryCache(state DiscoveryCacheState) error {
	path := discoveryCacheFilePath()
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	state.SchemaVersion = discoveryCacheSchemaVersion
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

// updateDiscoveryCacheContext is the single mutator: it loads the current
// state (or starts an empty one), replaces a context's entry list with
// `entries`, and writes the result back to disk. Other contexts in the file
// are preserved untouched so cluster-level updates don't trash unrelated
// snapshots.
func updateDiscoveryCacheContext(contextName string, entries []model.ResourceTypeEntry) error {
	if contextName == "" {
		return nil
	}
	state := loadDiscoveryCache()
	if state == nil {
		state = &DiscoveryCacheState{
			SchemaVersion: discoveryCacheSchemaVersion,
			Contexts:      make(map[string]DiscoveryCacheContext),
		}
	}
	if state.Contexts == nil {
		state.Contexts = make(map[string]DiscoveryCacheContext)
	}
	state.Contexts[contextName] = DiscoveryCacheContext{
		UpdatedAt: time.Now().UTC(),
		Entries:   discoveryCacheEntriesFromModel(entries),
	}
	return saveDiscoveryCache(*state)
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
