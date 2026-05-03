package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// RenderFilterPresetOverlay renders the quick filter preset selection overlay content.
// activePresetName is the name of the currently active preset (empty if none).
// width is the inner content width used to pad the cursor row so the
// selection background spans the entire line.
func RenderFilterPresetOverlay(presets []FilterPresetEntry, cursor int, activePresetName string, width int) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Quick Filters"))
	b.WriteString("\n\n")

	if len(presets) == 0 {
		b.WriteString(OverlayDimStyle.Render("No filter presets available"))
		return b.String()
	}

	for i, preset := range presets {
		if i == cursor {
			// Selected row: render as plain text with a single uniform
			// style so the highlight background covers the whole line
			// (embedded styles would otherwise punch holes in the
			// selection background).
			activeMarker := "  "
			if preset.Name == activePresetName {
				activeMarker = "✓ "
			}
			line := fmt.Sprintf("  %s[%s] %s  %s", activeMarker, preset.Key, preset.Name, preset.Description)
			b.WriteString(OverlaySelectedStyle.Width(width).Render(line))
		} else {
			keyHint := OverlayFilterStyle.Render("[" + preset.Key + "]")
			activeMarker := "  "
			if preset.Name == activePresetName {
				activeMarker = OverlayFilterStyle.Render("✓ ")
			}
			line := fmt.Sprintf("  %s%s %s  %s", activeMarker, keyHint, preset.Name, OverlayDimStyle.Render(preset.Description))
			b.WriteString(OverlayNormalStyle.Render(line))
		}
		if i < len(presets)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// RenderRBACOverlay renders the RBAC permission check overlay content.
func RenderRBACOverlay(results []RBACCheckEntry, kind string) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render(fmt.Sprintf("RBAC Permissions: %s", kind)))
	b.WriteString("\n\n")

	for _, r := range results {
		indicator := OverlayWarningStyle.Render("✗") // cross mark
		if r.Allowed {
			indicator = lipgloss.NewStyle().Foreground(ThemeColor("2")).Background(SurfaceBg).Render("✓") // check mark
		}
		verb := OverlayNormalStyle.Render(fmt.Sprintf("  %-10s", r.Verb))
		b.WriteString(verb)
		b.WriteString(indicator)
		b.WriteString("\n")
	}

	return b.String()
}

// RenderBatchLabelOverlay renders the batch label/annotation editor overlay.
func RenderBatchLabelOverlay(mode int, input string, remove bool) string {
	var b strings.Builder

	kindName := "Labels"
	if mode == 1 {
		kindName = "Annotations"
	}
	action := "Add"
	if remove {
		action = "Remove"
	}

	b.WriteString(OverlayTitleStyle.Render(fmt.Sprintf("%s %s", action, kindName)))
	b.WriteString("\n\n")

	if remove {
		b.WriteString(OverlayNormalStyle.Render("  Enter key to remove:"))
	} else {
		b.WriteString(OverlayNormalStyle.Render("  Enter key=value:"))
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "  %s%s", OverlayInputStyle.Render(input), OverlayDimStyle.Render("█"))
	return b.String()
}

