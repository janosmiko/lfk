package ui

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// RenderNetworkPolicyOverlay renders the network policy visualizer overlay content.
// The overlay shows pod selector, policy types, affected pods, and a visual diagram
// of ingress/egress rules using box-drawing characters and arrows. A non-empty
// query highlights matching text in the visible lines.
func RenderNetworkPolicyOverlay(info NetworkPolicyEntry, scroll, width, height int, query string) string {
	return renderScrollableLines(buildNetpolOverlayLines(info, width), scroll, width, height, query)
}

// NetworkPolicyOverlayLines returns the full (unscrolled) styled line list of
// the single-policy view, for search/match scanning by the key handler.
func NetworkPolicyOverlayLines(info NetworkPolicyEntry, width int) []string {
	return buildNetpolOverlayLines(info, width)
}

// buildNetpolOverlayLines composes the full (unscrolled) line list for the
// single-policy view.
func buildNetpolOverlayLines(info NetworkPolicyEntry, width int) []string {
	greenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Background(SurfaceBg)
	arrowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary)).Bold(true).Background(SurfaceBg)
	boxBorderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBorder)).Background(SurfaceBg)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPurple)).Background(SurfaceBg)
	cidrStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorOrange)).Background(SurfaceBg)
	sectionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary)).Bold(true).Underline(true).Background(SurfaceBg)

	lines := renderNetpolHeader(info, greenStyle, labelStyle, sectionStyle)
	targetLabel := renderNetpolTargetLabel(info)
	lines = append(lines, renderNetpolDirectionRules(info, targetLabel, width, sectionStyle, boxBorderStyle, arrowStyle, labelStyle, cidrStyle, greenStyle)...)
	lines = append(lines, "")
	return flattenRenderedLines(lines)
}

// flattenRenderedLines splits multi-row entries into physical rows. Styles
// with vertical padding (e.g. OverlayTitleStyle) render embedded newlines;
// scrolling must operate on terminal rows or the overlay height shifts as a
// multi-row entry enters or leaves the viewport.
func flattenRenderedLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, strings.Split(l, "\n")...)
	}
	return out
}

// NetworkPolicyOverlayLineCount returns the total line count of the
// single-policy view at the given width, for scroll clamping.
func NetworkPolicyOverlayLineCount(info NetworkPolicyEntry, width int) int {
	return len(buildNetpolOverlayLines(info, width))
}

// OverlayMaxScroll returns the bottom scroll position for a scrollable
// overlay body of lineCount lines in a viewport of the given height. Must
// stay in sync with renderScrollableLines.
func OverlayMaxScroll(lineCount, height int) int {
	maxVisible := max(height, 3)
	return max(lineCount-maxVisible, 0)
}

