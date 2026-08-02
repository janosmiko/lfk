package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newReadsSchedulerModel returns a baseFinalModel wired to a fresh, worker-less
// scheduler. With workers stopped the submitted task stays queued, so a wired
// loader can be observed synchronously via QueueLen — the same technique as
// TestLoadResourcesRegistersTaskSynchronously.
func newReadsSchedulerModel() Model {
	m := baseFinalModel()
	m.scheduler = scheduler.New(0)
	m.scheduler.StopWorkers()
	return m
}

// TestK8sReadsAreScheduled asserts every previously-unwired K8s read loader now
// dispatches through scheduleK8sCall (which Submits synchronously while the Cmd
// is built), rather than running in a raw closure that bypasses the worker pool,
// priority lanes, coalescing, and gen-based cancellation.
func TestK8sReadsAreScheduled(t *testing.T) {
	t.Run("loadSecretData", func(t *testing.T) {
		m := newReadsSchedulerModel()
		assertSchedulesOne(t, m, m.loadSecretData())
	})
	t.Run("loadConfigMapData", func(t *testing.T) {
		m := newReadsSchedulerModel()
		assertSchedulesOne(t, m, m.loadConfigMapData())
	})
	t.Run("loadLabelData", func(t *testing.T) {
		m := newReadsSchedulerModel()
		assertSchedulesOne(t, m, m.loadLabelData())
	})
	t.Run("loadRevisions", func(t *testing.T) {
		m := newReadsSchedulerModel()
		assertSchedulesOne(t, m, m.loadRevisions())
	})
	t.Run("loadContainersForAction", func(t *testing.T) {
		m := newReadsSchedulerModel()
		assertSchedulesOne(t, m, m.loadContainersForAction())
	})
	t.Run("loadContainersForLogFilter", func(t *testing.T) {
		m := newReadsSchedulerModel()
		assertSchedulesOne(t, m, m.loadContainersForLogFilter())
	})
	t.Run("detectExecPodOSCmd", func(t *testing.T) {
		m := newReadsSchedulerModel()
		assertSchedulesOne(t, m, m.detectExecPodOSCmd())
	})
	t.Run("loadPodsForAction", func(t *testing.T) {
		m := newReadsSchedulerModel()
		assertSchedulesOne(t, m, m.loadPodsForAction())
	})
	t.Run("loadRightsizing", func(t *testing.T) {
		m := newReadsSchedulerModel()
		assertSchedulesOne(t, m, m.loadRightsizing())
	})
	t.Run("copyYAMLToClipboard", func(t *testing.T) {
		m := newReadsSchedulerModel()
		assertSchedulesOne(t, m, m.copyYAMLToClipboard())
	})
	t.Run("copyYAMLForScope", func(t *testing.T) {
		m := newReadsSchedulerModel()
		scope := []model.Item{{Name: "pod-1", Namespace: "default", Kind: "Pod"}}
		assertSchedulesOne(t, m, m.copyYAMLForScope(scope))
	})
	t.Run("exportResourceToFile", func(t *testing.T) {
		m := newReadsSchedulerModel()
		assertSchedulesOne(t, m, m.exportResourceToFile())
	})
	t.Run("loadDiff", func(t *testing.T) {
		m := newReadsSchedulerModel()
		rt := model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
		itemA := model.Item{Name: "pod-1", Namespace: "default", Kind: "Pod"}
		itemB := model.Item{Name: "pod-2", Namespace: "default", Kind: "Pod"}
		assertSchedulesOne(t, m, m.loadDiff(rt, itemA, itemB))
	})
	t.Run("watchArgoWorkflow", func(t *testing.T) {
		m := newReadsSchedulerModel()
		assertSchedulesOne(t, m, m.watchArgoWorkflow())
	})
	t.Run("loadAutoSyncConfig", func(t *testing.T) {
		m := newReadsSchedulerModel()
		assertSchedulesOne(t, m, m.loadAutoSyncConfig())
	})
}

