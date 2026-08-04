package app

import (
	"github.com/janosmiko/lfk/internal/ui"
)

// wkLogCtx is the log viewer's resolved which-key context.
//
// visual is read by nearly every entry: handleLogKey routes to
// handleLogVisualKey before the movement/action dispatchers
// (update_logs.go:20-22), and the visual handler has no case for the toggles,
// the filter, the severity step, or the save keys — pressing them there is a
// silent no-op. The remaining log state (follow, previous, sevThreshold,
// parentKind) is a plain field read, so it stays on m rather than being
// copied in here.
type wkLogCtx struct {
	m      *Model
	visual bool
	// counted reports an armed digit prefix, which handleLogNormalCopy
	// consumes so `y` yanks N lines (update_logs_normal.go:555-567).
	counted bool
	// onLine is the cursor guard handleLogNormalCopy applies before yanking
	// (update_logs_normal.go:557-559); logView.cursor is -1 while inactive.
	onLine bool
}

func newWKLogCtx(m *Model) *wkLogCtx {
	return &wkLogCtx{
		m:       m,
		visual:  m.logView.visualMode,
		counted: m.logView.lineInput != "",
		onLine:  m.logView.cursor >= 0 && m.logView.cursor < len(m.logView.lines),
	}
}

func wkLogNormal(c *wkLogCtx) bool { return !c.visual }
func wkLogVisual(c *wkLogCtx) bool { return c.visual }

// wkLogSwitchPodAvailable and wkLogFilterContainersAvailable are the two
// branches handleLogKeyOther takes (update_logs_normal.go:502-538): a group
// resource opens the pod selector, a single Pod opens the container filter,
// and anything else returns unchanged. They are mutually exclusive because the
// handler tests parentKind first.
func wkLogSwitchPodAvailable(c *wkLogCtx) bool {
	return !c.visual && c.m.logView.parentKind != ""
}

func wkLogFilterContainersAvailable(c *wkLogCtx) bool {
	return !c.visual && c.m.logView.parentKind == "" && c.m.actionCtx.kind == "Pod"
}

