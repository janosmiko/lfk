package app

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
		{kb.GotoArgoApplications, "Application", "argoproj.io", "ArgoCD Applications"},
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
	m.nav.ResourceType = rt
	m.applyResourceTypeSortDefault(m.nav.ResourceType, m.nav.Context)
	m.nav.Level = model.LevelResources
	m.normalizeGotoPanes()
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

// handleGotoChord is called while the g prefix is armed (m.pendingG). The
// second "g" falls through to the gg jump-top handler. Every other key closes
// the prefix and the popup (which-key semantics): a registered chord navigates,
// and an unregistered key (e.g. gP) is swallowed as a silent noop rather than
// falling through to the key's normal explorer action.
func (m Model) handleGotoChord(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	key := msg.String()
	if key == ui.ActiveKeybindings.JumpTop { // "g" -> belongs to gg jump-top
		return m, nil, false
	}
	m.pendingG = false
	m.whichKeyShown = false
	if gt, ok := m.gotoTargetForChord(ui.ActiveKeybindings.JumpTop + key); ok {
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
		m.whichKeyShown = false
		return m, nil
	}
	if ui.ConfigWhichKeyDelayMs <= 0 {
		m.whichKeyShown = true
		return m, nil
	}
	m.whichKeyShown = false
	d := time.Duration(ui.ConfigWhichKeyDelayMs) * time.Millisecond
	return m, tea.Tick(d, func(time.Time) tea.Msg { return whichKeyTickMsg{} })
}

// whichKeyCell is one continuation key and its label in the which-key panel.
type whichKeyCell struct {
	key  string // continuation after the prefix, e.g. "p" for "gp"
	desc string
}

// whichKeyCells builds the sorted continuation entries for the g prefix: gg
// (list top) plus every goto target, keyed by the part of the chord after the
// prefix. Sorted alphanumerically like neovim's which-key.
func (m Model) whichKeyCells() []whichKeyCell {
	prefix := ui.ActiveKeybindings.JumpTop
	targets := m.gotoTargets()
	cells := make([]whichKeyCell, 0, len(targets)+1)
	cells = append(cells, whichKeyCell{prefix, "list top"})
	for _, gt := range targets {
		cells = append(cells, whichKeyCell{strings.TrimPrefix(gt.Chord, prefix), gt.Label})
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

// renderWhichKey draws the goto cheatsheet anchored to the bottom of the screen
// when the g prefix is armed and visible, styled after neovim's which-key
// "modern" preset: a compact full-width double-bordered panel of key/desc
// columns. Returns background unchanged when hidden/disabled.
func (m Model) renderWhichKey(background string) string {
	if !m.pendingG || !m.whichKeyShown || !ui.ConfigWhichKeyEnabled {
		return background
	}
	cells := m.whichKeyCells()
	if len(cells) == 0 {
		return background
	}

	// Panel-local styles: foreground only, all on the theme base background so
	// the panel matches the theme rather than carrying the status bar's grey
	// surface. Keys use the help-key accent; descriptions use normal text.
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorSecondary)).Bold(true).Background(ui.BaseBg)
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorFile)).Background(ui.BaseBg)

	// Column width is the widest "key desc" pair, floored at which-key's
	// min column width of 5.
	plain := make([]string, len(cells))
	styled := make([]string, len(cells))
	cellW := 5
	for i, c := range cells {
		plain[i] = c.key + " " + c.desc
		styled[i] = keyStyle.Render(c.key) + " " + descStyle.Render(c.desc)
		cellW = max(cellW, lipgloss.Width(plain[i]))
	}

	const gap = 1
	// Panel spans ~75% of the screen width, centered, with the rounded border
	// inside that budget.
	inner := max(m.width*3/4-2, cellW)
	cols := max((inner+gap)/(cellW+gap), 1)
	rows := (len(cells) + cols - 1) / cols

	// Spread the columns across the full inner width instead of packing them
	// left: each column is at least cellW wide, and the rounding remainder is
	// absorbed by the leftmost columns.
	avail := max(inner-gap*(cols-1), cols)
	baseW, rem := avail/cols, avail%cols
	colW := func(c int) int {
		if c < rem {
			return baseW + 1
		}
		return baseW
	}

	body := make([]string, rows)
	for r := range rows {
		parts := make([]string, cols)
		for c := range cols {
			idx := c*rows + r // column-major: fill down each column first
			if idx >= len(cells) {
				parts[c] = strings.Repeat(" ", colW(c))
				continue
			}
			pad := max(colW(c)-lipgloss.Width(plain[idx]), 0)
			parts[c] = styled[idx] + strings.Repeat(" ", pad)
		}
		body[r] = strings.Join(parts, strings.Repeat(" ", gap))
	}
	// A blank leading row gives the first entries breathing room below the
	// top border.
	lines := append([]string{""}, body...)
	content := ui.FillLinesBg(strings.Join(lines, "\n"), inner, ui.BaseBg)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ui.ColorPrimary)).
		Background(ui.BaseBg).
		Width(inner).
		Render(content)

	// Dim the screen behind the panel like the other overlays do, then lift
	// the panel one row off the bottom so the hint bar stays visible.
	bg := ui.PadToHeight(background, m.height)
	if ui.ConfigDimOverlay {
		bg = ui.DimBackground(bg, 1)
	}
	return ui.PlaceOverlayBottom(m.width, m.height, 1, box, bg)
}