// TestK8sMutationsAreScheduled asserts in-process write operations route through
// scheduleK8sCall (KindMutation) instead of trackBgTask / raw closures, so they
// share the bounded worker pool and Critical priority. Mutations NeverCoalesce,
// so unlike reads there is no coalescing hazard. Subprocess mutations (kubectl /
// helm) and bulk-loop mutations stay on trackBgTask and are not covered here.
func TestK8sMutationsAreScheduled(t *testing.T) {
	podRT := model.ResourceTypeEntry{Resource: "pods", Kind: "Pod", Namespaced: true}
	deployRT := model.ResourceTypeEntry{Resource: "deployments", Kind: "Deployment", Namespaced: true}

	t.Run("deleteResource", func(t *testing.T) {
		m := withActionCtx(baseModelWithFakeClient(), "pod-1", "default", "Pod", podRT)
		assertSchedulesOne(t, m, m.deleteResource())
	})
	t.Run("scaleResource", func(t *testing.T) {
		m := withActionCtx(baseModelWithFakeClient(), "deploy-1", "default", "Deployment", deployRT)
		assertSchedulesOne(t, m, m.scaleResource(3))
	})
	t.Run("resizePVC", func(t *testing.T) {
		m := withActionCtx(baseModelWithFakeClient(), "pvc-1", "default", "PersistentVolumeClaim", model.ResourceTypeEntry{})
		assertSchedulesOne(t, m, m.resizePVC("10Gi"))
	})
	t.Run("rollbackDeployment", func(t *testing.T) {
		m := withActionCtx(baseModelWithFakeClient(), "deploy-1", "default", "Deployment", deployRT)
		assertSchedulesOne(t, m, m.rollbackDeployment(1))
	})
	t.Run("triggerCronJob", func(t *testing.T) {
		m := withActionCtx(baseModelWithFakeClient(), "cj-1", "default", "CronJob", model.ResourceTypeEntry{})
		assertSchedulesOne(t, m, m.triggerCronJob())
	})
	t.Run("syncArgoApp", func(t *testing.T) {
		m := withActionCtx(baseModelWithFakeClient(), "my-app", "argocd", "Application", model.ResourceTypeEntry{})
		assertSchedulesOne(t, m, m.syncArgoApp(false))
	})
	t.Run("terminateArgoSync", func(t *testing.T) {
		m := withActionCtx(baseModelWithFakeClient(), "my-app", "argocd", "Application", model.ResourceTypeEntry{})
		assertSchedulesOne(t, m, m.terminateArgoSync())
	})
	t.Run("disruptNodeClaim", func(t *testing.T) {
		m := withActionCtx(baseModelWithFakeClient(), "nc-1", "", "NodeClaim", model.ResourceTypeEntry{})
		assertSchedulesOne(t, m, m.disruptNodeClaim())
	})
	t.Run("activateKnativeRevision", func(t *testing.T) {
		m := withActionCtx(baseModelWithFakeClient(), "rev-1", "default", "Revision", model.ResourceTypeEntry{})
		assertSchedulesOne(t, m, m.activateKnativeRevision())
	})
	t.Run("saveAutoSyncConfig", func(t *testing.T) {
		m := withMiddleItem(baseModelWithFakeClient(), model.Item{Name: "my-app", Namespace: "argocd"})
		assertSchedulesOne(t, m, m.saveAutoSyncConfig())
	})
	t.Run("saveSecretData", func(t *testing.T) {
		m := withMiddleItem(baseModelWithFakeClient(), model.Item{Name: "my-secret", Namespace: "default"})
		m.secretData = &model.SecretData{Data: map[string]string{"k": "v"}}
		assertSchedulesOne(t, m, m.saveSecretData())
	})
	t.Run("saveConfigMapData", func(t *testing.T) {
		m := withMiddleItem(baseModelWithFakeClient(), model.Item{Name: "my-cm", Namespace: "default"})
		m.configMapData = &model.ConfigMapData{Data: map[string]string{"k": "v"}}
		assertSchedulesOne(t, m, m.saveConfigMapData())
	})
	t.Run("saveLabelData", func(t *testing.T) {
		m := withMiddleItem(baseModelWithFakeClient(), model.Item{Name: "pod-1", Namespace: "default"})
		m.labelData = &model.LabelAnnotationData{Labels: map[string]string{"l": "v"}, Annotations: map[string]string{"a": "v"}}
		assertSchedulesOne(t, m, m.saveLabelData())
	})
}

// assertSchedulesOne fails unless cmd is non-nil and a task was Submitted
// synchronously onto the model's scheduler (QueueLen == 1) at Cmd-construction
// time — proof the loader routes through scheduleK8sCall, not a raw closure.
func assertSchedulesOne(t *testing.T, m Model, cmd tea.Cmd) {
	t.Helper()
	require.NotNil(t, cmd, "loader returned nil cmd; preconditions not met")
	assert.Equal(t, 1, m.scheduler.QueueLen(m.nav.Context),
		"loader must Submit one task synchronously via scheduleK8sCall")
}