// whichKeyLogActionList is the log viewer's catalog. Motions and n/N are
// absent for the same reason they are in the explorer's; so are J/K, which
// scroll the structured preview pane (update_logs.go:159-170) and are
// navigation by the same rule that excludes kb.PreviewDown / kb.PreviewUp.
//
// ctrl+c is absent because it closes the tab or quits, which the explorer
// catalog does not advertise either. esc is absent for a reason specific to
// the panel: whichKeyLeaderIntercept CONSUMES esc while the panel is shown
// (whichkey_leader.go:169-181), so the first esc closes the panel rather than
// reaching the viewer — advertising it would be a lie.
var whichKeyLogActionList = []wkAction[*wkLogCtx]{
	// update_logs.go:113-114.
	{Key: whichKeyHelpKey, Label: "Full help", Group: wkViews, Avail: wkLogNormal},
	// update_logs.go:116-118 vs 247-249: q closes the viewer in normal mode
	// and cancels the selection in visual mode.
	{Key: wkLiteralKey("q"), Label: "Close log viewer", Group: wkViews, Avail: wkLogNormal},
	{Key: wkLiteralKey("q"), Label: "Cancel selection", Group: wkSelection, Avail: wkLogVisual},
	{Key: func(kb ui.Keybindings) string { return kb.LogTop }, Label: "Log Top aggregation", Group: wkViews, Avail: wkLogNormal},
	{Key: func(kb ui.Keybindings) string { return kb.TogglePreview }, Label: "Structured preview panel", Group: wkViews, Avail: wkLogNormal},

	// update_logs.go:131-133 / 144-145 — filter and search.
	{Key: func(kb ui.Keybindings) string { return kb.Filter }, Label: "Filter log lines", Group: wkFilter, Avail: wkLogNormal},
	{Key: func(kb ui.Keybindings) string { return kb.Search }, Label: "Search in content", Group: wkFilter, Avail: wkLogNormal},
	// severityStep clamps to [0, ui.LogError] (logfilter.go:108-112), so at
	// either end the key redraws the same view and changes nothing.
	{Key: func(kb ui.Keybindings) string { return kb.SeverityUp }, Label: "Raise min severity", Group: wkFilter, Avail: func(c *wkLogCtx) bool {
		return !c.visual && c.m.logView.sevThreshold < ui.LogError
	}},
	{Key: func(kb ui.Keybindings) string { return kb.SeverityDown }, Label: "Lower min severity", Group: wkFilter, Avail: func(c *wkLogCtx) bool {
		return !c.visual && c.m.logView.sevThreshold > 0
	}},
	// update_logs_normal.go:502-538 — one key, two different overlays.
	{Key: wkLiteralKey("\\"), Label: "Switch pod", Group: wkFilter, Avail: wkLogSwitchPodAvailable},
	{Key: wkLiteralKey("\\"), Label: "Filter containers", Group: wkFilter, Avail: wkLogFilterContainersAvailable},

	// Saving. Both are unconditional: saveLoadedLogs writes whatever is
	// buffered (commands_logs.go:717-721) and saveAllLogs re-runs kubectl.
	{Key: wkLiteralKey("S"), Label: "Save loaded logs", Group: wkActions, Avail: wkLogNormal},
	{Key: wkLiteralKey("ctrl+s"), Label: "Save all logs", Group: wkActions, Avail: wkLogNormal},

	// Yank: three entries, one key. handleLogNormalCopy consumes a count
	// (update_logs_normal.go:555-567); handleLogVisualKeyY yanks the selection
	// (update_logs_visual.go:34-39).
	{Key: wkLiteralKey("y"), Label: "Copy line", Group: wkActions, Avail: func(c *wkLogCtx) bool {
		return !c.visual && !c.counted && c.onLine
	}},
	{Key: wkLiteralKey("y"), Label: "Copy N lines", Group: wkActions, Avail: func(c *wkLogCtx) bool {
		return !c.visual && c.counted && c.onLine
	}},
	{Key: wkLiteralKey("y"), Label: "Copy selection", Group: wkActions, Avail: wkLogVisual},

	// Visual selection: update_logs.go:119-127 enters it, 214-222 switches or
	// cancels the type once inside.
	{Key: wkLiteralKey("v"), Label: "Visual select", Group: wkSelection, Avail: wkLogNormal},
	{Key: wkLiteralKey("V"), Label: "Visual line select", Group: wkSelection, Avail: wkLogNormal},
	{Key: wkLiteralKey("ctrl+v"), Label: "Visual block select", Group: wkSelection, Avail: wkLogNormal},
	{Key: wkLiteralKey("v"), Label: "Char selection", Group: wkSelection, Avail: wkLogVisual},
	{Key: wkLiteralKey("V"), Label: "Line selection", Group: wkSelection, Avail: wkLogVisual},
	{Key: wkLiteralKey("ctrl+v"), Label: "Block selection", Group: wkSelection, Avail: wkLogVisual},
	// update_logs.go:208-213 — i/a arm a text object w/W completes. In normal
	// mode the same two keys are the severity step and an unbound key, which
	// is why every entry on this pair carries the visual gate.
	{Key: wkLiteralKey("i"), Label: "Inner word (iw/iW)", Group: wkSelection, Avail: wkLogVisual},
	{Key: wkLiteralKey("a"), Label: "Around word (aw/aW)", Group: wkSelection, Avail: wkLogVisual},

	// Display toggles. handleLogKeyF also jumps to the bottom when it turns
	// following ON (update_logs_normal.go:239-247), so the two directions get
	// their own labels rather than one "toggle".
	{Key: func(kb ui.Keybindings) string { return kb.ToggleFollow }, Label: "Follow new lines", Group: wkSettings, Avail: func(c *wkLogCtx) bool {
		return !c.visual && !c.m.logView.follow
	}},
	{Key: func(kb ui.Keybindings) string { return kb.ToggleFollow }, Label: "Stop following", Group: wkSettings, Avail: func(c *wkLogCtx) bool {
		return !c.visual && c.m.logView.follow
	}},
	{Key: func(kb ui.Keybindings) string { return kb.ToggleWrap }, Label: "Toggle line wrapping", Group: wkSettings, Avail: wkLogNormal},
	{Key: func(kb ui.Keybindings) string { return kb.ToggleLineNumbers }, Label: "Toggle line numbers", Group: wkSettings, Avail: wkLogNormal},
	{Key: func(kb ui.Keybindings) string { return kb.ToggleTimestamps }, Label: "Toggle timestamps", Group: wkSettings, Avail: wkLogNormal},
	{Key: func(kb ui.Keybindings) string { return kb.TogglePrefixes }, Label: "Toggle pod prefixes", Group: wkSettings, Avail: wkLogNormal},
	// handleLogKeyC restarts the stream with or without --previous
	// (update_logs_normal.go:463-491).
	{Key: wkLiteralKey("c"), Label: "Previous container logs", Group: wkSettings, Avail: func(c *wkLogCtx) bool {
		return !c.visual && !c.m.logView.previous
	}},
	{Key: wkLiteralKey("c"), Label: "Current container logs", Group: wkSettings, Avail: func(c *wkLogCtx) bool {
		return !c.visual && c.m.logView.previous
	}},
}

// whichKeyLogCatalog is the log viewer's registry entry. Both the search and
// the live filter prompts swallow printable keys
// (update_logs.go:11-17), so "?" typed there is part of the query.
var whichKeyLogCatalog = wkCatalog[*wkLogCtx]{
	resolve: newWKLogCtx,
	input:   func(m *Model) bool { return m.logView.searchActive || m.logView.filterActive },
	actions: whichKeyLogActionList,
}
