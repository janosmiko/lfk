package app

import (
	"github.com/janosmiko/lfk/internal/ui"
)

// wkEventViewerCtx is the fullscreen event viewer's resolved which-key context.
//
// visual gates nearly every entry: handleEventTimelineOverlayKey routes to
// handleEventTimelineVisualKey before its own switch
// (update_overlays_events.go:155-158), and the visual handler has no case for
// the wrap toggle, the search key or help — pressing them there is a silent
// no-op. The two exceptions are handled one level up, in
// handleEventViewerModeKey (update_overlays_events.go:118-142), which claims
// q/esc and kb.Fullscreen for the fullscreen mode BEFORE the visual routing
// ever runs.
type wkEventViewerCtx struct {
	m      *Model
	visual bool
	// counted reports an armed digit prefix, which the yank consumes so `y`
	// copies N lines (update_overlays_events.go:709-717).
	counted bool
	// onLine is that yank's cursor guard (update_overlays_events.go:711-713).
	onLine bool
}

func newWKEventViewerCtx(m *Model) *wkEventViewerCtx {
	return &wkEventViewerCtx{
		m:       m,
		visual:  m.eventTimelineVisualMode != 0,
		counted: m.eventTimelineLineInput != "",
		onLine:  m.eventTimelineCursor >= 0 && m.eventTimelineCursor < len(m.eventTimelineLines),
	}
}

func wkEventViewerNormal(c *wkEventViewerCtx) bool { return !c.visual }
func wkEventViewerVisual(c *wkEventViewerCtx) bool { return c.visual }

// whichKeyEventViewerActionList is the fullscreen event viewer's catalog.
// Motions and n/N are excluded on the explorer's rule; ctrl+c is excluded
// because it closes the tab or quits.
//
// esc is absent because whichKeyLeaderIntercept consumes it while the panel is
// shown (whichkey_leader.go:169-181). It is worth naming what that hides here:
// esc and q are the SAME case in the handler except for one branch — an applied
// search is cleared by esc only (update_overlays_events.go:128-131), so q with
// a search applied still leaves the viewer. That is why the q entry below reads
// "Back to explorer" in every state rather than splitting on the search the way
// the YAML and describe catalogs do.
var whichKeyEventViewerActionList = []wkAction[*wkEventViewerCtx]{
	// update_overlays_events.go:168-169 — kb.Help and the f1 alias.
	{Key: whichKeyHelpKey, Label: "Full help", Group: wkViews, Avail: wkEventViewerNormal},
	// update_overlays_events.go:119-135: in visual mode q cancels the
	// selection, otherwise it leaves fullscreen for the explorer.
	{Key: wkLiteralKey("q"), Label: "Back to explorer", Group: wkViews, Avail: wkEventViewerNormal},
	{Key: wkLiteralKey("q"), Label: "Cancel selection", Group: wkSelection, Avail: wkEventViewerVisual},
	// update_overlays_events.go:136-142 — the only key that survives visual
	// mode, because the fullscreen handler claims it before the visual routing.
	{Key: func(kb ui.Keybindings) string { return kb.Fullscreen }, Label: "Minimize to overlay", Group: wkViews},

	// update_overlays_events.go:182-183.
	{Key: func(kb ui.Keybindings) string { return kb.Search }, Label: "Search in content", Group: wkFilter, Avail: wkEventViewerNormal},

	// Yank: three entries, one key (update_overlays_events.go:709-717 counted,
	// handleEventTimelineVisualKeyY at 485-503 for the selection).
	{Key: wkLiteralKey("y"), Label: "Copy line", Group: wkActions, Avail: func(c *wkEventViewerCtx) bool {
		return !c.visual && !c.counted && c.onLine
	}},
	{Key: wkLiteralKey("y"), Label: "Copy N lines", Group: wkActions, Avail: func(c *wkEventViewerCtx) bool {
		return !c.visual && c.counted && c.onLine
	}},
	{Key: wkLiteralKey("y"), Label: "Copy selection", Group: wkActions, Avail: wkEventViewerVisual},

	// Visual selection: update_overlays_events.go:174-179 enters it, 311-316
	// switches or cancels the type once inside.
	{Key: wkLiteralKey("v"), Label: "Visual select", Group: wkSelection, Avail: wkEventViewerNormal},
	{Key: wkLiteralKey("V"), Label: "Visual line select", Group: wkSelection, Avail: wkEventViewerNormal},
	{Key: wkLiteralKey("ctrl+v"), Label: "Visual block select", Group: wkSelection, Avail: wkEventViewerNormal},
	{Key: wkLiteralKey("v"), Label: "Char selection", Group: wkSelection, Avail: wkEventViewerVisual},
	{Key: wkLiteralKey("V"), Label: "Line selection", Group: wkSelection, Avail: wkEventViewerVisual},
	{Key: wkLiteralKey("ctrl+v"), Label: "Block selection", Group: wkSelection, Avail: wkEventViewerVisual},
	// update_overlays_events.go:305-310 — i/a arm a text object w/W completes.
	{Key: wkLiteralKey("i"), Label: "Inner word (iw/iW)", Group: wkSelection, Avail: wkEventViewerVisual},
	{Key: wkLiteralKey("a"), Label: "Around word (aw/aW)", Group: wkSelection, Avail: wkEventViewerVisual},

	// update_overlays_events.go:196-198.
	{Key: func(kb ui.Keybindings) string { return kb.ToggleWrap }, Label: "Toggle line wrapping", Group: wkSettings, Avail: wkEventViewerNormal},
}

// whichKeyEventViewerCatalog is the fullscreen event viewer's registry entry.
// Its search prompt claims printable keys ahead of everything else
// (update_overlays_events.go:150-153), so "?" typed there is part of the query.
//
// Registered for modeEventViewer only. The same timeline also renders as an
// overlay (overlayEventTimeline), where handleKey returns at the overlay branch
// before the leader dispatch (update_keys.go:35-37) — the panel cannot reach
// it, and pretending otherwise would arm a leader nothing would draw.
var whichKeyEventViewerCatalog = wkCatalog[*wkEventViewerCtx]{
	resolve: newWKEventViewerCtx,
	input:   func(m *Model) bool { return m.eventTimelineSearchActive },
	actions: whichKeyEventViewerActionList,
}
