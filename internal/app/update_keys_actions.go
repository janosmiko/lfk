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
	kb := ui.ActiveKeybindings
	switch msg.String() {
	case kb.AllNamespaces:
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

	case kb.QuotaDashboard:
		m.loading = true
		m.setStatusMessage("Loading quota data...", false)
		return m, m.loadQuotas(), true

	case kb.PageDown:
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
		m.rightItems = nil
		m.previewYAML = ""
		m.previewScroll = 0
		m.loading = true
		return m, m.loadPreview(), true

	case kb.PageUp:
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
		m.rightItems = nil
		m.previewYAML = ""
		m.previewScroll = 0
		m.loading = true
		return m, m.loadPreview(), true

	case kb.PageForward:
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
		m.rightItems = nil
		m.previewYAML = ""
		m.previewScroll = 0
		m.loading = true
		return m, m.loadPreview(), true

	case kb.PageBack:
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
		m.rightItems = nil
		m.previewYAML = ""
		m.previewScroll = 0
		m.loading = true
		return m, m.loadPreview(), true

	case kb.LevelCluster:
		// Jump to clusters level.
		for m.nav.Level > model.LevelClusters {
			ret, _ := m.navigateParent()
			m = ret.(Model)
		}
		return m, m.loadPreview(), true

	case kb.LevelTypes:
		// Jump to resource types level.
		if m.nav.Level < model.LevelResourceTypes {
			return m, nil, true // can't jump forward
		}
		for m.nav.Level > model.LevelResourceTypes {
			ret, _ := m.navigateParent()
			m = ret.(Model)
		}
		return m, m.loadPreview(), true

	case kb.LevelResources:
		// Jump to resources level.
		if m.nav.Level < model.LevelResources {
			return m, nil, true
		}
		for m.nav.Level > model.LevelResources {
			ret, _ := m.navigateParent()
			m = ret.(Model)
		}
		return m, m.loadPreview(), true

	case kb.SaveResource:
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

	case kb.PreviewDown:
		// Scroll preview pane down, clamped to content length.
		m.previewScroll++
		m.clampPreviewScroll()
		return m, nil, true

	case kb.PreviewUp:
		// Scroll preview pane up.
		if m.previewScroll > 0 {
			m.previewScroll--
		}
		return m, nil, true

	case kb.JumpOwner:
		// Navigate to owner/controller.
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

	case kb.SortNext:
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

	case kb.SortPrev:
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

	case kb.SortFlip:
		m.sortAscending = !m.sortAscending
		m.sortMiddleItems()
		m.clampCursor()
		m.setStatusMessage("Sort: "+m.sortModeName(), false)
		return m, tea.Batch(m.loadPreview(), scheduleStatusClear()), true

	case kb.SortReset:
		m.sortColumnName = sortColDefault
		m.sortAscending = true
		m.sortMiddleItems()
		m.clampCursor()
		m.setStatusMessage("Sort: "+m.sortModeName(), false)
		return m, tea.Batch(m.loadPreview(), scheduleStatusClear()), true

	case kb.OpenBrowser:
		// Open ingress host in browser (works when viewing an Ingress resource).
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

	case kb.CopyName:
		// Copy resource name to clipboard.
		sel := m.selectedMiddleItem()
		if sel != nil {
			m.setStatusMessage("Copied: "+sel.Name, false)
			return m, tea.Batch(copyToSystemClipboard(sel.Name), scheduleStatusClear()), true
		}
		return m, nil, true

	case kb.CopyYAML:
		// Copy resource YAML to clipboard.
		sel := m.selectedMiddleItem()
		if sel != nil {
			return m, m.copyYAMLToClipboard(), true
		}
		return m, nil, true

	case kb.PasteApply:
		return m, m.applyFromClipboard(), true

	case kb.NewTab:
		// Create new tab (clone current state, max 9).
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

	case kb.NextTab:
		// Next tab.
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

	case kb.PrevTab:
		// Previous tab.
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

	case kb.CreateTemplate:
		// Open template creation overlay.
		// Sort templates so the one matching the current resource kind appears first.
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

	case kb.SecretEditor:
		// Open secret editor when a Secret resource is selected.
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

	case kb.APIExplorer:
		// Open API explain browser (resource structure).
		ret, cmd := m.openExplainBrowser()
		return ret, cmd, true

	case kb.RBACBrowser:
		// Open RBAC permissions browser (can-i).
		ret, cmd := m.openCanIBrowser()
		return ret, cmd, true

	case kb.LabelEditor:
		// Open label/annotation editor for any resource (not port forwards).
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

	case kb.FilterPresets:
		// Quick filter presets: toggle or open overlay.
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

	case kb.Diff:
		// Diff two selected resources side by side.
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

	case kb.ErrorLog:
		// Open the error log overlay.
		m.overlayErrorLog = true
		m.errorLogScroll = 0
		return m, nil, true

	case kb.TerminalToggle:
		// Toggle between PTY (embedded) and exec (takes over terminal) mode.
		if ui.ConfigTerminalMode == "pty" {
			ui.ConfigTerminalMode = "exec"
			m.setStatusMessage("Terminal mode: exec (takes over terminal)", false)
		} else {
			ui.ConfigTerminalMode = "pty"
			m.setStatusMessage("Terminal mode: pty (embedded)", false)
		}
		return m, scheduleStatusClear(), true

	case kb.Monitoring:
		// Navigate to the Monitoring dashboard item.
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

	// Configurable direct action keybindings.
	key := msg.String()
	if key == kb.Logs {
		ret, cmd := m.directActionLogs()
		return ret, cmd, true
	}
	if key == kb.Refresh {
		ret, cmd := m.directActionRefresh()
		return ret, cmd, true
	}
	// Restart (r) and Exec (s) are only accessible via the action menu (x).
	if key == kb.Edit {
		ret, cmd := m.directActionEdit()
		return ret, cmd, true
	}
	if key == kb.Describe {
		ret, cmd := m.directActionDescribe()
		return ret, cmd, true
	}
	if key == kb.Delete {
		ret, cmd := m.directActionDelete()
		return ret, cmd, true
	}
	if key == kb.ForceDelete {
		ret, cmd := m.directActionForceDelete()
		return ret, cmd, true
	}
	if key == kb.Scale {
		ret, cmd := m.directActionScale()
		return ret, cmd, true
	}

	return m, nil, false
}
