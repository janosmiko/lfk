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

func (m Model) renderConstraintsRows() []string {
	rows := m.constraints.visibleRows()
	if len(rows) == 0 {
		return []string{ui.DimStyle.Render("  no constraints found")}
	}
	width := max(m.width-4, 20)
	height := m.constraintsViewportHeight() - 1 // banner line
	scroll := min(max(m.constraints.scroll, 0), max(len(rows)-1, 0))
	end := min(scroll+height, len(rows))

	cols := constraintsColumns(width)
	out := make([]string, 0, max(end-scroll, 0))
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

// constraintsColumns hands cells back to the detail column, widest first,
// until the row fits. Gaps between the five columns cost 5 cells.
func constraintsColumns(width int) constraintColumns {
	const (
		gaps        = 5
		detailFloor = 10
	)
	cols := constraintColumns{source: 9, kind: 26, name: 28, headroom: 10, line: width - 1}

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

func formatConstraintRow(row k8s.ConstraintRow, isCursor bool, cols constraintColumns) string {
	gutter := " "
	if isCursor {
		gutter = ui.YamlCursorIndicatorStyle.Render("▎")
	}
	nsName := ui.SanitizeTerminalText(row.Namespace)
	if row.Name != "" {
		if nsName != "" {
			nsName += "/"
		}
		nsName += ui.SanitizeTerminalText(row.Name)
	}
	line := fmt.Sprintf("%-*s %-*s %-*s %-*s  %s",
		cols.source, ui.Truncate(row.Source, cols.source),
		cols.kind, ui.Truncate(ui.SanitizeTerminalText(row.Kind), cols.kind),
		cols.name, ui.Truncate(nsName, cols.name),
		cols.headroom, ui.Truncate(row.Headroom, cols.headroom),
		ui.Truncate(ui.SanitizeTerminalText(row.Detail), cols.detail),
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
