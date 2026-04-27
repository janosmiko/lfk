package app

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) handleOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Toggle: pressing the same hotkey that opened an overlay closes it.
	if m.isOverlayToggleKey(msg.String()) {
		m.overlay = overlayNone
		return m, nil
	}
	if mdl, cmd, ok := m.handleOverlayKeyPrimary(msg); ok {
		return mdl, cmd
	}
	if mdl, cmd, ok := m.handleOverlayKeySecondary(msg); ok {
		return mdl, cmd
	}
	return m, nil
}

// isOverlayToggleKey returns true when key matches the hotkey that
// originally opened the current overlay. This lets users press the
// same key to close an overlay instead of reaching for Esc.
func (m Model) isOverlayToggleKey(key string) bool {
	kb := ui.ActiveKeybindings
	switch m.overlay {
	case overlayBackgroundTasks:
		return key == kb.TasksOverlay
	case overlayNamespace:
		return key == kb.NamespaceSelector
	case overlayAction:
		return key == kb.ActionMenu
	case overlayColorscheme:
		return key == kb.ThemeSelector
	case overlayFilterPreset:
		return key == kb.FilterPresets
	case overlayColumnToggle:
		return key == kb.ColumnToggle
	case overlayQuotaDashboard:
		return key == kb.QuotaDashboard
	}
	return false
}

// handleOverlayKeyPrimary dispatches overlay keys for core overlays
// (selectors, confirmations, editors).
func (m Model) handleOverlayKeyPrimary(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.overlay {
	case overlayNamespace:
		mdl, cmd := m.handleNamespaceOverlayKey(msg)
		return mdl, cmd, true
	case overlayAction:
		mdl, cmd := m.handleActionOverlayKey(msg)
		return mdl, cmd, true
	case overlayConfirm:
		mdl, cmd := m.handleConfirmOverlayKey(msg)
		return mdl, cmd, true
	case overlayConfirmType:
		mdl, cmd := m.handleConfirmTypeOverlayKey(msg)
		return mdl, cmd, true
	case overlayScaleInput:
		mdl, cmd := m.handleScaleOverlayKey(msg)
		return mdl, cmd, true
	case overlayPVCResize:
		mdl, cmd := m.handlePVCResizeOverlayKey(msg)
		return mdl, cmd, true
	case overlayPortForward:
		mdl, cmd := m.handlePortForwardOverlayKey(msg)
		return mdl, cmd, true
	case overlayContainerSelect:
		mdl, cmd := m.handleContainerSelectOverlayKey(msg)
		return mdl, cmd, true
	case overlayPodSelect:
		mdl, cmd := m.handlePodSelectOverlayKey(msg)
		return mdl, cmd, true
	case overlayBookmarks:
		mdl, cmd := m.handleBookmarkOverlayKey(msg)
		return mdl, cmd, true
	case overlayTemplates:
		mdl, cmd := m.handleTemplateOverlayKey(msg)
		return mdl, cmd, true
	case overlaySecretEditor:
		mdl, cmd := m.handleSecretEditorKey(msg)
		return mdl, cmd, true
	case overlayConfigMapEditor:
		mdl, cmd := m.handleConfigMapEditorKey(msg)
		return mdl, cmd, true
	case overlayRollback:
		mdl, cmd := m.handleRollbackOverlayKey(msg)
		return mdl, cmd, true
	case overlayHelmRollback:
		mdl, cmd := m.handleHelmRollbackOverlayKey(msg)
		return mdl, cmd, true
	case overlayHelmHistory:
		mdl, cmd := m.handleHelmHistoryOverlayKey(msg)
		return mdl, cmd, true
	case overlayLabelEditor:
		mdl, cmd := m.handleLabelEditorKey(msg)
		return mdl, cmd, true
	case overlayAutoSync:
		mdl, cmd := m.handleAutoSyncKey(msg)
		return mdl, cmd, true
	case overlayColorscheme:
		mdl, cmd := m.handleColorschemeOverlayKey(msg)
		return mdl, cmd, true
	case overlayFilterPreset:
		mdl, cmd := m.handleFilterPresetOverlayKey(msg)
		return mdl, cmd, true
	}
	return m, nil, false
}

