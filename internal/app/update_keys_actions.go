package app

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// handleExplorerActionKey handles key bindings for explorer-mode actions
// such as namespace toggling, scrolling/paging, tab management, editors,
// resource actions, and configurable direct-action keybindings.
// Returns (model, cmd, handled) where handled indicates whether the key
// was consumed. If not handled, the caller should fall through.
func (m Model) handleExplorerActionKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	// Try navigation/scroll keybindings first.
	if ret, cmd, handled := m.handleExplorerNavKeys(msg); handled {
		return ret, cmd, true
	}
	// Try tool/editor keybindings.
	if ret, cmd, handled := m.handleExplorerToolKeys(msg); handled {
		return ret, cmd, true
	}
	// Try direct action keybindings.
	return m.handleExplorerDirectActionKeys(msg)
}

// handleExplorerNavKeys handles navigation and scroll keybindings.
func (m Model) handleExplorerNavKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	kb := ui.ActiveKeybindings
	switch msg.String() {
	case kb.AllNamespaces:
		return m.handleExplorerActionKeyAllNamespaces()
	case kb.QuotaDashboard:
		return m.handleExplorerActionKeyQuotaDashboard()
	case kb.PageDown:
		return m.handleExplorerActionKeyPageDown()
	case kb.PageUp:
		return m.handleExplorerActionKeyPageUp()
	case kb.PageForward:
		return m.handleExplorerActionKeyPageForward()
	case kb.PageBack:
		return m.handleExplorerActionKeyPageBack()
	case kb.LevelCluster:
		return m.handleExplorerActionKeyLevelCluster()
	case kb.LevelTypes:
		return m.handleExplorerActionKeyLevelTypes()
	case kb.LevelResources:
		return m.handleExplorerActionKeyLevelResources()
	case kb.SaveResource:
		return m.handleExplorerActionKeySaveResource()
	case kb.PreviewDown:
		return m.handleExplorerActionKeyPreviewDown()
	case kb.PreviewUp:
		return m.handleExplorerActionKeyPreviewUp()
	case kb.JumpOwner:
		return m.handleExplorerActionKeyJumpOwner()
	case kb.SortNext:
		return m.handleExplorerActionKeySortNext()
	case kb.SortPrev:
		return m.handleExplorerActionKeySortPrev()
	case kb.SortFlip:
		return m.handleExplorerActionKeySortFlip()
	case kb.SortReset:
		return m.handleExplorerActionKeySortReset()
	case kb.Monitoring:
		return m.handleExplorerActionKeyMonitoring()
	}
	return m, nil, false
}

// handleExplorerToolKeys handles tool/editor keybindings.
func (m Model) handleExplorerToolKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	kb := ui.ActiveKeybindings
	switch msg.String() {
	case kb.OpenBrowser:
		return m.handleExplorerActionKeyOpenBrowser()
	case kb.CopyName:
		return m.handleExplorerActionKeyCopyName()
	case kb.CopyYAML:
		return m.handleExplorerActionKeyCopyYAML()
	case kb.PasteApply:
		return m, m.applyFromClipboard(), true
	case kb.NewTab:
		return m.handleExplorerActionKeyNewTab()
	case kb.NextTab:
		return m.handleExplorerActionKeyNextTab()
	case kb.PrevTab:
		return m.handleExplorerActionKeyPrevTab()
	case kb.CreateTemplate:
		return m.handleExplorerActionKeyCreateTemplate()
	case kb.SecretEditor:
		return m.handleExplorerActionKeySecretEditor()
	case kb.APIExplorer:
		ret, cmd := m.openExplainBrowser()
		return ret, cmd, true
	case kb.RBACBrowser:
		ret, cmd := m.openCanIBrowser()
		return ret, cmd, true
	case kb.LabelEditor:
		return m.handleExplorerActionKeyLabelEditor()
	case kb.FilterPresets:
		return m.handleExplorerActionKeyFilterPresets()
	case kb.Diff:
		return m.handleExplorerActionKeyDiff()
	case kb.ErrorLog:
		return m.handleExplorerActionKeyErrorLog()
	case kb.TerminalToggle:
		return m.handleExplorerActionKeyTerminalToggle()
	case kb.ToggleRare:
		return m.handleExplorerActionKeyToggleRare()
	}
	return m, nil, false
}