// RenderPodStartupOverlay renders the pod startup analysis overlay content.
func RenderPodStartupOverlay(entry PodStartupEntry) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Pod Startup Analysis"))
	b.WriteString("\n")

	// Pod info header.
	b.WriteString(OverlayNormalStyle.Render(fmt.Sprintf("  Pod:       %s", entry.PodName)))
	b.WriteString("\n")
	b.WriteString(OverlayNormalStyle.Render(fmt.Sprintf("  Namespace: %s", entry.Namespace)))
	b.WriteString("\n")
	b.WriteString(OverlayNormalStyle.Render(fmt.Sprintf("  Total:     %s", formatDuration(entry.TotalTime))))
	b.WriteString("\n\n")

	if len(entry.Phases) == 0 {
		b.WriteString(OverlayDimStyle.Render("  No startup phases available"))
		return b.String()
	}

	// Find max duration for bar scaling.
	var maxDur time.Duration
	for _, p := range entry.Phases {
		if p.Duration > maxDur {
			maxDur = p.Duration
		}
	}

	// Phase color styles.
	schedulingStyle := lipgloss.NewStyle().Foreground(ThemeColor("#7aa2f7")).Background(SurfaceBg) // blue
	pullStyle := lipgloss.NewStyle().Foreground(ThemeColor("#e0af68")).Background(SurfaceBg)       // yellow
	initStyle := lipgloss.NewStyle().Foreground(ThemeColor("#73daca")).Background(SurfaceBg)       // cyan
	containerStyle := lipgloss.NewStyle().Foreground(ThemeColor("#9ece6a")).Background(SurfaceBg)  // green
	readinessStyle := lipgloss.NewStyle().Foreground(ThemeColor("#bb9af7")).Background(SurfaceBg)  // purple
	inProgressStyle := lipgloss.NewStyle().Foreground(ThemeColor("#ff9e64")).Background(SurfaceBg) // orange
	unknownStyle := lipgloss.NewStyle().Foreground(ThemeColor("#565f89")).Background(SurfaceBg)    // dim

	// Max bar width (characters).
	barWidth := 25

	for _, phase := range entry.Phases {
		// Determine color based on phase name and status.
		style := unknownStyle
		if phase.Status == "in-progress" {
			style = inProgressStyle
		} else {
			switch {
			case strings.HasPrefix(phase.Name, "Scheduling"):
				style = schedulingStyle
			case strings.HasPrefix(phase.Name, "Image Pull"):
				style = pullStyle
			case strings.Contains(phase.Name, "Init"):
				style = initStyle
			case strings.Contains(phase.Name, "Container") || strings.HasPrefix(phase.Name, "  container:"):
				style = containerStyle
			case strings.HasPrefix(phase.Name, "Readiness"):
				style = readinessStyle
			case strings.HasPrefix(phase.Name, "  init:"):
				style = initStyle
			}
		}

		// Build duration bar.
		barLen := 0
		if maxDur > 0 {
			barLen = int(float64(barWidth) * float64(phase.Duration) / float64(maxDur))
		}
		if barLen < 1 && phase.Duration > 0 {
			barLen = 1
		}

		bar := strings.Repeat("▓", barLen) // medium shade block
		emptyBar := strings.Repeat("░", barWidth-barLen)

		// Status indicator.
		statusIndicator := ""
		switch phase.Status {
		case "in-progress":
			statusIndicator = " ○" // circle
		case "unknown":
			statusIndicator = " ?"
		}

		// Format the phase line.
		nameWidth := 20
		name := phase.Name
		if len(name) > nameWidth {
			name = name[:nameWidth-1] + "…"
		}

		durStr := formatDuration(phase.Duration)
		line := fmt.Sprintf("  %-*s %s%s %7s%s",
			nameWidth, name,
			style.Render(bar),
			OverlayDimStyle.Render(emptyBar),
			durStr,
			statusIndicator,
		)
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Legend.
	b.WriteString(OverlayDimStyle.Render("  "))
	b.WriteString(schedulingStyle.Render("▓"))
	b.WriteString(OverlayDimStyle.Render(" schedule  "))
	b.WriteString(pullStyle.Render("▓"))
	b.WriteString(OverlayDimStyle.Render(" pull  "))
	b.WriteString(initStyle.Render("▓"))
	b.WriteString(OverlayDimStyle.Render(" init  "))
	b.WriteString(containerStyle.Render("▓"))
	b.WriteString(OverlayDimStyle.Render(" start  "))
	b.WriteString(readinessStyle.Render("▓"))
	b.WriteString(OverlayDimStyle.Render(" ready"))
	b.WriteString("\n")
	b.WriteString(OverlayDimStyle.Render("  "))
	b.WriteString(inProgressStyle.Render("○"))
	b.WriteString(OverlayDimStyle.Render(" in-progress"))

	return b.String()
}

// formatDuration formats a duration into a human-readable string.
func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm%ds", mins, secs)
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%dm", hours, mins)
}

