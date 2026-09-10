package app

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/ui"
)

// constraintsViewportHeight is the number of table rows visible at once —
// shared by the key handler (scroll math) and the renderer (page size) so
// they agree on what "one page" means.
func (m Model) constraintsViewportHeight() int {
	return max(m.height-4, 3)
}

func (m Model) viewConstraints() string {
	title := ui.ViewTitle(m.width, m.constraints.title+" — what constrains this object")

	var body []string
	if m.constraints.loading {
		body = append(body, ui.DimStyle.Render("  loading..."))
	} else if m.constraints.err != nil {
		body = append(body, ui.ErrorStyle.Render("  "+ui.SanitizeTerminalText(m.constraints.err.Error())))
	} else {
		body = append(body, constraintsBanner(m.constraints.report.Skipped, m.constraints.report.Failed))
		body = append(body, m.renderConstraintsRows()...)
	}

	maxLines := m.constraintsViewportHeight()
	for len(body) < maxLines {
		body = append(body, "")
	}
	borderStyle := ui.FullscreenBorderStyle(m.width, maxLines)
	content := borderStyle.Render(strings.Join(body, "\n"))

	return lipgloss.JoinVertical(lipgloss.Left, title, content, m.constraintsHintBar())
}

// constraintsBanner names the sources the scan couldn't read — one line,
// never a per-source row implying "absent". Denied and failed sources are
// named separately so an RBAC gap doesn't read as a bug, or vice versa.
func constraintsBanner(skipped, failed []string) string {
	if len(skipped) == 0 && len(failed) == 0 {
		return ""
	}
	var parts []string
	if len(skipped) > 0 {
		parts = append(parts, "denied: "+strings.Join(sanitizedConstraintNames(skipped), ", "))
	}
	if len(failed) > 0 {
		parts = append(parts, "failed: "+strings.Join(sanitizedConstraintNames(failed), ", "))
	}
	return ui.StatusWarning.Render("  skipped (" + strings.Join(parts, "; ") + ")")
}

func sanitizedConstraintNames(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = ui.SanitizeTerminalText(n)
	}
	return out
}

// renderConstraintsRows renders the header line plus one line per visible
// row. The header always leads and is never part of the scrolled range,
// which visibleRows and the cursor index without it.
func (m Model) renderConstraintsRows() []string {
	rows := m.constraints.visibleRows()
	width := max(m.width-4, 20)
	cols := constraintsColumns(width, rows)
	header := constraintsHeaderLine(cols)

	if len(rows) == 0 {
		return []string{header, ui.DimStyle.Render("  no constraints found")}
	}
	height := m.constraintsViewportHeight() - 2 // banner + header lines
	scroll := min(max(m.constraints.scroll, 0), max(len(rows)-1, 0))
	end := min(scroll+height, len(rows))

	out := make([]string, 0, max(end-scroll, 0)+1)
	out = append(out, header)
	for i := scroll; i < end; i++ {
		out = append(out, formatConstraintRow(rows[i], i == m.constraints.cursor, cols))
	}
	return out
}

// constraintColumns holds the per-row column widths, recomputed for the
// terminal width so a narrow window still renders one row per line.
type constraintColumns struct {
	source, kind, name, headroom, detail, line int
}

const (
	constraintHeaderSource   = "SOURCE"
	constraintHeaderKind     = "KIND"
	constraintHeaderName     = "NAME"
	constraintHeaderHeadroom = "HEADROOM"
	constraintHeaderDetail   = "DETAIL"
)

// constraintCells is one row's plain, sanitized cell text, shared between
// column-width measurement and row rendering so they never disagree.
type constraintCells struct {
	source, kind, name, headroom, detail string
}

