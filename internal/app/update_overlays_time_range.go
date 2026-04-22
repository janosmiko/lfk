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
	now := time.Now()
	m.overlay = overlayLogTimeRange
	m.logTimeRangePresets = defaultPresetsAt(now)
	m.logTimeRangePendingRange = m.logTimeRange
	m.logTimeRangeFocus = logTimeRangeFocusPresets
	// Seed the Start/End editors with whatever is currently active.
	// The absolute-mode fallback anchors to "today 00:00" for Start
	// and "now" for End, matching the spec.
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	m.logTimeRangeStart = editorFromPoint(m.logTimeRange.Start, todayStart)
	m.logTimeRangeEnd = editorFromPoint(m.logTimeRange.End, now)
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
// build a log time range. Key dispatch fans out to a focus-specific
// sub-handler; Tab / Shift+Tab cycle focus globally.
func (m Model) handleLogTimeRangeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global Tab handling — works from any focus. Esc is handled
	// inside sub-handlers because its exact semantics differ: Esc
	// from Presets closes the overlay; Esc from an editor panel
	// drops back to Presets without committing.
	switch msg.String() {
	case "tab":
		return m.cycleLogTimeRangeFocus(+1), nil
	case "shift+tab":
		return m.cycleLogTimeRangeFocus(-1), nil
	}
	switch m.logTimeRangeFocus {
	case logTimeRangeFocusPresets:
		return m.handleLogTimeRangePresetsKey(msg)
	case logTimeRangeFocusStart, logTimeRangeFocusEnd:
		return m.handleLogTimeRangeEditorKey(msg)
	}
	return m, nil
}

// cycleLogTimeRangeFocus advances focus by `dir` (±1) over the three
// panels (Presets → Start → End → Presets). Returned Model is a value
// copy so callers can chain .handle* without worrying about pointer
// aliasing.
func (m Model) cycleLogTimeRangeFocus(dir int) Model {
	const n = 3
	next := (int(m.logTimeRangeFocus) + dir + n) % n
	m.logTimeRangeFocus = logTimeRangeFocus(next)
	return m
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
		// The "Custom…" preset (zero Range) is the sentinel that
		// shifts focus to the Start editor so the user can dial in a
		// custom offset. It does NOT commit anything.
		if !preset.Range.IsActive() {
			m.logTimeRangeFocus = logTimeRangeFocusStart
			// Preselect Relative mode so j/k immediately nudge the
			// spinner — the most common path into Custom is "I want
			// a specific N-day/hour window".
			m.logTimeRangeStart.Mode = logTimeRangeModeRelative
			m.logTimeRangeStart.RelField = logTimeRangeRelFieldHours
			return m, nil
		}
		return m.commitLogTimeRange(preset.Range)
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

// activeEditor returns a pointer to whichever Start/End editor has
// focus, or nil when focus is on the preset panel.
func (m *Model) activeEditor() *logTimeRangeEditor {
	switch m.logTimeRangeFocus {
	case logTimeRangeFocusStart:
		return &m.logTimeRangeStart
	case logTimeRangeFocusEnd:
		return &m.logTimeRangeEnd
	}
	return nil
}

// handleLogTimeRangeEditorKey routes key events inside the Start / End
// editor panels. The handler branches on the editor's Mode: Relative
// uses three d/h/m spinner fields with j/k (or +/-) to adjust and h/l
// to move between fields. Absolute lands in Phase 3.
//
// Enter commits the editor's current value to the pending range and
// advances focus (Start → End → close + apply). Esc drops focus back
// to the preset panel so the user can pick a preset again without
// losing the editor's state.
//
// Note: we accept `m Model` by value and call activeEditor() on the
// address of the local receiver so mutations land in the Model copy
// that we actually return. Delegating to a sub-method would take a
// fresh copy of `m` and silently drop any mutations the sub-method
// wrote through its (now stale) pointer.
func (m Model) handleLogTimeRangeEditorKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	ed := m.activeEditor()
	if ed == nil {
		return m, nil
	}
	// Mode-agnostic keys: mode cycling + focus / commit.
	switch msg.String() {
	case "esc":
		m.logTimeRangeFocus = logTimeRangeFocusPresets
		return m, nil
	case "enter":
		return m.commitActiveEditor()
	case "ctrl+c":
		return m.closeTabOrQuit()
	case " ":
		// Space cycles the panel's mode: Now → Relative → Absolute.
		// Keeps the editor reachable as the user learns the UI.
		ed.Mode = (ed.Mode + 1) % 3
		return m, nil
	}

	switch ed.Mode {
	case logTimeRangeModeRelative:
		applyRelativeEditorKey(ed, msg)
	case logTimeRangeModeAbsolute:
		if status := applyAbsoluteEditorKey(ed, msg); status != "" {
			m.setStatusMessage(status, false)
		}
	}
	return m, nil
}

