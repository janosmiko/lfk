package app

import (
	"strings"

	"github.com/janosmiko/lfk/internal/ui"
)

// overlayHintBar returns the hint bar content for the currently active overlay.
// Returns empty string when no overlay is active.
func (m Model) overlayHintBar() string {
	switch m.overlay {
	case overlayNone:
		return ""

	case overlayNamespace:
		return m.renderHints([]hintEntry{
			{"space", "select"},
			{"c", "clear"},
			{"enter", "apply"},
			{"/", "filter"},
			{"esc", "close"},
		})

	case overlayAction:
		return m.renderHints([]hintEntry{
			{"j/k", "navigate"},
			{"enter/key", "select"},
			{"esc", "close"},
		})

	case overlayConfirm:
		return m.renderHints([]hintEntry{
			{"y", "confirm"},
			{"n", "cancel"},
		})

	case overlayQuitConfirm:
		return m.renderHints([]hintEntry{
			{"y", "quit"},
			{"n", "cancel"},
		})

	case overlayConfirmType:
		return m.renderHints([]hintEntry{
			{"type DELETE", "confirm"},
			{"esc", "cancel"},
		})

	case overlayScaleInput:
		return m.renderHints([]hintEntry{
			{"Enter", "apply"},
			{"esc", "cancel"},
		})

	case overlayPortForward:
		return m.renderHints([]hintEntry{
			{"j/k", "select port"},
			{"enter", "forward"},
			{"esc", "cancel"},
		})

	case overlayContainerSelect:
		return m.renderHints([]hintEntry{
			{"j/k", "navigate"},
			{"enter", "select"},
			{"esc", "close"},
		})

	case overlayPodSelect, overlayLogPodSelect:
		return m.renderHints([]hintEntry{
			{"/", "filter"},
			{"j/k", "navigate"},
			{"enter", "select"},
			{"esc", "close"},
		})

	case overlayLogContainerSelect:
		hints := []hintEntry{
			{"space", "select"},
			{"enter", "apply"},
			{"/", "filter"},
		}
		if m.logParentKind != "" {
			hints = append(hints, hintEntry{"P", "switch pod"})
		}
		hints = append(hints, hintEntry{"esc", "close"})
		return m.renderHints(hints)

	case overlayBookmarks:
		switch m.bookmarkSearchMode {
		case bookmarkModeFilter:
			return m.renderHints([]hintEntry{
				{"type", "filter"},
				{"enter", "apply"},
				{"esc", "clear"},
			})
		case bookmarkModeConfirmDelete:
			return m.renderHints([]hintEntry{
				{"y", "confirm delete"},
				{"n", "cancel"},
			})
		case bookmarkModeConfirmDeleteAll:
			return m.renderHints([]hintEntry{
				{"y", "confirm delete all"},
				{"n", "cancel"},
			})
		default:
			return m.renderHints([]hintEntry{
				{"a-z/0-9", "jump"},
				{"enter", "jump"},
				{"/", "filter"},
				{"D", "delete"},
				{"ctrl+x", "delete all"},
				{"esc", "close"},
			})
		}

	case overlayTemplates:
		if m.templateSearchMode {
			return m.renderHints([]hintEntry{
				{"type", "filter"},
				{"enter", "apply"},
				{"esc", "clear"},
			})
		}
		return m.renderHints([]hintEntry{
			{"enter", "select"},
			{"/", "filter"},
			{"esc", "close"},
		})

	case overlayColorscheme:
		return m.renderHints([]hintEntry{
			{"j/k", "navigate"},
			{"enter", "apply"},
			{"/", "filter"},
			{"esc", "cancel"},
		})

	case overlayFilterPreset:
		return m.renderHints([]hintEntry{
			{"key", "apply"},
			{"enter", "apply"},
			{".", "clear"},
			{"esc", "close"},
		})

	case overlayRBAC:
		return m.renderHints([]hintEntry{
			{"any key", "close"},
		})

	case overlayBatchLabel:
		return m.renderHints([]hintEntry{
			{"Tab", "toggle add/remove"},
			{"Enter", "apply"},
			{"esc", "cancel"},
		})

	case overlayPodStartup:
		return m.renderHints([]hintEntry{
			{"any key", "close"},
		})

	case overlayQuotaDashboard:
		return m.renderHints([]hintEntry{
			{"esc", "close"},
		})

	case overlayEventTimeline:
		return m.renderHints([]hintEntry{
			{"j/k", "scroll"},
			{"g/G", "top/bottom"},
			{"esc", "close"},
		})

	case overlayAlerts:
		return m.renderHints([]hintEntry{
			{"j/k", "scroll"},
			{"esc", "close"},
		})

	case overlayNetworkPolicy:
		return m.renderHints([]hintEntry{
			{"j/k", "scroll"},
			{"g/G", "top/bottom"},
			{"ctrl+d/u", "half page"},
			{"ctrl+f/b", "page"},
			{"esc", "close"},
		})

	case overlaySecretEditor:
		if m.secretEditing {
			return m.renderHints([]hintEntry{
				{"ctrl+s", "save"},
				{"enter", "newline"},
				{"tab", "switch"},
				{"esc", "cancel"},
			})
		}
		return m.renderHints([]hintEntry{
			{"jk", "nav"},
			{"v", "toggle"},
			{"V", "all"},
			{"e", "edit"},
			{"a", "add"},
			{"y", "copy"},
			{"D", "del"},
			{"s", "save"},
			{"esc", "close"},
		})

	case overlayConfigMapEditor:
		if m.configMapEditing {
			return m.renderHints([]hintEntry{
				{"ctrl+s", "save"},
				{"enter", "newline"},
				{"tab", "switch"},
				{"esc", "cancel"},
			})
		}
		return m.renderHints([]hintEntry{
			{"jk", "nav"},
			{"e", "edit"},
			{"a", "add"},
			{"y", "copy"},
			{"D", "del"},
			{"s", "save"},
			{"esc", "close"},
		})

	case overlayRollback, overlayHelmRollback:
		return m.renderHints([]hintEntry{
			{"jk", "nav"},
			{"Enter", "rollback"},
			{"esc", "cancel"},
		})

	case overlayLabelEditor:
		if m.labelEditing {
			return m.renderHints([]hintEntry{
				{"ctrl+s", "save"},
				{"tab", "switch"},
				{"esc", "cancel"},
			})
		}
		return m.renderHints([]hintEntry{
			{"Tab", "switch"},
			{"jk", "nav"},
			{"e", "edit"},
			{"a", "add"},
			{"y", "copy"},
			{"D", "del"},
			{"s", "save"},
			{"esc", "close"},
		})

	case overlayCanI:
		if m.canISearchActive {
			return m.renderHints([]hintEntry{
				{"type", "search"},
				{"enter", "apply"},
				{"esc", "clear"},
			})
		}
		if m.canISearchQuery != "" {
			return m.renderHints([]hintEntry{
				{"j/k", "navigate"},
				{"/", "edit search"},
				{"esc", "clear search"},
			})
		}
		filterLabel := "all"
		if m.canIAllowedOnly {
			filterLabel = "allowed only"
		}
		return m.renderHints([]hintEntry{
			{"j/k", "navigate"},
			{"a", filterLabel},
			{"s", "switch subject"},
			{"/", "search groups"},
			{"q/Esc", "close"},
		})

	case overlayCanISubject:
		return m.renderHints([]hintEntry{
			{"enter", "select"},
			{"/", "filter"},
			{"esc", "cancel"},
		})

	case overlayExplainSearch:
		return m.renderHints([]hintEntry{
			{"enter", "navigate"},
			{"/", "filter"},
			{"esc", "close"},
		})

	default:
		return ""
	}
}

// hintEntry is a key-description pair for status bar hints.
type hintEntry struct {
	key  string
	desc string
}

// renderHints formats hint entries into a styled status bar string.
func (m Model) renderHints(hints []hintEntry) string {
	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		parts = append(parts, ui.HelpKeyStyle.Render(h.key)+ui.DimStyle.Render(": "+h.desc))
	}
	return strings.Join(parts, ui.DimStyle.Render(" \u2502 "))
}
