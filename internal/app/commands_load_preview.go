package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// loadPreview loads the right column based on the current level and selection.
func (m Model) loadPreview() tea.Cmd {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return nil
	}

	switch m.nav.Level {
	case model.LevelClusters:
		return m.loadResourceTypes()
	case model.LevelResourceTypes:
		return m.loadPreviewResourceTypes(sel)
	case model.LevelResources:
		return m.loadPreviewResources()
	case model.LevelOwned:
		return m.loadPreviewOwned(sel)
	case model.LevelContainers:
		return nil
	}
	return nil
}

// loadPreviewResourceTypes handles preview loading at the resource types level.
func (m Model) loadPreviewResourceTypes(sel *model.Item) tea.Cmd {
	if sel.Extra == "__overview__" {
		if ui.ConfigDashboard {
			return m.loadDashboard()
		}
		return nil
	}
	if sel.Extra == "__monitoring__" {
		return m.loadMonitoringDashboard()
	}
	if sel.Kind == "__collapsed_group__" {
		return nil
	}
	if sel.Kind == "__port_forwards__" {
		items := m.portForwardItems()
		return func() tea.Msg {
			return resourcesLoadedMsg{items: items, forPreview: true}
		}
	}
	return m.loadResources(true)
}

// loadPreviewResources handles preview loading at the resources level.
func (m Model) loadPreviewResources() tea.Cmd {
	if m.nav.ResourceType.Kind == "__port_forwards__" {
		return nil
	}
	var cmds []tea.Cmd
	switch {
	case m.mapView && m.resourceTypeHasChildren():
		cmds = append(cmds, m.loadResourceTree())
	case m.resourceTypeHasChildren():
		cmds = append(cmds, m.loadOwned(true))
	case m.nav.ResourceType.Kind == "Pod":
		cmds = append(cmds, m.loadContainers(true))
	}
	if m.fullYAMLPreview {
		cmds = append(cmds, m.loadPreviewYAML())
	}
	kind := m.nav.ResourceType.Kind
	if kind == "Pod" || kind == "Deployment" || kind == "StatefulSet" || kind == "DaemonSet" {
		if metricsCmd := m.loadMetrics(); metricsCmd != nil {
			cmds = append(cmds, metricsCmd)
		}
	}
	if eventsCmd := m.loadPreviewEvents(); eventsCmd != nil {
		cmds = append(cmds, eventsCmd)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// loadPreviewOwned handles preview loading at the owned level.
func (m Model) loadPreviewOwned(sel *model.Item) tea.Cmd {
	if sel.Kind == "Pod" {
		var cmds []tea.Cmd
		cmds = append(cmds, m.loadContainers(true))
		if m.fullYAMLPreview {
			cmds = append(cmds, m.loadPreviewYAML())
		}
		if metricsCmd := m.loadMetrics(); metricsCmd != nil {
			cmds = append(cmds, metricsCmd)
		}
		if eventsCmd := m.loadPreviewEvents(); eventsCmd != nil {
			cmds = append(cmds, eventsCmd)
		}
		return tea.Batch(cmds...)
	}
	if kindHasOwnedChildren(sel.Kind) {
		return nil
	}
	if m.fullYAMLPreview {
		return m.loadPreviewYAML()
	}
	name := sel.Name
	kctx := m.nav.Context
	// Fall back to nav.Namespace (set when drilling into a helm release or
	// argocd application) so children without a metadata.namespace — common
	// for helm manifests that rely on --namespace rather than templating
	// .Release.Namespace — are fetched from the parent's namespace instead
	// of the ambient namespace filter.
	ns := m.resolveNamespace()
	if sel.Namespace != "" {
		ns = sel.Namespace
	}
	reqCtx := m.reqCtx
	rt, ok := m.resolveOwnedResourceType(sel)
	if !ok {
		return func() tea.Msg {
			return yamlLoadedMsg{err: fmt.Errorf("unknown resource type: %s", sel.Kind)}
		}
	}
	return func() tea.Msg {
		content, err := m.client.GetResourceYAML(reqCtx, kctx, ns, rt, name)
		return yamlLoadedMsg{content: content, err: err}
	}
}

// loadPreviewYAML loads the YAML for the currently selected middle item into previewYAML.
func (m Model) loadPreviewYAML() tea.Cmd {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return nil
	}

	kctx := m.nav.Context
	ns := m.resolveNamespace()
	gen := m.requestGen
	reqCtx := m.reqCtx

	switch m.nav.Level {
	case model.LevelResources:
		rt := m.nav.ResourceType
		name := sel.Name
		itemNs := ns
		if sel.Namespace != "" {
			itemNs = sel.Namespace
		}
		return func() tea.Msg {
			content, err := m.client.GetResourceYAML(reqCtx, kctx, itemNs, rt, name)
			return previewYAMLLoadedMsg{content: content, err: err, gen: gen}
		}
	case model.LevelOwned:
		name := sel.Name
		itemNs := ns
		if sel.Namespace != "" {
			itemNs = sel.Namespace
		}
		if sel.Kind == "Pod" {
			return func() tea.Msg {
				content, err := m.client.GetPodYAML(reqCtx, kctx, itemNs, name)
				return previewYAMLLoadedMsg{content: content, err: err, gen: gen}
			}
		}
		rt, ok := m.resolveOwnedResourceType(sel)
		if !ok {
			return func() tea.Msg {
				return previewYAMLLoadedMsg{err: fmt.Errorf("unknown resource type: %s", sel.Kind), gen: gen}
			}
		}
		return func() tea.Msg {
			content, err := m.client.GetResourceYAML(reqCtx, kctx, itemNs, rt, name)
			return previewYAMLLoadedMsg{content: content, err: err, gen: gen}
		}
	}
	return nil
}

// loadEventTimeline fetches events correlated with the current action target resource.
func (m Model) loadEventTimeline() tea.Cmd {
	client := m.client
	ctx := m.actionCtx.context
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	kind := m.actionCtx.kind
	return func() tea.Msg {
		events, err := client.GetResourceEvents(context.Background(), ctx, ns, name, kind)
		return eventTimelineMsg{events: events, err: err}
	}
}

func (m Model) checkRBAC() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionCtx.namespace
	rt := m.actionCtx.resourceType
	return func() tea.Msg {
		results, err := m.client.CheckRBAC(context.Background(), ctx, ns, rt.APIGroup, rt.Resource)
		return rbacCheckMsg{results: results, kind: rt.Kind, resource: rt.Resource, err: err}
	}
}

