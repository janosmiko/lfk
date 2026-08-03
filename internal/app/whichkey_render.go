package app

import (
	"fmt"
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

// whichKeyGroupCells is one titled section of the panel. An empty Title renders
// the section without a header, which is what the goto popup uses.
type whichKeyGroupCells struct {
	Title string
	Cells []whichKeyCell
}

// Panel geometry.
const (
	whichKeyPadV      = 1  // rows of padding above and below
	whichKeyPadH      = 2  // columns of padding left and right
	whichKeyWidthPct  = 60 // panel spans this percent of the screen width
	whichKeyBottomGap = 5  // rows between the panel and the screen bottom
	whichKeyMinWidth  = 20
	// whichKeyMinHeight is the real floor for at least one content row: border
	// (2) + vertical padding (2*whichKeyPadV) + the bottom gap + 1 body row.
	// Computed from the other constants rather than hand-picked so it can't
	// drift out of sync with maxBodyRows's own arithmetic below.
	whichKeyMinHeight = 2 + 2*whichKeyPadV + whichKeyBottomGap + 1
)

// renderWhichKey draws the goto cheatsheet while the g prefix is armed and
// visible. Returns background unchanged when hidden or disabled.
func (m Model) renderWhichKey(background string) string {
	if !m.pendingG || !m.whichKey.shown || !ui.ConfigWhichKeyEnabled {
		return background
	}
	return m.renderWhichKeyPanel(background, []whichKeyGroupCells{{Cells: m.whichKeyCells()}}, "")
}

// whichKeyStyles are the three styles every which-key panel path renders
// with, factored out so the leader's pagination pass and the real render pass
// build cells identically.
func whichKeyStyles() (key, desc, title lipgloss.Style) {
	key = lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorSecondary)).Bold(true).Background(ui.BaseBg)
	desc = lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorFile)).Background(ui.BaseBg)
	title = lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorPrimary)).Bold(true).Background(ui.BaseBg)
	return key, desc, title
}

// whichKeyPanelGeometry derives the inner-width and body-row budget every
// which-key panel layout path needs — the real render pass and the leader's
// pagination pass alike — so the two can never disagree about how much fits.
// ok is false when the terminal is too small to show anything.
func (m Model) whichKeyPanelGeometry() (targetInner, maxInner, maxBodyRows int, ok bool) {
	if m.width < whichKeyMinWidth || m.height < whichKeyMinHeight {
		return 0, 0, 0, false
	}
	chrome := 2*whichKeyPadH + 2
	maxInner = max(m.width-chrome, 1)
	targetInner = max(m.width*whichKeyWidthPct/100-chrome, 1)
	// Border + vertical padding + the bottom gap all eat into the rows the body
	// may occupy.
	maxBodyRows = m.height - (2 + 2*whichKeyPadV + whichKeyBottomGap)
	if maxBodyRows < 1 {
		return 0, 0, 0, false
	}
	return targetInner, maxInner, maxBodyRows, true
}

