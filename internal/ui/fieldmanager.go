package ui

import (
	"hash/fnv"

	"charm.land/lipgloss/v2"
)

// fieldManagerPalette are the theme colors the blame gutter cycles through.
// They are read at call time, not captured, so a runtime theme switch moves
// the gutter with everything else. In no-color mode the variables are blank
// and lipgloss emits no color.
func fieldManagerPalette() []string {
	return []string{ColorPrimary, ColorCyan, ColorPurple, ColorOrange, ColorWarning, ColorSecondary}
}

// FieldManagerStyle returns the color for one field manager. The same name
// always maps to the same color, so a manager keeps its color between
// renders and between resources. dim marks a manager that was derived from
// a parent or a subtree rather than recorded on the line itself.
func FieldManagerStyle(manager string, dim bool) lipgloss.Style {
	palette := fieldManagerPalette()
	h := fnv.New32a()
	_, _ = h.Write([]byte(manager))
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(palette[int(h.Sum32())%len(palette)]))
	if dim {
		style = style.Faint(true)
	}
	return style
}

// FieldManagerColorIndex returns which palette slot a manager lands in. The
// help and the legend use it to show the same color as the gutter.
func FieldManagerColorIndex(manager string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(manager))
	return int(h.Sum32()) % len(fieldManagerPalette())
}
