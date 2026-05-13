package ui

import (
	"slices"
	"strings"
	"time"

	"github.com/janosmiko/lfk/internal/model"
)

// overlayNsScroll is the persistent scroll position for the namespace overlay.
var overlayNsScroll int

// ResetOverlayNsScroll resets the namespace overlay scroll position (call when opening the overlay).
func ResetOverlayNsScroll() { overlayNsScroll = 0 }

// GetOverlayNsScroll returns the current scroll offset of the namespace
// overlay. Used by mouse click resolution to translate a click row into
// the correct item index in the underlying items slice.
func GetOverlayNsScroll() int { return overlayNsScroll }

// ResetOverlayPodScroll is a deprecated no-op; the pod-selector overlay now
// derives its scroll window from the cursor on every render via OverlayList.
//
// Deprecated: drop the call site.
func ResetOverlayPodScroll() {}

// overlayContainerScroll is the persistent scroll position for the
// log-container multi-select overlay (still uses the legacy renderer
// pending its wave-3 OverlayList migration).
var overlayContainerScroll int

// ResetOverlayContainerScroll resets the container overlay scroll position
// (call when opening the overlay).
func ResetOverlayContainerScroll() { overlayContainerScroll = 0 }

// ResetOverlayCanISubjectScroll is a deprecated no-op; the can-i subject
// selector overlay now derives its scroll window from the cursor on every
// render.
//
// Deprecated: drop the call site.
func ResetOverlayCanISubjectScroll() {}

// ErrorLogEntry stores a single application log entry with its timestamp and severity level.
type ErrorLogEntry struct {
	Time    time.Time
	Message string
	Level   string // "ERR", "WRN", "INF", "DBG"
}

// PortInfo represents a discovered port for the port forward overlay.
type PortInfo struct {
	Port     string
	Name     string
	Protocol string
}

// RBACCheckEntry holds RBAC check data for rendering in the overlay.
type RBACCheckEntry struct {
	Verb    string
	Allowed bool
}

// PodStartupEntry holds pod startup data for rendering, avoiding k8s package import.
type PodStartupEntry struct {
	PodName   string
	Namespace string
	TotalTime time.Duration
	Phases    []StartupPhaseEntry
}

// StartupPhaseEntry represents a single phase in the pod startup sequence for rendering.
type StartupPhaseEntry struct {
	Name     string
	Duration time.Duration
	Status   string // "completed", "in-progress", "unknown"
}

// QuotaEntry holds quota data for rendering in the overlay (avoids importing k8s).
type QuotaEntry struct {
	Name      string
	Namespace string
	Resources []QuotaResourceEntry
}

// QuotaResourceEntry holds usage data for a single resource in the quota overlay.
type QuotaResourceEntry struct {
	Name    string
	Hard    string
	Used    string
	Percent float64
}

// EventTimelineEntry holds event data for rendering in the timeline overlay.
type EventTimelineEntry struct {
	Timestamp    time.Time
	Type         string // "Normal" or "Warning"
	Reason       string
	Message      string
	Source       string
	Count        int32
	InvolvedName string
	InvolvedKind string
}

// AlertEntry holds alert data for rendering in the overlay, decoupled from the k8s package.
type AlertEntry struct {
	Name        string
	State       string // "firing", "pending"
	Severity    string // "critical", "warning", "info"
	Summary     string
	Description string
	Since       time.Time
	GrafanaURL  string
}

// NetworkPolicyEntry holds network policy data for rendering, decoupled from the k8s package.
type NetworkPolicyEntry struct {
	Name         string
	Namespace    string
	PodSelector  map[string]string
	PolicyTypes  []string
	IngressRules []NetpolRuleEntry
	EgressRules  []NetpolRuleEntry
	AffectedPods []string
}

// NetpolRuleEntry holds a single ingress/egress rule for rendering.
type NetpolRuleEntry struct {
	Ports []NetpolPortEntry
	Peers []NetpolPeerEntry
}

// NetpolPortEntry holds port information for a network policy rule.
type NetpolPortEntry struct {
	Protocol string
	Port     string
}

// NetpolPeerEntry holds peer information for a network policy rule.
type NetpolPeerEntry struct {
	Type      string
	Selector  map[string]string
	CIDR      string
	Except    []string
	Namespace string
}

