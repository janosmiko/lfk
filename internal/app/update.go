package app

import (
	"context"
	"errors"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/creack/pty"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/ui"
)

// yamlFoldPrefixLen is the number of characters prepended by buildVisibleLines
// for fold indicators (always "  ", 2 chars). Cursor columns operate on these
// prefixed lines, so the first content character is at index yamlFoldPrefixLen.
const yamlFoldPrefixLen = 2

// isContextCanceled returns true if the error is due to an intentional context cancellation.
// It checks both Go context errors and string-based "context canceled" messages from kubectl.
func isContextCanceled(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "context canceled") || strings.Contains(msg, "context deadline exceeded")
}

// Update handles messages and re-arms the spinner loop centrally.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	mdl, cmd := m.updateImpl(msg)
	return armSpinner(mdl, cmd)
}

// updateImpl is the real message handler; Update wraps it to re-arm the spinner.
func (m Model) updateImpl(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Once graceful shutdown has begun, the drain goroutine owns the shared
	// managers; freeze the model so no concurrent Update path mutates state
	// underneath it. The only message that still matters is the drain's
	// completion, which triggers the actual quit.
	if m.shuttingDown {
		if _, ok := msg.(shutdownCompleteMsg); ok {
			return m, tea.Quit
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.updateWindowSize(msg)
	case tea.MouseMsg:
		return m.handleMouse(msg)
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case tea.PasteMsg:
		// v2 splits bracketed paste out of the key stream; re-encode it so it
		// reaches the focused input through the same routing as key presses.
		return m.handleKey(pasteKey(msg.Content))
	case tea.FocusMsg:
		return m.updateFocus(msg)
	case tea.BlurMsg:
		return m.updateBlur(msg)
	case spinner.TickMsg:
		return m.updateTick(msg)
	case stderrCapturedMsg:
		m.setStatusMessage("stderr: "+msg.message, true)
		return m, tea.Batch(scheduleStatusClear(), m.waitForStderr())
	case loggerUIMsg:
		m.appendLoggerUIEntry(logger.UIEntry(msg))
		return m, waitForLoggerUI()
	default:
		if dark, ok := ui.ParseColorModeMsg(msg); ok {
			ui.SetColorMode(dark)
			// The scheme is applied outside the model, so the cached previews
			// still hold the previous theme's baked colors; re-render them now
			// instead of leaving them stale until the next data tick.
			return m.recomposeThemedContent(), nil
		}
		if mdl, cmd, ok := m.updateResourceMsg(msg); ok {
			return mdl, cmd
		}
		if mdl, cmd, ok := m.updateResultMsg(msg); ok {
			return mdl, cmd
		}
	}
	return m, nil
}

// updateResourceMsg handles resource-loading and navigation-related messages.
func (m Model) updateResourceMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) { //nolint:gocyclo // flat type-switch dispatcher: complexity is "number of message types we route", not branching depth
	switch msg := msg.(type) {
	case contextsLoadedMsg:
		mdl, cmd := m.updateContextsLoaded(msg)
		return mdl, cmd, true
	case resourceTypesMsg:
		mdl, cmd := m.updateResourceTypes(msg)
		return mdl, cmd, true
	case apiResourceDiscoveryMsg:
		mdl, cmd := m.updateAPIResourceDiscovery(msg)
		return mdl, cmd, true
	case discoveryCacheLoadedMsg:
		mdl := m.updateDiscoveryCacheLoaded(msg)
		return mdl, nil, true
	case resourcesLoadedMsg:
		mdl, cmd := m.updateResourcesLoaded(msg)
		return mdl, cmd, true
	case ownedLoadedMsg:
		mdl, cmd := m.updateOwnedLoaded(msg)
		return mdl, cmd, true
	case resourceTreeLoadedMsg:
		mdl, cmd := m.updateResourceTreeLoaded(msg)
		return mdl, cmd, true
	case containersLoadedMsg:
		mdl, cmd := m.updateContainersLoaded(msg)
		return mdl, cmd, true
	case namespacesLoadedMsg:
		mdl, cmd := m.updateNamespacesLoaded(msg)
		return mdl, cmd, true
	case securityAvailabilityLoadedMsg:
		mdl, cmd := m.updateSecurityAvailabilityLoaded(msg)
		return mdl, cmd, true
	case securityFindingsLoadedMsg:
		mdl, cmd := m.updateSecurityFindingsLoaded(msg)
		return mdl, cmd, true
	case securityFindingsSeedMsg:
		return m.updateSecurityFindingsSeed(msg), nil, true
	case securityIgnoresSaveErrMsg:
		mdl, cmd := m.updateSecurityIgnoresSaveErr(msg)
		return mdl, cmd, true
	case yamlBlameLoadedMsg:
		mdl, cmd := m.updateYamlBlameLoaded(msg)
		return mdl, cmd, true
	case yamlLoadedMsg:
		mdl, cmd := m.updateYamlLoaded(msg)
		return mdl, cmd, true
	case previewYAMLLoadedMsg:
		mdl := m.updatePreviewYAMLLoaded(msg)
		return mdl, nil, true
	case containerPortsLoadedMsg:
		mdl := m.updateContainerPortsLoaded(msg)
		return mdl, nil, true
	case portForwardStartedMsg:
		mdl, cmd := m.updatePortForwardStarted(msg)
		return mdl, cmd, true
	case portForwardStoppedMsg:
		mdl, cmd := m.updatePortForwardStopped(msg)
		return mdl, cmd, true
	case portForwardUpdateMsg:
		mdl, cmd := m.updatePortForwardUpdate(msg)
		return mdl, cmd, true
	case statusMessageExpiredMsg:
		mdl := m.updateStatusMessageExpired(msg)
		return mdl, nil, true
	case startupTipMsg:
		mdl, cmd := m.updateStartupTip(msg)
		return mdl, cmd, true
	case watchTickMsg, previewDebounceTickMsg:
		mdl, cmd := m.dispatchNavigationTick(msg)
		return mdl, cmd, true
	case podSelectMsg:
		mdl, cmd := m.updatePodSelect(msg)
		return mdl, cmd, true
	case podLogSelectMsg:
		mdl, cmd := m.updatePodLogSelect(msg)
		return mdl, cmd, true
	case containerSelectMsg:
		mdl, cmd := m.updateContainerSelect(msg)
		return mdl, cmd, true
	case eventTimelineMsg:
		mdl, cmd := m.updateEventTimeline(msg)
		return mdl, cmd, true
	case metricsLoadedMsg:
		mdl := m.updateMetricsLoaded(msg)
		return mdl, nil, true
	case previewEventsLoadedMsg:
		mdl := m.updatePreviewEventsLoaded(msg)
		return mdl, nil, true
	case previewSecretDataLoadedMsg:
		mdl := m.updatePreviewSecretDataLoaded(msg)
		return mdl, nil, true
	case previewServiceEndpointsLoadedMsg:
		mdl := m.updatePreviewServiceEndpointsLoaded(msg)
		return mdl, nil, true
	case whoCanLoadedMsg:
		mdl := m.updateWhoCanLoaded(msg)
		return mdl, nil, true
	case podMetricsEnrichedMsg:
		mdl := m.updatePodMetricsEnriched(msg)
		return mdl, nil, true
	case nodeMetricsEnrichedMsg:
		mdl := m.updateNodeMetricsEnriched(msg)
		return mdl, nil, true
	case nodeUptimeEnrichedMsg:
		mdl := m.updateNodeUptimeEnriched(msg)
		return mdl, nil, true
	case dashboardPartialMsg:
		mdl, cmd := m.handleDashboardPartial(msg)
		return mdl, cmd, true
	case dashboardLoadedMsg:
		mdl := m.updateDashboardLoaded(msg)
		return mdl, nil, true
	case monitoringDashboardMsg:
		mdl := m.updateMonitoringDashboard(msg)
		return mdl, nil, true
	case logContainersLoadedMsg:
		mdl, cmd := m.updateLogContainersLoaded(msg)
		return mdl, cmd, true
	case localClustersDetectedMsg:
		mdl, cmd := m.updateLocalClustersDetected(msg)
		return mdl, cmd, true
	case localClusterCreatedMsg:
		mdl, cmd := m.updateLocalClusterCreated(msg)
		return mdl, cmd, true
	case localClusterMutatedMsg:
		mdl, cmd := m.updateLocalClusterMutated(msg)
		return mdl, cmd, true
	}

	return m.updateEasterEggMsg(msg)
}

