package app

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// gotoTarget is one g-prefix goto entry: a full chord and the resource type it
// jumps to.
type gotoTarget struct {
	Chord string // full chord, e.g. "gp"
	Kind  string
	Group string // APIGroup; "" for core
	Label string
}

// gotoTargets returns the active goto targets: built-ins from the keybinding
// config, then user-defined goto_targets (Task 3) which override built-ins on
// chord collision. Order is stable for rendering.
func (m Model) gotoTargets() []gotoTarget {
	kb := ui.ActiveKeybindings
	base := []gotoTarget{
		{kb.GotoPods, "Pod", "", "Pods"},
		{kb.GotoDeployments, "Deployment", "apps", "Deployments"},
		{kb.GotoServices, "Service", "", "Services"},
		{kb.GotoNodes, "Node", "", "Nodes"},
		{kb.GotoNamespaces, "Namespace", "", "Namespaces"},
		{kb.GotoIngresses, "Ingress", "networking.k8s.io", "Ingresses"},
		{kb.GotoJobs, "Job", "batch", "Jobs"},
		{kb.GotoCronJobs, "CronJob", "batch", "CronJobs"},
		{kb.GotoReplicaSets, "ReplicaSet", "apps", "ReplicaSets"},
		{kb.GotoDaemonSets, "DaemonSet", "apps", "DaemonSets"},
		{kb.GotoStatefulSets, "StatefulSet", "apps", "StatefulSets"},
		{kb.GotoConfigMaps, "ConfigMap", "", "ConfigMaps"},
		{kb.GotoSecrets, "Secret", "", "Secrets"},
		{kb.GotoHPAs, "HorizontalPodAutoscaler", "autoscaling", "HPAs"},
		{kb.GotoPVCs, "PersistentVolumeClaim", "", "PVCs"},
		{kb.GotoPVs, "PersistentVolume", "", "PVs"},
		{kb.GotoPDBs, "PodDisruptionBudget", "policy", "PDBs"},
	}
	out := make([]gotoTarget, 0, len(base)+len(ui.ConfigGotoTargets))
	idx := map[string]int{}
	for _, gt := range base {
		if gt.Chord == "" {
			continue
		}
		idx[gt.Chord] = len(out)
		out = append(out, gt)
	}
	// Custom targets override built-ins on chord collision. This loop is a
	// no-op until Task 3 populates ui.ConfigGotoTargets.
	for chord, ct := range ui.ConfigGotoTargets {
		label := ct.Name
		if label == "" {
			label = ct.Kind
		}
		gt := gotoTarget{Chord: chord, Kind: ct.Kind, Group: ct.Group, Label: label}
		if i, ok := idx[chord]; ok {
			out[i] = gt
			continue
		}
		idx[chord] = len(out)
		out = append(out, gt)
	}
	return out
}

// gotoTargetForChord looks up a goto target by its full chord (e.g. "gp").
func (m Model) gotoTargetForChord(chord string) (gotoTarget, bool) {
	for _, gt := range m.gotoTargets() {
		if gt.Chord == chord {
			return gt, true
		}
	}
	return gotoTarget{}, false
}

// gotoResourceType switches the current tab to the given resource type in the
// active context, preserving the namespace filter. It mirrors the descend path
// of navigateChildResourceType. Requires an active cluster/context.
func (m Model) gotoResourceType(kind, apiGroup string) (tea.Model, tea.Cmd) {
	if m.nav.Level == model.LevelClusters {
		m.setStatusMessage("Select a cluster first", true)
		return m, scheduleStatusClear()
	}
	discoveryCtx := m.nav.Context
	if m.isUnionSentinel() && len(m.unionContexts) > 0 {
		discoveryCtx = m.unionContexts[0]
	}
	rt, ok := model.FindResourceTypeByKindAndGroup(kind, apiGroup, m.discoveredResources[discoveryCtx])
	if !ok {
		m.setStatusMessage(fmt.Sprintf("%s not available in this cluster", kind), true)
		return m, scheduleStatusClear()
	}
	m.saveCursor()
	// Start the destination list clean (TASK-839). Deliberately no
	// restoreLevelFilter here — like a descend, a goto is a fresh start; only
	// back-nav (navigateParent) recalls a saved filter.
	m.resetFilterForTypeSwitch()
	m.nav.ResourceType = rt
	m.applyResourceTypeSortDefault(m.nav.ResourceType, m.nav.Context)
	m.nav.Level = model.LevelResources
	m.normalizeGotoPanes()
	m.primeTypesReturnCursor(rt)
	m.saveCurrentSession()
	if cached, hit := m.itemCache[m.navKey()]; hit {
		m.setMiddleItems(cached)
		m.restoreCursor()
	} else {
		m.setMiddleItems(nil)
		m.setCursor(0)
	}
	m.loading = true
	return m, m.loadResources(false)
}

