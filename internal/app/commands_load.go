package app

import (
	"context"
	"encoding/json"
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

// --- Commands ---

func (m Model) loadContexts() tea.Msg {
	items, err := m.client.GetContexts()
	return contextsLoadedMsg{items: items, err: err}
}

func (m Model) loadResourceTypes() tea.Cmd {
	crds := m.discoveredCRDs[m.nav.Context]
	return func() tea.Msg {
		if len(crds) > 0 {
			return resourceTypesMsg{items: model.MergeWithCRDs(crds)}
		}
		// No CRDs discovered yet: hide ArgoCD and Gateway API (require CRDs), show Helm (uses helm binary).
		// Pass nil availableGroups so individual CRD-dependent entries (e.g. VPA) are also hidden.
		return resourceTypesMsg{items: model.FlattenedResourceTypesFiltered(nil)}
	}
}

// discoverCRDs launches async CRD discovery for the given context.
// Uses context.Background() instead of m.reqCtx because CRD discovery should
// not be cancelled by navigation actions (cancelAndReset).
func (m Model) discoverCRDs(contextName string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		entries, err := client.DiscoverCRDs(context.Background(), contextName)
		return crdDiscoveryMsg{context: contextName, entries: entries, err: err}
	}
}

// loadQuotas fetches ResourceQuota objects for the current namespace.
func (m Model) loadQuotas() tea.Cmd {
	client := m.client
	ctx := m.nav.Context
	ns := m.effectiveNamespace()
	return func() tea.Msg {
		quotas, err := client.GetNamespaceQuotas(context.Background(), ctx, ns)
		return quotaLoadedMsg{quotas: quotas, err: err}
	}
}

func (m Model) loadResources(forPreview bool) tea.Cmd {
	kctx := m.nav.Context
	ns := m.effectiveNamespace()
	rt := m.nav.ResourceType
	gen := m.requestGen
	reqCtx := m.reqCtx
	if forPreview {
		sel := m.selectedMiddleItem()
		if sel == nil {
			return nil
		}
		found, ok := model.FindResourceTypeIn(sel.Extra, m.discoveredCRDs[kctx])
		if !ok {
			return nil
		}
		rt = found
	}
	return func() tea.Msg {
		items, err := m.client.GetResources(reqCtx, kctx, ns, rt)
		return resourcesLoadedMsg{items: items, err: err, forPreview: forPreview, gen: gen}
	}
}

func (m Model) loadOwned(forPreview bool) tea.Cmd {
	kctx := m.nav.Context
	ns := m.effectiveNamespace()
	kind := m.nav.ResourceType.Kind
	name := m.nav.ResourceName
	gen := m.requestGen
	reqCtx := m.reqCtx
	if forPreview {
		sel := m.selectedMiddleItem()
		if sel == nil {
			return nil
		}
		name = sel.Name
		if sel.Namespace != "" {
			ns = sel.Namespace
		}
	} else if ns == "" && m.nav.Namespace != "" {
		ns = m.nav.Namespace
	}
	return func() tea.Msg {
		items, err := m.client.GetOwnedResources(reqCtx, kctx, ns, kind, name)
		return ownedLoadedMsg{items: items, err: err, forPreview: forPreview, gen: gen}
	}
}

func (m Model) loadResourceTree() tea.Cmd {
	kctx := m.nav.Context
	ns := m.effectiveNamespace()
	gen := m.requestGen
	reqCtx := m.reqCtx

	var kind, name string
	switch m.nav.Level {
	case model.LevelResources:
		sel := m.selectedMiddleItem()
		if sel == nil {
			return nil
		}
		kind = m.nav.ResourceType.Kind
		name = sel.Name
		if sel.Namespace != "" {
			ns = sel.Namespace
		}
	case model.LevelOwned:
		sel := m.selectedMiddleItem()
		if sel == nil {
			return nil
		}
		kind = sel.Kind
		name = sel.Name
		if sel.Namespace != "" {
			ns = sel.Namespace
		}
	default:
		return nil
	}

	return func() tea.Msg {
		tree, err := m.client.GetResourceTree(reqCtx, kctx, ns, kind, name)
		return resourceTreeLoadedMsg{tree: tree, err: err, gen: gen}
	}
}

