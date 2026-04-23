package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/creack/pty"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
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

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.updateWindowSize(msg)
	case tea.MouseMsg:
		return m.handleMouse(msg)
	case tea.KeyMsg:
		return m.handleKey(msg)
	case spinner.TickMsg:
		return m.updateTick(msg)
	case stderrCapturedMsg:
		m.setStatusMessage("stderr: "+msg.message, true)
		return m, tea.Batch(scheduleStatusClear(), m.waitForStderr())
	default:
		if dark, ok := ui.ParseColorModeMsg(msg); ok {
			ui.SetColorMode(dark)
			return m, nil
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
func (m Model) updateResourceMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case contextsLoadedMsg:
		mdl, cmd := m.updateContextsLoaded(msg)
		return mdl, cmd, true
	case resourceTypesMsg:
		mdl, cmd := m.updateResourceTypes(msg)
		return mdl, cmd, true
	case apiResourceDiscoveryMsg:
		mdl := m.updateAPIResourceDiscovery(msg)
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
	case watchTickMsg:
		mdl, cmd := m.updateWatchTick(msg)
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
	case podMetricsEnrichedMsg:
		mdl := m.updatePodMetricsEnriched(msg)
		return mdl, nil, true
	case nodeMetricsEnrichedMsg:
		mdl := m.updateNodeMetricsEnriched(msg)
		return mdl, nil, true
	case dashboardLoadedMsg:
		mdl := m.updateDashboardLoaded(msg)
		return mdl, nil, true
	case monitoringDashboardMsg:
		mdl := m.updateMonitoringDashboard(msg)
		return mdl, nil, true
	case logContainersLoadedMsg:
		mdl, cmd := m.updateLogContainersLoaded(msg)
		return mdl, cmd, true
	}

	return m.updateEasterEggMsg(msg)
}

// updateEasterEggMsg handles easter egg tick/clear messages.
func (m Model) updateEasterEggMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg.(type) {
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
	case quotaLoadedMsg:
		mdl, cmd := m.updateQuotaLoaded(msg)
		return mdl, cmd, true
	case alertsLoadedMsg:
		mdl, cmd := m.updateAlertsLoaded(msg)
		return mdl, cmd, true
	case netpolLoadedMsg:
		mdl, cmd := m.updateNetpolLoaded(msg)
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
	case explainLoadedMsg:
		mdl, cmd := m.updateExplainLoaded(msg)
		return mdl, cmd, true
	case explainRecursiveMsg:
		mdl, cmd := m.updateExplainRecursive(msg)
		return mdl, cmd, true
	}
	return m, nil, false
}

// updateEditorResultMsg handles editor, revision, export, and exec-related messages.
func (m Model) updateEditorResultMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case secretDataLoadedMsg:
		mdl, cmd := m.updateSecretDataLoaded(msg)
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
	case execPTYStartMsg:
		mdl, cmd := m.updateExecPTYStart(msg)
		return mdl, cmd, true
	case logLineMsg:
		mdl, cmd := m.updateLogLine(msg)
		return mdl, cmd, true
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
	// Resize the embedded PTY terminal if active.
	if m.mode == modeExec && m.execTerm != nil && m.execPTY != nil {
		cols := m.width - 4
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
	return m, cmd
}

