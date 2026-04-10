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
		return m.renderHints([]ui.HintEntry{
			{Key: "y", Desc: "confirm"},
			{Key: "n", Desc: "cancel"},
		})
	case overlayQuitConfirm:
		return m.renderHints([]ui.HintEntry{
			{Key: "y", Desc: "quit"},
			{Key: "n", Desc: "cancel"},
		})
	case overlayConfirmType:
		return m.renderHints([]ui.HintEntry{
			{Key: "type DELETE", Desc: "confirm"},
			{Key: "esc", Desc: "cancel"},
		})
	case overlayScaleInput:
		return m.renderHints([]ui.HintEntry{
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
			{Key: "esc", Desc: "cancel"},
		})
	case overlayHelmHistory:
		return m.renderHints([]ui.HintEntry{
			{Key: "jk", Desc: "nav"},
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
			{Key: "c", Desc: "clear"},
			{Key: "enter", Desc: "apply"},
			{Key: "/", Desc: "filter"},
			{Key: "esc", Desc: "close"},
		})
	case overlayAction:
		return m.renderHints([]ui.HintEntry{
			{Key: "j/k", Desc: "navigate"},
			{Key: "enter/key", Desc: "select"},
			{Key: "esc", Desc: "close"},
		})
	case overlayPortForward:
		return m.renderHints([]ui.HintEntry{
			{Key: "j/k", Desc: "select port"},
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
	case overlayLabelEditor:
		return m.overlayHintBarOverlayLabelEditor()
	case overlayColumnToggle:
		return m.overlayHintBarOverlayColumnToggle()
	case overlayFinalizerSearch:
		return m.overlayHintBarOverlayFinalizerSearch()
	case overlayCanI:
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
	case overlayNetworkPolicy:
		return m.renderHints([]ui.HintEntry{
			{Key: "j/k", Desc: "scroll"},
			{Key: "g/G", Desc: "top/bottom"},
			{Key: "ctrl+d/u", Desc: "half page"},
			{Key: "ctrl+f/b", Desc: "page"},
			{Key: "esc", Desc: "close"},
		})
	case overlayQuotaDashboard:
		return m.renderHints([]ui.HintEntry{
			{Key: "esc", Desc: "close"},
		})
	}
	return ""
}

// overlayHintBarBookmarks returns hints for the bookmark overlay sub-modes.
func (m Model) overlayHintBarBookmarks() string {
	switch m.bookmarkSearchMode {
	case bookmarkModeFilter:
		return m.renderHints([]ui.HintEntry{
			{Key: "type", Desc: "filter"},
			{Key: "enter", Desc: "apply"},
			{Key: "esc", Desc: "clear"},
		})
	case bookmarkModeConfirmDelete:
		return m.renderHints([]ui.HintEntry{
			{Key: "y", Desc: "confirm delete"},
			{Key: "n", Desc: "cancel"},
		})
	case bookmarkModeConfirmDeleteAll:
		return m.renderHints([]ui.HintEntry{
			{Key: "y", Desc: "confirm delete all"},
			{Key: "n", Desc: "cancel"},
		})
	default:
		return m.renderHints([]ui.HintEntry{
			{Key: "a-z/0-9", Desc: "jump"},
			{Key: "enter", Desc: "jump"},
			{Key: "/", Desc: "filter"},
			{Key: "ctrl+x", Desc: "delete"},
			{Key: "alt+x", Desc: "delete all"},
			{Key: "esc", Desc: "close"},
		})
	}
}

// renderHints formats hint entries into a styled status bar string.
// It delegates to ui.FormatHintParts, which is the single source of truth
// for hint bar styling.
func (m Model) renderHints(hints []ui.HintEntry) string {
	return ui.FormatHintParts(hints)
}

func (m Model) overlayHintBarOverlayLogContainerSelect() string {
	hints := []ui.HintEntry{
		{Key: "space", Desc: "select"},
		{Key: "enter", Desc: "apply"},
		{Key: "/", Desc: "filter"},
	}
	if m.logParentKind != "" {
		hints = append(hints, ui.HintEntry{Key: "P", Desc: "switch pod"})
	}
	hints = append(hints, ui.HintEntry{Key: "esc", Desc: "close"})
	return m.renderHints(hints)
}

func (m Model) overlayHintBarOverlayTemplates() string {
	if m.templateSearchMode {
		return m.renderHints([]ui.HintEntry{
			{Key: "type", Desc: "filter"},
			{Key: "enter", Desc: "apply"},
			{Key: "esc", Desc: "clear"},
		})
	}
	return m.renderHints([]ui.HintEntry{
		{Key: "enter", Desc: "select"},
		{Key: "/", Desc: "filter"},
		{Key: "esc", Desc: "close"},
	})
}

