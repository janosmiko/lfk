package app

import (
	"github.com/janosmiko/lfk/internal/ui"
)

// whichKeyEntry is the context-free half of a catalog entry: everything the
// panel draws and every registry guard reads, with no predicate attached.
// Predicates are deliberately absent — a guard that sweeps every mode's
// catalog cannot know, and does not need, the context type each one resolves.
type whichKeyEntry struct {
	Key   func(kb ui.Keybindings) string
	Label string
	Group whichKeyGroup
	Order int
}

// wkAction is one catalog entry. C is the catalog's OWN resolved context type,
// which is what makes the per-mode split safe rather than merely tidy: a YAML
// predicate cannot compile against explorer row state, and an explorer
// predicate cannot compile against yamlView. The alternative — one wkCtx
// carrying every mode's fields — would leave a dozen viewers' worth of
// always-zero fields readable from every predicate, which is precisely how a
// predicate ends up silently answering from state its own mode never resolved.
//
// Key is a function so a rebind is picked up at render time rather than baked
// in at package init. Avail is nil for entries that always apply in that mode;
// it must be cheap and side-effect free because it runs on every render.
type wkAction[C any] struct {
	Key   func(kb ui.Keybindings) string
	Label string
	Group whichKeyGroup
	Avail func(c C) bool
	// Order is an optional explicit sort override, applied as a tiebreak
	// after the group and modifier-tier passes but before the plain key
	// sort (sortWhichKeyCells). Zero means "unspecified" and falls through
	// to the normal sort — mirrors neovim's which-key `order` sorter
	// (view.lua's M.fields.order), which is the escape hatch for a pair
	// like "<"/">" that a plain ASCII compare would otherwise split apart.
	// Leave unset unless an entry needs a specific position within its group.
	Order int
}

func (a wkAction[C]) entry() whichKeyEntry {
	return whichKeyEntry{Key: a.Key, Label: a.Label, Group: a.Group, Order: a.Order}
}

// wkLiteralKey wraps a key a handler hardcodes in its switch rather than
// reading from ui.Keybindings, so the entry advertises exactly the keystroke
// the case matches. The viewers do this a lot: "y", "v", "q" and — less
// obviously — "O" and "I" in the YAML viewer, which do NOT follow
// kb.ObjectExplorer / kb.APIExplorer.
func wkLiteralKey(key string) func(ui.Keybindings) string {
	return func(ui.Keybindings) string { return key }
}

// whichKeyCatalog is one mode's catalog with its context type erased, so the
// panel and the guards can iterate every mode through a single interface.
// Adding a viewer is a new wkCatalog value plus one row in
// whichKeyCatalogList — no dispatch, render, sort, or guard code changes.
type whichKeyCatalog interface {
	// entries lists what the catalog advertises, resolving no context. Used
	// by the drift and group guards, which ask what COULD be offered.
	entries() []whichKeyEntry
	// available filters the catalog against m, resolving its context once.
	available(m *Model) []whichKeyEntry
	// inputFocused reports whether a text input owns the keyboard right now,
	// in which case the leader key is a literal character and must not arm.
	inputFocused(m *Model) bool
}

// wkCatalog is the generic implementation behind whichKeyCatalog. resolve runs
// once per availability pass — the same once-per-call contract wkCtx was
// introduced for, and the reason a viewer gets a context struct at all rather
// than reading straight off the Model: a YAML predicate that needs the folded
// section under the cursor costs a full document walk, and two predicates
// needing it would otherwise pay for two.
type wkCatalog[C any] struct {
	resolve func(m *Model) C
	// input reports a focused text input; nil means the mode has none that
	// reaches the leader dispatch.
	input   func(m *Model) bool
	actions []wkAction[C]
}

func (c wkCatalog[C]) entries() []whichKeyEntry {
	out := make([]whichKeyEntry, 0, len(c.actions))
	for _, a := range c.actions {
		out = append(out, a.entry())
	}
	return out
}

func (c wkCatalog[C]) available(m *Model) []whichKeyEntry {
	kb := ui.ActiveKeybindings
	ctx := c.resolve(m)
	out := make([]whichKeyEntry, 0, len(c.actions))
	for _, a := range c.actions {
		if a.Key(kb) == "" {
			continue
		}
		if a.Avail != nil && !a.Avail(ctx) {
			continue
		}
		out = append(out, a.entry())
	}
	return out
}

func (c wkCatalog[C]) inputFocused(m *Model) bool {
	return c.input != nil && c.input(m)
}

// whichKeyModeCatalog binds one viewMode to its catalog. name is the label the
// guards report a failure under; the modes have no String() of their own.
type whichKeyModeCatalog struct {
	mode    viewMode
	name    string
	catalog whichKeyCatalog
}

// whichKeyCatalogList is the registry, in a stable order so guards that sweep
// every catalog report deterministically. This slice is the single seam: a
// mode that appears here has a which-key panel, and a mode that does not has
// the leader key fall through to whatever else claims it there.
var whichKeyCatalogList = []whichKeyModeCatalog{
	{modeExplorer, "explorer", whichKeyExplorerCatalog},
	{modeYAML, "YAML view", whichKeyYAMLCatalog},
	{modeLogs, "log viewer", whichKeyLogCatalog},
	{modeDescribe, "describe view", whichKeyDescribeCatalog},
}

// whichKeyCatalogs indexes whichKeyCatalogList for the render and dispatch
// paths, which look a single mode up per frame or keypress.
var whichKeyCatalogs = func() map[viewMode]whichKeyCatalog {
	out := make(map[viewMode]whichKeyCatalog, len(whichKeyCatalogList))
	for _, mc := range whichKeyCatalogList {
		out[mc.mode] = mc.catalog
	}
	return out
}()

// availableWhichKeyActions filters the CURRENT mode's catalog to what applies
// right now, dropping entries whose binding the user cleared. A mode with no
// catalog returns nothing, so the panel renders empty rather than showing
// another mode's keys.
//
// Pointer receiver on purpose: Model is ~18 KB, and taking its address for the
// predicates makes a value receiver's copy escape to the heap on every render.
// Safe because every predicate is required to be read-only.
func (m *Model) availableWhichKeyActions() []whichKeyEntry {
	cat, ok := whichKeyCatalogs[m.mode]
	if !ok {
		return nil
	}
	return cat.available(m)
}

// whichKeyLeaderArmable reports whether the leader key should open the panel in
// the current mode: the panel is enabled, the mode has a catalog, and no text
// input owns the keyboard — inside a search or filter prompt "?" is a
// character the user is typing, not a command.
func (m *Model) whichKeyLeaderArmable() bool {
	if !ui.ConfigWhichKeyEnabled {
		return false
	}
	cat, ok := whichKeyCatalogs[m.mode]
	return ok && !cat.inputFocused(m)
}
