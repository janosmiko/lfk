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
	Pair  string
}

// wkAction is one catalog entry. C is the catalog's OWN resolved context type.
// What that actually buys, precisely:
//
//   - A predicate cannot read another viewer's RESOLVED context. wkYAMLCtx's
//     memoized fields (visibleLines, foldSection, sel) are unreachable from a
//     log predicate, so a predicate can never answer from a field its own mode
//     never resolved and left at zero.
//   - Entries cannot be moved between catalogs. A wkAction[*wkYAMLCtx] does not
//     fit wkCatalog[*wkLogCtx].actions, so a copy-pasted row is a compile error
//     rather than a live entry whose predicate reads the wrong viewer.
//   - resolve runs once per availability pass no matter how many predicates read
//     the context, which is load-bearing: a regression that lost it took the
//     render from 211 to 420 allocs/op.
//
// What it does NOT buy, and you must check yourself: every context embeds
// m *Model, so ANY predicate can reach ANY Model field, including another
// viewer's state. That is sometimes correct — wkYAMLObjectExplorerAvailable
// reads explorer row state because its handler does — but the type system will
// not stop you writing a predicate that disagrees with its handler. Read the
// handler.
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
	// Pair names a BIDIRECTIONAL pair: two entries, two keys, one value moved
	// in opposite directions (raise/lower a threshold, next/previous column).
	// Both halves must carry the same name, and
	// TestWhichKeyCatalogs_BidirectionalPairsAppearTogether then requires them
	// to be offered together or not at all.
	//
	// USER DECISION: a clamped half is still shown at its limit, because the
	// alternative makes it undiscoverable — `i` (lower severity) only appeared
	// after the user had already found `o` (raise it), which is the wrong way
	// round for a discovery aid. Pressing the inert half is a harmless no-op.
	//
	// Declared, not inferred: nothing in an entry says which value its handler
	// moves or in which direction, so a guard that tried to work pairs out
	// from keys or labels would be guessing. Naming the pair is the honest
	// version, and an unnamed pair simply gets no guard rather than a wrong one.
	//
	// This is NOT for one key with two labels (follow/unfollow, unified/
	// side-by-side): those are mutually exclusive by construction, exactly one
	// is ever visible, and nothing is hidden from the user.
	Pair string
}

func (a wkAction[C]) entry() whichKeyEntry {
	return whichKeyEntry{Key: a.Key, Label: a.Label, Group: a.Group, Order: a.Order, Pair: a.Pair}
}

// wkLiteralKey wraps a key a handler hardcodes in its switch rather than
// reading from ui.Keybindings, so the entry advertises exactly the keystroke
// the case matches. The viewers do this a lot: "y", "v", "q" and — less
// obviously — "O" and "I" in the YAML viewer, which do NOT follow
// kb.ObjectExplorer / kb.APIExplorer.
func wkLiteralKey(key string) func(ui.Keybindings) string {
	return func(ui.Keybindings) string { return key }
}

// wkLiteralHelpKey is whichKeyHelpKey for the two viewers whose help case
// matches a HARDCODED "?" rather than kb.Help — the API Explorer
// (update_explain.go:240) and the Object Explorer (objectexplorer.go:275).
// Rebinding kb.Help does not move that key, so the panel must keep saying "?";
// the leader still wins it when it is bound there, leaving f1 the only way in.
func wkLiteralHelpKey(kb ui.Keybindings) string {
	if kb.WhichKeyLeader == "?" {
		return "f1"
	}
	return "?"
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
	keys := make([]string, 0, len(c.actions))
	for _, a := range c.actions {
		k := a.Key(kb)
		if k == "" {
			continue
		}
		if a.Avail != nil && !a.Avail(ctx) {
			continue
		}
		out = append(out, a.entry())
		keys = append(keys, k)
	}
	return wkDropAmbiguousKeys(out, keys)
}

// wkDropAmbiguousKeys removes every entry that shares its resolved key with
// another entry surviving the same pass.
//
// This only ever fires after a REBIND lands a ui.Keybindings field on a key one
// of the viewers hardcodes — kb.Refresh = "r" collides with the Object
// Explorer's literal "r" (objectexplorer.go:289), a k9s habit that costs
// nothing to configure. One of the two rows is then a lie, and the panel cannot
// tell which: the viewers interleave literal and kb.* cases in their switches
// (update_describe.go dispatches kb.ToggleWrap BEFORE "y"; update_yaml.go
// dispatches "y" before kb.Refresh), so neither "the literal wins" nor "the
// binding wins" holds. Advertising neither is the only answer that is never
// false; the key still works, it just stops being advertised as two things.
//
// Under any keybinding set that does not create such an overlap this is a
// no-op: the entries that deliberately share a key (y with three meanings, q
// with two) carry mutually exclusive predicates, so at most one survives the
// filter above. The scan is O(n^2) over ~40 short strings and allocates nothing
// unless it actually finds a collision.
func wkDropAmbiguousKeys(out []whichKeyEntry, keys []string) []whichKeyEntry {
	dup := false
	for i := range keys {
		for j := i + 1; j < len(keys) && !dup; j++ {
			dup = keys[i] == keys[j]
		}
		if dup {
			break
		}
	}
	if !dup {
		return out
	}
	kept := make([]whichKeyEntry, 0, len(out))
	for i := range out {
		unique := true
		for j := range keys {
			if j != i && keys[j] == keys[i] {
				unique = false
				break
			}
		}
		if unique {
			kept = append(kept, out[i])
		}
	}
	return kept
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
	{modeDiff, "diff view", whichKeyDiffCatalog},
	{modeExplain, "API Explorer", whichKeyExplainCatalog},
	{modeObjectExplorer, "Object Explorer", whichKeyObjectExplorerCatalog},
	{modeLogTop, "Log Top", whichKeyLogTopCatalog},
	{modeEventViewer, "event viewer", whichKeyEventViewerCatalog},
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