func (m Model) loadContainers(forPreview bool) tea.Cmd {
	kctx := m.nav.Context
	ns := m.effectiveNamespace()
	gen := m.requestGen
	reqCtx := m.reqCtx
	if ns == "" && m.nav.Namespace != "" {
		ns = m.nav.Namespace
	}
	podName := m.nav.OwnedName
	if forPreview {
		sel := m.selectedMiddleItem()
		if sel == nil {
			return nil
		}
		podName = sel.Name
		if sel.Namespace != "" {
			ns = sel.Namespace
		}
	}
	return func() tea.Msg {
		items, err := m.client.GetContainers(reqCtx, kctx, ns, podName)
		return containersLoadedMsg{items: items, err: err, forPreview: forPreview, gen: gen}
	}
}

func (m Model) loadNamespaces() tea.Cmd {
	kctx := m.nav.Context
	if kctx == "" {
		kctx = m.client.CurrentContext()
	}
	// Use an independent context so namespace loading is never blocked or
	// cancelled by in-flight resource requests.
	return func() tea.Msg {
		items, err := m.client.GetNamespaces(context.Background(), kctx)
		return namespacesLoadedMsg{items: items, err: err}
	}
}

// resolveOwnedResourceType determines the correct ResourceTypeEntry for an
// owned item at LevelOwned. It uses the item's Kind to look up the type in
// both built-in resource types and discovered CRDs. If the kind is not found,
// it falls back to constructing a ResourceTypeEntry from the item's Extra
// metadata (which may contain "group/version" from ArgoCD status) and the
// Kind. Returns false if the type cannot be resolved.
func (m Model) resolveOwnedResourceType(sel *model.Item) (model.ResourceTypeEntry, bool) {
	if sel == nil {
		return model.ResourceTypeEntry{}, false
	}
	kctx := m.nav.Context
	crds := m.discoveredCRDs[kctx]

	// First try to find by Kind in built-in types and discovered CRDs.
	if rt, ok := model.FindResourceTypeByKind(sel.Kind, crds); ok {
		return rt, true
	}

	// If Extra contains a full resource ref string ("group/version/resource"), use it.
	if sel.Extra != "" {
		if rt, ok := model.FindResourceTypeIn(sel.Extra, crds); ok {
			return rt, true
		}
	}

	// Fallback: if Extra contains "group/version" (from ArgoCD status.resources),
	// construct a ResourceTypeEntry by deriving the plural resource name from Kind.
	if sel.Extra != "" && sel.Kind != "" {
		parts := strings.SplitN(sel.Extra, "/", 2)
		if len(parts) == 2 {
			group := parts[0]
			version := parts[1]
			resource := strings.ToLower(sel.Kind) + "s"
			return model.ResourceTypeEntry{
				Kind:       sel.Kind,
				APIGroup:   group,
				APIVersion: version,
				Resource:   resource,
				Namespaced: true,
			}, true
		}
	}

	return model.ResourceTypeEntry{}, false
}

func (m Model) loadYAML() tea.Cmd {
	kctx := m.nav.Context
	ns := m.resolveNamespace()
	reqCtx := m.reqCtx

	switch m.nav.Level {
	case model.LevelResources:
		sel := m.selectedMiddleItem()
		if sel == nil {
			return nil
		}
		rt := m.nav.ResourceType
		name := sel.Name
		itemNs := ns
		if sel.Namespace != "" {
			itemNs = sel.Namespace
		}
		return func() tea.Msg {
			content, err := m.client.GetResourceYAML(reqCtx, kctx, itemNs, rt, name)
			return yamlLoadedMsg{content: content, err: err}
		}
	case model.LevelOwned:
		sel := m.selectedMiddleItem()
		if sel == nil {
			return nil
		}
		name := sel.Name
		itemNs := ns
		if sel.Namespace != "" {
			itemNs = sel.Namespace
		}
		if sel.Kind == "Pod" {
			return func() tea.Msg {
				content, err := m.client.GetPodYAML(reqCtx, kctx, itemNs, name)
				return yamlLoadedMsg{content: content, err: err}
			}
		}
		rt, ok := m.resolveOwnedResourceType(sel)
		if !ok {
			return func() tea.Msg {
				return yamlLoadedMsg{err: fmt.Errorf("unknown resource type: %s", sel.Kind)}
			}
		}
		return func() tea.Msg {
			content, err := m.client.GetResourceYAML(reqCtx, kctx, itemNs, rt, name)
			return yamlLoadedMsg{content: content, err: err}
		}
	case model.LevelContainers:
		podName := m.nav.OwnedName
		return func() tea.Msg {
			content, err := m.client.GetPodYAML(reqCtx, kctx, ns, podName)
			return yamlLoadedMsg{content: content, err: err}
		}
	}
	return nil
}