// renderNetpolHeader renders the title, pod selector, affected pods, and policy types sections.
func renderNetpolHeader(info NetworkPolicyEntry, greenStyle, labelStyle, sectionStyle lipgloss.Style) []string {
	clusterwide := info.Kind == "CiliumClusterwideNetworkPolicy"

	var lines []string
	kindLabel := "Network Policy"
	if info.Kind != "" && info.Kind != "NetworkPolicy" {
		kindLabel = info.Kind
	}
	lines = append(lines, OverlayTitleStyle.Render(fmt.Sprintf("%s: %s", kindLabel, info.Name)))
	namespace := info.Namespace
	if namespace == "" {
		namespace = "(cluster-wide)"
	}
	lines = append(lines, OverlayDimStyle.Render(fmt.Sprintf("  Namespace: %s", namespace)))
	lines = append(lines, "")

	lines = append(lines, sectionStyle.Render("Pod Selector"))
	switch {
	case info.NodePolicy:
		lines = append(lines, OverlayWarningStyle.Render("  (node policy: selects nodes, not pods)"))
	case len(info.PodSelector) == 0 && clusterwide:
		lines = append(lines, OverlayDimStyle.Render("  (all pods in all namespaces)"))
	case len(info.PodSelector) == 0:
		lines = append(lines, OverlayDimStyle.Render("  (all pods in namespace)"))
	default:
		for _, k := range sortedKeys(info.PodSelector) {
			lines = append(lines, fmt.Sprintf("  %s", labelStyle.Render(k+"="+info.PodSelector[k])))
		}
	}
	lines = append(lines, "")

	podCount := len(info.AffectedPods)
	podCountStr := fmt.Sprintf("%d pod(s)", podCount)
	if podCount == 0 {
		podCountStr = "0 pods (or unable to list)"
	}
	lines = append(lines, fmt.Sprintf("  %s %s",
		OverlayNormalStyle.Render("Affected Pods:"),
		greenStyle.Render(podCountStr)))
	maxShow := 10
	shown := min(podCount, maxShow)
	for _, name := range info.AffectedPods[:shown] {
		lines = append(lines, fmt.Sprintf("    %s", OverlayDimStyle.Render(name)))
	}
	if podCount > maxShow {
		lines = append(lines, fmt.Sprintf("    %s", OverlayDimStyle.Render(fmt.Sprintf("... and %d more", podCount-maxShow))))
	}
	lines = append(lines, "")

	if len(info.PolicyTypes) > 0 {
		lines = append(lines, fmt.Sprintf("  %s %s",
			OverlayNormalStyle.Render("Policy Types:"),
			greenStyle.Render(strings.Join(info.PolicyTypes, ", "))))
	} else {
		lines = append(lines, fmt.Sprintf("  %s %s",
			OverlayNormalStyle.Render("Policy Types:"),
			OverlayDimStyle.Render("(none specified)")))
	}
	lines = append(lines, "")
	return lines
}

// renderNetpolTargetLabel builds the target label from the pod selector.
func renderNetpolTargetLabel(info NetworkPolicyEntry) string {
	if len(info.PodSelector) == 0 {
		return "(all pods)"
	}
	keys := sortedKeys(info.PodSelector)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+info.PodSelector[k])
	}
	return strings.Join(parts, "\n")
}

// hasPolicyType returns true if the policy types list contains the given type.
func hasPolicyType(types []string, target string) bool {
	return slices.Contains(types, target)
}

// renderNetpolDirectionRules renders ingress and egress rule sections.
func renderNetpolDirectionRules(info NetworkPolicyEntry, targetLabel string, width int, sectionStyle, boxBorderStyle, arrowStyle, labelStyle, cidrStyle, greenStyle lipgloss.Style) []string {
	var lines []string
	hasIngress := hasPolicyType(info.PolicyTypes, "Ingress")
	hasEgress := hasPolicyType(info.PolicyTypes, "Egress")

	if hasIngress || len(info.IngressRules) > 0 {
		lines = append(lines, sectionStyle.Render("INGRESS RULES"))
		lines = append(lines, "")
		if len(info.IngressRules) == 0 {
			lines = append(lines, OverlayWarningStyle.Render("  No ingress rules = all ingress denied"))
			lines = append(lines, "")
		}
		for i, rule := range info.IngressRules {
			lines = append(lines, renderNetpolRuleLabel(rule, i))
			lines = append(lines, renderNetpolRuleDiagram(rule, targetLabel, true, width,
				boxBorderStyle, arrowStyle, labelStyle, cidrStyle, greenStyle)...)
			lines = append(lines, "")
		}
	}

	if hasEgress || len(info.EgressRules) > 0 {
		lines = append(lines, sectionStyle.Render("EGRESS RULES"))
		lines = append(lines, "")
		if len(info.EgressRules) == 0 {
			lines = append(lines, OverlayWarningStyle.Render("  No egress rules = all egress denied"))
			lines = append(lines, "")
		}
		for i, rule := range info.EgressRules {
			lines = append(lines, renderNetpolRuleLabel(rule, i))
			lines = append(lines, renderNetpolRuleDiagram(rule, targetLabel, false, width,
				boxBorderStyle, arrowStyle, labelStyle, cidrStyle, greenStyle)...)
			lines = append(lines, "")
		}
	}

	if !hasIngress && !hasEgress && len(info.IngressRules) == 0 && len(info.EgressRules) == 0 {
		lines = append(lines, OverlayDimStyle.Render("  No policy types or rules defined"))
		lines = append(lines, "")
	}
	return lines
}

