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
	// group tints the description so the flat, header-less list still says
	// which family an entry belongs to. Empty for the g-prefix goto popup,
	// whose entries are all the same kind of thing.
	group whichKeyGroup
	// disp is the key as drawn, resolved once per build by fillWhichKeyDisplay.
	disp string
}

// keyText is the key as drawn: chords and unprintable keys become glyphs
// ("ctrl+d" -> "⌃D", "space" -> "␣") when the icon mode allows them, and stay
// verbatim otherwise. Every width measurement and every write goes through
// this, so the grid arithmetic sees exactly what lands on screen.
func (c whichKeyCell) keyText() string {
	if c.disp != "" {
		return c.disp
	}
	return ui.KeyChordDisplay(c.key)
}

// fillWhichKeyDisplay resolves each cell's drawn key once per build. The grid
// measures the key twice (panel width, then per-column field) and the renderer
// writes it a third time; parsing the chord at each of those points is three
// passes per frame for the same answer.
func fillWhichKeyDisplay(cells []whichKeyCell) {
	for i := range cells {
		cells[i].disp = ui.KeyChordDisplay(cells[i].key)
	}
}

// Panel geometry, ported from neovim's which-key.nvim so the grid reads the
// same way: one flat list of entries flowing column-major, every column the
// SAME width, and the columns dividing the container evenly.
//
// Constants mirror which-key.nvim's defaults (lua/which-key/config.lua):
// layout.width = {min = 20}, layout.spacing = 3, win.padding = {1, 2},
// win.height = {min = 4, max = 25}.
const (
	whichKeySpacing = 3 // Config.layout.spacing
	// whichKeyMinColW is Config.layout.width.min, recalibrated for lfk's own
	// catalog rather than left at neovim's literal 20. In which-key.nvim the
	// min never actually binds: its labels ("Paste before from system
	// clipboard to previous line") run to ~50 columns, so content width alone
	// keeps column counts low. lfk's labels are terse ("Refresh view",
	// "Delete") - the widest entry in the explorer catalog is ~25 columns -
	// so the same min=20 never bound here either, and the grid packed as many
	// narrow columns as the terminal allowed (9 at 250 columns, vs neovim's
	// 4). 30 is set above that ~25-column ceiling so it actually drives
	// box_width the way neovim's min conceptually does, while still leaving
	// 2 columns at an 80-wide terminal (see whichKeyMaxCols for the wide end,
	// where even 30 alone still packs more columns than neovim's proportions
	// - verified empirically against real terminal widths, not guessed).
	whichKeyMinColW = 30
	// whichKeyMaxCols ceilings the column count on wide terminals. The width-
	// driven formula alone (box_count = container / (box_width + spacing))
	// keeps adding columns as the container grows, since lfk's boosted
	// whichKeyMinColW is still narrow enough that a 250-column terminal fits
	// 7+ of them. Neither which-key.nvim nor its config exposes an
	// equivalent knob - its long labels make one unnecessary - but lfk's
	// short ones need it to stop the panel from going wide-and-flat instead
	// of tall-and-narrow. 5 leaves whichKeyMinColW as the sole driver (never
	// binding) up to a 170-column terminal; the cap first binds at 171
	// columns, where it still leaves whichKeyMinColW's own 30-wide floor
	// exactly (33-wide columns, 30 of content once whichKeySpacing is
	// subtracted) rather than squeezing narrower.
	whichKeyMaxCols = 5
	whichKeyMinRows = 4  // Config.win.height.min
	whichKeyMaxRows = 25 // Config.win.height.max
	whichKeyPadV    = 1  // Config.win.padding[1]
	whichKeyPadH    = 2  // Config.win.padding[2]
	// whichKeyGap is the whole separation between a key and its label. neovim
	// draws Config.icons.separator between them, which is empty in the
	// reference config, leaving only the table's own one-column spacing
	// (layout.lua:70-72) — so a single space, no glyph.
	whichKeyGap = 1
	// whichKeyBottomGap leaves exactly the status-bar row uncovered, so the
	// panel sits directly on top of it.
	whichKeyBottomGap = 1
	// whichKeyOuterMargin insets the bordered box this many columns from each
	// side of the terminal, once whichKeyOuterMarginMinWidth is met, so the
	// panel reads as a floating box - the way neovim's own popup never
	// touches the editor edge - rather than a strip glued to the terminal
	// border. A fixed column count is used rather than a percentage: at a
	// 250-column terminal a percentage margin would balloon into double
	// digits per side for no visual gain, while a flat margin reads as the
	// same amount of "terminal visible outside the box" at every size that
	// gets one, which is what an inset is actually for. 4 is chosen to read
	// clearly against the border (twice whichKeyPadH's own inside-the-box
	// padding) while costing less width than a single grid column
	// (whichKeyMinColW + whichKeySpacing = 33), so applying it never forces
	// boxN down by itself.
	whichKeyOuterMargin = 4
	// whichKeyOuterMarginMinWidth is the terminal width below which the
	// outer margin collapses to zero instead of applying. Below it the grid
	// is already fighting for its first columns (see whichKeyGridFor), so
	// spending 2*whichKeyOuterMargin columns on a gutter would cost a column
	// the box could otherwise keep - full width is the better degrade than a
	// barely-there margin squeezing an already-cramped box.
	whichKeyOuterMarginMinWidth = 80
	whichKeyMinWidth            = 20
	// whichKeyMinHeight is the real floor for at least one content row: border
	// (2) + vertical padding (2*whichKeyPadV) + the bottom gap + 1 body row.
	whichKeyMinHeight = 2 + 2*whichKeyPadV + whichKeyBottomGap + 1
)