// RenderQuotaDashboardOverlay renders the namespace resource quota dashboard.
func RenderQuotaDashboardOverlay(quotas []QuotaEntry, width, height int) string {
	var b strings.Builder

	// Determine namespace for the title.
	ns := "all namespaces"
	if len(quotas) > 0 && quotas[0].Namespace != "" {
		ns = quotas[0].Namespace
	}
	b.WriteString(OverlayTitleStyle.Render(fmt.Sprintf("Resource Quotas - %s", ns)))
	b.WriteString("\n")

	// Bar width adapts to the overlay width. Reserve space for label, percentage, and values.
	barWidth := min(max(width-40, 10), 40)

	// Severity color styles.
	greenStyle := lipgloss.NewStyle().Foreground(ThemeColor("#9ece6a")).Background(SurfaceBg)
	yellowStyle := lipgloss.NewStyle().Foreground(ThemeColor("#e0af68")).Background(SurfaceBg)
	redStyle := lipgloss.NewStyle().Foreground(ThemeColor("#f7768e")).Background(SurfaceBg)

	for i, q := range quotas {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(OverlayFilterStyle.Render("  " + q.Name))
		b.WriteString("\n")

		for _, res := range q.Resources {
			// Resource name, left-aligned.
			nameLabel := fmt.Sprintf("    %-16s", res.Name)
			b.WriteString(OverlayNormalStyle.Render(nameLabel))

			// Build the usage bar.
			filled := max(min(int(res.Percent/100.0*float64(barWidth)), barWidth), 0)
			empty := barWidth - filled

			filledStr := strings.Repeat("█", filled)
			emptyStr := strings.Repeat("░", empty)

			// Color by severity.
			var barStyle lipgloss.Style
			switch {
			case res.Percent > 90:
				barStyle = redStyle
			case res.Percent > 70:
				barStyle = yellowStyle
			default:
				barStyle = greenStyle
			}

			bar := fmt.Sprintf("[%s%s]", barStyle.Render(filledStr), OverlayDimStyle.Render(emptyStr))
			pctLabel := fmt.Sprintf(" %3.0f%%", res.Percent)

			b.WriteString(bar)

			// Color the percentage label with the same severity color.
			b.WriteString(barStyle.Render(pctLabel))

			// Used/Hard values.
			valLabel := fmt.Sprintf("  %s / %s", res.Used, res.Hard)
			b.WriteString(OverlayDimStyle.Render(valLabel))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// RenderAlertsOverlay renders the Prometheus alerts overlay content.
// scroll controls the visible portion; width and height limit the overlay size.
func RenderAlertsOverlay(alerts []AlertEntry, scroll, width, height int) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Monitoring Overview — Active Alerts"))
	b.WriteString("\n")

	if len(alerts) == 0 {
		b.WriteString("\n")
		b.WriteString(OverlayDimStyle.Render("  No active alerts found"))
		b.WriteString("\n\n")
		b.WriteString(OverlayDimStyle.Render("  Prometheus was queried in well-known namespaces"))
		b.WriteString("\n")
		b.WriteString(OverlayDimStyle.Render("  (monitoring, prometheus, observability, kube-prometheus-stack)"))
		return b.String()
	}

	// Build lines for all alerts, then apply scroll window.
	lines := make([]string, 0, len(alerts)*4)
	for i, alert := range alerts {
		if i > 0 {
			lines = append(lines, "")
		}

		// Severity icon + state + alert name.
		var severityIcon string
		var severityStyle lipgloss.Style
		switch alert.Severity {
		case "critical":
			severityIcon = "●" // filled circle
			severityStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Bold(true).Background(SurfaceBg)
		case "warning":
			severityIcon = "●"
			severityStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorWarning)).Bold(true).Background(SurfaceBg)
		default:
			severityIcon = "●"
			severityStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary)).Background(SurfaceBg)
		}

		stateLabel := alert.State
		var stateStyle lipgloss.Style
		if alert.State == "firing" {
			stateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Bold(true).Background(SurfaceBg)
		} else {
			stateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorWarning)).Background(SurfaceBg)
		}

		header := fmt.Sprintf("  %s %s  %s",
			severityStyle.Render(severityIcon+" "+alert.Severity),
			stateStyle.Render("["+stateLabel+"]"),
			OverlayNormalStyle.Render(alert.Name),
		)
		lines = append(lines, header)

		// Summary.
		if alert.Summary != "" {
			lines = append(lines, "    "+OverlayDimStyle.Render(alert.Summary))
		}

		// Description (truncated if too long).
		if alert.Description != "" {
			desc := alert.Description
			maxDescLen := width - 6
			if maxDescLen > 0 && len(desc) > maxDescLen {
				desc = desc[:maxDescLen-3] + "..."
			}
			lines = append(lines, "    "+OverlayDimStyle.Render(desc))
		}

		// Since time.
		if !alert.Since.IsZero() {
			ago := formatRelativeTime(alert.Since)
			lines = append(lines, "    "+OverlayDimStyle.Render("since "+ago))
		}

		// Grafana link hint.
		if alert.GrafanaURL != "" {
			lines = append(lines, "    "+OverlayFilterStyle.Render("dashboard: "+alert.GrafanaURL))
		}
	}

	// Apply scroll window.
	// Reserve lines for header (1), blank line (1), footer (1).
	maxVisible := max(height-4, 1)

	maxScroll := max(len(lines)-maxVisible, 0)
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}

	end := min(scroll+maxVisible, len(lines))
	for i := scroll; i < end; i++ {
		b.WriteString("\n")
		b.WriteString(lines[i])
	}

	// Footer.
	b.WriteString("\n\n")
	info := fmt.Sprintf("%d alert(s)", len(alerts))
	if maxScroll > 0 {
		info += fmt.Sprintf(" | scroll %d/%d", scroll+1, maxScroll+1)
	}
	b.WriteString(OverlayDimStyle.Render(info))

	return b.String()
}