// handleOverlayKeySecondary dispatches overlay keys for secondary overlays
// (viewers, monitoring, info panels).
func (m Model) handleOverlayKeySecondary(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.overlay {
	case overlayRBAC, overlayPodStartup:
		m.overlay = overlayNone
		return m, nil, true
	case overlayAlerts:
		mdl, cmd := m.handleAlertsOverlayKey(msg)
		return mdl, cmd, true
	case overlayBackgroundTasks:
		mdl, cmd := m.handleBackgroundTasksOverlayKey(msg)
		return mdl, cmd, true
	case overlayBatchLabel:
		mdl, cmd := m.handleBatchLabelOverlayKey(msg)
		return mdl, cmd, true
	case overlayQuotaDashboard:
		return m.handleOverlayKeyOverlayQuotaDashboard(msg), nil, true
	case overlayEventTimeline:
		mdl, cmd := m.handleEventTimelineOverlayKey(msg)
		return mdl, cmd, true
	case overlayNetworkPolicy:
		return m.handleNetworkPolicyOverlayKey(msg), nil, true
	case overlayCanI:
		mdl, cmd := m.handleCanIKey(msg)
		return mdl, cmd, true
	case overlayCanISubject:
		mdl, cmd := m.handleCanISubjectOverlayKey(msg)
		return mdl, cmd, true
	case overlayExplainSearch:
		mdl, cmd := m.handleExplainSearchOverlayKey(msg)
		return mdl, cmd, true
	case overlayQuitConfirm:
		mdl, cmd := m.handleQuitConfirmOverlayKey(msg)
		return mdl, cmd, true
	case overlayLogPodSelect:
		mdl, cmd := m.handleLogPodSelectOverlayKey(msg)
		return mdl, cmd, true
	case overlayLogContainerSelect:
		mdl, cmd := m.handleLogContainerSelectOverlayKey(msg)
		return mdl, cmd, true
	case overlayFinalizerSearch:
		mdl, cmd := m.handleFinalizerSearchKey(msg)
		return mdl, cmd, true
	case overlayColumnToggle:
		mdl, cmd := m.handleColumnToggleKey(msg)
		return mdl, cmd, true
	case overlayPasteConfirm:
		mdl, cmd := m.handlePasteConfirmKey(msg)
		return mdl, cmd, true
	}
	return m, nil, false
}

// handlePasteConfirmKey handles the Enter/y / Esc/n confirmation for multiline paste.
func (m Model) handlePasteConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "y", "Y":
		m.overlay = overlayNone
		if target := m.resolvePasteTarget(m.pasteTargetID); target != nil && m.pendingPaste != "" {
			flattened := strings.ReplaceAll(strings.TrimRight(m.pendingPaste, "\n"), "\n", " ")
			target.Insert(flattened)
		}
		m.pendingPaste = ""
		m.pasteTargetID = pasteTargetNone
		m.setStatusMessage("Pasted (flattened to single line)", false)
		return m, scheduleStatusClear()
	case "n", "N", "esc":
		m.overlay = overlayNone
		m.pendingPaste = ""
		m.pasteTargetID = pasteTargetNone
		m.setStatusMessage("Paste cancelled", false)
		return m, scheduleStatusClear()
	}
	return m, nil
}

// handleNetworkPolicyOverlayKey handles keyboard input in the network policy visualizer overlay.
func (m Model) handleNetworkPolicyOverlayKey(msg tea.KeyMsg) Model {
	switch msg.String() {
	case "esc", "q":
		m.netpolLineInput = ""
		m.overlay = overlayNone
		m.netpolData = nil
	case "j", "down":
		m.netpolLineInput = ""
		m.netpolScroll++
	case "k", "up":
		m.netpolLineInput = ""
		if m.netpolScroll > 0 {
			m.netpolScroll--
		}
	case "g":
		m.netpolLineInput = ""
		if m.pendingG {
			m.pendingG = false
			m.netpolScroll = 0
		} else {
			m.pendingG = true
		}
	case "G":
		if m.netpolLineInput != "" {
			lineNum, _ := strconv.Atoi(m.netpolLineInput)
			m.netpolLineInput = ""
			if lineNum > 0 {
				lineNum--
			}
			m.netpolScroll = lineNum
		} else {
			// Jump to bottom: will be clamped during rendering.
			m.netpolScroll = 9999
		}
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.netpolLineInput += msg.String()
		return m
	case "0":
		if m.netpolLineInput != "" {
			m.netpolLineInput += "0"
			return m
		}
	case "ctrl+d":
		m.netpolLineInput = ""
		m.netpolScroll += m.height / 2
	case "ctrl+u":
		m.netpolLineInput = ""
		m.netpolScroll -= m.height / 2
		if m.netpolScroll < 0 {
			m.netpolScroll = 0
		}
	case "ctrl+f", "pgdown":
		m.netpolLineInput = ""
		m.netpolScroll += m.height
	case "ctrl+b", "pgup":
		m.netpolLineInput = ""
		m.netpolScroll -= m.height
		if m.netpolScroll < 0 {
			m.netpolScroll = 0
		}
	case "home":
		m.pendingG = false
		m.netpolLineInput = ""
		m.netpolScroll = 0
	case "end":
		m.netpolLineInput = ""
		// Jump to bottom: will be clamped during rendering (matches G behavior).
		m.netpolScroll = 9999
	default:
		m.netpolLineInput = ""
	}
	return m
}

// errorLogVisibleCount returns the number of visible entries and max dimensions for the error log overlay.
func (m Model) errorLogVisibleCount() (visibleCount, maxVisible, maxScroll int) {
	reversed := ui.FilteredErrorLogEntries(m.errorLog, m.showDebugLogs)
	visibleCount = len(reversed)

	var overlayH int
	if m.errorLogFullscreen {
		overlayH = m.height - 1
	} else {
		overlayH = min(30, m.height-4)
	}
	maxVisible = max(overlayH-4, 1)
	maxScroll = max(visibleCount-maxVisible, 0)
	return
}

