package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) handlePodSelectOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.logPodFilterActive {
		return m.handleLogPodFilterMode(msg)
	}

	items := m.filteredLogPodItems()

	switch msg.String() {
	case "esc", "q":
		if m.logPodFilterText != "" {
			m.logPodFilterText = ""
			m.overlayCursor = 0
			return m, nil
		}
		m.overlay = overlayNone
		m.pendingAction = ""
		m.logPodFilterText = ""
		m.logPodFilterActive = false
		return m, nil
	case "enter":
		if m.overlayCursor >= 0 && m.overlayCursor < len(items) {
			sel := items[m.overlayCursor]
			m.actionCtx.name = sel.Name
			m.actionCtx.kind = "Pod"
			if sel.Namespace != "" {
				m.actionCtx.namespace = sel.Namespace
			}
			m.overlay = overlayNone
			m.logPodFilterText = ""
			m.logPodFilterActive = false
			if m.pendingAction == "Go to Pod" {
				m.pendingAction = ""
				return m.navigateToOwner("Pod", sel.Name)
			}
			if m.pendingAction == "Logs" {
				m.pendingAction = ""
				return m.executeAction("Logs")
			}
			return m, m.loadContainersForAction()
		}
		m.overlay = overlayNone
		return m, nil
	case "/":
		m.logPodFilterActive = true
		m.logPodFilterText = ""
		return m, nil
	case "j", "down", "ctrl+n":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 1, len(items)-1)
		return m, nil
	case "k", "up", "ctrl+p":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -1, len(items)-1)
		return m, nil
	case "ctrl+d":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 10, len(items)-1)
		return m, nil
	case "ctrl+u":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -10, len(items)-1)
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

// handleLogPodSelectOverlayKey handles keyboard input for the inline pod selector
// overlay shown within the log viewer (triggered by pressing P while viewing logs).
func (m Model) handleLogPodSelectOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.logPodFilterActive {
		return m.handleLogPodFilterMode(msg)
	}

	items := m.filteredLogPodItems()

	switch msg.String() {
	case "esc", "q":
		if m.logPodFilterText != "" {
			m.logPodFilterText = ""
			m.overlayCursor = 0
			return m, nil
		}
		// Cancel pod selection and restart the previous log stream.
		m.overlay = overlayNone
		m.pendingAction = ""
		m.logPodFilterText = ""
		m.logPodFilterActive = false
		if m.logSavedPodName != "" {
			m.actionCtx.name = m.logSavedPodName
			m.actionCtx.kind = "Pod"
			m.actionCtx.containerName = ""
			m.logSavedPodName = ""
			// Reset container selection and log viewer state before restarting.
			m.logSelectedContainers = nil
			m.logContainers = nil
			m.logLines = nil
			m.logVisibleIndices = nil
			m.logRules = nil
			m.logIncludeMode = IncludeAny
			m.logFilterChain = NewFilterChain(nil, IncludeAny, m.logSeverityDetector)
			m.logScroll = 0
			m.logFollow = true
			m.logTailLines = ui.ConfigLogTailLines
			m.logHasMoreHistory = true
			m.logLoadingHistory = false
			m.logTitle = fmt.Sprintf("Logs: %s/%s", m.actionNamespace(), m.actionCtx.name)
			return m, m.startLogStream()
		}
		return m, nil
	case "enter":
		if m.overlayCursor >= 0 && m.overlayCursor < len(items) {
			sel := items[m.overlayCursor]
			m.overlay = overlayNone
			m.pendingAction = ""
			m.logSavedPodName = ""
			m.logPodFilterText = ""
			m.logPodFilterActive = false
			m.logSelectedContainers = nil
			m.logContainers = nil
			m.logLines = nil
			m.logVisibleIndices = nil
			m.logRules = nil
			m.logIncludeMode = IncludeAny
			m.logFilterChain = NewFilterChain(nil, IncludeAny, m.logSeverityDetector)
			m.logScroll = 0
			m.logTailLines = ui.ConfigLogTailLines
			m.logHasMoreHistory = true
			m.logLoadingHistory = false

			if sel.Status == "all" {
				// "All Pods" selected: stream all pods using the parent resource.
				m.actionCtx.kind = m.logParentKind
				m.actionCtx.name = m.logParentName
				m.actionCtx.containerName = ""
				m.logTitle = fmt.Sprintf("Logs: %s/%s (all pods)", m.actionNamespace(), m.logParentName)
			} else {
				// Specific pod selected.
				m.actionCtx.name = sel.Name
				m.actionCtx.kind = "Pod"
				if sel.Namespace != "" {
					m.actionCtx.namespace = sel.Namespace
				}
				m.actionCtx.containerName = ""
				m.logTitle = fmt.Sprintf("Logs: %s/%s", m.actionNamespace(), m.actionCtx.name)
			}
			return m, m.startLogStream()
		}
		return m, nil
	case "/":
		m.logPodFilterActive = true
		m.logPodFilterText = ""
		return m, nil
	case "j", "down", "ctrl+n":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 1, len(items)-1)
		return m, nil
	case "k", "up", "ctrl+p":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -1, len(items)-1)
		return m, nil
	case "ctrl+d":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 10, len(items)-1)
		return m, nil
	case "ctrl+u":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -10, len(items)-1)
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

