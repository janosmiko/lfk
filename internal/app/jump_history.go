package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
	"github.com/janosmiko/lfk/internal/ui"
)

// jumpHistoryCap bounds the jump-back stack. When a push would exceed it,
// the oldest entry is dropped.
const jumpHistoryCap = 50

// navSnapshot captures only the navigation-positioning state needed to restore
// the explorer to a previous location after a "teleport" jump. It deliberately
// does NOT snapshot the whole tabState — just enough to repaint the columns and
// re-issue a data load for the restored level.
type navSnapshot struct {
	nav              model.NavigationState
	leftItems        []model.Item
	middleItems      []model.Item
	leftItemsHistory [][]model.Item
	cursors          [5]int
	middleScroll     int // ui.ActiveMiddleScroll
	leftScroll       int // ui.ActiveLeftScroll
	expandedGroup    string
	filterText       string
	// securityActiveGroup preserves the security finding group whose
	// affected-resources view the snapshot was taken in, so a jump-back
	// reloads affected resources instead of owned children.
	securityActiveGroup string
	// securityActiveSource pairs with securityActiveGroup: the group's
	// source, needed to reload affected resources when the snapshot was
	// taken inside the cross-source per-resource findings view.
	securityActiveSource string
	// securityResourceFilter preserves the per-resource findings view's
	// ref filter so a jump-back into that view reloads the same list.
	securityResourceFilter []security.ResourceRef
	// ownedParentStack preserves nested owned-drill ancestry (Deployment ->
	// ReplicaSet -> Pods) so back-navigation after a jump-back walks the
	// same chain the user descended.
	ownedParentStack []ownedParentState
}

// captureNavSnapshot records the current navigation-positioning state with all
// slices deep-copied so later mutation of the Model can't corrupt the snapshot.
func (m *Model) captureNavSnapshot() navSnapshot {
	snap := navSnapshot{
		nav:                    m.nav,
		leftItems:              append([]model.Item(nil), m.leftItems...),
		middleItems:            append([]model.Item(nil), m.middleItems...),
		cursors:                m.cursors,
		middleScroll:           ui.ActiveMiddleScroll,
		leftScroll:             ui.ActiveLeftScroll,
		expandedGroup:          m.expandedGroup,
		filterText:             m.filterText,
		securityActiveGroup:    m.securityActiveGroup,
		securityActiveSource:   m.securityActiveSource,
		securityResourceFilter: append([]security.ResourceRef(nil), m.securityResourceFilter...),
		ownedParentStack:       append([]ownedParentState(nil), m.ownedParentStack...),
	}
	snap.leftItemsHistory = make([][]model.Item, len(m.leftItemsHistory))
	for i, hist := range m.leftItemsHistory {
		snap.leftItemsHistory[i] = append([]model.Item(nil), hist...)
	}
	// nav is copied by value above, but ResourceTypeEntry embeds slice
	// headers that would otherwise alias the live Model. Deep-copy them.
	snap.nav.ResourceType.Verbs = append([]string(nil), m.nav.ResourceType.Verbs...)
	snap.nav.ResourceType.PrinterColumns = append([]model.PrinterColumn(nil), m.nav.ResourceType.PrinterColumns...)
	return snap
}

// clone returns a fully deep-copied navSnapshot. Every inner slice is
// reallocated so the result shares no backing storage with the original. This
// makes the "snapshots are never mutated" invariant enforced by construction.
func (s navSnapshot) clone() navSnapshot {
	out := s
	out.leftItems = append([]model.Item(nil), s.leftItems...)
	out.middleItems = append([]model.Item(nil), s.middleItems...)
	out.securityResourceFilter = append([]security.ResourceRef(nil), s.securityResourceFilter...)
	out.ownedParentStack = append([]ownedParentState(nil), s.ownedParentStack...)
	out.leftItemsHistory = make([][]model.Item, len(s.leftItemsHistory))
	for i, hist := range s.leftItemsHistory {
		out.leftItemsHistory[i] = append([]model.Item(nil), hist...)
	}
	out.nav.ResourceType.Verbs = append([]string(nil), s.nav.ResourceType.Verbs...)
	out.nav.ResourceType.PrinterColumns = append([]model.PrinterColumn(nil), s.nav.ResourceType.PrinterColumns...)
	return out
}

// cloneNavSnapshots returns a deep copy of a navSnapshot slice, with each
// element structurally cloned via clone().
func cloneNavSnapshots(src []navSnapshot) []navSnapshot {
	if src == nil {
		return nil
	}
	out := make([]navSnapshot, len(src))
	for i, s := range src {
		out[i] = s.clone()
	}
	return out
}

// pushJumpHistory records the current navigation state on the back stack. It
// must be called by every "teleport" site BEFORE it mutates navigation state,
// so jump_back can restore the user's origin.
func (m *Model) pushJumpHistory() {
	m.jumpBackStack = append(m.jumpBackStack, m.captureNavSnapshot())
	if len(m.jumpBackStack) > jumpHistoryCap {
		m.jumpBackStack = m.jumpBackStack[len(m.jumpBackStack)-jumpHistoryCap:]
	}
}

