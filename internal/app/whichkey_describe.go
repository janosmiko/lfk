package app

import (
	"strings"

	"github.com/janosmiko/lfk/internal/ui"
)

// wkDescribeCtx is the describe viewer's resolved which-key context.
//
// visual is read by nearly every entry: handleDescribeKey routes to
// handleDescribeVisualKey before handleDescribeNormalKey
// (update_describe.go:17-22), and the visual handler has no case for the wrap
// toggle, the search key, or q — pressing them there is a silent no-op.
type wkDescribeCtx struct {
	m      *Model
	visual bool
	// counted reports an armed digit prefix, which the yank consumes so `y`
	// copies N lines (update_describe.go:131-139).
	counted bool
	// onLine is the cursor guard the yank applies (update_describe.go:133-135).
	// The describe viewer has no fold mapping, so the bound is the raw line
	// count of the content.
	onLine bool
}

func newWKDescribeCtx(m *Model) *wkDescribeCtx {
	lines := strings.Count(m.describeView.content, "\n") + 1
	return &wkDescribeCtx{
		m:       m,
		visual:  m.describeView.visualMode != 0,
		counted: m.describeView.lineInput != "",
		onLine:  m.describeView.cursor >= 0 && m.describeView.cursor < lines,
	}
}

func wkDescribeNormal(c *wkDescribeCtx) bool { return !c.visual }
func wkDescribeVisual(c *wkDescribeCtx) bool { return c.visual }

// whichKeyDescribeActionList is the describe viewer's catalog. It is the
// smallest of the three: handleDescribeNormalKey has no refresh, fold, or
// cross-view keys — everything else in its switch is a motion, n/N, or ctrl+c,
// all excluded on the same rules the explorer catalog uses. esc is excluded
// because whichKeyLeaderIntercept consumes it while the panel is shown
// (whichkey_leader.go:169-181).
var whichKeyDescribeActionList = []wkAction[*wkDescribeCtx]{
	// update_describe.go:35-43.
	{Key: whichKeyHelpKey, Label: "Full help", Group: wkViews, Avail: wkDescribeNormal},
	// update_describe.go:48-49 -> handleDescribeQuit (167-184): one key, two
	// outcomes, decided by whether a search is applied.
	{Key: wkLiteralKey("q"), Label: "Clear search", Group: wkFilter, Avail: func(c *wkDescribeCtx) bool {
		return !c.visual && c.m.describeView.searchQuery != ""
	}},
	{Key: wkLiteralKey("q"), Label: "Back", Group: wkViews, Avail: func(c *wkDescribeCtx) bool {
		return !c.visual && c.m.describeView.searchQuery == ""
	}},
	{Key: func(kb ui.Keybindings) string { return kb.Search }, Label: "Search in content", Group: wkFilter, Avail: wkDescribeNormal},
	{Key: func(kb ui.Keybindings) string { return kb.ToggleWrap }, Label: "Toggle line wrapping", Group: wkSettings, Avail: wkDescribeNormal},

	// Yank: three entries, one key (update_describe.go:131-139 counted,
	// describeVisualCopy at 364-381 for the selection).
	{Key: wkLiteralKey("y"), Label: "Copy line", Group: wkActions, Avail: func(c *wkDescribeCtx) bool {
		return !c.visual && !c.counted && c.onLine
	}},
	{Key: wkLiteralKey("y"), Label: "Copy N lines", Group: wkActions, Avail: func(c *wkDescribeCtx) bool {
		return !c.visual && c.counted && c.onLine
	}},
	{Key: wkLiteralKey("y"), Label: "Copy selection", Group: wkActions, Avail: wkDescribeVisual},

	// Visual selection: update_describe.go:125-130 enters it, 278-283
	// switches or cancels the type once inside (describeVisualToggle).
	{Key: wkLiteralKey("v"), Label: "Visual select", Group: wkSelection, Avail: wkDescribeNormal},
	{Key: wkLiteralKey("V"), Label: "Visual line select", Group: wkSelection, Avail: wkDescribeNormal},
	{Key: wkLiteralKey("ctrl+v"), Label: "Visual block select", Group: wkSelection, Avail: wkDescribeNormal},
	{Key: wkLiteralKey("v"), Label: "Char selection", Group: wkSelection, Avail: wkDescribeVisual},
	{Key: wkLiteralKey("V"), Label: "Line selection", Group: wkSelection, Avail: wkDescribeVisual},
	{Key: wkLiteralKey("ctrl+v"), Label: "Block selection", Group: wkSelection, Avail: wkDescribeVisual},
	// update_describe.go:272-277 — i/a arm a text object w/W completes.
	{Key: wkLiteralKey("i"), Label: "Inner word (iw/iW)", Group: wkSelection, Avail: wkDescribeVisual},
	{Key: wkLiteralKey("a"), Label: "Around word (aw/aW)", Group: wkSelection, Avail: wkDescribeVisual},
}

// whichKeyDescribeCatalog is the describe viewer's registry entry. Its search
// prompt swallows printable keys (update_describe.go:12-15).
var whichKeyDescribeCatalog = wkCatalog[*wkDescribeCtx]{
	resolve: newWKDescribeCtx,
	input:   func(m *Model) bool { return m.describeView.searchActive },
	actions: whichKeyDescribeActionList,
}
