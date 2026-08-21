package app

import (
	"github.com/janosmiko/lfk/internal/ui"
)

// overlayHintBar returns the hint bar content for the currently active overlay.
// Returns empty string when no overlay is active.
func (m Model) overlayHintBar() string {
	if hints := m.overlayHintBarDialog(); hints != "" {
		return hints
	}
	if hints := m.overlayHintBarSelector(); hints != "" {
		return hints
	}
	if hints := m.overlayHintBarEditor(); hints != "" {
		return hints
	}
	if hints := m.overlayHintBarMisc(); hints != "" {
		return hints
	}
	return ""
}

// overlayHintBarDialog handles confirmation and input dialog overlays.
func (m Model) overlayHintBarDialog() string {
	switch m.overlay {
	case overlayConfirm:
		return m.renderHints(cascadeConfirmHints("Enter/y", "Esc/n", m.deleteConfirmShowsPolicy()))
	case overlayQuitConfirm:
		return m.renderHints([]ui.HintEntry{
			{Key: "Enter/y", Desc: "quit"},
			{Key: "Esc/n", Desc: "cancel"},
		})
	case overlayPasteConfirm:
		return m.renderHints([]ui.HintEntry{
			{Key: "Enter/y", Desc: "paste"},
			{Key: "Esc/n", Desc: "cancel"},
		})
	case overlayConfirmType:
		return m.renderHints(cascadeConfirmHints("type DELETE", "esc", m.forceDeleteConfirmShowsPolicy()))
	case overlayScaleInput:
		return m.renderHints([]ui.HintEntry{
			{Key: "h/l -/+", Desc: "−/＋"},
			{Key: "Enter", Desc: "apply"},
			{Key: "esc", Desc: "cancel"},
		})
	case overlayHPAScale:
		return m.renderHints([]ui.HintEntry{
			{Key: "j/k", Desc: "field"},
			{Key: "h/l -/+", Desc: "−/＋"},
			{Key: "Enter", Desc: "apply"},
			{Key: "esc", Desc: "cancel"},
		})
	case overlayPVCResize:
		return m.renderHints([]ui.HintEntry{
			{Key: "Enter", Desc: "resize"},
			{Key: "esc", Desc: "cancel"},
		})
	case overlayBatchLabel:
		return m.renderHints([]ui.HintEntry{
			{Key: "Tab", Desc: "toggle add/remove"},
			{Key: "Enter", Desc: "apply"},
			{Key: "esc", Desc: "cancel"},
		})
	case overlayCrashInvestigator:
		return m.renderHints([]ui.HintEntry{
			{Key: "Tab", Desc: "switch tab"},
			{Key: "1-4", Desc: "jump"},
			{Key: "c", Desc: "container"},
			{Key: "p", Desc: "prev/curr"},
			{Key: "j/k", Desc: "scroll"},
			{Key: "C-f/C-b", Desc: "page"},
			{Key: "R", Desc: "refresh"},
			{Key: "esc", Desc: "close"},
		})
	case overlaySyncWave:
		hints := []ui.HintEntry{{Key: "R", Desc: "refresh"}}
		// Single-pane mode (m.width < 64) hides the sidebar so Tab has
		// no effect — omit the hint to match the actual keymap.
		if m.width >= 64 {
			hints = append(hints, ui.HintEntry{Key: "Tab", Desc: "toggle pane"})
		}
		hints = append(hints,
			ui.HintEntry{Key: "Enter", Desc: "collapse"},
			ui.HintEntry{Key: "j/k", Desc: "scroll"},
			ui.HintEntry{Key: "q", Desc: "close"},
		)
		return m.renderHints(hints)
	case overlayRBAC, overlayPodStartup:
		return m.renderHints([]ui.HintEntry{
			{Key: "any key", Desc: "close"},
		})
	case overlayAutoSync:
		return m.renderHints([]ui.HintEntry{
			{Key: "jk", Desc: "nav"},
			{Key: "space", Desc: "toggle"},
			{Key: "enter", Desc: "save"},
			{Key: "esc", Desc: "cancel"},
		})
	case overlayRollback, overlayHelmRollback:
		return m.renderHints([]ui.HintEntry{
			{Key: "jk", Desc: "nav"},
			{Key: "Enter", Desc: "rollback"},
			{Key: "y", Desc: "copy"},
			{Key: "esc", Desc: "cancel"},
		})
	case overlayHelmHistory:
		return m.renderHints([]ui.HintEntry{
			{Key: "jk", Desc: "nav"},
			{Key: "y", Desc: "copy"},
			{Key: "esc", Desc: "close"},
		})
	}
	return ""
}

