package app

import (
	"time"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
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
	// seeded is true when items came from model.SeedResources rather than
	// from actual API discovery. The middle-pane handler uses this flag to
	// preserve the loading spinner while discovery is still in flight —
	// overwriting middleItems with seeds on every watch-tick refresh would
	// clobber the loader set by navigateChildCluster. The right-pane
	// preview at LevelClusters still displays seeded items so the user
	// sees *something* while hovering a context.
	seeded bool
}

type resourcesLoadedMsg struct {
	items      []model.Item
	err        error
	forPreview bool
	gen        uint64
	// silent marks this load as originating from a watch-mode refresh
	// (or another caller that set Model.suppressBgtasks). Its downstream
	// preview/metrics cmds in updateResourcesLoadedMain must also run
	// suppressed so the title-bar indicator doesn't flash every 2 seconds.
	silent bool
	// rt is the resource type the load was issued for. When forPreview is
	// true this identifies the hovered sidebar item so the preview handler
	// can prime itemCache under the drill-in navKey (context/resource) and
	// skip a redundant refetch when the user actually drills in.
	rt model.ResourceTypeEntry
}

type ownedLoadedMsg struct {
	items      []model.Item
	err        error
	forPreview bool
	gen        uint64
	silent     bool
}

type containersLoadedMsg struct {
	items      []model.Item
	err        error
	forPreview bool
	gen        uint64
	silent     bool
}

type resourceTreeLoadedMsg struct {
	tree *model.ResourceNode
	err  error
	gen  uint64
}

type namespacesLoadedMsg struct {
	context string
	items   []model.Item
	err     error
	// silent marks this load as a background cache refresh (e.g., fired
	// by ensureNamespaceCacheFresh on session restore or context open).
	// The handler must not flip m.loading in this mode because that flag
	// belongs to the middle-column / resource-types load; clearing it
	// asynchronously while discovery is still in flight causes a "No
	// items" flash between the loader and the populated list.
	silent bool
}

// yamlLoadedMsg delivers a full YAML document for the YAML view. The content
// and sections are pre-processed in the loading goroutine so the main event
// loop never spends time on indentYAMLListItems or parseYAMLSections — on
// really long CRD manifests (50k+ lines) those calls can take seconds and
// freeze the UI. Producers must call buildYAMLViewPayload before sending.
type yamlLoadedMsg struct {
	content  string        // already indented via indentYAMLListItems
	sections []yamlSection // already parsed via parseYAMLSections
	err      error
}

// previewYAMLLoadedMsg carries YAML content for the split/full preview in the
// right column. As with yamlLoadedMsg, the content is pre-indented inside the
// loading goroutine to keep the main event loop responsive.
type previewYAMLLoadedMsg struct {
	content string // already indented via indentYAMLListItems
	err     error
	gen     uint64
}

// actionResultMsg is returned after an action completes.
type actionResultMsg struct {
	message string
	err     error
	// invalidateNamespaceCache, when true on a successful action,
	// drops the current context's namespace completion cache so the
	// next command bar open reflects the mutation (e.g. `:k create
	// ns`, `:k delete ns`, or a template apply).
	invalidateNamespaceCache bool
}

// triggerCronJobMsg carries the result of triggering a CronJob.
type triggerCronJobMsg struct {
	jobName string
	err     error
}

// statusMessageExpiredMsg clears the status message after a timeout.
type statusMessageExpiredMsg struct{}

// startupTipMsg delivers a random tip to display on startup.
type startupTipMsg struct{ tip string }

// watchTickMsg triggers a periodic refresh in watch mode.
type watchTickMsg struct{}

// describeRefreshTickMsg triggers a periodic refresh in the describe viewer.
type describeRefreshTickMsg struct{}

// containerSelectMsg carries the container list for action container selection.
type containerSelectMsg struct {
	items []model.Item
	err   error
}

// dashboardLoadedMsg carries the rendered dashboard content.
type dashboardLoadedMsg struct {
	content string
	events  string // recent warning events for the right column in two-column mode
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
	done bool        // true when the log stream has ended
	ch   chan string // the channel this line came from (for tab identity)
}

