package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) moveCursor(delta int) (tea.Model, tea.Cmd) {
	visible := m.visibleMiddleItems()
	c := m.cursor() + delta
	if c < 0 {
		c = 0
	}
	if c >= len(visible) {
		c = len(visible) - 1
	}
	if c < 0 {
		c = 0
	}
	m.setCursor(c)

	// Accordion behavior: auto-expand the group the cursor just entered.
	if m.nav.Level == model.LevelResourceTypes && !m.allGroupsExpanded {
		visible = m.visibleMiddleItems()
		if c >= 0 && c < len(visible) {
			newCat := visible[c].Category
			if newCat != "" && newCat != m.expandedGroup {
				m.expandedGroup = newCat
				// Recompute visible items after expansion change.
				newVisible := m.visibleMiddleItems()

				if delta < 0 {
					// Moving UP into a group: land on the LAST item of that group.
					for i := len(newVisible) - 1; i >= 0; i-- {
						if newVisible[i].Category == newCat && newVisible[i].Kind != "__collapsed_group__" {
							m.setCursor(i)
							break
						}
					}
				} else {
					// Moving DOWN into a group: land on the FIRST real item of that group.
					for i, item := range newVisible {
						if item.Category == newCat && item.Kind != "__collapsed_group__" {
							m.setCursor(i)
							break
						}
					}
				}
			}
		}
	}

	m.invalidatePreviewForCursorChange()
	// Reload resource map if active.
	if m.mapView {
		m.resourceTree = nil
		return m, tea.Batch(m.loadPreview(), m.loadResourceTree())
	}
	return m, m.loadPreview()
}

// invalidatePreviewForCursorChange resets the right-column state and bumps
// requestGen so any in-flight preview load triggered by the previous cursor
// position is discarded by its message handler instead of being applied to
// the wrong selection (which causes stale items to appear, followed by a
// brief "No resources found" flash before the new load returns).
func (m *Model) invalidatePreviewForCursorChange() {
	m.requestGen++
	m.rightItems = nil
	m.previewYAML = ""
	m.previewScroll = 0
	m.loading = true
	m.previewLoading = true
}

func (m Model) navigateParent() (tea.Model, tea.Cmd) {
	m.cancelAndReset()
	m.requestGen++
	m.clearSelection()
	m.activeFilterPreset = nil
	m.unfilteredMiddleItems = nil

	// Reset scroll positions when navigating to a new level.
	ui.ActiveMiddleScroll = 0
	ui.ActiveLeftScroll = 0
	switch m.nav.Level {
	case model.LevelClusters:
		return m, nil

	case model.LevelResourceTypes:
		m.saveCursor()
		m.nav.Level = model.LevelClusters
		m.nav.Context = ""
		m.middleItems = m.leftItems
		m.popLeft()
		m.clearRight()
		m.restoreCursor()
		return m, m.loadPreview()

	case model.LevelResources:
		m.saveCursor()
		m.nav.Level = model.LevelResourceTypes
		m.nav.ResourceType = model.ResourceTypeEntry{}
		m.middleItems = m.leftItems
		m.popLeft()
		m.clearRight()
		m.restoreCursor()
		m.syncExpandedGroup()
		return m, m.loadPreview()

	case model.LevelOwned:
		m.saveCursor()
		// If we drilled into a nested owned level (e.g., ArgoCD → Deployment),
		// pop back to the parent owned level instead of jumping to LevelResources.
		if n := len(m.ownedParentStack); n > 0 {
			parent := m.ownedParentStack[n-1]
			m.ownedParentStack = m.ownedParentStack[:n-1]
			m.nav.ResourceType = parent.resourceType
			m.nav.ResourceName = parent.resourceName
			m.nav.Namespace = parent.namespace
			// Stay at LevelOwned — we're returning to the parent's owned view.
			if cached, ok := m.itemCache[m.navKey()]; ok {
				m.middleItems = cached
			} else {
				m.middleItems = m.leftItems
			}
			m.popLeft()
			m.clearRight()
			m.restoreCursor()
			return m, m.loadPreview()
		}
		m.nav.Level = model.LevelResources
		m.nav.ResourceName = ""
		if cached, ok := m.itemCache[m.navKey()]; ok {
			m.middleItems = cached
		} else {
			m.middleItems = m.leftItems
		}
		m.popLeft()
		m.clearRight()
		m.restoreCursor()
		return m, m.loadPreview()

	case model.LevelContainers:
		m.saveCursor()
		// If we came directly from Pods (skipping LevelOwned), go back to LevelResources.
		if m.nav.ResourceType.Kind == "Pod" {
			m.nav.Level = model.LevelResources
			m.nav.ResourceName = ""
			m.nav.OwnedName = ""
		} else {
			m.nav.Level = model.LevelOwned
			m.nav.OwnedName = ""
		}
		if cached, ok := m.itemCache[m.navKey()]; ok {
			m.middleItems = cached
		} else {
			m.middleItems = m.leftItems
		}
		m.popLeft()
		m.clearRight()
		m.restoreCursor()
		return m, m.loadPreview()
	}
	return m, nil
}