// overlayHintBarSelector handles list/selector overlays.
func (m Model) overlayHintBarSelector() string {
	switch m.overlay {
	case overlayNamespace:
		return m.renderHints([]ui.HintEntry{
			{Key: "space", Desc: "select"},
			{Key: "tab", Desc: "exclude"},
			{Key: "A", Desc: "all"},
			{Key: ".", Desc: "filter to item's ns"},
			{Key: "enter", Desc: "apply"},
			{Key: "/", Desc: "filter"},
			{Key: "R", Desc: "refresh"},
			{Key: "esc", Desc: "close"},
		})
	case overlayAction:
		return m.renderHints([]ui.HintEntry{
			{Key: "j/k", Desc: "navigate"},
			{Key: "enter/key", Desc: "select"},
			{Key: "esc", Desc: "close"},
		})
	case overlayCopyFormat:
		return m.renderHints(m.copyFormatPickerHints())
	case overlayExportTemplate:
		return m.renderHints(exportTemplateHints())
	case overlayExportStrip:
		return m.renderHints(exportStripHints())
	case overlayCopyField:
		return m.renderHints(copyFieldPickerHints())
	case overlayTaintEditor:
		return m.renderHints(m.taintEditorHints())
	case overlayTaintPresets:
		return m.renderHints(taintPresetHints())
	case overlayPortForward:
		return m.renderHints([]ui.HintEntry{
			{Key: "j/k", Desc: "select port"},
			{Key: "0-9", Desc: "local[:remote] port (eg 8080:80)"},
			{Key: "enter", Desc: "forward"},
			{Key: "esc", Desc: "cancel"},
		})
	case overlayContainerSelect:
		return m.renderHints([]ui.HintEntry{
			{Key: "j/k", Desc: "navigate"},
			{Key: "enter", Desc: "select"},
			{Key: "esc", Desc: "close"},
		})
	case overlayPodSelect, overlayLogPodSelect:
		return m.renderHints([]ui.HintEntry{
			{Key: "/", Desc: "filter"},
			{Key: "j/k", Desc: "navigate"},
			{Key: "enter", Desc: "select"},
			{Key: "esc", Desc: "close"},
		})
	case overlayLogContainerSelect:
		return m.overlayHintBarOverlayLogContainerSelect()
	case overlayBookmarks:
		return m.overlayHintBarBookmarks()
	case overlaySessions:
		return m.overlayHintBarSessions()
	case overlayColorscheme:
		return m.renderHints([]ui.HintEntry{
			{Key: "j/k", Desc: "navigate"},
			{Key: "g/G", Desc: "top/bottom"},
			{Key: "enter", Desc: "apply"},
			{Key: "t", Desc: "transparent bg"},
			{Key: "/", Desc: "filter"},
			{Key: "esc", Desc: "cancel"},
		})
	case overlayFilterPreset:
		return m.renderHints([]ui.HintEntry{
			{Key: "key", Desc: "apply"},
			{Key: "enter", Desc: "apply"},
			{Key: ".", Desc: "clear"},
			{Key: "esc", Desc: "close"},
		})
	case overlayTemplates:
		return m.overlayHintBarOverlayTemplates()
	case overlayCanISubject:
		return m.renderHints([]ui.HintEntry{
			{Key: "enter", Desc: "select"},
			{Key: "/", Desc: "filter"},
			{Key: "esc", Desc: "cancel"},
		})
	case overlayExplainSearch:
		return m.renderHints([]ui.HintEntry{
			{Key: "enter", Desc: "navigate"},
			{Key: "/", Desc: "filter"},
			{Key: "esc", Desc: "close"},
		})
	case overlayObjectExplorerFind:
		return m.renderHints([]ui.HintEntry{
			{Key: "j/k", Desc: "navigate"},
			{Key: "enter", Desc: "jump"},
			{Key: "/", Desc: "filter"},
			{Key: "esc", Desc: "close"},
		})
	case overlayLogTopGroupBy, overlayLogTopProfile, overlayLogTopColumns:
		return m.overlayHintBarLogTop()
	case overlayClusterColor:
		if m.clusterColorFilterMode {
			return m.renderHints([]ui.HintEntry{
				{Key: "enter", Desc: "accept filter"},
				{Key: "esc", Desc: "clear filter"},
			})
		}
		return m.renderHints([]ui.HintEntry{
			{Key: "j/k", Desc: "navigate"},
			{Key: "/", Desc: "filter"},
			{Key: "enter", Desc: "apply"},
			{Key: "esc", Desc: "close"},
		})
	}
	return ""
}