func (m Model) loadCanIRules() tea.Cmd {
	client := m.client
	ctx := m.nav.Context
	ns := m.namespace
	if m.allNamespaces || ns == "" {
		ns = "default"
	}
	subject := m.canISubject

	// When checking a specific SA, discover all namespaces where it has
	// RoleBindings and query permissions across all of them.
	if subject != "" && strings.HasPrefix(subject, "system:serviceaccount:") {
		return func() tea.Msg {
			rules, namespaces, err := client.GetSelfRulesMultiNS(context.Background(), ctx, subject)
			return canILoadedMsg{rules: rules, namespaces: namespaces, err: err}
		}
	}

	// User or Group impersonation: query in the current namespace.
	// GetSelfRulesAs handles the "group:" prefix internally.
	if subject != "" {
		viewNS := ns
		return func() tea.Msg {
			rules, err := client.GetSelfRulesAs(context.Background(), ctx, viewNS, subject)
			return canILoadedMsg{rules: rules, namespaces: []string{viewNS}, err: err}
		}
	}

	// Current user: use the active namespace only.
	return func() tea.Msg {
		rules, err := client.GetSelfRulesAs(context.Background(), ctx, ns, "")
		return canILoadedMsg{rules: rules, namespaces: []string{ns}, err: err}
	}
}

