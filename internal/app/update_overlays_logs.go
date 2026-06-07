package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) handlePodSelectOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.logView.podFilterActive {
		return m.handleLogPodFilterMode(msg)
	}

	items := m.filteredLogPodItems()

	switch msg.String() {
	case "esc", "q":
		if m.logView.podFilterText != "" {
			m.logView.podFilterText = ""
			m.overlayCursor = 0
			return m, nil
		}
		m.overlay = overlayNone
		m.pendingAction = ""
		m.logView.podFilterText = ""
		m.logView.podFilterActive = false
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
			m.logView.podFilterText = ""
			m.logView.podFilterActive = false
			if m.pendingAction == "Go to Pod" {
				m.pendingAction = ""
				return m.navigateToOwner("Pod", sel.Name)
			}
			if m.pendingAction == "Logs" {
				m.pendingAction = ""
				return m.executeAction("Logs")
			}
			if m.pendingAction == "Tail Logs" {
				m.pendingAction = ""
				return m.executeAction("Tail Logs")
			}
			return m, m.loadContainersForAction()
		}
		m.overlay = overlayNone
		return m, nil
	case "/":
		m.logView.podFilterActive = true
		m.logView.podFilterText = ""
		return m, nil
	case "j", "down", "ctrl+n":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 1, len(items)-1)
		return m, nil
	case "k", "up", "ctrl+p":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -1, len(items)-1)
		return m, nil
	case "ctrl+d", "shift+down":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 10, len(items)-1)
		return m, nil
	case "ctrl+u", "shift+up":
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
	if m.logView.podFilterActive {
		return m.handleLogPodFilterMode(msg)
	}

	items := m.filteredLogPodItems()

	switch msg.String() {
	case "esc", "q":
		if m.logView.podFilterText != "" {
			m.logView.podFilterText = ""
			m.overlayCursor = 0
			return m, nil
		}
		// Cancel pod selection and restart the previous log stream.
		m.overlay = overlayNone
		m.pendingAction = ""
		m.logView.podFilterText = ""
		m.logView.podFilterActive = false
		if m.logView.savedPodName != "" {
			m.actionCtx.name = m.logView.savedPodName
			m.actionCtx.kind = "Pod"
			m.actionCtx.containerName = ""
			m.logView.savedPodName = ""
			// Reset container selection and log viewer state before restarting.
			m.logView.selectedContainers = nil
			m.logView.containers = nil
			m.logView.lines = nil
			m.logView.scroll = 0
			m.logView.follow = true
			m.logView.tailLines = ui.ConfigLogTailLines
			m.logView.hasMoreHistory = true
			m.logView.loadingHistory = false
			m.logView.title = "Logs: " + resourceTitleLabel(m.actionCtx.kind, m.actionNamespace(), m.actionCtx.name)
			return m, m.startLogStream()
		}
		return m, nil
	case "enter":
		if m.overlayCursor >= 0 && m.overlayCursor < len(items) {
			cmd := m.applyLogPodSelection(items[m.overlayCursor])
			return m, cmd
		}
		return m, nil
	case "/":
		m.logView.podFilterActive = true
		m.logView.podFilterText = ""
		return m, nil
	case "j", "down", "ctrl+n":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 1, len(items)-1)
		return m, nil
	case "k", "up", "ctrl+p":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -1, len(items)-1)
		return m, nil
	case "ctrl+d", "shift+down":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 10, len(items)-1)
		return m, nil
	case "ctrl+u", "shift+up":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -10, len(items)-1)
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