// loadDiff fetches YAML for two resources and returns a diffLoadedMsg.
func (m Model) loadDiff(rt model.ResourceTypeEntry, itemA, itemB model.Item) tea.Cmd {
	kctx := m.nav.Context
	reqCtx := m.reqCtx

	resolveNS := func(item model.Item) string {
		if item.Namespace != "" {
			return item.Namespace
		}
		return m.resolveNamespace()
	}

	nsA := resolveNS(itemA)
	nsB := resolveNS(itemB)

	nameA := itemA.Name
	nameB := itemB.Name

	leftLabel := nameA
	rightLabel := nameB
	if nsA != nsB {
		leftLabel = nsA + "/" + nameA
		rightLabel = nsB + "/" + nameB
	}

	return func() tea.Msg {
		yamlA, errA := m.client.GetResourceYAML(reqCtx, kctx, nsA, rt, nameA)
		if errA != nil {
			return diffLoadedMsg{err: fmt.Errorf("fetching %s: %w", nameA, errA)}
		}
		yamlB, errB := m.client.GetResourceYAML(reqCtx, kctx, nsB, rt, nameB)
		if errB != nil {
			return diffLoadedMsg{err: fmt.Errorf("fetching %s: %w", nameB, errB)}
		}
		return diffLoadedMsg{
			left:      yamlA,
			right:     yamlB,
			leftName:  leftLabel,
			rightName: rightLabel,
		}
	}
}

// resolveNamespace returns the namespace to use for get/describe operations.
func (m Model) resolveNamespace() string {
	if m.nav.Namespace != "" {
		return m.nav.Namespace
	}
	return m.namespace
}

// loadMetrics triggers async metrics loading for the current resource.
func (m Model) loadMetrics() tea.Cmd {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return nil
	}

	kctx := m.nav.Context
	ns := m.resolveNamespace()
	if sel.Namespace != "" {
		ns = sel.Namespace
	}
	gen := m.requestGen
	client := m.client
	reqCtx := m.reqCtx

	kind := m.nav.ResourceType.Kind
	if m.nav.Level == model.LevelOwned {
		kind = sel.Kind
	}

	switch kind {
	case "Pod":
		podName := sel.Name
		return func() tea.Msg {
			pm, err := client.GetPodMetrics(reqCtx, kctx, ns, podName)
			if err != nil {
				return metricsLoadedMsg{gen: gen} // silently ignore
			}
			cpuReq, cpuLim, memReq, memLim, err := client.GetPodResourceRequests(reqCtx, kctx, ns, podName)
			if err != nil {
				cpuReq, cpuLim, memReq, memLim = 0, 0, 0, 0
			}
			return metricsLoadedMsg{
				cpuUsed: pm.CPU, cpuReq: cpuReq, cpuLim: cpuLim,
				memUsed: pm.Memory, memReq: memReq, memLim: memLim,
				gen: gen,
			}
		}
	case "Deployment", "StatefulSet", "DaemonSet":
		name := sel.Name
		return func() tea.Msg {
			// Get child pods.
			childItems, err := client.GetOwnedResources(reqCtx, kctx, ns, kind, name)
			if err != nil || len(childItems) == 0 {
				return metricsLoadedMsg{gen: gen}
			}
			var podNames []string
			for _, item := range childItems {
				if item.Kind == "Pod" {
					podNames = append(podNames, item.Name)
				}
			}
			if len(podNames) == 0 {
				return metricsLoadedMsg{gen: gen}
			}
			metrics, err := client.GetPodsMetrics(reqCtx, kctx, ns, podNames)
			if err != nil || len(metrics) == 0 {
				return metricsLoadedMsg{gen: gen}
			}

			var totalCPU, totalMem int64
			for _, pm := range metrics {
				totalCPU += pm.CPU
				totalMem += pm.Memory
			}

			// Sum requests/limits from all pods.
			var totalCPUReq, totalCPULim, totalMemReq, totalMemLim int64
			for _, podName := range podNames {
				cpuReq, cpuLim, memReq, memLim, err := client.GetPodResourceRequests(reqCtx, kctx, ns, podName)
				if err != nil {
					continue
				}
				totalCPUReq += cpuReq
				totalCPULim += cpuLim
				totalMemReq += memReq
				totalMemLim += memLim
			}

			return metricsLoadedMsg{
				cpuUsed: totalCPU, cpuReq: totalCPUReq, cpuLim: totalCPULim,
				memUsed: totalMem, memReq: totalMemReq, memLim: totalMemLim,
				gen: gen,
			}
		}
	}
	return nil
}

