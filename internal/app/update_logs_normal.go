package app

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
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
	if m.logView.cancel != nil {
		m.logView.cancel()
		m.logView.cancel = nil
	}
	if m.logView.historyCancel != nil {
		m.logView.historyCancel()
		m.logView.historyCancel = nil
	}
	m.logView.ch = nil
	m.mode = modeExplorer
	m.logView.lineInput = ""
	m.logView.searchQuery = ""
	m.logView.searchInput.Clear()
	m.logView.parentKind = ""
	m.logView.parentName = ""
	m.logView.visualMode = false
	return m
}

func (m Model) handleLogKeyJ() Model {
	m.logView.follow = false
	n := consumeCountPrefix(&m.logView.lineInput)
	m.logView.cursor = min(m.logView.cursor+n, max(len(m.logView.lines)-1, 0))
	m.ensureLogCursorVisible()
	return m
}

func (m Model) handleLogKeyK() (tea.Model, tea.Cmd) {
	m.logView.follow = false
	n := consumeCountPrefix(&m.logView.lineInput)
	m.logView.cursor = max(m.logView.cursor-n, 0)
	m.ensureLogCursorVisible()
	cmd := m.maybeLoadMoreHistory()
	return m, cmd
}

func (m Model) handleLogKeyCtrlD() Model {
	m.logView.follow = false
	step := vimScrollStep(&m.logView.lineInput, &m.logView.scrollOption, m.logContentHeight())
	m.logView.cursor += step
	if m.logView.cursor >= len(m.logView.lines) {
		m.logView.cursor = len(m.logView.lines) - 1
	}
	m.ensureLogCursorVisible()
	return m
}

func (m Model) handleLogKeyCtrlU() (tea.Model, tea.Cmd) {
	m.logView.follow = false
	step := vimScrollStep(&m.logView.lineInput, &m.logView.scrollOption, m.logContentHeight())
	m.logView.cursor -= step
	if m.logView.cursor < 0 {
		m.logView.cursor = 0
	}
	m.ensureLogCursorVisible()
	cmd := m.maybeLoadMoreHistory()
	return m, cmd
}

func (m Model) handleLogKeyCtrlF() Model {
	m.logView.follow = false
	n := consumeCountPrefix(&m.logView.lineInput)
	m.logView.cursor += n * m.logContentHeight()
	if m.logView.cursor >= len(m.logView.lines) {
		m.logView.cursor = len(m.logView.lines) - 1
	}
	m.ensureLogCursorVisible()
	return m
}

func (m Model) handleLogKeyCtrlB() (tea.Model, tea.Cmd) {
	m.logView.follow = false
	n := consumeCountPrefix(&m.logView.lineInput)
	m.logView.cursor -= n * m.logContentHeight()
	if m.logView.cursor < 0 {
		m.logView.cursor = 0
	}
	m.ensureLogCursorVisible()
	cmd := m.maybeLoadMoreHistory()
	return m, cmd
}

func (m Model) handleLogKeyG() Model {
	if m.logView.lineInput != "" {
		lineNum, _ := strconv.Atoi(m.logView.lineInput)
		m.logView.lineInput = ""
		if lineNum > 0 {
			lineNum-- // 0-indexed
		}
		m.logView.cursor = min(lineNum, len(m.logView.lines)-1)
		m.logView.follow = false
	} else {
		m.logView.cursor = len(m.logView.lines) - 1
		m.logView.follow = true
	}
	m.ensureLogCursorVisible()
	return m
}

func (m Model) handleLogKeyG2() (tea.Model, tea.Cmd) {
	if m.pendingG {
		m.pendingG = false
		m.logView.follow = false
		m.logView.lineInput = ""
		m.logView.cursor = 0
		m.ensureLogCursorVisible()
		cmd := m.maybeLoadMoreHistory()
		return m, cmd
	}
	m.pendingG = true
	return m, nil
}

func (m Model) handleLogKeyH() Model {
	n := consumeCountPrefix(&m.logView.lineInput)
	m.logView.visualCurCol = max(m.logView.visualCurCol-n, 0)
	return m
}

func (m Model) handleLogKeyL() Model {
	n := consumeCountPrefix(&m.logView.lineInput)
	m.logView.visualCurCol += n
	return m
}

func (m Model) handleLogKeyDollar() Model {
	m.logView.lineInput = ""
	if m.logView.cursor >= 0 && m.logView.cursor < len(m.logView.lines) {
		lineLen := len([]rune(m.logView.lines[m.logView.cursor]))
		if lineLen > 0 {
			m.logView.visualCurCol = lineLen - 1
		}
	}
	return m
}