func (m Model) navigateToOwner(kind, name string) (tea.Model, tea.Cmd) {
	crds := m.discoveredResources[m.nav.Context]
	rt, ok := model.FindResourceTypeByKind(kind, crds)
	if !ok {
		m.setStatusMessage(fmt.Sprintf("Unknown resource type: %s", kind), true)
		return m, scheduleStatusClear()
	}

	// Navigate back to resource types level.
	for m.nav.Level > model.LevelResourceTypes {
		ret, _ := m.navigateParent()
		m = ret.(Model)
	}

	// Find and select the target resource type in middle items.
	for i, item := range m.middleItems {
		if item.Extra == rt.ResourceRef() {
			m.setCursor(i)
			break
		}
	}

	// Set pending target to auto-select the owner resource after load.
	m.pendingTarget = name

	// Navigate into the resource type.
	return m.navigateChild()
}

func (m Model) navigateChild() (tea.Model, tea.Cmd) {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil
	}

	m.cancelAndReset()
	m.requestGen++
	m.clearSelection()

	// Reset scroll positions when navigating to a new level.
	ui.ActiveMiddleScroll = 0
	ui.ActiveLeftScroll = 0

	// Clear filter when navigating into a child.
	m.filterText = ""
	m.filterInput.Clear()
	m.filterActive = false
	m.activeFilterPreset = nil
	m.unfilteredMiddleItems = nil

	switch m.nav.Level {
	case model.LevelClusters:
		return m.navigateChildCluster(sel)
	case model.LevelResourceTypes:
		return m.navigateChildResourceType(sel)
	case model.LevelResources:
		return m.navigateChildResource(sel)
	case model.LevelOwned:
		return m.navigateChildOwned(sel)
	case model.LevelContainers:
		return m, nil
	}
	return m, nil
}