// handleExplorerActionKeyToggleRare toggles the global ShowRareResources
// flag. When enabled, the sidebar surfaces curated entries marked Rare
// (CSI internals, admission webhooks, etc.) plus uncategorized core
// Kubernetes resources under the synthetic "Advanced" category. The state
// is not persisted and resets on each launch.
func (m Model) handleExplorerActionKeyToggleRare() (tea.Model, tea.Cmd, bool) {
	m.showRareResources = !m.showRareResources
	model.ShowRareResources = m.showRareResources

	// Rebuild the resource types list from the current context's discovered
	// set (falling back to the seed list when discovery hasn't completed).
	var merged []model.Item
	if discovered, ok := m.discoveredResources[m.nav.Context]; ok && len(discovered) > 0 {
		merged = model.BuildSidebarItems(discovered)
	} else {
		merged = model.BuildSidebarItems(model.SeedResources())
	}
	// Refresh the resource-types cache so cached reads stay in sync.
	m.itemCache[m.nav.Context] = merged

	// Apply the rebuild to whichever column currently shows the resource
	// types list, depending on the user's navigation level.
	switch m.nav.Level {
	case model.LevelResourceTypes:
		// User is on resource types level: update middleItems and preserve
		// cursor identity so they don't lose their place when entries
		// appear or disappear.
		prevName, prevNs, prevExtra, prevKind := m.cursorItemKey()
		m.middleItems = merged
		m.restoreCursorToItem(prevName, prevNs, prevExtra, prevKind)
	default:
		// User is deeper (LevelResources / LevelOwned / LevelContainers):
		// the resource types list is in leftItems. Refresh it so the
		// parent column reflects the new visibility immediately and
		// keep the resource-types cursor memory pointing at the current
		// resource type.
		m.leftItems = merged
		if m.nav.ResourceType.Resource != "" {
			rtRef := m.nav.ResourceType.ResourceRef()
			for i, item := range merged {
				if item.Extra == rtRef {
					m.cursorMemory[m.nav.Context] = i
					break
				}
			}
		}
	}

	if m.showRareResources {
		m.setStatusMessage("Rarely used resource types: ON", false)
	} else {
		m.setStatusMessage("Rarely used resource types: OFF", false)
	}
	return m, scheduleStatusClear(), true
}

// handleExplorerDirectActionKeys handles configurable direct action keybindings.
func (m Model) handleExplorerDirectActionKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	kb := ui.ActiveKeybindings
	key := msg.String()
	switch key {
	case kb.Logs:
		ret, cmd := m.directActionLogs()
		return ret, cmd, true
	case kb.Refresh:
		ret, cmd := m.directActionRefresh()
		return ret, cmd, true
	case kb.Edit:
		ret, cmd := m.directActionEdit()
		return ret, cmd, true
	case kb.Describe:
		ret, cmd := m.directActionDescribe()
		return ret, cmd, true
	case kb.Delete:
		ret, cmd := m.directActionDelete()
		return ret, cmd, true
	case kb.ForceDelete:
		ret, cmd := m.directActionForceDelete()
		return ret, cmd, true
	case kb.Scale:
		ret, cmd := m.directActionScale()
		return ret, cmd, true
	}
	return m, nil, false
}

func (m Model) handleExplorerActionKeyAllNamespaces() (tea.Model, tea.Cmd, bool) {
	m.allNamespaces = !m.allNamespaces
	if m.allNamespaces {
		m.selectedNamespaces = nil
		m.setStatusMessage("All namespaces mode ON", false)
	} else {
		m.setStatusMessage("All namespaces mode OFF (ns: "+m.namespace+")", false)
	}
	m.cancelAndReset()
	m.requestGen++
	return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear()), true
}