// overlayHintBarEditor handles editor and viewer overlays.
func (m Model) overlayHintBarEditor() string {
	switch m.overlay {
	case overlaySecretEditor:
		return m.overlayHintBarOverlaySecretEditor()
	case overlayConfigMapEditor:
		return m.overlayHintBarOverlayConfigMapEditor()
	case overlayRightsizing:
		return m.overlayHintBarOverlayRightsizing()
	case overlayLabelEditor:
		return m.overlayHintBarOverlayLabelEditor()
	case overlayColumnToggle:
		return m.overlayHintBarOverlayColumnToggle()
	case overlayFinalizerSearch:
		return m.overlayHintBarOverlayFinalizerSearch()
	case overlayCanI:
		if m.canIMode == canIModeWhoCan {
			if m.whoCan.resourceFilterActive {
				return m.renderHints([]ui.HintEntry{
					{Key: "type", Desc: "narrow list"},
					{Key: "enter", Desc: "accept"},
					{Key: "esc", Desc: "clear"},
				})
			}
			return m.renderHints([]ui.HintEntry{
				{Key: "j/k", Desc: "pick resource"},
				{Key: "J/K", Desc: "scroll subjects"},
				{Key: "g/G", Desc: "top/bottom"},
				{Key: "ctrl+d/u", Desc: "half page"},
				{Key: "ctrl+f/b", Desc: "page"},
				{Key: "←/→", Desc: "verb"},
				{Key: "/", Desc: "filter"},
				{Key: "A", Desc: "ns scope"},
				{Key: "Tab", Desc: "back"},
				{Key: "esc", Desc: "close"},
			})
		}
		return m.overlayHintBarOverlayCanI()
	}
	return ""
}

// overlayHintBarMisc handles remaining overlay types.
func (m Model) overlayHintBarMisc() string {
	switch m.overlay {
	case overlayEventTimeline:
		return m.overlayHintBarOverlayEventTimeline()
	case overlayAlerts:
		return m.renderHints([]ui.HintEntry{
			{Key: "j/k", Desc: "scroll"},
			{Key: "esc", Desc: "close"},
		})
	case overlayBackgroundTasks:
		tabDesc := "history"
		if m.tasksOverlayShowCompleted {
			tabDesc = "running"
		}
		hints := []ui.HintEntry{
			{Key: "tab", Desc: tabDesc},
			{Key: "j/k", Desc: "scroll"},
			{Key: "g/G", Desc: "top/bottom"},
		}
		// `a` is only meaningful in the completed history view.
		if m.tasksOverlayShowCompleted {
			aDesc := "show all"
			if m.tasksOverlayShowAll {
				aDesc = "hide sub-second"
			}
			hints = append(hints, ui.HintEntry{Key: "a", Desc: aDesc})
		}
		hints = append(hints, ui.HintEntry{Key: "esc", Desc: "close"})
		return m.renderHints(hints)
	case overlayNetworkPolicy:
		if m.netpolSearchActive {
			return ui.HelpKeyStyle.Render(ui.ActiveKeybindings.Search) +
				ui.BarNormalStyle.Render(m.netpolSearchInput.CursorLeft()) +
				ui.BarDimStyle.Render("█") +
				ui.BarNormalStyle.Render(m.netpolSearchInput.CursorRight())
		}
		return m.renderHints([]ui.HintEntry{
			{Key: "j/k", Desc: "scroll"},
			{Key: "g/G", Desc: "top/bottom"},
			{Key: "ctrl+d/u", Desc: "half page"},
			{Key: "ctrl+f/b", Desc: "page"},
			{Key: "/", Desc: "search"},
			{Key: "n/N", Desc: "next/prev match"},
			{Key: "esc", Desc: "close"},
		})
	case overlayQuotaDashboard:
		return m.renderHints([]ui.HintEntry{
			{Key: "esc", Desc: "close"},
		})
	case overlayLocalClusters:
		return m.overlayHintBarOverlayLocalClusters()
	case overlayTrafficCapture:
		return m.overlayHintBarOverlayTrafficCapture()
	case overlayOrphans:
		// Filter input mode hides most navigation hints — show only
		// the keys that actually do something while typing.
		if m.orphans.filterActive {
			return m.renderHints([]ui.HintEntry{
				{Key: "type", Desc: "filter"},
				{Key: "enter", Desc: "apply"},
				{Key: "esc", Desc: "clear"},
			})
		}
		modeDesc := "lenient"
		if m.orphans.strict {
			modeDesc = "strict"
		}
		return m.renderHints([]ui.HintEntry{
			{Key: "j/k", Desc: "move"},
			{Key: "g/G", Desc: "top/bottom"},
			{Key: "ctrl+d/u", Desc: "half page"},
			{Key: "tab", Desc: "kind"},
			{Key: "/", Desc: "filter"},
			{Key: "enter", Desc: "jump"},
			{Key: "s", Desc: modeDesc},
			{Key: "R", Desc: "refresh"},
			{Key: "q/esc", Desc: "close"},
		})
	}
	return ""
}

// renderHints formats hint entries into a styled status bar string.
// It delegates to ui.FormatHintParts, which is the single source of truth
// for hint bar styling.
func (m Model) renderHints(hints []ui.HintEntry) string {
	return ui.FormatHintParts(hints)
}