// whichKeyGrid is the cell geometry for one panel. Columns are uniformly wide,
// but the key field inside each is sized to that column's own widest key —
// otherwise one "ctrl+space" would indent every single-character key in the
// panel by ten columns.
type whichKeyGrid struct {
	boxW  int   // column width, spacing included
	boxN  int   // column count
	rowN  int   // grid rows; entries fill column-major
	lead  int   // spaces before each column, always whichKeySpacing
	keyW  []int // per column: right-aligned key field
	descW []int // per column: description field; 0 drops the description
}

// whichKeyGridFor ports which-key.nvim's box arithmetic (view.lua:340-344):
//
//	max_row_width = widest entry across ALL items
//	box_width     = clamp(max_row_width, layout.width.min, container)
//	box_count     = max(floor(container / (box_width + spacing)), 1)
//	box_width     = floor(container / box_count)   <- columns divide evenly
//	box_height    = ceil(#items / box_count)
//
// The key and description fields are then measured per column, the way
// which-key.nvim's table accumulates a width per column (layout.lua:74).
//
// which-key.nvim's extra `max(box_height, 2)` floor is deliberately dropped:
// there it keeps the popup window from being a single line tall, a job
// whichKeyMinRows already does here.
//
// One clamp is added that neovim's own arithmetic doesn't have:
// whichKeyMaxCols, applied to box_count right after it is derived from the
// container and before box_width is recomputed. See whichKeyMaxCols for why -
// short-label content defeats the width-driven column limit at wide
// terminals in a way neovim's own long-label catalog never triggers.
func whichKeyGridFor(cells []whichKeyCell, container int) whichKeyGrid {
	maxRow := 0
	for _, c := range cells {
		maxRow = max(maxRow, lipgloss.Width(c.keyText())+whichKeyGap+lipgloss.Width(c.desc))
	}
	boxW := min(max(maxRow, whichKeyMinColW), max(container, 1))
	boxN := max(container/(boxW+whichKeySpacing), 1)
	boxN = min(boxN, whichKeyMaxCols)
	boxW = max(container/boxN, 1)

	// which-key.nvim lays each box out at box_width - spacing (view.lua:346)
	// and prepends that spacing before every box, including the first, once
	// there is more than one column (view.lua:358) - so win.padding[2] alone
	// (whichKeyPadH) is not neovim's real left margin; padding[2] + spacing
	// is (2+3=5 here). lfk applies that same lead unconditionally, even at a
	// single column, which is where it deliberately parts from neovim: a
	// which-key.nvim popup shrink-wraps to its content there, so the reserved
	// spacing simply isn't part of its layout, but lfk's panel always draws
	// full width, so without a lead that budget would otherwise surface as an
	// accidental gap stranded on the right rather than a margin on the left.
	g := whichKeyGrid{boxW: boxW, boxN: boxN, lead: whichKeySpacing}
	if len(cells) == 0 {
		return g
	}
	g.rowN = (len(cells) + boxN - 1) / boxN
	contentW := max(boxW-whichKeySpacing, 1)
	g.keyW = make([]int, boxN)
	g.descW = make([]int, boxN)
	for b := range boxN {
		w := 0
		for i := b * g.rowN; i < (b+1)*g.rowN && i < len(cells); i++ {
			w = max(w, lipgloss.Width(cells[i].keyText()))
		}
		// A key wider than the cell would starve the label; clamp it so at least
		// one column of description survives, and let writeWhichKeyCell truncate.
		if w+whichKeyGap+1 > contentW {
			w = max(contentW-whichKeyGap-1, 1)
		}
		g.keyW[b] = w
		g.descW[b] = max(contentW-w-whichKeyGap, 0)
	}
	return g
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

// whichKeyCellStyles are the per-render constants a cell needs: the shared key
// style, the ungrouped description style, and one description style per
// registry group. Resolved once per render rather than per cell.
type whichKeyCellStyles struct {
	key   lipgloss.Style // uniform across every entry
	desc  lipgloss.Style // ungrouped fallback; the whole goto popup uses it
	group map[whichKeyGroup]lipgloss.Style
}

func newWhichKeyCellStyles() whichKeyCellStyles {
	key, desc := whichKeyStyles()
	return whichKeyCellStyles{key: key, desc: desc, group: whichKeyGroupStyles()}
}

// descStyle picks the accent for a cell. An unknown or empty group falls back
// to the ungrouped style, which is what keeps the g-prefix goto popup looking
// exactly as it did before groups were colored.
func (st whichKeyCellStyles) descStyle(g whichKeyGroup) lipgloss.Style {
	if s, ok := st.group[g]; ok {
		return s
	}
	return st.desc
}

// writeWhichKeyCell lays one entry out as which-key.nvim's two-column mini
// table (view.lua:323-331 with an empty separator): the key right-aligned in
// its column's key field, one space, then the label filling the rest of the
// cell, padded to the full column width so the next column starts at a fixed
// offset. Writes straight into the row's builder rather than returning a
// string — the panel re-renders every frame, and a per-cell buffer is an
// allocation per entry for nothing.
func (g whichKeyGrid) writeWhichKeyCell(sb *strings.Builder, c whichKeyCell, col int, st whichKeyCellStyles) {
	keyW, descW := g.keyW[col], g.descW[col]
	key := ui.Truncate(c.keyText(), keyW)
	sb.WriteString(wkPad(g.lead + max(keyW-lipgloss.Width(key), 0)))
	sb.WriteString(st.key.Render(key))
	if descW == 0 {
		// No room for a label: the key alone is still worth showing.
		sb.WriteString(wkPad(g.boxW - whichKeySpacing - keyW))
		return
	}
	desc := ui.Truncate(c.desc, descW)
	sb.WriteString(wkPad(whichKeyGap))
	sb.WriteString(st.descStyle(c.group).Render(desc))
	sb.WriteString(wkPad(descW - lipgloss.Width(desc)))
}

// whichKeyStyles are the two styles every which-key panel path renders with.
// The background is applied here rather than baked into the theme globals so
// the panel keeps tracking ui.BaseBg (and with it the transparency setting).
func whichKeyStyles() (key, desc lipgloss.Style) {
	return ui.WhichKeyKeyStyle.Background(ui.BaseBg), ui.WhichKeyDescStyle.Background(ui.BaseBg)
}

// whichKeyGroupStyles maps each registry group to its description accent.
// Built per render because the theme globals it reads are rebuilt by
// ApplyTheme, and a package-level copy would freeze the panel at the startup
// theme.
func whichKeyGroupStyles() map[whichKeyGroup]lipgloss.Style {
	bg := ui.BaseBg
	return map[whichKeyGroup]lipgloss.Style{
		wkActions:   ui.WhichKeyActionsStyle.Background(bg),
		wkViews:     ui.WhichKeyViewsStyle.Background(bg),
		wkFilter:    ui.WhichKeyFilterStyle.Background(bg),
		wkSelection: ui.WhichKeySelectionStyle.Background(bg),
		wkSort:      ui.WhichKeySortStyle.Background(bg),
		wkSettings:  ui.WhichKeySettingsStyle.Background(bg),
	}
}

// whichKeyPanelGeometry derives the container width and the on-screen row
// budget every which-key layout path needs. ok is false when the terminal is
// too small to show anything.
func (m Model) whichKeyPanelGeometry() (container, availRows int, ok bool) {
	if m.width < whichKeyMinWidth || m.height < whichKeyMinHeight {
		return 0, 0, false
	}
	margin := 0
	if m.width >= whichKeyOuterMarginMinWidth {
		margin = whichKeyOuterMargin
	}
	container = max(m.width-2*margin-(2*whichKeyPadH+2), 1)
	availRows = m.height - (2 + 2*whichKeyPadV + whichKeyBottomGap)
	if availRows < 1 {
		return 0, 0, false
	}
	return container, availRows, true
}

// whichKeyPanelLayout is everything a panel render (or a scroll-bounds query)
// needs, derived once: the grid plus how the content sits in the viewport.
type whichKeyPanelLayout struct {
	grid       whichKeyGrid
	container  int
	bodyRows   int // rows the content needs
	viewRows   int // rows the panel shows
	maxScroll  int // largest row offset; 0 when everything fits
	legendRows int // 1 when the group-color legend renders below the entries, else 0
}

// whichKeyLayoutFor reports how the given entries fit on screen. The viewport is
// the content height clamped to which-key.nvim's win.height {min = 4, max = 25}
// and then to what the terminal has left.
//
// Counts rows without styling anything, so the key handler and the hint bar can
// ask about overflow without paying for a full render.
func (m Model) whichKeyLayoutFor(cells []whichKeyCell) (whichKeyPanelLayout, bool) {
	container, availRows, ok := m.whichKeyPanelGeometry()
	if !ok {
		return whichKeyPanelLayout{}, false
	}
	lay := whichKeyPanelLayout{grid: whichKeyGridFor(cells, container), container: container}
	lay.bodyRows = lay.grid.rowN
	if lay.bodyRows == 0 {
		return whichKeyPanelLayout{}, false
	}
	// The legend is a fixed footer row, outside win.height's min/max clamp,
	// the same way the border and padding sit outside it. It only claims a
	// row when one is left over after reserving at least one for real
	// entries — explaining the colors must never be what pushes the colors
	// themselves off screen.
	if availRows >= 2 && whichKeyWantsLegend(cells) {
		lay.legendRows = 1
		availRows--
	}
	lay.viewRows = min(min(max(lay.bodyRows, whichKeyMinRows), whichKeyMaxRows), availRows)
	lay.maxScroll = max(lay.bodyRows-lay.viewRows, 0)
	return lay, true
}

// whichKeyWantsLegend reports whether the panel should draw the group-color
// legend: at least one cell carries a group, and NoColor mode hasn't
// collapsed every group style to plain (whichKeyGroupStyles), which would
// make a legend explain a color mapping that isn't actually on screen.
// Allocation-free by design — whichKeyLayoutFor's own alloc ceiling covers
// this path, and the actual group list is only built at render time.
func whichKeyWantsLegend(cells []whichKeyCell) bool {
	if ui.ConfigNoColor {
		return false
	}
	for _, c := range cells {
		if c.group != "" {
			return true
		}
	}
	return false
}

// whichKeyPresentGroups returns the declared groups (whichKeyGroupOrder
// order) that at least one cell in cells actually carries. The panel is
// context-aware — at the cluster picker, most action groups have no
// available entry — so the legend must reflect what today's cells contain,
// never the full declared set: a color with nothing on screen to match it is
// worse than no legend.
func whichKeyPresentGroups(cells []whichKeyCell) []whichKeyGroup {
	seen := make(map[whichKeyGroup]bool, len(whichKeyGroupOrder()))
	for _, c := range cells {
		if c.group != "" {
			seen[c.group] = true
		}
	}
	out := make([]whichKeyGroup, 0, len(seen))
	for _, g := range whichKeyGroupOrder() {
		if seen[g] {
			out = append(out, g)
		}
	}
	return out
}

// whichKeyLegendSep separates one legend entry from the next.
const whichKeyLegendSep = "  "

// whichKeyLegendLine draws each present group's name in that group's own
// description accent, so the color-to-category mapping teaches itself
// instead of staying tribal knowledge. Groups are whole units: one that
// doesn't fit in the remaining width is dropped entirely rather than
// character-truncated mid-word, the same policy ui.FormatHintPartsFit uses
// for the hint bar — a half-cut "Sett…" would teach the wrong word.
//
// The result is centered in width, computed from the PLAIN label widths
// (used), never the styled string — ANSI escapes would inflate lipgloss.Width
// and throw the centering off. Centering is applied to whatever actually fit,
// so a narrow terminal that dropped names centers the survivors, not the full
// untruncated legend.
func whichKeyLegendLine(groups []whichKeyGroup, st whichKeyCellStyles, width int) string {
	var sb strings.Builder
	used := 0
	first := true
	for _, g := range groups {
		label := string(g)
		add := lipgloss.Width(label)
		if !first {
			add += lipgloss.Width(whichKeyLegendSep)
		}
		if used+add > width {
			continue // a later, shorter name may still fit
		}
		if !first {
			sb.WriteString(whichKeyLegendSep)
		}
		sb.WriteString(st.descStyle(g).Render(label))
		used += add
		first = false
	}
	if used == 0 {
		return ""
	}
	return wkPad(max((width-used)/2, 0)) + sb.String()
}

// renderWhichKey draws the goto cheatsheet while the g prefix is armed and
// visible. Returns background unchanged when hidden or disabled.
func (m Model) renderWhichKey(background string) string {
	if !m.pendingG || !m.whichKey.shown || !ui.ConfigWhichKeyEnabled {
		return background
	}
	return m.renderWhichKeyPanel(background, m.whichKeyCells(), m.whichKey.scroll)
}

// renderWhichKeyPanel draws the given entries as a bordered, bottom-anchored
// panel spanning the full width, styled after neovim's which-key: one flat
// list, no section headers, flowing column-major through a uniform grid. The
// panel grows to fit its content up to whichKeyMaxRows and whatever the
// terminal allows; anything past that scrolls (scroll is a row offset, clamped
// here). Returns background unchanged when there is nothing to show or the
// terminal is too small.
func (m Model) renderWhichKeyPanel(background string, cells []whichKeyCell, scroll int) string {
	lay, ok := m.whichKeyLayoutFor(cells)
	if !ok {
		return background
	}
	st := newWhichKeyCellStyles()
	off := min(max(scroll, 0), lay.maxScroll)
	visible := whichKeyRows(lay.grid, cells, off, off+lay.viewRows, st)
	if pad := lay.viewRows - len(visible); pad > 0 {
		// whichKeyMinRows can ask for more rows than the content has.
		visible = append(visible, make([]string, pad)...)
	}
	// The bottom vertical padding is drawn into the content here, not left to
	// the box style below, so the legend can be appended after it and land on
	// the true last line, hugging the border, rather than floating above a
	// blank gap. Without a legend this reproduces the same blank rows the box
	// style would have added on its own.
	for range whichKeyPadV {
		visible = append(visible, "")
	}
	if lay.legendRows > 0 {
		// The legend describes the whole catalog, not just the scrolled-in
		// window, so it reads cells directly rather than the visible slice.
		visible = append(visible, whichKeyLegendLine(whichKeyPresentGroups(cells), st, lay.container))
	}

	content := ui.FillLinesBg(strings.Join(visible, "\n"), lay.container, ui.BaseBg)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ui.ColorPrimary)).
		Background(ui.BaseBg).
		PaddingTop(whichKeyPadV).
		PaddingLeft(whichKeyPadH).
		PaddingRight(whichKeyPadH).
		Render(content)

	// No DimBackground here, unlike the modal overlays: both which-key panels
	// (the leader panel and the g-prefix goto popup) are transient hints read
	// alongside the list they annotate, not modes that take over the screen.
	bg := ui.PadToHeight(background, m.height)
	return ui.PlaceOverlayBottom(m.width, m.height, whichKeyBottomGap, box, bg)
}

// whichKeyRows renders grid rows [lo, hi). Only the visible window is styled,
// so a scrolling panel costs a viewport per frame rather than the whole
// catalog.
//
// Fill is column-major — entries run down each column before moving right —
// matching view.lua:355 (i = (b-1) * box_height + l).
func whichKeyRows(grid whichKeyGrid, cells []whichKeyCell, lo, hi int, st whichKeyCellStyles) []string {
	lo = max(lo, 0)
	hi = min(hi, grid.rowN)
	if hi <= lo {
		return nil
	}
	out := make([]string, 0, hi-lo)
	for r := lo; r < hi; r++ {
		var sb strings.Builder
		sb.Grow(grid.boxW*grid.boxN + 64)
		for c := range grid.boxN {
			idx := c*grid.rowN + r
			if idx >= len(cells) {
				break // nothing to the right of the last entry on this row
			}
			grid.writeWhichKeyCell(&sb, cells[idx], c, st)
		}
		out = append(out, sb.String())
	}
	return out
}
