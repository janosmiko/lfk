package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// executeActionShell handles the "Shell" action.
func (m Model) executeActionShell() (tea.Model, tea.Cmd) {
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	m.addLogEntry("DBG", fmt.Sprintf("$ kubectl run lfk-node-shell-<rand> -n kube-system --rm -it --restart=Never --image=busybox --context %s --overrides='<spec pinned to %s with hostPID/IPC/Net + privileged + nsenter + system-node-critical + tolerate-everything>'", ctx, name))
	return m, m.execKubectlNodeShell()
}

// executeActionDebugPod handles the "Debug Pod" action.
func (m Model) executeActionDebugPod() (tea.Model, tea.Cmd) {
	ns := m.actionCtx.namespace
	ctx := m.actionCtx.context
	m.addLogEntry("DBG", fmt.Sprintf("$ kubectl run lfk-debug-<id> --image=alpine --rm -it --restart=Never -n %s --context %s -- sh", ns, ctx))
	return m, m.runDebugPod()
}

// executeActionGoToPod handles the "Go to Pod" action.
func (m Model) executeActionGoToPod() (tea.Model, tea.Cmd) {
	ns := m.actionCtx.namespace
	var podNames []string
	for _, kv := range m.actionCtx.columns {
		if kv.Key == "Used By" && kv.Value != "" {
			for p := range strings.SplitSeq(kv.Value, ", ") {
				p = strings.TrimSpace(p)
				if p != "" {
					podNames = append(podNames, p)
				}
			}
			break
		}
	}
	if len(podNames) == 0 {
		m.setStatusMessage("No pods using this PVC", true)
		return m, scheduleStatusClear()
	}
	if len(podNames) == 1 {
		return m.navigateToOwner("Pod", podNames[0])
	}
	var items []model.Item
	for _, pn := range podNames {
		items = append(items, model.Item{Name: pn, Namespace: ns})
	}
	m.overlayItems = items
	m.overlay = overlayPodSelect
	m.overlayCursor = 0
	m.pendingAction = "Go to Pod"
	m.logPodFilterText = ""
	m.logPodFilterActive = false
	ui.ResetOverlayPodScroll()
	return m, nil
}

// executeActionDebugMount handles the "Debug Mount" action.
func (m Model) executeActionDebugMount() (tea.Model, tea.Cmd) {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	m.addLogEntry("DBG", fmt.Sprintf("$ kubectl run debug-pvc --image=alpine -it --rm --restart=Never --overrides='{...pvc:%s...}' -n %s --context %s", name, ns, ctx))
	return m, m.runDebugPodWithPVC()
}

// executeActionOpenInBrowser handles the "Open in Browser" action.
func (m Model) executeActionOpenInBrowser() (tea.Model, tea.Cmd) {
	if m.actionCtx.kind == "__port_forward_entry__" || m.actionCtx.kind == "__port_forwards__" {
		var localPort string
		for _, kv := range m.actionCtx.columns {
			if kv.Key == "Local" {
				localPort = kv.Value
				break
			}
		}
		if localPort != "" {
			url := "http://localhost:" + localPort
			m.setStatusMessage("Opening "+url, false)
			return m, tea.Batch(openInBrowser(url), scheduleStatusClear())
		}
		m.setStatusMessage("No local port found", true)
		return m, scheduleStatusClear()
	}
	return m.openIngressInBrowser()
}

// openIngressInBrowser extracts the pre-computed URL from the selected Ingress
// resource's hidden __ingress_url column and opens it in the default browser.
func (m Model) openIngressInBrowser() (tea.Model, tea.Cmd) {
	sel := m.selectedMiddleItem()
	if sel == nil {
		m.setStatusMessage("No resource selected", true)
		return m, scheduleStatusClear()
	}
	// Find the pre-computed URL in the item's columns.
	var url string
	for _, kv := range sel.Columns {
		if kv.Key == "__ingress_url" {
			url = kv.Value
			break
		}
	}
	if url == "" {
		m.setStatusMessage("No host found for this ingress", true)
		return m, scheduleStatusClear()
	}
	m.setStatusMessage("Opening "+url, false)
	return m, tea.Batch(openInBrowser(url), scheduleStatusClear())
}

