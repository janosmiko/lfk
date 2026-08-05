package app

import (
	"github.com/janosmiko/lfk/internal/ui"
)

// wkDiffCtx is the diff viewer's resolved which-key context.
//
// visual is read by nearly every entry: handleDiffKey routes to
// handleDiffVisualKey BEFORE handleDiffNormalKey (update_diff.go:22-27), and
// the visual handler has no case for the toggles, the search key, the folds, or
// q — pressing them there is a silent no-op, not a fallthrough.
//
// The diff itself is resolved once because ui.computeDiff builds an O(nxm) LCS
// table (overlay_diff.go:20-50) and three entries need a different fact off the
// same pass: the visible-line count handleDiffNormalCopy stops at, the fold
// region under the cursor, and the text the yank would copy.
type wkDiffCtx struct {
	m      *Model
	visual bool
	// counted reports an armed digit prefix. handleDiffNormalCopy consumes it
	// (update_diff.go:500), so `y` copies N lines rather than one.
	counted bool
	// regions is the foldable set kb.ToggleFoldAll acts on; empty means the
	// key changes nothing (toggleAllDiffFolds, update_diff.go:626-639).
	regions []ui.DiffFoldRegion
	// vis is the visible-line list the cursor indexes into. Its length is
	// handleDiffNormalCopy's bound (update_diff.go:501) and its RegionIdx is
	// what toggleDiffFoldAtCursor needs (update_diff.go:604-607).
	vis []ui.VisibleDiffLine
	// lineText is the active side's text under the cursor. handleDiffNormalCopy
	// SKIPS empty-side lines and copies nothing when every line in its range is
	// empty (update_diff.go:503-512) — on a side-by-side diff that is every
	// added line seen from the left pane, so it is not an edge case.
	lineText string
	// countedText is lineText's answer for the whole COUNTED range: at least
	// one line in [cursor, cursor+count) carries text on the active side. The
	// handler's empty-parts return is a second silent no-op the cursor bound
	// alone does not model, and a run of consecutive added lines read from the
	// left pane hits it.
	countedText bool
}

// newWKDiffCtx resolves the diff and the cursor's line once per availability
// pass. Safe on a zero-value Model: computeDiff of two empty strings yields no
// lines and every bound below is a length compare.
func newWKDiffCtx(m *Model) *wkDiffCtx {
	// One computeDiff pass, not two: ComputeDiffFoldRegions would run its own.
	raw := ui.ComputeDiffLines(m.diffView.left, m.diffView.right)
	regions := ui.ComputeDiffFoldRegionsFromLines(raw)
	vis := ui.BuildVisibleDiffLines(raw, regions, m.diffView.foldState)
	c := &wkDiffCtx{
		m:        m,
		visual:   m.diffView.visualMode,
		counted:  m.diffView.lineInput != "",
		regions:  regions,
		vis:      vis,
		lineText: ui.DiffLineTextIn(raw, vis, m.diffView.cursor, m.diffView.cursorSide, m.diffView.unified),
	}
	// Mirror handleDiffNormalCopy's loop, not just its bound: it skips
	// empty-side lines and returns unchanged once none survived. Bounded by
	// len(vis) exactly as the handler's `end` is, so a huge count prefix costs
	// one pass over an already-resolved diff, not more.
	if c.counted {
		end := min(m.diffView.cursor+parseCountPrefix(m.diffView.lineInput), len(vis))
		for i := max(m.diffView.cursor, 0); i < end; i++ {
			if ui.DiffLineTextIn(raw, vis, i, m.diffView.cursorSide, m.diffView.unified) != "" {
				c.countedText = true
				break
			}
		}
	}
	return c
}

func wkDiffNormal(c *wkDiffCtx) bool { return !c.visual }
func wkDiffVisual(c *wkDiffCtx) bool { return c.visual }

// wkDiffFoldAtCursor mirrors toggleDiffFoldAtCursor (update_diff.go:592-622):
// the visible line under the cursor must belong to a fold region. The handler
// clamps an over-long cursor to the last line first, which is why the bound
// here is the clamped index rather than a plain range check.
func wkDiffFoldAtCursor(c *wkDiffCtx) bool {
	if c.visual || len(c.vis) == 0 {
		return false
	}
	idx := min(c.m.diffView.cursor, len(c.vis)-1)
	if idx < 0 {
		return false
	}
	return c.vis[idx].RegionIdx >= 0 && c.vis[idx].RegionIdx < len(c.regions)
}

