package app

// logPreviewMinTotalWidth is the minimum terminal width at which the log
// preview side panel is rendered. Below this the toggle stays armed but the
// panel is hidden so the log stream itself remains readable.
const logPreviewMinTotalWidth = 80

// splitLogPreviewWidth divides the available terminal width between the log
// viewer and the preview side panel. Returns (logWidth, previewWidth=0) when
// the terminal is too narrow to host both.
func splitLogPreviewWidth(total int) (int, int) {
	if total < logPreviewMinTotalWidth {
		return total, 0
	}
	previewW := total * 2 / 5
	switch {
	case previewW < 30:
		previewW = 30
	case previewW > 80:
		previewW = 80
	}
	logW := total - previewW
	if logW < 50 {
		logW = 50
		previewW = total - logW
	}
	return logW, previewW
}

// logEffectiveWidth is the width the log viewer is actually rendered at,
// accounting for the optional preview side panel. Used by wrap-math helpers
// so cursor visibility and follow-mode pinning stay in sync with the renderer.
func (m *Model) logEffectiveWidth() int {
	if !m.logPreviewVisible {
		return m.width
	}
	logW, previewW := splitLogPreviewWidth(m.width)
	if previewW == 0 {
		return m.width
	}
	return logW
}

// logPreviewLine returns the log line currently targeted by the preview pane.
// The cursor is the source of truth; when it is unset (initial state, no
// stream yet) we fall back to the most recent line.
func (m *Model) logPreviewLine() string {
	if len(m.logLines) == 0 {
		return ""
	}
	idx := m.logCursor
	if idx < 0 || idx >= len(m.logLines) {
		idx = len(m.logLines) - 1
	}
	return m.logLines[idx]
}
