package app

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// --- Messages ---

type stderrCapturedMsg struct {
	message string
}

type contextsLoadedMsg struct {
	items []model.Item
	err   error
}

type resourceTypesMsg struct {
	items []model.Item
}

type resourcesLoadedMsg struct {
	items      []model.Item
	err        error
	forPreview bool
	gen        uint64
}

type ownedLoadedMsg struct {
	items      []model.Item
	err        error
	forPreview bool
	gen        uint64
}

type containersLoadedMsg struct {
	items      []model.Item
	err        error
	forPreview bool
	gen        uint64
}

type resourceTreeLoadedMsg struct {
	tree *model.ResourceNode
	err  error
	gen  uint64
}

type namespacesLoadedMsg struct {
	items []model.Item
	err   error
}

type yamlLoadedMsg struct {
	content string
	err     error
}

// previewYAMLLoadedMsg carries YAML content for the split/full preview in the right column.
type previewYAMLLoadedMsg struct {
	content string
	err     error
	gen     uint64
}

// actionResultMsg is returned after an action completes.
type actionResultMsg struct {
	message string
	err     error
}

// triggerCronJobMsg carries the result of triggering a CronJob.
type triggerCronJobMsg struct {
	jobName string
	err     error
}

// statusMessageExpiredMsg clears the status message after a timeout.
type statusMessageExpiredMsg struct{}

// watchTickMsg triggers a periodic refresh in watch mode.
type watchTickMsg struct{}

// containerSelectMsg carries the container list for action container selection.
type containerSelectMsg struct {
	items []model.Item
	err   error
}

// dashboardLoadedMsg carries the rendered dashboard content.
type dashboardLoadedMsg struct {
	content string
	context string
}

// monitoringDashboardMsg carries the rendered monitoring dashboard content.
type monitoringDashboardMsg struct {
	content string
	context string
}

// yamlClipboardMsg carries YAML content to be copied to clipboard.
type yamlClipboardMsg struct {
	content string
	err     error
}

// exportDoneMsg carries the result of exporting a resource to a file.
type exportDoneMsg struct {
	path string
	err  error
}

// logLineMsg carries a single line of log output from kubectl.
type logLineMsg struct {
	line string
	done bool           // true when the log stream has ended
	ch   chan string     // the channel this line came from (for tab identity)
}

// podSelectMsg carries the pod list for exec/attach pod selection on parent resources.
type podSelectMsg struct {
	items []model.Item
	err   error
}

// podLogSelectMsg carries the pod list for log pod selection on parent resources.
type podLogSelectMsg struct {
	items []model.Item
	err   error
}

// describeLoadedMsg carries the output of kubectl describe.
type describeLoadedMsg struct {
	content string
	title   string
	err     error
}

// diffLoadedMsg carries the YAML content of two resources for side-by-side comparison.
type diffLoadedMsg struct {
	left      string
	right     string
	leftName  string
	rightName string
	err       error
}

// execRetryShMsg is sent when bash exec fails, to retry with sh.
type execRetryShMsg struct{}

// bulkActionResultMsg is returned after a bulk action completes.
type bulkActionResultMsg struct {
	succeeded int
	failed    int
	errors    []string
}

// commandBarResultMsg carries the result of a command bar execution.
type commandBarResultMsg struct {
	output string
	err    error
}

// quotaLoadedMsg carries quota data for the namespace quota dashboard.
type quotaLoadedMsg struct {
	quotas []k8s.QuotaInfo
	err    error
}

// crdDiscoveryMsg carries the result of CRD discovery for a cluster context.
type crdDiscoveryMsg struct {
	context string
	entries []model.ResourceTypeEntry
	err     error
}

// metricsLoadedMsg carries resource usage metrics for a pod or set of pods.
type metricsLoadedMsg struct {
	cpuUsed int64
	cpuReq  int64
	cpuLim  int64
	memUsed int64
	memReq  int64
	memLim  int64
	gen     uint64
}

// podMetricsEnrichedMsg carries pod metrics to enrich the middle pane items.
type podMetricsEnrichedMsg struct {
	metrics map[string]model.PodMetrics // pod name (or ns/name) -> metrics
	gen     uint64
}

type nodeMetricsEnrichedMsg struct {
	metrics map[string]model.PodMetrics // node name -> metrics
	gen     uint64
}

// secretDataLoadedMsg carries the fetched secret data.
type secretDataLoadedMsg struct {
	data *model.SecretData
	err  error
}

// secretSavedMsg carries the result of saving secret data.
type secretSavedMsg struct {
	err error
}

// configMapDataLoadedMsg carries the fetched configmap data.
type configMapDataLoadedMsg struct {
	data *model.ConfigMapData
	err  error
}

// configMapSavedMsg carries the result of saving configmap data.
type configMapSavedMsg struct {
	err error
}

// revisionListMsg carries the list of deployment revisions.
type revisionListMsg struct {
	revisions []k8s.DeploymentRevision
	err       error
}

// rollbackDoneMsg carries the result of a rollback operation.
type rollbackDoneMsg struct {
	err error
}

// labelDataLoadedMsg carries fetched label/annotation data.
type labelDataLoadedMsg struct {
	data *model.LabelAnnotationData
	err  error
}

// labelSavedMsg carries the result of saving labels/annotations.
type labelSavedMsg struct {
	err error
}

// helmRevisionListMsg carries the list of Helm release revisions.
type helmRevisionListMsg struct {
	revisions []ui.HelmRevision
	err       error
}

// helmRollbackDoneMsg carries the result of a Helm rollback operation.
type helmRollbackDoneMsg struct {
	err error
}

// helmValuesLoadedMsg carries the fetched Helm release values.
type helmValuesLoadedMsg struct {
	content string // YAML values output
	title   string
	err     error
}

// containerPortsLoadedMsg carries discovered container/service ports.
type containerPortsLoadedMsg struct {
	ports []k8s.ContainerPort
	err   error
}

// portForwardStartedMsg is sent after a port forward has been started.
type portForwardStartedMsg struct {
	id         int
	localPort  string
	remotePort string
	err        error
}

// portForwardStoppedMsg is sent after a port forward has been stopped.
type portForwardStoppedMsg struct {
	id  int
	err error
}