func (m Model) handleExplorerActionKeyQuotaDashboard() (tea.Model, tea.Cmd, bool) {
	m.loading = true
	m.setStatusMessage("Loading quota data...", false)
	return m, m.loadQuotas(), true
}

func (m Model) handleExplorerActionKeyPageDown() (tea.Model, tea.Cmd, bool) {
	if m.fullscreenDashboard {
		halfPage := (m.height - 4) / 2
		m.previewScroll += halfPage
		m.clampPreviewScroll()
		return m, nil, true
	}
	visible := m.visibleMiddleItems()
	halfPage := (m.height - 4) / 2
	c := m.cursor() + halfPage
	if c >= len(visible) {
		c = len(visible) - 1
	}
	if c < 0 {
		c = 0
	}
	m.setCursor(c)
	m.syncExpandedGroup()
	m.invalidatePreviewForCursorChange()
	return m, m.loadPreview(), true
}

func (m Model) handleExplorerActionKeyPageUp() (tea.Model, tea.Cmd, bool) {
	if m.fullscreenDashboard {
		halfPage := (m.height - 4) / 2
		m.previewScroll -= halfPage
		if m.previewScroll < 0 {
			m.previewScroll = 0
		}
		return m, nil, true
	}
	visible := m.visibleMiddleItems()
	halfPage := (m.height - 4) / 2
	c := m.cursor() - halfPage
	if c < 0 {
		c = 0
	}
	if c >= len(visible) {
		c = len(visible) - 1
	}
	m.setCursor(c)
	m.syncExpandedGroup()
	m.invalidatePreviewForCursorChange()
	return m, m.loadPreview(), true
}

func (m Model) handleExplorerActionKeyPageForward() (tea.Model, tea.Cmd, bool) {
	if m.fullscreenDashboard {
		fullPage := m.height - 4
		m.previewScroll += fullPage
		m.clampPreviewScroll()
		return m, nil, true
	}
	visible := m.visibleMiddleItems()
	fullPage := m.height - 4
	c := m.cursor() + fullPage
	if c >= len(visible) {
		c = len(visible) - 1
	}
	if c < 0 {
		c = 0
	}
	m.setCursor(c)
	m.syncExpandedGroup()
	m.invalidatePreviewForCursorChange()
	return m, m.loadPreview(), true
}

func (m Model) handleExplorerActionKeyPageBack() (tea.Model, tea.Cmd, bool) {
	if m.fullscreenDashboard {
		fullPage := m.height - 4
		m.previewScroll -= fullPage
		if m.previewScroll < 0 {
			m.previewScroll = 0
		}
		return m, nil, true
	}
	visible := m.visibleMiddleItems()
	fullPage := m.height - 4
	c := m.cursor() - fullPage
	if c < 0 {
		c = 0
	}
	if c >= len(visible) {
		c = len(visible) - 1
	}
	m.setCursor(c)
	m.syncExpandedGroup()
	m.invalidatePreviewForCursorChange()
	return m, m.loadPreview(), true
}

func (m Model) handleExplorerActionKeyLevelCluster() (tea.Model, tea.Cmd, bool) {
	for m.nav.Level > model.LevelClusters {
		ret, _ := m.navigateParent()
		m = ret.(Model)
	}
	return m, m.loadPreview(), true
}

func (m Model) handleExplorerActionKeyLevelTypes() (tea.Model, tea.Cmd, bool) {
	if m.nav.Level < model.LevelResourceTypes {
		return m, nil, true // can't jump forward
	}
	for m.nav.Level > model.LevelResourceTypes {
		ret, _ := m.navigateParent()
		m = ret.(Model)
	}
	return m, m.loadPreview(), true
}

func (m Model) handleExplorerActionKeyLevelResources() (tea.Model, tea.Cmd, bool) {
	if m.nav.Level < model.LevelResources {
		return m, nil, true
	}
	for m.nav.Level > model.LevelResources {
		ret, _ := m.navigateParent()
		m = ret.(Model)
	}
	return m, m.loadPreview(), true
}