// updateEasterEggMsg handles easter egg tick/clear messages.
func (m Model) updateEasterEggMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case whichKeyTickMsg:
		if m.pendingG {
			m.whichKey.shown = true
		}
		return m, nil, true
	case whichKeyLeaderTickMsg:
		// Drop ticks from a superseded arming (seq) or after disarm.
		if m.whichKey.armed && msg.seq == m.whichKey.seq {
			m.whichKey.shown = true
		}
		return m, nil, true
	case konamiClearMsg:
		m = m.clearKonami()
		return m, nil, true
	case nyanTickMsg:
		if m.nyanMode {
			m.nyanTick++
			return m, scheduleNyanTick(), true
		}
		return m, nil, true
	case creditsTickMsg:
		var stopped bool
		m, stopped = m.tickCredits()
		if stopped {
			// Content reached center -- wait 10 seconds then auto-close.
			return m, scheduleCreditsClose(), true
		}
		return m, scheduleCreditsScroll(), true
	case creditsCloseMsg:
		m.mode = modeExplorer
		m.creditsStopped = false
		return m, nil, true
	case kubetrisAnimTickMsg:
		// Visual-only animation countdown -- doesn't block gameplay.
		if m.mode == modeKubetris && m.kubetrisGame != nil && m.kubetrisGame.animating {
			m.kubetrisGame.animTicks--
			if m.kubetrisGame.animTicks <= 0 {
				m.kubetrisGame.finishAnimation()
			} else {
				return m, scheduleKubetrisAnimTick(), true
			}
		}
		return m, nil, true
	case kubetrisLockTickMsg:
		if m.mode == modeKubetris && m.kubetrisGame != nil {
			m.kubetrisGame.doLock()
			if m.kubetrisGame.gameOver {
				m.kubetrisGame.saveHighScore()
				return m, nil, true
			}
			if m.kubetrisGame.animating {
				return m, scheduleKubetrisAnimTick(), true
			}
		}
		return m, nil, true
	case kubetrisTickMsg:
		if m.mode == modeKubetris && m.kubetrisGame != nil && !m.kubetrisGame.paused && !m.kubetrisGame.gameOver {
			needsLock := m.kubetrisGame.tick()
			if m.kubetrisGame.gameOver {
				m.kubetrisGame.saveHighScore()
				return m, nil, true
			}
			var cmds []tea.Cmd
			cmds = append(cmds, m.scheduleKubetrisTick())
			if needsLock {
				cmds = append(cmds, scheduleKubetrisLockDelay())
			}
			if m.kubetrisGame.animating {
				cmds = append(cmds, scheduleKubetrisAnimTick())
			}
			return m, tea.Batch(cmds...), true
		}
		return m, nil, true
	}
	return m, nil, false
}

