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

// whichKeyGroupCells is one titled section of the panel. An empty Title renders
// the section without a header, which is what the goto popup uses.
type whichKeyGroupCells struct {
	Title string
	Cells []whichKeyCell
}

// Panel geometry, ported from neovim's which-key.nvim so the grid reads the
// same way: every column is the SAME width and the columns divide the container
// evenly, instead of each column being sized to its own widest entry with the
// leftover space dumped into the gaps.
//
// Constants mirror which-key.nvim's defaults (lua/which-key/config.lua):
// layout.width = {min = 20}, layout.spacing = 3, win.padding = {1, 2},
// win.height = {min = 4, max = 25}.
const (
	whichKeySpacing = 3  // Config.layout.spacing
	whichKeyMinColW = 20 // Config.layout.width.min
	whichKeyMinRows = 4  // Config.win.height.min
	whichKeyMaxRows = 25 // Config.win.height.max
	whichKeyPadV    = 1  // Config.win.padding[1]
	whichKeyPadH    = 2  // Config.win.padding[2]
	// whichKeySep sits between a key and its label. which-key.nvim uses "➜";
	// an ASCII arrow is used instead because that glyph is East-Asian-ambiguous
	// (double width in CJK-configured terminals), which would desync every
	// column measurement from what the terminal actually draws.
	whichKeySep = "->"
	// whichKeyBottomGap leaves exactly the status-bar row uncovered, so the
	// panel sits directly on top of it.
	whichKeyBottomGap = 1
	whichKeyMinWidth  = 20
	// whichKeyMinHeight is the real floor for at least one content row: border
	// (2) + vertical padding (2*whichKeyPadV) + the bottom gap + 1 body row.
	whichKeyMinHeight = 2 + 2*whichKeyPadV + whichKeyBottomGap + 1
)

// whichKeyGrid is the uniform cell geometry shared by every group in one panel.
// One grid for the whole panel — not one per group — is what makes the keys
// line up in a single vertical edge across group boundaries.
type whichKeyGrid struct {
	boxW  int // column width, spacing included
	boxN  int // column count
	lead  int // spaces before each column (0 for a single column)
	keyW  int // right-aligned key field
	descW int // description field; 0 drops the description entirely
}

// whichKeyGridFor ports which-key.nvim's box arithmetic (view.lua:340-344):
//
//	max_row_width = widest entry across ALL items
//	box_width     = clamp(max_row_width, layout.width.min, container)
//	box_count     = max(floor(container / (box_width + spacing)), 1)
//	box_width     = floor(container / box_count)   <- columns divide evenly
//
// The key field is measured across all items too, so the right-aligned keys
// share one edge no matter which group a row belongs to.
func whichKeyGridFor(groups []whichKeyGroupCells, container int) whichKeyGrid {
	// Two separating spaces, one on each side of the separator — the same
	// per-column spacing which-key.nvim's table applies (layout.lua:70-72).
	overhead := lipgloss.Width(whichKeySep) + 2
	maxRow, keyW := 0, 0
	for _, g := range groups {
		for _, c := range g.Cells {
			kw := lipgloss.Width(c.key)
			keyW = max(keyW, kw)
			maxRow = max(maxRow, kw+overhead+lipgloss.Width(c.desc))
		}
	}
	boxW := min(max(maxRow, whichKeyMinColW), max(container, 1))
	boxN := max(container/(boxW+whichKeySpacing), 1)
	boxW = max(container/boxN, 1)

	g := whichKeyGrid{boxW: boxW, boxN: boxN, keyW: keyW}
	if boxN > 1 {
		g.lead = whichKeySpacing
	}
	// which-key.nvim lays each box out at box_width - spacing regardless of the
	// column count (view.lua:346), and only prefixes the spacing when there is
	// more than one column (view.lua:358).
	contentW := max(boxW-whichKeySpacing, 1)
	// A key wider than the cell would starve the label; clamp it so at least
	// one column of description survives, and let writeWhichKeyCell truncate.
	if g.keyW+overhead+1 > contentW {
		g.keyW = max(contentW-overhead-1, 1)
	}
	g.descW = max(contentW-g.keyW-overhead, 0)
	return g
}