func (m Model) handleExplorerActionKeySaveResource() (tea.Model, tea.Cmd, bool) {
	if m.nav.Level == model.LevelResources && m.nav.ResourceType.Kind == "Event" {
		m.warningEventsOnly = !m.warningEventsOnly
		// Re-filter the current items.
		if cached, ok := m.itemCache[m.navKey()]; ok {
			if m.warningEventsOnly {
				var filtered []model.Item
				for _, item := range cached {
					if item.Status == "Warning" {
						filtered = append(filtered, item)
					}
				}
				m.middleItems = filtered
			} else {
				m.middleItems = cached
			}
			m.clampCursor()
		}
		if m.warningEventsOnly {
			m.setStatusMessage("Showing warnings only", false)
		} else {
			m.setStatusMessage("Showing all events", false)
		}
		return m, scheduleStatusClear(), true
	}
	// Save resource YAML to file (same as Scale key).
	if m.nav.Level == model.LevelResources || m.nav.Level == model.LevelOwned || m.nav.Level == model.LevelContainers {
		sel := m.selectedMiddleItem()
		if sel != nil {
			m.setStatusMessage("Exporting...", false)
			return m, m.exportResourceToFile(), true
		}
	}
	return m, nil, true
}

func (m Model) handleExplorerActionKeyPreviewDown() (tea.Model, tea.Cmd, bool) {
	m.previewScroll++
	m.clampPreviewScroll()
	return m, nil, true
}

func (m Model) handleExplorerActionKeyPreviewUp() (tea.Model, tea.Cmd, bool) {
	if m.previewScroll > 0 {
		m.previewScroll--
	}
	return m, nil, true
}

func (m Model) handleExplorerActionKeyJumpOwner() (tea.Model, tea.Cmd, bool) {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil, true
	}
	type ownerRef struct {
		kind, name, apiVersion string
	}
	var owners []ownerRef
	for _, kv := range sel.Columns {
		if strings.HasPrefix(kv.Key, "owner:") {
			parts := strings.SplitN(kv.Value, "||", 3)
			if len(parts) == 3 {
				owners = append(owners, ownerRef{
					apiVersion: parts[0],
					kind:       parts[1],
					name:       parts[2],
				})
			}
		}
	}
	if len(owners) == 0 {
		m.setStatusMessage("No owner references found", true)
		return m, scheduleStatusClear(), true
	}
	// Use first owner (most resources have exactly one).
	owner := owners[0]
	ret, cmd := m.navigateToOwner(owner.kind, owner.name)
	return ret, cmd, true
}

func (m Model) handleExplorerActionKeySortNext() (tea.Model, tea.Cmd, bool) {
	colCount := ui.ActiveSortableColumnCount
	if colCount > 0 {
		idx := sortColumnIndex(m.sortColumnName)
		idx = (idx + 1) % colCount
		m.sortColumnName = ui.ActiveSortableColumns[idx]
	}
	m.sortMiddleItems()
	m.clampCursor()
	m.setStatusMessage("Sort: "+m.sortModeName(), false)
	return m, tea.Batch(m.loadPreview(), scheduleStatusClear()), true
}

func (m Model) handleExplorerActionKeySortPrev() (tea.Model, tea.Cmd, bool) {
	colCount := ui.ActiveSortableColumnCount
	if colCount > 0 {
		idx := sortColumnIndex(m.sortColumnName)
		idx = (idx - 1 + colCount) % colCount
		m.sortColumnName = ui.ActiveSortableColumns[idx]
	}
	m.sortMiddleItems()
	m.clampCursor()
	m.setStatusMessage("Sort: "+m.sortModeName(), false)
	return m, tea.Batch(m.loadPreview(), scheduleStatusClear()), true
}

func (m Model) handleExplorerActionKeySortFlip() (tea.Model, tea.Cmd, bool) {
	m.sortAscending = !m.sortAscending
	m.sortMiddleItems()
	m.clampCursor()
	m.setStatusMessage("Sort: "+m.sortModeName(), false)
	return m, tea.Batch(m.loadPreview(), scheduleStatusClear()), true
}