func (m Model) handleLogKeyE() Model {
	n := consumeCountPrefix(&m.logView.lineInput)
	for range n {
		if m.logView.cursor < 0 || m.logView.cursor >= len(m.logView.lines) {
			break
		}
		lineLen := len([]rune(m.logView.lines[m.logView.cursor]))
		newCol := wordEnd(m.logView.lines[m.logView.cursor], m.logView.visualCurCol)
		if newCol >= lineLen && m.logView.cursor < len(m.logView.lines)-1 {
			m.logView.cursor++
			newCol = wordEnd(m.logView.lines[m.logView.cursor], 0)
			nextLineLen := len([]rune(m.logView.lines[m.logView.cursor]))
			if newCol >= nextLineLen {
				newCol = max(nextLineLen-1, 0)
			}
			m.logView.visualCurCol = newCol
			m.clampLogScroll()
		} else {
			m.logView.visualCurCol = newCol
		}
	}
	return m
}

func (m Model) handleLogKeyB() Model {
	n := consumeCountPrefix(&m.logView.lineInput)
	for range n {
		if m.logView.cursor < 0 || m.logView.cursor >= len(m.logView.lines) {
			break
		}
		newCol := prevWordStart(m.logView.lines[m.logView.cursor], m.logView.visualCurCol)
		if newCol < 0 && m.logView.cursor > 0 {
			m.logView.cursor--
			lineLen := len([]rune(m.logView.lines[m.logView.cursor]))
			newCol = max(prevWordStart(m.logView.lines[m.logView.cursor], lineLen), 0)
			m.logView.visualCurCol = newCol
			m.clampLogScroll()
		} else {
			m.logView.visualCurCol = max(newCol, 0)
		}
	}
	return m
}

func (m Model) handleLogKeyV() Model {
	m.logView.lineInput = ""
	if m.logView.cursor < 0 {
		m.logView.cursor = m.logView.scroll
	}
	m.logView.visualMode = true
	m.logView.visualType = 'V'
	m.logView.visualStart = m.logView.cursor
	m.logView.visualCol = m.logView.visualCurCol
	return m
}

func (m Model) handleLogKeyV2() Model {
	m.logView.lineInput = ""
	if m.logView.cursor < 0 {
		m.logView.cursor = m.logView.scroll
	}
	m.logView.visualMode = true
	m.logView.visualType = 'v'
	m.logView.visualStart = m.logView.cursor
	m.logView.visualCol = m.logView.visualCurCol
	return m
}

func (m Model) handleLogKeyCtrlV() Model {
	m.logView.lineInput = ""
	if m.logView.cursor < 0 {
		m.logView.cursor = m.logView.scroll
	}
	m.logView.visualMode = true
	m.logView.visualType = 'B'
	m.logView.visualStart = m.logView.cursor
	m.logView.visualCol = m.logView.visualCurCol
	return m
}

func (m Model) handleLogKeyF() Model {
	m.logView.lineInput = ""
	m.logView.follow = !m.logView.follow
	if m.logView.follow {
		m.logView.cursor = len(m.logView.lines) - 1
		m.logView.scroll, m.logView.wrapTopSkip = m.logMaxScrollAndSkip()
	}
	return m
}

func (m Model) handleLogKeyTab() Model {
	m.logView.lineInput = ""
	m.logView.wrap = !m.logView.wrap
	m.setViewerPref(prefLogWrap, m.logView.wrap)
	// Re-pin to the bottom on toggle: maxScroll and topSkip both depend on
	// wrap mode, so the previous values are stale. ensureLogCursorVisible
	// snaps to the follow position when m.logView.follow is true and otherwise
	// just clamps + clears the sub-line skip.
	m.ensureLogCursorVisible()
	return m
}

func (m Model) handleLogKeyW() Model {
	n := consumeCountPrefix(&m.logView.lineInput)
	for range n {
		if m.logView.cursor < 0 || m.logView.cursor >= len(m.logView.lines) {
			break
		}
		lineLen := len([]rune(m.logView.lines[m.logView.cursor]))
		newCol := nextWordStart(m.logView.lines[m.logView.cursor], m.logView.visualCurCol)
		if newCol >= lineLen && m.logView.cursor < len(m.logView.lines)-1 {
			m.logView.cursor++
			newCol = nextWordStart(m.logView.lines[m.logView.cursor], 0)
			nextLineLen := len([]rune(m.logView.lines[m.logView.cursor]))
			if newCol >= nextLineLen {
				newCol = max(nextLineLen-1, 0)
			}
			m.logView.visualCurCol = newCol
			m.clampLogScroll()
		} else {
			m.logView.visualCurCol = newCol
		}
	}
	return m
}

