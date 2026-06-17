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
		sev[i] = ui.LineLogLevel(ln)
	}
	m.logView.rawSev = sev
}

// rebuildLogView recomputes lines from rawLines applying the live text filter
// and the severity threshold, then clamps cursor/scroll into the new range.
// Each line is bucketed (debug/info/warn/error) by its structured level when
// present, else by a plain-text keyword scan (defaulting to info); a line is
// shown when its bucket is at or above the threshold.
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
	for i, ln := range m.logView.rawLines {
		if m.logView.sevThreshold > 0 && m.logView.rawSev[i] < m.logView.sevThreshold {
			continue
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
		r := ui.LineLogLevel(line)
		m.logView.rawSev = append(m.logView.rawSev, r)
		if r < m.logView.sevThreshold {
			return
		}
	}
	if m.logView.filterQuery != "" && !ui.MatchLine(line, m.logView.filterQuery) {
		return
	}
	m.logView.lines = append(m.logView.lines, line)
}

// countVisibleRaw returns how many of the given raw log lines pass the active
// filters (text + severity). Used to shift the projected offsets by the VISIBLE
// delta when raw lines are trimmed or prepended while a filter is active, so a
// non-following viewport keeps its place instead of jumping.
func (m *Model) countVisibleRaw(lines []string) int {
	n := 0
	for _, ln := range lines {
		if m.logView.sevThreshold > 0 && ui.LineLogLevel(ln) < m.logView.sevThreshold {
			continue
		}
		if m.logView.filterQuery != "" && !ui.MatchLine(ln, m.logView.filterQuery) {
			continue
		}
		n++
	}
	return n
}

// severityStep raises (+1) or lowers (-1) the minimum-severity threshold by
// one level, clamped to [off, LogError], then re-projects the view. The
// threshold cycles off -> INFO+ -> WARN+ -> ERROR+ (debug is never a
// threshold, only a line bucket).
func (m Model) severityStep(delta int) Model {
	m.logView.sevThreshold = max(0, min(m.logView.sevThreshold+delta, ui.LogError))
	(&m).rebuildLogView()
	return m
}

// clampLogOffsets keeps cursor/scroll/visualStart within the current lines.
func (m *Model) clampLogOffsets() {
	n := len(m.logView.lines)
	if n == 0 {
		m.logView.visualMode = false
		m.logView.wrapTopSkip = 0
	}
	if m.logView.follow {
		if n > 0 {
			m.logView.cursor = n - 1
		} else {
			m.logView.cursor = -1
		}
		// Pin the viewport to the bottom of the filtered output. Setting
		// scroll to 0 here would show the TOP of the results until the next
		// streamed line re-pinned it (filter edits don't run ensureLogCursorVisible).
		m.logView.scroll, m.logView.wrapTopSkip = m.logMaxScrollAndSkip()
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