// handleErrorLogOverlayKey handles keyboard input when the error log overlay is open.
// errorLogForwardGlobalKey forwards a small set of "global" navigation keys
// (new/next/prev tab, theme selector) to the underlying explorer handlers so
// users can keep the error log overlay visible while switching tabs or while
// opening the theme selector on top. The error log overlay state is left
// alone — fullscreen + theme selector should layer the way the dashboard
// fullscreen + theme selector does, with the error log staying behind the
// colorscheme overlay until it closes.
// Returns handled=false for non-matching keys so the regular overlay key
// dispatch can run. Visual mode disables the forwarding so 't' / 'T' inside
// a selection stay local.
func (m Model) errorLogForwardGlobalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	if m.errorLogVisualMode != 0 {
		return m, nil, false
	}
	kb := ui.ActiveKeybindings
	switch msg.String() {
	case kb.NewTab, kb.NextTab, kb.PrevTab:
		if mdl, cmd, ok := m.handleExplorerActionKey(msg); ok {
			return mdl, cmd, true
		}
	case kb.ThemeSelector:
		return m.handleKeyThemeSelector(), nil, true
	}
	return m, nil, false
}

func (m Model) handleErrorLogOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	visibleCount, maxVisible, maxScroll := m.errorLogVisibleCount()
	maxCursor := max(visibleCount-1, 0)

	key := msg.String()

	// Toggle: pressing the error log hotkey again closes the overlay.
	if key == ui.ActiveKeybindings.ErrorLog {
		return m.handleErrorLogOverlayKeyEsc()
	}

	// Allow tab switching and theme selector to work while the overlay
	// is up — extracted to keep this function under the gocyclo cap.
	if mdl, cmd, handled := m.errorLogForwardGlobalKey(msg); handled {
		return mdl, cmd
	}

	// In visual mode, Esc cancels visual mode instead of closing.
	if key == "esc" && m.errorLogVisualMode != 0 {
		m.errorLogLineInput = ""
		m.errorLogVisualMode = 0
		return m, nil
	}

	switch key {
	case "esc", "q":
		return m.handleErrorLogOverlayKeyEsc()

	case "f":
		return m.handleErrorLogOverlayKeyF()

	case "V":
		return m.handleErrorLogOverlayKeyV()

	case "v":
		return m.handleErrorLogOverlayKeyV2()

	case "h", "left":
		return m.handleErrorLogOverlayKeyH()

	case "l", "right":
		return m.handleErrorLogOverlayKeyL()

	case "0":
		return m.handleErrorLogOverlayKeyZero()

	case "$":
		return m.handleErrorLogOverlayKeyDollar()

	case "y":
		m.errorLogLineInput = ""
		return m.errorLogYank()

	case "d":
		return m.handleErrorLogOverlayKeyD()

	case "j", "down":
		m.errorLogLineInput = ""
		if m.errorLogCursorLine < maxCursor {
			m.errorLogCursorLine++
		}
		m.errorLogScroll = m.errorLogEnsureCursorVisible(maxVisible, maxScroll)
		return m, nil

	case "k", "up":
		m.errorLogLineInput = ""
		if m.errorLogCursorLine > 0 {
			m.errorLogCursorLine--
		}
		m.errorLogScroll = m.errorLogEnsureCursorVisible(maxVisible, maxScroll)
		return m, nil

	case "g":
		return m.handleErrorLogOverlayKeyG()

	case "G":
		if m.errorLogLineInput != "" {
			lineNum, _ := strconv.Atoi(m.errorLogLineInput)
			m.errorLogLineInput = ""
			if lineNum > 0 {
				lineNum--
			}
			m.errorLogCursorLine = min(lineNum, maxCursor)
			m.errorLogScroll = m.errorLogEnsureCursorVisible(maxVisible, maxScroll)
			return m, nil
		}
		m.errorLogCursorLine = maxCursor
		m.errorLogScroll = maxScroll
		return m, nil

	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.errorLogLineInput += key
		return m, nil

	case "ctrl+d":
		m.errorLogLineInput = ""
		halfPage := maxVisible / 2
		m.errorLogCursorLine = min(m.errorLogCursorLine+halfPage, maxCursor)
		m.errorLogScroll = m.errorLogEnsureCursorVisible(maxVisible, maxScroll)
		return m, nil

	case "ctrl+u":
		m.errorLogLineInput = ""
		halfPage := maxVisible / 2
		m.errorLogCursorLine = max(m.errorLogCursorLine-halfPage, 0)
		m.errorLogScroll = m.errorLogEnsureCursorVisible(maxVisible, maxScroll)
		return m, nil

	case "ctrl+f", "pgdown":
		m.errorLogLineInput = ""
		m.errorLogCursorLine = min(m.errorLogCursorLine+maxVisible, maxCursor)
		m.errorLogScroll = m.errorLogEnsureCursorVisible(maxVisible, maxScroll)
		return m, nil

	case "ctrl+b", "pgup":
		m.errorLogLineInput = ""
		m.errorLogCursorLine = max(m.errorLogCursorLine-maxVisible, 0)
		m.errorLogScroll = m.errorLogEnsureCursorVisible(maxVisible, maxScroll)
		return m, nil

	case "home":
		m.pendingG = false
		m.errorLogLineInput = ""
		m.errorLogCursorLine = 0
		m.errorLogScroll = 0
		return m, nil

	case "end":
		m.errorLogLineInput = ""
		m.errorLogCursorLine = maxCursor
		m.errorLogScroll = maxScroll
		return m, nil

	default:
		m.errorLogLineInput = ""
	}
	return m, nil
}

