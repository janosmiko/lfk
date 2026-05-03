package app

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) handleLogKeyQuestion() Model {
	m.helpPreviousMode = modeLogs
	m.mode = modeHelp
	m.helpScroll = 0
	m.helpFilter.Clear()
	m.helpSearchActive = false
	m.helpContextMode = "Log Viewer"
	return m
}

func (m Model) handleLogKeyQ() Model {
	if m.logCancel != nil {
		m.logCancel()
		m.logCancel = nil
	}
	if m.logHistoryCancel != nil {
		m.logHistoryCancel()
		m.logHistoryCancel = nil
	}
	m.logCh = nil
	m.mode = modeExplorer
	m.logLineInput = ""
	m.logSearchQuery = ""
	m.logSearchInput.Clear()
	m.logParentKind = ""
	m.logParentName = ""
	m.logVisualMode = false
	return m
}

func (m Model) handleLogKeyJ() Model {
	m.logFollow = false
	n := consumeCountPrefix(&m.logLineInput)
	m.logCursor = min(m.logCursor+n, max(len(m.logLines)-1, 0))
	m.ensureLogCursorVisible()
	return m
}

func (m Model) handleLogKeyK() (tea.Model, tea.Cmd) {
	m.logFollow = false
	n := consumeCountPrefix(&m.logLineInput)
	m.logCursor = max(m.logCursor-n, 0)
	m.ensureLogCursorVisible()
	cmd := m.maybeLoadMoreHistory()
	return m, cmd
}

func (m Model) handleLogKeyCtrlD() Model {
	m.logFollow = false
	n := consumeCountPrefix(&m.logLineInput)
	// Round the half-page step before scaling by N: with odd content
	// heights `n*h/2` over-shoots by floor(n/2). For h=5 a single C-d
	// is 2; `2<C-d>` must land at 4 (= 2*2), not 5 (= 2*5/2).
	step := m.logContentHeight() / 2
	m.logCursor += n * step
	if m.logCursor >= len(m.logLines) {
		m.logCursor = len(m.logLines) - 1
	}
	m.ensureLogCursorVisible()
	return m
}

func (m Model) handleLogKeyCtrlU() (tea.Model, tea.Cmd) {
	m.logFollow = false
	n := consumeCountPrefix(&m.logLineInput)
	step := m.logContentHeight() / 2
	m.logCursor -= n * step
	if m.logCursor < 0 {
		m.logCursor = 0
	}
	m.ensureLogCursorVisible()
	cmd := m.maybeLoadMoreHistory()
	return m, cmd
}

func (m Model) handleLogKeyCtrlF() Model {
	m.logFollow = false
	n := consumeCountPrefix(&m.logLineInput)
	m.logCursor += n * m.logContentHeight()
	if m.logCursor >= len(m.logLines) {
		m.logCursor = len(m.logLines) - 1
	}
	m.ensureLogCursorVisible()
	return m
}

func (m Model) handleLogKeyCtrlB() (tea.Model, tea.Cmd) {
	m.logFollow = false
	n := consumeCountPrefix(&m.logLineInput)
	m.logCursor -= n * m.logContentHeight()
	if m.logCursor < 0 {
		m.logCursor = 0
	}
	m.ensureLogCursorVisible()
	cmd := m.maybeLoadMoreHistory()
	return m, cmd
}

func (m Model) handleLogKeyG() Model {
	if m.logLineInput != "" {
		lineNum, _ := strconv.Atoi(m.logLineInput)
		m.logLineInput = ""
		if lineNum > 0 {
			lineNum-- // 0-indexed
		}
		m.logCursor = min(lineNum, len(m.logLines)-1)
		m.logFollow = false
	} else {
		m.logCursor = len(m.logLines) - 1
		m.logFollow = true
	}
	m.ensureLogCursorVisible()
	return m
}

func (m Model) handleLogKeyG2() (tea.Model, tea.Cmd) {
	if m.pendingG {
		m.pendingG = false
		m.logFollow = false
		m.logLineInput = ""
		m.logCursor = 0
		m.ensureLogCursorVisible()
		cmd := m.maybeLoadMoreHistory()
		return m, cmd
	}
	m.pendingG = true
	return m, nil
}

func (m Model) handleLogKeyH() Model {
	n := consumeCountPrefix(&m.logLineInput)
	m.logVisualCurCol = max(m.logVisualCurCol-n, 0)
	return m
}

func (m Model) handleLogKeyL() Model {
	n := consumeCountPrefix(&m.logLineInput)
	m.logVisualCurCol += n
	return m
}

func (m Model) handleLogKeyDollar() Model {
	m.logLineInput = ""
	if m.logCursor >= 0 && m.logCursor < len(m.logLines) {
		lineLen := len([]rune(m.logLines[m.logCursor]))
		if lineLen > 0 {
			m.logVisualCurCol = lineLen - 1
		}
	}
	return m
}