// renderWhichKeyPanel draws the given groups as a bordered panel near the bottom
// of the screen, styled after neovim's which-key "modern" preset. Groups render
// in slice order. When the full catalog doesn't fit the screen height, the last
// group that doesn't fit is truncated cell-by-cell rather than dropped whole —
// with 40+ catalog entries the panel is routinely shorter than the content, and
// dropping whole groups can leave a short terminal showing nothing at all even
// though rows were available. A "+N more" footer discloses exactly how many
// entries were cut, unless the caller supplies its own footer (e.g. the space
// leader's page indicator) — footer then takes that row instead, and only
// yields back to "+N more" if the supplied groups still don't fit even a
// single page. Returns background unchanged when there is nothing to show, the
// terminal is too small, or nothing actually fits (a box containing only a
// footer line and no real content is worse than no panel — CRITICAL fix,
// review round 1).
func (m Model) renderWhichKeyPanel(background string, groups []whichKeyGroupCells, footer string) string {
	total := 0
	for _, g := range groups {
		total += len(g.Cells)
	}
	if total == 0 {
		return background
	}
	targetInner, maxInner, rawBodyRows, ok := m.whichKeyPanelGeometry()
	if !ok {
		return background
	}

	keyStyle, descStyle, titleStyle := whichKeyStyles()
	rendered := renderWhichKeyGroups(groups, targetInner, maxInner, keyStyle, descStyle, titleStyle)
	totalNeeded := 0
	for _, r := range rendered {
		totalNeeded += len(r.block)
	}

	// A footer row — the caller's own (e.g. the leader's page indicator) or
	// the "+N more" overflow disclosure below — is reserved exactly once,
	// here, against the true row budget. Reserving it again inside
	// fitWhichKeyGroups on top of this, or a third time in the leader's own
	// pagination pass, previously starved a titled group down to zero
	// visible cells at realistic short terminal heights (80x11, 80x12).
	maxBodyRows := rawBodyRows
	if footer != "" || totalNeeded > rawBodyRows {
		maxBodyRows--
	}
	if maxBodyRows < 1 {
		return background
	}

	body, shown, inner := fitWhichKeyGroups(rendered, maxBodyRows, targetInner, maxInner, keyStyle, descStyle, titleStyle)
	if len(body) == 0 {
		// Nothing fit even after reserving room for a footer: a box with
		// only a footer line and no real content tells the user nothing
		// useful, so skip rendering entirely instead.
		return background
	}
	switch {
	case total-shown > 0:
		hidden := total - shown
		// Width is measured on the plain string: lipgloss.Width on a styled
		// string would count the ANSI escapes.
		plainFooter := fmt.Sprintf("+%d more (%s for help)", hidden, ui.ActiveKeybindings.Help)
		body = append(body, descStyle.Render(plainFooter))
		inner = max(inner, lipgloss.Width(plainFooter))
	case footer != "":
		body = append(body, descStyle.Render(footer))
		inner = max(inner, lipgloss.Width(footer))
	}

	content := ui.FillLinesBg(strings.Join(body, "\n"), min(inner, maxInner), ui.BaseBg)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ui.ColorPrimary)).
		Background(ui.BaseBg).
		Padding(whichKeyPadV, whichKeyPadH).
		Render(content)

	bg := ui.PadToHeight(background, m.height)
	if ui.ConfigDimOverlay {
		bg = ui.DimBackground(bg, 1)
	}
	return ui.PlaceOverlayBottom(m.width, m.height, whichKeyBottomGap, box, bg)
}

// whichKeyRenderedGroup is a group's full block, rendered once up front so
// fitWhichKeyGroups can size against it without re-rendering per candidate.
type whichKeyRenderedGroup struct {
	group whichKeyGroupCells
	block []string
	width int
}

// renderWhichKeyGroups renders every non-empty group once, up front, so a
// caller sizing multiple candidate layouts (fitWhichKeyGroups's own budget
// pass, and the space leader's pagination) shares exactly the block each
// group produces instead of re-deriving row counts separately.
func renderWhichKeyGroups(groups []whichKeyGroupCells, targetInner, maxInner int, keyStyle, descStyle, titleStyle lipgloss.Style) []whichKeyRenderedGroup {
	rendered := make([]whichKeyRenderedGroup, 0, len(groups))
	for _, g := range groups {
		if len(g.Cells) == 0 {
			continue
		}
		block, w := renderWhichKeyGroup(g, keyStyle, descStyle, titleStyle, targetInner, maxInner)
		rendered = append(rendered, whichKeyRenderedGroup{g, block, w})
	}
	return rendered
}

