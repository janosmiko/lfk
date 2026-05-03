package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) updateRbacCheck(msg rbacCheckMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setStatusMessage(fmt.Sprintf("RBAC check failed: %v", msg.err), true)
		return m, scheduleStatusClear()
	}
	m.rbacResults = msg.results
	m.rbacKind = msg.kind
	m.overlay = overlayRBAC
	return m, nil
}

func (m Model) updateCanILoaded(msg canILoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setStatusMessage(fmt.Sprintf("RBAC rules check failed: %v", msg.err), true)
		return m, scheduleStatusClear()
	}
	m.processCanIRules(msg.rules)
	m.canINamespaces = msg.namespaces
	m.overlay = overlayCanI
	m.canIGroupCursor = 0
	m.canIGroupScroll = 0
	return m, nil
}

func (m Model) updateCanISAList(msg canISAListMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setStatusMessage(fmt.Sprintf("Failed to list subjects: %v", msg.err), true)
		return m, scheduleStatusClear()
	}
	m.canIServiceAccounts = msg.accounts
	// Build overlay items: "Current User", then Users, Groups, ServiceAccounts.
	items := make([]model.Item, 0, len(msg.subjects)+len(msg.accounts)+1)
	items = append(items, model.Item{Name: "Current User", Extra: ""})

	// Add Users from RBAC bindings.
	for _, subj := range msg.subjects {
		if subj.Kind == "User" {
			items = append(items, model.Item{
				Name:  "[User] " + subj.Name,
				Kind:  "User",
				Extra: subj.Name,
			})
		}
	}
	// Add Groups from RBAC bindings.
	for _, subj := range msg.subjects {
		if subj.Kind == "Group" {
			items = append(items, model.Item{
				Name:  "[Group] " + subj.Name,
				Kind:  "Group",
				Extra: "group:" + subj.Name,
			})
		}
	}
	// Add ServiceAccounts.
	for _, sa := range msg.accounts {
		var name, ns string
		if parts := strings.SplitN(sa, "/", 2); len(parts) == 2 {
			ns = parts[0]
			name = parts[1]
		} else {
			ns = m.namespace
			name = sa
		}
		items = append(items, model.Item{
			Name:  "[SA] " + ns + "/" + name,
			Kind:  "ServiceAccount",
			Extra: fmt.Sprintf("system:serviceaccount:%s:%s", ns, name),
		})
	}
	m.overlayItems = items
	m.overlayCursor = 0
	m.overlayFilter.Clear()
	m.canISubjectFilterMode = false
	ui.ResetOverlayCanISubjectScroll()
	m.overlay = overlayCanISubject
	return m, nil
}

func (m Model) updateQuotaLoaded(msg quotaLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setStatusMessage(fmt.Sprintf("Quota load failed: %v", msg.err), true)
		return m, scheduleStatusClear()
	}
	if len(msg.quotas) == 0 {
		m.setStatusMessage("No resource quotas found in namespace", false)
		return m, scheduleStatusClear()
	}
	m.quotaData = msg.quotas
	m.overlay = overlayQuotaDashboard
	return m, nil
}

func (m Model) updateAlertsLoaded(msg alertsLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setStatusMessage(fmt.Sprintf("Alerts: %v", msg.err), true)
		return m, scheduleStatusClear()
	}
	m.alertsData = msg.alerts
	m.alertsScroll = 0
	m.overlay = overlayAlerts
	return m, nil
}

func (m Model) updateNetpolLoaded(msg netpolLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setStatusMessage(fmt.Sprintf("Failed to load network policy: %v", msg.err), true)
		return m, scheduleStatusClear()
	}
	m.netpolData = msg.info
	m.netpolScroll = 0
	m.overlay = overlayNetworkPolicy
	return m, nil
}
