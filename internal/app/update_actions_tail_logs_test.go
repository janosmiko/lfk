package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/ui"
)

// TestExecuteActionTailLogsSetsShortTail verifies that executeActionTailLogs
// picks up ConfigLogTailLinesShort rather than the full ConfigLogTailLines.
func TestExecuteActionTailLogsSetsShortTail(t *testing.T) {
	prev := ui.ConfigLogTailLinesShort
	t.Cleanup(func() { ui.ConfigLogTailLinesShort = prev })
	ui.ConfigLogTailLinesShort = 7

	m := newTestModelWithClient(t)
	m.actionCtx = actionContext{
		kind:      "Pod",
		name:      "my-pod",
		namespace: "default",
		context:   "test-ctx",
	}

	result, _ := m.executeAction("Tail Logs")
	mdl := result.(Model)

	assert.Equal(t, modeLogs, mdl.mode)
	assert.Equal(t, 7, mdl.logTailLines, "logTailLines should equal ConfigLogTailLinesShort")
}

// TestExecuteActionTailLogsTitleContainsTailIndicator verifies the log viewer
// title distinguishes tail mode from full logs.
func TestExecuteActionTailLogsTitleContainsTailIndicator(t *testing.T) {
	m := newTestModelWithClient(t)
	m.actionCtx = actionContext{
		kind:      "Pod",
		name:      "my-pod",
		namespace: "default",
		context:   "test-ctx",
	}

	result, _ := m.executeAction("Tail Logs")
	mdl := result.(Model)

	assert.Contains(t, mdl.logTitle, "tail", "log title should contain 'tail' for Tail Logs action")
}

// TestExecuteActionTailLogsContainerTitleContainsTailIndicator verifies that
// when a container name is set, the tail title still contains the indicator.
func TestExecuteActionTailLogsContainerTitleContainsTailIndicator(t *testing.T) {
	m := newTestModelWithClient(t)
	m.actionCtx = actionContext{
		kind:          "Pod",
		name:          "my-pod",
		containerName: "sidecar",
		namespace:     "default",
		context:       "test-ctx",
	}

	result, _ := m.executeAction("Tail Logs")
	mdl := result.(Model)

	assert.Contains(t, mdl.logTitle, "tail", "container log title should contain 'tail' for Tail Logs action")
	assert.Contains(t, mdl.logTitle, "sidecar", "container log title should contain the container name")
}

// TestExecuteActionLogsUsesConfiguredTailLines verifies that the refactored
// executeActionLogs still uses ConfigLogTailLines (not the short value).
func TestExecuteActionLogsUsesConfiguredTailLines(t *testing.T) {
	prevFull := ui.ConfigLogTailLines
	prevShort := ui.ConfigLogTailLinesShort
	t.Cleanup(func() {
		ui.ConfigLogTailLines = prevFull
		ui.ConfigLogTailLinesShort = prevShort
	})
	ui.ConfigLogTailLines = 250
	ui.ConfigLogTailLinesShort = 5

	m := newTestModelWithClient(t)
	m.actionCtx = actionContext{
		kind:      "Pod",
		name:      "my-pod",
		namespace: "default",
		context:   "test-ctx",
	}

	result, _ := m.executeAction("Logs")
	mdl := result.(Model)

	assert.Equal(t, 250, mdl.logTailLines, "full Logs must use ConfigLogTailLines")
}

// TestExecuteActionLogsFullTitleHasNoTailIndicator ensures the full Logs
// action does not add a "tail" indicator to the title.
func TestExecuteActionLogsFullTitleHasNoTailIndicator(t *testing.T) {
	m := newTestModelWithClient(t)
	m.actionCtx = actionContext{
		kind:      "Pod",
		name:      "my-pod",
		namespace: "default",
		context:   "test-ctx",
	}

	result, _ := m.executeAction("Logs")
	mdl := result.(Model)

	assert.NotContains(t, mdl.logTitle, "tail", "full Logs title must not contain 'tail'")
}

// TestExecuteActionTailLogsGroupResourceSetsPendingAction verifies that when a
// group resource (e.g., Deployment) triggers Tail Logs, the pendingAction is
// set to "Tail Logs" so the pod-select overlay can continue with the correct action.
func TestExecuteActionTailLogsGroupResourceSetsPendingAction(t *testing.T) {
	m := newTestModelWithClient(t)
	m.actionCtx = actionContext{
		kind:      "Deployment",
		name:      "my-deploy",
		namespace: "default",
		context:   "test-ctx",
	}

	// Deployment with no containerName goes to group-resource streaming path
	// (not the container selector). The model should enter modeLogs directly.
	result, _ := m.executeAction("Tail Logs")
	mdl := result.(Model)

	// For group resources, the log parent is set and modeLogs entered directly.
	assert.Equal(t, modeLogs, mdl.mode, "Deployment Tail Logs should enter modeLogs directly")
	assert.Equal(t, "Deployment", mdl.logParentKind, "logParentKind should be set for group resource")
}
