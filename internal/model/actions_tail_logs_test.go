package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// findAction searches a slice of ActionMenuItems by label and returns it.
func findAction(items []ActionMenuItem, label string) (ActionMenuItem, bool) {
	for _, item := range items {
		if item.Label == label {
			return item, true
		}
	}
	return ActionMenuItem{}, false
}

// assertTailLogsPresent verifies that Tail Logs is on "l" and Logs is on "L"
// within the provided action list. It also ensures no key collision.
func assertTailLogsPresent(t *testing.T, items []ActionMenuItem, contextName string) {
	t.Helper()

	tailLogs, hasTail := findAction(items, "Tail Logs")
	logs, hasLogs := findAction(items, "Logs")

	require.True(t, hasTail, "%s: Tail Logs action not found", contextName)
	require.True(t, hasLogs, "%s: Logs action not found", contextName)

	assert.Equal(t, "l", tailLogs.Key, "%s: Tail Logs must be on key 'l'", contextName)
	assert.Equal(t, "L", logs.Key, "%s: Logs must be on key 'L'", contextName)
}

// TestActionsForContainerTailLogs verifies container-level action remapping.
func TestActionsForContainerTailLogs(t *testing.T) {
	items := ActionsForContainer()
	assertTailLogsPresent(t, items, "ActionsForContainer")
}

// TestActionsForKindPodTailLogs verifies Pod action remapping.
func TestActionsForKindPodTailLogs(t *testing.T) {
	items := ActionsForKind("Pod")
	assertTailLogsPresent(t, items, "ActionsForKind(Pod)")
}

// TestActionsForKindServiceTailLogs verifies Service action remapping.
func TestActionsForKindServiceTailLogs(t *testing.T) {
	items := ActionsForKind("Service")
	assertTailLogsPresent(t, items, "ActionsForKind(Service)")
}

// TestActionsForKindDeploymentTailLogs verifies Deployment action remapping.
func TestActionsForKindDeploymentTailLogs(t *testing.T) {
	items := ActionsForKind("Deployment")
	assertTailLogsPresent(t, items, "ActionsForKind(Deployment)")
}

// TestActionsForKindStatefulSetTailLogs verifies StatefulSet action remapping.
func TestActionsForKindStatefulSetTailLogs(t *testing.T) {
	items := ActionsForKind("StatefulSet")
	assertTailLogsPresent(t, items, "ActionsForKind(StatefulSet)")
}

// TestActionsForKindDaemonSetTailLogs verifies DaemonSet action remapping.
func TestActionsForKindDaemonSetTailLogs(t *testing.T) {
	items := ActionsForKind("DaemonSet")
	assertTailLogsPresent(t, items, "ActionsForKind(DaemonSet)")
}

// TestActionsForKindJobTailLogs verifies Job action remapping.
func TestActionsForKindJobTailLogs(t *testing.T) {
	items := ActionsForKind("Job")
	assertTailLogsPresent(t, items, "ActionsForKind(Job)")
}

// TestActionsForKindCronJobTailLogs verifies CronJob action remapping.
func TestActionsForKindCronJobTailLogs(t *testing.T) {
	items := ActionsForKind("CronJob")
	assertTailLogsPresent(t, items, "ActionsForKind(CronJob)")
}

// TestActionsForKindWorkflowTailLogs verifies Workflow action remapping.
func TestActionsForKindWorkflowTailLogs(t *testing.T) {
	items := ActionsForKind("Workflow")
	assertTailLogsPresent(t, items, "ActionsForKind(Workflow)")
}

// TestActionsForBulkNoTailLogs ensures bulk actions are not modified.
// Bulk Logs stays on "L" and Tail Logs must not be present.
func TestActionsForBulkNoTailLogs(t *testing.T) {
	items := ActionsForBulk("")

	_, hasTail := findAction(items, "Tail Logs")
	assert.False(t, hasTail, "ActionsForBulk must not contain Tail Logs")

	logs, hasLogs := findAction(items, "Logs")
	require.True(t, hasLogs, "ActionsForBulk must still have Logs")
	assert.Equal(t, "L", logs.Key, "ActionsForBulk Logs must remain on key 'L'")
}

// TestActionsForBulkApplicationNoTailLogs ensures Application bulk actions are
// also unmodified.
func TestActionsForBulkApplicationNoTailLogs(t *testing.T) {
	items := ActionsForBulk("Application")

	_, hasTail := findAction(items, "Tail Logs")
	assert.False(t, hasTail, "ActionsForBulk(Application) must not contain Tail Logs")
}

// TestTailLogsKeyUniquenessPerKind verifies that no two actions share the same
// key within a single action list for each affected kind.
func TestTailLogsKeyUniquenessPerKind(t *testing.T) {
	kinds := []string{
		"Pod", "Service",
		"Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob",
		"Workflow",
	}
	for _, kind := range kinds {
		t.Run(kind, func(t *testing.T) {
			items := ActionsForKind(kind)
			seen := make(map[string]string, len(items))
			for _, item := range items {
				if prev, exists := seen[item.Key]; exists {
					t.Errorf("%s: key %q used by both %q and %q", kind, item.Key, prev, item.Label)
				}
				seen[item.Key] = item.Label
			}
		})
	}
}

// TestContainerActionKeyUniqueness verifies no key collision in container actions.
func TestContainerActionKeyUniqueness(t *testing.T) {
	items := ActionsForContainer()
	seen := make(map[string]string, len(items))
	for _, item := range items {
		if prev, exists := seen[item.Key]; exists {
			t.Errorf("ActionsForContainer: key %q used by both %q and %q", item.Key, prev, item.Label)
		}
		seen[item.Key] = item.Label
	}
}
