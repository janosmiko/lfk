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
	done bool        // true when the log stream has ended
	ch   chan string // the channel this line came from (for tab identity)
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