func (m Model) navigateChildCluster(sel *model.Item) (tea.Model, tea.Cmd) {
	logger.Info("Context selected", "context", sel.Name)
	m.saveCursor()
	m.nav.Context = sel.Name
	m.dashboardPreview = ""
	m.dashboardEventsPreview = ""
	m.monitoringPreview = ""
	// Re-register security sources against the newly selected context so
	// they use the right per-cluster clients. Safe to call even when the
	// manager has no sources yet.
	m.refreshSecuritySources()
	m.applyPinnedGroups()
	m.nav.Level = model.LevelResourceTypes
	m.pushLeft()
	m.clearRight()
	if discovered, ok := m.discoveredResources[sel.Name]; ok && len(discovered) > 0 {
		m.middleItems = model.BuildSidebarItems(discovered)
	} else {
		m.middleItems = model.BuildSidebarItems(model.SeedResources())
	}
	m.itemCache[m.navKey()] = m.middleItems
	m.restoreCursor()
	m.syncExpandedGroup()
	m.saveCurrentSession()
	cmds := []tea.Cmd{m.loadPreview()}
	if _, ok := m.discoveredResources[sel.Name]; !ok {
		cmds = append(cmds, m.discoverAPIResources(sel.Name))
	}
	if cmd := m.loadSecurityAvailability(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m Model) navigateChildResourceType(sel *model.Item) (tea.Model, tea.Cmd) {
	if sel.Extra == "__overview__" || sel.Extra == "__monitoring__" {
		m.fullscreenDashboard = true
		m.previewScroll = 0
		m.setStatusMessage("Dashboard fullscreen ON", false)
		return m, scheduleStatusClear()
	}
	if sel.Kind == "__port_forwards__" {
		m.saveCursor()
		m.nav.ResourceType = model.ResourceTypeEntry{
			DisplayName: "Port Forwards",
			Kind:        "__port_forwards__",
			APIGroup:    "_portforward",
			APIVersion:  "v1",
			Resource:    "portforwards",
			Namespaced:  false,
		}
		m.nav.Level = model.LevelResources
		m.pushLeft()
		m.clearRight()
		m.middleItems = m.portForwardItems()
		m.setCursor(0)
		m.clampCursor()
		m.saveCurrentSession()
		return m, m.waitForPortForwardUpdate()
	}
	if sel.Kind == "__collapsed_group__" {
		m.expandedGroup = sel.Category
		visible := m.visibleMiddleItems()
		for i, item := range visible {
			if item.Category == sel.Category && item.Kind != "__collapsed_group__" {
				m.setCursor(i)
				break
			}
		}
		m.rightItems = nil
		m.previewYAML = ""
		m.loading = true
		return m, m.loadPreview()
	}
	rt, ok := model.FindResourceTypeIn(sel.Extra, m.discoveredResources[m.nav.Context])
	if !ok {
		return m, nil
	}
	m.saveCursor()
	m.nav.ResourceType = rt
	m.nav.Level = model.LevelResources
	m.pushLeft()
	m.clearRight()
	m.saveCurrentSession()
	if cached, ok := m.itemCache[m.navKey()]; ok {
		m.middleItems = cached
		m.restoreCursor()
	} else {
		m.middleItems = nil
		m.setCursor(0)
	}
	m.loading = true
	return m, m.loadResources(false)
}

func (m Model) navigateChildResource(sel *model.Item) (tea.Model, tea.Cmd) {
	if !m.resourceTypeHasChildren() && m.nav.ResourceType.Kind != "Pod" {
		return m, nil
	}
	m.saveCursor()
	m.nav.ResourceName = sel.Name
	if sel.Namespace != "" {
		m.nav.Namespace = sel.Namespace
	} else if !m.allNamespaces {
		m.nav.Namespace = m.namespace
	}
	m.saveCurrentSession()
	if m.nav.ResourceType.Kind == "Pod" {
		m.nav.OwnedName = sel.Name
		m.nav.Level = model.LevelContainers
		m.pushLeft()
		m.clearRight()
		if cached, ok := m.itemCache[m.navKey()]; ok {
			m.middleItems = cached
			m.restoreCursor()
		} else {
			m.middleItems = nil
			m.setCursor(0)
		}
		m.loading = true
		return m, m.loadContainers(false)
	}
	m.nav.Level = model.LevelOwned
	m.pushLeft()
	m.clearRight()
	if cached, ok := m.itemCache[m.navKey()]; ok {
		m.middleItems = cached
		m.restoreCursor()
	} else {
		m.middleItems = nil
		m.setCursor(0)
	}
	m.loading = true
	return m, m.loadOwned(false)
}

func (m Model) navigateChildOwned(sel *model.Item) (tea.Model, tea.Cmd) {
	if sel.Kind == "Pod" {
		m.saveCursor()
		m.nav.OwnedName = sel.Name
		if sel.Namespace != "" {
			m.nav.Namespace = sel.Namespace
		}
		m.nav.Level = model.LevelContainers
		m.pushLeft()
		m.clearRight()
		if cached, ok := m.itemCache[m.navKey()]; ok {
			m.middleItems = cached
			m.restoreCursor()
		} else {
			m.middleItems = nil
			m.setCursor(0)
		}
		m.loading = true
		return m, m.loadContainers(false)
	}
	if kindHasOwnedChildren(sel.Kind) {
		m.saveCursor()
		m.ownedParentStack = append(m.ownedParentStack, ownedParentState{
			resourceType: m.nav.ResourceType,
			resourceName: m.nav.ResourceName,
			namespace:    m.nav.Namespace,
		})
		m.nav.ResourceType.Kind = sel.Kind
		m.nav.ResourceName = sel.Name
		if sel.Namespace != "" {
			m.nav.Namespace = sel.Namespace
		}
		m.pushLeft()
		m.clearRight()
		if cached, ok := m.itemCache[m.navKey()]; ok {
			m.middleItems = cached
			m.restoreCursor()
		} else {
			m.middleItems = nil
			m.setCursor(0)
		}
		m.loading = true
		return m, m.loadOwned(false)
	}
	return m, nil
}

func (m Model) enterFullView() (tea.Model, tea.Cmd) {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil
	}

	// Security findings jump to the affected resource rather than open YAML.
	if sel.Kind == "__security_finding__" {
		return m.jumpToFindingResource(*sel)
	}

	if m.nav.Level == model.LevelClusters || m.nav.Level == model.LevelResourceTypes {
		return m.navigateChild()
	}

	// Port forward entries are virtual — no YAML to display.
	if m.nav.ResourceType.Kind == "__port_forwards__" {
		return m, nil
	}

	m.mode = modeYAML
	m.yamlScroll = 0
	m.yamlContent = "Loading..."
	m.yamlSections = nil
	m.yamlVisualCurCol = yamlFoldPrefixLen
	return m, m.loadYAML()
}

// itemIndexFromDisplayLine maps a display line number to the actual item index,
// accounting for category headers and separators in the middle column.
func (m *Model) itemIndexFromDisplayLine(displayLine int) int {
	visible := m.visibleMiddleItems()
	line := 0
	lastCategory := ""
	for i, item := range visible {
		if item.Category != "" && item.Category != lastCategory {
			lastCategory = item.Category
			if i > 0 {
				line++ // separator
			}
			line++ // category header
		}
		if line == displayLine {
			return i
		}
		line++
	}
	return -1
}
