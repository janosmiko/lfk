package ui

import (
	"strings"
	"time"
)

// previewLogTimeColWidth is the fixed visual width of the time column in the
// log preview pane. Relative times are at most ~8 chars ("365d ago"). 10 gives
// a clean 2-char gap between the time and the message column.
const previewLogTimeColWidth = 10

// formatPreviewLogLine formats a single raw kubectl log line for the preview
// pane: the kubectl "[pod/...]" prefix is dropped, the RFC3339Nano timestamp
// is converted to a relative "Xs ago" form (rendered dim), and the rest of the
// body is returned as-is. sanitizeLogLine / WrapLine are NOT applied here —
// the caller handles that pipeline so all physical lines share the same path.
//
// This form (joined, no indent) is used by the existing tests that call it
// directly. Production rendering goes through previewPhysicalLines.
func formatPreviewLogLine(raw string) string {
	p := ParseLogLine(raw)
	// p.Prefix is dropped entirely.
	var sb strings.Builder
	if p.Time != "" {
		if t, err := time.Parse(time.RFC3339Nano, p.Time); err == nil {
			rel := RelativeTime(t)
			sb.WriteString(DimStyle.Render(rel))
			sb.WriteByte(' ')
		}
		// If unparseable, skip the timestamp silently.
	}
	sb.WriteString(p.Body)
	return sb.String()
}

// previewPhysicalLines builds and returns the full physical (wrapped) line
// slice for the given raw log lines at the given content width. It is a shared
// helper called by both RenderLogPreview (render) and PreviewLogPhysicalCount
// (clamp), so clamp and render always agree on the physical line count.
//
// Each raw line is rendered as a table row:
//   - A fixed-width time column (previewLogTimeColWidth visual cells) holds the
//     dim-rendered relative timestamp, padded with trailing spaces.
//   - The message starts at column previewLogTimeColWidth on the first sub-line.
//   - Continuation sub-lines (from wrapping) are indented by
//     previewLogTimeColWidth plain spaces so they align under the message column
//     (hanging indent).
//
// When the pane is too narrow (width <= previewLogTimeColWidth + 10) the table
// layout is skipped and lines are wrapped full-width to avoid layout breakage.
func previewPhysicalLines(lines []string, width int) []string {
	physical := make([]string, 0, len(lines)*2)

	// Fallback: pane too narrow for the time column — wrap full-width.
	if width <= previewLogTimeColWidth+10 {
		for _, raw := range lines {
			formatted := formatPreviewLogLine(raw)
			s := sanitizeLogLine(formatted, ConfigLogRenderAnsi)
			physical = append(physical, WrapLine(s, width)...)
		}
		return physical
	}

	msgWidth := max(width-previewLogTimeColWidth, 10)
	indent := strings.Repeat(" ", previewLogTimeColWidth)

	for _, raw := range lines {
		p := ParseLogLine(raw)

		// Compute the dim-rendered relative time (empty when no timestamp).
		var rel string
		if p.Time != "" {
			if t, err := time.Parse(time.RFC3339Nano, p.Time); err == nil {
				rel = RelativeTime(t)
			}
		}

		body := sanitizeLogLine(p.Body, ConfigLogRenderAnsi)

		subs := WrapLine(body, msgWidth)
		if len(subs) == 0 {
			subs = []string{""}
		}

		// First sub-line: time column (dim-styled, padded to previewLogTimeColWidth)
		// immediately followed by the first wrapped segment.
		timePart := padRight(DimStyle.Render(rel), previewLogTimeColWidth)
		physical = append(physical, timePart+subs[0])

		// Continuation sub-lines: hanging indent so they align under the message.
		for _, sub := range subs[1:] {
			physical = append(physical, indent+sub)
		}
	}

	return physical
}

// PreviewLogPhysicalCount returns the number of physical (post-wrap) lines that
// the preview pane body would occupy for the given raw log lines at the given
// content width. Used by clampPreviewScroll to bound the scroll offset without
// re-rendering.
func PreviewLogPhysicalCount(lines []string, width int) int {
	return len(previewPhysicalLines(lines, width))
}

// RenderLogPreview renders the right-pane live-log tail for the selected pod.
//
// When podLabel is empty the pane shows a "Select a pod to see live logs"
// placeholder and ignores lines/errMsg. When errMsg is non-empty the error is
// shown beneath the header. Otherwise the body pipeline formats each raw line
// (prefix stripped, timestamp relativised), wraps, and windows the result
// according to fromBottom:
//   - fromBottom == 0: auto-follow — shows the newest lines (tail)
//   - fromBottom > 0: shows lines fromBottom rows up from the bottom, revealing
//     older log history
//
// The output is padded to exactly height newline-separated rows.
func RenderLogPreview(lines []string, errMsg string, width, height int, podLabel string, fromBottom int) string {
	if width < 4 {
		width = 4
	}
	if height < 1 {
		height = 1
	}

	// Placeholder when no pod is selected.
	if podLabel == "" {
		return renderLogPreviewRows([]string{
			DimStyle.Render("Select a pod to see live logs"),
		}, width, height)
	}

	// Title row: "LIVE LOGS  <podLabel>" — dim+bold to match the right-pane
	// DETAILS header style (no blue bar, no leading space).
	titleText := "LIVE LOGS  " + podLabel
	titleRow := Truncate(DimStyle.Bold(true).Render(titleText), width)

	bodyHeight := max(height-1, 1)

	var bodyLines []string
	if errMsg != "" {
		errLine := Truncate(ErrorStyle.Render(errMsg), width)
		bodyLines = []string{errLine}
	} else {
		// Build physical lines: format each raw line (strip prefix, relativise
		// timestamp), sanitize, then hard-wrap to width with table-aligned columns.
		// Physical lines are already width-fit. Do NOT re-truncate or re-wrap here.
		physical := previewPhysicalLines(lines, width)
		total := len(physical)
		// Window: bottom-anchored by fromBottom.
		//   fromBottom == 0 → take the last bodyHeight lines (auto-follow).
		//   fromBottom > 0  → shift the window up by fromBottom rows.
		end := max(total-fromBottom, 0)
		start := max(end-bodyHeight, 0)
		bodyLines = physical[start:end]
	}

	// Pad body to bodyHeight rows.
	for len(bodyLines) < bodyHeight {
		bodyLines = append(bodyLines, "")
	}

	rows := make([]string, 0, height)
	rows = append(rows, titleRow)
	rows = append(rows, bodyLines...)
	return strings.Join(rows, "\n")
}

// renderLogPreviewRows renders a plain (no title bar) placeholder that fills
// height rows at the given width. Used when there is no pod to display.
func renderLogPreviewRows(content []string, width, height int) string {
	rows := make([]string, 0, height)
	for _, line := range content {
		rows = append(rows, Truncate(line, width))
	}
	for len(rows) < height {
		rows = append(rows, "")
	}
	if len(rows) > height {
		rows = rows[:height]
	}
	return strings.Join(rows, "\n")
}