// paginateWhichKeyGroups bins groups into pages that each fit within
// maxBodyRows, in caller-given order. Unlike renderWhichKeyPanel's own
// single-page "+N more" overflow disclosure (used by the goto popup, and as
// the leader's own fallback if a single cell can't fit at all), pagination
// never drops an entry: a group whose cells don't fit the room left on the
// current page is split — as many cells as fit go on this page (with the
// group's title, so a continuation page still says what it's continuing),
// and the rest carry over to the next page(s). This is what makes every
// catalog entry reachable via repeated space presses regardless of terminal
// height (IMPORTANT fix, review round 1 — a solo oversized page used to hand
// the renderer more than it could show, and the excess fell into "+N more"
// with no way to page to it).
func paginateWhichKeyGroups(groups []whichKeyGroupCells, maxBodyRows, targetInner, maxInner int) [][]whichKeyGroupCells {
	if maxBodyRows < 1 {
		return nil
	}
	var pages [][]whichKeyGroupCells
	var page []whichKeyGroupCells
	used := 0

	for _, g := range groups {
		if len(g.Cells) == 0 {
			continue
		}
		titleRows := 0
		if g.Title != "" {
			titleRows = 1
		}
		remaining := g.Cells
		for len(remaining) > 0 {
			budget := maxBodyRows - used
			k, rows := fitPartialGroupCellsRows(remaining, budget-titleRows, targetInner, maxInner)
			if k == 0 {
				if len(page) == 0 {
					// Not even one cell of this group fits an empty page —
					// the terminal is too small for any content at all.
					// renderWhichKeyPanel's own guards render nothing in
					// that case; stop rather than loop forever.
					if len(pages) == 0 {
						return nil
					}
					return pages
				}
				pages = append(pages, page)
				page = nil
				used = 0
				continue
			}
			page = append(page, whichKeyGroupCells{Title: g.Title, Cells: remaining[:k]})
			used += titleRows + rows
			remaining = remaining[k:]
		}
	}
	if len(page) > 0 {
		pages = append(pages, page)
	}
	return pages
}

// fitWhichKeyGroups lays out as many pre-rendered groups as fit within
// maxBodyRows, in order. maxBodyRows must already account for any footer row
// the caller needs — renderWhichKeyPanel computes that reservation exactly
// once; a second reservation here on top of it previously starved content at
// short terminal heights (CRITICAL fix, review round 1). The first group
// that doesn't fully fit is truncated to as many of its own cells as remain
// — every group after it is fully hidden.
func fitWhichKeyGroups(rendered []whichKeyRenderedGroup, maxBodyRows, targetInner, maxInner int, keyStyle, descStyle, titleStyle lipgloss.Style) (body []string, shown, inner int) {
	inner = 1
	for _, r := range rendered {
		remaining := maxBodyRows - len(body)
		if remaining <= 0 {
			break
		}
		if len(r.block) <= remaining {
			body = append(body, r.block...)
			inner = max(inner, r.width)
			shown += len(r.group.Cells)
			continue
		}
		titleRows := 0
		if r.group.Title != "" {
			titleRows = 1
		}
		if k := fitPartialGroupCells(r.group.Cells, remaining-titleRows, targetInner, maxInner); k > 0 {
			partial := whichKeyGroupCells{Title: r.group.Title, Cells: r.group.Cells[:k]}
			block, w := renderWhichKeyGroup(partial, keyStyle, descStyle, titleStyle, targetInner, maxInner)
			body = append(body, block...)
			inner = max(inner, w)
			shown += k
		}
		// Deliberately stop here rather than trying a later, smaller group in
		// the same leftover rows: groups render in caller-given priority order,
		// and skipping ahead would show a lower-priority group while hiding a
		// higher-priority one that just missed the cutoff. Costs at most a row
		// or two of unused space in the rare case where remaining-titleRows is
		// 0 (the title itself doesn't fit).
		break
	}
	return body, shown, inner
}