func (m Model) handleLogKeyW2() Model {
	n := consumeCountPrefix(&m.logView.lineInput)
	for range n {
		if m.logView.cursor < 0 || m.logView.cursor >= len(m.logView.lines) {
			break
		}
		lineLen := len([]rune(m.logView.lines[m.logView.cursor]))
		newCol := nextWORDStart(m.logView.lines[m.logView.cursor], m.logView.visualCurCol)
		if newCol >= lineLen && m.logView.cursor < len(m.logView.lines)-1 {
			m.logView.cursor++
			newCol = nextWORDStart(m.logView.lines[m.logView.cursor], 0)
			nextLineLen := len([]rune(m.logView.lines[m.logView.cursor]))
			if newCol >= nextLineLen {
				newCol = max(nextLineLen-1, 0)
			}
			m.logView.visualCurCol = newCol
			m.clampLogScroll()
		} else {
			m.logView.visualCurCol = newCol
		}
	}
	return m
}

func (m Model) handleLogKeyE2() Model {
	n := consumeCountPrefix(&m.logView.lineInput)
	for range n {
		if m.logView.cursor < 0 || m.logView.cursor >= len(m.logView.lines) {
			break
		}
		lineLen := len([]rune(m.logView.lines[m.logView.cursor]))
		newCol := WORDEnd(m.logView.lines[m.logView.cursor], m.logView.visualCurCol)
		if newCol >= lineLen && m.logView.cursor < len(m.logView.lines)-1 {
			m.logView.cursor++
			newCol = WORDEnd(m.logView.lines[m.logView.cursor], 0)
			nextLineLen := len([]rune(m.logView.lines[m.logView.cursor]))
			if newCol >= nextLineLen {
				newCol = max(nextLineLen-1, 0)
			}
			m.logView.visualCurCol = newCol
			m.clampLogScroll()
		} else {
			m.logView.visualCurCol = newCol
		}
	}
	return m
}

func (m Model) handleLogKeyB2() Model {
	n := consumeCountPrefix(&m.logView.lineInput)
	for range n {
		if m.logView.cursor < 0 || m.logView.cursor >= len(m.logView.lines) {
			break
		}
		newCol := prevWORDStart(m.logView.lines[m.logView.cursor], m.logView.visualCurCol)
		if newCol < 0 && m.logView.cursor > 0 {
			m.logView.cursor--
			lineLen := len([]rune(m.logView.lines[m.logView.cursor]))
			newCol = max(prevWORDStart(m.logView.lines[m.logView.cursor], lineLen), 0)
			m.logView.visualCurCol = newCol
			m.clampLogScroll()
		} else {
			m.logView.visualCurCol = max(newCol, 0)
		}
	}
	return m
}

func (m Model) handleLogKeyCaret() Model {
	m.logView.lineInput = ""
	if m.logView.cursor >= 0 && m.logView.cursor < len(m.logView.lines) {
		m.logView.visualCurCol = firstNonWhitespace(m.logView.lines[m.logView.cursor])
	}
	return m
}

func (m Model) handleLogKeySlash() Model {
	m.logView.lineInput = ""
	m.logView.searchActive = true
	m.logView.searchInput.Clear()
	m.logView.searchHistory.reset()
	return m
}

func (m Model) handleLogKeyN() Model {
	n := consumeCountPrefix(&m.logView.lineInput)
	for range n {
		m.findNextLogMatch(true)
	}
	return m
}

func (m Model) handleLogKeyN2() Model {
	n := consumeCountPrefix(&m.logView.lineInput)
	for range n {
		m.findNextLogMatch(false)
	}
	return m
}

func (m Model) handleLogKeyP() Model {
	m.logView.lineInput = ""
	m.logView.hidePrefixes = !m.logView.hidePrefixes
	m.setViewerPref(prefLogShowPrefixes, !m.logView.hidePrefixes)
	return m
}

func (m Model) handleLogKeyP2() Model {
	m.logView.lineInput = ""
	m.logView.previewVisible = !m.logView.previewVisible
	m.setViewerPref(prefLogShowPreview, m.logView.previewVisible)
	m.logView.previewScroll = 0
	// Effective viewer width changes when the panel toggles, so wrap-aware
	// scroll/skip values need recomputing for the new geometry.
	m.ensureLogCursorVisible()
	return m
}