func (m Model) loadCanISAList() tea.Cmd {
	client := m.client
	ctx := m.nav.Context
	// Always list SAs across all namespaces so the user can check
	// permissions for any service account regardless of the current view.
	// Also discover Users and Groups from RBAC bindings.
	return func() tea.Msg {
		accounts, err := client.ListServiceAccounts(context.Background(), ctx, "")
		if err != nil {
			return canISAListMsg{err: err}
		}
		subjects, _ := client.ListRBACSubjects(context.Background(), ctx)
		return canISAListMsg{accounts: accounts, subjects: subjects}
	}
}

func (m Model) loadPodStartup() tea.Cmd {
	client := m.client
	ctx := m.actionCtx.context
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	return func() tea.Msg {
		info, err := client.GetPodStartupAnalysis(context.Background(), ctx, ns, name)
		return podStartupMsg{info: info, err: err}
	}
}

func (m Model) loadAlerts() tea.Cmd {
	kubeCtx := m.actionCtx.context
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	kind := m.actionCtx.kind
	return func() tea.Msg {
		alerts, err := m.client.GetActiveAlerts(context.Background(), kubeCtx, ns, name, kind)
		return alertsLoadedMsg{alerts: alerts, err: err}
	}
}

// loadNetworkPolicy fetches and parses a NetworkPolicy for visualization.
func (m Model) loadNetworkPolicy() tea.Cmd {
	client := m.client
	kctx := m.actionCtx.context
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	return func() tea.Msg {
		info, err := client.GetNetworkPolicyInfo(context.Background(), kctx, ns, name)
		return netpolLoadedMsg{info: info, err: err}
	}
}

// loadHelmValues runs `helm get values` and returns the output as a message.
// If allValues is true, the --all flag is included to show computed defaults too.
func (m Model) loadHelmValues(allValues bool) tea.Cmd {
	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return func() tea.Msg {
			return helmValuesLoadedMsg{err: fmt.Errorf("helm not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	kubeconfigPaths := m.client.KubeconfigPaths()

	args := []string{"get", "values", name, "-n", ns, "--kube-context", ctx, "-o", "yaml"}
	titleSuffix := "User Values"
	if allValues {
		args = append(args, "--all")
		titleSuffix = "All Values"
	}

	title := fmt.Sprintf("Helm %s: %s", titleSuffix, name)

	return func() tea.Msg {
		cmd := exec.Command(helmPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
		logger.Info("Running helm command", "cmd", cmd.String())
		output, cmdErr := cmd.CombinedOutput()
		if cmdErr != nil {
			logger.Error("helm get values failed", "cmd", cmd.String(), "error", cmdErr, "output", string(output))
			return helmValuesLoadedMsg{
				title: title,
				err:   fmt.Errorf("%w: %s", cmdErr, strings.TrimSpace(string(output))),
			}
		}
		content := strings.TrimSpace(string(output))
		if content == "" || content == "null" {
			content = "# No user-supplied values"
		}
		return helmValuesLoadedMsg{
			content: content,
			title:   title,
		}
	}
}

// loadContainerPorts loads the available ports for the action context resource.
func (m Model) loadContainerPorts() tea.Cmd {
	client := m.client
	kctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	kind := m.actionCtx.kind

	return func() tea.Msg {
		var ports []k8s.ContainerPort
		var err error
		switch kind {
		case "Pod":
			ports, err = client.GetContainerPorts(context.Background(), kctx, ns, name)
		case "Service":
			ports, err = client.GetServicePorts(context.Background(), kctx, ns, name)
		case "Deployment":
			ports, err = client.GetDeploymentPorts(context.Background(), kctx, ns, name)
		case "StatefulSet":
			ports, err = client.GetStatefulSetPorts(context.Background(), kctx, ns, name)
		case "DaemonSet":
			ports, err = client.GetDaemonSetPorts(context.Background(), kctx, ns, name)
		default:
			err = fmt.Errorf("unsupported kind for port discovery: %s", kind)
		}
		return containerPortsLoadedMsg{ports: ports, err: err}
	}
}

// waitForStderr listens for captured stderr output and returns it as a message.
func (m Model) waitForStderr() tea.Cmd {
	if m.stderrChan == nil {
		return nil
	}
	ch := m.stderrChan
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return stderrCapturedMsg{message: msg}
	}
}