// errorLogEnsureCursorVisible adjusts scroll so the cursor line is within the
// visible window with scrolloff margin.
func (m Model) errorLogEnsureCursorVisible(maxVisible, maxScroll int) int {
	scroll := m.errorLogScroll
	so := min(ui.ConfigScrollOff, maxVisible/2)
	if m.errorLogCursorLine < scroll+so {
		scroll = m.errorLogCursorLine - so
	}
	if m.errorLogCursorLine >= scroll+maxVisible-so {
		scroll = m.errorLogCursorLine - maxVisible + so + 1
	}
	return max(min(scroll, maxScroll), 0)
}

// errorLogYank copies error log content to clipboard.
// In visual mode: copies selected lines. Otherwise: copies all visible entries.
func (m Model) errorLogYank() (tea.Model, tea.Cmd) {
	reversed := ui.FilteredErrorLogEntries(m.errorLog, m.showDebugLogs)
	if len(reversed) == 0 {
		return m, nil
	}

	var lines []string
	switch m.errorLogVisualMode {
	case 'v':
		// Character visual mode: extract partial text respecting column positions.
		selStart := min(m.errorLogVisualStart, m.errorLogCursorLine)
		selEnd := max(m.errorLogVisualStart, m.errorLogCursorLine)
		// Determine start/end columns based on direction.
		var startCol, endCol int
		if m.errorLogVisualStart <= m.errorLogCursorLine {
			startCol = m.errorLogVisualStartCol
			endCol = m.errorLogCursorCol
		} else {
			startCol = m.errorLogCursorCol
			endCol = m.errorLogVisualStartCol
		}
		for i := selStart; i <= selEnd && i < len(reversed); i++ {
			plain := ui.ErrorLogEntryPlainText(reversed[i])
			runes := []rune(plain)
			if selStart == selEnd {
				// Single line: extract between columns.
				cStart := min(startCol, endCol)
				cEnd := min(max(startCol, endCol)+1, len(runes))
				if cStart < len(runes) {
					lines = append(lines, string(runes[cStart:cEnd]))
				}
			} else if i == selStart {
				if startCol < len(runes) {
					lines = append(lines, string(runes[startCol:]))
				}
			} else if i == selEnd {
				cEnd := min(endCol+1, len(runes))
				lines = append(lines, string(runes[:cEnd]))
			} else {
				lines = append(lines, plain)
			}
		}
		m.errorLogVisualMode = 0
	case 'V':
		// Line visual mode: full lines.
		selStart := min(m.errorLogVisualStart, m.errorLogCursorLine)
		selEnd := max(m.errorLogVisualStart, m.errorLogCursorLine)
		for i := selStart; i <= selEnd && i < len(reversed); i++ {
			lines = append(lines, ui.ErrorLogEntryPlainText(reversed[i]))
		}
		m.errorLogVisualMode = 0
	default:
		for _, e := range reversed {
			lines = append(lines, ui.ErrorLogEntryPlainText(e))
		}
	}

	text := strings.Join(lines, "\n")
	m.setStatusMessage(fmt.Sprintf("Copied %d entries to clipboard", len(lines)), false)
	return m, tea.Batch(copyToSystemClipboard(text), scheduleStatusClear())
}

func (m Model) handleFilterPresetOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc", "q":
		m.overlay = overlayNone
		return m, nil
	case "enter":
		if m.overlayCursor >= 0 && m.overlayCursor < len(m.filterPresets) {
			return m.applyFilterPreset(m.filterPresets[m.overlayCursor])
		}
		m.overlay = overlayNone
		return m, nil
	case "up", "k":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -1, len(m.filterPresets)-1)
		return m, nil
	case "down", "j":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 1, len(m.filterPresets)-1)
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		// Shortcut key: match against preset hotkeys.
		for _, preset := range m.filterPresets {
			if preset.Key == key {
				return m.applyFilterPreset(preset)
			}
		}
	}
	return m, nil
}

// applyFilterPreset applies a filter preset to the middle items and closes the overlay.
func (m Model) applyFilterPreset(preset FilterPreset) (tea.Model, tea.Cmd) {
	m.overlay = overlayNone

	// Save the unfiltered list so we can restore it later.
	m.unfilteredMiddleItems = append([]model.Item(nil), m.middleItems...)

	// Filter middleItems.
	var filtered []model.Item
	for _, item := range m.middleItems {
		if preset.MatchFn(item) {
			filtered = append(filtered, item)
		}
	}
	m.setMiddleItems(filtered)
	m.activeFilterPreset = &preset
	m.setCursor(0)
	m.clampCursor()
	m.setStatusMessage(fmt.Sprintf("Filter: %s (%d matches)", preset.Name, len(filtered)), false)
	return m, tea.Batch(scheduleStatusClear(), m.loadPreview())
}