func (m Model) updateContextsLoaded(msg contextsLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if isContextCanceled(msg.err) {
		return m, nil
	}
	if msg.err != nil {
		m.err = msg.err
		m.setErrorFromErr("Warning: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.err = nil
	m.middleItems = msg.items
	m.itemCache[m.navKey()] = m.middleItems
	m.leftItems = nil
	m.clearRight()
	m.clampCursor()

	// Restore saved port forwards in the background.
	var pfCmds []tea.Cmd
	if m.pendingPortForwards != nil && len(m.pendingPortForwards.PortForwards) > 0 {
		pfCmds = m.restorePortForwards()
		m.pendingPortForwards = nil
	}

	// Restore session: navigate to the saved context/namespace/resource type.
	if m.pendingSession != nil && !m.sessionRestored {
		mdl, cmd := m.restoreSession(msg.items)
		if len(pfCmds) > 0 {
			pfCmds = append(pfCmds, cmd)
			return mdl, tea.Batch(pfCmds...)
		}
		return mdl, cmd
	}

	cmds := make([]tea.Cmd, 0, 1+len(pfCmds))
	cmds = append(cmds, m.loadPreview())
	cmds = append(cmds, pfCmds...)
	return m, tea.Batch(cmds...)
}

func (m Model) updateResourceTypes(msg resourceTypesMsg) (tea.Model, tea.Cmd) {
	m.err = nil
	if m.nav.Level == model.LevelClusters {
		// Right-pane preview at the cluster list: always update so the user
		// sees *something* (seeds or real) while hovering a context.
		m.rightItems = msg.items
		m.loading = false
		return m, nil
	}
	// Middle pane at LevelResourceTypes: if discovery is still in flight
	// and only seeds are available, don't clobber the loader with seeds
	// — that would cause a one-tick flash of basic resource types every
	// watch interval. Real discovery results (seeded=false) or an
	// explicit seed fallback from updateAPIResourceDiscovery (which
	// writes middleItems directly) take precedence via their own paths.
	if msg.seeded && m.loading {
		return m, nil
	}
	m.loading = false
	m.middleItems = msg.items
	m.itemCache[m.navKey()] = m.middleItems
	m.clampCursor()
	return m, m.loadPreview()
}

func (m Model) updateAPIResourceDiscovery(msg apiResourceDiscoveryMsg) Model {
	// Clear the in-flight flag for this context regardless of outcome so
	// the user can retry (or hover again) without getting stuck on a
	// permanently-deduplicated call.
	delete(m.discoveringContexts, msg.context)
	if isContextCanceled(msg.err) {
		return m
	}
	if msg.err != nil {
		// API resource discovery failed (permissions, etc.) -- fall back to
		// seed resources so the user can still navigate.
		logger.Info("API resource discovery failed", "context", msg.context, "error", msg.err.Error())
		if m.nav.Context == msg.context && m.loading {
			m.loading = false
			m.middleItems = model.BuildSidebarItems(model.SeedResources())
			m.itemCache[m.navKey()] = m.middleItems
			m.restoreCursor()
			m.syncExpandedGroup()
		}
		return m
	}
	// Prepend LFK pseudo-resources (helm releases, port forwards) so they
	// resolve via FindResourceType* and appear in the sidebar uniformly
	// with real discovered resources.
	entries := append(model.PseudoResources(), msg.entries...)
	m.discoveredResources[msg.context] = entries
	merged := model.BuildSidebarItems(entries)
	// If the user is at LevelClusters and hovering this context, refresh
	// the right-pane preview so the discovered list replaces the seed
	// fallback that was emitted synchronously when loadPreviewClusters ran.
	if m.nav.Level == model.LevelClusters {
		if sel := m.selectedMiddleItem(); sel != nil && sel.Name == msg.context {
			m.rightItems = merged
		}
	}
	if m.nav.Context == msg.context {
		// Update the item cache for the resource types level.
		rtCacheKey := msg.context
		m.itemCache[rtCacheKey] = merged
		if m.nav.Level == model.LevelResourceTypes {
			// User is on resource types level: update the visible list.
			m.loading = false
			m.middleItems = merged
			m.restoreCursor()
			m.syncExpandedGroup()
		} else if m.nav.Level != model.LevelClusters {
			// User is deeper: update leftItems so back-navigation shows CRDs.
			m.leftItems = merged
			// Update cursor memory for the resource types level so
			// navigating back lands on the correct resource type.
			if m.nav.ResourceType.Resource != "" {
				rtRef := m.nav.ResourceType.ResourceRef()
				for i, item := range merged {
					if item.Extra == rtRef {
						m.cursorMemory[msg.context] = i
						break
					}
				}
			}
		}
	}
	return m
}

func (m Model) updateResourcesLoaded(msg resourcesLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.requestGen {
		return m, nil // stale response, discard
	}
	m.loading = false
	if isContextCanceled(msg.err) {
		return m, nil
	}
	if msg.err != nil {
		m.err = msg.err
		m.setErrorFromErr("Warning: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.err = nil
	if msg.forPreview {
		return m.updateResourcesLoadedPreview(msg)
	}
	return m.updateResourcesLoadedMain(msg)
}

func (m Model) updateResourcesLoadedPreview(msg resourcesLoadedMsg) (tea.Model, tea.Cmd) {
	m.previewLoading = false
	// Filter by selected namespaces when multi-select is active.
	if len(m.selectedNamespaces) > 1 {
		filtered := make([]model.Item, 0, len(msg.items))
		for _, item := range msg.items {
			if item.Namespace == "" || m.selectedNamespaces[item.Namespace] {
				filtered = append(filtered, item)
			}
		}
		msg.items = filtered
	}
	// Prime itemCache under the drill-in navKey so loadResources can serve
	// the list instantly and skip a redundant fetch when the user drills
	// in or re-hovers this rt later. Only do this when msg.rt carries a
	// real resource — synthetic previews (port-forwards, dashboards) have
	// a zero-valued rt and must not write an empty-resource key. The
	// fingerprint records the fetch-affecting state so the shortcut can
	// detect later invalidations (namespace switch, allNS toggle,
	// multi-select update) without relying on requestGen, which
	// navigateChild bumps before child handlers even run.
	if msg.rt.Resource != "" {
		drillInKey := m.nav.Context + "/" + msg.rt.Resource
		m.itemCache[drillInKey] = msg.items
		m.cacheFingerprints[drillInKey] = m.fetchFingerprint()
	}
	m.rightItems = msg.items
	// Filter events in children view to warnings-only when enabled.
	if m.warningEventsOnly && len(m.rightItems) > 0 && m.rightItems[0].Kind == "Event" {
		filtered := make([]model.Item, 0, len(m.rightItems))
		for _, item := range m.rightItems {
			if item.Status == "Warning" {
				filtered = append(filtered, item)
			}
		}
		m.rightItems = filtered
	}
	// Collapse duplicate events so noisy pods don't drown out the preview.
	// The preview pane is always a summary, so we follow the main list's
	// grouping toggle without offering a separate control — toggling `z` in
	// the Events view also affects the preview shown for other resources.
	if m.eventGrouping && len(m.rightItems) > 0 && m.rightItems[0].Kind == "Event" {
		m.rightItems = groupEvents(m.rightItems)
	}
	if len(m.rightItems) == 0 {
		logger.Info("No child resources found", "resourceType", m.nav.ResourceType.Kind, "resource", m.nav.ResourceName)
	}
	return m, nil
}

func (m Model) updateResourcesLoadedMain(msg resourcesLoadedMsg) (tea.Model, tea.Cmd) {
	// Filter by selected namespaces when multi-select is active.
	if len(m.selectedNamespaces) > 1 {
		filtered := make([]model.Item, 0, len(msg.items))
		for _, item := range msg.items {
			if item.Namespace == "" || m.selectedNamespaces[item.Namespace] {
				filtered = append(filtered, item)
			}
		}
		msg.items = filtered
	}
	if len(msg.items) == 0 {
		logger.Info("No resources found", "resourceType", m.nav.ResourceType.Kind, "namespace", m.namespace)
	}
	prevName, prevNs, prevExtra, prevKind := m.cursorItemKey()

	kind := m.nav.ResourceType.Kind
	if (kind == "Pod" || kind == "Node") && len(m.middleItems) > 0 {
		m.carryOverMetricsColumns(msg.items)
	}
	m.middleItems = msg.items
	mainCacheKey := m.navKey()
	m.itemCache[mainCacheKey] = m.middleItems
	// Record the cache-freshness fingerprint so a subsequent load for the
	// same resource (drill-in from the sidebar, preview on navigate-out-
	// then-hover, or a hover-cycle between sibling rts) can serve from
	// cache instead of refetching. Only record for actual resource lists;
	// __port_forwards__ is synthetic (sourced from the in-process manager)
	// and doesn't go through GetResources.
	if m.nav.ResourceType.Resource != "" && m.nav.ResourceType.Kind != "__port_forwards__" {
		m.cacheFingerprints[mainCacheKey] = m.fetchFingerprint()
	}
	// Always sort: the k8s layer uses a non-stable single-key sort that
	// shuffles ties between refreshes (e.g. Helm releases with the same
	// name in different namespaces). Running sortMiddleItems guarantees
	// the app-level tiebreaker chain is applied on every load — even the
	// default Name/ascending case — so watch-mode output is deterministic.
	m.sortMiddleItems()
	m.applyWarningEventsFilter()
	m.applyEventGrouping()
	m.reapplyFilterPreset()
	if m.pendingTarget != "" {
		for i, item := range m.middleItems {
			if item.Name == m.pendingTarget {
				m.setCursor(i)
				break
			}
		}
		m.pendingTarget = ""
	} else {
		m.restoreCursorToItem(prevName, prevNs, prevExtra, prevKind)
	}
	// If this load originated from a watch-mode refresh, propagate the
	// suppress flag to the downstream preview/metrics cmds so they too
	// stay off the title-bar indicator. Capture the prior flag so the
	// returned model resets it cleanly for subsequent user Updates.
	savedSuppress := m.suppressBgtasks
	if msg.silent {
		m.suppressBgtasks = true
	}
	var cmds []tea.Cmd
	// Mark the preview pane as loading so the right column shows the
	// spinner during the gap between the main list arriving (which cleared
	// the global loading flag) and the preview load completing. Without
	// this the right pane briefly renders "No resources found".
	previewCmd := m.loadPreview()
	if previewCmd != nil {
		m.previewLoading = true
		cmds = append(cmds, previewCmd)
	}
	switch kind {
	case "Pod":
		cmds = append(cmds, m.loadPodMetricsForList())
	case "Node":
		cmds = append(cmds, m.loadNodeMetricsForList())
	}
	m.suppressBgtasks = savedSuppress
	return m, tea.Batch(cmds...)
}

func (m *Model) applyWarningEventsFilter() {
	if m.warningEventsOnly && m.nav.ResourceType.Kind == "Event" {
		var filtered []model.Item
		for _, item := range m.middleItems {
			if item.Status == "Warning" {
				filtered = append(filtered, item)
			}
		}
		m.middleItems = filtered
	}
}

// applyEventGrouping collapses duplicate Events sharing Type/Reason/Message/Object
// into a single row with a summed Count. Runs only when viewing the Event
// resource list with grouping enabled; other resource kinds pass through untouched.
func (m *Model) applyEventGrouping() {
	if !m.eventGrouping || m.nav.ResourceType.Kind != "Event" {
		return
	}
	m.middleItems = groupEvents(m.middleItems)
}

// rebuildEventsFromCache re-derives the visible Event list from the raw cache
// after an Events-view toggle (warnings-only, grouping). It re-applies the
// full pipeline — warning filter, grouping, and the active filter preset —
// so toggling any one of them never silently drops the others. A cache miss
// leaves m.middleItems untouched; the next resource load will rebuild it.
func (m *Model) rebuildEventsFromCache() {
	cached, ok := m.itemCache[m.navKey()]
	if !ok {
		return
	}
	m.middleItems = append([]model.Item(nil), cached...)
	m.applyWarningEventsFilter()
	m.applyEventGrouping()
	m.reapplyFilterPreset()
	m.clampCursor()
}

func (m *Model) reapplyFilterPreset() {
	if m.activeFilterPreset != nil {
		m.unfilteredMiddleItems = append([]model.Item(nil), m.middleItems...)
		var filtered []model.Item
		for _, item := range m.middleItems {
			if m.activeFilterPreset.MatchFn(item) {
				filtered = append(filtered, item)
			}
		}
		m.middleItems = filtered
	}
}

func (m Model) updateOwnedLoaded(msg ownedLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.requestGen {
		return m, nil // stale response, discard
	}
	m.loading = false
	if isContextCanceled(msg.err) {
		return m, nil
	}
	if msg.err != nil {
		m.err = msg.err
		m.setErrorFromErr("Warning: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.err = nil
	// Filter by selected namespaces when multi-select is active.
	if len(m.selectedNamespaces) > 1 {
		filtered := make([]model.Item, 0, len(msg.items))
		for _, item := range msg.items {
			if m.selectedNamespaces[item.Namespace] {
				filtered = append(filtered, item)
			}
		}
		msg.items = filtered
	}
	if msg.forPreview {
		m.previewLoading = false
		m.rightItems = msg.items
		return m, nil
	}
	prevName, prevNs, prevExtra, prevKind := m.cursorItemKey()
	m.middleItems = msg.items
	m.itemCache[m.navKey()] = m.middleItems
	// Sort with the app-level tiebreaker on every load (see
	// updateResourcesLoadedMain for rationale): the k8s layer returns
	// items in a non-deterministic order for equal keys, so the
	// tiebreaker chain must run here too or owned-resource refreshes
	// will flicker.
	m.sortMiddleItems()
	// Re-apply active filter preset on owned refresh (same as resourcesLoadedMsg).
	if m.activeFilterPreset != nil {
		m.unfilteredMiddleItems = append([]model.Item(nil), m.middleItems...)
		var filtered []model.Item
		for _, item := range m.middleItems {
			if m.activeFilterPreset.MatchFn(item) {
				filtered = append(filtered, item)
			}
		}
		m.middleItems = filtered
		m.itemCache[m.navKey()] = m.middleItems
	}
	m.restoreCursorToItem(prevName, prevNs, prevExtra, prevKind)
	// Propagate the silent flag to the downstream preview cmd.
	savedSuppress := m.suppressBgtasks
	if msg.silent {
		m.suppressBgtasks = true
	}
	// Mark the preview pane as loading (see updateResourcesLoadedMain).
	previewCmd := m.loadPreview()
	if previewCmd != nil {
		m.previewLoading = true
	}
	m.suppressBgtasks = savedSuppress
	return m, previewCmd
}

func (m Model) updateResourceTreeLoaded(msg resourceTreeLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.requestGen {
		return m, nil
	}
	if isContextCanceled(msg.err) {
		return m, nil
	}
	if msg.err != nil {
		m.setErrorFromErr("Resource tree: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.resourceTree = msg.tree
	return m, nil
}

func (m Model) updateContainersLoaded(msg containersLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.requestGen {
		return m, nil // stale response, discard
	}
	m.loading = false
	if isContextCanceled(msg.err) {
		return m, nil
	}
	if msg.err != nil {
		m.err = msg.err
		m.setErrorFromErr("Warning: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.err = nil
	if msg.forPreview {
		m.previewLoading = false
		m.rightItems = msg.items
		return m, nil
	}
	m.middleItems = msg.items
	m.itemCache[m.navKey()] = m.middleItems
	// Sort with the app-level tiebreaker on every container-list load
	// (see updateResourcesLoadedMain for rationale): container rows use
	// the parent pod's namespace and only differ by Name/Kind, so the
	// tiebreaker still provides a stable order across refreshes.
	m.sortMiddleItems()
	m.clampCursor()
	// Propagate the silent flag to the downstream preview cmd.
	savedSuppress := m.suppressBgtasks
	if msg.silent {
		m.suppressBgtasks = true
	}
	// Mark the preview pane as loading (see updateResourcesLoadedMain).
	previewCmd := m.loadPreview()
	if previewCmd != nil {
		m.previewLoading = true
	}
	m.suppressBgtasks = savedSuppress
	return m, previewCmd
}

func (m Model) updateNamespacesLoaded(msg namespacesLoadedMsg) (tea.Model, tea.Cmd) {
	// Only clear the global loading flag for overlay-triggered loads.
	// Background cache refreshes (session restore, context open) must not
	// touch it — it belongs to the middle-column/resource-types load and
	// clearing it asynchronously while API discovery is still in flight
	// produces a "No items" flash between the loader and the populated
	// list.
	if !msg.silent {
		m.loading = false
	}
	if isContextCanceled(msg.err) {
		return m, nil
	}
	if msg.err != nil {
		m.err = msg.err
		m.overlay = overlayNone
		m.setErrorFromErr("Warning: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.err = nil
	allNsItem := model.Item{Name: "All Namespaces", Status: "all"}
	m.overlayItems = append([]model.Item{allNsItem}, msg.items...)
	// Cache namespace names for command bar autocompletion, keyed by the
	// context the fetch was issued for. Keying avoids stale results when
	// tabs / `:ctx` change nav.Context between the request and reply.
	// Stamp fetchedAt so the next command bar open can decide whether
	// the entry is still fresh or should trigger a background refresh.
	if m.cachedNamespaces == nil {
		m.cachedNamespaces = make(map[string]namespaceCacheEntry)
	}
	names := make([]string, 0, len(msg.items))
	for _, item := range msg.items {
		names = append(names, item.Name)
	}
	m.cachedNamespaces[msg.context] = namespaceCacheEntry{
		names:     names,
		fetchedAt: time.Now(),
	}
	if m.allNamespaces {
		m.overlayCursor = 0
	} else {
		for i, item := range m.overlayItems {
			if item.Name == m.namespace {
				m.overlayCursor = i
				break
			}
		}
	}
	return m, nil
}

func (m Model) updateYamlLoaded(msg yamlLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if isContextCanceled(msg.err) {
		return m, nil
	}
	if msg.err != nil {
		m.err = msg.err
		m.setErrorFromErr("Warning: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.err = nil
	// Content and sections are pre-processed in the loading goroutine so
	// the main event loop stays responsive on very large CRD manifests.
	m.yamlContent = msg.content
	m.yamlSections = msg.sections
	return m, nil
}

func (m Model) updatePreviewYAMLLoaded(msg previewYAMLLoadedMsg) Model {
	if msg.gen != m.requestGen {
		return m // stale response, discard
	}
	if msg.err != nil {
		m.previewYAML = ""
		return m
	}
	// Pre-indented in the loading goroutine — no heavy work on main thread.
	m.previewYAML = msg.content
	return m
}

func (m Model) updateActionResult(msg actionResultMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	m.bulkMode = false
	if msg.err != nil {
		m.setErrorFromErr("Error: ", msg.err)
	} else {
		if msg.message != "" {
			logger.Info("Action completed", "message", msg.message)
			m.setStatusMessage(msg.message, false)
		}
		// Only invalidate when the action succeeded; a failed `create
		// ns` or template apply did not actually mutate the cluster.
		if msg.invalidateNamespaceCache {
			m.invalidateNamespaceCache()
		}
	}
	return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())
}

func (m Model) updateContainerPortsLoaded(msg containerPortsLoadedMsg) Model {
	m.loading = false
	if msg.err != nil {
		// No ports discovered, still open the overlay for manual entry.
		m.addLogEntry("WRN", fmt.Sprintf("Could not discover ports: %v", msg.err))
		m.pfAvailablePorts = nil
		m.pfPortCursor = -1
	} else {
		m.pfAvailablePorts = nil
		for _, p := range msg.ports {
			m.pfAvailablePorts = append(m.pfAvailablePorts, ui.PortInfo{
				Port:     strconv.Itoa(int(p.ContainerPort)),
				Name:     p.Name,
				Protocol: p.Protocol,
			})
		}
		if len(m.pfAvailablePorts) > 0 {
			m.pfPortCursor = 0
		} else {
			m.pfPortCursor = -1
		}
	}
	m.portForwardInput.Clear()
	m.overlay = overlayPortForward
	return m
}

func (m Model) updatePortForwardStarted(msg portForwardStartedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Port forward failed: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.pfLastCreatedID = msg.id
	if msg.localPort == "0" {
		m.setStatusMessage(fmt.Sprintf("Port forward started (resolving local port...) -> %s", msg.remotePort), false)
		m.addLogEntry("INF", fmt.Sprintf("Port forward %d started: localhost:? -> %s (random port)", msg.id, msg.remotePort))
	} else {
		m.setStatusMessage(fmt.Sprintf("Port forward started on localhost:%s -> %s", msg.localPort, msg.remotePort), false)
		m.addLogEntry("INF", fmt.Sprintf("Port forward %d started: localhost:%s -> %s", msg.id, msg.localPort, msg.remotePort))
	}
	// Navigate to the Port Forwards list.
	m.navigateToPortForwards()
	m.saveCurrentPortForwards()
	cmds := []tea.Cmd{m.waitForPortForwardUpdate()}
	return m, tea.Batch(cmds...)
}

func (m Model) updatePortForwardStopped(msg portForwardStoppedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Error stopping port forward: ", msg.err)
	} else {
		m.setStatusMessage("Port forward stopped", false)
	}
	m.saveCurrentPortForwards()
	// Refresh the port forwards list if viewing it.
	cmds := []tea.Cmd{scheduleStatusClear()}
	if m.nav.Level == model.LevelResources && m.nav.ResourceType.Kind == "__port_forwards__" {
		cmds = append(cmds, m.refreshCurrentLevel())
	}
	return m, tea.Batch(cmds...)
}

func (m Model) updatePortForwardUpdate(msg portForwardUpdateMsg) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{m.waitForPortForwardUpdate()}
	if msg.err != nil {
		m.addLogEntry("ERR", msg.err.Error())
	}
	// Log newly failed port forwards.
	for _, e := range m.portForwardMgr.Entries() {
		if e.Status == k8s.PortForwardFailed && e.Error != "" {
			if _, seen := m.pfLoggedErrors[e.ID]; !seen {
				m.addLogEntry("ERR", fmt.Sprintf("Port forward %s/%s failed: %s", e.ResourceKind, e.ResourceName, e.Error))
				if m.pfLoggedErrors == nil {
					m.pfLoggedErrors = make(map[int]struct{})
				}
				m.pfLoggedErrors[e.ID] = struct{}{}
			}
		}
	}
	// Show resolved port notification for recently created port forward.
	if m.pfLastCreatedID > 0 {
		for _, e := range m.portForwardMgr.Entries() {
			if e.ID == m.pfLastCreatedID && e.LocalPort != "0" && e.LocalPort != "" {
				m.setStatusMessage(fmt.Sprintf("Port forward active on localhost:%s -> %s", e.LocalPort, e.RemotePort), false)
				m.addLogEntry("INF", fmt.Sprintf("Port forward %d resolved: localhost:%s -> %s", e.ID, e.LocalPort, e.RemotePort))
				m.pfLastCreatedID = 0
				m.saveCurrentPortForwards()
				cmds = append(cmds, scheduleStatusClear())
				break
			}
		}
	}
	if m.nav.Level == model.LevelResources && m.nav.ResourceType.Kind == "__port_forwards__" {
		m.middleItems = m.portForwardItems()
		m.clampCursor()
	}
	return m, tea.Batch(cmds...)
}

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

func (m Model) updatePodSelect(msg podSelectMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Error: ", msg.err)
		m.pendingAction = ""
		return m, scheduleStatusClear()
	}
	// Filter to only pods.
	var pods []model.Item
	for _, item := range msg.items {
		if item.Kind == "Pod" {
			pods = append(pods, item)
		}
	}
	if len(pods) == 0 {
		m.setStatusMessage("No pods found", true)
		m.pendingAction = ""
		return m, scheduleStatusClear()
	}
	if len(pods) == 1 {
		// Only one pod, proceed to container selection.
		m.actionCtx.name = pods[0].Name
		m.actionCtx.kind = "Pod"
		if pods[0].Namespace != "" {
			m.actionCtx.namespace = pods[0].Namespace
		}
		return m, m.loadContainersForAction()
	}
	// Multiple pods, show picker.
	m.overlayItems = pods
	m.overlay = overlayPodSelect
	m.overlayCursor = 0
	m.logPodFilterText = ""
	m.logPodFilterActive = false
	ui.ResetOverlayPodScroll()
	return m, nil
}

func (m Model) updatePodLogSelect(msg podLogSelectMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setErrorFromErr("Error: ", msg.err)
		m.pendingAction = ""
		// If in log mode, restart the previous log stream on error.
		if m.mode == modeLogs && m.logSavedPodName != "" {
			m.actionCtx.name = m.logSavedPodName
			m.actionCtx.kind = "Pod"
			m.logSavedPodName = ""
			return m, m.startLogStream()
		}
		return m, scheduleStatusClear()
	}
	var pods []model.Item
	for _, item := range msg.items {
		if item.Kind == "Pod" {
			pods = append(pods, item)
		}
	}
	if len(pods) == 0 {
		m.setStatusMessage("No pods found", true)
		m.pendingAction = ""
		// If in log mode, restart the previous log stream when no pods found.
		if m.mode == modeLogs && m.logSavedPodName != "" {
			m.actionCtx.name = m.logSavedPodName
			m.actionCtx.kind = "Pod"
			m.logSavedPodName = ""
			return m, m.startLogStream()
		}
		return m, scheduleStatusClear()
	}

	// If we're in log mode (P was pressed from the log viewer), handle inline.
	if m.mode == modeLogs {
		if len(pods) == 1 {
			// Only one pod; switch directly to its logs.
			m.actionCtx.name = pods[0].Name
			m.actionCtx.kind = "Pod"
			if pods[0].Namespace != "" {
				m.actionCtx.namespace = pods[0].Namespace
			}
			m.pendingAction = ""
			m.logSavedPodName = ""
			m.logLines = nil
			m.logScroll = 0
			m.logFollow = true
			m.logTailLines = ui.ConfigLogTailLines
			m.logHasMoreHistory = true
			m.logLoadingHistory = false
			m.logCursor = 0
			m.logVisualMode = false
			m.logTitle = fmt.Sprintf("Logs: %s/%s", m.actionNamespace(), m.actionCtx.name)
			return m, m.startLogStream()
		}
		// Multiple pods; show inline pod selector overlay with "All Pods" at top.
		allItem := model.Item{Name: "All Pods", Status: "all"}
		m.overlayItems = append([]model.Item{allItem}, pods...)
		m.overlay = overlayLogPodSelect
		m.overlayCursor = 0
		m.logPodFilterText = ""
		m.logPodFilterActive = false
		ui.ResetOverlayPodScroll()
		return m, nil
	}

	if len(pods) == 1 {
		// Only one pod, start log streaming directly.
		m.actionCtx.name = pods[0].Name
		m.actionCtx.kind = "Pod"
		if pods[0].Namespace != "" {
			m.actionCtx.namespace = pods[0].Namespace
		}
		m.pendingAction = ""
		return m.executeAction("Logs")
	}
	// Multiple pods, show picker.
	m.overlayItems = pods
	m.overlay = overlayPodSelect
	m.overlayCursor = 0
	m.logPodFilterText = ""
	m.logPodFilterActive = false
	ui.ResetOverlayPodScroll()
	return m, nil
}

func (m Model) updateContainerSelect(msg containerSelectMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Error: ", msg.err)
		m.pendingAction = ""
		return m, scheduleStatusClear()
	}
	if len(msg.items) == 1 {
		// Only one container; proceed directly.
		m.actionCtx.containerName = msg.items[0].Name
		action := m.pendingAction
		m.pendingAction = ""
		return m.executeAction(action)
	}
	if len(msg.items) == 0 {
		m.setStatusMessage("No containers found", true)
		m.pendingAction = ""
		return m, scheduleStatusClear()
	}
	// Multiple containers; show selection overlay.
	m.overlayItems = msg.items
	m.overlay = overlayContainerSelect
	m.overlayCursor = 0
	return m, nil
}

func (m Model) updateYamlClipboard(msg yamlClipboardMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Error: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.setStatusMessage("YAML copied to clipboard", false)
	return m, tea.Batch(copyToSystemClipboard(msg.content), scheduleStatusClear())
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

func (m Model) updateRbacCheck(msg rbacCheckMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setStatusMessage(fmt.Sprintf("RBAC check failed: %v", msg.err), true)
		return m, scheduleStatusClear()
	}
	m.rbacResults = msg.results
	m.rbacKind = msg.kind
	m.overlay = overlayRBAC
	return m, nil
}

func (m Model) updateCanILoaded(msg canILoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setStatusMessage(fmt.Sprintf("RBAC rules check failed: %v", msg.err), true)
		return m, scheduleStatusClear()
	}
	m.processCanIRules(msg.rules)
	m.canINamespaces = msg.namespaces
	m.overlay = overlayCanI
	m.canIGroupCursor = 0
	m.canIGroupScroll = 0
	return m, nil
}

func (m Model) updateCanISAList(msg canISAListMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setStatusMessage(fmt.Sprintf("Failed to list subjects: %v", msg.err), true)
		return m, scheduleStatusClear()
	}
	m.canIServiceAccounts = msg.accounts
	// Build overlay items: "Current User", then Users, Groups, ServiceAccounts.
	items := make([]model.Item, 0, len(msg.subjects)+len(msg.accounts)+1)
	items = append(items, model.Item{Name: "Current User", Extra: ""})

	// Add Users from RBAC bindings.
	for _, subj := range msg.subjects {
		if subj.Kind == "User" {
			items = append(items, model.Item{
				Name:  "[User] " + subj.Name,
				Kind:  "User",
				Extra: subj.Name,
			})
		}
	}
	// Add Groups from RBAC bindings.
	for _, subj := range msg.subjects {
		if subj.Kind == "Group" {
			items = append(items, model.Item{
				Name:  "[Group] " + subj.Name,
				Kind:  "Group",
				Extra: "group:" + subj.Name,
			})
		}
	}
	// Add ServiceAccounts.
	for _, sa := range msg.accounts {
		var name, ns string
		if parts := strings.SplitN(sa, "/", 2); len(parts) == 2 {
			ns = parts[0]
			name = parts[1]
		} else {
			ns = m.namespace
			name = sa
		}
		items = append(items, model.Item{
			Name:  "[SA] " + ns + "/" + name,
			Kind:  "ServiceAccount",
			Extra: fmt.Sprintf("system:serviceaccount:%s:%s", ns, name),
		})
	}
	m.overlayItems = items
	m.overlayCursor = 0
	m.overlayFilter.Clear()
	m.canISubjectFilterMode = false
	ui.ResetOverlayCanISubjectScroll()
	m.overlay = overlayCanISubject
	return m, nil
}

func (m Model) updatePodStartup(msg podStartupMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setStatusMessage(fmt.Sprintf("Startup analysis failed: %v", msg.err), true)
		return m, scheduleStatusClear()
	}
	m.podStartupData = msg.info
	m.overlay = overlayPodStartup
	return m, nil
}

func (m Model) updateQuotaLoaded(msg quotaLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setStatusMessage(fmt.Sprintf("Quota load failed: %v", msg.err), true)
		return m, scheduleStatusClear()
	}
	if len(msg.quotas) == 0 {
		m.setStatusMessage("No resource quotas found in namespace", false)
		return m, scheduleStatusClear()
	}
	m.quotaData = msg.quotas
	m.overlay = overlayQuotaDashboard
	return m, nil
}

func (m Model) updateAlertsLoaded(msg alertsLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setStatusMessage(fmt.Sprintf("Alerts: %v", msg.err), true)
		return m, scheduleStatusClear()
	}
	m.alertsData = msg.alerts
	m.alertsScroll = 0
	m.overlay = overlayAlerts
	return m, nil
}

func (m Model) updateNetpolLoaded(msg netpolLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setStatusMessage(fmt.Sprintf("Failed to load network policy: %v", msg.err), true)
		return m, scheduleStatusClear()
	}
	m.netpolData = msg.info
	m.netpolScroll = 0
	m.overlay = overlayNetworkPolicy
	return m, nil
}

func (m Model) updateDescribeLoaded(msg describeLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setErrorFromErr("Error: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.mode = modeDescribe
	m.describeContent = msg.content
	// Preserve scroll/cursor on auto-refresh, reset on first load.
	if !m.describeAutoRefresh {
		m.describeScroll = 0
		m.describeCursor = 0
		m.describeCursorCol = 0
	}
	m.describeTitle = msg.title
	if m.describeAutoRefresh {
		return m, scheduleDescribeRefresh()
	}
	return m, nil
}

func (m Model) updateDescribeRefreshTick(msg describeRefreshTickMsg) (tea.Model, tea.Cmd) {
	if m.mode != modeDescribe || !m.describeAutoRefresh || m.describeRefreshFunc == nil {
		return m, nil
	}
	return m, m.describeRefreshFunc()
}

func (m Model) updateHelmValuesLoaded(msg helmValuesLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setErrorFromErr("Error loading Helm values: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.mode = modeDescribe
	m.describeContent = msg.content
	m.describeScroll = 0
	m.describeCursor = 0
	m.describeCursorCol = 0
	m.describeTitle = msg.title
	return m, nil
}

func (m Model) updateDiffLoaded(msg diffLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setErrorFromErr("Diff failed: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.mode = modeDiff
	m.diffLeft = msg.left
	m.diffRight = msg.right
	m.diffLeftName = msg.leftName
	m.diffRightName = msg.rightName
	m.diffScroll = 0
	m.diffUnified = false
	return m, nil
}

func (m Model) updateExplainLoaded(msg explainLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setErrorFromErr("Explain failed: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.mode = modeExplain
	m.explainFields = msg.fields
	m.explainDesc = msg.description
	m.explainPath = msg.path
	m.explainTitle = msg.title
	m.explainCursor = 0
	m.explainScroll = 0
	m.explainSearchActive = false
	return m, nil
}

func (m Model) updateExplainRecursive(msg explainRecursiveMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setErrorFromErr("Recursive search failed: ", msg.err)
		return m, scheduleStatusClear()
	}
	if len(msg.matches) == 0 {
		m.setStatusMessage("No fields found", true)
		return m, scheduleStatusClear()
	}
	m.explainRecursiveResults = msg.matches
	m.explainRecursiveCursor = 0
	m.explainRecursiveScroll = 0
	m.explainRecursiveFilter.Clear()
	m.explainRecursiveFilterActive = false
	m.overlay = overlayExplainSearch
	return m, nil
}

func (m Model) updateLogContainersLoaded(msg logContainersLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Failed to load containers: ", msg.err)
		m.overlay = overlayNone
		return m, scheduleStatusClear()
	}
	if len(msg.containers) <= 1 {
		// Only one container (or none), no need for a selector.
		m.overlay = overlayNone
		name := "this pod"
		if len(msg.containers) == 1 {
			name = msg.containers[0]
		}
		m.setStatusMessage(fmt.Sprintf("Only one container: %s", name), false)
		return m, scheduleStatusClear()
	}
	m.logContainers = msg.containers
	// Build overlay items with "All Containers" virtual item at the top.
	items := []model.Item{{Name: "All Containers", Status: "all"}}
	for _, c := range msg.containers {
		items = append(items, model.Item{Name: c})
	}
	m.overlayItems = items
	return m, nil
}

func (m Model) updateMetricsLoaded(msg metricsLoadedMsg) Model {
	if msg.gen != m.requestGen {
		return m // stale response
	}
	if msg.cpuUsed == 0 && msg.memUsed == 0 {
		m.metricsContent = ""
		return m
	}
	// Calculate available width for the metrics bar.
	usable := m.width - 6
	rightW := max(10, usable-max(10, usable*12/100)-max(10, usable*51/100))
	innerW := rightW - 4 // column padding + border
	if innerW < 20 {
		innerW = 20
	}
	m.metricsContent = ui.RenderResourceUsage(
		msg.cpuUsed, msg.cpuReq, msg.cpuLim,
		msg.memUsed, msg.memReq, msg.memLim,
		innerW,
	)
	return m
}

func (m Model) updatePreviewEventsLoaded(msg previewEventsLoadedMsg) Model {
	if msg.gen != m.requestGen {
		return m // stale response
	}
	if len(msg.events) == 0 {
		m.previewEventsContent = ""
		return m
	}
	// Calculate available width for the events section.
	usable := m.width - 6
	rightW := max(10, usable-max(10, usable*12/100)-max(10, usable*51/100))
	innerW := rightW - 4
	if innerW < 20 {
		innerW = 20
	}
	entries := make([]ui.EventTimelineEntry, len(msg.events))
	for i, e := range msg.events {
		entries[i] = ui.EventTimelineEntry{
			Timestamp:    e.Timestamp,
			Type:         e.Type,
			Reason:       e.Reason,
			Message:      e.Message,
			Source:       e.Source,
			Count:        e.Count,
			InvolvedName: e.InvolvedName,
			InvolvedKind: e.InvolvedKind,
		}
	}
	m.previewEventsContent = ui.RenderPreviewEvents(entries, innerW)
	return m
}

func (m Model) updatePodMetricsEnriched(msg podMetricsEnrichedMsg) Model {
	if msg.gen != m.requestGen {
		return m // stale response
	}
	if len(msg.metrics) == 0 {
		return m
	}
	// Enrich middle items with CPU/Memory usage + percentage columns.
	for i := range m.middleItems {
		item := &m.middleItems[i]
		key := item.Name
		if item.Namespace != "" {
			key = item.Namespace + "/" + item.Name
		}
		pm, ok := msg.metrics[key]
		if !ok {
			continue
		}

		// Look up existing request/limit values from item columns.
		var cpuReqStr, cpuLimStr, memReqStr, memLimStr string
		for _, kv := range item.Columns {
			switch kv.Key {
			case "CPU Req":
				cpuReqStr = kv.Value
			case "CPU Lim":
				cpuLimStr = kv.Value
			case "Mem Req":
				memReqStr = kv.Value
			case "Mem Lim":
				memLimStr = kv.Value
			}
		}

		cpuUse := ui.FormatCPU(pm.CPU)
		memUse := ui.FormatMemory(pm.Memory)

		// Detect significant usage trends (arrows before value).
		if m.prevPodMetrics != nil {
			if prev, ok := m.prevPodMetrics[key]; ok {
				cpuDiff := pm.CPU - prev.CPU
				memDiff := pm.Memory - prev.Memory
				// CPU: significant if >10% change AND >20m absolute change.
				if prev.CPU > 0 {
					pctChange := float64(cpuDiff) / float64(prev.CPU)
					if pctChange > 0.10 && cpuDiff > 20 {
						cpuUse = "↑ " + cpuUse
					} else if pctChange < -0.10 && cpuDiff < -20 {
						cpuUse = "↓ " + cpuUse
					}
				}
				// Memory: significant if >10% change AND >20Mi absolute change.
				if prev.Memory > 0 {
					pctChange := float64(memDiff) / float64(prev.Memory)
					if pctChange > 0.10 && memDiff > 20*1024*1024 {
						memUse = "↑ " + memUse
					} else if pctChange < -0.10 && memDiff < -20*1024*1024 {
						memUse = "↓ " + memUse
					}
				}
			}
		}

		cpuReqPct := ui.ComputePctStr(pm.CPU, cpuReqStr, true)
		cpuLimPct := ui.ComputePctStr(pm.CPU, cpuLimStr, true)
		memReqPct := ui.ComputePctStr(pm.Memory, memReqStr, false)
		memLimPct := ui.ComputePctStr(pm.Memory, memLimStr, false)

		// Rebuild columns: replace old CPU/Mem columns with new compact format.
		removeCols := map[string]bool{
			"CPU Use": true, "CPU Req": true, "CPU Lim": true,
			"Mem Use": true, "Mem Req": true, "Mem Lim": true,
			"CPU/R": true, "CPU/L": true, "MEM/R": true, "MEM/L": true,
		}
		var newCols []model.KeyValue
		newCols = append(newCols,
			model.KeyValue{Key: "CPU", Value: cpuUse},
			model.KeyValue{Key: "CPU/R", Value: cpuReqPct},
			model.KeyValue{Key: "CPU/L", Value: cpuLimPct},
			model.KeyValue{Key: "MEM", Value: memUse},
			model.KeyValue{Key: "MEM/R", Value: memReqPct},
			model.KeyValue{Key: "MEM/L", Value: memLimPct},
		)
		for _, kv := range item.Columns {
			if !removeCols[kv.Key] {
				newCols = append(newCols, kv)
			}
		}
		item.Columns = newCols
	}
	// Only update the baseline every 60s so trend arrows persist longer.
	if m.prevPodMetrics == nil || time.Since(m.prevPodMetricsTime) > 60*time.Second {
		m.prevPodMetrics = msg.metrics
		m.prevPodMetricsTime = time.Now()
	}
	// Update cache.
	m.itemCache[m.navKey()] = m.middleItems
	return m
}

func (m Model) updateNodeMetricsEnriched(msg nodeMetricsEnrichedMsg) Model {
	if msg.gen != m.requestGen {
		return m
	}
	if len(msg.metrics) == 0 {
		return m
	}
	for i := range m.middleItems {
		item := &m.middleItems[i]
		nm, ok := msg.metrics[item.Name]
		if !ok {
			continue
		}

		// Look up allocatable values from item columns.
		var cpuAllocStr, memAllocStr string
		for _, kv := range item.Columns {
			switch kv.Key {
			case "CPU Alloc":
				cpuAllocStr = kv.Value
			case "Mem Alloc":
				memAllocStr = kv.Value
			}
		}

		cpuUse := ui.FormatCPU(nm.CPU)
		memUse := ui.FormatMemory(nm.Memory)

		// Detect significant usage trends (arrows before value).
		if m.prevNodeMetrics != nil {
			if prev, ok := m.prevNodeMetrics[item.Name]; ok {
				cpuDiff := nm.CPU - prev.CPU
				memDiff := nm.Memory - prev.Memory
				// CPU: significant if >10% change AND >20m absolute change.
				if prev.CPU > 0 {
					pctChange := float64(cpuDiff) / float64(prev.CPU)
					if pctChange > 0.10 && cpuDiff > 20 {
						cpuUse = "↑ " + cpuUse
					} else if pctChange < -0.10 && cpuDiff < -20 {
						cpuUse = "↓ " + cpuUse
					}
				}
				// Memory: significant if >10% change AND >20Mi absolute change.
				if prev.Memory > 0 {
					pctChange := float64(memDiff) / float64(prev.Memory)
					if pctChange > 0.10 && memDiff > 20*1024*1024 {
						memUse = "↑ " + memUse
					} else if pctChange < -0.10 && memDiff < -20*1024*1024 {
						memUse = "↓ " + memUse
					}
				}
			}
		}

		cpuPct := ui.ComputePctStr(nm.CPU, cpuAllocStr, true)
		memPct := ui.ComputePctStr(nm.Memory, memAllocStr, false)

		removeCols := map[string]bool{
			"CPU": true, "CPU%": true, "MEM": true, "MEM%": true,
			"CPU Alloc": true, "Mem Alloc": true,
		}
		var newCols []model.KeyValue
		newCols = append(newCols,
			model.KeyValue{Key: "CPU", Value: cpuUse},
			model.KeyValue{Key: "CPU%", Value: cpuPct},
			model.KeyValue{Key: "MEM", Value: memUse},
			model.KeyValue{Key: "MEM%", Value: memPct},
		)
		for _, kv := range item.Columns {
			if !removeCols[kv.Key] {
				newCols = append(newCols, kv)
			}
		}
		item.Columns = newCols
	}
	// Only update the baseline every 60s so trend arrows persist longer.
	if m.prevNodeMetrics == nil || time.Since(m.prevNodeMetricsTime) > 60*time.Second {
		m.prevNodeMetrics = msg.metrics
		m.prevNodeMetricsTime = time.Now()
	}
	m.itemCache[m.navKey()] = m.middleItems
	return m
}

func (m Model) updateSecretDataLoaded(msg secretDataLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Error loading secret: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.secretData = msg.data
	// Snapshot the original data for dirty detection on save.
	m.secretDataOriginal = make(map[string]string, len(msg.data.Data))
	for k, v := range msg.data.Data {
		m.secretDataOriginal[k] = v
	}
	m.secretCursor = 0
	m.secretRevealed = make(map[string]bool)
	m.secretAllRevealed = false
	m.secretEditing = false
	m.secretEditColumn = -1
	m.overlay = overlaySecretEditor
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

func (m Model) updateSecretSaved(msg secretSavedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Error saving secret: ", msg.err)
	} else {
		m.setStatusMessage("Secret saved", false)
		m.overlay = overlayNone
	}
	return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())
}

func (m Model) updateConfigMapDataLoaded(msg configMapDataLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Error loading configmap: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.configMapData = msg.data
	// Snapshot the original data for dirty detection on save.
	m.configMapDataOriginal = make(map[string]string, len(msg.data.Data))
	for k, v := range msg.data.Data {
		m.configMapDataOriginal[k] = v
	}
	m.configMapCursor = 0
	m.configMapEditing = false
	m.configMapEditColumn = -1
	m.overlay = overlayConfigMapEditor
	return m, nil
}

func (m Model) updateConfigMapSaved(msg configMapSavedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Error saving configmap: ", msg.err)
	} else {
		m.setStatusMessage("ConfigMap saved", false)
		m.overlay = overlayNone
	}
	return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())
}

func (m Model) updateLabelDataLoaded(msg labelDataLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Error loading labels: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.labelData = msg.data
	// Snapshot both maps for dirty detection on save.
	m.labelLabelsOriginal = make(map[string]string, len(msg.data.Labels))
	for k, v := range msg.data.Labels {
		m.labelLabelsOriginal[k] = v
	}
	m.labelAnnotationsOriginal = make(map[string]string, len(msg.data.Annotations))
	for k, v := range msg.data.Annotations {
		m.labelAnnotationsOriginal[k] = v
	}
	m.labelCursor = 0
	m.labelTab = 0
	m.labelEditing = false
	m.labelEditColumn = -1
	m.overlay = overlayLabelEditor
	return m, nil
}

func (m Model) updateLabelSaved(msg labelSavedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Error saving labels: ", msg.err)
	} else {
		m.setStatusMessage("Labels/annotations saved", false)
		m.overlay = overlayNone
	}
	return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())
}

