package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// FieldDocPane is what the side pane shows about one field. Err wins over Desc:
// a description left over from another field would read as if it belonged to
// the field that failed.
type FieldDocPane struct {
	Path      string
	FieldType string
	Desc      string
	Err       string
	Loading   bool
}

// RenderFieldDocPane draws the schema side pane: a title bar naming the field,
// a bordered body holding the description, and a status row that keeps the
// pane level with the viewer's hint bar. The result is exactly width columns
// and height rows, so it joins horizontally next to the main view.
//
// omitFooter drops the status row, for callers that draw one full-width footer
// under both panes. The output is then one row shorter.
//
// It mirrors RenderLogPreviewPane, which solves the same layout problem for the
// log viewer's structured preview.
func RenderFieldDocPane(width, height int, p FieldDocPane, omitFooter bool) string {
	if width < 10 {
		width = 10
	}
	if height < 3 {
		height = 3
	}

	// The pane must come out exactly height rows, or JoinHorizontal pads the
	// shorter side and the two panes drift apart. The rows it does not own:
	// the title bar, the two border rows, and the status row.
	// omitFooter drops the status row rather than growing the body, so the
	// body keeps the same size and the pane comes out one row shorter.
	contentHeight := max(height-4, 1)
	contentWidth := max(width-4, 6) // border 2 + padding 2

	titleBar := FillLinesBg(
		TitleStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(fieldDocTitle(p)),
		width, BarBg)

	bodyLines := fieldDocBody(contentWidth, p)
	if len(bodyLines) > contentHeight {
		bodyLines = bodyLines[:contentHeight]
	}
	for len(bodyLines) < contentHeight {
		bodyLines = append(bodyLines, "")
	}
	bodyContent := FillLinesBg(strings.Join(bodyLines, "\n"), contentWidth, BaseBg)
	body := FullscreenBorderStyle(width, contentHeight).Render(bodyContent)

	if omitFooter {
		return lipgloss.JoinVertical(lipgloss.Left, titleBar, body)
	}
	// An empty status row keeps the pane's height level with the viewer's
	// title + body + hint-bar layout, so JoinHorizontal pads neither side.
	footer := StatusBarBgStyle.Width(width).MaxWidth(width).MaxHeight(1).Render("")
	return lipgloss.JoinVertical(lipgloss.Left, titleBar, body, footer)
}

// fieldDocTitle names the field and its type. A deep path is cut from the
// front, because the leaf is what says which field is being read.
func fieldDocTitle(p FieldDocPane) string {
	label := p.Path
	if label == "" {
		label = "(root)"
	}
	title := " SCHEMA "
	if p.FieldType != "" {
		title += HelpKeyStyle.Render(p.FieldType) + " "
	}
	return title + label
}

// fieldDocBody wraps the text the pane is showing to the content width. Each
// line is padded out so the border encloses a solid block.
func fieldDocBody(contentWidth int, p FieldDocPane) []string {
	var text string
	switch {
	case p.Err != "":
		text = p.Err
	case p.Loading:
		text = "Loading description from the cluster schema…"
	case p.Desc != "":
		text = p.Desc
	default:
		text = "No description in the cluster schema for this field."
	}

	style := FieldDocTextStyle
	if p.Err != "" {
		style = FieldDocErrorStyle
	}

	out := make([]string, 0, 8)
	for para := range strings.SplitSeq(text, "\n") {
		if strings.TrimSpace(para) == "" {
			out = append(out, "")
			continue
		}
		for line := range strings.SplitSeq(ansi.Wordwrap(para, contentWidth, ""), "\n") {
			out = append(out, style.Render(Truncate(strings.TrimRight(line, " "), contentWidth)))
		}
	}
	return out
}

// FieldDocTitleFor is what the pane's title bar would read for a field. It lets
// callers advertise the same label elsewhere without re-rendering the pane.
func FieldDocTitleFor(p FieldDocPane) string { return fieldDocTitle(p) }
