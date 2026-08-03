package app

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/janosmiko/lfk/internal/ui"
)

// whichKeyCell is one continuation key and its label in the which-key panel.
type whichKeyCell struct {
	key  string // continuation after the prefix, e.g. "p" for "gp"
	desc string
}

// which-key grid geometry. The grid grows vertically (more rows) as entries
// are added, keeping a stable column count; the count drops only when the
// terminal is too narrow to fit them.
const (
	whichKeyMaxCols = 4 // preferred column count
	whichKeyMinGap  = 3 // minimum spaces between columns
)

// whichKeyLayout describes a laid-out grid: the per-column widths, the spacing
// inserted between each pair of columns (length cols-1), the row count, and the
// total inner content width.
type whichKeyLayout struct {
	colW  []int
	gaps  []int
	rows  int
	inner int
}

// layoutWhichKey lays the cells into the largest column count (up to
// whichKeyMaxCols) that fits maxInner, sizing each column to its own widest
// entry. The columns are then spread to targetInner (bounded by maxInner) by
// widening the inter-column gaps, so the grid fills the width without any
// trailing gap past the last column. Column-major fill: entries go down each
// column.
func layoutWhichKey(plain []string, targetInner, maxInner int) whichKeyLayout {
	cols := min(whichKeyMaxCols, len(plain))
	for {
		rows := (len(plain) + cols - 1) / cols
		colW := make([]int, cols)
		sumW := 0
		for c := range cols {
			for r := range rows {
				if idx := c*rows + r; idx < len(plain) {
					colW[c] = max(colW[c], lipgloss.Width(plain[idx]))
				}
			}
			sumW += colW[c]
		}
		if cols == 1 {
			return whichKeyLayout{colW: colW, rows: rows, inner: sumW}
		}
		minWidth := sumW + whichKeyMinGap*(cols-1)
		if minWidth <= maxInner {
			// Stretch toward targetInner (never below the minimal layout, never
			// past the screen). The slack is shared across the gaps.
			want := min(max(targetInner, minWidth), maxInner)
			slack := want - sumW
			base, rem := slack/(cols-1), slack%(cols-1)
			gaps := make([]int, cols-1)
			for i := range gaps {
				gaps[i] = base
				if i < rem {
					gaps[i]++
				}
			}
			return whichKeyLayout{colW: colW, gaps: gaps, rows: rows, inner: want}
		}
		cols--
	}
}

// renderWhichKey draws the goto cheatsheet near the bottom of the screen when
// the g prefix is armed and visible, styled after neovim's which-key "modern"
// preset: a rounded-border panel of key/desc columns with uniform padding over
// a dimmed background. Returns background unchanged when hidden/disabled.
func (m Model) renderWhichKey(background string) string {
	if !m.pendingG || !m.whichKeyShown || !ui.ConfigWhichKeyEnabled {
		return background
	}
	cells := m.whichKeyCells()
	if len(cells) == 0 {
		return background
	}

	// Panel-local styles: foreground only, all on the theme base background so
	// the panel matches the theme rather than carrying the status bar's grey
	// surface. Keys use the help-key accent; descriptions use normal text.
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorSecondary)).Bold(true).Background(ui.BaseBg)
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorFile)).Background(ui.BaseBg)

	plain := make([]string, len(cells))
	styled := make([]string, len(cells))
	for i, c := range cells {
		plain[i] = c.key + " " + c.desc
		styled[i] = keyStyle.Render(c.key) + " " + descStyle.Render(c.desc)
	}

	const (
		padV     = 1  // rows of padding above and below
		padH     = 2  // columns of padding left and right
		widthPct = 60 // panel spans this percent of the screen width
	)
	// Target ~widthPct of the screen, but never wider than the screen leaves
	// room for once padding and the border are accounted for.
	chrome := 2*padH + 2
	maxInner := max(m.width-chrome, 1)
	targetInner := max(m.width*widthPct/100-chrome, 1)
	lay := layoutWhichKey(plain, targetInner, maxInner)
	cols := len(lay.colW)

	body := make([]string, lay.rows)
	for r := range lay.rows {
		var sb strings.Builder
		for c := range cols {
			idx := c*lay.rows + r // column-major: fill down each column first
			if idx < len(cells) {
				sb.WriteString(styled[idx])
				sb.WriteString(strings.Repeat(" ", max(lay.colW[c]-lipgloss.Width(plain[idx]), 0)))
			} else {
				sb.WriteString(strings.Repeat(" ", lay.colW[c]))
			}
			if c < cols-1 {
				sb.WriteString(strings.Repeat(" ", lay.gaps[c]))
			}
		}
		body[r] = sb.String()
	}
	content := ui.FillLinesBg(strings.Join(body, "\n"), lay.inner, ui.BaseBg)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ui.ColorPrimary)).
		Background(ui.BaseBg).
		Padding(padV, padH).
		Render(content)

	// Dim the screen behind the panel like the other overlays do, then lift the
	// panel a few rows off the bottom.
	bg := ui.PadToHeight(background, m.height)
	if ui.ConfigDimOverlay {
		bg = ui.DimBackground(bg, 1)
	}
	return ui.PlaceOverlayBottom(m.width, m.height, 5, box, bg)
}