// whichKeyDiffActionList is the diff viewer's catalog. Motions (j/k/h/l,
// w/b/e, gg/G, ctrl+d/u/f/b, 0/$/^, the digit prefix) and n/N are absent for
// the same reason they are absent from the explorer's: the panel advertises
// actions, not the keymap.
//
// esc is absent because whichKeyLeaderIntercept CONSUMES it while the panel is
// shown (whichkey_leader.go:169-181), so the first esc closes the panel rather
// than reaching handleDiffQuit. ctrl+c is absent because it closes the tab or
// quits, which no catalog advertises.
//
// Unlike the YAML and describe viewers, q has ONE meaning here: handleDiffQuit
// (update_diff.go:234-249) leaves the viewer whether or not a search is
// applied — it clears the query on the way out rather than as a first step.
var whichKeyDiffActionList = []wkAction[*wkDiffCtx]{
	// update_diff.go:109-116 — kb.Help and the f1 alias both reach help.
	{Key: whichKeyHelpKey, Label: "Full help", Group: wkViews, Avail: wkDiffNormal},
	{Key: wkLiteralKey("q"), Label: "Back", Group: wkViews, Avail: wkDiffNormal},

	// update_diff.go:214-221 — folds. The two keys share one case; which one
	// ran is re-read from the key string inside it.
	{Key: func(kb ui.Keybindings) string { return kb.ToggleFold }, Label: "Fold unchanged block at cursor", Group: wkViews, Avail: wkDiffFoldAtCursor},
	{Key: func(kb ui.Keybindings) string { return kb.ToggleFoldAll }, Label: "Fold / unfold all", Group: wkViews, Avail: func(c *wkDiffCtx) bool {
		return !c.visual && len(c.regions) > 0
	}},

	// update_diff.go:198-204.
	{Key: func(kb ui.Keybindings) string { return kb.Search }, Label: "Search in content", Group: wkFilter, Avail: wkDiffNormal},

	// update_diff.go:209-213 — tab picks the side the cursor, the yank and the
	// search all act on, so it files with the other operand-picking keys. It is
	// a no-op in unified mode, where there is only one column.
	{Key: wkLiteralKey("tab"), Label: "Switch active side", Group: wkSelection, Avail: func(c *wkDiffCtx) bool {
		return !c.visual && !c.m.diffView.unified
	}},

	// Yank: three entries, one key, mutually exclusive predicates.
	// handleDiffNormalCopy consumes a count (update_diff.go:499-515) and
	// diffVisualCopy yanks the selection (update_diff.go:518-537).
	{Key: wkLiteralKey("y"), Label: "Copy line", Group: wkActions, Avail: func(c *wkDiffCtx) bool {
		return !c.visual && !c.counted && c.lineText != ""
	}},
	// A count copies a RANGE, and the handler skips the empty-side lines inside
	// it, so the cursor line being empty does not on its own make the key a
	// no-op — a later line in the range can still carry text. What does make it
	// one is running off the end of the visible list, or the WHOLE range being
	// empty on the active side (countedText).
	{Key: wkLiteralKey("y"), Label: "Copy N lines", Group: wkActions, Avail: func(c *wkDiffCtx) bool {
		return !c.visual && c.counted && c.m.diffView.cursor >= 0 && c.m.diffView.cursor < len(c.vis) && c.countedText
	}},
	{Key: wkLiteralKey("y"), Label: "Copy selection", Group: wkActions, Avail: wkDiffVisual},

	// Visual selection. update_diff.go:184-186 enters it; the same three keys
	// switch or cancel the selection type once inside (diffVisualToggle,
	// update_diff.go:396-401, 486-493).
	{Key: wkLiteralKey("v"), Label: "Visual select", Group: wkSelection, Avail: wkDiffNormal},
	{Key: wkLiteralKey("V"), Label: "Visual line select", Group: wkSelection, Avail: wkDiffNormal},
	{Key: wkLiteralKey("ctrl+v"), Label: "Visual block select", Group: wkSelection, Avail: wkDiffNormal},
	{Key: wkLiteralKey("v"), Label: "Char selection", Group: wkSelection, Avail: wkDiffVisual},
	{Key: wkLiteralKey("V"), Label: "Line selection", Group: wkSelection, Avail: wkDiffVisual},
	{Key: wkLiteralKey("ctrl+v"), Label: "Block selection", Group: wkSelection, Avail: wkDiffVisual},
	// update_diff.go:390-395 — i/a arm a text object that w/W completes
	// (consumeTextObjectPrelude, update_vim.go).
	{Key: wkLiteralKey("i"), Label: "Inner word (iw/iW)", Group: wkSelection, Avail: wkDiffVisual},
	{Key: wkLiteralKey("a"), Label: "Around word (aw/aW)", Group: wkSelection, Avail: wkDiffVisual},

	// Display toggles. update_diff.go:117-119, 189-197. Unified reads its
	// direction: the label names the layout the key switches TO.
	{Key: func(kb ui.Keybindings) string { return kb.ToggleUnified }, Label: "Unified diff", Group: wkSettings, Avail: func(c *wkDiffCtx) bool {
		return !c.visual && !c.m.diffView.unified
	}},
	{Key: func(kb ui.Keybindings) string { return kb.ToggleUnified }, Label: "Side-by-side diff", Group: wkSettings, Avail: func(c *wkDiffCtx) bool {
		return !c.visual && c.m.diffView.unified
	}},
	{Key: func(kb ui.Keybindings) string { return kb.ToggleLineNumbers }, Label: "Toggle line numbers", Group: wkSettings, Avail: wkDiffNormal},
	{Key: func(kb ui.Keybindings) string { return kb.ToggleWrap }, Label: "Toggle line wrapping", Group: wkSettings, Avail: wkDiffNormal},
}

// whichKeyDiffCatalog is the diff viewer's registry entry. Its search prompt
// swallows every printable key (handleDiffSearchInput, update_diff.go:18-20),
// so "?" typed there is part of the query.
var whichKeyDiffCatalog = wkCatalog[*wkDiffCtx]{
	resolve: newWKDiffCtx,
	input:   func(m *Model) bool { return m.diffView.searchMode },
	actions: whichKeyDiffActionList,
}