// portForwardUpdateMsg is sent when port forward state changes (process exits, etc).
type portForwardUpdateMsg struct{}

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
	} else if m.allNamespaces {
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
	if m.allNamespaces {
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
	if m.allNamespaces && m.nav.Namespace != "" {
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

// loadDashboard fetches cluster summary data and renders a dashboard.
// renderBar renders a horizontal bar graph like [████████░░░░░░░░] 52%.
// The filled portion is colored based on usage percentage: green (<75%), orange (75-90%), red (>90%).
func renderBar(used, total int64, width int) string {
	if total <= 0 {
		return "[" + strings.Repeat("\u2591", width) + "] N/A"
	}
	pct := float64(used) / float64(total) * 100
	if pct > 100 {
		pct = 100
	}
	filled := int(pct / 100 * float64(width))
	if filled > width {
		filled = width
	}
	empty := width - filled

	filledStr := strings.Repeat("\u2588", filled)
	emptyStr := strings.Repeat("\u2591", empty)

	var style lipgloss.Style
	switch {
	case pct >= 90:
		style = ui.StatusFailed
	case pct >= 75:
		style = ui.StatusPending
	default:
		style = ui.StatusRunning
	}

	return "[" + style.Render(filledStr) + emptyStr + "] " + fmt.Sprintf("%.0f%%", pct)
}

// renderStackedBar renders a stacked bar showing proportions of multiple segments.
func renderStackedBar(segments []struct{ count int; style lipgloss.Style }, total, width int) string {
	if total <= 0 {
		return "[" + strings.Repeat("\u2591", width) + "]"
	}
	bar := ""
	used := 0
	for i, seg := range segments {
		chars := int(float64(seg.count) / float64(total) * float64(width))
		// Last segment gets remaining chars to avoid rounding issues.
		if i == len(segments)-1 {
			chars = width - used
		}
		if chars < 0 {
			chars = 0
		}
		if used+chars > width {
			chars = width - used
		}
		bar += seg.style.Render(strings.Repeat("\u2588", chars))
		used += chars
	}
	if used < width {
		bar += strings.Repeat("\u2591", width-used)
	}
	return "[" + bar + "]"
}

func (m Model) loadDashboard() tea.Cmd {
	kctx := m.nav.Context
	client := m.client
	reqCtx := m.reqCtx
	return func() tea.Msg {
		var lines []string
		lines = append(lines, "")

		// Fetch nodes.
		nodeItems, err := client.GetResources(reqCtx, kctx, "", model.ResourceTypeEntry{
			Kind: "Node", APIGroup: "", APIVersion: "v1", Resource: "nodes", Namespaced: false,
		})
		nodeCount := 0
		readyNodes := 0
		if err == nil {
			nodeCount = len(nodeItems)
			for _, n := range nodeItems {
				if n.Status == "Ready" {
					readyNodes++
				}
			}
		}

		// Fetch all pods across namespaces.
		podItems, err := client.GetResources(reqCtx, kctx, "", model.ResourceTypeEntry{
			Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true,
		})
		podCount := 0
		runningPods := 0
		failedPods := 0
		pendingPods := 0
		crashLoopPods := 0
		if err == nil {
			podCount = len(podItems)
			for _, p := range podItems {
				switch p.Status {
				case "Running":
					runningPods++
				case "CrashLoopBackOff":
					failedPods++
					crashLoopPods++
				case "Failed", "Error", "ImagePullBackOff", "ErrImagePull", "OOMKilled":
					failedPods++
				case "Pending", "ContainerCreating", "Init":
					pendingPods++
				}
			}
		}

		// Fetch namespaces.
		namespaces, _ := client.GetNamespaces(reqCtx, kctx)
		nsCount := len(namespaces)

		// Fetch warning events.
		eventItems, _ := client.GetResources(reqCtx, kctx, "", model.ResourceTypeEntry{
			Kind: "Event", APIGroup: "", APIVersion: "v1", Resource: "events", Namespaced: true,
		})
		var warningEvents []model.Item
		for _, e := range eventItems {
			if e.Status == "Warning" {
				warningEvents = append(warningEvents, e)
			}
		}
		// Sort by creation time (most recent first) and limit to 10.
		sort.Slice(warningEvents, func(i, j int) bool {
			return warningEvents[i].CreatedAt.After(warningEvents[j].CreatedAt)
		})
		if len(warningEvents) > 10 {
			warningEvents = warningEvents[:10]
		}

		// Fetch PodDisruptionBudgets to detect violations.
		type pdbWarning struct {
			name               string
			namespace          string
			minAvailable       string
			currentHealthy     string
			disruptionsAllowed string
		}
		var pdbWarnings []pdbWarning
		pdbItems, pdbErr := client.GetResources(reqCtx, kctx, "", model.ResourceTypeEntry{
			Kind: "PodDisruptionBudget", APIGroup: "policy", APIVersion: "v1", Resource: "poddisruptionbudgets", Namespaced: true,
		})
		if pdbErr == nil {
			for _, pdb := range pdbItems {
				var minAvail, currentHealthy, disruptionsAllowed string
				var disruptionsVal int64 = -1
				var currentVal int64 = -1
				var minAvailVal int64 = -1
				for _, kv := range pdb.Columns {
					switch kv.Key {
					case "Min Available":
						minAvail = kv.Value
						// Try to parse as integer; percentage values won't parse.
						if v, err := strconv.ParseInt(kv.Value, 10, 64); err == nil {
							minAvailVal = v
						}
					case "Current Healthy":
						currentHealthy = kv.Value
						if v, err := strconv.ParseInt(kv.Value, 10, 64); err == nil {
							currentVal = v
						}
					case "Disruptions Allowed":
						disruptionsAllowed = kv.Value
						if v, err := strconv.ParseInt(kv.Value, 10, 64); err == nil {
							disruptionsVal = v
						}
					}
				}
				// Flag PDBs where no disruptions are allowed or healthy pods are at/below minimum.
				atRisk := disruptionsVal == 0
				if !atRisk && minAvailVal >= 0 && currentVal >= 0 {
					atRisk = currentVal <= minAvailVal
				}
				if atRisk {
					pdbWarnings = append(pdbWarnings, pdbWarning{
						name:               pdb.Name,
						namespace:          pdb.Namespace,
						minAvailable:       minAvail,
						currentHealthy:     currentHealthy,
						disruptionsAllowed: disruptionsAllowed,
					})
				}
			}
		}

		// Node metrics: per-node and totals.
		nodeMetrics, _ := client.GetAllNodeMetrics(reqCtx, kctx)
		type nodeInfo struct {
			name                             string
			cpuUsed, cpuAlloc, memUsed, memAlloc int64
		}
		var nodes []nodeInfo
		var totalCPUUsed, totalCPUAlloc, totalMemUsed, totalMemAlloc int64
		for _, ni := range nodeItems {
			info := nodeInfo{name: ni.Name}
			if nm, ok := nodeMetrics[ni.Name]; ok {
				info.cpuUsed = nm.CPU
				info.memUsed = nm.Memory
				totalCPUUsed += nm.CPU
				totalMemUsed += nm.Memory
			}
			for _, kv := range ni.Columns {
				switch kv.Key {
				case "CPU Alloc":
					v := ui.ParseResourceValue(kv.Value, true)
					info.cpuAlloc = v
					totalCPUAlloc += v
				case "Mem Alloc":
					v := ui.ParseResourceValue(kv.Value, false)
					info.memAlloc = v
					totalMemAlloc += v
				}
			}
			nodes = append(nodes, info)
		}

		// Build dashboard content.
		lines = append(lines, ui.DimStyle.Bold(true).Render("  CLUSTER OVERVIEW"))
		lines = append(lines, "")

		// Nodes section.
		nodeStatus := ui.StatusRunning.Render(fmt.Sprintf("%d Ready", readyNodes))
		if readyNodes < nodeCount {
			notReady := nodeCount - readyNodes
			nodeStatus += " " + ui.StatusFailed.Render(fmt.Sprintf("%d NotReady", notReady))
		}
		lines = append(lines, fmt.Sprintf("  %s %s  %s",
			ui.HelpKeyStyle.Render("Nodes:"),
			ui.NormalStyle.Render(fmt.Sprintf("%d", nodeCount)),
			nodeStatus))

		// Node readiness bar.
		if nodeCount > 0 {
			nodeBar := renderBar(int64(readyNodes), int64(nodeCount), 30)
			lines = append(lines, fmt.Sprintf("  %s %s",
				ui.HelpKeyStyle.Render("           "),
				nodeBar))
		}

		lines = append(lines, "")

		// Namespaces.
		lines = append(lines, fmt.Sprintf("  %s %s",
			ui.HelpKeyStyle.Render("Namespaces:"),
			ui.NormalStyle.Render(fmt.Sprintf("%d", nsCount))))

		lines = append(lines, "")
		lines = append(lines, ui.DimStyle.Render("  "+strings.Repeat("─", 50)))

		// Pods section.
		podStatus := ui.StatusRunning.Render(fmt.Sprintf("%d Running", runningPods))
		if failedPods > 0 {
			podStatus += " " + ui.StatusFailed.Render(fmt.Sprintf("%d Failed", failedPods))
		}
		if pendingPods > 0 {
			podStatus += " " + ui.StatusPending.Render(fmt.Sprintf("%d Pending", pendingPods))
		}
		lines = append(lines, fmt.Sprintf("  %s %s  %s",
			ui.HelpKeyStyle.Render("Pods:"),
			ui.NormalStyle.Render(fmt.Sprintf("%d", podCount)),
			podStatus))

		// Pod status stacked bar.
		if podCount > 0 {
			segments := []struct {
				count int
				style lipgloss.Style
			}{
				{runningPods, ui.StatusRunning},
				{pendingPods, ui.StatusPending},
				{failedPods, ui.StatusFailed},
			}
			podBar := renderStackedBar(segments, podCount, 30)
			lines = append(lines, fmt.Sprintf("  %s %s",
				ui.HelpKeyStyle.Render("           "),
				podBar))
		}

		lines = append(lines, "")
		lines = append(lines, ui.DimStyle.Render("  "+strings.Repeat("─", 50)))

		// Cluster resources.
		if totalCPUAlloc > 0 || totalMemAlloc > 0 {
			lines = append(lines, ui.DimStyle.Bold(true).Render("  CLUSTER RESOURCES"))
			lines = append(lines, "")
			if totalCPUAlloc > 0 {
				cpuBar := renderBar(totalCPUUsed, totalCPUAlloc, 30)
				lines = append(lines, fmt.Sprintf("  %s %s  %s / %s",
					ui.HelpKeyStyle.Render("CPU:"),
					cpuBar,
					ui.FormatCPU(totalCPUUsed),
					ui.FormatCPU(totalCPUAlloc)))
			}
			if totalMemAlloc > 0 {
				memBar := renderBar(totalMemUsed, totalMemAlloc, 30)
				lines = append(lines, fmt.Sprintf("  %s %s  %s / %s",
					ui.HelpKeyStyle.Render("Mem:"),
					memBar,
					ui.FormatMemory(totalMemUsed),
					ui.FormatMemory(totalMemAlloc)))
			}
			lines = append(lines, "")
			lines = append(lines, ui.DimStyle.Render("  "+strings.Repeat("─", 50)))
		}

		// Per-node breakdown.
		if len(nodes) > 0 && (totalCPUAlloc > 0 || totalMemAlloc > 0) {
			lines = append(lines, ui.DimStyle.Bold(true).Render("  NODES"))
			lines = append(lines, "")

			// Find max node name length for alignment.
			maxNameLen := 0
			for _, n := range nodes {
				if len(n.name) > maxNameLen {
					maxNameLen = len(n.name)
				}
			}
			if maxNameLen > 48 {
				maxNameLen = 48
			}

			for _, n := range nodes {
				name := n.name
				if len(name) > maxNameLen {
					name = name[:maxNameLen]
				}

				// Status indicator dot.
				statusDot := ui.StatusRunning.Render("●")
				for _, ni := range nodeItems {
					if ni.Name == n.name && ni.Status != "Ready" {
						statusDot = ui.StatusFailed.Render("●")
						break
					}
				}

				// Role info.
				role := ""
				for _, ni := range nodeItems {
					if ni.Name == n.name {
						for _, kv := range ni.Columns {
							if kv.Key == "Role" {
								role = kv.Value
								break
							}
						}
						break
					}
				}
				roleStr := ""
				if role != "" {
					roleStr = " " + ui.DimStyle.Render("["+role+"]")
				}

				cpuBar := renderBar(n.cpuUsed, n.cpuAlloc, 15)
				memBar := renderBar(n.memUsed, n.memAlloc, 15)
				// Node name on first line, bars on second line to avoid wrapping.
				lines = append(lines, fmt.Sprintf("  %s %s%s",
					statusDot, name, roleStr))
				lines = append(lines, fmt.Sprintf("      %s %s   %s %s",
					ui.HelpKeyStyle.Render("CPU"), cpuBar,
					ui.HelpKeyStyle.Render("MEM"), memBar))
			}
			lines = append(lines, "")
		}

		// Warnings.
		lines = append(lines, ui.DimStyle.Bold(true).Render("  WARNINGS"))
		lines = append(lines, "")
		hasWarnings := false
		if failedPods > 0 {
			lines = append(lines, ui.StatusFailed.Render(fmt.Sprintf("  ! %d pod(s) in failed state", failedPods)))
			hasWarnings = true
		}
		notReadyWorkerNodes := 0
		for _, ni := range nodeItems {
			if ni.Status != "Ready" {
				isControlPlane := false
				for _, kv := range ni.Columns {
					if kv.Key == "Role" && strings.Contains(kv.Value, "control-plane") {
						isControlPlane = true
						break
					}
				}
				if !isControlPlane {
					notReadyWorkerNodes++
				}
			}
		}
		if notReadyWorkerNodes > 0 {
			lines = append(lines, ui.StatusFailed.Render(fmt.Sprintf("  ! %d worker node(s) not ready", notReadyWorkerNodes)))
			hasWarnings = true
		}
		if crashLoopPods > 0 {
			lines = append(lines, ui.StatusFailed.Render(fmt.Sprintf("  ! %d pod(s) in CrashLoopBackOff", crashLoopPods)))
			hasWarnings = true
		}
		// PDB violation warnings.
		if len(pdbWarnings) > 0 {
			lines = append(lines, "")
			lines = append(lines, ui.DimStyle.Bold(true).Render("  PDB WARNINGS"))
			lines = append(lines, "")
			for _, pw := range pdbWarnings {
				lines = append(lines, fmt.Sprintf("  %s %s/%s",
					ui.StatusPending.Render("⊘"),
					ui.DimStyle.Render(pw.namespace),
					ui.StatusPending.Render(pw.name)))
				detail := fmt.Sprintf("       MinAvail=%s  Healthy=%s  DisruptionsAllowed=%s",
					pw.minAvailable, pw.currentHealthy, pw.disruptionsAllowed)
				lines = append(lines, ui.DimStyle.Render(detail))
			}
			hasWarnings = true
		}
		// Recent warning events.
		if len(warningEvents) > 0 {
			lines = append(lines, "")
			lines = append(lines, ui.DimStyle.Bold(true).Render("  RECENT WARNING EVENTS"))
			lines = append(lines, "")
			for _, ev := range warningEvents {
				reason := ""
				object := ""
				message := ""
				count := ""
				for _, kv := range ev.Columns {
					switch kv.Key {
					case "Reason":
						reason = kv.Value
					case "Object":
						object = kv.Value
					case "Message":
						msg := kv.Value
						if len(msg) > 60 {
							msg = msg[:57] + "..."
						}
						message = msg
					case "Count":
						count = kv.Value
					}
				}
				// Format: warning icon [Age] (xN) Reason: Object - Message
				countLabel := ""
				if count != "" && count != "1" {
					countLabel = ui.DimStyle.Render(fmt.Sprintf("(x%s) ", count))
				}
				line := fmt.Sprintf("  %s %s %s%s %s",
					ui.StatusPending.Render("⚠"),
					ui.DimStyle.Render(fmt.Sprintf("%-4s", ev.Age)),
					countLabel,
					ui.StatusFailed.Render(reason+":"),
					ui.NormalStyle.Render(object))
				lines = append(lines, line)
				if message != "" {
					lines = append(lines, fmt.Sprintf("       %s", ui.DimStyle.Render(message)))
				}
			}
			hasWarnings = true
		}
		if !hasWarnings {
			lines = append(lines, ui.StatusRunning.Render("  No warnings"))
		}

		lines = append(lines, "")

		return dashboardLoadedMsg{content: strings.Join(lines, "\n"), context: kctx}
	}
}

// loadMonitoringDashboard fetches active Prometheus alerts and renders a monitoring dashboard.
func (m Model) loadMonitoringDashboard() tea.Cmd {
	kctx := m.nav.Context
	client := m.client
	ns := m.resolveNamespace()
	return func() tea.Msg {
		var lines []string
		lines = append(lines, "")
		lines = append(lines, ui.DimStyle.Bold(true).Render("  MONITORING OVERVIEW"))
		lines = append(lines, "")

		// Fetch all active alerts with a timeout.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		alerts, err := client.GetAllActiveAlerts(ctx, kctx, ns)

		if err != nil {
			lines = append(lines, ui.DimStyle.Render("  Prometheus/Alertmanager not reachable"))
			lines = append(lines, ui.DimStyle.Render("  "+err.Error()))
			lines = append(lines, "")
			lines = append(lines, ui.DimStyle.Render("  Searched in well-known namespaces:"))
			lines = append(lines, ui.DimStyle.Render("  monitoring, prometheus, observability, kube-prometheus-stack"))
			lines = append(lines, "")
			return monitoringDashboardMsg{content: strings.Join(lines, "\n"), context: kctx}
		}

		// Summary counts.
		firing := 0
		pending := 0
		critical := 0
		warning := 0
		info := 0
		for _, a := range alerts {
			switch a.State {
			case "firing":
				firing++
			case "pending":
				pending++
			}
			switch a.Severity {
			case "critical":
				critical++
			case "warning":
				warning++
			default:
				info++
			}
		}

		totalAlerts := len(alerts)

		// Alert summary header.
		lines = append(lines, fmt.Sprintf("  %s %s",
			ui.HelpKeyStyle.Render("Alerts:"),
			ui.NormalStyle.Render(fmt.Sprintf("%d total", totalAlerts))))

		if totalAlerts == 0 {
			lines = append(lines, ui.StatusRunning.Render("  ✓ No active alerts"))
		} else {
			// State breakdown.
			stateStr := ""
			if firing > 0 {
				stateStr += ui.StatusFailed.Render(fmt.Sprintf("%d firing", firing))
			}
			if pending > 0 {
				if stateStr != "" {
					stateStr += "  "
				}
				stateStr += ui.StatusPending.Render(fmt.Sprintf("%d pending", pending))
			}
			if stateStr != "" {
				lines = append(lines, "           "+stateStr)
			}

			// Severity breakdown.
			sevStr := ""
			if critical > 0 {
				sevStr += ui.StatusFailed.Bold(true).Render(fmt.Sprintf("%d critical", critical))
			}
			if warning > 0 {
				if sevStr != "" {
					sevStr += "  "
				}
				sevStr += ui.StatusPending.Render(fmt.Sprintf("%d warning", warning))
			}
			if info > 0 {
				if sevStr != "" {
					sevStr += "  "
				}
				sevStr += ui.DimStyle.Render(fmt.Sprintf("%d info", info))
			}
			if sevStr != "" {
				lines = append(lines, "           "+sevStr)
			}
		}

		lines = append(lines, "")

		// Sort alerts: critical firing first, then warning firing, then pending, then info.
		sort.SliceStable(alerts, func(i, j int) bool {
			severityOrder := map[string]int{"critical": 0, "warning": 1, "info": 2}
			stateOrder := map[string]int{"firing": 0, "pending": 1}
			si := stateOrder[alerts[i].State]*10 + severityOrder[alerts[i].Severity]
			sj := stateOrder[alerts[j].State]*10 + severityOrder[alerts[j].Severity]
			return si < sj
		})

		// Critical alerts section.
		if critical > 0 {
			lines = append(lines, ui.StatusFailed.Bold(true).Render("  CRITICAL ALERTS"))
			lines = append(lines, "")
			for _, a := range alerts {
				if a.Severity != "critical" {
					continue
				}
				stateIcon := "●"
				stateStyle := ui.StatusFailed
				if a.State == "pending" {
					stateStyle = ui.StatusPending
				}

				header := fmt.Sprintf("  %s %s",
					stateStyle.Bold(true).Render(stateIcon),
					ui.StatusFailed.Bold(true).Render(a.Name))

				if a.State == "pending" {
					header += " " + ui.StatusPending.Render("[pending]")
				}

				lines = append(lines, header)

				if a.Summary != "" {
					summary := a.Summary
					if len(summary) > 80 {
						summary = summary[:77] + "..."
					}
					lines = append(lines, "    "+ui.DimStyle.Render(summary))
				} else if a.Description != "" {
					desc := a.Description
					if len(desc) > 80 {
						desc = desc[:77] + "..."
					}
					lines = append(lines, "    "+ui.DimStyle.Render(desc))
				}

				if !a.Since.IsZero() {
					lines = append(lines, "    "+ui.DimStyle.Render("since "+formatTimeAgo(a.Since)))
				}

				// Show relevant labels (namespace, pod, deployment, etc.)
				if len(a.Labels) > 0 {
					labelParts := monitoringAlertLabels(a)
					if len(labelParts) > 0 {
						lines = append(lines, "    "+strings.Join(labelParts, " "))
					}
				}

				if a.GrafanaURL != "" {
					lines = append(lines, "    "+ui.HelpKeyStyle.Render("dashboard: "+a.GrafanaURL))
				}

				lines = append(lines, "")
			}
		}

		// Warning alerts section.
		if warning > 0 {
			lines = append(lines, ui.StatusPending.Bold(true).Render("  WARNING ALERTS"))
			lines = append(lines, "")
			for _, a := range alerts {
				if a.Severity != "warning" {
					continue
				}
				stateIcon := "●"
				stateStyle := ui.StatusPending
				if a.State == "pending" {
					stateStyle = ui.StatusPending
				}

				lines = append(lines, fmt.Sprintf("  %s %s",
					stateStyle.Render(stateIcon),
					ui.StatusPending.Render(a.Name)))

				if a.Summary != "" {
					summary := a.Summary
					if len(summary) > 80 {
						summary = summary[:77] + "..."
					}
					lines = append(lines, "    "+ui.DimStyle.Render(summary))
				} else if a.Description != "" {
					desc := a.Description
					if len(desc) > 80 {
						desc = desc[:77] + "..."
					}
					lines = append(lines, "    "+ui.DimStyle.Render(desc))
				}

				if !a.Since.IsZero() {
					lines = append(lines, "    "+ui.DimStyle.Render("since "+formatTimeAgo(a.Since)))
				}

				if len(a.Labels) > 0 {
					labelParts := monitoringAlertLabels(a)
					if len(labelParts) > 0 {
						lines = append(lines, "    "+strings.Join(labelParts, " "))
					}
				}

				lines = append(lines, "")
			}
		}

		// Info alerts section.
		if info > 0 {
			lines = append(lines, ui.DimStyle.Bold(true).Render("  INFO ALERTS"))
			lines = append(lines, "")
			for _, a := range alerts {
				if a.Severity == "critical" || a.Severity == "warning" {
					continue
				}
				lines = append(lines, fmt.Sprintf("  %s %s",
					ui.DimStyle.Render("●"),
					ui.NormalStyle.Render(a.Name)))

				if a.Summary != "" {
					summary := a.Summary
					if len(summary) > 80 {
						summary = summary[:77] + "..."
					}
					lines = append(lines, "    "+ui.DimStyle.Render(summary))
				}

				lines = append(lines, "")
			}
		}

		lines = append(lines, "")
		return monitoringDashboardMsg{content: strings.Join(lines, "\n"), context: kctx}
	}
}

// formatTimeAgo formats a time.Time as a human-readable relative duration.
func formatTimeAgo(t time.Time) string {
	ago := time.Since(t)
	switch {
	case ago < time.Minute:
		return fmt.Sprintf("%ds ago", int(ago.Seconds()))
	case ago < time.Hour:
		return fmt.Sprintf("%dm ago", int(ago.Minutes()))
	case ago < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(ago.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(ago.Hours()/24))
	}
}

// monitoringAlertLabels extracts relevant labels from an alert for display.
func monitoringAlertLabels(a k8s.AlertInfo) []string {
	var parts []string
	for _, key := range []string{"namespace", "pod", "deployment", "statefulset", "daemonset", "node", "service", "job", "container"} {
		if v, ok := a.Labels[key]; ok {
			parts = append(parts, ui.DimStyle.Render(key+"=")+ui.NormalStyle.Render(v))
		}
	}
	return parts
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

// rollbackDeployment performs the actual rollback.
func (m Model) rollbackDeployment(revision int64) tea.Cmd {
	kctx := m.nav.Context
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	client := m.client

	return func() tea.Msg {
		err := client.RollbackDeployment(context.Background(), kctx, ns, name, revision)
		return rollbackDoneMsg{err: err}
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
			return helmRevisionListMsg{err: fmt.Errorf("%s: %s", cmdErr, strings.TrimSpace(string(output)))}
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

// rollbackHelmRelease performs a Helm rollback to the specified revision.
func (m Model) rollbackHelmRelease(revision int) tea.Cmd {
	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return func() tea.Msg {
			return helmRollbackDoneMsg{err: fmt.Errorf("helm not found: %w", err)}
		}
	}

	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	kubeconfigPaths := m.client.KubeconfigPaths()

	return func() tea.Msg {
		args := []string{"rollback", name, fmt.Sprintf("%d", revision), "-n", ns, "--kube-context", ctx}
		cmd := exec.Command(helmPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
		logger.Info("Running helm command", "cmd", cmd.String())
		output, cmdErr := cmd.CombinedOutput()
		if cmdErr != nil {
			logger.Error("helm rollback failed", "cmd", cmd.String(), "error", cmdErr, "output", string(output))
			return helmRollbackDoneMsg{err: fmt.Errorf("%s: %s", cmdErr, strings.TrimSpace(string(output)))}
		}
		return helmRollbackDoneMsg{}
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
		if m.mapView && m.resourceTypeHasChildren() {
			cmds = append(cmds, m.loadResourceTree())
		} else if m.resourceTypeHasChildren() {
			cmds = append(cmds, m.loadOwned(true))
		} else if m.nav.ResourceType.Kind == "Pod" {
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

// --- Action commands ---

// startLogStream launches kubectl logs as a subprocess, creates a channel, and
// starts a goroutine that reads stdout line by line. Returns a tea.Cmd that
// reads the first line from the channel.
func (m *Model) startLogStream() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	kind := m.actionCtx.kind
	name := m.actionCtx.name
	kctx := m.actionCtx.context
	containerName := m.actionCtx.containerName
	kubeconfigPaths := m.client.KubeconfigPaths()

	ctx, cancel := context.WithCancel(context.Background())
	m.logCancel = cancel

	ch := make(chan string, 256)
	m.logCh = ch

	// Run selector discovery and kubectl logs entirely in a background
	// goroutine so that OIDC browser auth doesn't freeze the TUI.
	go func() {
		defer close(ch)

		var args []string
		switch kind {
		case "Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob", "Service":
			// Try to get the pod selector via kubectl so we can follow ALL pods.
			// kubectl logs deployment/<name> only follows a single pod.
			selector := kubectlGetPodSelector(kubectlPath, kubeconfigPaths, ns, kind, name, kctx)
			if selector != "" {
				args = []string{"logs", "-l", selector, "--all-containers=true", "--prefix", "-f",
					"--max-log-requests=20", "-n", ns, "--context", kctx}
			} else {
				// Fallback: use resource reference (follows only one pod).
				resourceRef := strings.ToLower(kind) + "/" + name
				args = []string{"logs", resourceRef, "--all-containers=true", "--prefix", "-f", "-n", ns, "--context", kctx}
			}
		default:
			args = []string{"logs", "-f", name, "-n", ns, "--context", kctx}
			if containerName != "" {
				args = append(args, "-c", containerName)
			} else if kind == "Pod" {
				args = append(args, "--all-containers=true", "--prefix")
			}
		}

		logger.Info("Starting kubectl logs", "args", strings.Join(args, " "))

		cmd := exec.CommandContext(ctx, kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			logger.Error("Failed to create stdout pipe", "error", err)
			return
		}
		cmd.Stderr = cmd.Stdout

		if err := cmd.Start(); err != nil {
			logger.Error("Failed to start kubectl logs", "error", err)
			return
		}

		defer cmd.Wait() //nolint:errcheck
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
		for scanner.Scan() {
			select {
			case ch <- scanner.Text():
			case <-ctx.Done():
				return
			}
		}
	}()

	return m.waitForLogLine()
}

// kubectlGetPodSelector runs kubectl to extract the pod selector labels for a
// parent resource (Deployment, StatefulSet, etc.). It uses kubectl rather than
// the Go client so that OIDC tokens are discovered/cached by the same credential
// helper that kubectl uses, avoiding a separate browser auth flow.
func kubectlGetPodSelector(kubectlPath, kubeconfigPaths, ns, kind, name, kctx string) string {
	// For CronJobs there's no direct pod selector.
	if kind == "CronJob" {
		return ""
	}

	resourceRef := strings.ToLower(kind) + "/" + name

	getArgs := []string{"get", resourceRef,
		"-n", ns, "--context", kctx,
		"-o", "json",
	}

	cmd := exec.Command(kubectlPath, getArgs...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
	logger.Info("Running kubectl command", "cmd", cmd.String())
	out, err := cmd.Output()
	if err != nil {
		logger.Error("Failed to get pod selector via kubectl", "cmd", cmd.String(), "resource", resourceRef, "error", err)
		return ""
	}

	// Parse the JSON to extract the selector.
	var obj struct {
		Spec struct {
			Selector json.RawMessage `json:"selector"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(out, &obj); err != nil {
		logger.Error("Failed to parse kubectl output", "error", err)
		return ""
	}

	var labels map[string]string

	if kind == "Service" {
		// Service selector is a plain map.
		if err := json.Unmarshal(obj.Spec.Selector, &labels); err != nil {
			logger.Error("Failed to parse service selector", "error", err)
			return ""
		}
	} else {
		// Deployment/StatefulSet/DaemonSet/Job selector has matchLabels.
		var sel struct {
			MatchLabels map[string]string `json:"matchLabels"`
		}
		if err := json.Unmarshal(obj.Spec.Selector, &sel); err != nil {
			logger.Error("Failed to parse selector", "error", err)
			return ""
		}
		labels = sel.MatchLabels
	}

	if len(labels) == 0 {
		return ""
	}

	var parts []string
	for k, v := range labels {
		parts = append(parts, k+"="+v)
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

// waitForLogLine returns a tea.Cmd that reads the next line from the log channel.
func (m Model) waitForLogLine() tea.Cmd {
	ch := m.logCh
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return logLineMsg{done: true, ch: ch}
		}
		return logLineMsg{line: line, ch: ch}
	}
}

// startMultiLogStream spawns one kubectl logs process per selected item and
// merges their output into a single log channel. This supports streaming logs
// from multiple pods or parent resources simultaneously.
func (m *Model) startMultiLogStream(items []model.Item) (tea.Model, tea.Cmd) {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return m, func() tea.Msg { return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)} }
	}

	// Initialize log viewer state.
	m.mode = modeLogs
	m.logLines = nil
	m.logScroll = 0
	m.logFollow = true
	m.logWrap = false
	m.logLineNumbers = true
	m.logTitle = fmt.Sprintf("Logs: %d resources", len(items))

	ctx, cancel := context.WithCancel(context.Background())
	m.logCancel = cancel
	ch := make(chan string, 256)
	m.logCh = ch

	kctx := m.nav.Context
	ns := m.resolveNamespace()

	var wg sync.WaitGroup
	for _, item := range items {
		item := item // capture loop variable
		itemNs := ns
		if item.Namespace != "" {
			itemNs = item.Namespace
		}

		kind := item.Kind
		if kind == "" {
			kind = m.nav.ResourceType.Kind
		}

		var args []string
		switch kind {
		case "Pod":
			args = []string{"logs", item.Name, "--all-containers=true", "--prefix", "-f", "-n", itemNs, "--context", kctx}
		default:
			resourceRef := strings.ToLower(kind) + "/" + item.Name
			args = []string{"logs", resourceRef, "--all-containers=true", "--prefix", "-f", "-n", itemNs, "--context", kctx}
		}

		logger.Info("Starting multi-log kubectl", "item", item.Name, "args", strings.Join(args, " "))
		m.addLogEntry("DBG", "kubectl "+strings.Join(args, " "))

		cmd := exec.CommandContext(ctx, kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			logger.Error("Failed to create stdout pipe for multi-log", "item", item.Name, "error", err)
			continue
		}
		cmd.Stderr = cmd.Stdout

		if err := cmd.Start(); err != nil {
			logger.Error("Failed to start kubectl logs for multi-log", "item", item.Name, "error", err)
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer cmd.Wait() //nolint:errcheck
			scanner := bufio.NewScanner(stdout)
			scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
			for scanner.Scan() {
				select {
				case ch <- scanner.Text():
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// Close the channel once all goroutines finish.
	go func() {
		wg.Wait()
		close(ch)
	}()

	return m, m.waitForLogLine()
}

func (m Model) execKubectlExec() tea.Cmd {
	return m.execKubectlExecWithShell("bash")
}

func (m Model) execKubectlExecWithShell(shell string) tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	args := []string{"exec", "-it", m.actionCtx.name, "-n", ns, "--context", m.actionCtx.context}
	if m.actionCtx.containerName != "" {
		args = append(args, "-c", m.actionCtx.containerName)
	}
	args = append(args, "--", shell)

	logger.Info("Starting kubectl exec", "shell", shell, "args", strings.Join(args, " "))
	cmd := exec.Command(kubectlPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())

	if ui.ConfigTerminalMode == "pty" {
		cols := m.width - 4
		rows := m.height - 6
		if cols < 20 {
			cols = 80
		}
		if rows < 5 {
			rows = 24
		}
		title := fmt.Sprintf("Exec: %s/%s [%s]", m.actionNamespace(), m.actionCtx.name, shell)
		return startPTYExecCmd(cmd, title, cols, rows)
	}

	return tea.ExecProcess(clearBeforeExec(cmd), func(err error) tea.Msg {
		if err != nil && shell == "bash" {
			// bash not found — retry with sh.
			logger.Info("bash exec failed, retrying with sh", "error", err.Error())
			return execRetryShMsg{}
		}
		return actionResultMsg{message: "Exec session ended", err: err}
	})
}

func (m Model) execKubectlAttach() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	args := []string{"attach", "-it", m.actionCtx.name, "-n", ns, "--context", m.actionCtx.context}
	if m.actionCtx.containerName != "" {
		args = append(args, "-c", m.actionCtx.containerName)
	}

	cmd := exec.Command(kubectlPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
	logger.Info("Running kubectl command", "cmd", cmd.String())

	if ui.ConfigTerminalMode == "pty" {
		cols := m.width - 4
		rows := m.height - 6
		if cols < 20 {
			cols = 80
		}
		if rows < 5 {
			rows = 24
		}
		title := fmt.Sprintf("Attach: %s/%s", m.actionNamespace(), m.actionCtx.name)
		return startPTYExecCmd(cmd, title, cols, rows)
	}

	return tea.ExecProcess(clearBeforeExec(cmd), func(err error) tea.Msg {
		return actionResultMsg{message: "Attach session ended", err: err}
	})
}

func (m Model) execKubectlEdit() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	rt := m.actionCtx.resourceType
	args := []string{"edit", rt.Resource, m.actionCtx.name, "--context", m.actionCtx.context}
	if rt.Namespaced {
		args = append(args, "-n", ns)
	}

	cmd := exec.Command(kubectlPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
	logger.Info("Running kubectl command", "cmd", cmd.String())
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			logger.Error("kubectl edit failed", "cmd", cmd.String(), "error", err)
		}
		return actionResultMsg{message: "Edit completed", err: err}
	})
}

func (m Model) execKubectlDescribe() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	rt := m.actionCtx.resourceType
	name := m.actionCtx.name
	args := []string{"describe", rt.Resource, name, "--context", m.actionCtx.context}
	if rt.Namespaced {
		args = append(args, "-n", ns)
	}

	title := fmt.Sprintf("Describe: %s/%s", rt.Resource, name)

	return func() tea.Msg {
		cmd := exec.Command(kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
		logger.Info("Running kubectl command", "cmd", cmd.String())
		output, err := cmd.CombinedOutput()
		if err != nil {
			logger.Error("kubectl describe failed", "cmd", cmd.String(), "error", err, "output", string(output))
			return describeLoadedMsg{
				title: title,
				err:   fmt.Errorf("%s: %s", err, strings.TrimSpace(string(output))),
			}
		}
		return describeLoadedMsg{
			content: string(output),
			title:   title,
		}
	}
}

// execKubectlPortForward starts a port forward as a background process via the manager.
func (m Model) execKubectlPortForward(portMapping string) tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return portForwardStartedMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	resourceKind := "pod"
	if m.actionCtx.kind == "Service" {
		resourceKind = "svc"
	}

	// Parse the port mapping.
	parts := strings.SplitN(portMapping, ":", 2)
	localPort := parts[0]
	remotePort := localPort
	if len(parts) == 2 {
		remotePort = parts[1]
	}

	mgr := m.portForwardMgr
	kubeconfigPaths := m.client.KubeconfigPaths()
	name := m.actionCtx.name
	kctx := m.actionCtx.context

	logger.Info("Running kubectl port-forward", "resource", resourceKind+"/"+name, "localPort", localPort, "remotePort", remotePort, "namespace", ns, "context", kctx)
	return func() tea.Msg {
		id, err := mgr.Start(kubectlPath, kubeconfigPaths, resourceKind, name, ns, kctx, localPort, remotePort)
		if err != nil {
			logger.Error("kubectl port-forward failed", "resource", resourceKind+"/"+name, "error", err)
		}
		return portForwardStartedMsg{id: id, localPort: localPort, remotePort: remotePort, err: err}
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

// stopPortForward stops a port forward by ID.
func (m Model) stopPortForward(id int) tea.Cmd {
	mgr := m.portForwardMgr
	return func() tea.Msg {
		err := mgr.Stop(id)
		return portForwardStoppedMsg{id: id, err: err}
	}
}

// restartPortForward restarts a stopped/failed port forward by ID.
// It removes the old entry and starts a new one with the same parameters.
func (m Model) restartPortForward(id int) tea.Cmd {
	mgr := m.portForwardMgr
	entry := mgr.GetEntry(id)
	if entry == nil {
		return func() tea.Msg {
			return portForwardStartedMsg{err: fmt.Errorf("port forward %d not found", id)}
		}
	}
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return portForwardStartedMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}
	kubeconfigPaths := m.client.KubeconfigPaths()
	resourceKind := entry.ResourceKind
	name := entry.ResourceName
	ns := entry.Namespace
	kctx := entry.Context
	remotePort := entry.RemotePort
	// Use "0" for local port to get a fresh random port assignment.
	localPort := "0"

	mgr.Remove(id)

	return func() tea.Msg {
		newID, err := mgr.Start(kubectlPath, kubeconfigPaths, resourceKind, name, ns, kctx, localPort, remotePort)
		if err != nil {
			logger.Error("kubectl port-forward restart failed", "resource", resourceKind+"/"+name, "error", err)
		}
		return portForwardStartedMsg{id: newID, localPort: localPort, remotePort: remotePort, err: err}
	}
}

// waitForPortForwardUpdate returns a command that waits for a port forward state change.
func (m Model) waitForPortForwardUpdate() tea.Cmd {
	ch := make(chan struct{}, 1)
	m.portForwardMgr.SetUpdateCallback(func() {
		select {
		case ch <- struct{}{}:
		default:
		}
	})
	return func() tea.Msg {
		<-ch
		return portForwardUpdateMsg{}
	}
}

func (m Model) execKubectlDebug() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	args := []string{"debug", m.actionCtx.name, "-it", "--image=busybox", "--context", m.actionCtx.context, "-n", ns}

	cmd := exec.Command(kubectlPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
	logger.Info("Running kubectl command", "cmd", cmd.String())

	if ui.ConfigTerminalMode == "pty" {
		cols := m.width - 4
		rows := m.height - 6
		if cols < 20 {
			cols = 80
		}
		if rows < 5 {
			rows = 24
		}
		title := fmt.Sprintf("Debug: %s/%s", m.actionNamespace(), m.actionCtx.name)
		return startPTYExecCmd(cmd, title, cols, rows)
	}

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return actionResultMsg{message: "Debug session ended", err: err}
	})
}

// runDebugPod runs a standalone alpine debug pod in the target namespace.
func (m Model) runDebugPod() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	ctx := m.actionCtx.context
	podName := fmt.Sprintf("debug-%d", time.Now().Unix())

	args := []string{"run", podName, "--image=alpine", "-it", "--rm",
		"--restart=Never", "--context", ctx, "-n", ns, "--", "sh"}

	cmd := exec.Command(kubectlPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
	logger.Info("Running kubectl command", "cmd", cmd.String())
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			logger.Error("kubectl run debug pod failed", "cmd", cmd.String(), "error", err)
		}
		return actionResultMsg{message: "Debug pod session ended", err: err}
	})
}

// runDebugPodWithPVC runs an alpine debug pod with a PVC mounted at /mnt/data.
func (m Model) runDebugPodWithPVC() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	ctx := m.actionCtx.context
	pvcName := m.actionCtx.name
	podName := fmt.Sprintf("debug-pvc-%d", time.Now().Unix())

	// Create a pod manifest with the PVC mounted.
	manifest := fmt.Sprintf(`{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {"name": "%s"},
		"spec": {
			"containers": [{
				"name": "debug",
				"image": "alpine",
				"command": ["sh"],
				"stdin": true,
				"tty": true,
				"volumeMounts": [{"name": "data", "mountPath": "/mnt/data"}]
			}],
			"volumes": [{"name": "data", "persistentVolumeClaim": {"claimName": "%s"}}],
			"restartPolicy": "Never"
		}
	}`, podName, pvcName)

	// Use kubectl run with overrides to mount the PVC.
	args := []string{"run", podName, "--image=alpine", "-it", "--rm",
		"--restart=Never", "--context", ctx, "-n", ns,
		"--overrides", manifest, "--", "sh"}

	cmd := exec.Command(kubectlPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
	logger.Info("Running kubectl command", "cmd", cmd.String())
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			logger.Error("kubectl run debug pod with PVC failed", "cmd", cmd.String(), "error", err)
		}
		return actionResultMsg{message: "Debug pod session ended", err: err}
	})
}

func (m Model) execKubectlEvents() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	name := m.actionCtx.name
	rt := m.actionCtx.resourceType
	args := []string{"get", "events", "--field-selector", "involvedObject.name=" + name, "--context", m.actionCtx.context}
	if rt.Namespaced {
		args = append(args, "-n", ns)
	}

	title := fmt.Sprintf("Events: %s/%s", rt.Resource, name)

	return func() tea.Msg {
		cmd := exec.Command(kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
		logger.Info("Running kubectl command", "cmd", cmd.String())
		output, err := cmd.CombinedOutput()
		if err != nil {
			logger.Error("kubectl get events failed", "cmd", cmd.String(), "error", err, "output", string(output))
			return describeLoadedMsg{
				title: title,
				err:   fmt.Errorf("%s: %s", err, strings.TrimSpace(string(output))),
			}
		}
		content := strings.TrimSpace(string(output))
		if content == "" {
			content = "No events found"
		}
		return describeLoadedMsg{
			content: content,
			title:   title,
		}
	}
}

func (m Model) deleteResource() tea.Cmd {
	// Special handling for Helm releases.
	if m.actionCtx.resourceType.APIGroup == "_helm" {
		return m.uninstallHelmRelease()
	}

	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	rt := m.actionCtx.resourceType
	name := m.actionCtx.name
	logger.Info("Deleting resource", "resource", rt.Resource, "name", name, "namespace", ns, "context", ctx)
	return func() tea.Msg {
		err := m.client.DeleteResource(ctx, ns, rt, name)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Deleted %s/%s", rt.Resource, name)}
	}
}

func (m Model) forceDeleteResource() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	rt := m.actionCtx.resourceType
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	logger.Info("Force deleting resource", "resource", rt.Resource, "name", name, "namespace", ns, "context", ctx)

	patchArgs := []string{"patch", rt.Resource, name, "--context", ctx,
		"--type", "merge", "-p", `{"metadata":{"finalizers":null}}`}
	if rt.Namespaced {
		patchArgs = append(patchArgs, "-n", ns)
	}

	deleteArgs := []string{"delete", rt.Resource, name, "--context", ctx,
		"--grace-period=0", "--force"}
	if rt.Namespaced {
		deleteArgs = append(deleteArgs, "-n", ns)
	}

	return func() tea.Msg {
		// Remove finalizers first (ignore errors — resource may not have finalizers).
		patchCmd := exec.Command(kubectlPath, patchArgs...)
		patchCmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
		logger.Info("Running kubectl command", "cmd", patchCmd.String())
		patchCmd.Run() //nolint:errcheck
		// Force delete.
		cmd := exec.Command(kubectlPath, deleteArgs...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
		logger.Info("Running kubectl command", "cmd", cmd.String())
		if output, err := cmd.CombinedOutput(); err != nil {
			logger.Error("kubectl force delete failed", "cmd", cmd.String(), "error", err, "output", string(output))
			return actionResultMsg{err: fmt.Errorf("%s: %s", err, strings.TrimSpace(string(output)))}
		}
		return actionResultMsg{message: fmt.Sprintf("Force deleted %s/%s", rt.Resource, name)}
	}
}

func (m Model) syncArgoApp() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return func() tea.Msg {
		err := m.client.SyncArgoApp(ctx, ns, name)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Sync initiated for %s", name)}
	}
}

func (m Model) refreshArgoApp() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	return func() tea.Msg {
		err := m.client.RefreshArgoApp(ctx, ns, name)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Hard refresh initiated for %s", name)}
	}
}

// reconcileFluxResource triggers reconciliation of a Flux resource.
func (m Model) reconcileFluxResource() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	rt := m.actionCtx.resourceType
	gvr := schema.GroupVersionResource{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
	}
	return func() tea.Msg {
		err := m.client.ReconcileFluxResource(ctx, ns, name, gvr)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Reconciliation triggered for %s", name)}
	}
}

// suspendFluxResource suspends a Flux resource.
func (m Model) suspendFluxResource() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	rt := m.actionCtx.resourceType
	gvr := schema.GroupVersionResource{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
	}
	return func() tea.Msg {
		err := m.client.SuspendFluxResource(ctx, ns, name, gvr)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Suspended %s", name)}
	}
}

// resumeFluxResource resumes a Flux resource.
func (m Model) resumeFluxResource() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	rt := m.actionCtx.resourceType
	gvr := schema.GroupVersionResource{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
	}
	return func() tea.Msg {
		err := m.client.ResumeFluxResource(ctx, ns, name, gvr)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Resumed %s", name)}
	}
}

func (m Model) diffArgoApp() tea.Cmd {
	ns := m.actionNamespace()
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	title := fmt.Sprintf("ArgoCD Diff: %s", name)

	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return describeLoadedMsg{
				title: title,
				err:   fmt.Errorf("kubectl not found in PATH"),
			}
		}
	}

	return func() tea.Msg {
		kubeEnv := "KUBECONFIG=" + m.client.KubeconfigPaths()

		// Get the Application resource as JSON to extract managed resources.
		getArgs := []string{"get", "applications.argoproj.io", name,
			"-n", ns, "--context", ctx, "-o", "json"}
		getCmd := exec.Command(kubectlPath, getArgs...)
		getCmd.Env = append(os.Environ(), kubeEnv)
		logger.Info("Running kubectl command", "cmd", getCmd.String())
		appJSON, err := getCmd.CombinedOutput()
		if err != nil {
			logger.Error("kubectl get application failed", "cmd", getCmd.String(), "error", err, "output", string(appJSON))
			return describeLoadedMsg{
				title: title,
				err:   fmt.Errorf("failed to get application: %s: %s", err, strings.TrimSpace(string(appJSON))),
			}
		}

		// Parse the managed resources from status.resources
		type managedResource struct {
			Group     string `json:"group"`
			Kind      string `json:"kind"`
			Namespace string `json:"namespace"`
			Name      string `json:"name"`
			Status    string `json:"status"`
			Health    struct {
				Status string `json:"status"`
			} `json:"health"`
		}
		var app struct {
			Status struct {
				Resources []managedResource `json:"resources"`
				Sync      struct {
					Status string `json:"status"`
				} `json:"sync"`
			} `json:"status"`
		}

		if jsonErr := json.Unmarshal(appJSON, &app); jsonErr != nil {
			return describeLoadedMsg{
				title: title,
				err:   fmt.Errorf("failed to parse application JSON: %w", jsonErr),
			}
		}

		if len(app.Status.Resources) == 0 {
			return describeLoadedMsg{
				content: "No managed resources found. The application may not have been synced yet.",
				title:   title,
			}
		}

		// Build a diff output by running kubectl diff on each out-of-sync resource
		var diffBuf strings.Builder
		diffBuf.WriteString(fmt.Sprintf("Application: %s\n", name))
		diffBuf.WriteString(fmt.Sprintf("Sync Status: %s\n", app.Status.Sync.Status))
		diffBuf.WriteString(fmt.Sprintf("Managed Resources: %d\n\n", len(app.Status.Resources)))

		outOfSync := 0
		for _, res := range app.Status.Resources {
			if res.Status != "OutOfSync" {
				continue
			}
			outOfSync++

			// Build the resource identifier
			resource := res.Kind
			if res.Group != "" {
				resource = res.Kind + "." + res.Group
			}

			diffBuf.WriteString(fmt.Sprintf("━━━ %s/%s", resource, res.Name))
			if res.Namespace != "" {
				diffBuf.WriteString(fmt.Sprintf(" (ns: %s)", res.Namespace))
			}
			diffBuf.WriteString(" ━━━\n")

			// Run kubectl diff for this specific resource
			diffArgs := []string{"diff", "--context", ctx}
			if res.Namespace != "" {
				diffArgs = append(diffArgs, "-n", res.Namespace)
			}

			// Get the desired manifest from the application
			// Use kubectl get to fetch live state, then we'll show the status
			getResArgs := []string{"get", resource, res.Name, "--context", ctx, "-o", "yaml"}
			if res.Namespace != "" {
				getResArgs = append(getResArgs, "-n", res.Namespace)
			}
			getResCmd := exec.Command(kubectlPath, getResArgs...)
			getResCmd.Env = append(os.Environ(), kubeEnv)
			logger.Info("Running kubectl command", "cmd", getResCmd.String())
			resOut, resErr := getResCmd.CombinedOutput()
			if resErr != nil {
				logger.Error("kubectl get resource failed", "cmd", getResCmd.String(), "error", resErr, "output", string(resOut))
				diffBuf.WriteString(fmt.Sprintf("  Status: OutOfSync (resource may not exist yet)\n"))
				if res.Health.Status != "" {
					diffBuf.WriteString(fmt.Sprintf("  Health: %s\n", res.Health.Status))
				}
				diffBuf.WriteString("\n")
				continue
			}
			_ = resOut

			diffBuf.WriteString(fmt.Sprintf("  Status: OutOfSync\n"))
			if res.Health.Status != "" {
				diffBuf.WriteString(fmt.Sprintf("  Health: %s\n", res.Health.Status))
			}
			diffBuf.WriteString("\n")
		}

		if outOfSync == 0 {
			diffBuf.WriteString("All resources are in sync. No differences found.\n\n")

			// Still show a summary of all managed resources
			diffBuf.WriteString("Resource Summary:\n")
			for _, res := range app.Status.Resources {
				resource := res.Kind
				if res.Group != "" {
					resource = res.Kind + "." + res.Group
				}
				health := res.Health.Status
				if health == "" {
					health = "-"
				}
				diffBuf.WriteString(fmt.Sprintf("  %-40s  Sync: %-10s  Health: %s\n",
					resource+"/"+res.Name, res.Status, health))
			}
		}

		return describeLoadedMsg{
			content: diffBuf.String(),
			title:   title,
		}
	}
}

func (m Model) uninstallHelmRelease() tea.Cmd {
	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("helm not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	args := []string{"uninstall", name, "-n", ns, "--kube-context", ctx}

	cmd := exec.Command(helmPath, args...)
	logger.Info("Running helm command", "cmd", cmd.String())
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			logger.Error("helm uninstall failed", "cmd", cmd.String(), "error", err)
		}
		return actionResultMsg{message: fmt.Sprintf("Uninstalled %s", name), err: err}
	})
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
				err:   fmt.Errorf("%s: %s", cmdErr, strings.TrimSpace(string(output))),
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

// editHelmValues fetches current values, writes them to a temp file, and opens
// in $EDITOR for viewing and editing.
// Uses a shell script via tea.ExecProcess so the editor can take over the terminal.
func (m Model) editHelmValues() tea.Cmd {
	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("helm not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	kubeconfigPaths := m.client.KubeconfigPaths()

	// Build a shell script that: gets values -> writes to temp file -> opens editor ->
	// checks for changes -> applies with helm upgrade --reuse-values using the
	// chart reference from `helm list`.
	script := fmt.Sprintf(`
set -e
HELM=%q
RELEASE=%q
NS=%q
CTX=%q
export KUBECONFIG=%q

TMPFILE=$(mktemp /tmp/helm-values-${RELEASE}-XXXXXX.yaml)

$HELM get values "$RELEASE" -n "$NS" --kube-context "$CTX" -o yaml > "$TMPFILE" 2>&1
# Replace bare 'null' with a helpful comment
if [ "$(cat "$TMPFILE" | tr -d '[:space:]')" = "null" ]; then
  echo "# Add your values here" > "$TMPFILE"
fi

# Save checksum before editing
BEFORE=$(md5sum "$TMPFILE" 2>/dev/null || md5 -q "$TMPFILE" 2>/dev/null || cat "$TMPFILE")

${EDITOR:-${VISUAL:-vi}} "$TMPFILE"

AFTER=$(md5sum "$TMPFILE" 2>/dev/null || md5 -q "$TMPFILE" 2>/dev/null || cat "$TMPFILE")

if [ "$BEFORE" = "$AFTER" ]; then
  rm -f "$TMPFILE"
  echo "No changes detected."
  exit 0
fi

# Parse the chart-version string from helm list JSON, then strip the version
# suffix to get the chart name for repo-based resolution.
CHART_VERSION=$($HELM list -n "$NS" --kube-context "$CTX" --filter "^${RELEASE}$" -o json 2>/dev/null \
  | sed -n 's/.*"chart":"\([^"]*\)".*/\1/p' | head -1)
# Strip trailing -<semver> (e.g. "nginx-ingress-1.2.3" -> "nginx-ingress").
CHART_NAME=$(echo "$CHART_VERSION" | sed 's/-[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*.*$//')
if [ -z "$CHART_NAME" ]; then
  echo ""
  echo "Could not determine chart for release $RELEASE."
  echo "Your edited values have been saved to: $TMPFILE"
  echo "Apply manually with:"
  echo "  helm upgrade $RELEASE <CHART> -n $NS --kube-context $CTX --reuse-values -f $TMPFILE"
  exit 1
fi

echo "Applying values with chart $CHART_NAME..."
if ! $HELM upgrade "$RELEASE" "$CHART_NAME" -n "$NS" --kube-context "$CTX" --reuse-values -f "$TMPFILE" 2>&1; then
  echo ""
  echo "Upgrade failed. Your edited values have been saved to: $TMPFILE"
  echo "You may need to specify the full chart reference. Apply manually with:"
  echo "  helm upgrade $RELEASE <REPO/CHART> -n $NS --kube-context $CTX --reuse-values -f $TMPFILE"
  exit 1
fi
rm -f "$TMPFILE"
`,
		helmPath, name, ns, ctx, kubeconfigPaths,
	)

	cmd := exec.Command("sh", "-c", script)
	cmd.Env = os.Environ()
	logger.Info("Running helm edit values", "release", name, "namespace", ns, "context", ctx)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			logger.Error("helm edit values failed", "release", name, "error", err)
			return actionResultMsg{err: fmt.Errorf("helm edit values: %w", err)}
		}
		return actionResultMsg{message: fmt.Sprintf("Values updated for %s", name)}
	})
}

func (m Model) scaleDeployment(replicas int32) tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	logger.Info("Scaling deployment", "name", name, "replicas", replicas, "namespace", ns, "context", ctx)
	return func() tea.Msg {
		err := m.client.ScaleDeployment(ctx, ns, name, replicas)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Scaled %s to %d replicas", name, replicas)}
	}
}

func (m Model) restartDeployment() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	logger.Info("Restarting deployment", "name", name, "namespace", ns, "context", ctx)
	return func() tea.Msg {
		err := m.client.RestartDeployment(ctx, ns, name)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Restarting %s", name)}
	}
}

func (m Model) execKubectlCordon() tea.Cmd {
	return m.execKubectlNodeCmd("cordon")
}

func (m Model) execKubectlUncordon() tea.Cmd {
	return m.execKubectlNodeCmd("uncordon")
}

func (m Model) execKubectlDrain() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}
	name := m.actionCtx.name
	args := []string{"drain", name, "--context", m.actionCtx.context,
		"--ignore-daemonsets", "--delete-emptydir-data"}

	cmd := exec.Command(kubectlPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
	logger.Info("Running kubectl command", "cmd", cmd.String())
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			logger.Error("kubectl drain failed", "cmd", cmd.String(), "error", err)
			return actionResultMsg{err: fmt.Errorf("drain %s: %w", name, err)}
		}
		return actionResultMsg{message: fmt.Sprintf("Drained %s", name)}
	})
}

func (m Model) execKubectlNodeCmd(subcmd string) tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}
	name := m.actionCtx.name
	args := []string{subcmd, name, "--context", m.actionCtx.context}

	return func() tea.Msg {
		cmd := exec.Command(kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
		logger.Info("Running kubectl command", "cmd", cmd.String())
		output, err := cmd.CombinedOutput()
		if err != nil {
			logger.Error("kubectl node command failed", "cmd", cmd.String(), "error", err, "output", string(output))
			return actionResultMsg{err: fmt.Errorf("%s %s: %s", subcmd, name, strings.TrimSpace(string(output)))}
		}
		return actionResultMsg{message: strings.TrimSpace(string(output))}
	}
}

// triggerCronJob creates a Job from a CronJob.
func (m Model) triggerCronJob() tea.Cmd {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	kctx := m.actionCtx.context
	client := m.client

	return func() tea.Msg {
		jobName, err := client.TriggerCronJob(context.Background(), kctx, ns, name)
		return triggerCronJobMsg{jobName: jobName, err: err}
	}
}

// execKubectlNodeShell opens a debug shell on a node using kubectl debug.
func (m Model) execKubectlNodeShell() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	nodeName := m.actionCtx.name
	ctx := m.actionCtx.context

	args := []string{"debug", "node/" + nodeName, "-it",
		"--image=busybox",
		"--context", ctx,
		"--", "chroot", "/host", "/bin/sh"}

	cmd := exec.Command(kubectlPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
	logger.Info("Running kubectl command", "cmd", cmd.String())

	if ui.ConfigTerminalMode == "pty" {
		cols := m.width - 4
		rows := m.height - 6
		if cols < 20 {
			cols = 80
		}
		if rows < 5 {
			rows = 24
		}
		title := fmt.Sprintf("Node Shell: %s", nodeName)
		return startPTYExecCmd(cmd, title, cols, rows)
	}

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return actionResultMsg{err: fmt.Errorf("node shell: %w", err)}
		}
		return actionResultMsg{message: "Node shell session ended"}
	})
}

func (m Model) bulkDeleteResources() tea.Cmd {
	items := m.bulkItems
	ctx := m.actionCtx.context
	rt := m.actionCtx.resourceType
	client := m.client
	ns := m.actionNamespace()

	return func() tea.Msg {
		var succeeded, failed int
		var errors []string
		for _, item := range items {
			itemNs := ns
			if item.Namespace != "" {
				itemNs = item.Namespace
			}
			logger.Info("Bulk deleting", "resource", rt.Resource, "name", item.Name, "namespace", itemNs)
			err := client.DeleteResource(ctx, itemNs, rt, item.Name)
			if err != nil {
				failed++
				errors = append(errors, fmt.Sprintf("%s: %s", item.Name, err.Error()))
			} else {
				succeeded++
			}
		}
		return bulkActionResultMsg{succeeded: succeeded, failed: failed, errors: errors}
	}
}

func (m Model) bulkForceDeleteResources() tea.Cmd {
	items := m.bulkItems
	ctx := m.actionCtx.context
	rt := m.actionCtx.resourceType
	client := m.client
	ns := m.actionNamespace()

	return func() tea.Msg {
		kubectlPath, err := exec.LookPath("kubectl")
		if err != nil {
			return bulkActionResultMsg{failed: len(items), errors: []string{"kubectl not found"}}
		}

		var succeeded, failed int
		var errors []string
		for _, item := range items {
			itemNs := ns
			if item.Namespace != "" {
				itemNs = item.Namespace
			}
			logger.Info("Bulk force deleting", "resource", rt.Resource, "name", item.Name, "namespace", itemNs)

			// Remove finalizers first.
			patchArgs := []string{"patch", rt.Resource, item.Name, "--context", ctx,
				"--type", "merge", "-p", `{"metadata":{"finalizers":null}}`}
			if rt.Namespaced {
				patchArgs = append(patchArgs, "-n", itemNs)
			}
			patchCmd := exec.Command(kubectlPath, patchArgs...)
			patchCmd.Env = append(os.Environ(), "KUBECONFIG="+client.KubeconfigPaths())
			logger.Info("Running kubectl command", "cmd", patchCmd.String())
			patchCmd.Run() //nolint:errcheck

			// Force delete.
			deleteArgs := []string{"delete", rt.Resource, item.Name, "--context", ctx,
				"--grace-period=0", "--force"}
			if rt.Namespaced {
				deleteArgs = append(deleteArgs, "-n", itemNs)
			}
			cmd := exec.Command(kubectlPath, deleteArgs...)
			cmd.Env = append(os.Environ(), "KUBECONFIG="+client.KubeconfigPaths())
			logger.Info("Running kubectl command", "cmd", cmd.String())
			if output, err := cmd.CombinedOutput(); err != nil {
				logger.Error("kubectl bulk force delete failed", "cmd", cmd.String(), "error", err, "output", string(output))
				failed++
				errors = append(errors, fmt.Sprintf("%s: %s", item.Name, strings.TrimSpace(string(output))))
			} else {
				succeeded++
			}
		}
		return bulkActionResultMsg{succeeded: succeeded, failed: failed, errors: errors}
	}
}

func (m Model) bulkScaleResources(replicas int32) tea.Cmd {
	items := m.bulkItems
	ctx := m.actionCtx.context
	client := m.client
	ns := m.actionNamespace()

	return func() tea.Msg {
		var succeeded, failed int
		var errors []string
		for _, item := range items {
			itemNs := ns
			if item.Namespace != "" {
				itemNs = item.Namespace
			}
			logger.Info("Bulk scaling", "name", item.Name, "replicas", replicas, "namespace", itemNs)
			err := client.ScaleDeployment(ctx, itemNs, item.Name, replicas)
			if err != nil {
				failed++
				errors = append(errors, fmt.Sprintf("%s: %s", item.Name, err.Error()))
			} else {
				succeeded++
			}
		}
		return bulkActionResultMsg{succeeded: succeeded, failed: failed, errors: errors}
	}
}

func (m Model) bulkRestartResources() tea.Cmd {
	items := m.bulkItems
	ctx := m.actionCtx.context
	client := m.client
	ns := m.actionNamespace()

	return func() tea.Msg {
		var succeeded, failed int
		var errors []string
		for _, item := range items {
			itemNs := ns
			if item.Namespace != "" {
				itemNs = item.Namespace
			}
			logger.Info("Bulk restarting", "name", item.Name, "namespace", itemNs)
			err := client.RestartDeployment(ctx, itemNs, item.Name)
			if err != nil {
				failed++
				errors = append(errors, fmt.Sprintf("%s: %s", item.Name, err.Error()))
			} else {
				succeeded++
			}
		}
		return bulkActionResultMsg{succeeded: succeeded, failed: failed, errors: errors}
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

// scheduleStatusClear returns a command that sends a clear message after a delay.
func scheduleStatusClear() tea.Cmd {
	return tea.Tick(5*time.Second, func(_ time.Time) tea.Msg {
		return statusMessageExpiredMsg{}
	})
}

// scheduleWatchTick returns a command that sends a watchTickMsg after the interval.
func scheduleWatchTick(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(_ time.Time) tea.Msg {
		return watchTickMsg{}
	})
}

// openInBrowser opens the given URL in the user's default browser using
// platform-specific commands (open on macOS, xdg-open on Linux, start on Windows).
func openInBrowser(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "linux":
			cmd = exec.Command("xdg-open", url)
		case "windows":
			cmd = exec.Command("cmd", "/c", "start", url)
		default:
			return actionResultMsg{err: fmt.Errorf("browser open not supported on %s", runtime.GOOS)}
		}
		if err := cmd.Start(); err != nil {
			return actionResultMsg{err: fmt.Errorf("failed to open browser: %w", err)}
		}
		return actionResultMsg{message: "Opened " + url}
	}
}

// copyToSystemClipboard copies text to the system clipboard using platform-specific tools.
func copyToSystemClipboard(text string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("pbcopy")
		case "linux":
			cmd = exec.Command("xclip", "-selection", "clipboard")
		default:
			return actionResultMsg{err: fmt.Errorf("clipboard not supported on %s", runtime.GOOS)}
		}
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err != nil {
			return actionResultMsg{err: fmt.Errorf("clipboard: %w", err)}
		}
		return actionResultMsg{message: "Copied to clipboard"}
	}
}

// loadPodsForAction fetches pods owned by the action target resource (for exec/attach on parent resources).
func (m Model) loadPodsForAction() tea.Cmd {
	kctx := m.actionCtx.context
	ns := m.actionNamespace()
	kind := m.actionCtx.kind
	name := m.actionCtx.name
	return func() tea.Msg {
		items, err := m.client.GetOwnedResources(context.Background(), kctx, ns, kind, name)
		return podSelectMsg{items: items, err: err}
	}
}

// loadPodsForLogAction fetches pods matching the parent resource's selector using kubectl.
// Uses kubectl instead of the Go client to avoid separate OIDC auth flows.
func (m Model) loadPodsForLogAction() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return podLogSelectMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	kind := m.actionCtx.kind
	name := m.actionCtx.name
	kctx := m.actionCtx.context
	kubeconfigPaths := m.client.KubeconfigPaths()

	return func() tea.Msg {
		// Get the selector for this parent resource.
		selector := kubectlGetPodSelector(kubectlPath, kubeconfigPaths, ns, kind, name, kctx)
		if selector == "" {
			return podLogSelectMsg{err: fmt.Errorf("could not determine pod selector for %s/%s", kind, name)}
		}

		// Fetch pods matching the selector.
		args := []string{"get", "pods", "-l", selector, "-n", ns, "--context", kctx, "-o", "json"}
		cmd := exec.Command(kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
		logger.Info("Running kubectl command", "cmd", cmd.String())
		out, err := cmd.Output()
		if err != nil {
			logger.Error("kubectl get pods failed", "cmd", cmd.String(), "error", err)
			return podLogSelectMsg{err: fmt.Errorf("failed to list pods: %w", err)}
		}

		var podList struct {
			Items []struct {
				Metadata struct {
					Name      string `json:"name"`
					Namespace string `json:"namespace"`
				} `json:"metadata"`
				Status struct {
					Phase string `json:"phase"`
				} `json:"status"`
			} `json:"items"`
		}
		if err := json.Unmarshal(out, &podList); err != nil {
			return podLogSelectMsg{err: fmt.Errorf("failed to parse pod list: %w", err)}
		}

		var items []model.Item
		for _, pod := range podList.Items {
			items = append(items, model.Item{
				Name:      pod.Metadata.Name,
				Namespace: pod.Metadata.Namespace,
				Kind:      "Pod",
				Status:    pod.Status.Phase,
			})
		}
		return podLogSelectMsg{items: items}
	}
}

// loadContainersForAction fetches the container list for the action target pod.
func (m Model) loadContainersForAction() tea.Cmd {
	kctx := m.actionCtx.context
	ns := m.actionNamespace()
	podName := m.actionCtx.name
	return func() tea.Msg {
		items, err := m.client.GetContainers(context.Background(), kctx, ns, podName)
		// Reverse order for the selector: regular containers first (reversed),
		// then init/sidecar containers (reversed), so the most relevant
		// container is at the top.
		for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
			items[i], items[j] = items[j], items[i]
		}
		return containerSelectMsg{items: items, err: err}
	}
}

// eventTimelineMsg carries event timeline data for the overlay.
type eventTimelineMsg struct {
	events []k8s.EventInfo
	err    error
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

// rbacCheckMsg carries the result of an RBAC permission check.
type rbacCheckMsg struct {
	results  []k8s.RBACCheck
	kind     string
	resource string
	err      error
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

// podStartupMsg carries the result of a pod startup analysis.
type podStartupMsg struct {
	info *k8s.PodStartupInfo
	err  error
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

// alertsLoadedMsg carries the result of loading Prometheus alerts for a resource.
type alertsLoadedMsg struct {
	alerts []k8s.AlertInfo
	err    error
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

// loadMonitoringOverview loads all active Prometheus alerts for the current namespace.
// It is triggered by the global "@" hotkey and shows alerts in the alerts overlay.
func (m Model) loadMonitoringOverview() tea.Cmd {
	client := m.client
	kctx := m.nav.Context
	ns := m.resolveNamespace()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		alerts, err := client.GetAllActiveAlerts(ctx, kctx, ns)
		if err != nil {
			return alertsLoadedMsg{err: err}
		}
		return alertsLoadedMsg{alerts: alerts}
	}
}

// netpolLoadedMsg carries the result of loading a network policy.
type netpolLoadedMsg struct {
	info *k8s.NetworkPolicyInfo
	err  error
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

func (m Model) batchPatchLabels(key, value string, remove bool, isAnnotation bool) tea.Cmd {
	items := m.bulkItems
	ctx := m.actionCtx.context
	rt := m.actionCtx.resourceType
	gvr := schema.GroupVersionResource{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
	}
	client := m.client
	ns := m.namespace

	return func() tea.Msg {
		var success, failed int
		for _, item := range items {
			var patch map[string]interface{}
			if remove {
				patch = map[string]interface{}{key: nil}
			} else {
				patch = map[string]interface{}{key: value}
			}
			itemNs := item.Namespace
			if itemNs == "" {
				itemNs = ns
			}
			var err error
			if isAnnotation {
				err = client.PatchAnnotations(context.Background(), ctx, itemNs, item.Name, gvr, patch)
			} else {
				err = client.PatchLabels(context.Background(), ctx, itemNs, item.Name, gvr, patch)
			}
			if err != nil {
				failed++
			} else {
				success++
			}
		}
		return bulkActionResultMsg{succeeded: success, failed: failed}
	}
}

func (m Model) applyFromClipboard() tea.Cmd {
	ctx := m.nav.Context
	ns := m.effectiveNamespace()
	if ns == "" {
		ns = "default"
	}

	// Read from clipboard.
	var clipCmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		clipCmd = exec.Command("pbpaste")
	case "linux":
		clipCmd = exec.Command("xclip", "-selection", "clipboard", "-o")
	default:
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("clipboard not supported on %s", runtime.GOOS)}
		}
	}

	clipContent, err := clipCmd.Output()
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("reading clipboard: %w", err)}
		}
	}

	if len(strings.TrimSpace(string(clipContent))) == 0 {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("clipboard is empty")}
		}
	}

	// Write to temp file for editor review.
	tmpFile, err := os.CreateTemp("", "k-paste-*.yaml")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("creating temp file: %w", err)}
		}
	}
	if _, err := tmpFile.Write(clipContent); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("writing temp file: %w", err)}
		}
	}
	tmpFile.Close()

	// Open in editor for review/editing before applying.
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	tmpPath := tmpFile.Name()

	// Record modification time before opening editor.
	origModTime := time.Time{}
	if fi, err := os.Stat(tmpPath); err == nil {
		origModTime = fi.ModTime()
	}

	cmd := exec.Command(editor, tmpPath)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			os.Remove(tmpPath)
			return actionResultMsg{err: fmt.Errorf("editor: %w", err)}
		}
		return templateApplyMsg{tmpFile: tmpPath, context: ctx, ns: ns, origModTime: origModTime}
	})
}

// templateApplyMsg is sent after the editor closes, carrying the temp file path for kubectl apply.
type templateApplyMsg struct {
	tmpFile     string
	context     string
	ns          string
	origModTime time.Time // modification time before editor opened; skip apply if unchanged
}

// applyTemplate creates a temp file with the template YAML, opens it in $EDITOR,
// then applies it with kubectl after the editor exits.
func (m Model) applyTemplate(tmpl model.ResourceTemplate) tea.Cmd {
	ns := m.effectiveNamespace()
	if ns == "" {
		ns = "default"
	}
	ctx := m.nav.Context

	// Replace NAMESPACE placeholder.
	yamlContent := strings.ReplaceAll(tmpl.YAML, "NAMESPACE", ns)

	// Write to temp file.
	tmpFile, err := os.CreateTemp("", "k-template-*.yaml")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("creating temp file: %w", err)}
		}
	}
	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("writing temp file: %w", err)}
		}
	}
	tmpFile.Close()

	// Determine editor.
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	tmpPath := tmpFile.Name()

	// Record modification time before opening editor.
	origModTime := time.Time{}
	if fi, err := os.Stat(tmpPath); err == nil {
		origModTime = fi.ModTime()
	}

	cmd := exec.Command(editor, tmpPath)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			os.Remove(tmpPath)
			return actionResultMsg{err: fmt.Errorf("editor: %w", err)}
		}
		return templateApplyMsg{tmpFile: tmpPath, context: ctx, ns: ns, origModTime: origModTime}
	})
}

// applyTemplateFile runs kubectl apply -f on the given temp file and cleans it up.
func (m Model) applyTemplateFile(tmpFile, ctx, ns string) tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		os.Remove(tmpFile)
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	return func() tea.Msg {
		defer os.Remove(tmpFile)

		args := []string{"apply", "-f", tmpFile, "--context", ctx}
		if ns != "" {
			args = append(args, "-n", ns)
		}
		cmd := exec.Command(kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
		logger.Info("Running kubectl command", "cmd", cmd.String())
		output, err := cmd.CombinedOutput()
		if err != nil {
			logger.Error("kubectl apply failed", "cmd", cmd.String(), "error", err, "output", string(output))
			return actionResultMsg{err: fmt.Errorf("kubectl apply: %s", strings.TrimSpace(string(output)))}
		}
		return actionResultMsg{message: strings.TrimSpace(string(output))}
	}
}

// copyYAMLToClipboard fetches the YAML for the selected resource and sends it for clipboard copy.
func (m Model) copyYAMLToClipboard() tea.Cmd {
	kctx := m.nav.Context
	ns := m.resolveNamespace()

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
			content, err := m.client.GetResourceYAML(context.Background(), kctx, itemNs, rt, name)
			return yamlClipboardMsg{content: content, err: err}
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
				content, err := m.client.GetPodYAML(context.Background(), kctx, itemNs, name)
				return yamlClipboardMsg{content: content, err: err}
			}
		}
		rt, ok := m.resolveOwnedResourceType(sel)
		if !ok {
			return func() tea.Msg {
				return yamlClipboardMsg{err: fmt.Errorf("unknown resource type: %s", sel.Kind)}
			}
		}
		return func() tea.Msg {
			content, err := m.client.GetResourceYAML(context.Background(), kctx, itemNs, rt, name)
			return yamlClipboardMsg{content: content, err: err}
		}
	case model.LevelContainers:
		podName := m.nav.OwnedName
		return func() tea.Msg {
			content, err := m.client.GetPodYAML(context.Background(), kctx, ns, podName)
			return yamlClipboardMsg{content: content, err: err}
		}
	}
	return nil
}

// exportResourceToFile saves the selected resource YAML to a file.
func (m Model) exportResourceToFile() tea.Cmd {
	kctx := m.nav.Context
	ns := m.resolveNamespace()

	var fetchYAML func() (string, string, error) // returns (yaml, kindForFilename, error)

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
		kind := strings.ToLower(rt.Kind)
		fetchYAML = func() (string, string, error) {
			content, err := m.client.GetResourceYAML(context.Background(), kctx, itemNs, rt, name)
			return content, kind, err
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
			fetchYAML = func() (string, string, error) {
				content, err := m.client.GetPodYAML(context.Background(), kctx, itemNs, name)
				return content, "pod", err
			}
		} else {
			rt, ok := m.resolveOwnedResourceType(sel)
			if !ok {
				return func() tea.Msg {
					return exportDoneMsg{err: fmt.Errorf("unknown resource type: %s", sel.Kind)}
				}
			}
			kind := strings.ToLower(rt.Kind)
			fetchYAML = func() (string, string, error) {
				content, err := m.client.GetResourceYAML(context.Background(), kctx, itemNs, rt, name)
				return content, kind, err
			}
		}
	case model.LevelContainers:
		podName := m.nav.OwnedName
		fetchYAML = func() (string, string, error) {
			content, err := m.client.GetPodYAML(context.Background(), kctx, ns, podName)
			return content, "pod", err
		}
	default:
		return nil
	}

	return func() tea.Msg {
		yaml, kind, err := fetchYAML()
		if err != nil {
			return exportDoneMsg{err: fmt.Errorf("fetching resource: %w", err)}
		}

		// Build filename: <kind>_<name>.yaml
		var name string
		switch m.nav.Level {
		case model.LevelContainers:
			name = m.nav.OwnedName
		default:
			sel := m.selectedMiddleItem()
			if sel != nil {
				name = sel.Name
			}
		}
		sanitized := strings.ReplaceAll(name, "/", "_")
		filename := fmt.Sprintf("%s_%s.yaml", kind, sanitized)

		if err := os.WriteFile(filename, []byte(yaml), 0o644); err != nil {
			return exportDoneMsg{err: fmt.Errorf("writing file: %w", err)}
		}

		abs, _ := filepath.Abs(filename)
		if abs == "" {
			abs = filename
		}
		return exportDoneMsg{path: abs}
	}
}

// clearBeforeExec wraps cmd to clear the terminal screen before running it.
// This ensures the TUI artifacts are removed when switching to interactive mode.
func clearBeforeExec(cmd *exec.Cmd) *exec.Cmd {
	// Build a shell command: clear screen with ANSI reset, then exec the original command.
	var quoted []string
	for _, arg := range cmd.Args {
		quoted = append(quoted, shellQuote(arg))
	}
	shellCmd := fmt.Sprintf(`printf '\033c' && exec %s`, strings.Join(quoted, " "))
	wrapped := exec.Command("sh", "-c", shellCmd)
	wrapped.Env = cmd.Env
	wrapped.Dir = cmd.Dir
	wrapped.Stdin = cmd.Stdin
	wrapped.Stdout = cmd.Stdout
	wrapped.Stderr = cmd.Stderr
	return wrapped
}

// shellQuote quotes a string for safe use in a shell command.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// isKubectlCommand reports whether input (after trimming) looks like a kubectl
// command. It returns true when the input starts with "kubectl " or the first
// word matches a known kubectl subcommand.
func isKubectlCommand(input string) bool {
	trimmed := strings.TrimSpace(input)
	if strings.HasPrefix(trimmed, "kubectl ") || trimmed == "kubectl" {
		return true
	}
	firstWord := strings.ToLower(strings.Fields(trimmed)[0])
	for _, sub := range kubectlSubcommands() {
		if firstWord == sub {
			return true
		}
	}
	return false
}

// executeCommandBar runs a command from the command bar. If the input looks
// like a kubectl command (starts with "kubectl " or the first word is a known
// kubectl subcommand), it is executed via kubectl with automatic --context and
// -n flags. Otherwise the input is executed as an arbitrary shell command via
// sh -c with KUBECONFIG set in the environment.
func (m Model) executeCommandBar(input string) tea.Cmd {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	client := m.client
	kubeconfigEnv := "KUBECONFIG=" + client.KubeconfigPaths()

	// --- kubectl path ---
	if isKubectlCommand(input) {
		return m.executeCommandBarKubectl(input, kubeconfigEnv)
	}

	// --- arbitrary shell command ---
	m.addLogEntry("DBG", fmt.Sprintf("$ sh -c %q", input))

	return func() tea.Msg {
		cmd := exec.Command("sh", "-c", input)
		cmd.Env = append(os.Environ(), kubeconfigEnv)
		logger.Info("Running shell command", "cmd", cmd.String())
		output, err := cmd.CombinedOutput()
		if err != nil {
			logger.Error("Shell command failed", "cmd", cmd.String(), "error", err, "output", string(output))
			return commandBarResultMsg{output: string(output), err: err}
		}
		return commandBarResultMsg{output: string(output)}
	}
}

// executeCommandBarKubectl handles the kubectl-specific execution path. It
// strips the optional "kubectl " prefix, injects --context and -n flags when
// not already present, and runs the command via the kubectl binary.
func (m Model) executeCommandBarKubectl(input, kubeconfigEnv string) tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return commandBarResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	// Strip leading "kubectl " if user typed it.
	if strings.HasPrefix(input, "kubectl ") {
		input = strings.TrimPrefix(input, "kubectl ")
	}

	args := strings.Fields(input)
	if len(args) == 0 {
		return nil
	}

	// Add context if not already specified.
	hasContext := false
	hasNamespace := false
	for _, a := range args {
		if a == "--context" {
			hasContext = true
		}
		if a == "-n" || a == "--namespace" {
			hasNamespace = true
		}
	}
	if !hasContext && m.nav.Context != "" {
		args = append(args, "--context", m.nav.Context)
	}
	ns := m.resolveNamespace()
	if !hasNamespace && ns != "" {
		args = append(args, "-n", ns)
	}

	m.addLogEntry("DBG", fmt.Sprintf("$ kubectl %s", strings.Join(args, " ")))

	return func() tea.Msg {
		cmd := exec.Command(kubectlPath, args...)
		cmd.Env = append(os.Environ(), kubeconfigEnv)
		logger.Info("Running kubectl command", "cmd", cmd.String())
		output, err := cmd.CombinedOutput()
		if err != nil {
			logger.Error("kubectl command bar execution failed", "cmd", cmd.String(), "error", err, "output", string(output))
			return commandBarResultMsg{output: string(output), err: err}
		}
		return commandBarResultMsg{output: string(output)}
	}
}

// findCustomAction looks up a custom action by kind and label from the user config.
func findCustomAction(kind, label string) (ui.CustomAction, bool) {
	actions, ok := ui.ConfigCustomActions[kind]
	if !ok {
		return ui.CustomAction{}, false
	}
	for _, ca := range actions {
		if ca.Label == label {
			return ca, true
		}
	}
	return ui.CustomAction{}, false
}

// expandCustomActionTemplate substitutes template variables in a custom action command string.
// Supported variables: {name}, {namespace}, {context}, {kind}, and any column key
// from the resource item (e.g., {nodeName}, {IP}) with the key stripped of spaces and
// lowercased for matching.
func expandCustomActionTemplate(cmdTemplate string, actx actionContext) string {
	result := cmdTemplate
	result = strings.ReplaceAll(result, "{name}", actx.name)
	result = strings.ReplaceAll(result, "{namespace}", actx.namespace)
	result = strings.ReplaceAll(result, "{context}", actx.context)
	result = strings.ReplaceAll(result, "{kind}", actx.kind)

	// Substitute column-based variables. The user writes {columnKey} where columnKey
	// matches the column's Key field (case-insensitive, spaces removed). For example,
	// a column with Key="Node" can be referenced as {Node} or {node}.
	for _, kv := range actx.columns {
		// Exact match first (e.g., {Node} for Key="Node").
		result = strings.ReplaceAll(result, "{"+kv.Key+"}", kv.Value)
		// Also support camelCase-style references (e.g., {nodeName} for Key="Node").
		lowerKey := strings.ToLower(strings.ReplaceAll(kv.Key, " ", ""))
		if lowerKey != kv.Key {
			result = strings.ReplaceAll(result, "{"+lowerKey+"}", kv.Value)
		}
	}

	return result
}

// execCustomAction runs a user-defined custom action command via sh -c.
// The command is executed with the terminal handed over via tea.ExecProcess,
// allowing interactive commands to work properly.
func (m Model) execCustomAction(expandedCmd string) tea.Cmd {
	cmd := exec.Command("sh", "-c", expandedCmd)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
	logger.Info("Running custom action", "cmd", cmd.String())

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			logger.Error("Custom action failed", "cmd", cmd.String(), "error", err)
			return actionResultMsg{err: fmt.Errorf("custom action failed: %w", err)}
		}
		return actionResultMsg{message: "Custom action completed"}
	})
}