func (m Model) updateAutoSyncLoaded(msg autoSyncLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Loading autosync config: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.autoSyncEnabled = msg.enabled
	m.autoSyncSelfHeal = msg.selfHeal
	m.autoSyncPrune = msg.prune
	m.autoSyncCursor = 0
	m.overlay = overlayAutoSync
	return m, nil
}

func (m Model) updateAutoSyncSaved(msg autoSyncSavedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Saving autosync config: ", msg.err)
	} else {
		m.setStatusMessage("AutoSync configuration updated", false)
		m.overlay = overlayNone
	}
	return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())
}

func (m Model) updateExportDone(msg exportDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Export failed: ", msg.err)
	} else {
		m.setStatusMessage("Exported to "+msg.path, false)
	}
	return m, scheduleStatusClear()
}

func (m Model) updateRevisionList(msg revisionListMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Error loading revisions: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.rollbackRevisions = msg.revisions
	m.rollbackCursor = 0
	m.overlay = overlayRollback
	return m, nil
}

func (m Model) updateRollbackDone(msg rollbackDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Rollback failed: ", msg.err)
	} else {
		m.setStatusMessage("Rollback successful", false)
		m.overlay = overlayNone
	}
	return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())
}

func (m Model) updateHelmRevisionList(msg helmRevisionListMsg) (tea.Model, tea.Cmd) {
	m.helmRevisionsLoading = false
	if msg.err != nil {
		m.setErrorFromErr("Error loading Helm revisions: ", msg.err)
		m.overlay = overlayNone
		return m, scheduleStatusClear()
	}
	m.helmRollbackRevisions = msg.revisions
	m.helmRollbackCursor = 0
	m.overlay = overlayHelmRollback
	return m, nil
}

