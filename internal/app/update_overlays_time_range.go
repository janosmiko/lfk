package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/ui"
)

// logTimeRangeFocus selects which column of the log time-range picker
// currently accepts keyboard input.
//
//	logTimeRangeFocusPresets → the preset list (default).
//	logTimeRangeFocusStart   → the Start editor (Phase 2+).
//	logTimeRangeFocusEnd     → the End editor   (Phase 2+).
type logTimeRangeFocus int

const (
	logTimeRangeFocusPresets logTimeRangeFocus = iota
	logTimeRangeFocusStart
	logTimeRangeFocusEnd
)

// openLogTimeRangeOverlay resets and opens the time-range picker. The
// preset list is captured against the current wall clock so the
// Today/Yesterday anchors are stable for the lifetime of the overlay.
// The pending range starts as the currently-active range so the picker
// shows what's already applied (users can refine, not just replace).
func (m *Model) openLogTimeRangeOverlay() {
	m.overlay = overlayLogTimeRange
	m.logTimeRangePresets = defaultPresetsAt(time.Now())
	m.logTimeRangePendingRange = m.logTimeRange
	m.logTimeRangeFocus = logTimeRangeFocusPresets
	// Select the preset whose range matches the current one when
	// possible, so the user sees their committed choice highlighted.
	// Otherwise default to the first entry. We only consider active
	// presets — the Custom… sentinel has a zero Range that would
	// spuriously match a cleared range and surface the sentinel as
	// selected, confusing the "already applied" signal.
	m.logTimeRangeCursor = 0
	if m.logTimeRange.IsActive() {
		for i, p := range m.logTimeRangePresets {
			if !p.Range.IsActive() {
				continue
			}
			if rangesEqual(p.Range, m.logTimeRange) {
				m.logTimeRangeCursor = i
				break
			}
		}
	}
}

// rangesEqual reports whether two LogTimeRange values are structurally
// equivalent. Used by the preset-picker-open handler to surface the
// already-applied range as pre-highlighted. Compared field-by-field so
// callers don't need to worry about Go's "== doesn't work for structs
// with time.Time" pitfall (time.Time is comparable, but different
// monotonic clocks break direct comparison — Equal is safer).
func rangesEqual(a, b LogTimeRange) bool {
	return pointsEqual(a.Start, b.Start) && pointsEqual(a.End, b.End)
}

func pointsEqual(a, b LogTimePoint) bool {
	if a.Kind != b.Kind {
		return false
	}
	if a.Relative != b.Relative {
		return false
	}
	return a.Absolute.Equal(b.Absolute)
}

// handleLogTimeRangeKey drives the overlay that lets the user pick or
// build a log time range. Phase 1 supports preset selection only: j/k
// move the cursor, Enter commits, `c` clears, Esc cancels. Phase 2/3
// extend this handler with editor focus transitions.
func (m Model) handleLogTimeRangeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.logTimeRangeFocus {
	case logTimeRangeFocusPresets:
		return m.handleLogTimeRangePresetsKey(msg)
	}
	return m, nil
}

// handleLogTimeRangePresetsKey handles keys while the preset list has
// focus. Separated from handleLogTimeRangeKey so Phase 2/3 editor
// focus branches stay readable as they're added.
func (m Model) handleLogTimeRangePresetsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.logTimeRangePresets = nil
		return m, nil
	case "j", "down":
		if m.logTimeRangeCursor < len(m.logTimeRangePresets)-1 {
			m.logTimeRangeCursor++
		}
		return m, nil
	case "k", "up":
		if m.logTimeRangeCursor > 0 {
			m.logTimeRangeCursor--
		}
		return m, nil
	case "g":
		m.logTimeRangeCursor = 0
		return m, nil
	case "G":
		if n := len(m.logTimeRangePresets); n > 0 {
			m.logTimeRangeCursor = n - 1
		}
		return m, nil
	case "c":
		// Clear the active range entirely.
		return m.commitLogTimeRange(LogTimeRange{})
	case "enter":
		if m.logTimeRangeCursor < 0 || m.logTimeRangeCursor >= len(m.logTimeRangePresets) {
			return m, nil
		}
		preset := m.logTimeRangePresets[m.logTimeRangeCursor]
		// The "Custom…" preset is the sentinel that unlocks the
		// editor panel in Phase 2; Phase 1 treats it as a no-op so the
		// overlay stays open without overwriting the pending range.
		if !preset.Range.IsActive() {
			// Empty range = Custom sentinel. Phase 1: dismiss without
			// applying so the user can try again (or press Esc).
			m.setStatusMessage("Custom range editor lands in Phase 2", false)
			return m, nil
		}
		return m.commitLogTimeRange(preset.Range)
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

