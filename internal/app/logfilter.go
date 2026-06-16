package app

import "github.com/janosmiko/lfk/internal/ui"

// logFilterActive reports whether any log filter (text or severity) is on.
func (m Model) logFilterActive() bool {
	return m.logView.filterQuery != "" || m.logView.sevThreshold > 0
}

// resetLogBuffer clears the raw stream, the displayed projection, and the
// cached per-line severity. Filter settings (query/threshold) are preserved so
// they keep applying to a freshly (re)started stream.
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
		m.logView.lines = append([]string(nil), m.logView.rawLines...)
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
			// Continuation lines (no own level) inherit the previous detected
			// severity so a stack trace stays attached to its ERROR. This applies
			// to severity filtering only; text filtering matches each line on its
			// own content (grep-like).
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

// appendRawLogLine appends one streamed line to rawLines and updates the
// displayed projection. O(1) when no filter is active; when filtering, the
// new line is appended to lines only if it passes (avoids a full rescan).
//
// In the no-filter path, lines is intentionally aliased to rawLines — this is
// the hot path and lines is only ever re-pointed (never independently appended)
// once streaming routes exclusively through appendRawLogLine.
func (m *Model) appendRawLogLine(line string) {
	m.logView.rawLines = append(m.logView.rawLines, line)
	if !m.logFilterActive() {
		m.logView.lines = m.logView.rawLines
		return
	}
	if m.logView.sevThreshold > 0 {
		r := ui.LineSeverity(line)
		m.logView.rawSev = append(m.logView.rawSev, r)
		if r == ui.SevUnknown {
			r = m.lastKnownRawSev()
		}
		if r < m.logView.sevThreshold {
			return
		}
	}
	if m.logView.filterQuery != "" && !ui.MatchLine(line, m.logView.filterQuery) {
		return
	}
	m.logView.lines = append(m.logView.lines, line)
}

// lastKnownRawSev returns the most recent non-unknown severity rank in rawSev
// (excluding the just-appended last entry), for continuation-line inheritance.
func (m *Model) lastKnownRawSev() int {
	for i := len(m.logView.rawSev) - 2; i >= 0; i-- {
		if m.logView.rawSev[i] != ui.SevUnknown {
			return m.logView.rawSev[i]
		}
	}
	return ui.SevUnknown
}

// severityStep raises (+1) or lowers (-1) the minimum-severity threshold by
// one level, clamped to [off, SevFatal], then re-projects the view.
func (m Model) severityStep(delta int) Model {
	m.logView.sevThreshold = max(0, min(m.logView.sevThreshold+delta, ui.SevFatal))
	(&m).rebuildLogView()
	return m
}

// clampLogOffsets keeps cursor/scroll/visualStart within the current lines.
func (m *Model) clampLogOffsets() {
	n := len(m.logView.lines)
	if n == 0 {
		m.logView.visualMode = false
	}
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