// renderNetpolRuleLabel renders a rule's heading line: deny rules (Cilium)
// are marked and styled as warnings. An L7 summary is appended when present.
func renderNetpolRuleLabel(rule NetpolRuleEntry, idx int) string {
	label := fmt.Sprintf("  Rule %d:", idx+1)
	style := OverlayNormalStyle
	if rule.Deny {
		label = fmt.Sprintf("  Rule %d (deny):", idx+1)
		style = OverlayWarningStyle
	}
	out := style.Render(label)
	if rule.L7 != "" {
		out += OverlayDimStyle.Render("  L7: " + rule.L7)
	}
	return out
}

// sortedKeys returns the keys of a map sorted alphabetically.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// renderScrollableLines applies scroll/clamp logic and returns the visible body string.
// Lines wider than width are truncated: an over-wide line would wrap inside
// the overlay frame and change the overlay height while scrolling. When the
// content overflows the viewport, each row gets the shared right-edge
// scrollbar cell (same look as the list overlays). A non-empty query
// highlights matches in the visible lines (before truncation, so a match
// past the right edge is simply cut with the rest of the line).
func renderScrollableLines(lines []string, scroll, width, height int, query string) string {
	maxVisible := max(height, 3)
	maxScroll := OverlayMaxScroll(len(lines), height)
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}
	showScrollbar := maxScroll > 0 && width > 2
	lineWidth := width
	if showScrollbar {
		lineWidth = width - 2 // leave room for " " + scrollbar cell
	}
	end := min(scroll+maxVisible, len(lines))
	visible := make([]string, 0, maxVisible)
	for _, line := range lines[scroll:end] {
		if query != "" {
			line = HighlightMatchInline(line, query, SearchHighlightStyle)
		}
		if lineWidth > 0 && lipgloss.Width(line) > lineWidth {
			line = ansi.Truncate(line, lineWidth, "~")
		}
		visible = append(visible, line)
	}
	for len(visible) < maxVisible {
		visible = append(visible, "")
	}
	if showScrollbar {
		for i, line := range visible {
			visible[i] = padRight(line, lineWidth) + " " + renderScrollbar(i, maxVisible, len(lines), scroll)
		}
	}
	return strings.Join(visible, "\n")
}

