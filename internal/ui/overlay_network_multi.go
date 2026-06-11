package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderNetworkPoliciesOverlay renders the multi-policy view opened from a
// pod or service action menu: every network policy that selects the resource,
// stacked, with the same diagrams as the single-policy visualizer.
func RenderNetworkPoliciesOverlay(info ResourceNetpolsEntry, scroll, width, height int) string {
	return renderScrollableLines(buildNetpolsOverlayLines(info, width), scroll, width, height)
}

// NetworkPoliciesOverlayLineCount returns the total line count of the
// multi-policy view at the given width, for scroll clamping.
func NetworkPoliciesOverlayLineCount(info ResourceNetpolsEntry, width int) int {
	return len(buildNetpolsOverlayLines(info, width))
}

// buildNetpolsOverlayLines composes the full (unscrolled) line list for the
// multi-policy view.
func buildNetpolsOverlayLines(info ResourceNetpolsEntry, width int) []string {
	greenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Background(SurfaceBg)
	arrowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary)).Bold(true).Background(SurfaceBg)
	boxBorderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBorder)).Background(SurfaceBg)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPurple)).Background(SurfaceBg)
	cidrStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorOrange)).Background(SurfaceBg)
	sectionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary)).Bold(true).Underline(true).Background(SurfaceBg)

	var lines []string
	lines = append(lines, OverlayTitleStyle.Render(fmt.Sprintf("Network Policies: %s: %s", info.Kind, info.Name)))
	lines = append(lines, OverlayDimStyle.Render(fmt.Sprintf("  Namespace: %s", info.Namespace)))
	lines = append(lines, "")

	switch {
	case info.NoSelector:
		lines = append(lines, OverlayWarningStyle.Render("  Service has no pod selector"))
		lines = append(lines, OverlayDimStyle.Render("  Pod-selector based network policies cannot be matched against it."))
	case len(info.Policies) == 0:
		lines = append(lines, greenStyle.Render(fmt.Sprintf("  No network policies select this %s", strings.ToLower(info.Kind))))
		lines = append(lines, OverlayDimStyle.Render("  No policy restrictions apply: all traffic allowed."))
	default:
		lines = append(lines, renderNetpolMultiBody(info, width, greenStyle, labelStyle, sectionStyle, boxBorderStyle, arrowStyle, cidrStyle)...)
	}
	lines = append(lines, "")
	return flattenRenderedLines(lines)
}

// renderNetpolMultiBody renders the summary line and the stacked per-policy
// sections.
func renderNetpolMultiBody(info ResourceNetpolsEntry, width int, greenStyle, labelStyle, sectionStyle, boxBorderStyle, arrowStyle, cidrStyle lipgloss.Style) []string {
	var lines []string

	var summary string
	switch {
	case strings.HasPrefix(info.Kind, "Cilium"):
		// Stacked view of one multi-spec Cilium policy, not of policies
		// selecting a resource.
		summary = fmt.Sprintf("%d policy specs", len(info.Policies))
	case len(info.Policies) == 1:
		summary = fmt.Sprintf("1 network policy selects this %s", strings.ToLower(info.Kind))
	default:
		summary = fmt.Sprintf("%d network policies select this %s", len(info.Policies), strings.ToLower(info.Kind))
	}
	lines = append(lines, fmt.Sprintf("  %s", greenStyle.Render(summary)))
	if info.Kind == "Service" {
		lines = append(lines, OverlayDimStyle.Render(fmt.Sprintf("  Backing pods: %d", len(info.BackingPods))))
	}
	lines = append(lines, "")

	divider := OverlayDimStyle.Render(strings.Repeat("─", max(width-2, 10)))
	for _, pol := range info.Policies {
		lines = append(lines, divider)
		lines = append(lines, renderNetpolHeader(pol, greenStyle, labelStyle, sectionStyle)...)
		if info.Kind == "Service" {
			lines = append(lines, renderNetpolCoverage(pol, info, greenStyle)...)
		}
		targetLabel := renderNetpolTargetLabel(pol)
		lines = append(lines, renderNetpolDirectionRules(pol, targetLabel, width, sectionStyle, boxBorderStyle, arrowStyle, labelStyle, cidrStyle, greenStyle)...)
	}
	return lines
}

// renderNetpolCoverage renders which of the service's backing pods a policy
// selects, since a policy may cover only a subset of them.
func renderNetpolCoverage(pol NetworkPolicyEntry, info ResourceNetpolsEntry, greenStyle lipgloss.Style) []string {
	var lines []string
	coverage := fmt.Sprintf("%d of %d backing pod(s)", len(pol.MatchedPods), len(info.BackingPods))
	style := greenStyle
	if len(pol.MatchedPods) < len(info.BackingPods) {
		style = OverlayWarningStyle
	}
	lines = append(lines, fmt.Sprintf("  %s %s",
		OverlayNormalStyle.Render("Covers:"),
		style.Render(coverage)))
	maxShow := 10
	shown := min(len(pol.MatchedPods), maxShow)
	for _, name := range pol.MatchedPods[:shown] {
		lines = append(lines, fmt.Sprintf("    %s", OverlayDimStyle.Render(name)))
	}
	if len(pol.MatchedPods) > maxShow {
		lines = append(lines, fmt.Sprintf("    %s", OverlayDimStyle.Render(fmt.Sprintf("... and %d more", len(pol.MatchedPods)-maxShow))))
	}
	lines = append(lines, "")
	return lines
}