// commitLogTimeRange applies r as the active range, closes the overlay,
// and restarts the log stream so kubectl is invoked with the new
// --since-time argument. Clearing (r.IsActive() == false) restarts the
// stream with no upper/lower bound. A silent no-op is returned when
// the new range equals the currently-active one — restarting would
// just churn the stream for no visible change.
func (m Model) commitLogTimeRange(r LogTimeRange) (tea.Model, tea.Cmd) {
	m.overlay = overlayNone
	m.logTimeRangePresets = nil
	m.logTimeRangeFocus = logTimeRangeFocusPresets

	if rangesEqual(m.logTimeRange, r) {
		return m, nil
	}
	m.logTimeRange = r
	// Legacy string is retired; clear it so it can't mislead tab save.
	m.logSinceDuration = ""

	if r.IsActive() {
		m.setStatusMessage("Time range: "+r.Display(), false)
	} else {
		m.setStatusMessage("Time range cleared", false)
	}
	return m.restartLogStreamForTimeRange()
}

// buildLogTimeRangeOverlayState projects the Model's picker state onto
// the UI-layer overlay-state struct so internal/ui never imports
// internal/app. Preview text reflects the preset currently highlighted
// (not the already-committed range) so the user can scan the list
// without losing visibility of what they'd get on Enter.
func (m Model) buildLogTimeRangeOverlayState() ui.LogTimeRangeOverlayState {
	labels := make([]string, len(m.logTimeRangePresets))
	for i, p := range m.logTimeRangePresets {
		labels[i] = p.Label
	}
	var previewRange LogTimeRange
	if m.logTimeRangeCursor >= 0 && m.logTimeRangeCursor < len(m.logTimeRangePresets) {
		previewRange = m.logTimeRangePresets[m.logTimeRangeCursor].Range
	}
	preview := previewRange.Display()
	return ui.LogTimeRangeOverlayState{
		Presets:            labels,
		Cursor:             m.logTimeRangeCursor,
		ActiveRangeDisplay: preview,
	}
}

// restartLogStreamForTimeRange cancels the current log stream and any
// in-flight history fetch, clears the buffered lines, then re-dispatches
// the stream with the new logTimeRange applied. Multi-log streams use
// restartMultiLogStream; single streams use startLogStream. The filter
// chain is preserved — only the source data is reset.
func (m Model) restartLogStreamForTimeRange() (tea.Model, tea.Cmd) {
	if m.logCancel != nil {
		m.logCancel()
		m.logCancel = nil
	}
	if m.logHistoryCancel != nil {
		m.logHistoryCancel()
		m.logHistoryCancel = nil
	}
	m.logCh = nil
	m.logLines = nil
	m.logVisibleIndices = nil
	m.logScroll = 0
	m.logCursor = 0
	m.logVisualMode = false
	m.logFollow = true
	// When a time-range window is active, kubectl will not return lines
	// older than Start regardless of --tail, so back-scroll history is
	// pointless; same story when --previous is engaged or this is a
	// multi-stream (where history-prepend isn't supported).
	m.logHasMoreHistory = !m.logTimeRange.IsActive() && !m.logPrevious && !m.logIsMulti
	m.logLoadingHistory = false

	if m.logIsMulti && len(m.logMultiItems) > 0 {
		var cmd tea.Cmd
		m, cmd = m.restartMultiLogStream()
		return m, cmd
	}
	return m, m.startLogStream()
}