// logStreamRestartMsg triggers an automatic reconnect of the log stream when
// the previous stream ended (e.g. an init container completed and the next
// one hasn't produced output yet). The ch field correlates the restart with
// the stream it was scheduled for: if m.logCh no longer points at this
// channel (user switched pods or exited logs mode), the restart is dropped.
type logStreamRestartMsg struct {
	ch chan string
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

// apiResourceDiscoveryMsg delivers the result of DiscoverAPIResources.
type apiResourceDiscoveryMsg struct {
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

// helmHistoryListMsg carries the list of Helm release revisions for the
// read-only history overlay. It is parallel to helmRevisionListMsg but routed
// to a different overlay so the user can browse without any rollback action.
type helmHistoryListMsg struct {
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
// An optional err indicates a background failure (e.g., port forward restore failed).
type portForwardUpdateMsg struct {
	err error
}

// templateApplyMsg is sent after the editor closes, carrying the temp file path for kubectl apply.
type templateApplyMsg struct {
	tmpFile     string
	context     string
	ns          string
	origModTime time.Time // modification time before editor opened; skip apply if unchanged
}

// eventTimelineMsg carries event timeline data for the overlay.
type eventTimelineMsg struct {
	events []k8s.EventInfo
	err    error
}

// canILoadedMsg carries the result of a SelfSubjectRulesReview.
type canILoadedMsg struct {
	rules      []k8s.AccessRule
	namespaces []string // namespaces queried for the rules review
	err        error
}

// canISAListMsg carries the list of ServiceAccounts and RBAC subjects for the can-i browser.
type canISAListMsg struct {
	accounts []string
	subjects []k8s.RBACSubject // users and groups from role bindings
	err      error
}

// rbacCheckMsg carries the result of an RBAC permission check.
type rbacCheckMsg struct {
	results  []k8s.RBACCheck
	kind     string
	resource string
	err      error
}

// podStartupMsg carries the result of a pod startup analysis.
type podStartupMsg struct {
	info *k8s.PodStartupInfo
	err  error
}

// alertsLoadedMsg carries the result of loading Prometheus alerts for a resource.
type alertsLoadedMsg struct {
	alerts []k8s.AlertInfo
	err    error
}

// netpolLoadedMsg carries the result of loading a network policy.
type netpolLoadedMsg struct {
	info *k8s.NetworkPolicyInfo
	err  error
}

// previewEventsLoadedMsg carries events for the preview pane.
type previewEventsLoadedMsg struct {
	events []k8s.EventInfo
	gen    uint64
}

// explainLoadedMsg carries the parsed output of kubectl explain.
type explainLoadedMsg struct {
	fields      []model.ExplainField
	description string // resource/field-level description
	title       string // e.g., "deployments.v1.apps"
	path        string // current field path
	err         error
}

// logHistoryMsg carries a batch of older log lines fetched by a one-shot kubectl logs.
type logHistoryMsg struct {
	lines     []string
	prevTotal int
	err       error
}

// logSaveAllMsg carries the result of saving all logs to a file.
type logSaveAllMsg struct {
	path string
	err  error
}

// explainRecursiveMsg carries results from a recursive kubectl explain search.
type explainRecursiveMsg struct {
	matches []model.ExplainField // matching fields with full paths
	query   string
	err     error
}

// logContainersLoadedMsg carries the container list for the log container filter overlay.
type logContainersLoadedMsg struct {
	containers []string
	err        error
}

// finalizerSearchResultMsg carries the results of a finalizer search across resources.
type finalizerSearchResultMsg struct {
	results []k8s.FinalizerMatch
	err     error
}

// finalizerRemoveResultMsg carries the result of bulk finalizer removal.
type finalizerRemoveResultMsg struct {
	succeeded int
	failed    int
	errors    []string
}

// commandBarNamesFetchedMsg carries async resource names for command bar completion.
type commandBarNamesFetchedMsg struct {
	cacheKey string
	names    []string
}