// updateHelmHistoryList handles the helmHistoryListMsg. On error the overlay
// is closed (reset to overlayNone) because executeActionHelmHistory opened it
// optimistically before the fetch completed. On success the revisions are
// populated and the overlay stays open. The shared helmRevisionsLoading flag
// is cleared in either case so the loading placeholder disappears.
func (m Model) updateHelmHistoryList(msg helmHistoryListMsg) (tea.Model, tea.Cmd) {
	m.helmRevisionsLoading = false
	if msg.err != nil {
		m.setErrorFromErr("Error loading Helm history: ", msg.err)
		m.overlay = overlayNone
		return m, scheduleStatusClear()
	}
	m.helmHistoryRevisions = msg.revisions
	m.helmHistoryCursor = 0
	m.overlay = overlayHelmHistory
	return m, nil
}

func (m Model) updateHelmRollbackDone(msg helmRollbackDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Helm rollback failed: ", msg.err)
	} else {
		m.setStatusMessage("Helm rollback successful", false)
		m.overlay = overlayNone
	}
	return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())
}

func (m Model) updateTemplateApply(msg templateApplyMsg) (tea.Model, tea.Cmd) {
	if !msg.origModTime.IsZero() {
		if fi, err := os.Stat(msg.tmpFile); err == nil && fi.ModTime().Equal(msg.origModTime) {
			_ = os.Remove(msg.tmpFile)
			m.setStatusMessage("Template not saved — apply skipped", false)
			return m, scheduleStatusClear()
		}
	}
	return m, m.applyTemplateFile(msg.tmpFile, msg.context, msg.ns)
}