// handleLogPodFilterMode handles keyboard input while the pod selector filter is active.
func (m Model) handleLogPodFilterMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	fi := &stringFilterInput{ptr: &m.logPodFilterText}
	// Handle paste events.
	if msg.Paste {
		switch handlePastedText(fi, msg.Runes) {
		case filterContinue:
			m.overlayCursor = 0
			return m, nil
		case filterPasteMultiline:
			m.triggerPasteConfirm(strings.TrimRight(string(msg.Runes), "\n"), pasteTargetLogPodFilter)
			return m, nil
		}
		return m, nil
	}
	switch handleFilterKey(fi, msg.String()) {
	case filterEscape:
		m.logPodFilterActive = false
		m.logPodFilterText = ""
		m.overlayCursor = 0
		return m, nil
	case filterAccept:
		m.logPodFilterActive = false
		m.overlayCursor = 0
		return m, nil
	case filterClose:
		return m.closeTabOrQuit()
	case filterContinue:
		m.overlayCursor = 0
		return m, nil
	}
	return m, nil
}

// handleLogContainerSelectOverlayKey handles keyboard input for the log container
// filter overlay (triggered by pressing C in the log viewer).
func (m Model) handleLogContainerSelectOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.logContainerFilterActive {
		return m.handleLogContainerFilterMode(msg)
	}

	items := m.filteredLogContainerItems()

	switch msg.String() {
	case "esc", "q":
		if m.logContainerFilterText != "" {
			m.logContainerFilterText = ""
			m.overlayCursor = 0
			return m, nil
		}
		// Close overlay without changes.
		m.overlay = overlayNone
		m.logContainerFilterText = ""
		m.logContainerFilterActive = false
		return m, nil
	case " ":
		m.logContainerSelectionModified = true
		// Toggle selection (namespace-selector style).
		if m.overlayCursor >= 0 && m.overlayCursor < len(items) {
			item := items[m.overlayCursor]
			if item.Status == "all" {
				// "All Containers" selected: reset to all.
				m.logSelectedContainers = nil
			} else {
				containerName := item.Name
				if len(m.logSelectedContainers) == 0 {
					// Currently "all" selected; user selects one = select only that one.
					m.logSelectedContainers = []string{containerName}
				} else {
					// Check if container is currently selected.
					found := -1
					for i, sc := range m.logSelectedContainers {
						if sc == containerName {
							found = i
							break
						}
					}
					if found >= 0 {
						// Deselect: remove from list (but don't allow empty).
						if len(m.logSelectedContainers) > 1 {
							m.logSelectedContainers = append(m.logSelectedContainers[:found], m.logSelectedContainers[found+1:]...)
						}
					} else {
						// Select: add to list.
						m.logSelectedContainers = append(m.logSelectedContainers, containerName)
					}
					// If all containers are now selected, reset to nil (meaning "all").
					if len(m.logSelectedContainers) >= len(m.logContainers) {
						m.logSelectedContainers = nil
					}
				}
			}
		}
		return m, nil
	case "enter":
		// Apply selection and close overlay (namespace-selector style).
		switch {
		case m.logContainerSelectionModified:
			// User toggled with Space: apply those selections.
		case m.overlayCursor >= 0 && m.overlayCursor < len(items) && items[m.overlayCursor].Status != "all":
			// No Space toggling: apply cursor item as single selection.
			m.logSelectedContainers = []string{items[m.overlayCursor].Name}
		default:
			// Cursor on "All Containers" or no item.
			m.logSelectedContainers = nil
		}
		m.overlay = overlayNone
		m.logContainerFilterText = ""
		m.logContainerFilterActive = false
		m.logContainerSelectionModified = false
		// Call restartLogStreamForContainerFilter before return so that the
		// pointer-receiver mutations (new logCh, logCancel, etc.) are reflected
		// in the returned Model value.
		cmd := m.restartLogStreamForContainerFilter()
		return m, cmd
	case "/":
		m.logContainerFilterActive = true
		m.logContainerFilterText = ""
		return m, nil
	case "j", "down", "ctrl+n":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 1, len(items)-1)
		return m, nil
	case "k", "up", "ctrl+p":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -1, len(items)-1)
		return m, nil
	case "ctrl+d":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 10, len(items)-1)
		return m, nil
	case "ctrl+u":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -10, len(items)-1)
		return m, nil
	case "P", "\\":
		// Switch to pod selector if available (group resources like Deployment).
		if m.logParentKind != "" {
			m.overlay = overlayNone
			m.logContainerFilterText = ""
			m.logContainerFilterActive = false
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
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

// handleLogContainerFilterMode handles keyboard input while the container selector filter is active.
func (m Model) handleLogContainerFilterMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	fi := &stringFilterInput{ptr: &m.logContainerFilterText}
	// Handle paste events.
	if msg.Paste {
		switch handlePastedText(fi, msg.Runes) {
		case filterContinue:
			m.overlayCursor = 0
			return m, nil
		case filterPasteMultiline:
			m.triggerPasteConfirm(strings.TrimRight(string(msg.Runes), "\n"), pasteTargetLogContainerFilter)
			return m, nil
		}
		return m, nil
	}
	switch handleFilterKey(fi, msg.String()) {
	case filterEscape:
		m.logContainerFilterActive = false
		m.logContainerFilterText = ""
		m.overlayCursor = 0
		return m, nil
	case filterAccept:
		m.logContainerFilterActive = false
		m.overlayCursor = 0
		return m, nil
	case filterClose:
		return m.closeTabOrQuit()
	case filterContinue:
		m.overlayCursor = 0
		return m, nil
	}
	return m, nil
}