// normalizeGotoPanes rebuilds the left pane for a clean LevelResources view
// regardless of the level the goto fired from. After this call, leftItems holds
// the resource-types sidebar so that pressing h/left returns the user to the
// correct types list rather than a stale resources or owned pane.
func (m *Model) normalizeGotoPanes() {
	discoveryCtx := m.nav.Context
	if m.isUnionSentinel() && len(m.unionContexts) > 0 {
		discoveryCtx = m.unionContexts[0]
	}
	// Build a fresh resource-types sidebar for the left pane. This is the
	// same item set that navigateChildResourceType's pushLeft() would promote.
	typesItems := model.BuildSidebarItems(m.discoveredResources[discoveryCtx])

	// Retain only the deepest cluster-level history entry so that navigating
	// back past LevelResources -> LevelResourceTypes -> LevelClusters still
	// works correctly. The history at index 0 (if any) holds the cluster
	// picker items; everything above it was intermediate pane state that the
	// goto jumps over.
	var clusterHistory [][]model.Item
	if len(m.leftItemsHistory) > 0 {
		clusterHistory = [][]model.Item{m.leftItemsHistory[0]}
	}
	m.leftItemsHistory = clusterHistory
	m.leftItems = typesItems
	m.clearRight()
}

// primeTypesReturnCursor records the parent (LevelResourceTypes) cursor so that
// pressing h/left from the jumped-to resource list lands on the resource type we
// jumped to. A goto never moves the types-list cursor onto the target the way a
// manual descent does, so restoreCursor would otherwise recall a stale highlight.
// The index is computed through the real collapse logic (visibleMiddleItems) on a
// throwaway copy positioned at LevelResourceTypes, and the target's group is
// expanded so the recorded index and the restored view agree. Must run after
// normalizeGotoPanes (which populates leftItems) and after saveCursor.
func (m *Model) primeTypesReturnCursor(rt model.ResourceTypeEntry) {
	ref := rt.ResourceRef()
	if ref == "" || ref == "//" {
		return
	}
	// Expand the target's group so it renders as a real (non-collapsed) row.
	for _, it := range m.leftItems {
		if it.Extra == ref {
			if it.Category != "" {
				m.expandedGroup = it.Category
			}
			break
		}
	}
	probe := *m
	probe.nav.Level = model.LevelResourceTypes
	probe.nav.ResourceType = model.ResourceTypeEntry{}
	probe.middleItems = m.leftItems
	probe.filterText = ""
	idx := indexByExtra(probe.visibleMiddleItems(), ref)
	if idx < 0 {
		return
	}
	m.cursorMemory[probe.navKey()] = idx
}

// handleGotoChord is called while the g prefix is armed (m.pendingG). The
// second "g" falls through to the gg jump-top handler. Every other key closes
// the prefix and the popup (which-key semantics): a registered chord navigates,
// and an unregistered key (e.g. gP) is swallowed as a silent noop rather than
// falling through to the key's normal explorer action.
func (m Model) handleGotoChord(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	key := msg.String()
	if key == ui.ActiveKeybindings.JumpTop { // "g" -> belongs to gg jump-top
		return m, nil, false
	}
	m.pendingG = false
	m.whichKey.shown = false
	chord := ui.ActiveKeybindings.JumpTop + key
	if pn := ui.ActiveKeybindings.PreviousNamespace; pn != "" && chord == pn {
		out, cmd := m.jumpToPreviousNamespace()
		return out, cmd, true
	}
	if gt, ok := m.gotoTargetForChord(chord); ok {
		out, cmd := m.gotoResourceType(gt.Kind, gt.Group)
		return out, cmd, true
	}
	return m, nil, true
}

// whichKeyTickMsg fires after which_key_delay_ms to reveal the popup.
type whichKeyTickMsg struct{}

// armWhichKey is called when the g prefix arms. With no delay it shows the
// popup immediately; otherwise it schedules a reveal tick.
func (m Model) armWhichKey() (Model, tea.Cmd) {
	if !ui.ConfigWhichKeyEnabled {
		m.whichKey.shown = false
		return m, nil
	}
	if ui.ConfigWhichKeyDelayMs <= 0 {
		m.whichKey.shown = true
		return m, nil
	}
	m.whichKey.shown = false
	d := time.Duration(ui.ConfigWhichKeyDelayMs) * time.Millisecond
	return m, tea.Tick(d, func(time.Time) tea.Msg { return whichKeyTickMsg{} })
}

// whichKeyCells builds the sorted continuation entries for the g prefix: gg
// (list top) plus every goto target, keyed by the part of the chord after the
// prefix. Sorted alphanumerically like neovim's which-key.
func (m Model) whichKeyCells() []whichKeyCell {
	prefix := ui.ActiveKeybindings.JumpTop
	targets := m.gotoTargets()
	cells := make([]whichKeyCell, 0, len(targets)+2)
	cells = append(cells, whichKeyCell{prefix, "list top"})
	for _, gt := range targets {
		cells = append(cells, whichKeyCell{strings.TrimPrefix(gt.Chord, prefix), gt.Label})
	}
	if pn := ui.ActiveKeybindings.PreviousNamespace; pn != "" {
		cells = append(cells, whichKeyCell{strings.TrimPrefix(pn, prefix), "Previous namespace"})
	}
	sort.SliceStable(cells, func(i, j int) bool {
		li, lj := strings.ToLower(cells[i].key), strings.ToLower(cells[j].key)
		if li != lj {
			return li < lj
		}
		return cells[i].key < cells[j].key
	})
	return cells
}
