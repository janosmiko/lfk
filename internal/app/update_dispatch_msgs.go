package app

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) updateCommandBarResult(msg commandBarResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		errMsg := msg.err.Error()
		if msg.output != "" {
			errMsg = strings.TrimSpace(msg.output)
		}
		m.addLogEntry("ERR", errMsg)
		// Show error output in describe view if there's content, otherwise status bar.
		if msg.output != "" {
			m.mode = modeDescribe
			m.describeContent = strings.TrimSpace(msg.output)
			m.describeScroll = 0
			m.describeCursor = 0
			m.describeCursorCol = 0
			m.describeTitle = "Command Output (error)"
			return m, nil
		}
		m.setErrorFromErr("Command failed: ", fmt.Errorf("%s", errMsg))
		return m, scheduleStatusClear()
	}
	output := strings.TrimSpace(msg.output)
	if output != "" {
		m.addLogEntry("INF", output)
		// Open output in the describe viewer (scrollable, searchable, wrappable).
		m.mode = modeDescribe
		m.describeContent = output
		m.describeScroll = 0
		m.describeCursor = 0
		m.describeCursorCol = 0
		m.describeTitle = "Command Output"
		return m, nil
	}
	m.setStatusMessage("Command completed (no output)", false)
	return m, scheduleStatusClear()
}

func (m Model) updateTriggerCronJob(msg triggerCronJobMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Trigger failed: ", msg.err)
	} else {
		m.setStatusMessage("Job created: "+msg.jobName, false)
	}
	m.loading = false
	return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())
}

func (m Model) updateBulkActionResult(msg bulkActionResultMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	m.bulkMode = false
	m.clearSelection()
	if msg.failed > 0 {
		errSummary := fmt.Sprintf("Bulk: %d succeeded, %d failed", msg.succeeded, msg.failed)
		if len(msg.errors) > 0 {
			errSummary += ": " + msg.errors[0]
		}
		m.setStatusMessage(errSummary, true)
	} else {
		m.setStatusMessage(fmt.Sprintf("Bulk: %d resources processed", msg.succeeded), false)
	}
	return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())
}

func (m Model) updateFinalizerSearchResult(msg finalizerSearchResultMsg) (tea.Model, tea.Cmd) {
	m.finalizerSearchLoading = false
	if msg.err != nil {
		m.setErrorFromErr("Finalizer search: ", msg.err)
		m.overlay = overlayNone
		return m, scheduleStatusClear()
	}
	m.finalizerSearchResults = msg.results
	if len(msg.results) == 0 {
		m.setStatusMessage("No resources found with matching finalizer", false)
		m.overlay = overlayNone
		return m, scheduleStatusClear()
	}
	return m, nil
}

func (m Model) updateFinalizerRemoveResult(msg finalizerRemoveResultMsg) (tea.Model, tea.Cmd) {
	m.overlay = overlayNone
	if msg.failed > 0 {
		m.setStatusMessage(fmt.Sprintf("Removed finalizer from %d resources, %d failed", msg.succeeded, msg.failed), true)
	} else {
		m.setStatusMessage(fmt.Sprintf("Removed finalizer from %d resources", msg.succeeded), false)
	}
	m.finalizerSearchResults = nil
	m.finalizerSearchSelected = nil
	return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())
}

func (m Model) updateStatusMessageExpired(msg statusMessageExpiredMsg) Model {
	// A prior scheduleStatusClear tick may arrive while a newer message is
	// still active. If the current message's expiration hasn't actually
	// passed, leave it alone — the newer message's own tick (or the view
	// layer's time-based check) will clean it up at the right time.
	if m.statusMessage != "" && time.Now().Before(m.statusMessageExp) {
		return m
	}
	m.statusMessage = ""
	m.statusMessageTip = false
	return m
}

func (m Model) updateStartupTip(msg startupTipMsg) (tea.Model, tea.Cmd) {
	m.setStatusMessage("Tip: "+msg.tip, false)
	m.statusMessageTip = true
	return m, scheduleStatusClear()
}

func (m Model) updateWatchTick(msg watchTickMsg) (tea.Model, tea.Cmd) {
	if !m.watchMode {
		return m, nil
	}
	// Mark this dispatch as a watch-tick refresh so the instrumented
	// loaders called below (through refreshCurrentLevel) use
	// Registry.StartUntracked and don't flash the title-bar indicator
	// every 2 seconds.
	//
	// trackBgTask captures the decision synchronously at construction
	// time, so we only need the flag true for the duration of the
	// refreshCurrentLevel() call. Reset it to false before returning so
	// the flag doesn't leak into subsequent user-driven Updates — the
	// returned model becomes the framework's next state, and any
	// navigation that happens after this watch tick must see a clean
	// flag or its loaders would also call StartUntracked and the
	// indicator would never appear for user actions.
	m.suppressBgtasks = true
	cmd := tea.Batch(m.refreshCurrentLevel(), scheduleWatchTick(m.watchInterval))
	m.suppressBgtasks = false
	return m, cmd
}

// dispatchNavigationTick fans out tick messages whose handlers belong with
// cursor/navigation state. Kept as a wrapper so updateResourceMsg's switch
// stays under the gocyclo threshold (see .golangci config).
func (m Model) dispatchNavigationTick(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case watchTickMsg:
		return m.updateWatchTick(msg)
	case previewDebounceTickMsg:
		return m.updatePreviewDebounceTick(msg)
	}
	return m, nil
}

func (m Model) updatePreviewDebounceTick(msg previewDebounceTickMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.previewDebounceGen {
		return m, nil
	}
	if m.mapView {
		return m, tea.Batch(m.loadPreview(), m.loadResourceTree())
	}
	return m, m.loadPreview()
}

func (m Model) updateEventTimeline(msg eventTimelineMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setStatusMessage(fmt.Sprintf("Event load failed: %v", msg.err), true)
		return m, scheduleStatusClear()
	}
	if len(msg.events) == 0 {
		m.setStatusMessage("No events found", false)
		return m, scheduleStatusClear()
	}
	m.eventTimelineData = msg.events
	m.eventTimelineScroll = 0
	m.eventTimelineCursor = 0
	m.eventTimelineCursorCol = 0
	m.eventTimelineVisualMode = 0
	m.eventTimelineSearchQuery = ""
	m.eventTimelineSearchActive = false
	m.eventTimelineFullscreen = false
	m.eventTimelineLines = m.buildEventTimelineLines()
	m.overlay = overlayEventTimeline
	return m, nil
}

func (m Model) updateDashboardLoaded(msg dashboardLoadedMsg) Model {
	if msg.context == m.nav.Context {
		m.dashboardPreview = msg.content
		m.dashboardEventsPreview = msg.events
	}
	return m
}

func (m Model) updateMonitoringDashboard(msg monitoringDashboardMsg) Model {
	if msg.context == m.nav.Context {
		m.monitoringPreview = msg.content
	}
	return m
}
