package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
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
	m.logView.podFilterText = ""
	m.logView.podFilterActive = false
	return m, nil
}

// executeActionGoToNode handles the "Go to Node" action on a Pod —
// teleports the explorer to the Node hosting the selected pod. The
// node name is read from the pod's "Node" column (populated by
// populatePodDetails from spec.nodeName). Unscheduled pods (Pending,
// no nodeName yet) surface a status message instead of navigating to
// an empty name that would land the user on the Node list with the
// wrong cursor.
func (m Model) executeActionGoToNode() (tea.Model, tea.Cmd) {
	var nodeName string
	for _, kv := range m.actionCtx.columns {
		if kv.Key == "Node" {
			nodeName = kv.Value
			break
		}
	}
	if nodeName == "" {
		m.setStatusMessage("Pod is not scheduled to a node", true)
		return m, scheduleStatusClear()
	}
	return m.navigateToOwner("Node", nodeName)
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
	m.addLogEntry("DBG", fmt.Sprintf("$ helm get values %s -n %s --kube-context %s -o yaml → $KUBE_EDITOR/$EDITOR → helm upgrade --reuse-values", name, ns, ctx))
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
// For ServiceAccount/Role/ClusterRole/RoleBinding/ClusterRoleBinding kinds
// it opens the CanI (RBAC Explorer) browser with the selected resource
// pre-selected as the subject, so the user sees what that resource can
// actually do.  For all other kinds it falls back to a
// SelfSubjectAccessReview on the current user.
func (m Model) executeActionPermissions() (tea.Model, tea.Cmd) {
	kind := m.actionCtx.kind
	switch kind {
	case "ServiceAccount":
		ns := m.actionCtx.namespace
		name := m.actionCtx.name
		subject := fmt.Sprintf("system:serviceaccount:%s:%s", ns, name)
		m.canISubject = subject
		m.canISubjectName = name
		m.loading = true
		m.setStatusMessage("Loading RBAC permissions for "+name+"...", false)
		return m, tea.Batch(m.loadCanIRules(), scheduleStatusClear())
	case "Role", "ClusterRole":
		// Role/ClusterRole define permissions but cannot be impersonated
		// directly. Extract the rules from the resource spec and display
		// them directly in the RBAC explorer.
		rules := extractRoleRules(m.actionCtx.raw)
		m.canISubject = ""
		m.canISubjectName = m.actionCtx.name
		m.loading = true
		m.setStatusMessage("Loading RBAC permissions for "+m.actionCtx.name+"...", false)
		return m, tea.Batch(m.loadRoleRules(rules), scheduleStatusClear())
	case "RoleBinding", "ClusterRoleBinding":
		// Extract the first subject from the binding and check its permissions.
		subject, subjectName := extractBindingSubject(m.actionCtx.raw, m.actionCtx.namespace)
		m.canISubject = subject
		m.canISubjectName = subjectName
		m.loading = true
		m.setStatusMessage("Loading RBAC permissions for "+subjectName+"...", false)
		return m, tea.Batch(m.loadCanIRules(), scheduleStatusClear())
	default:
		m.loading = true
		m.setStatusMessage("Checking RBAC permissions...", false)
		return m, m.checkRBAC()
	}
}

// extractBindingSubject extracts the first subject from a RoleBinding or
// ClusterRoleBinding's raw object and returns the impersonation string and
// a display name.  When no subjects are present it returns empty strings.
func extractBindingSubject(raw map[string]any, bindingNS string) (subject, displayName string) {
	if raw == nil {
		return "", ""
	}
	subjects, ok := raw["subjects"].([]any)
	if !ok || len(subjects) == 0 {
		return "", ""
	}
	first, ok := subjects[0].(map[string]any)
	if !ok {
		return "", ""
	}
	kind, _ := first["kind"].(string)
	name, _ := first["name"].(string)
	if name == "" {
		return "", ""
	}
	switch kind {
	case "ServiceAccount":
		saNS, _ := first["namespace"].(string)
		if saNS == "" {
			saNS = bindingNS
		}
		subject = fmt.Sprintf("system:serviceaccount:%s:%s", saNS, name)
		displayName = name
	case "User":
		subject = name
		displayName = name
	case "Group":
		subject = "group:" + name
		displayName = "group:" + name
	default:
		// Unknown subject kind — treat as a user name.
		subject = name
		displayName = name
	}
	return subject, displayName
}

// extractRoleRules extracts the rules from a Role or ClusterRole spec
// and returns them as []k8s.AccessRule.  When the raw object or its
// rules field is missing it returns nil.
func extractRoleRules(raw map[string]any) []k8s.AccessRule {
	if raw == nil {
		return nil
	}
	// Role and ClusterRole store rules at the top level (not under spec).
	rulesRaw, ok := raw["rules"].([]any)
	if !ok || len(rulesRaw) == 0 {
		return nil
	}
	var rules []k8s.AccessRule
	for _, r := range rulesRaw {
		ruleMap, ok := r.(map[string]any)
		if !ok {
			continue
		}
		rules = append(rules, k8s.AccessRule{
			Verbs:         toStringSlice(ruleMap["verbs"]),
			APIGroups:     toStringSlice(ruleMap["apiGroups"]),
			Resources:     toStringSlice(ruleMap["resources"]),
			ResourceNames: toStringSlice(ruleMap["resourceNames"]),
		})
	}
	return rules
}

// toStringSlice converts an any value that is a slice of strings (or
// float64 numbers from unmarshalled JSON) into []string.  Values that
// are already []string are returned as-is; numeric values from JSON
// unmarshalling are converted via fmt.Sprintf.
func toStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	switch s := v.(type) {
	case []string:
		return s
	case []any:
		var out []string
		for _, item := range s {
			switch val := item.(type) {
			case string:
				out = append(out, val)
			case float64:
				out = append(out, fmt.Sprintf("%.0f", val))
			}
		}
		return out
	}
	return nil
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

// removeSelectedPortForward removes the port forward under the cursor in the
// __port_forwards__ browser — the path taken by the D / delete key. Mirrors
// the action-menu "Remove" but sources the entry from the selected row
// rather than m.actionCtx.
func (m Model) removeSelectedPortForward() (tea.Model, tea.Cmd) {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil
	}
	pfID := m.getPortForwardID(sel.Columns)
	if pfID <= 0 {
		return m, nil
	}
	m.portForwardMgr.Remove(pfID)
	m.setMiddleItems(m.portForwardItems())
	m.clampCursor()
	m.saveCurrentPortForwards()
	m.setStatusMessage("Port forward removed", false)
	return m, scheduleStatusClear()
}