func constraintRowCells(row k8s.ConstraintRow) constraintCells {
	nsName := ui.SanitizeTerminalText(row.Namespace)
	if row.Name != "" {
		if nsName != "" {
			nsName += "/"
		}
		nsName += ui.SanitizeTerminalText(row.Name)
	}
	return constraintCells{
		source:   row.Source,
		kind:     ui.SanitizeTerminalText(row.Kind),
		name:     nsName,
		headroom: row.Headroom,
		detail:   ui.SanitizeTerminalText(row.Detail),
	}
}

// constraintsColumns sizes Source/Kind/Name/Headroom to their widest cell,
// shrinks them toward their floors widest-first until Detail keeps at
// least detailFloor cells, then hands Detail whatever remains.
func constraintsColumns(width int, rows []k8s.ConstraintRow) constraintColumns {
	const (
		gaps        = 5
		detailFloor = 10
	)
	cols := constraintColumns{
		source:   lipgloss.Width(constraintHeaderSource),
		kind:     lipgloss.Width(constraintHeaderKind),
		name:     lipgloss.Width(constraintHeaderName),
		headroom: lipgloss.Width(constraintHeaderHeadroom),
		line:     width - 1,
	}
	for _, row := range rows {
		c := constraintRowCells(row)
		cols.source = max(cols.source, lipgloss.Width(c.source))
		cols.kind = max(cols.kind, lipgloss.Width(c.kind))
		cols.name = max(cols.name, lipgloss.Width(c.name))
		cols.headroom = max(cols.headroom, lipgloss.Width(c.headroom))
	}

	shrinkable := []*int{&cols.name, &cols.kind, &cols.headroom, &cols.source}
	floors := []int{6, 6, 4, 4}
	for i, col := range shrinkable {
		over := cols.source + cols.kind + cols.name + cols.headroom + gaps + detailFloor - cols.line
		if over <= 0 {
			break
		}
		*col -= min(*col-floors[i], over)
	}
	cols.detail = max(cols.line-(cols.source+cols.kind+cols.name+cols.headroom+gaps), 1)
	return cols
}

func constraintsHeaderLine(cols constraintColumns) string {
	line := fmt.Sprintf("%-*s %-*s %-*s %-*s  %s",
		cols.source, ui.Truncate(constraintHeaderSource, cols.source),
		cols.kind, ui.Truncate(constraintHeaderKind, cols.kind),
		cols.name, ui.Truncate(constraintHeaderName, cols.name),
		cols.headroom, ui.Truncate(constraintHeaderHeadroom, cols.headroom),
		ui.Truncate(constraintHeaderDetail, cols.detail),
	)
	line = ui.Truncate(line, cols.line)
	return " " + ui.DimStyle.Bold(true).Render(line)
}

func formatConstraintRow(row k8s.ConstraintRow, isCursor bool, cols constraintColumns) string {
	gutter := " "
	if isCursor {
		gutter = ui.YamlCursorIndicatorStyle.Render("▎")
	}
	c := constraintRowCells(row)
	line := fmt.Sprintf("%-*s %-*s %-*s %-*s  %s",
		cols.source, ui.Truncate(c.source, cols.source),
		cols.kind, ui.Truncate(c.kind, cols.kind),
		cols.name, ui.Truncate(c.name, cols.name),
		cols.headroom, ui.Truncate(c.headroom, cols.headroom),
		ui.Truncate(c.detail, cols.detail),
	)
	// The floors can still overrun a very narrow terminal, and a row wider
	// than the box wraps into the border.
	line = ui.Truncate(line, cols.line)
	if row.Blocking {
		line = ui.StatusFailed.Render(line)
	}
	return gutter + line
}

func (m Model) constraintsHintBar() string {
	if m.hasStatusMessage() {
		return m.renderStatusHint()
	}
	return ui.RenderHintBar([]ui.HintEntry{
		{Key: "j/k g/G", Desc: "navigate"},
		{Key: "ctrl+d/u", Desc: "half page"},
		{Key: "enter", Desc: "jump to object"},
		{Key: "q/esc", Desc: "back"},
	}, m.width)
}