// restoreNavSnapshot applies a snapshot back onto the Model and returns a
// command that reloads data for the restored level. Restore is graceful: if the
// snapshot targets a level deeper than LevelClusters but its context is no
// longer known, it falls back to the nearest valid level and surfaces a status
// message instead of crashing.
func (m *Model) restoreNavSnapshot(snap navSnapshot) tea.Cmd {
	// Graceful fallback: a snapshot below the cluster picker needs a context
	// that still exists. If discovery no longer knows it, drop to the cluster
	// picker rather than restoring into a dead context.
	if snap.nav.Level > model.LevelClusters && snap.nav.Context != "" {
		if !m.contextStillValid(snap.nav.Context) {
			// A jump-back is a navigation transition: cancel in-flight work and
			// bump requestGen so async responses from the pre-fallback view
			// can't apply after this reset (mirrors navigateParent).
			m.cancelAndReset()
			m.requestGen++
			m.setStatusMessage("Jump target no longer available — returned to clusters", true)
			m.nav = model.NavigationState{Level: model.LevelClusters}
			m.leftItemsHistory = nil
			m.expandedGroup = ""
			// The pre-fallback view's security/owned jump context has no
			// meaning at the cluster picker; keeping it would leak a stale
			// finding group or owned-drill ancestry into the next cluster.
			m.securityActiveGroup = ""
			m.securityActiveSource = ""
			m.securityResourceFilter = nil
			m.ownedParentStack = nil
			m.filterText = ""
			m.filterInput.Clear()
			m.filterActive = false
			ui.ActiveMiddleScroll = 0
			ui.ActiveLeftScroll = 0
			m.clearRight()
			m.clearSelection()
			// Repopulate the cluster picker. On error keep the fallback flow
			// (empty list) but surface the read failure rather than dropping it.
			contexts, err := m.client.GetContexts()
			if err != nil {
				m.setErrorFromErr("Failed to load contexts: ", err)
			}
			m.leftItems = nil
			m.setMiddleItems(contexts)
			m.cursors = [5]int{}
			m.clampCursor()
			return tea.Batch(m.loadPreview(), scheduleStatusClear())
		}
	}

	m.cancelAndReset()
	m.requestGen++
	m.clearSelection()
	m.activeFilterPreset = nil
	m.unfilteredMiddleItems = nil

	m.nav = snap.nav
	m.leftItems = append([]model.Item(nil), snap.leftItems...)
	m.leftItemsHistory = make([][]model.Item, len(snap.leftItemsHistory))
	for i, hist := range snap.leftItemsHistory {
		m.leftItemsHistory[i] = append([]model.Item(nil), hist...)
	}
	m.cursors = snap.cursors
	m.expandedGroup = snap.expandedGroup
	m.securityActiveGroup = snap.securityActiveGroup
	m.securityActiveSource = snap.securityActiveSource
	m.securityResourceFilter = append([]security.ResourceRef(nil), snap.securityResourceFilter...)
	m.ownedParentStack = append([]ownedParentState(nil), snap.ownedParentStack...)

	m.filterText = snap.filterText
	m.filterInput.Clear()
	m.filterActive = false
	m.searchInput.Clear()

	ui.ActiveMiddleScroll = snap.middleScroll
	ui.ActiveLeftScroll = snap.leftScroll

	// Instant paint from the snapshot; the reload below refreshes it.
	m.setMiddleItems(append([]model.Item(nil), snap.middleItems...))
	m.clearRight()
	m.clampCursor()

	switch m.nav.Level {
	case model.LevelResources:
		m.loading = true
		return m.loadResources(false)
	case model.LevelOwned:
		// A security finding's affected-resources view also lives at
		// LevelOwned, but its data comes from the findings index — loadOwned
		// would look for owner references of a non-existent resource and
		// blank the panes with "No resources found".
		if strings.HasPrefix(m.nav.ResourceType.Kind, "__security_") {
			if cmd := m.loadSecurityAffectedResources(false); cmd != nil {
				m.loading = true
				return cmd
			}
			// No manager/group to reload from (e.g. security disabled since
			// the snapshot) — keep the instant-painted snapshot items rather
			// than spinning forever.
			return m.loadPreview()
		}
		m.loading = true
		return m.loadOwned(false)
	case model.LevelContainers:
		m.loading = true
		return m.loadContainers(false)
	default:
		return m.loadPreview()
	}
}

// contextStillValid reports whether the given Kubernetes context is still known
// to the client, so a restore doesn't land in a context that has gone away.
func (m *Model) contextStillValid(ctx string) bool {
	contexts, err := m.client.GetContexts()
	if err != nil {
		// Can't tell — assume valid rather than blocking a legitimate restore.
		return true
	}
	for _, c := range contexts {
		if c.Name == ctx {
			return true
		}
	}
	return false
}

// jumpBack restores the navigation state recorded before the last teleport.
// No-op when the back stack is empty.
func (m Model) jumpBack() (tea.Model, tea.Cmd) {
	n := len(m.jumpBackStack)
	if n == 0 {
		m.setStatusMessage("No jump history to go back to", false)
		return m, scheduleStatusClear()
	}
	snap := m.jumpBackStack[n-1]
	m.jumpBackStack = m.jumpBackStack[:n-1]
	cmd := m.restoreNavSnapshot(snap)
	m.saveCurrentSession()
	return m, cmd
}
