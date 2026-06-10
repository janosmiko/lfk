package app

import (
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/paths"
)

// persistedColumnPrefs is the on-disk form of a single kind's committed column
// layout: display order, visible extra columns, and hidden built-in columns.
type persistedColumnPrefs struct {
	Order []string `json:"order,omitempty" yaml:"order,omitempty"`
	// VisibleExtras has no omitempty: a non-nil empty slice means "user
	// explicitly configured no extra columns" and must round-trip distinctly
	// from an absent entry (auto-detect). See applySessionColumnsForKind.
	VisibleExtras  []string `json:"visible_extras" yaml:"visible_extras"`
	HiddenBuiltins []string `json:"hidden_builtins,omitempty" yaml:"hidden_builtins,omitempty"`
}

// ColumnPrefsState is the on-disk schema for committed per-kind column layouts,
// scoped by kube context then Kind (matching columnMemoryKey). It mirrors the
// sort-memory state file.
type ColumnPrefsState struct {
	Contexts map[string]map[string]persistedColumnPrefs `json:"contexts" yaml:"contexts"`
}

// columnPrefsFilePath returns the path to the column-prefs state file.
func columnPrefsFilePath() string {
	dir, err := paths.StateDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "column_prefs.yaml")
}

// loadColumnPrefsState reads the raw nested state from disk, returning an empty
// (never nil Contexts) value when the file is missing or corrupt.
func loadColumnPrefsState() ColumnPrefsState {
	empty := ColumnPrefsState{Contexts: map[string]map[string]persistedColumnPrefs{}}
	path := columnPrefsFilePath()
	if path == "" {
		return empty
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("Failed to read column-prefs state", "error", err, "path", path)
		}
		return empty
	}
	var s ColumnPrefsState
	if err := yaml.Unmarshal(data, &s); err != nil {
		logger.Warn("Column-prefs file is corrupt; starting fresh", "error", err, "path", path)
		return empty
	}
	if s.Contexts == nil {
		s.Contexts = map[string]map[string]persistedColumnPrefs{}
	}
	return s
}

// saveColumnPrefsState writes the nested state to disk.
func saveColumnPrefsState(s ColumnPrefsState) error {
	path := columnPrefsFilePath()
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// columnPrefMaps bundles the three in-memory column maps for seeding the model.
type columnPrefMaps struct {
	sessionColumns       map[string][]string
	hiddenBuiltinColumns map[string][]string
	columnOrder          map[string][]string
}

// loadColumnPrefs reads committed column layouts from disk and flattens them
// into the three "context\x00kind"-keyed maps the model uses. All three maps are
// non-nil. A persisted entry always carries VisibleExtras (a committed layout
// always records the visible-extras set, possibly empty); Order and
// HiddenBuiltins are populated only when present.
func loadColumnPrefs() columnPrefMaps {
	maps := columnPrefMaps{
		sessionColumns:       map[string][]string{},
		hiddenBuiltinColumns: map[string][]string{},
		columnOrder:          map[string][]string{},
	}
	state := loadColumnPrefsState()
	for ctx, kinds := range state.Contexts {
		for kind, p := range kinds {
			key := ctx + "\x00" + kind
			extras := p.VisibleExtras
			if extras == nil {
				extras = []string{}
			}
			maps.sessionColumns[key] = extras
			if len(p.HiddenBuiltins) > 0 {
				maps.hiddenBuiltinColumns[key] = p.HiddenBuiltins
			}
			if len(p.Order) > 0 {
				maps.columnOrder[key] = p.Order
			}
		}
	}
	return maps
}

// persistColumnPrefsEntry writes one kind's committed column layout to disk,
// merging with other contexts/kinds so a commit for one never drops another's.
// Best-effort. Runs only on the single Bubble Tea Update goroutine (commit/reset
// handlers), so the load-modify-save is never concurrent.
func persistColumnPrefsEntry(key string, p persistedColumnPrefs) {
	ctx, kind, ok := strings.Cut(key, "\x00")
	if !ok {
		return
	}
	state := loadColumnPrefsState()
	if state.Contexts[ctx] == nil {
		state.Contexts[ctx] = map[string]persistedColumnPrefs{}
	}
	state.Contexts[ctx][kind] = p
	if err := saveColumnPrefsState(state); err != nil {
		logger.Error("Failed to persist column prefs", "error", err)
	}
}

// persistForgottenColumnPrefs removes one kind's column layout from disk (the
// reset action or a total-clear commit), leaving every other entry intact.
func persistForgottenColumnPrefs(key string) {
	ctx, kind, ok := strings.Cut(key, "\x00")
	if !ok {
		return
	}
	state := loadColumnPrefsState()
	kinds, ok := state.Contexts[ctx]
	if !ok {
		return
	}
	if _, ok := kinds[kind]; !ok {
		return
	}
	delete(kinds, kind)
	if len(kinds) == 0 {
		delete(state.Contexts, ctx)
	}
	if err := saveColumnPrefsState(state); err != nil {
		logger.Error("Failed to persist column prefs", "error", err)
	}
}

// persistColumnPrefs persists (or clears) the committed column layout for the
// kind currently shown in the middle column. Called on Enter (commit) and R
// (reset) from the column-toggle overlay — never on live-apply, so an Esc that
// reverts the in-memory maps leaves the on-disk layout untouched.
func (m *Model) persistColumnPrefs() {
	key := m.columnMemoryKey(m.middleColumnKind())
	extras, ok := m.sessionColumns[key]
	if !ok {
		// No session entry means the layout was reset / cleared; drop it.
		persistForgottenColumnPrefs(key)
		return
	}
	if extras == nil {
		extras = []string{}
	}
	persistColumnPrefsEntry(key, persistedColumnPrefs{
		Order:          m.columnOrder[key],
		VisibleExtras:  extras,
		HiddenBuiltins: m.hiddenBuiltinColumns[key],
	})
}
