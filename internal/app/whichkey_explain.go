package app

import (
	"github.com/janosmiko/lfk/internal/ui"
)

// wkExplainCtx is the API Explorer's resolved which-key context.
//
// The API Explorer has no visual mode and no second key handler, so unlike the
// text viewers there is no wholesale catalog swap. What the entries branch on
// is the tree/flat state, which decides what kb.TreeView and the fold key do.
type wkExplainCtx struct {
	m *Model
	// foldable is toggleExplainTreeFold's full gate (explain_tree.go:280-287):
	// tree mode, a cursor on a real field, and that field having children in
	// the loaded subtree. Resolved once because explainTreeHasChildren scans
	// the whole tree.
	foldable bool
}

// newWKExplainCtx resolves the fold gate once per availability pass. Safe on a
// zero-value Model: explainTree is false, which short-circuits before any
// slice is indexed.
func newWKExplainCtx(m *Model) *wkExplainCtx {
	c := &wkExplainCtx{m: m}
	if m.explainTree && m.explainCursor >= 0 && m.explainCursor < len(m.explainFields) {
		c.foldable = explainTreeHasChildren(m.explainTreeAll, m.explainFields[m.explainCursor].Path)
	}
	return c
}

// whichKeyExplainActionList is the API Explorer's catalog. Its drill keys
// (l/enter/right) and back keys (h/backspace/left) are navigation and excluded
// on the explorer's rule, as are j/k, gg/G, the page keys, the digit prefix and
// n/N.
//
// esc is absent for two reasons at once: whichKeyLeaderIntercept consumes it
// while the panel is shown (whichkey_leader.go:169-181), and what it does here
// is walk back a schema level (handleExplainKeyEsc, update_explain.go:647-657),
// which is navigation. ctrl+c is absent because it closes the tab or quits.
//
// The help key is read with wkLiteralHelpKey, not whichKeyHelpKey: this view
// matches a literal "?" (update_explain.go:240) instead of kb.Help.
var whichKeyExplainActionList = []wkAction[*wkExplainCtx]{
	{Key: wkLiteralHelpKey, Label: "Full help", Group: wkViews},
	// update_explain.go:242-243 -> handleExplainKeyQ (640-645): q leaves the
	// view outright, unlike esc, which walks back one level first.
	{Key: wkLiteralKey("q"), Label: "Back", Group: wkViews},

	// update_explain.go:246-247 / 252-253 — the level search and the recursive
	// browser. Both are unconditional: the search prompt opens on an empty
	// level too, and the recursive fetch always has a resource to run against
	// (nothing reaches this mode without explainResource set,
	// openExplainBrowser update_explain.go:65-72).
	{Key: wkLiteralKey("/"), Label: "Search fields", Group: wkFilter},
	{Key: wkLiteralKey("r"), Label: "Recursive field browser", Group: wkFilter},

	// update_explain.go:254-256 -> toggleExplainTree (explain_tree.go:62-79).
	// Three states, three labels: in the tree the key leaves it, with a fetch
	// in flight it cancels that fetch, and otherwise it loads the tree.
	{Key: func(kb ui.Keybindings) string { return kb.TreeView }, Label: "Flat field list", Group: wkViews, Avail: func(c *wkExplainCtx) bool {
		return c.m.explainTree
	}},
	{Key: func(kb ui.Keybindings) string { return kb.TreeView }, Label: "Cancel field tree load", Group: wkViews, Avail: func(c *wkExplainCtx) bool {
		return !c.m.explainTree && c.m.explainTreeWanted
	}},
	{Key: func(kb ui.Keybindings) string { return kb.TreeView }, Label: "Field tree view", Group: wkViews, Avail: func(c *wkExplainCtx) bool {
		return !c.m.explainTree && !c.m.explainTreeWanted
	}},
	// update_explain.go:257-259. "space" is the same case and is left
	// unadvertised: one action, one row.
	{Key: func(kb ui.Keybindings) string { return kb.ToggleFold }, Label: "Fold subtree at cursor", Group: wkViews, Avail: func(c *wkExplainCtx) bool {
		return c.foldable
	}},
}

// whichKeyExplainCatalog is the API Explorer's registry entry. The level search
// prompt claims every printable key ahead of handleExplainKey
// (update_keys.go:241-244), so "?" typed there is part of the query. The
// recursive browser's own filter needs no entry here: it lives behind
// m.overlay, and handleKey returns at the overlay branch before the leader is
// ever offered the key (update_keys.go:35-37).
var whichKeyExplainCatalog = wkCatalog[*wkExplainCtx]{
	resolve: newWKExplainCtx,
	input:   func(m *Model) bool { return m.explainSearchActive },
	actions: whichKeyExplainActionList,
}
