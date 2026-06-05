package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// PadToHeight ensures a string has exactly `height` newline-separated lines,
// padding with empty lines or truncating as needed.
func PadToHeight(content string, height int) string {
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// ViewTitle renders a fullscreen view's title line. The two-space lead aligns
// it with the top breadcrumb (TitleBarStyle padding + a prepended space), so
// every fullscreen title (YAML, Describe, Logs, Diff, Event Viewer, the API and
// Object Explorers) starts at the same column.
func ViewTitle(width int, text string) string {
	return TitleStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(" " + text)
}

// FullscreenBorderStyle returns the standard rounded-border style used for
// fullscreen content panels (YAML view, log viewer, explain view, etc.).
// It applies the theme's primary border color, base background, and
// standard padding, sized to fill width x height.
func FullscreenBorderStyle(width, height int) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorPrimary)).
		BorderBackground(BaseBg).
		Background(BaseBg).
		Padding(0, 1).
		Width(width - 2).
		Height(height).
		MaxHeight(height + 2)
}