// handleBatchLabelOverlayKey handles keyboard input for the batch label/annotation editor overlay.
func (m Model) handleAlertsOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	maxScroll := max(len(m.alertsData)-1, 0)

	switch key {
	case "esc", "q":
		m.alertsLineInput = ""
		m.overlay = overlayNone
		return m, nil
	case "j", "down":
		m.alertsLineInput = ""
		m.alertsScroll++
		return m, nil
	case "k", "up":
		m.alertsLineInput = ""
		if m.alertsScroll > 0 {
			m.alertsScroll--
		}
		return m, nil
	case "g":
		m.alertsLineInput = ""
		if m.pendingG {
			m.pendingG = false
			m.alertsScroll = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "G":
		if m.alertsLineInput != "" {
			lineNum, _ := strconv.Atoi(m.alertsLineInput)
			m.alertsLineInput = ""
			if lineNum > 0 {
				lineNum--
			}
			m.alertsScroll = min(lineNum, maxScroll)
			return m, nil
		}
		// Jump to bottom -- the render function will clamp.
		m.alertsScroll = len(m.alertsData)
		return m, nil
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.alertsLineInput += key
		return m, nil
	case "0":
		if m.alertsLineInput != "" {
			m.alertsLineInput += "0"
			return m, nil
		}
	case "ctrl+d":
		m.alertsLineInput = ""
		m.alertsScroll += 10
		return m, nil
	case "ctrl+u":
		m.alertsLineInput = ""
		m.alertsScroll = max(m.alertsScroll-10, 0)
		return m, nil
	case "ctrl+f":
		m.alertsLineInput = ""
		m.alertsScroll += 20
		return m, nil
	case "ctrl+b":
		m.alertsLineInput = ""
		m.alertsScroll = max(m.alertsScroll-20, 0)
		return m, nil
	case "ctrl+c":
		m.alertsLineInput = ""
		return m.closeTabOrQuit()
	default:
		m.alertsLineInput = ""
	}
	return m, nil
}

func (m Model) handleBatchLabelOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.overlay = overlayNone
		return m, nil
	case "tab":
		m.batchLabelRemove = !m.batchLabelRemove
		return m, nil
	case "enter":
		if m.batchLabelInput.Value == "" {
			return m, nil
		}
		isAnnotation := m.batchLabelMode == 1
		// Parse input: "key=value" for add, "key" for remove.
		var labelKey, labelValue string
		if m.batchLabelRemove {
			labelKey = m.batchLabelInput.Value
		} else {
			parts := strings.SplitN(m.batchLabelInput.Value, "=", 2)
			if len(parts) != 2 || parts[0] == "" {
				m.setStatusMessage("Format: key=value", true)
				return m, scheduleStatusClear()
			}
			labelKey = parts[0]
			labelValue = parts[1]
		}
		m.overlay = overlayNone
		m.loading = true
		action := "labels"
		if isAnnotation {
			action = "annotations"
		}
		mode := "Adding"
		if m.batchLabelRemove {
			mode = "Removing"
		}
		m.setStatusMessage(fmt.Sprintf("%s %s...", mode, action), false)
		m.clearSelection()
		return m, m.batchPatchLabels(labelKey, labelValue, m.batchLabelRemove, isAnnotation)
	case "backspace":
		if len(m.batchLabelInput.Value) > 0 {
			m.batchLabelInput.Backspace()
		}
		return m, nil
	case "ctrl+w":
		m.batchLabelInput.DeleteWord()
		return m, nil
	case "ctrl+a":
		m.batchLabelInput.Home()
		return m, nil
	case "ctrl+e":
		m.batchLabelInput.End()
		return m, nil
	case "left":
		m.batchLabelInput.Left()
		return m, nil
	case "right":
		m.batchLabelInput.Right()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.batchLabelInput.Insert(key)
		}
		return m, nil
	}
}

func (m Model) handleActionOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc", "q":
		m.overlay = overlayNone
		return m, nil
	case "enter":
		if m.overlayCursor >= 0 && m.overlayCursor < len(m.overlayItems) {
			return m.executeAction(m.overlayItems[m.overlayCursor].Name)
		}
		m.overlay = overlayNone
		return m, nil
	case "up", "k", "ctrl+p":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -1, len(m.overlayItems)-1)
		return m, nil
	case "down", "j", "ctrl+n":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 1, len(m.overlayItems)-1)
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		// Shortcut key: match against action hotkeys (stored in Status field).
		for _, item := range m.overlayItems {
			if item.Status == key {
				return m.executeAction(item.Name)
			}
		}
	}
	return m, nil
}