func (m Model) handleLogKeyE() Model {
	n := consumeCountPrefix(&m.logLineInput)
	for range n {
		if m.logCursor < 0 || m.logCursor >= len(m.logLines) {
			break
		}
		lineLen := len([]rune(m.logLines[m.logCursor]))
		newCol := wordEnd(m.logLines[m.logCursor], m.logVisualCurCol)
		if newCol >= lineLen && m.logCursor < len(m.logLines)-1 {
			m.logCursor++
			newCol = wordEnd(m.logLines[m.logCursor], 0)
			nextLineLen := len([]rune(m.logLines[m.logCursor]))
			if newCol >= nextLineLen {
				newCol = max(nextLineLen-1, 0)
			}
			m.logVisualCurCol = newCol
			m.clampLogScroll()
		} else {
			m.logVisualCurCol = newCol
		}
	}
	return m
}

func (m Model) handleLogKeyB() Model {
	n := consumeCountPrefix(&m.logLineInput)
	for range n {
		if m.logCursor < 0 || m.logCursor >= len(m.logLines) {
			break
		}
		newCol := prevWordStart(m.logLines[m.logCursor], m.logVisualCurCol)
		if newCol < 0 && m.logCursor > 0 {
			m.logCursor--
			lineLen := len([]rune(m.logLines[m.logCursor]))
			newCol = max(prevWordStart(m.logLines[m.logCursor], lineLen), 0)
			m.logVisualCurCol = newCol
			m.clampLogScroll()
		} else {
			m.logVisualCurCol = max(newCol, 0)
		}
	}
	return m
}

func (m Model) handleLogKeyV() Model {
	m.logLineInput = ""
	if m.logCursor < 0 {
		m.logCursor = m.logScroll
	}
	m.logVisualMode = true
	m.logVisualType = 'V'
	m.logVisualStart = m.logCursor
	m.logVisualCol = m.logVisualCurCol
	return m
}

func (m Model) handleLogKeyV2() Model {
	m.logLineInput = ""
	if m.logCursor < 0 {
		m.logCursor = m.logScroll
	}
	m.logVisualMode = true
	m.logVisualType = 'v'
	m.logVisualStart = m.logCursor
	m.logVisualCol = m.logVisualCurCol
	return m
}

func (m Model) handleLogKeyCtrlV() Model {
	m.logLineInput = ""
	if m.logCursor < 0 {
		m.logCursor = m.logScroll
	}
	m.logVisualMode = true
	m.logVisualType = 'B'
	m.logVisualStart = m.logCursor
	m.logVisualCol = m.logVisualCurCol
	return m
}

func (m Model) handleLogKeyF() Model {
	m.logLineInput = ""
	m.logFollow = !m.logFollow
	if m.logFollow {
		m.logCursor = len(m.logLines) - 1
		m.logScroll, m.logWrapTopSkip = m.logMaxScrollAndSkip()
	}
	return m
}

func (m Model) handleLogKeyTab() Model {
	m.logLineInput = ""
	m.logWrap = !m.logWrap
	// Re-pin to the bottom on toggle: maxScroll and topSkip both depend on
	// wrap mode, so the previous values are stale. ensureLogCursorVisible
	// snaps to the follow position when m.logFollow is true and otherwise
	// just clamps + clears the sub-line skip.
	m.ensureLogCursorVisible()
	return m
}

func (m Model) handleLogKeyW() Model {
	n := consumeCountPrefix(&m.logLineInput)
	for range n {
		if m.logCursor < 0 || m.logCursor >= len(m.logLines) {
			break
		}
		lineLen := len([]rune(m.logLines[m.logCursor]))
		newCol := nextWordStart(m.logLines[m.logCursor], m.logVisualCurCol)
		if newCol >= lineLen && m.logCursor < len(m.logLines)-1 {
			m.logCursor++
			newCol = nextWordStart(m.logLines[m.logCursor], 0)
			nextLineLen := len([]rune(m.logLines[m.logCursor]))
			if newCol >= nextLineLen {
				newCol = max(nextLineLen-1, 0)
			}
			m.logVisualCurCol = newCol
			m.clampLogScroll()
		} else {
			m.logVisualCurCol = newCol
		}
	}
	return m
}

func (m Model) handleLogKeyW2() Model {
	n := consumeCountPrefix(&m.logLineInput)
	for range n {
		if m.logCursor < 0 || m.logCursor >= len(m.logLines) {
			break
		}
		lineLen := len([]rune(m.logLines[m.logCursor]))
		newCol := nextWORDStart(m.logLines[m.logCursor], m.logVisualCurCol)
		if newCol >= lineLen && m.logCursor < len(m.logLines)-1 {
			m.logCursor++
			newCol = nextWORDStart(m.logLines[m.logCursor], 0)
			nextLineLen := len([]rune(m.logLines[m.logCursor]))
			if newCol >= nextLineLen {
				newCol = max(nextLineLen-1, 0)
			}
			m.logVisualCurCol = newCol
			m.clampLogScroll()
		} else {
			m.logVisualCurCol = newCol
		}
	}
	return m
}

