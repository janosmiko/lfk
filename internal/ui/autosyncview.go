package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderAutoSyncOverlay renders the ArgoCD autosync configuration overlay.
func RenderAutoSyncOverlay(enabled, selfHeal, prune bool, cursor, screenWidth, screenHeight int) string {
	boxW := min(46, screenWidth-4)

	title := OverlayTitleStyle.Render("Configure AutoSync")

	type optRow struct {
		label string
		on    bool
	}
	opts := []optRow{
		{"AutoSync", enabled},
		{"Self-Heal", selfHeal},
		{"Prune", prune},
	}

	onStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Bold(true)
	offStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError))
	disabledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDimmed))

	var lines []string
	for i, opt := range opts {
		// Self-Heal and Prune are disabled when AutoSync is off.
		isDisabled := !enabled && i > 0

		indicator := offStyle.Render("OFF")
		if opt.on {
			indicator = onStyle.Render(" ON")
		}
		if isDisabled {
			indicator = disabledStyle.Render("  -")
		}

		label := fmt.Sprintf("%-14s", opt.label)
		if isDisabled {
			label = disabledStyle.Render(label)
		}
		line := fmt.Sprintf("  %s  %s", label, indicator)

		if i == cursor {
			raw := fmt.Sprintf("  %s  %s", label, indicator)
			lines = append(lines, OverlaySelectedStyle.Render(raw))
		} else {
			lines = append(lines, OverlayNormalStyle.Render(line))
		}
	}

	content := strings.Join(lines, "\n")
	hints := OverlayDimStyle.Render("space: toggle | enter: save | esc: cancel")
	body := title + "\n" + content + "\n\n" + hints

	return OverlayStyle.
		Width(boxW).
		Render(body)
}