func (m Model) handleExplorerActionKeySortReset() (tea.Model, tea.Cmd, bool) {
	m.sortColumnName = sortColDefault
	m.sortAscending = true
	m.sortMiddleItems()
	m.clampCursor()
	m.setStatusMessage("Sort: "+m.sortModeName(), false)
	return m, tea.Batch(m.loadPreview(), scheduleStatusClear()), true
}

func (m Model) handleExplorerActionKeyOpenBrowser() (tea.Model, tea.Cmd, bool) {
	kind := m.selectedResourceKind()
	if kind != "Ingress" {
		m.setStatusMessage("Open in browser is only available for Ingress resources", true)
		return m, scheduleStatusClear(), true
	}
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil, true
	}
	m.actionCtx = m.buildActionCtx(sel, kind)
	ret, cmd := m.executeAction("Open in Browser")
	return ret, cmd, true
}

func (m Model) handleExplorerActionKeyCopyName() (tea.Model, tea.Cmd, bool) {
	sel := m.selectedMiddleItem()
	if sel != nil {
		m.setStatusMessage("Copied: "+sel.Name, false)
		return m, tea.Batch(copyToSystemClipboard(sel.Name), scheduleStatusClear()), true
	}
	return m, nil, true
}

func (m Model) handleExplorerActionKeyCopyYAML() (tea.Model, tea.Cmd, bool) {
	sel := m.selectedMiddleItem()
	if sel != nil {
		return m, m.copyYAMLToClipboard(), true
	}
	return m, nil, true
}

func (m Model) handleExplorerActionKeyNewTab() (tea.Model, tea.Cmd, bool) {
	if len(m.tabs) >= 9 {
		m.setStatusMessage("Max 9 tabs", true)
		return m, scheduleStatusClear(), true
	}
	m.saveCurrentTab()
	insertAt := m.activeTab + 1
	newTab := m.cloneCurrentTab()
	m.tabs = append(m.tabs[:insertAt], append([]TabState{newTab}, m.tabs[insertAt:]...)...)
	m.activeTab = insertAt
	m.setStatusMessage(fmt.Sprintf("Tab %d created", m.activeTab+1), false)
	return m, scheduleStatusClear(), true
}

func (m Model) handleExplorerActionKeyNextTab() (tea.Model, tea.Cmd, bool) {
	if len(m.tabs) <= 1 {
		return m, nil, true
	}
	m.saveCurrentTab()
	next := (m.activeTab + 1) % len(m.tabs)
	if cmd := m.loadTab(next); cmd != nil {
		return m, cmd, true
	}
	if m.mode == modeExec && m.execPTY != nil {
		return m, m.scheduleExecTick(), true
	}
	return m, m.loadPreview(), true
}

