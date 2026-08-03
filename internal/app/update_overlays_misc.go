package app

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// handleNetworkPolicyOverlayKey handles keyboard input in the network policy visualizer overlay.
func (m Model) handleNetworkPolicyOverlayKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if m.netpolSearchActive {
		return m.handleNetpolSearchKey(msg)
	}
	switch msg.String() {
	case "esc":
		// Esc clears an active search first; a second Esc closes.
		if m.netpolSearchQuery != "" {
			m.netpolSearchQuery = ""
			m.netpolSearchInput.Clear()
			m.netpolSearchPos = 0
			return m, nil
		}
		return m.closeNetpolOverlay(), nil
	case "q":
		return m.closeNetpolOverlay(), nil
	case ui.ActiveKeybindings.Search:
		m.pendingG = false
		m.netpolLineInput = ""
		m.netpolSearchActive = true
		m.netpolSearchInput.Clear()
	case "n":
		m.netpolLineInput = ""
		return m.findNetpolMatch(true)
	case "N":
		m.netpolLineInput = ""
		return m.findNetpolMatch(false)
	case "j", "down":
		m.netpolLineInput = ""
		m.netpolScroll = min(m.netpolScroll+1, m.netpolMaxScroll())
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
			m.netpolScroll = min(lineNum, m.netpolMaxScroll())
		} else {
			m.netpolScroll = m.netpolMaxScroll()
		}
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.netpolLineInput += msg.String()
		return m, nil
	case "0":
		if m.netpolLineInput != "" {
			m.netpolLineInput += "0"
			return m, nil
		}
	case "ctrl+d", "shift+down":
		m.netpolLineInput = ""
		m.netpolScroll = min(m.netpolScroll+m.height/2, m.netpolMaxScroll())
	case "ctrl+u", "shift+up":
		m.netpolLineInput = ""
		m.netpolScroll = max(m.netpolScroll-m.height/2, 0)
	case "ctrl+f", "pgdown":
		m.netpolLineInput = ""
		m.netpolScroll = min(m.netpolScroll+m.height, m.netpolMaxScroll())
	case "ctrl+b", "pgup":
		m.netpolLineInput = ""
		m.netpolScroll = max(m.netpolScroll-m.height, 0)
	case "home":
		m.pendingG = false
		m.netpolLineInput = ""
		m.netpolScroll = 0
	case "end":
		m.netpolLineInput = ""
		m.netpolScroll = m.netpolMaxScroll()
	default:
		m.netpolLineInput = ""
	}
	return m, nil
}

func (m Model) handleFilterPresetOverlayKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
	if len(filtered) == 0 {
		// loadPreview short-circuits when nothing is selected, so the
		// previously-loaded children / YAML / metrics / events / map
		// would otherwise sit in the right pane describing a pod that
		// no longer matches. Drop them in step with the empty middle
		// column. requestGen++ also discards any in-flight preview load
		// from the prior cursor — its gen-gated handler will skip the
		// previewLoading=false reset, so do that here too or the
		// renderer (rightItems == nil && previewLoading) keeps showing
		// the spinner instead of "No resources found".
		m.requestGen++
		m.previewLoading = false
		m.rightItems = nil
		m.previewYAML = ""
		m.previewScroll = 0
		m.resourceTree = nil
		m.metricsContent = ""
		m.previewEventsContent = ""
		m.metricsData = nil
		m.metricsLoading = false
		m.previewEventsData = nil
	}
	m.setStatusMessage(fmt.Sprintf("Filter: %s (%d matches)", preset.Name, len(filtered)), false)
	return m, tea.Batch(scheduleStatusClear(), m.loadPreview())
}

// handleAlertsOverlayKey handles keyboard input for the alerts overlay.
func (m Model) handleAlertsOverlayKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
	case "ctrl+d", "shift+down":
		m.alertsLineInput = ""
		m.alertsScroll += 10
		return m, nil
	case "ctrl+u", "shift+up":
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

func (m Model) handleBatchLabelOverlayKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
		// Belt-and-suspenders read-only gate: the dispatcher already blocks
		// "Labels / Annotations" upstream, but a user who toggled RO on
		// while this overlay was open could otherwise commit a mutation.
		if m.readOnly {
			m.overlay = overlayNone
			m.setStatusMessage(readOnlyBlockedMessage("Labels / Annotations"), true)
			return m, scheduleStatusClear()
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
		if msg.Text != "" {
			m.batchLabelInput.Insert(msg.Text)
		}
		return m, nil
	}
}

func (m Model) handleActionOverlayKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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

func (m Model) handleOverlayKeyOverlayQuotaDashboard(msg tea.KeyPressMsg) Model {
	if msg.String() == "esc" || msg.String() == "q" {
		m.overlay = overlayNone
	}
	return m
}
