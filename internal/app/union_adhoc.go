package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// adHocUnionPending is the pendingUnionSetName value while the namespace
// overlay picks a namespace for marked picker contexts. It reuses the named-set
// gates that reject all/Space/Tab/A/".".
const adHocUnionPending = "\x00adhoc"

func (m Model) markedContexts() []string {
	var out []string
	for _, it := range m.middleItems {
		if it.IsContext && m.isSelected(it) {
			out = append(out, it.Name)
		}
	}
	return out
}

func (m Model) isAdHocUnion() bool {
	return m.unionMode && m.unionSetName == "" && m.unionStartedFromPicker
}

func unionStatusLabel(setName string, contexts []string) string {
	if setName != "" {
		return "Union set: " + setName
	}
	return "Union: " + strings.Join(contexts, ", ")
}

func (m Model) namespaceOverlayTitle() string {
	if m.pendingUnionSetName != adHocUnionPending {
		return "Select Namespace"
	}
	title := "Union (read-only): " + strings.Join(m.pendingUnionContexts, ", ") + " - pick namespace"
	return ui.Truncate(title, min(60, m.width-10)-6)
}

// handleClusterMarkKey marks or unmarks the highlighted picker context for an
// ad-hoc union.
func (m Model) handleClusterMarkKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	if m.nav.Level != model.LevelClusters || msg.String() != ui.ActiveKeybindings.ToggleSelect {
		return m, nil, false
	}
	sel := m.selectedMiddleItem()
	switch {
	case sel == nil:
		return m, nil, true
	case isUnionSetItem(sel):
		m.setStatusMessage("Union sets cannot be marked", true)
		return m, scheduleStatusClear(), true
	case !sel.IsContext:
		return m, nil, true
	case !m.isSelected(*sel) && len(m.markedContexts()) >= MaxUnionContexts:
		m.setStatusMessage(fmt.Sprintf("A union accepts at most %d clusters", MaxUnionContexts), true)
		return m, scheduleStatusClear(), true
	}
	m.toggleSelection(*sel)
	c := min(m.cursor()+1, len(m.visibleMiddleItems())-1)
	m.setCursor(max(c, 0))
	return m, nil, true
}

// openAdHocNamespacePicker asks for the namespace of an ad-hoc union. The
// marks stay until the union starts.
func (m Model) openAdHocNamespacePicker(contexts []string) (tea.Model, tea.Cmd) {
	opts := StartupOptions{UnionContexts: contexts, Namespaces: []string{"pending"}}
	if err := ValidateUnionOptions(opts, m.contextExistsFn()); err != nil {
		m.setStatusMessage(err.Error(), true)
		return m, scheduleStatusClear()
	}
	m.pendingUnionSetName = adHocUnionPending
	m.pendingUnionContexts = contexts
	m.allNamespaces = false
	m.selectedNamespaces = nil
	m.namespace = ""
	return m.openNamespaceSelectorForContext(contexts[0])
}

func (m Model) contextExistsFn() func(string) bool {
	if m.client == nil {
		return nil
	}
	return m.client.ContextExists
}

func (m Model) activateAdHocUnion(namespace string) (tea.Model, tea.Cmd) {
	contexts := m.pendingUnionContexts
	m.pendingUnionSetName = ""
	m.pendingUnionContexts = nil
	opts := StartupOptions{UnionContexts: contexts, Namespaces: []string{namespace}}
	if err := ValidateUnionOptions(opts, m.contextExistsFn()); err != nil {
		m.setStatusMessage(err.Error(), true)
		return m, scheduleStatusClear()
	}
	m.clearSelection()
	return m.enterUnion("", contexts, nil, namespace, true)
}