func (m Model) overlayHintBarOverlayEventTimeline() string {
	if m.eventTimelineSearchActive {
		return m.renderHints([]ui.HintEntry{
			{Key: "type", Desc: "search"},
			{Key: "enter", Desc: "find"},
			{Key: "esc", Desc: "cancel"},
		})
	}
	if m.eventTimelineVisualMode != 0 {
		return m.renderHints([]ui.HintEntry{
			{Key: "j/k", Desc: "extend"},
			{Key: "y", Desc: "copy"},
			{Key: "v/V", Desc: "switch mode"},
			{Key: "esc", Desc: "cancel"},
		})
	}
	return m.renderHints([]ui.HintEntry{
		{Key: "j/k", Desc: "move"},
		{Key: "g/G", Desc: "top/bottom"},
		{Key: "v/V", Desc: "select"},
		{Key: "y", Desc: "copy"},
		{Key: "/", Desc: "search"},
		{Key: "f", Desc: "fullscreen"},
		{Key: ">", Desc: "wrap"},
		{Key: "esc", Desc: "close"},
	})
}

func (m Model) overlayHintBarOverlaySecretEditor() string {
	if m.secretEditing {
		return m.renderHints([]ui.HintEntry{
			{Key: "ctrl+s", Desc: "save"},
			{Key: "enter", Desc: "newline"},
			{Key: "tab", Desc: "switch"},
			{Key: "esc", Desc: "cancel"},
		})
	}
	return m.renderHints([]ui.HintEntry{
		{Key: "jk", Desc: "nav"},
		{Key: "v", Desc: "toggle"},
		{Key: "V", Desc: "all"},
		{Key: "e", Desc: "edit"},
		{Key: "a", Desc: "add"},
		{Key: "y", Desc: "copy"},
		{Key: "D", Desc: "del"},
		{Key: "enter", Desc: "save"},
		{Key: "esc", Desc: "close"},
	})
}

func (m Model) overlayHintBarOverlayConfigMapEditor() string {
	if m.configMapEditing {
		return m.renderHints([]ui.HintEntry{
			{Key: "ctrl+s", Desc: "save"},
			{Key: "enter", Desc: "newline"},
			{Key: "tab", Desc: "switch"},
			{Key: "esc", Desc: "cancel"},
		})
	}
	return m.renderHints([]ui.HintEntry{
		{Key: "jk", Desc: "nav"},
		{Key: "e", Desc: "edit"},
		{Key: "a", Desc: "add"},
		{Key: "y", Desc: "copy"},
		{Key: "D", Desc: "del"},
		{Key: "enter", Desc: "save"},
		{Key: "esc", Desc: "close"},
	})
}

func (m Model) overlayHintBarOverlayLabelEditor() string {
	if m.labelEditing {
		return m.renderHints([]ui.HintEntry{
			{Key: "ctrl+s", Desc: "save"},
			{Key: "tab", Desc: "switch"},
			{Key: "esc", Desc: "cancel"},
		})
	}
	return m.renderHints([]ui.HintEntry{
		{Key: "Tab", Desc: "switch"},
		{Key: "jk", Desc: "nav"},
		{Key: "e", Desc: "edit"},
		{Key: "a", Desc: "add"},
		{Key: "y", Desc: "copy"},
		{Key: "D", Desc: "del"},
		{Key: "enter", Desc: "save"},
		{Key: "esc", Desc: "close"},
	})
}

func (m Model) overlayHintBarOverlayFinalizerSearch() string {
	if m.finalizerSearchFilterActive {
		return m.renderHints([]ui.HintEntry{
			{Key: "type", Desc: "filter"},
			{Key: "enter", Desc: "apply"},
			{Key: "esc", Desc: "clear"},
		})
	}
	return m.renderHints([]ui.HintEntry{
		{Key: "space", Desc: "select"},
		{Key: "ctrl+a", Desc: "all"},
		{Key: "enter", Desc: "remove"},
		{Key: "/", Desc: "filter"},
		{Key: "esc", Desc: "close"},
	})
}

func (m Model) overlayHintBarOverlayCanI() string {
	if m.canISearchActive {
		return m.renderHints([]ui.HintEntry{
			{Key: "type", Desc: "search"},
			{Key: "enter", Desc: "apply"},
			{Key: "esc", Desc: "clear"},
		})
	}
	if m.canISearchQuery != "" {
		return m.renderHints([]ui.HintEntry{
			{Key: "j/k", Desc: "navigate"},
			{Key: "/", Desc: "edit search"},
			{Key: "esc", Desc: "clear search"},
		})
	}
	filterLabel := "all"
	if m.canIAllowedOnly {
		filterLabel = "allowed only"
	}
	return m.renderHints([]ui.HintEntry{
		{Key: "j/k", Desc: "navigate"},
		{Key: "a", Desc: filterLabel},
		{Key: "s", Desc: "switch subject"},
		{Key: "/", Desc: "search groups"},
		{Key: "q/Esc", Desc: "close"},
	})
}

func (m Model) overlayHintBarOverlayColumnToggle() string {
	if m.columnToggleFilterActive {
		return m.renderHints([]ui.HintEntry{
			{Key: "type", Desc: "filter"},
			{Key: "esc", Desc: "clear/close"},
		})
	}
	return m.renderHints([]ui.HintEntry{
		{Key: "space", Desc: "toggle"},
		{Key: "J/K", Desc: "reorder"},
		{Key: "enter", Desc: "apply"},
		{Key: "c", Desc: "clear"},
		{Key: "R", Desc: "reset"},
		{Key: "/", Desc: "filter"},
		{Key: "esc", Desc: "close"},
	})
}