// updateResultMsg handles action results, editor operations, and other response messages.
func (m Model) updateResultMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	if mdl, cmd, ok := m.updateActionResultMsg(msg); ok {
		return mdl, cmd, true
	}
	return m.updateEditorResultMsg(msg)
}

// updateActionResultMsg handles action and command result messages.
func (m Model) updateActionResultMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case actionResultMsg:
		mdl, cmd := m.updateActionResult(msg)
		return mdl, cmd, true
	case commandBarResultMsg:
		mdl, cmd := m.updateCommandBarResult(msg)
		return mdl, cmd, true
	case triggerCronJobMsg:
		mdl, cmd := m.updateTriggerCronJob(msg)
		return mdl, cmd, true
	case bulkActionResultMsg:
		mdl, cmd := m.updateBulkActionResult(msg)
		return mdl, cmd, true
	case finalizerSearchResultMsg:
		mdl, cmd := m.updateFinalizerSearchResult(msg)
		return mdl, cmd, true
	case finalizerRemoveResultMsg:
		mdl, cmd := m.updateFinalizerRemoveResult(msg)
		return mdl, cmd, true
	case commandBarNamesFetchedMsg:
		if m.commandBarNameCache == nil {
			m.commandBarNameCache = make(map[string][]string)
		}
		m.commandBarNameCache[msg.cacheKey] = msg.names
		m.commandBarNameLoading = ""
		// Refresh suggestions if command bar is still active.
		if m.commandBarActive {
			m.commandBarSuggestions = m.generateCommandBarSuggestions()
		}
		return m, nil, true
	case yamlClipboardMsg:
		mdl, cmd := m.updateYamlClipboard(msg)
		return mdl, cmd, true
	case copyFieldManifestsMsg, taintsLoadedMsg, taintsAppliedMsg:
		mdl, cmd := m.routeOverlayEditorMsg(msg)
		return mdl, cmd, true
	case rbacCheckMsg:
		mdl, cmd := m.updateRbacCheck(msg)
		return mdl, cmd, true
	case canILoadedMsg:
		mdl, cmd := m.updateCanILoaded(msg)
		return mdl, cmd, true
	case canISAListMsg:
		mdl, cmd := m.updateCanISAList(msg)
		return mdl, cmd, true
	case podStartupMsg:
		mdl, cmd := m.updatePodStartup(msg)
		return mdl, cmd, true
	case crashInvestigationMsg:
		mdl, cmd := m.updateCrashInvestigation(msg)
		return mdl, cmd, true
	case syncWaveTimelineMsg:
		mdl, cmd := m.updateSyncWaveTimeline(msg)
		return mdl, cmd, true
	case syncWaveTickMsg:
		mdl, cmd := m.handleSyncWaveTick(msg)
		return mdl, cmd, true
	case syncWaveSpinnerTickMsg:
		mdl, cmd := m.handleSyncWaveSpinnerTick(msg)
		return mdl, cmd, true
	case captureBackendsLoadedMsg, captureStartedMsg, captureFailedMsg,
		captureStoppedMsg, captureLiveTickMsg, kubesharkLaunchedMsg,
		captureUpdateMsg:
		mdl, cmd := m.routeCaptureMsg(msg)
		return mdl, cmd, true
	case quotaLoadedMsg:
		mdl, cmd := m.updateQuotaLoaded(msg)
		return mdl, cmd, true
	case alertsLoadedMsg:
		mdl, cmd := m.updateAlertsLoaded(msg)
		return mdl, cmd, true
	case netpolLoadedMsg, netpolsForResourceLoadedMsg:
		mdl, cmd := m.routeNetpolMsg(msg)
		return mdl, cmd, true
	case orphansLoadedMsg:
		mdl, cmd := m.handleOrphansLoaded(msg)
		return mdl, cmd, true
	case describeLoadedMsg:
		mdl, cmd := m.updateDescribeLoaded(msg)
		return mdl, cmd, true
	case describeRefreshTickMsg:
		mdl, cmd := m.updateDescribeRefreshTick(msg)
		return mdl, cmd, true
	case helmValuesLoadedMsg:
		mdl, cmd := m.updateHelmValuesLoaded(msg)
		return mdl, cmd, true
	case diffLoadedMsg:
		mdl, cmd := m.updateDiffLoaded(msg)
		return mdl, cmd, true
	case explainLoadedMsg, explainRecursiveMsg, explainTreeLoadedMsg, explainTreeDescMsg:
		return m.updateExplainMsg(msg)
	}
	return m, nil, false
}