func (m Model) handleConfirmOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "y", "Y":
		m.overlay = overlayNone
		m.loading = true
		action := m.pendingAction
		m.pendingAction = ""
		m.confirmAction = ""

		ns := m.actionCtx.namespace
		name := m.actionCtx.name
		ctx := m.actionCtx.context
		rt := m.actionCtx.resourceType
		nsArg := ""
		if rt.Namespaced {
			nsArg = " -n " + ns
		}

		// Bulk delete.
		if m.bulkMode && len(m.bulkItems) > 0 {
			m.clearSelection()
			expanded := expandGroupedItems(m.bulkItems)
			m.addLogEntry("DBG", fmt.Sprintf("$ kubectl delete %s (%d items)%s --context %s", rt.Resource, len(expanded), nsArg, ctx))
			return m, m.bulkDeleteResources()
		}

		if action == "Drain" {
			m.addLogEntry("DBG", fmt.Sprintf("$ kubectl drain %s --ignore-daemonsets --delete-emptydir-data --context %s", name, ctx))
			return m, m.execKubectlDrain()
		}

		// Regular delete.
		if rt.APIGroup == "_helm" {
			m.addLogEntry("DBG", fmt.Sprintf("$ helm uninstall %s -n %s --kube-context %s", name, ns, ctx))
		} else {
			m.addLogEntry("DBG", fmt.Sprintf("$ kubectl delete %s %s%s --context %s", rt.Resource, name, nsArg, ctx))
		}
		return m, m.deleteResource()
	case "n", "N", "esc", "q":
		m.overlay = overlayNone
		m.confirmAction = ""
		m.pendingAction = ""
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

func (m Model) handleConfirmTypeOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.confirmAction = ""
		m.confirmTitle = ""
		m.confirmQuestion = ""
		m.pendingAction = ""
		m.confirmTypeInput.Clear()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	case "enter":
		if m.confirmTypeInput.Value == "DELETE" {
			m.overlay = overlayNone
			m.loading = true
			action := m.pendingAction
			m.pendingAction = ""
			m.confirmAction = ""
			m.confirmTitle = ""
			m.confirmQuestion = ""
			m.confirmTypeInput.Clear()

			ns := m.actionCtx.namespace
			name := m.actionCtx.name
			ctx := m.actionCtx.context
			rt := m.actionCtx.resourceType
			nsArg := ""
			if rt.Namespaced {
				nsArg = " -n " + ns
			}

			// Bulk force delete.
			if m.bulkMode && len(m.bulkItems) > 0 && action == "Force Delete" {
				m.clearSelection()
				expanded := expandGroupedItems(m.bulkItems)
				m.addLogEntry("DBG", fmt.Sprintf("$ kubectl delete --force --grace-period=0 %s (%d items)%s --context %s", rt.Resource, len(expanded), nsArg, ctx))
				return m, m.bulkForceDeleteResources()
			}

			switch action {
			case "Force Finalize":
				m.addLogEntry("DBG", fmt.Sprintf("$ kubectl patch %s %s --type merge -p '{\"metadata\":{\"finalizers\":null}}'%s --context %s", rt.Resource, name, nsArg, ctx))
				return m, m.removeFinalizers()
			case "Force Delete":
				m.addLogEntry("DBG", fmt.Sprintf("$ kubectl delete %s %s --grace-period=0 --force%s --context %s", rt.Resource, name, nsArg, ctx))
				return m, m.forceDeleteResource()
			case "Finalizer Remove":
				m.loading = false
				m.overlay = overlayFinalizerSearch
				selectedCount := len(m.finalizerSearchSelected)
				m.addLogEntry("DBG", fmt.Sprintf("Removing finalizer %q from %d resources", m.finalizerSearchPattern, selectedCount))
				return m, m.bulkRemoveFinalizer()
			}
		}
		return m, nil
	case "backspace":
		m.confirmTypeInput.Backspace()
		return m, nil
	case "ctrl+w":
		m.confirmTypeInput.DeleteWord()
		return m, nil
	case "ctrl+u":
		m.confirmTypeInput.Clear()
		return m, nil
	default:
		if len(msg.String()) == 1 {
			m.confirmTypeInput.Insert(msg.String())
		}
		return m, nil
	}
}

func (m Model) handleScaleOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.scaleInput.Clear()
		return m, nil
	case "enter":
		replicas, err := strconv.ParseInt(m.scaleInput.Value, 10, 32)
		if err != nil || replicas < 0 {
			m.setStatusMessage("Invalid replica count", true)
			m.overlay = overlayNone
			m.scaleInput.Clear()
			return m, scheduleStatusClear()
		}
		m.overlay = overlayNone
		m.loading = true
		m.scaleInput.Clear()

		// Bulk mode.
		if m.bulkMode && len(m.bulkItems) > 0 {
			m.addLogEntry("DBG", fmt.Sprintf("$ kubectl scale %s --replicas=%d (%d items) -n %s --context %s", strings.ToLower(m.actionCtx.kind), replicas, len(m.bulkItems), m.actionCtx.namespace, m.actionCtx.context))
			m.clearSelection()
			return m, m.bulkScaleResources(int32(replicas))
		}

		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl scale %s %s --replicas=%d -n %s --context %s", strings.ToLower(m.actionCtx.kind), m.actionCtx.name, replicas, m.actionCtx.namespace, m.actionCtx.context))
		return m, m.scaleResource(int32(replicas))
	case "backspace":
		if len(m.scaleInput.Value) > 0 {
			m.scaleInput.Backspace()
		}
		return m, nil
	case "ctrl+w":
		m.scaleInput.DeleteWord()
		return m, nil
	case "ctrl+a":
		m.scaleInput.Home()
		return m, nil
	case "ctrl+e":
		m.scaleInput.End()
		return m, nil
	case "left":
		m.scaleInput.Left()
		return m, nil
	case "right":
		m.scaleInput.Right()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= '0' && key[0] <= '9' {
			m.scaleInput.Insert(key)
		}
		return m, nil
	}
}