// loadPreviewEvents loads events for the currently selected resource to display
// in the preview pane below RESOURCE USAGE.
func (m Model) loadPreviewEvents() tea.Cmd {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return nil
	}

	kctx := m.nav.Context
	ns := m.resolveNamespace()
	if sel.Namespace != "" {
		ns = sel.Namespace
	}
	gen := m.requestGen
	client := m.client
	name := sel.Name

	kind := m.nav.ResourceType.Kind
	if m.nav.Level == model.LevelOwned {
		kind = sel.Kind
	}

	return func() tea.Msg {
		events, err := client.GetResourceEvents(context.Background(), kctx, ns, name, kind)
		if err != nil {
			return previewEventsLoadedMsg{gen: gen}
		}
		return previewEventsLoadedMsg{events: events, gen: gen}
	}
}

// loadPodMetricsForList fetches metrics for all pods in the current namespace
// and returns them to enrich the middle pane items.
func (m Model) loadPodMetricsForList() tea.Cmd {
	kctx := m.nav.Context
	ns := m.effectiveNamespace()
	gen := m.requestGen
	client := m.client
	reqCtx := m.reqCtx
	return func() tea.Msg {
		metrics, err := client.GetAllPodMetrics(reqCtx, kctx, ns)
		if err != nil {
			return podMetricsEnrichedMsg{gen: gen} // silently ignore
		}
		return podMetricsEnrichedMsg{metrics: metrics, gen: gen}
	}
}

// loadNodeMetricsForList fetches metrics for all nodes and returns them
// to enrich the middle pane items with CPU/MEM usage columns.
func (m Model) loadNodeMetricsForList() tea.Cmd {
	kctx := m.nav.Context
	gen := m.requestGen
	client := m.client
	reqCtx := m.reqCtx
	return func() tea.Msg {
		metrics, err := client.GetAllNodeMetrics(reqCtx, kctx)
		if err != nil {
			return nodeMetricsEnrichedMsg{gen: gen}
		}
		return nodeMetricsEnrichedMsg{metrics: metrics, gen: gen}
	}
}

// loadSecretData fetches secret data for the secret editor.
func (m Model) loadSecretData() tea.Cmd {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return nil
	}

	kctx := m.nav.Context
	ns := m.resolveNamespace()
	if sel.Namespace != "" {
		ns = sel.Namespace
	}
	name := sel.Name
	client := m.client
	reqCtx := m.reqCtx

	return func() tea.Msg {
		data, err := client.GetSecretData(reqCtx, kctx, ns, name)
		return secretDataLoadedMsg{data: data, err: err}
	}
}

// saveSecretData saves the modified secret data back to the cluster.
func (m Model) saveSecretData() tea.Cmd {
	if m.secretData == nil {
		return nil
	}

	ctx := m.nav.Context
	ns := m.resolveNamespace()
	sel := m.selectedMiddleItem()
	if sel == nil {
		return nil
	}
	if sel.Namespace != "" {
		ns = sel.Namespace
	}
	name := sel.Name
	data := make(map[string]string, len(m.secretData.Data))
	for k, v := range m.secretData.Data {
		data[k] = v
	}
	client := m.client

	return func() tea.Msg {
		err := client.UpdateSecretData(ctx, ns, name, data)
		return secretSavedMsg{err: err}
	}
}