// applyAbsoluteEditorKey mutates the absolute-datetime editor in place.
// Returns a status string ("" when nothing noteworthy happened) that
// the caller surfaces in the status bar — the spec requires a visible
// hint whenever a clamp or carry is applied so the user knows why
// their typed value changed.
func applyAbsoluteEditorKey(ed *logTimeRangeEditor, msg tea.KeyMsg) string {
	switch msg.String() {
	case "h", "left":
		ed.absFieldLeft()
		return ""
	case "l", "right":
		ed.absFieldRight()
		return ""
	case "j", "down", "-":
		return ed.absFieldAdjust(-1)
	case "k", "up", "+":
		return ed.absFieldAdjust(1)
	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		d := int(msg.String()[0] - '0')
		return ed.absFieldOverwriteDigit(d)
	}
	return ""
}

// applyRelativeEditorKey mutates the spinner editor in place. Separate
// from handleLogTimeRangeEditorKey so the method boundaries stay
// narrow and readable while keeping the mutation pointer valid (it
// lives on the caller's Model receiver for the lifetime of the handler
// call).
func applyRelativeEditorKey(ed *logTimeRangeEditor, msg tea.KeyMsg) {
	switch msg.String() {
	case "h", "left":
		ed.relFieldLeft()
	case "l", "right":
		ed.relFieldRight()
	case "j", "down", "-":
		ed.relFieldAdjust(-1)
	case "k", "up", "+":
		ed.relFieldAdjust(1)
	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		d := int(msg.String()[0] - '0')
		ed.relFieldSetDigit(d)
	}
}

// commitActiveEditor writes the active editor's value into the pending
// range and advances focus. From Start we move to End so the user can
// set the upper bound; from End we commit the full range and close.
func (m Model) commitActiveEditor() (tea.Model, tea.Cmd) {
	switch m.logTimeRangeFocus {
	case logTimeRangeFocusStart:
		m.logTimeRangePendingRange.Start = m.logTimeRangeStart.toPoint()
		m.logTimeRangeFocus = logTimeRangeFocusEnd
		return m, nil
	case logTimeRangeFocusEnd:
		m.logTimeRangePendingRange.End = m.logTimeRangeEnd.toPoint()
		return m.commitLogTimeRange(m.logTimeRangePendingRange)
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
// (Presets focus) OR the pending range composed of the editor values
// (Start/End focus) so the strip tracks whatever the user is currently
// tweaking.
func (m Model) buildLogTimeRangeOverlayState() ui.LogTimeRangeOverlayState {
	labels := make([]string, len(m.logTimeRangePresets))
	for i, p := range m.logTimeRangePresets {
		labels[i] = p.Label
	}
	preview := m.livePreviewRange().Display()
	return ui.LogTimeRangeOverlayState{
		Presets:            labels,
		Cursor:             m.logTimeRangeCursor,
		ActiveRangeDisplay: preview,
		Focus:              int(m.logTimeRangeFocus),
		Start:              editorToView(m.logTimeRangeStart),
		End:                editorToView(m.logTimeRangeEnd),
	}
}

// livePreviewRange returns the range the user would commit if they
// pressed Enter right now. While the Presets panel has focus, that's
// the preset under the cursor. While an editor has focus, the preview
// combines both editor values — the pending range composed in-place.
func (m Model) livePreviewRange() LogTimeRange {
	switch m.logTimeRangeFocus {
	case logTimeRangeFocusStart, logTimeRangeFocusEnd:
		return LogTimeRange{
			Start: m.logTimeRangeStart.toPoint(),
			End:   m.logTimeRangeEnd.toPoint(),
		}
	}
	if m.logTimeRangeCursor >= 0 && m.logTimeRangeCursor < len(m.logTimeRangePresets) {
		return m.logTimeRangePresets[m.logTimeRangeCursor].Range
	}
	return LogTimeRange{}
}

// editorToView adapts an app-layer editor into its UI mirror. Mode is
// projected to a string so the renderer doesn't need to import the
// enum from internal/app.
func editorToView(e logTimeRangeEditor) ui.LogTimeRangeEditorView {
	mode := "Now"
	switch e.Mode {
	case logTimeRangeModeRelative:
		mode = "Relative"
	case logTimeRangeModeAbsolute:
		mode = "Absolute"
	}
	return ui.LogTimeRangeEditorView{
		Mode:      mode,
		RelDays:   e.RelDays,
		RelHrs:    e.RelHrs,
		RelMin:    e.RelMin,
		RelField:  int(e.RelField),
		AbsYear:   e.AbsTime.Year(),
		AbsMonth:  int(e.AbsTime.Month()),
		AbsDay:    e.AbsTime.Day(),
		AbsHour:   e.AbsTime.Hour(),
		AbsMinute: e.AbsTime.Minute(),
		AbsSecond: e.AbsTime.Second(),
		AbsField:  int(e.AbsField),
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