func (m Model) handlePVCResizeOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.scaleInput.Clear()
		return m, nil
	case "enter":
		newSize := strings.TrimSpace(m.scaleInput.Value)
		if newSize == "" {
			m.setStatusMessage("No size specified", true)
			m.overlay = overlayNone
			m.scaleInput.Clear()
			return m, scheduleStatusClear()
		}
		m.overlay = overlayNone
		m.loading = true
		m.addLogEntry("DBG", fmt.Sprintf("Resizing PVC %s to %s in %s", m.actionCtx.name, newSize, m.actionNamespace()))
		m.scaleInput.Clear()
		return m, m.resizePVC(newSize)
	case "backspace":
		if len(m.scaleInput.Value) > 0 {
			m.scaleInput.Backspace()
		}
		return m, nil
	case "ctrl+w":
		m.scaleInput.DeleteWord()
		return m, nil
	case "ctrl+a":
		m.scaleInput.Home()
		return m, nil
	case "ctrl+e":
		m.scaleInput.End()
		return m, nil
	case "left":
		m.scaleInput.Left()
		return m, nil
	case "right":
		m.scaleInput.Right()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		key := msg.String()
		if len(key) == 1 {
			m.scaleInput.Insert(key)
		}
		return m, nil
	}
}

func (m Model) handlePortForwardOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.portForwardInput.Clear()
		m.pfAvailablePorts = nil
		m.pfPortCursor = -1
		return m, nil
	case "j", "down":
		if len(m.pfAvailablePorts) > 0 && m.pfPortCursor < len(m.pfAvailablePorts)-1 {
			m.pfPortCursor++
		}
		return m, nil
	case "k", "up":
		if m.pfPortCursor > 0 {
			m.pfPortCursor--
		}
		return m, nil
	case "enter":
		var localPort, remotePort string
		switch {
		case m.pfPortCursor >= 0 && m.pfPortCursor < len(m.pfAvailablePorts):
			p := m.pfAvailablePorts[m.pfPortCursor]
			remotePort = p.Port
			if m.portForwardInput.Value != "" {
				// User typed a custom local port.
				localPort = m.portForwardInput.Value
			} else {
				// Empty input: let kubectl pick a random high port.
				localPort = "0"
			}
		case m.portForwardInput.Value != "":
			// Manual entry: parse as localPort:remotePort or just port.
			parts := strings.SplitN(m.portForwardInput.Value, ":", 2)
			localPort = parts[0]
			if len(parts) == 2 {
				remotePort = parts[1]
			} else {
				remotePort = localPort
			}
		default:
			m.setStatusMessage("Port mapping required (e.g., 8080:80)", true)
			m.overlay = overlayNone
			return m, scheduleStatusClear()
		}
		portMapping := localPort + ":" + remotePort
		m.overlay = overlayNone
		m.portForwardInput.Clear()
		m.pfAvailablePorts = nil
		m.pfPortCursor = -1
		resourcePrefix := "pod/"
		if m.actionCtx.kind == "Service" {
			resourcePrefix = "svc/"
		}
		m.addLogEntry("DBG", fmt.Sprintf("$ kubectl port-forward %s%s %s -n %s --context %s", resourcePrefix, m.actionCtx.name, portMapping, m.actionCtx.namespace, m.actionCtx.context))
		return m, m.execKubectlPortForward(portMapping)
	case "backspace":
		if len(m.portForwardInput.Value) > 0 {
			m.portForwardInput.Backspace()
		}
		return m, nil
	case "ctrl+w":
		m.portForwardInput.DeleteWord()
		return m, nil
	case "ctrl+a":
		m.portForwardInput.Home()
		return m, nil
	case "ctrl+e":
		m.portForwardInput.End()
		return m, nil
	case "left":
		m.portForwardInput.Left()
		return m, nil
	case "right":
		m.portForwardInput.Right()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		key := msg.String()
		if len(key) == 1 && ((key[0] >= '0' && key[0] <= '9') || key[0] == ':') {
			m.portForwardInput.Insert(key)
			// When user types ':', they want manual local:remote mode — deselect from port list.
			if key[0] == ':' {
				m.pfPortCursor = -1
			}
		}
		return m, nil
	}
}