// updateEditorResultMsg handles editor, revision, export, and exec-related messages.
func (m Model) updateEditorResultMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case secretDataLoadedMsg:
		mdl, cmd := m.updateSecretDataLoaded(msg)
		return mdl, cmd, true
	case rightsizingLoadedMsg:
		m = m.updateRightsizingLoaded(msg)
		return m, nil, true
	case rightsizingStrategiesProbedMsg:
		mdl, cmd := m.updateRightsizingStrategiesProbed(msg)
		return mdl, cmd, true
	case secretSavedMsg:
		mdl, cmd := m.updateSecretSaved(msg)
		return mdl, cmd, true
	case configMapDataLoadedMsg:
		mdl, cmd := m.updateConfigMapDataLoaded(msg)
		return mdl, cmd, true
	case configMapSavedMsg:
		mdl, cmd := m.updateConfigMapSaved(msg)
		return mdl, cmd, true
	case labelDataLoadedMsg:
		mdl, cmd := m.updateLabelDataLoaded(msg)
		return mdl, cmd, true
	case labelSavedMsg:
		mdl, cmd := m.updateLabelSaved(msg)
		return mdl, cmd, true
	case autoSyncLoadedMsg:
		mdl, cmd := m.updateAutoSyncLoaded(msg)
		return mdl, cmd, true
	case autoSyncSavedMsg:
		mdl, cmd := m.updateAutoSyncSaved(msg)
		return mdl, cmd, true
	case exportDoneMsg:
		mdl, cmd := m.updateExportDone(msg)
		return mdl, cmd, true
	case revisionListMsg:
		mdl, cmd := m.updateRevisionList(msg)
		return mdl, cmd, true
	case rollbackDoneMsg:
		mdl, cmd := m.updateRollbackDone(msg)
		return mdl, cmd, true
	case helmRevisionListMsg:
		mdl, cmd := m.updateHelmRevisionList(msg)
		return mdl, cmd, true
	case helmHistoryListMsg:
		mdl, cmd := m.updateHelmHistoryList(msg)
		return mdl, cmd, true
	case helmRollbackDoneMsg:
		mdl, cmd := m.updateHelmRollbackDone(msg)
		return mdl, cmd, true
	case templateApplyMsg:
		mdl, cmd := m.updateTemplateApply(msg)
		return mdl, cmd, true
	case execPTYTickMsg:
		mdl, cmd := m.updateExecPTYTick(msg)
		return mdl, cmd, true
	case execPTYExitMsg:
		mdl := m.updateExecPTYExit(msg)
		return mdl, nil, true
	case execPodOSResolvedMsg:
		mdl, cmd := m.updateExecPodOSResolved(msg)
		return mdl, cmd, true
	case execPTYStartMsg:
		mdl, cmd := m.updateExecPTYStart(msg)
		return mdl, cmd, true
	case drainNodeResolvedMsg:
		mdl, cmd := m.updateDrainNodeResolved(msg)
		return mdl, cmd, true
	case logLineMsg:
		mdl, cmd := m.updateLogLine(msg)
		return mdl, cmd, true
	case previewLogLineMsg:
		mdl, cmd := m.updatePreviewLogLine(msg)
		return mdl, cmd, true
	case previewLogRestartMsg:
		mdl, cmd := m.updatePreviewLogRestart(msg)
		return mdl, cmd, true
	case previewLogHistoryMsg:
		mdl := m.updatePreviewLogHistory(msg)
		return mdl, nil, true
	case logStreamRestartMsg:
		mdl, cmd := m.updateLogStreamRestart(msg)
		return mdl, cmd, true
	case logHistoryMsg:
		mdl := m.updateLogHistory(msg)
		return mdl, nil, true
	case logSaveAllMsg:
		mdl, cmd := m.updateLogSaveAll(msg)
		return mdl, cmd, true
	}
	return m, nil, false
}

func (m Model) updateWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	m.clampAllCursors()
	// Re-render every width-dependent cached preview (dashboard, monitoring,
	// metrics bar, events footer) so they use the new width.
	m = m.recomposeThemedContent()
	// Resize the embedded PTY terminal if active.
	if m.mode == modeExec && m.execTerm != nil && m.execPTY != nil {
		cols := m.width
		rows := m.height - 6
		if cols < 20 {
			cols = 20
		}
		if rows < 5 {
			rows = 5
		}
		m.execMu.Lock()
		m.execTerm.Resize(cols, rows)
		m.execMu.Unlock()
		_ = pty.Setsize(m.execPTY, &pty.Winsize{
			Rows: uint16(rows),
			Cols: uint16(cols),
		})
	}
	return m, nil
}

func (m Model) updateTick(msg spinner.TickMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	if !m.spinnerNeeded() {
		m.spinnerTicking = false
		return m, nil
	}
	return m, cmd
}