// applyLogPodSelection switches the log viewer to the given pod selection
// and returns a fresh log stream command. Shared by the normal-mode Enter
// handler and the filter-mode single-result fast-path so both commit
// paths stay in lockstep.
func (m *Model) applyLogPodSelection(sel model.Item) tea.Cmd {
	m.overlay = overlayNone
	m.pendingAction = ""
	m.logView.savedPodName = ""
	m.logView.podFilterText = ""
	m.logView.podFilterActive = false
	m.logView.selectedContainers = nil
	m.logView.containers = nil
	m.logView.lines = nil
	m.logView.scroll = 0
	m.logView.tailLines = ui.ConfigLogTailLines
	m.logView.hasMoreHistory = true
	m.logView.loadingHistory = false

	if sel.Status == "all" {
		// "All Pods" selected: stream all pods using the parent resource.
		m.actionCtx.kind = m.logView.parentKind
		m.actionCtx.name = m.logView.parentName
		m.actionCtx.containerName = ""
		m.logView.title = "Logs: " + resourceTitleLabel(m.actionCtx.kind, m.actionNamespace(), m.logView.parentName) + " (all pods)"
	} else {
		// Specific pod selected.
		m.actionCtx.name = sel.Name
		m.actionCtx.kind = "Pod"
		if sel.Namespace != "" {
			m.actionCtx.namespace = sel.Namespace
		}
		m.actionCtx.containerName = ""
		m.logView.title = "Logs: " + resourceTitleLabel(m.actionCtx.kind, m.actionNamespace(), m.actionCtx.name)
	}
	return m.startLogStream()
}