// fitPartialGroupCells returns the largest prefix length (0..len(cells)) whose
// laid-out rows fit within dataRows, so a group that doesn't fully fit still
// shows an accurate truncated slice instead of disappearing outright.
func fitPartialGroupCells(cells []whichKeyCell, dataRows, targetInner, maxInner int) int {
	if dataRows <= 0 {
		return 0
	}
	plain := make([]string, len(cells))
	for i, c := range cells {
		key, desc := fitCellText(c.key, c.desc, maxInner)
		plain[i] = joinCellText(key, desc)
	}
	best := 0
	for k := 1; k <= len(cells); k++ {
		if layoutWhichKey(plain[:k], targetInner, maxInner).rows <= dataRows {
			best = k
		}
	}
	return best
}

// fitPartialGroupCellsRows is fitPartialGroupCells plus the row count the
// chosen prefix actually lays out to, so a caller packing multiple chunks
// onto a page (pagination) doesn't need a second layout pass to find out how
// much of its budget the chunk consumed.
func fitPartialGroupCellsRows(cells []whichKeyCell, dataRows, targetInner, maxInner int) (k, rows int) {
	k = fitPartialGroupCells(cells, dataRows, targetInner, maxInner)
	if k == 0 {
		return 0, 0
	}
	plain := make([]string, k)
	for i, c := range cells[:k] {
		key, desc := fitCellText(c.key, c.desc, maxInner)
		plain[i] = joinCellText(key, desc)
	}
	return k, layoutWhichKey(plain, targetInner, maxInner).rows
}

// fitCellText ellipsizes desc (never key) so the rendered "key desc" text
// never exceeds maxInner columns. Real catalog labels ("Copy as
// (YAML/JSON/table)") can otherwise be wider than a narrow terminal, which
// blows the whole panel out past m.width — layoutWhichKey sizes columns to
// their widest cell with no independent width cap of its own, so the cap has
// to be applied to the text before it ever reaches layout. The key is kept
// fully visible since a which-key panel is useless if the shortcut itself is
// cut off; only if the key alone doesn't fit is it truncated as a last
// resort.
func fitCellText(key, desc string, maxInner int) (string, string) {
	if lipgloss.Width(joinCellText(key, desc)) <= maxInner {
		return key, desc
	}
	budget := maxInner - lipgloss.Width(key) - 1 // -1 for the separating space
	if budget < 1 {
		return ui.Truncate(key, maxInner), ""
	}
	return key, ui.Truncate(desc, budget)
}

// joinCellText renders a key/desc pair the way every cell is laid out: space-
// joined, or bare when desc was truncated away entirely.
func joinCellText(key, desc string) string {
	if desc == "" {
		return key
	}
	return key + " " + desc
}

// renderWhichKeyGroup lays one group out as rows, returning the rendered lines
// and the inner width they occupy. A non-empty title becomes a header row.
func renderWhichKeyGroup(g whichKeyGroupCells, keyStyle, descStyle, titleStyle lipgloss.Style, targetInner, maxInner int) ([]string, int) {
	plain := make([]string, len(g.Cells))
	styled := make([]string, len(g.Cells))
	for i, c := range g.Cells {
		key, desc := fitCellText(c.key, c.desc, maxInner)
		plain[i] = joinCellText(key, desc)
		if desc == "" {
			styled[i] = keyStyle.Render(key)
		} else {
			styled[i] = keyStyle.Render(key) + " " + descStyle.Render(desc)
		}
	}
	lay := layoutWhichKey(plain, targetInner, maxInner)
	cols := len(lay.colW)

	var out []string
	if g.Title != "" {
		out = append(out, titleStyle.Render(g.Title))
	}
	for r := range lay.rows {
		var sb strings.Builder
		for c := range cols {
			idx := c*lay.rows + r // column-major: fill down each column first
			if idx < len(g.Cells) {
				sb.WriteString(styled[idx])
				sb.WriteString(strings.Repeat(" ", max(lay.colW[c]-lipgloss.Width(plain[idx]), 0)))
			} else {
				sb.WriteString(strings.Repeat(" ", lay.colW[c]))
			}
			if c < cols-1 {
				sb.WriteString(strings.Repeat(" ", lay.gaps[c]))
			}
		}
		out = append(out, sb.String())
	}
	return out, lay.inner
}