func (m Model) handleLogKeyE2() Model {
	n := consumeCountPrefix(&m.logLineInput)
	for range n {
		if m.logCursor < 0 || m.logCursor >= len(m.logLines) {
			break
		}
		lineLen := len([]rune(m.logLines[m.logCursor]))
		newCol := WORDEnd(m.logLines[m.logCursor], m.logVisualCurCol)
		if newCol >= lineLen && m.logCursor < len(m.logLines)-1 {
			m.logCursor++
			newCol = WORDEnd(m.logLines[m.logCursor], 0)
			nextLineLen := len([]rune(m.logLines[m.logCursor]))
			if newCol >= nextLineLen {
				newCol = max(nextLineLen-1, 0)
			}
			m.logVisualCurCol = newCol
			m.clampLogScroll()
		} else {
			m.logVisualCurCol = newCol
		}
	}
	return m
}

func (m Model) handleLogKeyB2() Model {
	n := consumeCountPrefix(&m.logLineInput)
	for range n {
		if m.logCursor < 0 || m.logCursor >= len(m.logLines) {
			break
		}
		newCol := prevWORDStart(m.logLines[m.logCursor], m.logVisualCurCol)
		if newCol < 0 && m.logCursor > 0 {
			m.logCursor--
			lineLen := len([]rune(m.logLines[m.logCursor]))
			newCol = max(prevWORDStart(m.logLines[m.logCursor], lineLen), 0)
			m.logVisualCurCol = newCol
			m.clampLogScroll()
		} else {
			m.logVisualCurCol = max(newCol, 0)
		}
	}
	return m
}

func (m Model) handleLogKeyCaret() Model {
	m.logLineInput = ""
	if m.logCursor >= 0 && m.logCursor < len(m.logLines) {
		m.logVisualCurCol = firstNonWhitespace(m.logLines[m.logCursor])
	}
	return m
}

func (m Model) handleLogKeySlash() Model {
	m.logLineInput = ""
	m.logSearchActive = true
	m.logSearchInput.Clear()
	m.logSearchHistory.reset()
	return m
}

func (m Model) handleLogKeyN() Model {
	n := consumeCountPrefix(&m.logLineInput)
	for range n {
		m.findNextLogMatch(true)
	}
	return m
}

func (m Model) handleLogKeyN2() Model {
	n := consumeCountPrefix(&m.logLineInput)
	for range n {
		m.findNextLogMatch(false)
	}
	return m
}

func (m Model) handleLogKeyP() Model {
	m.logLineInput = ""
	m.logHidePrefixes = !m.logHidePrefixes
	return m
}

func (m Model) handleLogKeyP2() Model {
	m.logLineInput = ""
	m.logPreviewVisible = !m.logPreviewVisible
	m.logPreviewScroll = 0
	// Effective viewer width changes when the panel toggles, so wrap-aware
	// scroll/skip values need recomputing for the new geometry.
	m.ensureLogCursorVisible()
	return m
}

// handleLogKeyJ2 scrolls the structured preview pane down by one body row.
// Caller is responsible for checking m.logPreviewVisible — this only runs
// when the panel is on, so it is safe to assume a valid preview width.
func (m Model) handleLogKeyJ2() Model {
	m.logLineInput = ""
	_, previewW := splitLogPreviewWidth(m.width)
	if previewW == 0 {
		return m
	}
	// LogPreviewMaxScroll's `height` arg is the outer panel height — it
	// subtracts 2 internally for the border to reach the inner content
	// height. logContentHeight already gives that inner height (it
	// accounts for the View()-time app title / tab bar reductions that
	// m.logViewHeight() can't see from Update context), so add 2 to map
	// back. Using logViewHeight here would over-count by 1 (or 2 with
	// tabs) and clip the last body rows off the user's viewport.
	previewH := m.logContentHeight() + 2
	maxScroll := ui.LogPreviewMaxScroll(m.logPreviewLine(), previewW, previewH)
	if m.logPreviewScroll < maxScroll {
		m.logPreviewScroll++
	}
	return m
}

// handleLogKeyK2 scrolls the structured preview pane up by one body row.
func (m Model) handleLogKeyK2() Model {
	m.logLineInput = ""
	if m.logPreviewScroll > 0 {
		m.logPreviewScroll--
	}
	return m
}

func (m Model) handleLogKeyHash() Model {
	m.logLineInput = ""
	m.logLineNumbers = !m.logLineNumbers
	return m
}

