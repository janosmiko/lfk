package app

import (
	"github.com/janosmiko/lfk/internal/ui"
)

// wkLogTopCtx is the Log Top aggregation view's resolved which-key context.
//
// Both derived fields walk the aggregation's column and dimension lists
// (logTopSortColumns -> logTopVisibleDims + logTopVisibleMetrics,
// logtop_build.go:292-295; logTopDrillCandidates, update_logtop.go:293-310),
// and each is read by more than one entry.
type wkLogTopCtx struct {
	m *Model
	// sortable reports at least one sortable column. logTopCycleSort returns
	// unchanged without one (logtop_build.go:395-399).
	sortable bool
	// drillable reports an un-pinned dimension left to break down by, the
	// condition logTopCycleDrillTarget and logTopDrillIn both check
	// (update_logtop.go:271-278, 327-331).
	drillable bool
}

func newWKLogTopCtx(m *Model) *wkLogTopCtx {
	return &wkLogTopCtx{
		m:         m,
		sortable:  len(m.logTopSortColumns()) > 0,
		drillable: len(m.logTopDrillCandidates()) > 0,
	}
}

// whichKeyLogTopActionList is Log Top's catalog. It is a table browser, not a
// text viewer: j/k, g/G, home/end and the page keys are its only motions and
// are excluded on the explorer's rule, as are n/N and enter — drilling in is
// navigation, the same way the explorer's Enter is.
//
// There is no "Full help" entry because there is no way in: handleLogTopKey
// (update_logtop.go:22-148) has no help case at all, so "?" was a silent no-op
// here before the panel claimed it. esc is absent because
// whichKeyLeaderIntercept consumes it while the panel is shown
// (whichkey_leader.go:169-181); q is the same case in the handler
// (update_logtop.go:34) and carries both of its outcomes below.
var whichKeyLogTopActionList = []wkAction[*wkLogTopCtx]{
	// update_logtop.go:34-47 — one key, two outcomes, decided by the drill stack.
	{Key: wkLiteralKey("q"), Label: "Pop drill level", Group: wkViews, Avail: func(c *wkLogTopCtx) bool {
		return len(c.m.logTop.drillStack) > 0
	}},
	{Key: wkLiteralKey("q"), Label: "Back to log viewer", Group: wkViews, Avail: func(c *wkLogTopCtx) bool {
		return len(c.m.logTop.drillStack) == 0
	}},

	// update_logtop.go:95-103 — the three pickers. All open an overlay
	// unconditionally.
	{Key: wkLiteralKey("."), Label: "Group-by fields", Group: wkViews},
	{Key: func(kb ui.Keybindings) string { return kb.ColumnToggle }, Label: "Column visibility", Group: wkViews},
	// update_logtop.go:123-126 — picks which dimension the next drill uses.
	{Key: wkLiteralKey("tab"), Label: "Cycle drill dimension", Group: wkSelection, Avail: func(c *wkLogTopCtx) bool {
		return c.drillable
	}},

	// update_logtop.go:130-139.
	{Key: func(kb ui.Keybindings) string { return kb.Filter }, Label: "Filter rows", Group: wkFilter},
	{Key: func(kb ui.Keybindings) string { return kb.Search }, Label: "Search rows", Group: wkFilter},

	// Sort. update_logtop.go:104-122. The cycle keys need a column to cycle
	// through; flip and reset write the sort state either way. The explicit
	// Order matches the explorer's ("<" then ">" then "="), which a plain ASCII
	// compare would otherwise split apart.
	{Key: func(kb ui.Keybindings) string { return kb.SortPrev }, Label: "Sort previous column", Group: wkSort, Order: 1, Avail: func(c *wkLogTopCtx) bool {
		return c.sortable
	}},
	{Key: func(kb ui.Keybindings) string { return kb.SortNext }, Label: "Sort next column", Group: wkSort, Order: 2, Avail: func(c *wkLogTopCtx) bool {
		return c.sortable
	}},
	{Key: func(kb ui.Keybindings) string { return kb.SortFlip }, Label: "Flip sort direction", Group: wkSort, Order: 3},
	{Key: func(kb ui.Keybindings) string { return kb.SortReset }, Label: "Reset sort", Group: wkSort},

	// update_logtop.go:98-100 — the parser profile the rows are aggregated
	// from. A parsing setting, so it files with the other sticky toggles rather
	// than with the two pickers that restructure the table.
	{Key: wkLiteralKey("p"), Label: "Log format profile", Group: wkSettings},
}

// whichKeyLogTopCatalog is Log Top's registry entry. Both the filter and the
// search prompt claim printable keys ahead of the browsing keys
// (update_logtop.go:12-17), so "?" typed there is part of the query.
var whichKeyLogTopCatalog = wkCatalog[*wkLogTopCtx]{
	resolve: newWKLogTopCtx,
	input:   func(m *Model) bool { return m.logTop.filterActive || m.logTop.searchActive },
	actions: whichKeyLogTopActionList,
}