// handleLogKeyJ2 scrolls the structured preview pane down by one body row.
// Caller is responsible for checking m.logView.previewVisible — this only runs
// when the panel is on, so it is safe to assume a valid preview width.
func (m Model) handleLogKeyJ2() Model {
	m.logView.lineInput = ""
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
	if m.logView.previewScroll < maxScroll {
		m.logView.previewScroll++
	}
	return m
}

// handleLogKeyK2 scrolls the structured preview pane up by one body row.
func (m Model) handleLogKeyK2() Model {
	m.logView.lineInput = ""
	if m.logView.previewScroll > 0 {
		m.logView.previewScroll--
	}
	return m
}

func (m Model) handleLogKeyHash() Model {
	m.logView.lineInput = ""
	m.logView.lineNumbers = !m.logView.lineNumbers
	return m
}

func (m Model) handleLogKeyS() Model {
	m.logView.lineInput = ""
	m.logView.timestamps = !m.logView.timestamps
	m.setViewerPref(prefLogShowTimestamps, m.logView.timestamps)
	return m
}

func (m Model) handleLogKeyS2() (tea.Model, tea.Cmd) {
	m.logView.lineInput = ""
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
	m.logView.lineInput = ""
	m.setStatusMessage("Saving all logs...", false)
	return m, m.saveAllLogs()
}

func (m Model) handleLogKeyC() (tea.Model, tea.Cmd) {
	m.logView.lineInput = ""
	m.logView.previous = !m.logView.previous
	// --previous is incompatible with -f (follow).
	if m.logView.previous {
		m.logView.follow = false
	}
	// Restart the log stream.
	if m.logView.cancel != nil {
		m.logView.cancel()
	}
	if m.logView.historyCancel != nil {
		m.logView.historyCancel()
		m.logView.historyCancel = nil
	}
	m.resetLogBuffer()
	m.logView.scroll = 0
	m.logView.cursor = 0
	m.logView.visualMode = false
	m.logView.tailLines = ui.ConfigLogTailLines
	m.logView.hasMoreHistory = !m.logView.previous && !m.logView.isMulti
	m.logView.loadingHistory = false
	if m.logView.isMulti && len(m.logView.multiItems) > 0 {
		var cmd tea.Cmd
		m, cmd = m.restartMultiLogStream()
		return m, cmd
	}
	return m, m.startLogStream()
}

func (m Model) handleLogKeyZero() Model {
	if m.logView.lineInput != "" {
		m.logView.lineInput += "0"
	} else {
		m.logView.visualCurCol = 0
	}
	return m
}

func (m Model) handleLogKeyOther() (tea.Model, tea.Cmd) {
	m.logView.lineInput = ""
	if m.logView.parentKind != "" {
		// Group resource: show pod selector to switch between pods.
		m.logView.savedPodName = m.actionCtx.name
		if m.logView.cancel != nil {
			m.logView.cancel()
			m.logView.cancel = nil
		}
		if m.logView.historyCancel != nil {
			m.logView.historyCancel()
			m.logView.historyCancel = nil
		}
		m.logView.ch = nil
		m.actionCtx.kind = m.logView.parentKind
		m.actionCtx.name = m.logView.parentName
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
	if m.logView.cancel != nil {
		m.logView.cancel()
	}
	if m.logView.historyCancel != nil {
		m.logView.historyCancel()
		m.logView.historyCancel = nil
	}
	return m.closeTabOrQuit()
}

// handleLogNormalCopy copies log lines at and below the cursor (in display
// form, so timestamps and pod prefixes follow the user's toggles) to the
// clipboard. A digit prefix (e.g. `123y`) yanks that many lines; an empty
// buffer falls back to a single line.
func (m Model) handleLogNormalCopy() (tea.Model, tea.Cmd) {
	n := consumeCountPrefix(&m.logView.lineInput)
	if m.logView.cursor < 0 || m.logView.cursor >= len(m.logView.lines) {
		return m, nil
	}
	end := min(m.logView.cursor+n, len(m.logView.lines))
	parts := make([]string, 0, end-m.logView.cursor)
	for i := m.logView.cursor; i < end; i++ {
		parts = append(parts, m.logDisplayLine(i))
	}
	m.setStatusMessage(formatCopiedLines(len(parts)), false)
	return m, tea.Batch(copyToSystemClipboard(strings.Join(parts, "\n")), scheduleStatusClear())
}
