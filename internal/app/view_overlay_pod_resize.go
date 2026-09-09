package app

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/janosmiko/lfk/internal/ui"
)

// podResizeFieldLabels returns one label per editable row, in field-index
// order (matching podResizeRow.fieldAt).
func podResizeFieldLabels(st podResizeState) []string {
	labels := make([]string, 0, st.totalFields())
	for _, c := range st.containers {
		labels = append(labels,
			c.name+" CPU Req:",
			c.name+" CPU Lim:",
			c.name+" Mem Req:",
			c.name+" Mem Lim:",
		)
	}
	return labels
}

// podResizeVisibleWindow returns the [start, end) slice of field indices to
// render, keeping the focused field inside the window.
func podResizeVisibleWindow(scroll, total, maxVisible int) (int, int) {
	if total <= maxVisible {
		return 0, total
	}
	start := min(max(scroll, 0), total-maxVisible)
	return start, start + maxVisible
}

// renderPodResizeOverlay paints the Resize Pod overlay: the pod-level
// spec.resources read-only, one editable row per container per field, and a
// restart-required note for containers whose resizePolicy demands it.
func renderPodResizeOverlay(m Model) (string, int, int) {
	st := m.podResize
	w := min(60, m.width-10)
	innerW := max(w-4, 10)

	fieldLabels := podResizeFieldLabels(st)
	podLabels := make([]string, len(st.podLevel))
	for i, kv := range st.podLevel {
		podLabels[i] = "Pod " + kv.Key + ":"
	}
	labelWidth := 0
	for _, l := range fieldLabels {
		labelWidth = max(labelWidth, len(l))
	}
	for _, l := range podLabels {
		labelWidth = max(labelWidth, len(l))
	}
	labelWidth++ // trailing space before the value

	rows := make([]ui.OverlayInputRow, 0, len(st.podLevel)+min(len(fieldLabels), m.podResizeMaxVisible()))
	for i, kv := range st.podLevel {
		rows = append(rows, ui.OverlayInputRow{Label: padRight(podLabels[i], labelWidth), Input: kv.Value, ReadOnly: true})
	}

	start, end := podResizeVisibleWindow(st.scroll, len(fieldLabels), m.podResizeMaxVisible())
	for i := start; i < end; i++ {
		row := &st.containers[i/podResizeFieldCount]
		input := row.fieldAt(i % podResizeFieldCount)
		rows = append(rows, ui.OverlayInputRow{
			Label:      padRight(fieldLabels[i], labelWidth),
			Input:      input.Value,
			ShowCursor: i == st.field,
			Cursor:     input.Cursor,
		})
	}

	var notes []ui.ConfirmNote
	if len(st.restartWarn) > 0 {
		notes = append(notes, ui.ConfirmNote{
			Label: "Restart",
			Text:  strings.Join(st.restartWarn, ", ") + " will restart to apply",
			Warn:  true,
		})
	}

	content := ui.RenderOverlayInput(ui.OverlayInputConfig{
		Title:    "Resize Pod",
		Subtitle: st.name,
		Width:    innerW,
		Rows:     rows,
		Notes:    notes,
	})
	h := min(lipgloss.Height(content)+2, max(m.height-4, 3))
	return content, w, h
}
