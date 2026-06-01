package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/k8s"
)

// newTestModelWithClient returns a baseModel with a minimal k8s.Client so that
// executeAction("Logs") can reach startLogStream without a nil-pointer panic.
// startLogStream will still fail (no kubectl / no real cluster), but the
// returned cmd is discarded -- we only inspect model state mutations.
func newTestModelWithClient(t *testing.T) Model {
	t.Helper()
	m := baseModel()
	c, err := k8s.NewClient("", nil)
	if err != nil {
		t.Fatalf("k8s.NewClient() for test setup: %v", err)
	}
	m.client = c
	return m
}

func TestExecuteLogsResetsLogParentKindForPods(t *testing.T) {
	m := newTestModelWithClient(t)
	// Simulate stale parent context from a previous log session.
	m.logView.parentKind = "Job"
	m.logView.parentName = "old-job"
	m.actionCtx = actionContext{
		kind:      "Pod",
		name:      "my-pod",
		namespace: "default",
		context:   "test-ctx",
	}

	result, _ := m.executeAction("Logs")
	mdl := result.(Model)

	assert.Equal(t, modeLogs, mdl.mode)
	assert.Equal(t, "", mdl.logView.parentKind, "logParentKind should be reset for direct Pod logs")
	assert.Equal(t, "", mdl.logView.parentName, "logParentName should be reset for direct Pod logs")
}

func TestExecuteLogsResetsLogParentKindForContainers(t *testing.T) {
	m := newTestModelWithClient(t)
	// Simulate stale parent context from a previous log session.
	m.logView.parentKind = "Deployment"
	m.logView.parentName = "old-deploy"
	m.actionCtx = actionContext{
		kind:          "Pod",
		name:          "my-pod",
		containerName: "web",
		namespace:     "default",
		context:       "test-ctx",
	}

	result, _ := m.executeAction("Logs")
	mdl := result.(Model)

	assert.Equal(t, modeLogs, mdl.mode)
	assert.Equal(t, "", mdl.logView.parentKind, "logParentKind should be reset for container-level Pod logs")
	assert.Equal(t, "", mdl.logView.parentName, "logParentName should be reset for container-level Pod logs")
}

func TestExecuteLogsSetsSelectedContainersForSingleContainer(t *testing.T) {
	m := newTestModelWithClient(t)
	m.actionCtx = actionContext{
		kind:          "Pod",
		name:          "my-pod",
		containerName: "web",
		namespace:     "default",
		context:       "test-ctx",
	}

	result, _ := m.executeAction("Logs")
	mdl := result.(Model)

	assert.Equal(t, []string{"web"}, mdl.logView.selectedContainers,
		"logSelectedContainers should contain the single container name")
	assert.Contains(t, mdl.logView.title, "[web]")
}

func TestExecuteLogsAllContainersHasNilSelectedContainers(t *testing.T) {
	m := newTestModelWithClient(t)
	m.actionCtx = actionContext{
		kind:      "Pod",
		name:      "my-pod",
		namespace: "default",
		context:   "test-ctx",
	}

	result, _ := m.executeAction("Logs")
	mdl := result.(Model)

	assert.Nil(t, mdl.logView.selectedContainers,
		"logSelectedContainers should be nil for all-container logs")
}

func TestExecuteLogsGroupResourceSetsLogParent(t *testing.T) {
	m := newTestModelWithClient(t)
	m.actionCtx = actionContext{
		kind:      "Job",
		name:      "my-job",
		namespace: "default",
		context:   "test-ctx",
	}

	result, _ := m.executeAction("Logs")
	mdl := result.(Model)

	// Group resources now stream all pods directly (no pod selector).
	// Parent context is still saved for pod/container re-selection from the log viewer.
	assert.Equal(t, "Job", mdl.logView.parentKind)
	assert.Equal(t, "my-job", mdl.logView.parentName)
	assert.Equal(t, modeLogs, mdl.mode,
		"group resource should go directly to log streaming")
}
