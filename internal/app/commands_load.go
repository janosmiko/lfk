package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/app/bgtasks"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// helmHistoryTimeout caps how long the helm history subprocess may run before
// being killed. Prevents the loader goroutine from leaking when helm hangs on
// an unresponsive API server.
const helmHistoryTimeout = 30 * time.Second

// helmErrOutputCap bounds how much of helm's stderr/stdout we propagate into
// error messages and logs. Helm can emit multi-kilobyte NOTES.txt content and
// values snippets that would otherwise bloat logs and the UI error overlay.
const helmErrOutputCap = 512

// truncateHelmOutput trims helm subprocess output to helmErrOutputCap bytes so
// logs and UI error messages stay bounded.
func truncateHelmOutput(out []byte) string {
	s := strings.TrimSpace(string(out))
	if len(s) > helmErrOutputCap {
		return s[:helmErrOutputCap] + "...(truncated)"
	}
	return s
}

// --- Commands ---

func (m Model) loadContexts() tea.Cmd {
	return m.trackBgTask(
		bgtasks.KindResourceList,
		"List contexts",
		"",
		func() tea.Msg {
			items, err := m.client.GetContexts()
			return contextsLoadedMsg{items: items, err: err}
		},
	)
}

func (m Model) loadResourceTypes() tea.Cmd {
	kctx := m.nav.Context
	discovered := m.discoveredResources[kctx]
	var items []model.Item
	if len(discovered) > 0 {
		items = model.BuildSidebarItems(discovered)
	} else {
		items = model.BuildSidebarItems(model.SeedResources())
	}
	return func() tea.Msg {
		return resourceTypesMsg{items: items}
	}
}

// discoverAPIResources launches async API resource discovery for the given
// context via Client.DiscoverAPIResources. Uses context.Background() instead
// of m.reqCtx because discovery should not be cancelled by navigation
// actions (cancelAndReset). Results are delivered via apiResourceDiscoveryMsg.
func (m Model) discoverAPIResources(contextName string) tea.Cmd {
	client := m.client
	return m.trackBgTask(
		bgtasks.KindResourceList,
		"Discover API resources",
		contextName,
		func() tea.Msg {
			entries, err := client.DiscoverAPIResources(context.Background(), contextName)
			return apiResourceDiscoveryMsg{context: contextName, entries: entries, err: err}
		},
	)
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
	silent := m.suppressBgtasks
	reqCtx := m.reqCtx
	if forPreview {
		sel := m.selectedMiddleItem()
		if sel == nil {
			return nil
		}
		found, ok := model.FindResourceTypeIn(sel.Extra, m.discoveredResources[kctx])
		if !ok {
			return nil
		}
		rt = found
	}
	return m.trackBgTask(
		bgtasks.KindResourceList,
		"List "+model.DisplayNameFor(rt),
		bgtaskTarget(kctx, ns),
		func() tea.Msg {
			items, err := m.client.GetResources(reqCtx, kctx, ns, rt)
			return resourcesLoadedMsg{items: items, err: err, forPreview: forPreview, gen: gen, silent: silent}
		},
	)
}

func (m Model) loadOwned(forPreview bool) tea.Cmd {
	kctx := m.nav.Context
	ns := m.effectiveNamespace()
	kind := m.nav.ResourceType.Kind
	name := m.nav.ResourceName
	gen := m.requestGen
	silent := m.suppressBgtasks
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
	return m.trackBgTask(
		bgtasks.KindResourceList,
		"List "+kind+" children",
		bgtaskTarget(kctx, ns),
		func() tea.Msg {
			items, err := m.client.GetOwnedResources(reqCtx, kctx, ns, kind, name)
			return ownedLoadedMsg{items: items, err: err, forPreview: forPreview, gen: gen, silent: silent}
		},
	)
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

	return m.trackBgTask(
		bgtasks.KindResourceTree,
		"Resource tree: "+kind+"/"+name,
		bgtaskTarget(kctx, ns),
		func() tea.Msg {
			tree, err := m.client.GetResourceTree(reqCtx, kctx, ns, kind, name)
			return resourceTreeLoadedMsg{tree: tree, err: err, gen: gen}
		},
	)
}

func (m Model) loadContainers(forPreview bool) tea.Cmd {
	kctx := m.nav.Context
	ns := m.effectiveNamespace()
	gen := m.requestGen
	silent := m.suppressBgtasks
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
	taskTarget := bgtaskTarget(kctx, ns)
	if podName != "" {
		taskTarget = taskTarget + " / " + podName
	}
	return m.trackBgTask(
		bgtasks.KindContainers,
		"List containers",
		taskTarget,
		func() tea.Msg {
			items, err := m.client.GetContainers(reqCtx, kctx, ns, podName)
			return containersLoadedMsg{items: items, err: err, forPreview: forPreview, gen: gen, silent: silent}
		},
	)
}