func (m Model) updateExecPTYTick(msg execPTYTickMsg) (tea.Model, tea.Cmd) {
	if msg.ptmx != m.execPTY || m.mode != modeExec {
		// Check if a background tab owns this PTY — keep ticking for it.
		for i := range m.tabs {
			if i != m.activeTab && m.tabs[i].execPTY == msg.ptmx && m.tabs[i].execPTY != nil {
				ptmx := msg.ptmx
				return m, tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
					return execPTYTickMsg{ptmx: ptmx}
				})
			}
		}
		return m, nil
	}
	// Continue ticking to refresh the terminal view.
	return m, m.scheduleExecTick()
}

func (m Model) updateExecPTYExit(msg execPTYExitMsg) Model {
	if msg.ptmx == m.execPTY {
		m.execDone.Store(true)
	} else {
		// Mark the background tab's exec as done.
		for i := range m.tabs {
			if m.tabs[i].execPTY == msg.ptmx {
				m.tabs[i].execDone.Store(true)
				break
			}
		}
	}
	return m
}

func (m Model) updateExecPTYStart(msg execPTYStartMsg) (tea.Model, tea.Cmd) {
	m.execPTY = msg.ptmx
	m.execTerm = msg.term
	m.execTitle = msg.title
	m.execDone = &atomic.Bool{}
	m.execMu = &sync.Mutex{}
	m.mode = modeExec

	// Start background reader goroutine.
	startExecPTYReader(msg.ptmx, msg.term, msg.cmd, m.execMu, m.execDone)

	return m, m.scheduleExecTick()
}

