package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) refreshCurrentLevel() tea.Cmd {
	switch m.nav.Level {
	case model.LevelClusters:
		return m.loadContexts()
	case model.LevelResourceTypes:
		// Discovery is cached for the lifetime of the session; without an
		// explicit re-run, newly-installed CRDs (or removed ones) stay
		// hidden until lfk restarts. shift+r at this level should pick
		// them up. Dedup against an already-in-flight discovery so rapid
		// presses don't stack API calls.
		//
		// In union mode at LevelResourceTypes, m.nav.Context is the
		// UnionContextSentinel; never a valid cluster name. effectiveContext
		// resolves the sentinel to unionContexts[0], matching the rest of
		// the union design (discovery is keyed by the first union cluster
		// in updateAPIResourceDiscovery and loadResources). Without this
		// resolution every watch tick fires a discovery against the literal
		// "__union__" string, which restConfigForContext immediately rejects.
		discoveryCtx := m.effectiveContext()
		var cmds []tea.Cmd
		if !m.discoveringContexts[discoveryCtx] {
			if m.discoveringContexts != nil {
				m.discoveringContexts[discoveryCtx] = true
			}
			// Force a round-trip; otherwise shift+r would serve stale cache.
			m.client.InvalidateDiscoveryCache(discoveryCtx)
			cmds = append(cmds, m.discoverAPIResources(discoveryCtx))
		}
		// Drop the preview-cache fingerprint for the hovered resource type
		// so updateResourceTypes' cascade through loadPreview misses the
		// hover-cycle shortcut and fetches fresh data. Without this, a
		// kubectl delete (or any external mutation) leaves the right-pane
		// list stuck on the cached pre-mutation snapshot until the user
		// tab-switches or drills in. The cascade carries the silent flag
		// (set by loadResourceTypesFor from suppressBgtasks) so the watch
		// tick doesn't flash the title-bar indicator on the resulting fetch.
		m.invalidatePreviewFingerprintForCurrentSelection()
		// Always emit the current cached list too so the UI repaints
		// immediately while the fresh discovery runs in the background.
		// updateAPIResourceDiscovery overwrites middleItems on completion.
		cmds = append(cmds, m.loadResourceTypes())
		return tea.Batch(cmds...)
	case model.LevelResources:
		// Port forwards are virtual - refresh from the manager directly.
		// The gen field MUST be captured and forwarded so the update
		// handler doesn't discard the message as stale when requestGen
		// has been bumped by any cursor movement since the cmd was built.
		if m.nav.ResourceType.Kind == "__port_forwards__" {
			gen := m.requestGen
			items := m.portForwardItems()
			return func() tea.Msg {
				return resourcesLoadedMsg{items: items, gen: gen}
			}
		}
		// Captures are also virtual - same pattern as port forwards.
		if m.nav.ResourceType.Kind == "__captures__" {
			gen := m.requestGen
			items := capturesPseudoItems(m.captureMgr)
			return func() tea.Msg {
				return resourcesLoadedMsg{items: items, gen: gen}
			}
		}
		return m.loadResources(false)
	case model.LevelOwned:
		return m.loadOwned(false)
	case model.LevelContainers:
		return m.loadContainers(false)
	}
	return nil
}

// cancelActiveTabLogStreams cancels the live (Model-level) log stream
// and history-fetch contexts. Used by tab-close paths so the closing
// tab's kubectl subprocess and reader goroutine exit immediately, while
// sibling tabs' streams (held in TabState.logCancel) keep running.
func (m *Model) cancelActiveTabLogStreams() {
	if m.logCancel != nil {
		m.logCancel()
		m.logCancel = nil
	}
	if m.logHistoryCancel != nil {
		m.logHistoryCancel()
		m.logHistoryCancel = nil
	}
}

// cancelAllTabLogStreams cancels every log stream owned by the Model:
// the active tab's stream + history (held on Model) and every inactive
// tab's stream (held in TabState.logCancel). Used by quit paths so no
// kubectl subprocess or reader goroutine outlives the lfk process.
func (m *Model) cancelAllTabLogStreams() {
	m.cancelActiveTabLogStreams()
	for i := range m.tabs {
		if m.tabs[i].logCancel != nil {
			m.tabs[i].logCancel()
			m.tabs[i].logCancel = nil
		}
	}
}

// closeTabOrQuit closes the current tab if multiple tabs are open,
// otherwise quits the application (with optional confirmation).
func (m Model) closeTabOrQuit() (tea.Model, tea.Cmd) {
	if len(m.tabs) > 1 {
		m.cancelActiveTabLogStreams()
		m.tabs = append(m.tabs[:m.activeTab], m.tabs[m.activeTab+1:]...)
		if m.activeTab > 0 {
			m.activeTab--
		}
		// Load the surviving tab BEFORE saving session, so saveCurrentTab
		// writes the surviving tab's data (not the closed tab's stale state).
		cmd := m.loadTab(m.activeTab)
		m.saveCurrentSession()
		if cmd != nil {
			return m, cmd
		}
		return m, m.loadPreview()
	}
	// On last tab, show confirmation if configured.
	if ui.ConfigConfirmOnExit {
		m.overlay = overlayQuitConfirm
		return m, nil
	}
	m.performQuitCleanup()
	return m, tea.Quit
}

func (m Model) executeActionScale() Model {
	m.scaleInput.Clear()
	m.overlay = overlayScaleInput
	return m
}

func (m Model) executeActionVulnScan() (tea.Model, tea.Cmd) {
	image := m.actionCtx.image
	if image == "" {
		m.setStatusMessage("No image found for this container", true)
		return m, scheduleStatusClear()
	}
	m.addLogEntry("DBG", fmt.Sprintf("$ trivy image %s", image))
	m.loading = true
	m.setStatusMessage("Scanning image for vulnerabilities...", false)
	return m, m.vulnScanImage(image)
}

func (m Model) executeActionVisualize() (tea.Model, tea.Cmd) {
	m.loading = true
	m.setStatusMessage("Loading network policy...", false)
	return m, m.loadNetworkPolicy()
}

func (m Model) executeActionDefault(actionLabel string) (tea.Model, tea.Cmd) {
	if ca, ok := findCustomAction(m.actionCtx.kind, actionLabel); ok {
		// Custom actions are arbitrary shell commands. Block them in
		// read-only mode unless the user explicitly marked the action
		// safe via read_only_safe: true. The dispatcher gate at the top
		// of executeAction only checks the static mutatingActions set,
		// which doesn't know about user-defined labels; this is the
		// last chance to refuse.
		if m.readOnly && !ca.ReadOnlySafe {
			m.setStatusMessage(readOnlyBlockedMessage(actionLabel), true)
			return m, scheduleStatusClear()
		}
		expandedCmd := expandCustomActionTemplate(ca.Command, m.actionCtx)
		m.addLogEntry("DBG", fmt.Sprintf("$ sh -c %q", expandedCmd))
		return m, m.execCustomAction(expandedCmd)
	}
	return m, nil
}