// loadConfigMapData fetches configmap data for the configmap editor.
func (m Model) loadConfigMapData() tea.Cmd {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return nil
	}

	kctx := m.nav.Context
	ns := m.resolveNamespace()
	if sel.Namespace != "" {
		ns = sel.Namespace
	}
	name := sel.Name
	client := m.client
	reqCtx := m.reqCtx

	return func() tea.Msg {
		data, err := client.GetConfigMapData(reqCtx, kctx, ns, name)
		return configMapDataLoadedMsg{data: data, err: err}
	}
}

// saveConfigMapData saves the modified configmap data back to the cluster.
func (m Model) saveConfigMapData() tea.Cmd {
	if m.configMapData == nil {
		return nil
	}

	ctx := m.nav.Context
	ns := m.resolveNamespace()
	sel := m.selectedMiddleItem()
	if sel == nil {
		return nil
	}
	if sel.Namespace != "" {
		ns = sel.Namespace
	}
	name := sel.Name
	data := make(map[string]string, len(m.configMapData.Data))
	for k, v := range m.configMapData.Data {
		data[k] = v
	}
	client := m.client

	return func() tea.Msg {
		err := client.UpdateConfigMapData(ctx, ns, name, data)
		return configMapSavedMsg{err: err}
	}
}

// loadLabelData fetches labels and annotations for the selected resource.
func (m Model) loadLabelData() tea.Cmd {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return nil
	}
	kctx := m.nav.Context
	ns := m.resolveNamespace()
	if sel.Namespace != "" {
		ns = sel.Namespace
	}
	name := sel.Name
	rt := m.labelResourceType
	client := m.client
	reqCtx := m.reqCtx

	return func() tea.Msg {
		data, err := client.GetLabelAnnotationData(reqCtx, kctx, rt, ns, name)
		return labelDataLoadedMsg{data: data, err: err}
	}
}

// saveLabelData saves modified labels and annotations.
func (m Model) saveLabelData() tea.Cmd {
	if m.labelData == nil {
		return nil
	}
	sel := m.selectedMiddleItem()
	if sel == nil {
		return nil
	}
	kctx := m.nav.Context
	ns := m.resolveNamespace()
	if sel.Namespace != "" {
		ns = sel.Namespace
	}
	name := sel.Name
	rt := m.labelResourceType
	labels := make(map[string]string, len(m.labelData.Labels))
	for k, v := range m.labelData.Labels {
		labels[k] = v
	}
	annotations := make(map[string]string, len(m.labelData.Annotations))
	for k, v := range m.labelData.Annotations {
		annotations[k] = v
	}
	client := m.client

	return func() tea.Msg {
		err := client.UpdateLabelAnnotationData(context.Background(), kctx, rt, ns, name, labels, annotations)
		return labelSavedMsg{err: err}
	}
}

// loadRevisions fetches the revision history for a deployment.
func (m Model) loadRevisions() tea.Cmd {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return nil
	}
	kctx := m.nav.Context
	ns := m.resolveNamespace()
	if sel.Namespace != "" {
		ns = sel.Namespace
	}
	name := sel.Name
	client := m.client
	reqCtx := m.reqCtx

	return func() tea.Msg {
		revs, err := client.GetDeploymentRevisions(reqCtx, kctx, ns, name)
		return revisionListMsg{revisions: revs, err: err}
	}
}

