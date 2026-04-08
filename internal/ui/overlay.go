package ui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/janosmiko/lfk/internal/model"
)

// overlayNsScroll is the persistent scroll position for the namespace overlay.
var overlayNsScroll int

// ResetOverlayNsScroll resets the namespace overlay scroll position (call when opening the overlay).
func ResetOverlayNsScroll() { overlayNsScroll = 0 }

// overlayPodScroll is the persistent scroll position for the pod selection overlay.
var overlayPodScroll int

// ResetOverlayPodScroll resets the pod overlay scroll position (call when opening the overlay).
func ResetOverlayPodScroll() { overlayPodScroll = 0 }

// overlayContainerScroll is the persistent scroll position for the container selection overlay.
var overlayContainerScroll int

// ResetOverlayContainerScroll resets the container overlay scroll position (call when opening the overlay).
func ResetOverlayContainerScroll() { overlayContainerScroll = 0 }

// overlayCanISubjectScroll is the persistent scroll position for the can-i subject selector overlay.
var overlayCanISubjectScroll int

// ResetOverlayCanISubjectScroll resets the can-i subject overlay scroll position (call when opening the overlay).
func ResetOverlayCanISubjectScroll() { overlayCanISubjectScroll = 0 }

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

// FilterPresetEntry holds the display info for a single filter preset in the overlay.
type FilterPresetEntry struct {
	Name        string
	Description string
	Key         string
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

// bookmarkModeFilter matches the app-level bookmarkModeFilter enum value.
const bookmarkModeFilter = 1

// RenderNamespaceOverlay renders the namespace selection overlay content.
func RenderNamespaceOverlay(items []model.Item, filter string, cursor int, currentNs string, allNs bool, selectedNamespaces map[string]bool, filterMode bool) string {
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

	maxVisible := min(15, len(items))
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

	end := start + maxVisible
	if end > len(items) {
		end = len(items)
	}

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

	return b.String()
}

// RenderActionOverlay renders the action menu overlay content.
func RenderActionOverlay(items []model.Item, cursor int, width int) string {
	// Account for overlay border (1 each side) + padding (2 each side) = 6 total.
	innerW := width - 6
	if innerW < 20 {
		innerW = 20
	}

	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Actions"))
	b.WriteString("\n")

	for i, item := range items {
		keyHint := ""
		if item.Status != "" {
			keyHint = "[" + item.Status + "] "
		}
		label := fmt.Sprintf("  %s%s - %s", keyHint, item.Name, item.Extra)
		// Pad label with spaces to fill the inner width.
		if len(label) < innerW {
			label += strings.Repeat(" ", innerW-len(label))
		}
		if i == cursor {
			b.WriteString(OverlaySelectedStyle.Render(label))
		} else {
			b.WriteString(OverlayNormalStyle.Render(label))
		}
		if i < len(items)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// RenderConfirmOverlay renders the y/n confirmation overlay content
// for standard destructive actions (delete, drain).
func RenderConfirmOverlay(action string) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Confirm Delete"))
	b.WriteString("\n\n")
	b.WriteString(OverlayWarningStyle.Render(fmt.Sprintf("Delete %s?", action)))
	b.WriteString("\n\n")
	return b.String()
}

// RenderQuitConfirmOverlay renders the quit confirmation overlay content.
func RenderQuitConfirmOverlay() string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Quit"))
	b.WriteString("\n\n")
	b.WriteString(OverlayNormalStyle.Render("Quit lfk?"))
	return b.String()
}

// RenderPasteConfirmOverlay renders the multiline paste confirmation overlay.
// lineCount is the number of lines in the pasted text.
func RenderPasteConfirmOverlay(lineCount int) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Paste"))
	b.WriteString("\n\n")
	b.WriteString(OverlayNormalStyle.Render(fmt.Sprintf("Paste contains %d lines.", lineCount)))
	b.WriteString("\n")
	b.WriteString(OverlayNormalStyle.Render("Flatten to single line and insert?"))
	b.WriteString("\n\n")
	b.WriteString(OverlayDimStyle.Render("[y] yes  [n] no"))
	return b.String()
}

// RenderConfirmTypeOverlay renders the type-to-confirm overlay content.
// The user must type "DELETE" to confirm the action.
// title is the overlay header (e.g. "Confirm Delete"), question is the
// action-specific prompt (e.g. "Delete my-pod?").
func RenderConfirmTypeOverlay(title, question, input string) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render(title))
	b.WriteString("\n\n")
	b.WriteString(OverlayWarningStyle.Render(question))
	b.WriteString("\n\n")
	b.WriteString(OverlayNormalStyle.Render("Type "))
	b.WriteString(OverlayFilterStyle.Render("DELETE"))
	b.WriteString(OverlayNormalStyle.Render(" to confirm: "))
	if input == "" {
		b.WriteString(OverlayDimStyle.Render("_"))
	} else {
		b.WriteString(OverlayFilterStyle.Render(input))
	}
	return b.String()
}

