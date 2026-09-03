package app

import (
	"context"
	"time"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/k8s/localcluster"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// --- Messages ---

type stderrCapturedMsg struct {
	message string
}

// shutdownCompleteMsg signals that the asynchronous quit drain (worker
// pools, informer caches, session save) has finished, so the program can
// dispatch tea.Quit and exit cleanly.
type shutdownCompleteMsg struct{}

type contextsLoadedMsg struct {
	items []model.Item
	err   error
	// reloaded marks a list that came from a fresh read of the kubeconfig. A
	// context name can then point at another cluster or another user, so
	// per-context caches keyed by that name are no longer trustworthy.
	reloaded bool
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
	// silent mirrors resourcesLoadedMsg.silent: captured from
	// suppressBgtasks at construction time so updateResourceTypes can
	// propagate it into the loadPreview cascade. Without this, the watch
	// tick at LevelResourceTypes would flash the title-bar indicator on
	// every refresh once the preview cache shortcut is bypassed for
	// cluster-side mutations to surface.
	silent bool
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
	// belongs to the middle-column / resource-types load. Clearing it
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

// yamlBlameLoadedMsg carries the finished blame lines for the YAML viewer.
// It arrives from a separate fetch, because the YAML itself is rendered
// without managedFields. The per-line walk happens in the loader goroutine,
// like the content and section parse in buildYAMLLoadedMsg, so a 50k-line CRD
// does not stall the event loop. contentHash identifies the document the
// lines were built from, so a reply that lands after a refresh is dropped
// instead of naming the wrong owner. The length is compared with the hash,
// because the hash alone is unkeyed and a collision would attach the wrong
// lines to the document on screen.
// req numbers the fetch this reply answers, because two fetches over one
// unchanged document share a hash and would otherwise land in either order.
type yamlBlameLoadedMsg struct {
	blame       []blameLine
	req         uint64
	contentHash uint64
	contentLen  int
	err         error
}

// fieldDocLoadedMsg carries one field description read from the cluster schema.
// req numbers the fetch this reply answers: moving the cursor bumps the number,
// so a reply for the line the user already left is dropped instead of painting
// the wrong text under the new one. key names the field the text describes and
// is what the reply is cached under.
type fieldDocLoadedMsg struct {
	req   uint64
	key   fieldDocKey
	entry fieldDocEntry
	err   error
}

// fieldDocDebounceMsg fires after the cursor has rested for fieldDocDebounce.
// Without it, holding j down would spawn one kubectl process per line.
type fieldDocDebounceMsg struct {
	req uint64
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

// drainNodeResolvedMsg carries the node name resolved from a Karpenter
// NodeClaim so the drain can then stream through the shared drainNodeCmd path.
type drainNodeResolvedMsg struct {
	nodeName string
}

// statusMessageExpiredMsg clears the status message after a timeout.
type statusMessageExpiredMsg struct{}

// startupTipMsg delivers a random tip to display on startup.
type startupTipMsg struct{ tip string }

// watchTickMsg triggers a periodic refresh in watch mode. gen identifies the
// tick chain. A tick whose gen != Model.watchTickGen is from a retired chain.
type watchTickMsg struct{ gen uint64 }

type previewDebounceTickMsg struct{ gen uint64 }

// describeRefreshTickMsg triggers a periodic refresh in the describe viewer.
type describeRefreshTickMsg struct{}

// containerSelectMsg carries the container list for action container selection.
type containerSelectMsg struct {
	items []model.Item
	err   error
}

// dashboardLoadedMsg carries the rendered dashboard content.
type dashboardLoadedMsg struct {
	data    dashboardData // composed into preview/events at the current width on receipt
	content string        // optional pre-rendered override (e.g. "disabled"). Used verbatim when non-empty
	context string
}

// dashboardPartialMsg delivers one section of the dashboard fan-out to the
// renderer, which accumulates sections per-(kctx, gen). key identifies the
// section ("nodes", "pods", ..., or "pinned:<group/resource>"), total is the
// number of sections this load fanned out to (6 fixed + one per pinned
// summary), so the accumulator knows when the set is complete. Once all
// expected sections have arrived, a dashboardLoadedMsg is dispatched for
// downstream consumers.
type dashboardPartialMsg struct {
	context string
	key     string
	// total seeds dashboardAccumulator.expected on the first section to
	// arrive for a (context, gen) key. Invariant: one accumulator per
	// fan-out — a new fan-out for the same (context, gen) (e.g. a reload
	// with a different pin count while the prior one is still in flight)
	// must start from a clean accumulator, not merge into the old one's
	// stale expected count. loadDashboardFor enforces this by evicting any
	// pre-existing accumulator for its (context, gen) before returning.
	total int
	gen   uint64
	data  dashboardData
}

// monitoringDashboardMsg carries the rendered monitoring dashboard content
// plus the raw alert payload it was built from. The raw payload is retained so
// the dashboard can be re-rendered on a theme change or resize without
// re-querying Prometheus (see recomposeMonitoring).
type monitoringDashboardMsg struct {
	content string          // pre-rendered body for immediate display
	alerts  []k8s.AlertInfo // raw payload retained for theme/resize recompose
	errMsg  string          // non-empty when the monitoring backend was unreachable
	context string
}

// yamlClipboardMsg carries serialized content to be copied to the clipboard.
// format is one of "yaml" (default), "json", "table". Empty format is treated
// as "yaml" for back-compat with existing call sites that haven't been
// updated. count is the number of items joined into content (1 = single).
type yamlClipboardMsg struct {
	content string
	count   int
	format  string
	err     error
}

// exportTemplateReadyMsg carries a manifest stripped to a template, ready for
// the destination picker.
type exportTemplateReadyMsg struct {
	name string
	kind string
	// namespace is "" for a cluster-scoped resource.
	namespace string
	// raw is the fetched document, stripped when the picker opens so a later
	// category toggle can re-strip it.
	raw string
	err error
}

// exportDoneMsg carries the result of exporting a resource to a file.
type exportDoneMsg struct {
	path string
	// note is appended to the success line. Used by the template export to say
	// that Secret values were redacted, so nobody finds out by pasting a blank.
	note string
	err  error
}

// logLineMsg carries a single line of log output from kubectl.
type logLineMsg struct {
	line string
	done bool        // true when the log stream has ended
	ch   chan string // the channel this line came from (for tab identity)
}

// previewLogLineMsg carries one line for the right-pane live-log preview. ch
// correlates it with the active preview stream so a stale stream's lines are
// dropped after a pod switch.
type previewLogLineMsg struct {
	line string
	done bool
	ch   chan string
}

// logStreamRestartMsg triggers an automatic reconnect of the log stream when
// the previous stream ended (e.g. an init container completed and the next
// one hasn't produced output yet). The ch field correlates the restart with
// the stream it was scheduled for: if m.logView.ch no longer points at this
// channel (user switched pods or exited logs mode), the restart is dropped.
type logStreamRestartMsg struct {
	ch chan string
}

// previewLogRestartMsg triggers an automatic reconnect of the right-pane live-log
// preview stream after a short delay. ch correlates it with the preview stream
// that ended: if m.previewLog.ch no longer matches (user switched pods, toggled
// off, or a new stream started), the restart is silently dropped.
type previewLogRestartMsg struct {
	ch chan string
}

// previewLogHistoryMsg carries a batch of older log lines fetched by a one-shot
// kubectl logs (no -f). podKey correlates the result to the pod that was active
// when the fetch was issued. Stale results are dropped on receipt.
type previewLogHistoryMsg struct {
	podKey string
	lines  []string
	err    error
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

// containerMetricsEnrichedMsg carries per-container metrics, keyed by name.
type containerMetricsEnrichedMsg struct {
	metrics map[string]k8s.ContainerUsage
	gen     uint64
}

// nodeUptimeEnrichedMsg carries Prometheus node uptimes to enrich the middle
// pane items. uptimes separates name-keyed from address-keyed (IP/hostname)
// results (see k8s.GetNodeUptimes) since node_exporter series aren't
// reliably keyed by node name, and a name/address collision must not
// silently overwrite either node's uptime.
type nodeUptimeEnrichedMsg struct {
	uptimes k8s.NodeUptimes
	gen     uint64
}

// secretDataLoadedMsg carries the fetched secret data.
type secretDataLoadedMsg struct {
	data *model.SecretData
	err  error
}

// rightsizingLoadedMsg carries the lazily-fetched right-sizing
// recommendations. The handler shows the result only when key still
// names the view on screen. A successful result is cached either way.
type rightsizingLoadedMsg struct {
	key  string // cache key the fetch was dispatched for
	data *model.Rightsizing
	err  error
}

// rightsizingStrategiesProbedMsg carries the result of an async probe
// for the strategies usable on the current workload. The probe is
// dispatched as a tea.Cmd from executeActionRightsizing because
// AvailableRightsizingStrategies internally calls findVPA which makes
// blocking dynamic-client List requests — running it on the Bubble Tea
// update loop freezes the UI on every overlay open.
//
// The handler reconciles available + the current strategy: if the
// sticky strategy is still in the probe result it's kept. Otherwise
// the picker re-seeds via pickRightsizingStrategy and a fresh load is
// dispatched (the in-memory data was for the wrong strategy).
//
// `generation` is checked against m.rightsizing.gen so a stale probe
// response from a previous overlay open is dropped.
type rightsizingStrategiesProbedMsg struct {
	available  []model.RightsizingStrategy
	generation int
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
	origModTime time.Time // modification time before editor opened. Skip apply if unchanged
}

// eventTimelineMsg carries event timeline data for the overlay.
type eventTimelineMsg struct {
	events []k8s.EventInfo
	err    error
}

type canIContextRules struct {
	context    string
	rules      []k8s.AccessRule
	namespaces []string
}

// canILoadedMsg carries the result of a SelfSubjectRulesReview.
type canILoadedMsg struct {
	rules        []k8s.AccessRule
	namespaces   []string // namespaces queried for the rules review
	contextRules []canIContextRules
	union        bool
	err          error
	roleRules    bool // true when rules come from a Role/ClusterRole spec, not SSR
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

// crashInvestigationMsg carries the result of a CrashLoopBackOff investigation.
type crashInvestigationMsg struct {
	info *k8s.CrashInvestigation
	err  error
}

// syncWaveTimelineMsg carries the result of one GetSyncWaveTimeline call.
// token must match Model.syncWave.token for the result to be applied —
// otherwise the overlay was closed and reopened during the fetch and
// applying the data would clobber the new session.
type syncWaveTimelineMsg struct {
	info  *k8s.SyncWaveTimeline
	err   error
	token uint64
}

// syncWaveTickMsg is the auto-refresh tick. token works the same way:
// stale ticks from a closed overlay are dropped.
type syncWaveTickMsg struct {
	token uint64
}

// syncWaveSpinnerTickMsg cycles the loading spinner glyph in the overlay
// header while the wave-annotation fan-out is in flight. Same token
// guard: stale ticks from a previous overlay session are dropped. The
// handler stops issuing new ticks the moment data.Loading flips to false
// so no goroutines accumulate after the full fetch lands.
type syncWaveSpinnerTickMsg struct {
	token uint64
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

// netpolsForResourceLoadedMsg carries the network policies affecting a pod or
// service, for the "Network Policies" action.
type netpolsForResourceLoadedMsg struct {
	info *k8s.NetpolsForResource
	err  error
}

// previewEventsLoadedMsg carries events for the preview pane.
type previewEventsLoadedMsg struct {
	events []k8s.EventInfo
	gen    uint64
}

// explainLoadedMsg carries the parsed output of kubectl explain. gen is the
// explain session it was started for. Handlers drop a reply whose gen no
// longer matches m.explainGen. See explainSessionState.
type explainLoadedMsg struct {
	fields      []model.ExplainField
	description string // resource/field-level description
	title       string // e.g., "deployments.v1.apps"
	path        string // current field path
	gen         uint64
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
	gen     uint64 // explain session, see explainLoadedMsg
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

// previewSecretDataLoadedMsg carries lazily-fetched secret data for the hover
// preview pane at LevelResources. The gen field guards against stale responses;
// handlers must discard messages where gen != m.requestGen.
type previewSecretDataLoadedMsg struct {
	gen  uint64
	ctx  string
	ns   string
	name string
	data *model.SecretData
	err  error
}

// previewServiceEndpointsLoadedMsg carries the lazily-fetched
// EndpointSlice rollup for the hovered Service at LevelResources.
// Same gen guard as the secret variant — handlers discard messages
// where gen != m.requestGen so a fresh hover doesn't get clobbered by
// the response from the previous one.
//
// fromCache distinguishes a stale-while-revalidate cache emit from a
// fresh-fetch response. Cache emits use the flag to decline writing
// the cache and to skip the inject when a fresher fetch has already
// updated it (the unlikely race where the fresh response beats the
// cache emit's tea.Batch goroutine to the runtime).
type previewServiceEndpointsLoadedMsg struct {
	gen       uint64
	ctx       string
	ns        string
	name      string
	data      *k8s.ServiceEndpoints
	err       error
	fromCache bool
}

// orphanCacheKey identifies a slot in Model.orphanCache. namespace == ""
// means cluster-wide (the overlay path). A non-empty namespace is the
// filter-preset path scoped to a single namespace.
type orphanCacheKey struct {
	kubeContext string
	namespace   string
}

// orphanInflight is the per-key bookkeeping for an in-flight scan.
// gen lets handleOrphansLoaded drop a superseded result. Cancel lets
// invalidators stop the scan immediately when the user switches
// namespace/context or refreshes — without it, a stale scan could
// repopulate the cache after invalidation.
type orphanInflight struct {
	gen    uint64
	cancel context.CancelFunc
}

// orphansLoadedMsg carries the result of a DetectOrphans run. Err may be
// non-nil while Report holds partial data — the UI renders a banner and
// still shows what was returned. gen is checked against the recorded
// inflight entry so cancelled / superseded scans don't write back.
type orphansLoadedMsg struct {
	key    orphanCacheKey
	gen    uint64
	report k8s.OrphanReport
	err    error
}

// --- Local cluster manager messages ---

// localClustersDetectedMsg is the result of a fan-out List() across
// every installed local-cluster provider. gen guards against stale
// fetches (the manager bumps Model.localClusterState.gen on every
// open / refresh, and the handler drops msgs with mismatched gen).
type localClustersDetectedMsg struct {
	gen            uint64
	clusters       []localcluster.Cluster
	providerErrors map[string]string // provider name -> error message
}

// localClusterCreatedMsg carries the result of a create-cluster wizard
// submission. Emitted by createLocalClusterCmd after Provider.Create
// returns. The handler reloads the manager table and contexts list.
// gen guards against late results landing after the manager closed —
// without it a 2-minute create that completes after the user navigated
// away would silently mutate localClusterState.creating + .info.
type localClusterCreatedMsg struct {
	gen      uint64
	provider string
	name     string
	err      error
}

// localClusterMutatedMsg carries the result of a Start/Stop/Delete
// action on an existing local cluster. verb names the action so the
// status-bar message can render uniformly. gen guards against late
// results landing after the manager closed (same reasoning as
// localClusterCreatedMsg).
type localClusterMutatedMsg struct {
	gen      uint64
	provider string
	name     string
	verb     string // "started" | "stopped" | "deleted"
	err      error
}