// loadHelmRevisions fetches the revision history for a Helm release.
func (m Model) loadHelmRevisions() tea.Cmd {
	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return func() tea.Msg {
			return helmRevisionListMsg{err: fmt.Errorf("helm not found: %w", err)}
		}
	}

	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	kubeconfigPaths := m.client.KubeconfigPaths()

	return func() tea.Msg {
		args := []string{"history", name, "-n", ns, "--kube-context", ctx, "-o", "json", "--max", "50"}
		cmd := exec.Command(helmPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
		logger.Info("Running helm command", "cmd", cmd.String())
		output, cmdErr := cmd.CombinedOutput()
		if cmdErr != nil {
			logger.Error("helm history failed", "cmd", cmd.String(), "error", cmdErr, "output", string(output))
			return helmRevisionListMsg{err: fmt.Errorf("%w: %s", cmdErr, strings.TrimSpace(string(output)))}
		}

		var entries []struct {
			Revision    int    `json:"revision"`
			Status      string `json:"status"`
			Chart       string `json:"chart"`
			AppVersion  string `json:"app_version"`
			Description string `json:"description"`
			Updated     string `json:"updated"`
		}
		if jsonErr := json.Unmarshal(output, &entries); jsonErr != nil {
			return helmRevisionListMsg{err: fmt.Errorf("failed to parse helm history: %w", jsonErr)}
		}

		revisions := make([]ui.HelmRevision, len(entries))
		for i, e := range entries {
			revisions[i] = ui.HelmRevision{
				Revision:    e.Revision,
				Status:      e.Status,
				Chart:       e.Chart,
				AppVersion:  e.AppVersion,
				Description: e.Description,
				Updated:     e.Updated,
			}
		}
		// Reverse so newest revision is first.
		for i, j := 0, len(revisions)-1; i < j; i, j = i+1, j-1 {
			revisions[i], revisions[j] = revisions[j], revisions[i]
		}
		return helmRevisionListMsg{revisions: revisions}
	}
}

// loadPreview loads the right column based on the current level and selection.
func (m Model) loadPreview() tea.Cmd {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return nil
	}

	var cmds []tea.Cmd

	switch m.nav.Level {
	case model.LevelClusters:
		return m.loadResourceTypes()
	case model.LevelResourceTypes:
		sel := m.selectedMiddleItem()
		if sel != nil && sel.Extra == "__overview__" {
			if ui.ConfigDashboard {
				return m.loadDashboard()
			}
			return nil
		}
		if sel != nil && sel.Extra == "__monitoring__" {
			return m.loadMonitoringDashboard()
		}
		// Collapsed group placeholder: no preview to load.
		if sel != nil && sel.Kind == "__collapsed_group__" {
			return nil
		}
		// Port forwards: show active port forwards in preview.
		if sel != nil && sel.Kind == "__port_forwards__" {
			items := m.portForwardItems()
			return func() tea.Msg {
				return resourcesLoadedMsg{items: items, forPreview: true}
			}
		}
		return m.loadResources(true)
	case model.LevelResources:
		// Port forwards list: no sub-resources to load.
		if m.nav.ResourceType.Kind == "__port_forwards__" {
			return nil
		}
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
		// Load metrics for pods and workload controllers.
		kind := m.nav.ResourceType.Kind
		if kind == "Pod" || kind == "Deployment" || kind == "StatefulSet" || kind == "DaemonSet" {
			if metricsCmd := m.loadMetrics(); metricsCmd != nil {
				cmds = append(cmds, metricsCmd)
			}
		}
		// Load events for the preview pane.
		if eventsCmd := m.loadPreviewEvents(); eventsCmd != nil {
			cmds = append(cmds, eventsCmd)
		}
		if len(cmds) == 0 {
			return nil
		}
		return tea.Batch(cmds...)
	case model.LevelOwned:
		if sel.Kind == "Pod" {
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
		// Items with owned children show those children in the right
		// column on navigation; skip loading a YAML preview so it
		// doesn't appear when popping back via ownedParentStack.
		if kindHasOwnedChildren(sel.Kind) {
			return nil
		}
		if m.fullYAMLPreview {
			cmds = append(cmds, m.loadPreviewYAML())
			return tea.Batch(cmds...)
		}
		name := sel.Name
		kctx := m.nav.Context
		ns := m.namespace
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
	case model.LevelContainers:
		return nil
	}
	return nil
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
	return func() tea.Msg {
		rules, err := client.GetSelfRulesAs(context.Background(), ctx, ns, subject)
		return canILoadedMsg{rules: rules, err: err}
	}
}

func (m Model) loadCanISAList() tea.Cmd {
	client := m.client
	ctx := m.nav.Context
	// Always list SAs across all namespaces so the user can check
	// permissions for any service account regardless of the current view.
	return func() tea.Msg {
		accounts, err := client.ListServiceAccounts(context.Background(), ctx, "")
		return canISAListMsg{accounts: accounts, err: err}
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