// RenderScaleOverlay renders the scale deployment overlay content.
func RenderScaleOverlay(input string) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Scale Deployment"))
	b.WriteString("\n\n")
	b.WriteString(OverlayNormalStyle.Render("Replicas: "))
	if input == "" {
		b.WriteString(OverlayDimStyle.Render("_"))
	} else {
		b.WriteString(OverlayInputStyle.Render(input))
	}
	return b.String()
}

// RenderPVCResizeOverlay renders the PVC resize overlay content.
func RenderPVCResizeOverlay(input, currentSize string) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Resize PVC"))
	b.WriteString("\n\n")
	if currentSize != "" {
		b.WriteString(OverlayDimStyle.Render("Current: " + currentSize))
		b.WriteString("\n")
	}
	b.WriteString(OverlayNormalStyle.Render("New size: "))
	if input == "" {
		b.WriteString(OverlayDimStyle.Render("e.g. 10Gi"))
	} else {
		b.WriteString(OverlayInputStyle.Render(input))
	}
	return b.String()
}

// RenderPortForwardOverlay renders the port forward overlay content.
// availablePorts shows discovered ports from the resource.
// cursor indicates which available port is selected (-1 for manual input mode).
func RenderPortForwardOverlay(input string, availablePorts []PortInfo, cursor int, resourceName string) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Port Forward"))
	if resourceName != "" {
		b.WriteString(OverlayDimStyle.Render("  " + resourceName))
	}
	b.WriteString("\n\n")

	if len(availablePorts) > 0 {
		b.WriteString(OverlayNormalStyle.Render("Available ports:"))
		b.WriteString("\n")
		for i, p := range availablePorts {
			label := fmt.Sprintf("  %s", p.Port)
			if p.Name != "" {
				label += " (" + p.Name + ")"
			}
			if p.Protocol != "" && p.Protocol != "TCP" {
				label += " [" + p.Protocol + "]"
			}
			if i == cursor {
				b.WriteString(OverlaySelectedStyle.Render(label))
			} else {
				b.WriteString(OverlayNormalStyle.Render(label))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if cursor >= 0 && cursor < len(availablePorts) {
		// A port is selected from the list.
		b.WriteString(OverlayNormalStyle.Render("Remote port: "))
		b.WriteString(OverlayInputStyle.Render(availablePorts[cursor].Port))
		b.WriteString("\n")
		b.WriteString(OverlayNormalStyle.Render("Local port:  "))
		if input == "" {
			b.WriteString(OverlayDimStyle.Render("(random)"))
		} else {
			b.WriteString(OverlayInputStyle.Render(input))
		}
	} else {
		b.WriteString(OverlayNormalStyle.Render("Port mapping: "))
		if input == "" {
			b.WriteString(OverlayDimStyle.Render("local:remote"))
		} else {
			b.WriteString(OverlayInputStyle.Render(input))
		}
	}
	return b.String()
}

// RenderContainerSelectOverlay renders the container selection overlay content.
func RenderContainerSelectOverlay(items []model.Item, cursor int) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Select Container"))
	b.WriteString("\n")

	for i, item := range items {
		line := fmt.Sprintf("  %s", item.Name)
		if item.Category != "" && item.Category != "Containers" {
			line += "  (" + item.Category + ")"
		}
		if item.Status != "" {
			line += "  " + item.Status
		}
		if i == cursor {
			b.WriteString(OverlaySelectedStyle.Render(line))
		} else {
			b.WriteString(OverlayNormalStyle.Render(line))
		}
		if i < len(items)-1 {
			b.WriteString("\n")
		}
	}

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

	return b.String()
}

// RenderPodSelectOverlay renders the pod selection overlay content.
func RenderPodSelectOverlay(items []model.Item, cursor int, filter string, filterActive bool) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Select Pod"))
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
		b.WriteString(OverlayDimStyle.Render("Loading pods..."))
		return b.String()
	}
	if len(items) == 0 {
		b.WriteString(OverlayDimStyle.Render("No matching pods"))
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
	start := VimScrollOff(overlayPodScroll, cursor, len(items), maxVisible, scrollOff, displayLines)
	overlayPodScroll = start

	end := min(start+maxVisible, len(items))

	for i := start; i < end; i++ {
		item := items[i]
		line := fmt.Sprintf("  %s", item.Name)
		if item.Status != "" {
			styledStatus := StatusStyle(item.Status).Render(item.Status)
			line += "  " + styledStatus
		}
		if i == cursor {
			if item.Status != "" {
				// Re-render without status styling so selected style dominates the name.
				plainLine := fmt.Sprintf("  %s  %s", item.Name, item.Status)
				b.WriteString(OverlaySelectedStyle.Render(plainLine))
			} else {
				b.WriteString(OverlaySelectedStyle.Render(line))
			}
		} else {
			b.WriteString(OverlayNormalStyle.Render(fmt.Sprintf("  %s", item.Name)))
			if item.Status != "" {
				b.WriteString("  " + StatusStyle(item.Status).Render(item.Status))
			}
		}
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// RenderBookmarkOverlay renders the bookmark list overlay content.
// mode: 0 = normal, 1 = filter. overlayH is the total overlay height for footer pinning.
func RenderBookmarkOverlay(allBookmarks []model.Bookmark, filter string, cursor, mode int) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Bookmarks"))
	b.WriteString("\n")

	// Show mode-specific input line.
	switch mode {
	case bookmarkModeFilter:
		b.WriteString(OverlayFilterStyle.Render("filter> " + filter))
		b.WriteString(OverlayDimStyle.Render("\u2588"))
	default:
		if filter != "" {
			b.WriteString(OverlayDimStyle.Render("filter: "))
			b.WriteString(OverlayFilterStyle.Render(filter))
		}
	}
	b.WriteString("\n\n")

	if len(allBookmarks) == 0 {
		b.WriteString(OverlayDimStyle.Render("No bookmarks yet"))
		b.WriteString("\n\n")
		b.WriteString(OverlayDimStyle.Render("Press "))
		b.WriteString(OverlayFilterStyle.Render("m<key>"))
		b.WriteString(OverlayDimStyle.Render(" in explorer to set a mark"))
		return b.String()
	}

	// Apply filter.
	var bookmarks []model.Bookmark
	if filter == "" {
		bookmarks = allBookmarks
	} else {
		for _, bm := range allBookmarks {
			if MatchLine(bm.Name, filter) {
				bookmarks = append(bookmarks, bm)
			}
		}
	}

	if len(bookmarks) == 0 {
		b.WriteString(OverlayDimStyle.Render("No matching bookmarks"))
		return b.String()
	}

	maxVisible := min(15, len(bookmarks))
	start := 0
	if cursor >= maxVisible {
		start = cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(bookmarks) {
		end = len(bookmarks)
	}

	for i := start; i < end; i++ {
		bm := bookmarks[i]

		if i == cursor {
			// Build plain text for the selected line so the highlight
			// background covers the entire line uniformly.
			var prefix string
			if bm.Slot != "" {
				prefix = bm.Slot + "   "
			} else {
				prefix = "    "
			}
			name := bm.Name
			if len(bm.Namespaces) > 1 {
				name += " [" + strings.Join(bm.Namespaces, ", ") + "]"
			} else if bm.Namespace != "" {
				name += " [" + bm.Namespace + "]"
			}
			line := fmt.Sprintf("  %s%s", prefix, name)
			b.WriteString(OverlaySelectedStyle.Render(line))
		} else {
			// Non-selected: use styled prefix and namespace.
			var prefix string
			if bm.Slot != "" {
				prefix = OverlayFilterStyle.Render(bm.Slot) + "   "
			} else {
				prefix = "    "
			}
			name := bm.Name
			if len(bm.Namespaces) > 1 {
				name += DimStyle.Render(" [" + strings.Join(bm.Namespaces, ", ") + "]")
			} else if bm.Namespace != "" {
				name += DimStyle.Render(" [" + bm.Namespace + "]")
			}
			line := fmt.Sprintf("  %s%s", prefix, name)
			b.WriteString(OverlayNormalStyle.Render(line))
		}
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// RenderTemplateOverlay renders the template selection overlay content.
// filter is the current search text, filterMode indicates active filter input,
// and overlayH is the total overlay height for footer positioning.
func RenderTemplateOverlay(templates []model.ResourceTemplate, filter string, cursor int, filterMode bool, overlayH int) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Create from Template"))
	b.WriteString("\n")

	// Show filter input line.
	if filterMode {
		b.WriteString(OverlayFilterStyle.Render("filter> " + filter))
		b.WriteString(OverlayDimStyle.Render("\u2588"))
	} else if filter != "" {
		b.WriteString(OverlayDimStyle.Render("filter: "))
		b.WriteString(OverlayFilterStyle.Render(filter))
	}
	b.WriteString("\n")

	if len(templates) == 0 {
		b.WriteString(OverlayDimStyle.Render("No templates available"))
		return b.String()
	}

	// Fixed-height list: title(1) + filter(1) = 2 header lines + padding(2).
	interiorH := overlayH - 2
	maxVisible := interiorH - 3 // 2 header lines + 1 blank
	if maxVisible < 1 {
		maxVisible = 1
	}
	if maxVisible > len(templates) {
		maxVisible = len(templates)
	}
	start := 0
	if cursor >= maxVisible {
		start = cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(templates) {
		end = len(templates)
	}

	for i := start; i < end; i++ {
		tmpl := templates[i]
		cat := OverlayDimStyle.Render("[" + tmpl.Category + "]")
		if i == cursor {
			fmt.Fprintf(&b, "  > %s %s", cat, OverlaySelectedStyle.Render(tmpl.Name))
		} else {
			fmt.Fprintf(&b, "    %s %s", cat, OverlayNormalStyle.Render(tmpl.Name))
		}
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}