func (m Model) updateLogLine(msg logLineMsg) (tea.Model, tea.Cmd) {
	if msg.ch != m.logCh {
		// Message from a background tab's log stream — buffer it into that tab's state.
		for i := range m.tabs {
			if m.tabs[i].logCh == msg.ch {
				if !msg.done {
					m.tabs[i].logLines = append(m.tabs[i].logLines, msg.line)
					// Continue draining: re-issue waitForLogLine for that channel.
					ch := msg.ch
					return m, func() tea.Msg {
						line, ok := <-ch
						if !ok {
							return logLineMsg{done: true, ch: ch}
						}
						return logLineMsg{line: line, ch: ch}
					}
				}
				break
			}
		}
		return m, nil
	}
	if msg.done {
		// When following all containers of a single Pod, the stream ends as
		// soon as the currently-running set of containers all exit. For a
		// pod still in its init phase that's every init container
		// transition — schedule an auto-reconnect so the next container
		// streams in without manual action. Bail out after
		// logAutoReconnectMaxAttempts consecutive empty reconnects so we
		// don't spin forever once the pod is truly terminated.
		if m.shouldAutoReconnectLogs() && m.logAutoReconnectAttempt < logAutoReconnectMaxAttempts {
			m.logAutoReconnectAttempt++
			return m, m.scheduleLogStreamRestart(msg.ch)
		}
		return m, nil
	}
	// A line arrived — the stream is producing output, so any pending
	// auto-reconnect backoff is no longer relevant.
	if m.logAutoReconnectAttempt > 0 {
		m.logAutoReconnectAttempt = 0
	}
	m.logLines = append(m.logLines, msg.line)
	if m.logFollow {
		m.logScroll = m.logMaxScroll()
		m.logCursor = len(m.logLines) - 1
	}
	return m, m.waitForLogLine()
}