// renderNetpolRuleDiagram renders a visual diagram for a single ingress/egress rule.
// For ingress: [Source] -----> [Target Pods]
// For egress:  [Target Pods] -----> [Destination]
func renderNetpolRuleDiagram(
	rule NetpolRuleEntry,
	targetLabel string,
	isIngress bool,
	width int,
	boxBorder, arrowSt, labelSt, cidrSt, greenSt lipgloss.Style,
) []string {
	var lines []string

	// Maximum label width for truncation (roughly half the available width).
	maxLabel := 0
	if width > 0 {
		maxLabel = width / 2
	}

	truncLabel := func(s string) string {
		if maxLabel > 0 && len(s) > maxLabel {
			return s[:maxLabel-1] + "~"
		}
		return s
	}

	for _, peer := range rule.Peers {
		// Build the peer box content.
		var peerLines []string
		switch peer.Type {
		case "All":
			peerLines = append(peerLines, greenSt.Render("All"))
		case "Pod":
			peerLines = append(peerLines, OverlayNormalStyle.Render("Pod:"))
			if len(peer.Selector) > 0 {
				peerKeys := make([]string, 0, len(peer.Selector))
				for k := range peer.Selector {
					peerKeys = append(peerKeys, k)
				}
				sort.Strings(peerKeys)
				for _, k := range peerKeys {
					peerLines = append(peerLines, labelSt.Render(truncLabel(k+"="+peer.Selector[k])))
				}
			} else {
				peerLines = append(peerLines, OverlayDimStyle.Render("(all pods)"))
			}
		case "Namespace":
			peerLines = append(peerLines, OverlayNormalStyle.Render("Namespace:"))
			peerLines = append(peerLines, labelSt.Render(truncLabel(peer.Namespace)))
		case "Namespace+Pod":
			peerLines = append(peerLines, OverlayNormalStyle.Render("NS: "+truncLabel(peer.Namespace)))
			if len(peer.Selector) > 0 {
				peerLines = append(peerLines, OverlayNormalStyle.Render("Pod:"))
				nsPodKeys := make([]string, 0, len(peer.Selector))
				for k := range peer.Selector {
					nsPodKeys = append(nsPodKeys, k)
				}
				sort.Strings(nsPodKeys)
				for _, k := range nsPodKeys {
					peerLines = append(peerLines, labelSt.Render(truncLabel(k+"="+peer.Selector[k])))
				}
			}
		case "CIDR":
			peerLines = append(peerLines, OverlayNormalStyle.Render("CIDR:"))
			peerLines = append(peerLines, cidrSt.Render(peer.CIDR))
			if len(peer.Except) > 0 {
				peerLines = append(peerLines, OverlayDimStyle.Render("Except:"))
				for _, e := range peer.Except {
					peerLines = append(peerLines, cidrSt.Render("  "+e))
				}
			}
		case "Entity":
			peerLines = append(peerLines, OverlayNormalStyle.Render("Entity:"))
			peerLines = append(peerLines, greenSt.Render(truncLabel(peer.Value)))
		case "FQDN":
			peerLines = append(peerLines, OverlayNormalStyle.Render("FQDN:"))
			peerLines = append(peerLines, cidrSt.Render(truncLabel(peer.Value)))
		case "Service":
			peerLines = append(peerLines, OverlayNormalStyle.Render("Service:"))
			peerLines = append(peerLines, labelSt.Render(truncLabel(peer.Value)))
		}

		// Add port info.
		if len(rule.Ports) > 0 {
			peerLines = append(peerLines, "")
			for _, port := range rule.Ports {
				portStr := port.Protocol
				if port.Port != "" {
					portStr += "/" + port.Port
				}
				peerLines = append(peerLines, OverlayDimStyle.Render("Port: ")+greenSt.Render(portStr))
			}
		}

		// Build the target box content.
		targetLabelLines := strings.Split(targetLabel, "\n")
		targetLines := make([]string, 0, 1+len(targetLabelLines))
		targetLines = append(targetLines, greenSt.Render("Target Pods"))
		for _, line := range targetLabelLines {
			targetLines = append(targetLines, labelSt.Render(truncLabel(line)))
		}

		// Render the two boxes with an arrow between them.
		var leftBox, rightBox []string
		var arrow string
		if isIngress {
			leftBox = peerLines
			rightBox = targetLines
			arrow = arrowSt.Render(" -----> ")
		} else {
			leftBox = targetLines
			rightBox = peerLines
			arrow = arrowSt.Render(" -----> ")
		}

		// The diagram is indented by 2 columns below, so cap the boxes 2
		// short of the available width or the indented lines overflow it.
		boxWidth := width
		if boxWidth > 0 {
			boxWidth -= 2
		}
		boxLines := renderTwoBoxes(leftBox, rightBox, arrow, boxBorder, boxWidth)
		for _, bl := range boxLines {
			lines = append(lines, "  "+bl)
		}
	}

	return lines
}