func (m Model) handleLogKeyS() Model {
	m.logLineInput = ""
	m.logTimestamps = !m.logTimestamps
	return m
}

func (m Model) handleLogKeyS2() (tea.Model, tea.Cmd) {
	m.logLineInput = ""
	path, err := m.saveLoadedLogs()
	if err != nil {
		m.setErrorFromErr("Log save failed: ", err)
		return m, scheduleStatusClear()
	}
	logger.Info("Saved loaded logs", "path", path)
	m.setStatusMessage("Loaded logs saved to "+path+" (copied to clipboard)", false)
	return m, tea.Batch(copyToSystemClipboard(path), scheduleStatusClear())
}

func (m Model) handleLogKeyCtrlS() (tea.Model, tea.Cmd) {
	m.logLineInput = ""
	m.setStatusMessage("Saving all logs...", false)
	return m, m.saveAllLogs()
}

func (m Model) handleLogKeyC() (tea.Model, tea.Cmd) {
	m.logLineInput = ""
	m.logPrevious = !m.logPrevious
	// --previous is incompatible with -f (follow).
	if m.logPrevious {
		m.logFollow = false
	}
	// Restart the log stream.
	if m.logCancel != nil {
		m.logCancel()
	}
	if m.logHistoryCancel != nil {
		m.logHistoryCancel()
		m.logHistoryCancel = nil
	}
	m.logLines = nil
	m.logScroll = 0
	m.logCursor = 0
	m.logVisualMode = false
	m.logTailLines = ui.ConfigLogTailLines
	m.logHasMoreHistory = !m.logPrevious && !m.logIsMulti
	m.logLoadingHistory = false
	if m.logIsMulti && len(m.logMultiItems) > 0 {
		var cmd tea.Cmd
		m, cmd = m.restartMultiLogStream()
		return m, cmd
	}
	return m, m.startLogStream()
}

func (m Model) handleLogKeyZero() Model {
	if m.logLineInput != "" {
		m.logLineInput += "0"
	} else {
		m.logVisualCurCol = 0
	}
	return m
}

func (m Model) handleLogKeyOther() (tea.Model, tea.Cmd) {
	m.logLineInput = ""
	if m.logParentKind != "" {
		// Group resource: show pod selector to switch between pods.
		m.logSavedPodName = m.actionCtx.name
		if m.logCancel != nil {
			m.logCancel()
			m.logCancel = nil
		}
		if m.logHistoryCancel != nil {
			m.logHistoryCancel()
			m.logHistoryCancel = nil
		}
		m.logCh = nil
		m.actionCtx.kind = m.logParentKind
		m.actionCtx.name = m.logParentName
		m.actionCtx.containerName = ""
		m.pendingAction = "Logs"
		m.loading = true
		m.setStatusMessage("Loading pods...", false)
		return m, m.loadPodsForLogAction()
	}
	if m.actionCtx.kind == "Pod" {
		// Single pod: load the container list, then open the filter overlay
		// once the data is ready. Setting m.overlay = overlayLogContainerSelect
		// before the load completes used to flash the empty/loading overlay
		// (and any leftover overlayItems from a prior selector use, often
		// namespaces) for the few hundred ms while kubectl returned. Mirror
		// the group-resource branch above and the existing pattern in
		// handleKeyNamespaceSelector: defer the overlay until data lands.
		m.overlayItems = nil
		m.loading = true
		m.setStatusMessage("Loading containers...", false)
		return m, m.loadContainersForLogFilter()
	}
	return m, nil
}

func (m Model) handleLogKeyCtrlC() (tea.Model, tea.Cmd) {
	if m.logCancel != nil {
		m.logCancel()
	}
	if m.logHistoryCancel != nil {
		m.logHistoryCancel()
		m.logHistoryCancel = nil
	}
	return m.closeTabOrQuit()
}

// handleLogNormalCopy copies log lines at and below the cursor (in display
// form, so timestamps and pod prefixes follow the user's toggles) to the
// clipboard. A digit prefix (e.g. `123y`) yanks that many lines; an empty
// buffer falls back to a single line.
func (m Model) handleLogNormalCopy() (tea.Model, tea.Cmd) {
	n := consumeCountPrefix(&m.logLineInput)
	if m.logCursor < 0 || m.logCursor >= len(m.logLines) {
		return m, nil
	}
	end := min(m.logCursor+n, len(m.logLines))
	parts := make([]string, 0, end-m.logCursor)
	for i := m.logCursor; i < end; i++ {
		parts = append(parts, m.logDisplayLine(i))
	}
	m.setStatusMessage(formatCopiedLines(len(parts)), false)
	return m, tea.Batch(copyToSystemClipboard(strings.Join(parts, "\n")), scheduleStatusClear())
}
