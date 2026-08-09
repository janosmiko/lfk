package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// FieldDocPaneHeight is the total line count the footnote pane claims, header
// included. It is fixed rather than sized to the text so the body above does
// not resize under the cursor as the user walks the document.
const FieldDocPaneHeight = 6

// fieldDocBodyLines is what is left for the description once the header takes
// its line.
const fieldDocBodyLines = FieldDocPaneHeight - 1

// FieldDocPane is what the footnote shows about one field. Err wins over Desc:
// a description left over from another field would read as if it belonged to
// the field that failed.
type FieldDocPane struct {
	Path      string
	FieldType string
	Desc      string
	Err       string
	Loading   bool
}

// RenderFieldDocPane draws the schema footnote: a header naming the field and
// its type, then the description. It always returns FieldDocPaneHeight lines.
func RenderFieldDocPane(width int, p FieldDocPane) string {
	if width < 1 {
		width = 1
	}

	lines := make([]string, 0, FieldDocPaneHeight)
	lines = append(lines, fieldDocHeader(width, p))

	for _, l := range fieldDocBody(width, p) {
		lines = append(lines, FieldDocTextStyle.Render(l))
	}

	return FillLinesBg(strings.Join(lines, "\n"), width, BaseBg)
}

// fieldDocHeader names the field, its type, and rules off the pane from the
// body above it.
func fieldDocHeader(width int, p FieldDocPane) string {
	label := p.Path
	if label == "" {
		label = "(root)"
	}
	if p.FieldType != "" {
		label += " " + p.FieldType
	}
	label = " " + label + " "

	head := "──" + label
	if w := ansi.StringWidth(head); w < width {
		head += strings.Repeat("─", width-w)
	}
	return FieldDocHeaderStyle.Render(Truncate(head, width))
}

// fieldDocBody returns exactly fieldDocBodyLines lines of text, wrapped to the
// width and padded out so the pane keeps its height.
func fieldDocBody(width int, p FieldDocPane) []string {
	text := ""
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

	// One leading space keeps the text off the left edge; the trailing one
	// stops a wrapped word from touching the right edge.
	textWidth := max(width-2, 1)
	wrapped := strings.Split(ansi.Wordwrap(text, textWidth, ""), "\n")

	out := make([]string, 0, fieldDocBodyLines)
	for i := range fieldDocBodyLines {
		if i >= len(wrapped) {
			out = append(out, "")
			continue
		}
		line := " " + strings.TrimRight(wrapped[i], " ")
		// The last line absorbs any overflow marker, so a long description
		// reads as cut off rather than as ending mid-sentence.
		if i == fieldDocBodyLines-1 && len(wrapped) > fieldDocBodyLines {
			line = Truncate(line, max(textWidth-1, 1)) + "…"
		}
		out = append(out, Truncate(line, width))
	}
	return out
}