func (m Model) handleExplorerActionKeyPrevTab() (tea.Model, tea.Cmd, bool) {
	if len(m.tabs) <= 1 {
		return m, nil, true
	}
	m.saveCurrentTab()
	prev := (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
	if cmd := m.loadTab(prev); cmd != nil {
		return m, cmd, true
	}
	if m.mode == modeExec && m.execPTY != nil {
		return m, m.scheduleExecTick(), true
	}
	return m, m.loadPreview(), true
}

func (m Model) handleExplorerActionKeyCreateTemplate() (tea.Model, tea.Cmd, bool) {
	templates := model.BuiltinTemplates()
	currentKind := m.nav.ResourceType.Kind
	if currentKind != "" {
		sort.SliceStable(templates, func(i, j int) bool {
			iMatch := strings.EqualFold(templates[i].Name, currentKind)
			jMatch := strings.EqualFold(templates[j].Name, currentKind)
			return iMatch && !jMatch
		})
	}
	m.templateItems = templates
	m.templateCursor = 0
	m.templateFilter.Clear()
	m.templateSearchMode = false
	m.overlay = overlayTemplates
	return m, nil, true
}

func (m Model) handleExplorerActionKeySecretEditor() (tea.Model, tea.Cmd, bool) {
	if m.nav.Level == model.LevelResources && m.nav.ResourceType.Kind == "Secret" {
		sel := m.selectedMiddleItem()
		if sel != nil {
			return m, m.loadSecretData(), true
		}
	}
	// Open configmap editor when a ConfigMap resource is selected.
	if m.nav.Level == model.LevelResources && m.nav.ResourceType.Kind == "ConfigMap" {
		sel := m.selectedMiddleItem()
		if sel != nil {
			return m, m.loadConfigMapData(), true
		}
	}
	return m, nil, true
}

func (m Model) handleExplorerActionKeyLabelEditor() (tea.Model, tea.Cmd, bool) {
	if m.nav.Level == model.LevelResources && m.nav.ResourceType.Kind != "__port_forwards__" {
		sel := m.selectedMiddleItem()
		if sel != nil {
			m.labelResourceType = m.nav.ResourceType
			return m, m.loadLabelData(), true
		}
	} else if m.nav.Level == model.LevelOwned {
		sel := m.selectedMiddleItem()
		if sel != nil {
			rt, ok := m.resolveOwnedResourceType(sel)
			if ok {
				m.labelResourceType = rt
				return m, m.loadLabelData(), true
			}
		}
	}
	return m, nil, true
}

func (m Model) handleExplorerActionKeyFilterPresets() (tea.Model, tea.Cmd, bool) {
	if m.nav.Level < model.LevelResources {
		m.setStatusMessage("Quick filters are only available at resource level", true)
		return m, scheduleStatusClear(), true
	}
	if m.activeFilterPreset != nil {
		// Clear the active filter preset and restore the full list.
		name := m.activeFilterPreset.Name
		m.activeFilterPreset = nil
		m.middleItems = m.unfilteredMiddleItems
		m.unfilteredMiddleItems = nil
		m.setCursor(0)
		m.clampCursor()
		m.setStatusMessage("Filter cleared: "+name, false)
		return m, tea.Batch(scheduleStatusClear(), m.loadPreview()), true
	}
	// Open the filter preset overlay.
	m.filterPresets = builtinFilterPresets(m.nav.ResourceType.Kind)
	m.overlayCursor = 0
	m.overlay = overlayFilterPreset
	return m, nil, true
}

func (m Model) handleExplorerActionKeyDiff() (tea.Model, tea.Cmd, bool) {
	if m.nav.Level < model.LevelResources {
		m.setStatusMessage("Diff is only available at resource level", true)
		return m, scheduleStatusClear(), true
	}
	selected := m.selectedItemsList()
	if len(selected) != 2 {
		m.setStatusMessage("Select exactly 2 resources to diff (use Space to select)", true)
		return m, scheduleStatusClear(), true
	}
	m.loading = true
	m.setStatusMessage("Loading diff...", false)
	return m, m.loadDiff(m.nav.ResourceType, selected[0], selected[1]), true
}

func (m Model) handleExplorerActionKeyErrorLog() (tea.Model, tea.Cmd, bool) {
	m.overlayErrorLog = true
	m.errorLogScroll = 0
	return m, nil, true
}

func (m Model) handleExplorerActionKeyTerminalToggle() (tea.Model, tea.Cmd, bool) {
	if ui.ConfigTerminalMode == "pty" {
		ui.ConfigTerminalMode = "exec"
		m.setStatusMessage("Terminal mode: exec (takes over terminal)", false)
	} else {
		ui.ConfigTerminalMode = "pty"
		m.setStatusMessage("Terminal mode: pty (embedded)", false)
	}
	return m, scheduleStatusClear(), true
}

func (m Model) handleExplorerActionKeyMonitoring() (tea.Model, tea.Cmd, bool) {
	if m.nav.Level < model.LevelResourceTypes {
		m.setStatusMessage("Select a cluster first", true)
		return m, scheduleStatusClear(), true
	}
	// Find the Monitoring item in the middle column and select it.
	for i, item := range m.middleItems {
		if item.Extra == "__monitoring__" {
			m.setCursor(i)
			m.clampCursor()
			return m, m.loadPreview(), true
		}
	}
	return m, nil, true
}