// RenderNamespaceOverlay renders the namespace selection overlay content.
//
// The height parameter is the overlay box height the caller will pass to
// OverlayStyle.Height(). The renderer caps its visible-item count to fit
// inside that budget so lipgloss never has to grow the box on overflow \u2014
// without this, a list of 30+ namespaces overflows a 20-tall box (the
// renderer used to emit ~21 lines), and as the user typed into the
// filter the box visibly "shrank" back to its declared size when fewer
// items fit. Mirrors the layout contract enforced for the column toggle
// overlay.
func RenderNamespaceOverlay(items []model.Item, filter string, cursor int, currentNs string, allNs bool, selectedNamespaces map[string]bool, filterMode bool, height int) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Select Namespace"))
	b.WriteString("\n")

	// Filter input.
	switch {
	case filterMode:
		b.WriteString(OverlayFilterStyle.Render("/ " + filter + "\u2588"))
	case filter != "":
		b.WriteString(OverlayFilterStyle.Render("/ " + filter))
	default:
		b.WriteString(OverlayDimStyle.Render("/ to filter"))
	}
	b.WriteString("\n\n")

	if items == nil {
		b.WriteString(OverlayDimStyle.Render("Loading namespaces..."))
		return b.String()
	}
	if len(items) == 0 {
		b.WriteString(OverlayDimStyle.Render("No matching namespaces"))
		return b.String()
	}

	// Reserve rows the rendered overlay needs that the caller's `height`
	// must absorb:
	//   chrome: title (1 + 1 bottom padding) + filter (1) + blank
	//           separator (1) + scroll-above (1) + scroll-below (1) = 6
	//   lipgloss vertical padding from OverlayStyle.Padding(1,2):     2
	// so the item budget is `height - 8`.
	//
	// Reserving only 6 (the obvious chrome) is wrong: lipgloss
	// `Height(h)` measures the *content area including padding*, so
	// padding eats 2 rows out of `height` — content over `height-2`
	// makes lipgloss grow the box on overflow, and as the filter
	// narrows the list the box visibly "shrinks" back to its nominal
	// size (the 21→20→…→22 cascade the user reported, observed right
	// as the "↓ N below" indicator turns into its placeholder row).
	maxVisible := min(max(height-8, 1), len(items))
	scrollOff := ConfigScrollOff
	// Disable or reduce scrolloff when all items fit the visible area.
	if len(items) <= maxVisible {
		scrollOff = 0
	} else if maxSO := (maxVisible - 1) / 2; scrollOff > maxSO {
		scrollOff = maxSO
	}

	// Use VimScrollOff for stable viewport behavior.
	displayLines := func(from, to int) int { return to - from }
	start := VimScrollOff(overlayNsScroll, cursor, len(items), maxVisible, scrollOff, displayLines)
	overlayNsScroll = start

	end := min(start+maxVisible, len(items))

	b.WriteString(RenderScrollAbove(start, end-start, len(items), 0))
	b.WriteString("\n")

	for i := start; i < end; i++ {
		item := items[i]
		prefix := "  "
		switch {
		case item.Status == "all":
			if allNs && len(selectedNamespaces) == 0 {
				prefix = "\u2713 "
			}
		case selectedNamespaces != nil && selectedNamespaces[item.Name]:
			prefix = "\u2713 "
		case item.Name == currentNs && !allNs && len(selectedNamespaces) == 0:
			prefix = "* "
		}
		line := prefix + item.Name
		if i == cursor {
			b.WriteString(OverlaySelectedStyle.Render(line))
		} else {
			b.WriteString(OverlayNormalStyle.Render(line))
		}
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(RenderScrollBelow(start, end-start, len(items), 0))

	return b.String()
}

// RenderLogContainerSelectOverlay renders the container filter overlay for the log viewer.
// The first item should be an "All Containers" virtual item with Status "all".
// Empty selectedContainers means all containers are selected.
func RenderLogContainerSelectOverlay(items []model.Item, cursor int, selectedContainers []string, filter string, filterActive bool, canSwitchPod bool) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Filter Containers"))
	b.WriteString("\n")

	// Filter input.
	switch {
	case filterActive:
		b.WriteString(OverlayFilterStyle.Render("/ " + filter + "\u2588"))
	case filter != "":
		b.WriteString(OverlayFilterStyle.Render("/ " + filter))
	default:
		b.WriteString(OverlayDimStyle.Render("/ to filter"))
	}
	b.WriteString("\n\n")

	if items == nil {
		b.WriteString(OverlayDimStyle.Render("Loading containers..."))
		return b.String()
	}
	if len(items) == 0 {
		b.WriteString(OverlayDimStyle.Render("No matching containers"))
		return b.String()
	}

	maxVisible := min(15, len(items))
	scrollOff := ConfigScrollOff
	if len(items) <= maxVisible {
		scrollOff = 0
	} else if maxSO := (maxVisible - 1) / 2; scrollOff > maxSO {
		scrollOff = maxSO
	}

	displayLines := func(from, to int) int { return to - from }
	start := VimScrollOff(overlayContainerScroll, cursor, len(items), maxVisible, scrollOff, displayLines)
	overlayContainerScroll = start

	end := min(start+maxVisible, len(items))

	b.WriteString(RenderScrollAbove(start, end-start, len(items), 0))
	b.WriteString("\n")

	for i := start; i < end; i++ {
		item := items[i]
		prefix := "  "
		switch {
		case item.Status == "all":
			if len(selectedContainers) == 0 {
				prefix = "\u2713 "
			}
		case slices.Contains(selectedContainers, item.Name):
			prefix = "\u2713 "
		}
		line := prefix + item.Name

		if i == cursor {
			b.WriteString(OverlaySelectedStyle.Render(line))
		} else {
			b.WriteString(OverlayNormalStyle.Render(line))
		}
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(RenderScrollBelow(start, end-start, len(items), 0))

	return b.String()
}
