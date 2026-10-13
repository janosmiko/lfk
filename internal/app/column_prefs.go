package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/logger"
)

const columnPrefsFileName = "column_prefs.yaml"

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
	return stateFilePath(columnPrefsFileName)
}

// loadColumnPrefsState reads the raw nested state from disk, returning an empty
// (never nil Contexts) value when the file is missing or corrupt.
func loadColumnPrefsState() ColumnPrefsState {
	s := loadStateFile[ColumnPrefsState](columnPrefsFileName)
	if s.Contexts == nil {
		s.Contexts = map[string]map[string]persistedColumnPrefs{}
	}
	return s
}

// loadColumnPrefsStateStrict is the read-modify-write loader: it fails on an
// unreadable or corrupt file instead of returning empty state.
func loadColumnPrefsStateStrict() (ColumnPrefsState, error) {
	s, err := loadStateFileStrict[ColumnPrefsState](columnPrefsFileName)
	if err != nil {
		return s, err
	}
	if s.Contexts == nil {
		s.Contexts = map[string]map[string]persistedColumnPrefs{}
	}
	return s, nil
}

// saveColumnPrefsState writes the nested state to disk.
func saveColumnPrefsState(s ColumnPrefsState) error {
	return saveStateFile(columnPrefsFileName, s)
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
// Best-effort. Callers run on the single Bubble Tea Update goroutine, and the
// lock extends that guarantee across processes: two lfk instances sharing a
// state directory would otherwise read the same file and the later writer would
// drop the layout the other one just committed.
func persistColumnPrefsEntry(key string, p persistedColumnPrefs) error {
	ctx, kind, ok := strings.Cut(key, "\x00")
	if !ok {
		return nil
	}
	path, err := stateFilePathErr(columnPrefsFileName)
	if err != nil {
		return err
	}
	return updateStateFile(path, func() error {
		state, err := loadColumnPrefsStateStrict()
		if err != nil {
			return err
		}
		if state.Contexts[ctx] == nil {
			state.Contexts[ctx] = map[string]persistedColumnPrefs{}
		}
		state.Contexts[ctx][kind] = p
		err = saveColumnPrefsState(state)
		if err != nil {
			logger.Error("Failed to persist column prefs", "error", err)
		}
		return err
	})
}

// persistForgottenColumnPrefs removes one kind's column layout from disk (the
// reset action or a total-clear commit), leaving every other entry intact.
func persistForgottenColumnPrefs(key string) error {
	ctx, kind, ok := strings.Cut(key, "\x00")
	if !ok {
		return nil
	}
	path, err := stateFilePathErr(columnPrefsFileName)
	if err != nil {
		return err
	}
	return updateStateFile(path, func() error {
		state, err := loadColumnPrefsStateStrict()
		if err != nil {
			return err
		}
		kinds, ok := state.Contexts[ctx]
		if !ok {
			return nil
		}
		if _, ok := kinds[kind]; !ok {
			return nil
		}
		delete(kinds, kind)
		if len(kinds) == 0 {
			delete(state.Contexts, ctx)
		}
		err = saveColumnPrefsState(state)
		if err != nil {
			logger.Error("Failed to persist column prefs", "error", err)
		}
		return err
	})
}

// persistColumnPrefs persists (or clears) the committed column layout for the
// kind currently shown in the middle column. Called on Enter (commit) and R
// (reset) from the column-toggle overlay — never on live-apply, so an Esc that
// reverts the in-memory maps leaves the on-disk layout untouched.
func (m *Model) persistColumnPrefs() tea.Cmd {
	key := m.columnMemoryKey(m.middleColumnKind())
	extras, ok := m.sessionColumns[key]
	if !ok {
		// No session entry means the layout was reset / cleared; drop it.
		return m.reportPrefSaveErr(persistForgottenColumnPrefs(key))
	}
	if extras == nil {
		extras = []string{}
	}
	return m.reportPrefSaveErr(persistColumnPrefsEntry(key, persistedColumnPrefs{
		Order:          m.columnOrder[key],
		VisibleExtras:  extras,
		HiddenBuiltins: m.hiddenBuiltinColumns[key],
	}))
}