// handleLogPodFilterMode handles keyboard input while the pod selector filter is active.
func (m Model) handleLogPodFilterMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	fi := &stringFilterInput{ptr: &m.logView.podFilterText}
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
		m.logView.podFilterActive = false
		m.logView.podFilterText = ""
		m.overlayCursor = 0
		return m, nil
	case filterAccept:
		m.logView.podFilterActive = false
		m.overlayCursor = 0
		// When the filter narrows to a single pod, Enter is unambiguous:
		// apply it and start streaming. Without this, the user has to press
		// Enter twice (once to leave filter mode, once to commit) on a
		// one-row list.
		items := m.filteredLogPodItems()
		if len(items) == 1 {
			cmd := m.applyLogPodSelection(items[0])
			return m, cmd
		}
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
	if m.logView.containerFilterActive {
		return m.handleLogContainerFilterMode(msg)
	}

	items := m.filteredLogContainerItems()

	switch msg.String() {
	case "esc", "q":
		if m.logView.containerFilterText != "" {
			m.logView.containerFilterText = ""
			m.overlayCursor = 0
			return m, nil
		}
		// Close overlay without changes.
		m.overlay = overlayNone
		m.logView.containerFilterText = ""
		m.logView.containerFilterActive = false
		return m, nil
	case " ":
		m.logView.containerSelectionModified = true
		// Toggle selection (namespace-selector style).
		if m.overlayCursor >= 0 && m.overlayCursor < len(items) {
			item := items[m.overlayCursor]
			if item.Status == "all" {
				// "All Containers" selected: reset to all.
				m.logView.selectedContainers = nil
			} else {
				containerName := item.Name
				if len(m.logView.selectedContainers) == 0 {
					// Currently "all" selected; user selects one = select only that one.
					m.logView.selectedContainers = []string{containerName}
				} else {
					// Check if container is currently selected.
					found := -1
					for i, sc := range m.logView.selectedContainers {
						if sc == containerName {
							found = i
							break
						}
					}
					if found >= 0 {
						// Deselect: remove from list (but don't allow empty).
						if len(m.logView.selectedContainers) > 1 {
							m.logView.selectedContainers = append(m.logView.selectedContainers[:found], m.logView.selectedContainers[found+1:]...)
						}
					} else {
						// Select: add to list.
						m.logView.selectedContainers = append(m.logView.selectedContainers, containerName)
					}
					// If all containers are now selected, reset to nil (meaning "all").
					if len(m.logView.selectedContainers) >= len(m.logView.containers) {
						m.logView.selectedContainers = nil
					}
				}
			}
		}
		return m, nil
	case "enter":
		// Apply selection and close overlay (namespace-selector style).
		switch {
		case m.logView.containerSelectionModified:
			// User toggled with Space: apply those selections.
		case m.overlayCursor >= 0 && m.overlayCursor < len(items) && items[m.overlayCursor].Status != "all":
			// No Space toggling: apply cursor item as single selection.
			m.logView.selectedContainers = []string{items[m.overlayCursor].Name}
		default:
			// Cursor on "All Containers" or no item.
			m.logView.selectedContainers = nil
		}
		m.overlay = overlayNone
		m.logView.containerFilterText = ""
		m.logView.containerFilterActive = false
		m.logView.containerSelectionModified = false
		// Call restartLogStreamForContainerFilter before return so that the
		// pointer-receiver mutations (new logCh, logCancel, etc.) are reflected
		// in the returned Model value.
		cmd := m.restartLogStreamForContainerFilter()
		return m, cmd
	case "/":
		m.logView.containerFilterActive = true
		m.logView.containerFilterText = ""
		return m, nil
	case "j", "down", "ctrl+n":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 1, len(items)-1)
		return m, nil
	case "k", "up", "ctrl+p":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -1, len(items)-1)
		return m, nil
	case "ctrl+d", "shift+down":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 10, len(items)-1)
		return m, nil
	case "ctrl+u", "shift+up":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -10, len(items)-1)
		return m, nil
	case "P", "\\":
		// Switch to pod selector if available (group resources like Deployment).
		if m.logView.parentKind != "" {
			m.overlay = overlayNone
			m.logView.containerFilterText = ""
			m.logView.containerFilterActive = false
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
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

// handleLogContainerFilterMode handles keyboard input while the container selector filter is active.
func (m Model) handleLogContainerFilterMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	fi := &stringFilterInput{ptr: &m.logView.containerFilterText}
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
		m.logView.containerFilterActive = false
		m.logView.containerFilterText = ""
		m.overlayCursor = 0
		return m, nil
	case filterAccept:
		m.logView.containerFilterActive = false
		m.overlayCursor = 0
		// When the filter narrows to a single container and the user hasn't
		// been Space-toggling selections, Enter is unambiguous: apply that
		// container and restart the stream. Multi-select via Space is
		// preserved: if the user already toggled selections, Enter still
		// just exits filter mode without replacing them.
		if !m.logView.containerSelectionModified {
			items := m.filteredLogContainerItems()
			if len(items) == 1 {
				if items[0].Status == "all" {
					m.logView.selectedContainers = nil
				} else {
					m.logView.selectedContainers = []string{items[0].Name}
				}
				m.overlay = overlayNone
				m.logView.containerFilterText = ""
				cmd := m.restartLogStreamForContainerFilter()
				return m, cmd
			}
		}
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
	if m.logView.cancel != nil {
		m.logView.cancel()
	}
	if m.logView.historyCancel != nil {
		m.logView.historyCancel()
		m.logView.historyCancel = nil
	}
	// Clear single-container override so startLogStream uses --all-containers --prefix,
	// which is required for the prefix-based container filtering to work.
	m.actionCtx.containerName = ""
	m.logView.lines = nil
	m.logView.scroll = 0
	m.logView.cursor = 0
	m.logView.follow = true
	m.logView.visualMode = false
	m.logView.tailLines = ui.ConfigLogTailLines
	m.logView.hasMoreHistory = !m.logView.previous && !m.logView.isMulti
	m.logView.loadingHistory = false
	// Update the log title to show selected containers.
	m.logView.title = m.buildLogTitle()
	return m.startLogStream()
}

// buildLogTitle constructs the log title, including selected container names if filtered.
func (m *Model) buildLogTitle() string {
	base := "Logs: " + resourceTitleLabel(m.actionCtx.kind, m.actionNamespace(), m.actionCtx.name)
	if len(m.logView.selectedContainers) > 0 && len(m.logView.selectedContainers) < len(m.logView.containers) {
		base += " [" + strings.Join(m.logView.selectedContainers, ", ") + "]"
	}
	return base
}