// restartLogStreamForContainerFilter cancels the current log stream and restarts
// it with the updated container filter.
func (m *Model) restartLogStreamForContainerFilter() tea.Cmd {
	if m.logCancel != nil {
		m.logCancel()
	}
	if m.logHistoryCancel != nil {
		m.logHistoryCancel()
		m.logHistoryCancel = nil
	}
	// Clear single-container override so startLogStream uses --all-containers --prefix,
	// which is required for the prefix-based container filtering to work.
	m.actionCtx.containerName = ""
	m.logLines = nil
	m.logVisibleIndices = nil
	m.logScroll = 0
	m.logCursor = 0
	m.logFollow = true
	m.logVisualMode = false
	m.logTailLines = ui.ConfigLogTailLines
	m.logHasMoreHistory = !m.logPrevious && !m.logIsMulti
	m.logLoadingHistory = false
	// Update the log title to show selected containers.
	m.logTitle = m.buildLogTitle()
	return m.startLogStream()
}

// buildLogTitle constructs the log title, including selected container names if filtered.
func (m *Model) buildLogTitle() string {
	base := fmt.Sprintf("Logs: %s/%s", m.actionNamespace(), m.actionCtx.name)
	if len(m.logSelectedContainers) > 0 && len(m.logSelectedContainers) < len(m.logContainers) {
		base += " [" + strings.Join(m.logSelectedContainers, ", ") + "]"
	}
	return base
}