// executeActionHelmValues handles the "Values" and "All Values" actions.
func (m Model) executeActionHelmValues(all bool) (tea.Model, tea.Cmd) {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	if all {
		m.addLogEntry("DBG", fmt.Sprintf("$ helm get values %s -n %s --kube-context %s -o yaml --all", name, ns, ctx))
	} else {
		m.addLogEntry("DBG", fmt.Sprintf("$ helm get values %s -n %s --kube-context %s -o yaml", name, ns, ctx))
	}
	m.loading = true
	return m, m.loadHelmValues(all)
}

// executeActionEditValues handles the "Edit Values" action.
func (m Model) executeActionEditValues() (tea.Model, tea.Cmd) {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	m.addLogEntry("DBG", fmt.Sprintf("$ helm get values %s -n %s --kube-context %s -o yaml → $EDITOR → helm upgrade --reuse-values", name, ns, ctx))
	return m, m.editHelmValues()
}

// executeActionDiff handles the "Diff" action.
func (m Model) executeActionDiff() (tea.Model, tea.Cmd) {
	name := m.actionCtx.name
	if m.actionCtx.kind == "HelmRelease" {
		m.addLogEntry("DBG", fmt.Sprintf("Comparing default vs user values for %s", name))
		m.loading = true
		return m, m.helmDiff()
	}
	// Non-Helm diff (two-resource YAML diff) is handled via bulk action.
	return m, nil
}

// executeActionUpgrade handles the "Upgrade" action.
func (m Model) executeActionUpgrade() (tea.Model, tea.Cmd) {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	m.addLogEntry("DBG", fmt.Sprintf("$ helm upgrade %s -n %s --kube-context %s", name, ns, ctx))
	return m, m.helmUpgrade()
}

// executeActionPermissions handles the "Permissions" action.
func (m Model) executeActionPermissions() (tea.Model, tea.Cmd) {
	m.loading = true
	m.setStatusMessage("Checking RBAC permissions...", false)
	return m, m.checkRBAC()
}

// executeActionStartupAnalysis handles the "Startup Analysis" action.
func (m Model) executeActionStartupAnalysis() (tea.Model, tea.Cmd) {
	m.loading = true
	m.setStatusMessage("Analyzing pod startup...", false)
	return m, m.loadPodStartup()
}

// executeActionAlerts handles the "Alerts" action.
func (m Model) executeActionAlerts() (tea.Model, tea.Cmd) {
	m.loading = true
	m.setStatusMessage("Loading Prometheus alerts...", false)
	return m, m.loadAlerts()
}

// executeActionLabelsAnnotations handles the "Labels / Annotations" action.
func (m Model) executeActionLabelsAnnotations() (tea.Model, tea.Cmd) {
	m.labelResourceType = m.actionCtx.resourceType
	return m, m.loadLabelData()
}

// executeActionStop handles the "Stop" action.
func (m Model) executeActionStop() (tea.Model, tea.Cmd) {
	// Stop a port forward entry.
	if m.actionCtx.kind == "__port_forward_entry__" || m.actionCtx.kind == "__port_forwards__" {
		pfID := m.getPortForwardID(m.actionCtx.columns)
		if pfID > 0 {
			return m, m.stopPortForward(pfID)
		}
	}
	// Stop a capture entry.
	if m.actionCtx.kind == "__captures__" {
		sel := m.selectedMiddleItem()
		if sel != nil {
			return m.stopCaptureFromPseudo(*sel)
		}
	}
	return m, nil
}

// executeActionRemove handles the "Remove" action.
func (m Model) executeActionRemove() (tea.Model, tea.Cmd) {
	// Remove a port forward entry.
	if m.actionCtx.kind == "__port_forward_entry__" || m.actionCtx.kind == "__port_forwards__" {
		pfID := m.getPortForwardID(m.actionCtx.columns)
		if pfID > 0 {
			m.portForwardMgr.Remove(pfID)
			m.setMiddleItems(m.portForwardItems())
			m.clampCursor()
			m.saveCurrentPortForwards()
			m.setStatusMessage("Port forward removed", false)
			return m, scheduleStatusClear()
		}
	}
	return m, nil
}
