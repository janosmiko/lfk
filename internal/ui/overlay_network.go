package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// RenderNetworkPolicyOverlay renders the network policy visualizer overlay content.
// The overlay shows pod selector, policy types, affected pods, and a visual diagram
// of ingress/egress rules using box-drawing characters and arrows.
func RenderNetworkPolicyOverlay(info NetworkPolicyEntry, scroll, width, height int) string {
	// Styles for the diagram.
	greenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary))
	arrowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary)).Bold(true)
	boxBorderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBorder))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPurple))
	cidrStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorOrange))
	sectionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary)).Bold(true).Underline(true)

	var lines []string

	// Title.
	lines = append(lines, OverlayTitleStyle.Render(fmt.Sprintf("Network Policy: %s", info.Name)))
	lines = append(lines, OverlayDimStyle.Render(fmt.Sprintf("  Namespace: %s", info.Namespace)))
	lines = append(lines, "")

	// Pod Selector section.
	lines = append(lines, sectionStyle.Render("Pod Selector"))
	if len(info.PodSelector) == 0 {
		lines = append(lines, OverlayDimStyle.Render("  (all pods in namespace)"))
	} else {
		selectorKeys := make([]string, 0, len(info.PodSelector))
		for k := range info.PodSelector {
			selectorKeys = append(selectorKeys, k)
		}
		sort.Strings(selectorKeys)
		for _, k := range selectorKeys {
			lines = append(lines, fmt.Sprintf("  %s", labelStyle.Render(k+"="+info.PodSelector[k])))
		}
	}
	lines = append(lines, "")

	// Affected Pods count.
	podCount := len(info.AffectedPods)
	podCountStr := fmt.Sprintf("%d pod(s)", podCount)
	if podCount == 0 {
		podCountStr = "0 pods (or unable to list)"
	}
	lines = append(lines, fmt.Sprintf("  %s %s",
		OverlayNormalStyle.Render("Affected Pods:"),
		greenStyle.Render(podCountStr)))
	if podCount > 0 && podCount <= 10 {
		for _, name := range info.AffectedPods {
			lines = append(lines, fmt.Sprintf("    %s", OverlayDimStyle.Render(name)))
		}
	} else if podCount > 10 {
		for _, name := range info.AffectedPods[:10] {
			lines = append(lines, fmt.Sprintf("    %s", OverlayDimStyle.Render(name)))
		}
		lines = append(lines, fmt.Sprintf("    %s", OverlayDimStyle.Render(fmt.Sprintf("... and %d more", podCount-10))))
	}
	lines = append(lines, "")

	// Policy Types.
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

	// Build the target box content (reused for all rules).
	targetLabel := "(all pods)"
	if len(info.PodSelector) > 0 {
		targetKeys := make([]string, 0, len(info.PodSelector))
		for k := range info.PodSelector {
			targetKeys = append(targetKeys, k)
		}
		sort.Strings(targetKeys)
		parts := make([]string, 0, len(info.PodSelector))
		for _, k := range targetKeys {
			parts = append(parts, k+"="+info.PodSelector[k])
		}
		targetLabel = strings.Join(parts, "\n")
	}

	// --- INGRESS RULES ---
	hasIngress := false
	for _, pt := range info.PolicyTypes {
		if pt == "Ingress" {
			hasIngress = true
			break
		}
	}
	if hasIngress || len(info.IngressRules) > 0 {
		lines = append(lines, sectionStyle.Render("INGRESS RULES"))
		lines = append(lines, "")

		if len(info.IngressRules) == 0 {
			lines = append(lines, OverlayWarningStyle.Render("  No ingress rules = all ingress denied"))
			lines = append(lines, "")
		}

		for i, rule := range info.IngressRules {
			lines = append(lines, OverlayNormalStyle.Render(fmt.Sprintf("  Rule %d:", i+1)))
			ruleLines := renderNetpolRuleDiagram(rule, targetLabel, true, width,
				boxBorderStyle, arrowStyle, labelStyle, cidrStyle, greenStyle)
			lines = append(lines, ruleLines...)
			lines = append(lines, "")
		}
	}

	// --- EGRESS RULES ---
	hasEgress := false
	for _, pt := range info.PolicyTypes {
		if pt == "Egress" {
			hasEgress = true
			break
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
			lines = append(lines, OverlayNormalStyle.Render(fmt.Sprintf("  Rule %d:", i+1)))
			ruleLines := renderNetpolRuleDiagram(rule, targetLabel, false, width,
				boxBorderStyle, arrowStyle, labelStyle, cidrStyle, greenStyle)
			lines = append(lines, ruleLines...)
			lines = append(lines, "")
		}
	}

	// No rules at all.
	if !hasIngress && !hasEgress && len(info.IngressRules) == 0 && len(info.EgressRules) == 0 {
		lines = append(lines, OverlayDimStyle.Render("  No policy types or rules defined"))
		lines = append(lines, "")
	}

	// Footer.
	lines = append(lines, "")

	// Content area height: total height minus hint bar (1 line).
	maxVisible := max(height-1, 3)

	// Clamp scroll.
	maxScroll := max(len(lines)-maxVisible, 0)
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}

	end := min(scroll+maxVisible, len(lines))
	visible := lines[scroll:end]

	// Pad visible lines to fill the content area so the hint bar stays at the bottom.
	for len(visible) < maxVisible {
		visible = append(visible, "")
	}

	body := strings.Join(visible, "\n")

	// Scroll info (hints moved to the main status bar).
	if maxScroll > 0 {
		body += "\n" + DimStyle.Render(fmt.Sprintf(" [%d/%d]", scroll+1, maxScroll+1))
	}

	return body
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

		boxLines := renderTwoBoxes(leftBox, rightBox, arrow, boxBorder, width)
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
		available := maxWidth - overhead
		if available < 2 {
			available = 2
		}
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