// rows returns the row count one group of n cells occupies in this grid,
// ceil(n / box_count) per view.lua:344. which-key.nvim's extra `max(..., 2)`
// floor is deliberately dropped: there it keeps the popup window from being a
// single line tall, a job whichKeyMinRows already does here, and applying it
// per group would append a blank row after every one-row section.
func (g whichKeyGrid) rows(n int) int {
	if n <= 0 || g.boxN <= 0 {
		return 0
	}
	return (n + g.boxN - 1) / g.boxN
}

// wkSpaces backs wkPad, which hands out padding without allocating at the
// widths a real panel uses.
const wkSpaces = "                                                                "

func wkPad(n int) string {
	switch {
	case n <= 0:
		return ""
	case n <= len(wkSpaces):
		return wkSpaces[:n]
	default:
		return strings.Repeat(" ", n)
	}
}

// whichKeyCellStyles are the per-render constants a cell needs. The separator
// arrives pre-styled: it never changes, so styling it per entry was a Render
// call per row of the panel for nothing.
type whichKeyCellStyles struct {
	key   lipgloss.Style
	desc  lipgloss.Style
	sep   string
	title lipgloss.Style
}

func newWhichKeyCellStyles() whichKeyCellStyles {
	key, desc, title := whichKeyStyles()
	return whichKeyCellStyles{key: key, desc: desc, sep: desc.Render(" " + whichKeySep + " "), title: title}
}

// writeWhichKeyCell lays one entry out as which-key.nvim's three-column mini
// table (view.lua:323-331): the key right-aligned, the separator, then the
// label filling the rest of the cell, padded to the full column width so the
// next column starts at a fixed offset. Writes straight into the row's builder
// rather than returning a string — the panel re-renders every frame, and a
// per-cell buffer is an allocation per entry for nothing.
func (g whichKeyGrid) writeWhichKeyCell(sb *strings.Builder, c whichKeyCell, st whichKeyCellStyles) {
	key := ui.Truncate(c.key, g.keyW)
	sb.WriteString(wkPad(g.lead + max(g.keyW-lipgloss.Width(key), 0)))
	sb.WriteString(st.key.Render(key))
	if g.descW == 0 {
		// No room for a label: the key alone is still worth showing.
		sb.WriteString(wkPad(g.boxW - whichKeySpacing - g.keyW))
		return
	}
	desc := ui.Truncate(c.desc, g.descW)
	sb.WriteString(st.sep)
	sb.WriteString(st.desc.Render(desc))
	sb.WriteString(wkPad(g.descW - lipgloss.Width(desc)))
}

// whichKeyStyles are the three styles every which-key panel path renders with.
func whichKeyStyles() (key, desc, title lipgloss.Style) {
	key = lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorSecondary)).Bold(true).Background(ui.BaseBg)
	desc = lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorFile)).Background(ui.BaseBg)
	title = lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorPrimary)).Bold(true).Background(ui.BaseBg)
	return key, desc, title
}

// whichKeyPanelGeometry derives the container width and the on-screen row
// budget every which-key layout path needs. ok is false when the terminal is
// too small to show anything.
func (m Model) whichKeyPanelGeometry() (container, availRows int, ok bool) {
	if m.width < whichKeyMinWidth || m.height < whichKeyMinHeight {
		return 0, 0, false
	}
	container = max(m.width-(2*whichKeyPadH+2), 1)
	availRows = m.height - (2 + 2*whichKeyPadV + whichKeyBottomGap)
	if availRows < 1 {
		return 0, 0, false
	}
	return container, availRows, true
}

// whichKeyPanelLayout is everything a panel render (or a scroll-bounds query)
// needs, derived once: the shared grid plus how the content sits in the
// viewport.
type whichKeyPanelLayout struct {
	grid      whichKeyGrid
	container int
	bodyRows  int // rows the content needs
	viewRows  int // rows the panel shows
	maxScroll int // largest row offset; 0 when everything fits
}