// shouldAutoReconnectLogs reports whether the log stream should automatically
// reconnect when it ends. Auto-reconnect is limited to single-Pod streams
// following all containers while the user is still in follow mode — that's
// the case where kubectl exits on every init-container transition.
// Specific-container, multi-pod, previous-logs, and non-Pod flows either
// have explicit end semantics (--previous) or use selector-based follows
// where "done" doesn't necessarily mean a transition. If the user has
// scrolled away from the tail (logFollow=false) they're reading history,
// not watching live — no point re-arming the stream on their behalf.
func (m Model) shouldAutoReconnectLogs() bool {
	return m.mode == modeLogs &&
		m.logFollow &&
		!m.logIsMulti &&
		!m.logPrevious &&
		m.actionCtx.kind == "Pod" &&
		m.actionCtx.containerName == ""
}

// updateLogStreamRestart fires when a scheduled auto-reconnect is due. If
// the user has switched pods, exited logs mode, or the stream has been
// replaced (e.g. by a manual action), the restart is silently dropped.
func (m Model) updateLogStreamRestart(msg logStreamRestartMsg) (tea.Model, tea.Cmd) {
	if m.mode != modeLogs || m.logCh != msg.ch || !m.shouldAutoReconnectLogs() {
		return m, nil
	}
	m.logReconnecting = true
	cmd := m.startLogStream()
	m.logReconnecting = false
	return m, cmd
}

