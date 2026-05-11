package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func isUnionSetItem(item *model.Item) bool {
	return item != nil && item.Kind == unionSetItemKind
}

func (m Model) findUnionSetConfig(name string) (ui.UnionSetConfig, bool) {
	for _, set := range ui.ConfigUnionSets {
		if set.Name == name {
			return set, true
		}
	}
	return ui.UnionSetConfig{}, false
}

func unionSetContextsAndColors(set ui.UnionSetConfig) ([]string, map[string]string) {
	contexts := make([]string, 0, len(set.Contexts))
	colors := make(map[string]string, len(set.Contexts))
	for _, ctx := range set.Contexts {
		contexts = append(contexts, ctx.Context)
		if ctx.Color != "" {
			colors[ctx.Context] = ctx.Color
		}
	}
	return contexts, colors
}

func (m Model) withUnionSetRows(items []model.Item) []model.Item {
	if len(ui.ConfigUnionSets) == 0 {
		return items
	}
	out := append([]model.Item(nil), items...)
	for _, set := range ui.ConfigUnionSets {
		status := fmt.Sprintf("%d contexts", len(set.Contexts))
		if len(set.Contexts) == 1 {
			status = "1 context"
		}
		if set.Namespace != "" {
			status += "  " + set.Namespace
		}
		out = append(out, model.Item{
			Name:     set.Name,
			Status:   status,
			Kind:     unionSetItemKind,
			Extra:    set.Name,
			Category: unionSetCategory,
		})
	}
	return out
}

func unionSetPreviewItems(set ui.UnionSetConfig) []model.Item {
	items := make([]model.Item, 0, len(set.Contexts))
	for _, ctx := range set.Contexts {
		status := "context"
		if ctx.Color != "" {
			status = ctx.Color
		}
		items = append(items, model.Item{
			Name:         ctx.Context,
			Status:       status,
			ClusterColor: ctx.Color,
		})
	}
	return items
}

func (m Model) navigateChildUnionSet(sel *model.Item) (tea.Model, tea.Cmd) {
	set, ok := m.findUnionSetConfig(sel.Extra)
	if !ok {
		m.setStatusMessage(fmt.Sprintf("Union set not found: %s", sel.Name), true)
		return m, scheduleStatusClear()
	}
	contexts, colors := unionSetContextsAndColors(set)
	opts := StartupOptions{
		UnionSet:           set.Name,
		UnionContexts:      contexts,
		UnionContextColors: colors,
	}
	if set.Namespace != "" {
		opts.Namespaces = []string{set.Namespace}
	}
	var contextExists func(string) bool
	if m.client != nil {
		contextExists = m.client.ContextExists
	}
	if err := ValidateUnionOptions(opts, contextExists); err != nil {
		m.setStatusMessage(err.Error(), true)
		return m, scheduleStatusClear()
	}

	m.saveCursor()
	oldCtx := m.nav.Context
	m.unionMode = true
	m.unionStartedFromPicker = true
	m.unionSetName = set.Name
	m.unionContexts = contexts
	m.unionContextColors = colors
	m.nav.Context = UnionContextSentinel
	m.nav.Level = model.LevelResourceTypes
	m.nav.ResourceType = model.ResourceTypeEntry{}
	m.nav.ResourceName = ""
	m.nav.OwnedName = ""
	m.nav.Namespace = ""
	m.invalidateOrphanCacheForContext(oldCtx)
	m.readOnly = m.cliReadOnly
	if len(m.tabs) > m.activeTab {
		m.tabs[m.activeTab].readOnly = m.readOnly
	}
	m.allNamespaces = false
	m.namespace = set.Namespace
	m.selectedNamespaces = map[string]bool{set.Namespace: true}
	m.dashboardPreview = ""
	m.dashboardEventsPreview = ""
	m.monitoringPreview = ""
	m.applyPinnedGroups()
	m.pushLeft()
	m.clearRight()

	discoveryCtx := contexts[0]
	switch {
	case len(m.discoveredResources[discoveryCtx]) > 0:
		m.setMiddleItems(model.BuildSidebarItems(m.discoveredResources[discoveryCtx]))
		m.itemCache[m.navKey()] = m.middleItems
		m.restoreCursor()
		m.syncExpandedGroup()
	default:
		m.setMiddleItems(nil)
		m.loading = true
	}
	m.setStatusMessage("Union set: "+set.Name, false)
	m.saveCurrentSession()

	cmds := []tea.Cmd{m.loadPreview(), scheduleStatusClear()}
	if m.shouldFireDiscoveryFor(discoveryCtx) {
		m.markDiscoveryStarted(discoveryCtx)
		cmds = append(cmds, m.discoverAPIResources(discoveryCtx))
	}
	return m, tea.Batch(cmds...)
}

func (m Model) navigateParentFromPickerUnion() (tea.Model, tea.Cmd) {
	m.saveCursor()
	m.unionMode = false
	m.unionStartedFromPicker = false
	m.unionSetName = ""
	m.unionContexts = nil
	m.unionContextColors = nil
	m.nav.Level = model.LevelClusters
	m.nav.Context = ""
	m.nav.ResourceType = model.ResourceTypeEntry{}
	m.nav.ResourceName = ""
	m.nav.OwnedName = ""
	m.nav.Namespace = ""
	m.dashboardPreview = ""
	m.dashboardEventsPreview = ""
	m.monitoringPreview = ""
	m.applyPinnedGroups()
	m.setMiddleItems(m.leftItems)
	m.popLeft()
	m.clearRight()
	m.restoreCursor()
	m.refreshContextReadOnlyMarkers()
	return m, m.loadPreview()
}
