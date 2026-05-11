package app

import (
	"fmt"
	"strings"

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

// ExpandUnionSetConfig flattens a configured union set into the startup fields
// used by both CLI activation and picker activation. Namespace precedence is:
// per-member namespace in union_sets, set-level namespace, then an explicitly
// configured namespace on one of the kubeconfig contexts.
func ExpandUnionSetConfig(
	set ui.UnionSetConfig,
	contextNamespace func(string) (string, bool),
) (contexts []string, namespace string, colors map[string]string) {
	contexts = make([]string, 0, len(set.Contexts))
	colors = make(map[string]string, len(set.Contexts))
	memberNamespace := ""
	kubeconfigNamespace := ""
	for _, ctx := range set.Contexts {
		context := strings.TrimSpace(ctx.Context)
		contexts = append(contexts, context)
		if color := strings.TrimSpace(ctx.Color); color != "" {
			colors[context] = color
		}
		if memberNamespace == "" {
			memberNamespace = strings.TrimSpace(ctx.Namespace)
		}
		if kubeconfigNamespace == "" && contextNamespace != nil {
			if ns, ok := contextNamespace(context); ok {
				kubeconfigNamespace = strings.TrimSpace(ns)
			}
		}
	}
	switch {
	case memberNamespace != "":
		namespace = memberNamespace
	case strings.TrimSpace(set.Namespace) != "":
		namespace = strings.TrimSpace(set.Namespace)
	default:
		namespace = kubeconfigNamespace
	}
	return contexts, namespace, colors
}

func (m Model) withUnionSetRows(items []model.Item) []model.Item {
	if len(ui.ConfigUnionSets) == 0 {
		return items
	}
	out := make([]model.Item, 0, len(ui.ConfigUnionSets)+len(items))
	var namespaceLookup func(string) (string, bool)
	if m.client != nil {
		namespaceLookup = m.client.ContextNamespace
	}
	for _, set := range ui.ConfigUnionSets {
		contexts, namespace, _ := ExpandUnionSetConfig(set, namespaceLookup)
		status := fmt.Sprintf("%d contexts", len(contexts))
		if len(contexts) == 1 {
			status = "1 context"
		}
		if namespace != "" {
			status += "  " + namespace
		}
		out = append(out, model.Item{
			Name:     set.Name,
			Status:   status,
			Kind:     unionSetItemKind,
			Extra:    set.Name,
			Category: unionSetCategory,
		})
	}
	out = append(out, items...)
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
	var namespaceLookup func(string) (string, bool)
	if m.client != nil {
		namespaceLookup = m.client.ContextNamespace
	}
	contexts, namespace, _ := ExpandUnionSetConfig(set, namespaceLookup)
	if namespace == "" {
		return m.openUnionSetNamespacePicker(set, contexts)
	}
	return m.activateUnionSet(set, namespace)
}

func (m Model) openUnionSetNamespacePicker(set ui.UnionSetConfig, contexts []string) (tea.Model, tea.Cmd) {
	if len(contexts) == 0 {
		m.setStatusMessage(fmt.Sprintf("Union set has no contexts: %s", set.Name), true)
		return m, scheduleStatusClear()
	}
	opts := StartupOptions{
		UnionSet:      set.Name,
		UnionContexts: contexts,
		Namespaces:    []string{"pending"},
	}
	var contextExists func(string) bool
	if m.client != nil {
		contextExists = m.client.ContextExists
	}
	if err := ValidateUnionOptions(opts, contextExists); err != nil {
		m.setStatusMessage(err.Error(), true)
		return m, scheduleStatusClear()
	}
	m.pendingUnionSetName = set.Name
	m.allNamespaces = false
	m.selectedNamespaces = nil
	m.namespace = ""
	return m.openNamespaceSelectorForContext(contexts[0])
}

func (m Model) activateUnionSet(set ui.UnionSetConfig, namespace string) (tea.Model, tea.Cmd) {
	var namespaceLookup func(string) (string, bool)
	if m.client != nil {
		namespaceLookup = m.client.ContextNamespace
	}
	contexts, _, colors := ExpandUnionSetConfig(set, namespaceLookup)
	opts := StartupOptions{
		UnionSet:           set.Name,
		UnionContexts:      contexts,
		UnionContextColors: colors,
		Namespaces:         []string{namespace},
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
	m.pendingUnionSetName = ""
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
	m.namespace = namespace
	m.selectedNamespaces = map[string]bool{namespace: true}
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
	activeUnionSet := m.unionSetName
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
	for i, item := range m.middleItems {
		if item.Kind == unionSetItemKind && (item.Extra == activeUnionSet || item.Name == activeUnionSet) {
			m.setCursor(i)
			break
		}
	}
	m.refreshContextReadOnlyMarkers()
	return m, m.loadPreview()
}