// renderTwoBoxes renders two boxes side by side connected by an arrow.
// Uses box-drawing characters for borders. If maxWidth > 0, box widths are
// capped so the total diagram fits within that width, and content lines are
// truncated accordingly.
func renderTwoBoxes(leftContent, rightContent []string, arrow string, borderStyle lipgloss.Style, maxWidth int) []string {
	// Calculate box widths.
	leftW := 0
	for _, line := range leftContent {
		if w := lipgloss.Width(line); w > leftW {
			leftW = w
		}
	}
	rightW := 0
	for _, line := range rightContent {
		if w := lipgloss.Width(line); w > rightW {
			rightW = w
		}
	}

	// Add padding.
	leftW += 2
	rightW += 2

	// Ensure minimum widths.
	if leftW < 14 {
		leftW = 14
	}
	if rightW < 14 {
		rightW = 14
	}

	arrowW := lipgloss.Width(arrow)

	// Cap box widths so the total diagram fits within maxWidth.
	// Total width = (1 + 1 + leftW + 1 + 1) + arrowW + (1 + 1 + rightW + 1 + 1)
	//             = leftW + 4 + arrowW + rightW + 4
	if maxWidth > 0 {
		// Overhead: left border(1) + space(1) + space(1) + right border(1) = 4 per box.
		overhead := 4 + arrowW + 4
		available := max(maxWidth-overhead, 2)
		if leftW+rightW > available {
			// Split available space proportionally, each gets at least half of minimum (7).
			half := available / 2
			switch {
			case leftW > half && rightW > half:
				leftW = half
				rightW = available - half
			case leftW > half:
				leftW = available - rightW
			default:
				rightW = available - leftW
			}
			if leftW < 7 {
				leftW = 7
			}
			if rightW < 7 {
				rightW = 7
			}
		}
	}

	// Equalize heights.
	maxH := max(len(leftContent), len(rightContent))
	for len(leftContent) < maxH {
		leftContent = append(leftContent, "")
	}
	for len(rightContent) < maxH {
		rightContent = append(rightContent, "")
	}

	// Truncate content lines that exceed their box width.
	for i, line := range leftContent {
		if lipgloss.Width(line) > leftW {
			leftContent[i] = ansi.Truncate(line, leftW, "~")
		}
	}
	for i, line := range rightContent {
		if lipgloss.Width(line) > rightW {
			rightContent[i] = ansi.Truncate(line, rightW, "~")
		}
	}

	result := make([]string, 0, maxH+2)

	// The inner width of each box is: 1 space + content + 1 space = leftW + 2.
	// Border dashes span that same inner width.
	leftInner := leftW + 2
	rightInner := rightW + 2

	// Top borders.
	topLine := borderStyle.Render("\u250c"+strings.Repeat("\u2500", leftInner)+"\u2510") +
		strings.Repeat(" ", arrowW) +
		borderStyle.Render("\u250c"+strings.Repeat("\u2500", rightInner)+"\u2510")
	result = append(result, topLine)

	// Content lines with arrow at the midpoint.
	midRow := maxH / 2
	for i := range maxH {
		leftLine := padRight(leftContent[i], leftW)
		rightLine := padRight(rightContent[i], rightW)

		connector := strings.Repeat(" ", arrowW)
		if i == midRow {
			connector = arrow
		}

		line := borderStyle.Render("\u2502") + " " + leftLine + " " + borderStyle.Render("\u2502") +
			connector +
			borderStyle.Render("\u2502") + " " + rightLine + " " + borderStyle.Render("\u2502")

		result = append(result, line)
	}

	// Bottom borders.
	bottomLine := borderStyle.Render("\u2514"+strings.Repeat("\u2500", leftInner)+"\u2518") +
		strings.Repeat(" ", arrowW) +
		borderStyle.Render("\u2514"+strings.Repeat("\u2500", rightInner)+"\u2518")
	result = append(result, bottomLine)

	return result
}
