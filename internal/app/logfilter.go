package app

import "github.com/janosmiko/lfk/internal/ui"

// logFilterActive reports whether any log filter (text or severity) is on.
func (m Model) logFilterActive() bool {
	return m.logView.filterQuery != "" || m.logView.sevThreshold > 0
}

// resetLogBuffer clears the raw stream, the displayed projection, and the
// cached per-line severity. Filter settings (query/threshold) are preserved so
// they keep applying to a freshly (re)started stream.
//
//nolint:unused // wired in Task 7 of the log-filter feature
func (m *Model) resetLogBuffer() {
	m.logView.rawLines = nil
	m.logView.lines = nil
	m.logView.rawSev = nil
}

// ensureRawSev populates rawSev with per-line severity ranks. Cheap no-op when
// already in sync. Only called while a severity filter is active.
func (m *Model) ensureRawSev() {
	if len(m.logView.rawSev) == len(m.logView.rawLines) {
		return
	}
	sev := make([]int, len(m.logView.rawLines))
	for i, ln := range m.logView.rawLines {
		sev[i] = ui.LineSeverity(ln)
	}
	m.logView.rawSev = sev
}

// rebuildLogView recomputes lines from rawLines applying the live text filter
// and the severity threshold, then clamps cursor/scroll into the new range.
// Continuation lines (no detected level) inherit the previous line's severity
// so a stack trace stays attached to its ERROR.
func (m *Model) rebuildLogView() {
	if !m.logFilterActive() {
		m.logView.lines = m.logView.rawLines
		m.clampLogOffsets()
		return
	}
	if m.logView.sevThreshold > 0 {
		m.ensureRawSev()
	}
	out := make([]string, 0, len(m.logView.rawLines))
	lastRank := ui.SevUnknown
	for i, ln := range m.logView.rawLines {
		if m.logView.sevThreshold > 0 {
			r := m.logView.rawSev[i]
			if r == ui.SevUnknown {
				r = lastRank
			} else {
				lastRank = r
			}
			if r < m.logView.sevThreshold {
				continue
			}
		}
		if m.logView.filterQuery != "" && !ui.MatchLine(ln, m.logView.filterQuery) {
			continue
		}
		out = append(out, ln)
	}
	m.logView.lines = out
	m.clampLogOffsets()
}

// clampLogOffsets keeps cursor/scroll/visualStart within the current lines.
func (m *Model) clampLogOffsets() {
	n := len(m.logView.lines)
	if m.logView.follow {
		m.logView.scroll = 0
		m.logView.wrapTopSkip = 0
		if n > 0 {
			m.logView.cursor = n - 1
		} else {
			m.logView.cursor = -1
		}
		return
	}
	if m.logView.cursor >= n {
		m.logView.cursor = n - 1
	}
	if m.logView.cursor < -1 {
		m.logView.cursor = -1
	}
	if m.logView.scroll >= n {
		m.logView.scroll = max(0, n-1)
	}
	if m.logView.scroll < 0 {
		m.logView.scroll = 0
	}
	if m.logView.visualStart >= n {
		m.logView.visualStart = max(0, n-1)
	}
}