func (m Model) updateLogHistory(msg logHistoryMsg) Model {
	m.logLoadingHistory = false
	if msg.err != nil {
		m.logHasMoreHistory = false
		return m
	}
	if m.mode != modeLogs {
		return m
	}

	// Find overlap: search for the first 3 current lines in the fetched history.
	overlapIdx := -1
	if len(m.logLines) >= 3 && len(msg.lines) > 3 {
		first3 := m.logLines[:3]
		for i := len(msg.lines) - 3; i >= 0; i-- {
			if msg.lines[i] == first3[0] && msg.lines[i+1] == first3[1] && msg.lines[i+2] == first3[2] {
				overlapIdx = i
				break
			}
		}
	} else if len(m.logLines) > 0 && len(msg.lines) > 0 {
		// Single-line fallback.
		for i := len(msg.lines) - 1; i >= 0; i-- {
			if msg.lines[i] == m.logLines[0] {
				overlapIdx = i
				break
			}
		}
	}

	var newOlderLines []string
	if overlapIdx > 0 {
		newOlderLines = msg.lines[:overlapIdx]
	} else if overlapIdx == -1 && len(msg.lines) > 0 {
		// No overlap found; prepend all (logs may have rotated).
		newOlderLines = msg.lines
	}

	if len(newOlderLines) == 0 {
		m.logHasMoreHistory = false
		return m
	}

	// Prepend and adjust scroll to maintain view position.
	prepended := len(newOlderLines)
	m.logLines = append(newOlderLines, m.logLines...)
	m.logScroll += prepended
	if m.logCursor >= 0 {
		m.logCursor += prepended
	}
	m.logTailLines += ui.ConfigLogTailLines

	// Cap total to prevent unbounded growth.
	if m.logTailLines > 100000 {
		m.logHasMoreHistory = false
	}

	return m
}

func (m Model) updateLogSaveAll(msg logSaveAllMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Log save failed: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.setStatusMessage("All logs saved to "+msg.path, false)
	return m, scheduleStatusClear()
}
