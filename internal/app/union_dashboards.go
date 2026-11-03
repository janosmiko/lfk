package app

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

type unionDashboardMode string

const (
	unionDashboardCluster    unionDashboardMode = "__overview__"
	unionDashboardMonitoring unionDashboardMode = "__monitoring__"
)

func unionDashboardModeFromExtra(extra string) (unionDashboardMode, bool) {
	switch extra {
	case string(unionDashboardCluster):
		return unionDashboardCluster, true
	case string(unionDashboardMonitoring):
		return unionDashboardMonitoring, true
	default:
		return "", false
	}
}

func unionDashboardModeFromKind(kind string) (unionDashboardMode, bool) {
	switch kind {
	case unionClusterDashboardKind:
		return unionDashboardCluster, true
	case unionMonitoringDashboardKind:
		return unionDashboardMonitoring, true
	default:
		return "", false
	}
}

func isUnionDashboardResourceKind(kind string) bool {
	_, ok := unionDashboardModeFromKind(kind)
	return ok
}

func unionDashboardResourceType(mode unionDashboardMode) model.ResourceTypeEntry {
	switch mode {
	case unionDashboardMonitoring:
		return model.ResourceTypeEntry{
			DisplayName: "Monitoring",
			Kind:        unionMonitoringDashboardKind,
			APIGroup:    unionDashboardMemberAPIGroup,
			APIVersion:  "v1",
			Resource:    unionMonitoringDashboardResource,
			Namespaced:  false,
		}
	default:
		return model.ResourceTypeEntry{
			DisplayName: "Cluster",
			Kind:        unionClusterDashboardKind,
			APIGroup:    unionDashboardMemberAPIGroup,
			APIVersion:  "v1",
			Resource:    unionClusterDashboardResource,
			Namespaced:  false,
		}
	}
}

func unionDashboardMemberItems(contexts []string, colors map[string]string, mode unionDashboardMode, namespace string) []model.Item {
	items := make([]model.Item, 0, len(contexts))
	status := "Cluster dashboard"
	if mode == unionDashboardMonitoring {
		status = "Monitoring dashboard"
	}
	if namespace = strings.TrimSpace(namespace); namespace != "" {
		status += "  " + namespace
	}
	for _, ctx := range contexts {
		items = append(items, model.Item{
			Name:         ctx,
			Status:       status,
			Kind:         unionDashboardMemberItemKind,
			Extra:        string(mode),
			ClusterName:  ctx,
			ClusterColor: colors[ctx],
			Columns: []model.KeyValue{
				{Key: "Context", Value: ctx},
				{Key: "Dashboard", Value: dashboardModeLabel(mode)},
			},
		})
	}
	return items
}

func dashboardModeLabel(mode unionDashboardMode) string {
	if mode == unionDashboardMonitoring {
		return "Monitoring"
	}
	return "Cluster"
}

func isUnionDashboardMemberList(items []model.Item) bool {
	return len(items) > 0 && items[0].Kind == unionDashboardMemberItemKind
}

func (m Model) hasUnionDashboardMemberBreadcrumb() bool {
	if isUnionDashboardMemberList(m.leftItems) || isUnionDashboardMemberList(m.middleItems) {
		return true
	}
	return slices.ContainsFunc(m.leftItemsHistory, isUnionDashboardMemberList)
}

func (m Model) dashboardPreviewTargetContext() string {
	if isUnionDashboardResourceKind(m.nav.ResourceType.Kind) {
		if sel := m.selectedMiddleItem(); sel != nil && sel.ClusterName != "" {
			return sel.ClusterName
		}
	}
	return m.nav.Context
}

func (m Model) loadPreviewUnionDashboardMember() tea.Cmd {
	sel := m.selectedMiddleItem()
	if sel == nil || sel.ClusterName == "" {
		return nil
	}
	kctx := sel.ClusterName
	mode, _ := unionDashboardModeFromKind(m.nav.ResourceType.Kind)
	if mode == unionDashboardMonitoring {
		return m.loadMonitoringDashboardFor(kctx)
	}
	if !ui.ConfigDashboard {
		return func() tea.Msg {
			return dashboardLoadedMsg{content: "Cluster dashboard disabled", context: kctx}
		}
	}
	return m.loadDashboardFor(kctx)
}

func (m Model) navigateChildUnionDashboardMember(sel *model.Item) (tea.Model, tea.Cmd) {
	if sel == nil || sel.ClusterName == "" {
		return m, nil
	}
	logger.Info("Union dashboard member selected", "context", sel.ClusterName, "dashboard", sel.Extra)
	m.saveCursor()
	oldCtx := m.nav.Context
	m.nav.Context = sel.ClusterName
	m.invalidateOrphanCacheForContext(oldCtx)
	m.recomputeReadOnly(sel.ClusterName)
	m.nav.Level = model.LevelResourceTypes
	m.nav.ResourceType = model.ResourceTypeEntry{}
	m.nav.ResourceName = ""
	m.nav.OwnedName = ""
	m.nav.Namespace = ""
	m.dashboardPreview = ""
	m.dashboardEventsPreview = ""
	m.monitoringPreview = ""
	m.applyPinnedTypes()

	m.pushLeft()
	m.clearRight()
	switch {
	case len(m.discoveredResources[sel.ClusterName]) > 0:
		m.setMiddleItems(model.BuildSidebarItems(m.discoveredResources[sel.ClusterName]))
		m.itemCache[m.navKey()] = m.middleItems
		m.restoreCursor()
		m.syncExpandedGroup()
	default:
		m.setMiddleItems(nil)
		m.loading = true
	}
	m.setStatusMessage("Context: "+sel.ClusterName, false)
	m.saveCurrentSession()

	cmds := []tea.Cmd{m.loadPreview(), scheduleStatusClear()}
	if m.shouldFireDiscoveryFor(sel.ClusterName) {
		m.markDiscoveryStarted(sel.ClusterName)
		cmds = append(cmds, m.discoverAPIResources(sel.ClusterName))
	}
	if cmd := m.ensureNamespaceCacheFresh(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m Model) navigateParentToUnionDashboardMembers() (tea.Model, tea.Cmd) {
	m.saveCursor()
	members := append([]model.Item(nil), m.leftItems...)
	mode := unionDashboardCluster
	if len(members) > 0 {
		if parsed, ok := unionDashboardModeFromExtra(members[0].Extra); ok {
			mode = parsed
		}
	}
	oldCtx := m.nav.Context
	m.nav.Context = UnionContextSentinel
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = unionDashboardResourceType(mode)
	m.nav.ResourceName = ""
	m.nav.OwnedName = ""
	m.nav.Namespace = ""
	m.invalidateOrphanCacheForContext(oldCtx)
	m.readOnly = m.cliReadOnly
	m.applyPinnedTypes()
	m.dashboardPreview = ""
	m.dashboardEventsPreview = ""
	m.monitoringPreview = ""
	m.setMiddleItems(members)
	m.popLeft()
	m.clearRight()
	m.restoreCursor()
	m.restoreLevelFilter()
	m.syncExpandedGroup()
	m.saveCurrentSession()
	return m, m.loadPreview()
}