func (m Model) handleContainerSelectOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.pendingAction = ""
		return m, nil
	case "enter":
		if m.overlayCursor >= 0 && m.overlayCursor < len(m.overlayItems) {
			m.actionCtx.containerName = m.overlayItems[m.overlayCursor].Name
			m.overlay = overlayNone
			action := m.pendingAction
			m.pendingAction = ""
			return m.executeAction(action)
		}
		m.overlay = overlayNone
		return m, nil
	case "up", "k", "ctrl+p":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -1, len(m.overlayItems)-1)
		return m, nil
	case "down", "j", "ctrl+n":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 1, len(m.overlayItems)-1)
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

// handleQuitConfirmOverlayKey handles keyboard input for the quit confirmation overlay.
func (m Model) handleQuitConfirmOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "y", "Y":
		m.overlay = overlayNone
		if m.portForwardMgr != nil {
			m.portForwardMgr.StopAll()
		}
		m.cancelAllTabLogStreams()
		m.saveCurrentSession()
		return m, tea.Quit
	case "n", "N", "esc", "q":
		m.overlay = overlayNone
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

func (m Model) handleOverlayKeyOverlayQuotaDashboard(msg tea.KeyMsg) Model {
	if msg.String() == "esc" || msg.String() == "q" {
		m.overlay = overlayNone
	}
	return m
}

func (m Model) handleErrorLogOverlayKeyEsc() (tea.Model, tea.Cmd) {
	m.errorLogLineInput = ""
	m.overlayErrorLog = false
	m.errorLogScroll = 0
	m.errorLogFullscreen = false
	m.errorLogVisualMode = 0
	m.errorLogCursorLine = 0
	return m, nil
}

func (m Model) handleErrorLogOverlayKeyF() (tea.Model, tea.Cmd) {
	m.errorLogLineInput = ""
	m.errorLogFullscreen = !m.errorLogFullscreen
	// Reset scroll when toggling to avoid out-of-bounds.
	m.errorLogScroll = 0
	return m, nil
}

func (m Model) handleErrorLogOverlayKeyV() (tea.Model, tea.Cmd) {
	m.errorLogLineInput = ""
	if m.errorLogVisualMode == 'V' {
		m.errorLogVisualMode = 0
	} else {
		m.errorLogVisualMode = 'V'
		m.errorLogVisualStart = m.errorLogCursorLine
	}
	return m, nil
}

func (m Model) handleErrorLogOverlayKeyV2() (tea.Model, tea.Cmd) {
	m.errorLogLineInput = ""
	if m.errorLogVisualMode == 'v' {
		m.errorLogVisualMode = 0
	} else {
		m.errorLogVisualMode = 'v'
		m.errorLogVisualStart = m.errorLogCursorLine
		m.errorLogVisualStartCol = m.errorLogCursorCol
	}
	return m, nil
}

func (m Model) handleErrorLogOverlayKeyH() (tea.Model, tea.Cmd) {
	m.errorLogLineInput = ""
	if m.errorLogVisualMode == 'v' && m.errorLogCursorCol > 0 {
		m.errorLogCursorCol--
	}
	return m, nil
}

func (m Model) handleErrorLogOverlayKeyL() (tea.Model, tea.Cmd) {
	m.errorLogLineInput = ""
	if m.errorLogVisualMode == 'v' {
		// Clamp to line length.
		reversed := ui.FilteredErrorLogEntries(m.errorLog, m.showDebugLogs)
		if m.errorLogCursorLine < len(reversed) {
			lineLen := len([]rune(ui.ErrorLogEntryPlainText(reversed[m.errorLogCursorLine])))
			if m.errorLogCursorCol < lineLen-1 {
				m.errorLogCursorCol++
			}
		}
	}
	return m, nil
}

func (m Model) handleErrorLogOverlayKeyZero() (tea.Model, tea.Cmd) {
	if m.errorLogLineInput != "" {
		m.errorLogLineInput += "0"
		return m, nil
	}
	if m.errorLogVisualMode == 'v' {
		m.errorLogCursorCol = 0
	}
	return m, nil
}

func (m Model) handleErrorLogOverlayKeyDollar() (tea.Model, tea.Cmd) {
	m.errorLogLineInput = ""
	if m.errorLogVisualMode == 'v' {
		reversed := ui.FilteredErrorLogEntries(m.errorLog, m.showDebugLogs)
		if m.errorLogCursorLine < len(reversed) {
			lineLen := len([]rune(ui.ErrorLogEntryPlainText(reversed[m.errorLogCursorLine])))
			m.errorLogCursorCol = max(lineLen-1, 0)
		}
	}
	return m, nil
}

func (m Model) handleErrorLogOverlayKeyD() (tea.Model, tea.Cmd) {
	m.errorLogLineInput = ""
	if m.errorLogVisualMode != 0 {
		// Don't toggle debug in visual mode — 'd' is ambiguous.
		return m, nil
	}
	m.showDebugLogs = !m.showDebugLogs
	m.errorLogScroll = 0
	m.errorLogCursorLine = 0
	return m, nil
}

func (m Model) handleErrorLogOverlayKeyG() (tea.Model, tea.Cmd) {
	m.errorLogLineInput = ""
	if m.pendingG {
		m.pendingG = false
		m.errorLogCursorLine = 0
		m.errorLogScroll = 0
		return m, nil
	}
	m.pendingG = true
	return m, nil
}
