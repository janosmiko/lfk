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
		body = append(body, constraintsBanner(m.constraints.report.Skipped))
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

// constraintsBanner names the sources RBAC (or another failure) kept the
// scan from reading — one line, never a per-source row implying "absent".
func constraintsBanner(skipped []string) string {
	if len(skipped) == 0 {
		return ""
	}
	names := make([]string, len(skipped))
	for i, s := range skipped {
		names[i] = ui.SanitizeTerminalText(s)
	}
	return ui.StatusWarning.Render("  skipped (denied or unreachable): " + strings.Join(names, ", "))
}

func (m Model) renderConstraintsRows() []string {
	rows := m.constraints.visibleRows()
	if len(rows) == 0 {
		return []string{ui.DimStyle.Render("  no constraints found")}
	}
	width := max(m.width-4, 20)
	scroll := m.constraints.scroll
	height := m.constraintsViewportHeight() - 1 // banner line
	end := min(scroll+height, len(rows))

	out := make([]string, 0, end-scroll)
	for i := scroll; i < end; i++ {
		out = append(out, formatConstraintRow(rows[i], i == m.constraints.cursor, width))
	}
	return out
}

func formatConstraintRow(row k8s.ConstraintRow, isCursor bool, width int) string {
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
	line := fmt.Sprintf("%-9s %-26s %-28s %-10s  %s",
		ui.Truncate(row.Source, 9),
		ui.Truncate(ui.SanitizeTerminalText(row.Kind), 26),
		ui.Truncate(nsName, 28),
		ui.Truncate(row.Headroom, 10),
		ui.Truncate(ui.SanitizeTerminalText(row.Detail), max(width-80, 10)),
	)
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