// whichKeyLayoutFor reports how the given groups fit on screen. The viewport is
// the content height clamped to which-key.nvim's win.height {min = 4, max = 25}
// and then to what the terminal has left.
//
// Counts rows without styling anything, so the key handler and the hint bar can
// ask about overflow without paying for a full render.
func (m Model) whichKeyLayoutFor(groups []whichKeyGroupCells) (whichKeyPanelLayout, bool) {
	container, availRows, ok := m.whichKeyPanelGeometry()
	if !ok {
		return whichKeyPanelLayout{}, false
	}
	lay := whichKeyPanelLayout{grid: whichKeyGridFor(groups, container), container: container}
	for _, g := range groups {
		rows := lay.grid.rows(len(g.Cells))
		if rows == 0 {
			continue
		}
		if g.Title != "" {
			lay.bodyRows++
		}
		lay.bodyRows += rows
	}
	if lay.bodyRows == 0 {
		return whichKeyPanelLayout{}, false
	}
	lay.viewRows = min(min(max(lay.bodyRows, whichKeyMinRows), whichKeyMaxRows), availRows)
	lay.maxScroll = max(lay.bodyRows-lay.viewRows, 0)
	return lay, true
}

// renderWhichKey draws the goto cheatsheet while the g prefix is armed and
// visible. Returns background unchanged when hidden or disabled.
func (m Model) renderWhichKey(background string) string {
	if !m.pendingG || !m.whichKey.shown || !ui.ConfigWhichKeyEnabled {
		return background
	}
	return m.renderWhichKeyPanel(background, m.gotoWhichKeyGroups(), m.whichKey.scroll)
}

// renderWhichKeyPanel draws the given groups as a bordered, bottom-anchored
// panel spanning the full width, styled after neovim's which-key. Groups render
// in slice order, all sharing one uniform column grid. The panel grows to fit
// its content up to whichKeyMaxRows and whatever the terminal allows; anything
// past that scrolls (scroll is a row offset, clamped here). Returns background
// unchanged when there is nothing to show or the terminal is too small.
func (m Model) renderWhichKeyPanel(background string, groups []whichKeyGroupCells, scroll int) string {
	lay, ok := m.whichKeyLayoutFor(groups)
	if !ok {
		return background
	}
	st := newWhichKeyCellStyles()
	off := min(max(scroll, 0), lay.maxScroll)
	visible := make([]string, 0, lay.viewRows)
	row := 0
	for _, g := range groups {
		visible, row = appendWhichKeyRows(visible, g, lay.grid, row, off, off+lay.viewRows, st)
	}
	if pad := lay.viewRows - len(visible); pad > 0 {
		// whichKeyMinRows can ask for more rows than the content has.
		visible = append(visible, make([]string, pad)...)
	}

	content := ui.FillLinesBg(strings.Join(visible, "\n"), lay.container, ui.BaseBg)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ui.ColorPrimary)).
		Background(ui.BaseBg).
		Padding(whichKeyPadV, whichKeyPadH).
		Render(content)

	// No DimBackground here, unlike the modal overlays: both which-key panels
	// (the leader panel and the g-prefix goto popup) are transient hints read
	// alongside the list they annotate, not modes that take over the screen.
	bg := ui.PadToHeight(background, m.height)
	return ui.PlaceOverlayBottom(m.width, m.height, whichKeyBottomGap, box, bg)
}

// appendWhichKeyRows appends the group's header and grid rows whose absolute
// row index falls inside [lo, hi), and returns the absolute index just past the
// group. Only the visible window is styled, so a scrolling panel costs a
// viewport per frame rather than the whole catalog.
//
// Fill is column-major — entries run down each column before moving right —
// matching view.lua:355 (i = (b-1) * box_height + l).
func appendWhichKeyRows(out []string, g whichKeyGroupCells, grid whichKeyGrid, row, lo, hi int, st whichKeyCellStyles) ([]string, int) {
	rows := grid.rows(len(g.Cells))
	if rows == 0 {
		return out, row
	}
	if g.Title != "" {
		if row >= lo && row < hi {
			out = append(out, st.title.Render(g.Title))
		}
		row++
	}
	for r := range rows {
		if row >= hi {
			return out, row + (rows - r)
		}
		if row < lo {
			row++
			continue
		}
		var sb strings.Builder
		sb.Grow(grid.boxW*grid.boxN + 64)
		for c := range grid.boxN {
			idx := c*rows + r
			if idx >= len(g.Cells) {
				break // nothing to the right of the last entry on this row
			}
			grid.writeWhichKeyCell(&sb, g.Cells[idx], st)
		}
		out = append(out, sb.String())
		row++
	}
	return out, row
}
