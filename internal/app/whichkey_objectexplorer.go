package app

import (
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// wkObjectExplorerCtx is the Object Explorer's resolved which-key context.
//
// Every row-dependent entry reads the same cursor row, and resolving it is not
// free: selected() goes through visible(), which re-filters the level (or
// re-flattens the folded tree) on every call (objectexplorer.go:63-93,
// objectexplorer_treemode.go:72-101). Four predicates asking independently
// would be four rebuilds per frame.
type wkObjectExplorerCtx struct {
	m *Model
	// hasSel is rt.selected()'s ok — the gate copySelectedNodeYAML applies
	// before anything else (objectexplorer.go:376-379).
	hasSel bool
	// path is selectedNodePath(): nil is copySelectedNodePath's own no-op
	// condition (objectexplorer.go:390-394).
	path []string
	// yankable reports that the node's YAML resolves. copySelectedNodeYAML
	// only toasts "Nothing to copy" when it does not (objectexplorer.go:632-646).
	// The marshal step is deliberately not run here: it allocates the whole
	// subtree as text, and a value that resolved out of the live object is what
	// yaml.Marshal is built for.
	yankable bool
	// foldable is toggleObjectExplorerTreeFold's full gate
	// (objectexplorer_treemode.go:105-113).
	foldable bool
}

// newWKObjectExplorerCtx resolves the cursor row once per availability pass.
// Safe on a zero-value Model: the state's root is nil, visible() returns an
// empty level and every ok below is false.
func newWKObjectExplorerCtx(m *Model) *wkObjectExplorerCtx {
	rt := &m.objectExplorerView
	c := &wkObjectExplorerCtx{m: m}
	_, c.hasSel = rt.selected()
	c.path = m.selectedNodePath()
	if c.hasSel && c.path != nil {
		_, c.yankable = model.ResolveObjectPath(rt.root, c.path)
	}
	if rt.tree && rt.filter == "" {
		if row, ok := rt.selectedTreeRow(); ok {
			c.foldable = row.Field.HasChildren
		}
	}
	return c
}

// whichKeyObjectExplorerActionList is the Object Explorer's catalog. Its drill
// and back keys (l/enter/right, h/backspace/left), j/k, g/G, the page keys and
// the J/K preview scroll are navigation and excluded on the explorer's rule —
// kb.PreviewDown / kb.PreviewUp are excluded registry-wide for exactly that.
//
// esc is absent because whichKeyLeaderIntercept consumes it while the panel is
// shown (whichkey_leader.go:169-181); what it would do here is the three-step
// clear-filter / back / close chain (objectexplorer.go:261-274), of which q
// only does the last step. ctrl+c is absent because it closes the tab or quits.
//
// The help key is read with wkLiteralHelpKey: this view matches a literal "?"
// (objectexplorer.go:275), not kb.Help.
var whichKeyObjectExplorerActionList = []wkAction[*wkObjectExplorerCtx]{
	{Key: wkLiteralHelpKey, Label: "Full help", Group: wkViews},
	// objectexplorer.go:258-260 -> exitObjectExplorer (237-241): q closes the
	// browser outright at any depth, where esc would walk back one level. The
	// label says so — this view has levels, so "Back" would read as the step
	// esc takes and h/backspace advertise.
	{Key: wkLiteralKey("q"), Label: "Close Object Explorer", Group: wkViews},

	// objectexplorer.go:286-291 — the in-level filter and the whole-object find.
	{Key: wkLiteralKey("/"), Label: "Filter this level", Group: wkFilter},
	{Key: wkLiteralKey("r"), Label: "Find across object", Group: wkFilter},

	// objectexplorer.go:294-297 — tree mode and its folds.
	{Key: func(kb ui.Keybindings) string { return kb.TreeView }, Label: "Flat level list", Group: wkViews, Avail: func(c *wkObjectExplorerCtx) bool {
		return c.m.objectExplorerView.tree
	}},
	{Key: func(kb ui.Keybindings) string { return kb.TreeView }, Label: "Subtree view", Group: wkViews, Avail: func(c *wkObjectExplorerCtx) bool {
		return !c.m.objectExplorerView.tree
	}},
	// "space" is the same case (objectexplorer.go:296) and is left
	// unadvertised: one action, one row.
	{Key: func(kb ui.Keybindings) string { return kb.ToggleFold }, Label: "Fold subtree at cursor", Group: wkViews, Avail: func(c *wkObjectExplorerCtx) bool {
		return c.foldable
	}},
	// objectexplorer.go:306-307 — hands the whole resource to the YAML viewer.
	{Key: wkLiteralKey("P"), Label: "Open in YAML viewer", Group: wkViews},
	// objectexplorer.go:300-301 -> openExplainAtObjectPath (update_explain.go:81-90),
	// which toasts "Cannot determine resource type" when the navigated type has
	// neither a plural resource nor a Kind to pluralise.
	{Key: wkLiteralKey("I"), Label: "API Explorer", Group: wkViews, Avail: func(c *wkObjectExplorerCtx) bool {
		rt := c.m.nav.ResourceType
		return rt.Resource != "" || rt.Kind != ""
	}},

	// objectexplorer.go:298-299 — a one-shot re-fetch, independent of the live
	// setting and of the cursor.
	{Key: func(kb ui.Keybindings) string { return kb.Refresh }, Label: "Refresh object", Group: wkActions},
	// objectexplorer.go:302-305 — the two yanks. Both need a cursor row; only
	// the YAML one needs the node to resolve.
	{Key: wkLiteralKey("y"), Label: "Copy node path", Group: wkActions, Avail: func(c *wkObjectExplorerCtx) bool {
		return c.path != nil
	}},
	{Key: wkLiteralKey("Y"), Label: "Copy node YAML", Group: wkActions, Avail: func(c *wkObjectExplorerCtx) bool {
		return c.yankable
	}},

	// objectexplorer.go:292-293 -> toggleObjectExplorerLive (216-225), which
	// reads its own direction, so the label names the state the key moves to.
	{Key: func(kb ui.Keybindings) string { return kb.WatchMode }, Label: "Live refresh on", Group: wkSettings, Avail: func(c *wkObjectExplorerCtx) bool {
		return !c.m.objectExplorerLive
	}},
	{Key: func(kb ui.Keybindings) string { return kb.WatchMode }, Label: "Live refresh off", Group: wkSettings, Avail: func(c *wkObjectExplorerCtx) bool {
		return c.m.objectExplorerLive
	}},
}

// whichKeyObjectExplorerCatalog is the Object Explorer's registry entry. The
// in-level filter input claims every printable key ahead of the browsing keys
// (handleObjectExplorerKey, objectexplorer.go:245-250), so "?" typed there is
// part of the filter.
var whichKeyObjectExplorerCatalog = wkCatalog[*wkObjectExplorerCtx]{
	resolve: newWKObjectExplorerCtx,
	input:   func(m *Model) bool { return m.objectExplorerView.filterActive },
	actions: whichKeyObjectExplorerActionList,
}