func (m Model) loadNamespaces() tea.Cmd {
	if m.client == nil {
		return nil
	}
	kctx := m.activeContext()
	// Use an independent context so namespace loading is never blocked or
	// cancelled by in-flight resource requests.
	return func() tea.Msg {
		items, err := m.client.GetNamespaces(context.Background(), kctx)
		return namespacesLoadedMsg{context: kctx, items: items, err: err}
	}
}

// resolveOwnedResourceType determines the correct ResourceTypeEntry for an
// owned item at LevelOwned. It uses the item's Kind to look up the type in
// both built-in resource types and discovered CRDs. If the kind is not found,
// it falls back to constructing a ResourceTypeEntry from the item's Extra
// metadata (which may contain "group/version" from ArgoCD status) and the
// Kind. Returns false if the type cannot be resolved.
//
// When sel.Extra carries a "group/version" hint (e.g. helm manifest items
// expose the rendered apiVersion there), the lookup is biased toward the
// matching APIGroup so that two CRDs sharing the same Kind name but living
// in different API groups (e.g. VaultDynamicSecret in secrets.hashicorp.com
// vs. generators.external-secrets.io) resolve to the right CRD instead of
// whichever one was iterated first.
func (m Model) resolveOwnedResourceType(sel *model.Item) (model.ResourceTypeEntry, bool) {
	if sel == nil {
		return model.ResourceTypeEntry{}, false
	}
	kctx := m.nav.Context
	crds := m.discoveredResources[kctx]

	// If Extra carries an apiVersion ("group/version"), prefer matching by
	// Kind+APIGroup so duplicate Kind names across API groups resolve to
	// the right CRD. Core types (Extra="v1") have no group component and
	// fall through to the Kind-only lookup below.
	if sel.Extra != "" && sel.Kind != "" {
		parts := strings.SplitN(sel.Extra, "/", 2)
		if len(parts) == 2 && parts[0] != "" {
			if rt, ok := model.FindResourceTypeByKindAndGroup(sel.Kind, parts[0], crds); ok {
				return rt, true
			}
		}
	}

	// Try to find by Kind in built-in types and discovered CRDs.
	if rt, ok := model.FindResourceTypeByKind(sel.Kind, crds); ok {
		return rt, true
	}

	// If Extra contains a full resource ref string ("group/version/resource"), use it.
	if sel.Extra != "" {
		if rt, ok := model.FindResourceTypeIn(sel.Extra, crds); ok {
			return rt, true
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
		return m.trackBgTask(
			bgtasks.KindYAMLFetch,
			"YAML: "+name,
			bgtaskTarget(kctx, itemNs),
			func() tea.Msg {
				content, err := m.client.GetResourceYAML(reqCtx, kctx, itemNs, rt, name)
				return buildYAMLLoadedMsg(content, err)
			},
		)
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
		taskTarget := bgtaskTarget(kctx, itemNs)
		if sel.Kind == "Pod" {
			return m.trackBgTask(
				bgtasks.KindYAMLFetch,
				"YAML: "+name,
				taskTarget,
				func() tea.Msg {
					content, err := m.client.GetPodYAML(reqCtx, kctx, itemNs, name)
					return buildYAMLLoadedMsg(content, err)
				},
			)
		}
		rt, ok := m.resolveOwnedResourceType(sel)
		if !ok {
			return func() tea.Msg {
				return buildYAMLLoadedMsg("", fmt.Errorf("unknown resource type: %s", sel.Kind))
			}
		}
		return m.trackBgTask(
			bgtasks.KindYAMLFetch,
			"YAML: "+name,
			taskTarget,
			func() tea.Msg {
				content, err := m.client.GetResourceYAML(reqCtx, kctx, itemNs, rt, name)
				return buildYAMLLoadedMsg(content, err)
			},
		)
	case model.LevelContainers:
		podName := m.nav.OwnedName
		return m.trackBgTask(
			bgtasks.KindYAMLFetch,
			"YAML: "+podName,
			bgtaskTarget(kctx, ns),
			func() tea.Msg {
				content, err := m.client.GetPodYAML(reqCtx, kctx, ns, podName)
				return buildYAMLLoadedMsg(content, err)
			},
		)
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
		return m.trackBgTask(
			bgtasks.KindMetrics,
			"Metrics: Pod/"+podName,
			bgtaskTarget(kctx, ns),
			func() tea.Msg {
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
			},
		)
	case "Deployment", "StatefulSet", "DaemonSet":
		name := sel.Name
		return m.trackBgTask(
			bgtasks.KindMetrics,
			"Metrics: "+kind+"/"+name,
			bgtaskTarget(kctx, ns),
			func() tea.Msg {
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
			},
		)
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

	return m.trackBgTask(
		bgtasks.KindResourceList,
		"Preview events: "+name,
		bgtaskTarget(kctx, ns),
		func() tea.Msg {
			events, err := client.GetResourceEvents(context.Background(), kctx, ns, name, kind)
			if err != nil {
				return previewEventsLoadedMsg{gen: gen}
			}
			return previewEventsLoadedMsg{events: events, gen: gen}
		},
	)
}

// loadPodMetricsForList fetches metrics for all pods in the current namespace
// and returns them to enrich the middle pane items.
func (m Model) loadPodMetricsForList() tea.Cmd {
	kctx := m.nav.Context
	ns := m.effectiveNamespace()
	gen := m.requestGen
	client := m.client
	reqCtx := m.reqCtx
	return m.trackBgTask(
		bgtasks.KindMetrics,
		"Pod metrics",
		bgtaskTarget(kctx, ns),
		func() tea.Msg {
			metrics, err := client.GetAllPodMetrics(reqCtx, kctx, ns)
			if err != nil {
				return podMetricsEnrichedMsg{gen: gen} // silently ignore
			}
			return podMetricsEnrichedMsg{metrics: metrics, gen: gen}
		},
	)
}

// loadNodeMetricsForList fetches metrics for all nodes and returns them
// to enrich the middle pane items with CPU/MEM usage columns.
func (m Model) loadNodeMetricsForList() tea.Cmd {
	kctx := m.nav.Context
	gen := m.requestGen
	client := m.client
	reqCtx := m.reqCtx
	return m.trackBgTask(
		bgtasks.KindMetrics,
		"Node metrics",
		bgtaskTarget(kctx, ""),
		func() tea.Msg {
			metrics, err := client.GetAllNodeMetrics(reqCtx, kctx)
			if err != nil {
				return nodeMetricsEnrichedMsg{gen: gen}
			}
			return nodeMetricsEnrichedMsg{metrics: metrics, gen: gen}
		},
	)
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

// fetchHelmHistory shells out to `helm history` and returns the parsed
// revisions (newest first). It is shared between the rollback and read-only
// history overlays so both views use the same data source. The subprocess is
// bounded by helmHistoryTimeout and its error output is truncated before
// being logged or propagated to the UI.
func fetchHelmHistory(helmPath, name, ns, kubeCtx, kubeconfigPaths string) ([]ui.HelmRevision, error) {
	ctx, cancel := context.WithTimeout(context.Background(), helmHistoryTimeout)
	defer cancel()

	args := []string{"history", name, "-n", ns, "--kube-context", kubeCtx, "-o", "json", "--max", "50"}
	cmd := exec.CommandContext(ctx, helmPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
	logger.Info("Running helm command", "cmd", cmd.String())
	output, cmdErr := cmd.CombinedOutput()
	if cmdErr != nil {
		truncated := truncateHelmOutput(output)
		logger.Error("helm history failed", "cmd", cmd.String(), "error", cmdErr, "output", truncated)
		return nil, fmt.Errorf("%w: %s", cmdErr, truncated)
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
		return nil, fmt.Errorf("failed to parse helm history: %w", jsonErr)
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
	return revisions, nil
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
		revisions, err := fetchHelmHistory(helmPath, name, ns, ctx, kubeconfigPaths)
		if err != nil {
			return helmRevisionListMsg{err: err}
		}
		return helmRevisionListMsg{revisions: revisions}
	}
}

// loadHelmHistory fetches revision history for the read-only history overlay.
// It reuses fetchHelmHistory but routes the result to helmHistoryListMsg so
// the update path opens overlayHelmHistory instead of overlayHelmRollback.
func (m Model) loadHelmHistory() tea.Cmd {
	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return func() tea.Msg {
			return helmHistoryListMsg{err: fmt.Errorf("helm not found: %w", err)}
		}
	}

	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	kubeconfigPaths := m.client.KubeconfigPaths()

	return func() tea.Msg {
		revisions, err := fetchHelmHistory(helmPath, name, ns, ctx, kubeconfigPaths)
		if err != nil {
			return helmHistoryListMsg{err: err}
		}
		return helmHistoryListMsg{revisions: revisions}
	}
}

// bgtaskTarget formats a context+namespace pair for the :tasks overlay's
// Target column. Falls back gracefully when either part is empty.
func bgtaskTarget(kctx, ns string) string {
	switch {
	case kctx != "" && ns != "":
		return kctx + " / " + ns
	case kctx != "":
		return kctx
	case ns != "":
		return ns
	default:
		return ""
	}
}

// trackBgTask wraps a loader's inner closure with bgtasks Start/Finish.
// It calls Start SYNCHRONOUSLY (while Update is still building the Cmd
// return value), so the very next View() frame already sees the task in
// the registry. If Start were inside the returned closure instead, it
// would only run after the goroutine that executes the Cmd begins —
// which is after View() has already rendered that frame, so the user
// would never see the indicator for sub-frame loads.
//
// The deferred Finish still runs inside the goroutine, so it correctly
// fires on success, error, panic, or context cancellation.
//
// Pass a nil inner to skip tracking entirely (for loaders whose early
// return paths don't dispatch any work).
func (m Model) trackBgTask(kind bgtasks.Kind, name, target string, inner func() tea.Msg) tea.Cmd {
	if inner == nil {
		return nil
	}
	registry := m.bgtasks
	var id uint64
	if m.suppressBgtasks {
		id = registry.StartUntracked()
	} else {
		id = registry.Start(kind, name, target)
	}
	return func() tea.Msg {
		defer registry.Finish(id)
		return inner()
	}
}
