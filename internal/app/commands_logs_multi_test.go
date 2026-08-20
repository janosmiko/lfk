package app

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartMultiLogStream_AllFail_ReportsStatusAndKeepsMode(t *testing.T) {
	m := basePush80Model()
	m.mode = modeExplorer
	t.Setenv("KUBECTL_BIN", "/nonexistent/kubectl")

	items := []model.Item{
		{Name: "pod-1", Namespace: "default", Kind: "Pod"},
		{Name: "pod-2", Namespace: "default", Kind: "Pod"},
		{Name: "pod-3", Namespace: "default", Kind: "Pod"},
	}

	updated, cmd := m.startMultiLogStream(items)
	mdl, ok := updated.(*Model)
	require.True(t, ok)

	assert.Equal(t, modeExplorer, mdl.mode, "must not enter the log viewer when every stream fails")
	assert.True(t, mdl.statusMessageErr)
	assert.True(t, strings.HasPrefix(mdl.statusMessage, "Failed to start logs for 3 resources:"),
		"the count and the appended cause both belong in the message, got %q", mdl.statusMessage)
	require.NotNil(t, cmd)
}

// Pins the kubectl args restartMultiLogStream builds through the shared
// startMultiLogItem helper, so the extraction can't silently drop a flag.
func TestRestartMultiLogStream_BuildsExpectedArgsForNonPodItem(t *testing.T) {
	argvPath := fakeKubectl(t, `
if [ "$1" = "logs" ]; then
  echo "restarted log line"
fi
`)
	m := basePush80Model()
	m.mode = modeLogs
	m.logView.isMulti = true
	m.logView.previous = true
	m.logView.tailLines = 50
	m.logView.multiItems = []model.Item{
		{Name: "worker", Namespace: "custom-ns", Kind: "Deployment"},
	}

	updated, cmd := m.restartMultiLogStream()
	require.NotNil(t, cmd)
	msg := cmd()
	logMsg, ok := msg.(logLineMsg)
	require.True(t, ok)
	assert.Equal(t, "restarted log line", logMsg.line)
	assert.NotNil(t, updated.logView.cancel)

	argv, err := os.ReadFile(argvPath)
	require.NoError(t, err)
	got := string(argv)
	assert.Contains(t, got, "logs\ndeployment/worker", "non-Pod kinds must use kind/name as the resource ref")
	assert.Contains(t, got, "--previous", "logView.previous must map to --previous, not -f")
	assert.NotContains(t, got, "-f\n")
	assert.Contains(t, got, "--tail=50")
	assert.Contains(t, got, "--timestamps")
	assert.Contains(t, got, "-n\ncustom-ns")
	assert.Contains(t, got, "--context\ntest-ctx")
}

// Drives the real Bubble Tea Update loop for a two-pod multi-log stream,
// through both the initial start and a restart, and checks the rendered view
// (not a raw message) carries each pod's kubectl-prefixed line.
func TestMultiLogStream_UpdateAndRestart_RendersLinesFromBothItems(t *testing.T) {
	fakeKubectl(t, `
if [ "$1" = "logs" ]; then
  case "$2" in
    pod-a) echo "[pod/pod-a] hello from pod-a" ;;
    pod-b) echo "[pod/pod-b] hello from pod-b" ;;
  esac
fi
`)
	m := basePush80Model()
	items := []model.Item{
		{Name: "pod-a", Namespace: "default", Kind: "Pod"},
		{Name: "pod-b", Namespace: "default", Kind: "Pod"},
	}

	started, cmd := m.startMultiLogStream(items)
	mdl, ok := started.(*Model)
	require.True(t, ok)
	require.NotNil(t, cmd)

	current := drainMultiLogLines(t, *mdl, cmd, 2)
	rendered := stripANSI(current.View().Content)
	assert.Contains(t, rendered, "[pod/pod-a] hello from pod-a")
	assert.Contains(t, rendered, "[pod/pod-b] hello from pod-b")

	// Mirror handleLogKeyC's own restart flow: clear the buffer first so the
	// post-restart assertion only passes if the restarted streams actually
	// deliver a line each, not because old content lingered.
	current.resetLogBuffer()
	restarted, restartCmd := current.restartMultiLogStream()
	require.NotNil(t, restartCmd)

	final := drainMultiLogLines(t, restarted, restartCmd, 2)
	renderedAfterRestart := stripANSI(final.View().Content)
	assert.Contains(t, renderedAfterRestart, "[pod/pod-a] hello from pod-a")
	assert.Contains(t, renderedAfterRestart, "[pod/pod-b] hello from pod-b")
}

// drainMultiLogLines feeds messages from cmd through m.Update until n
// distinct log lines have been observed, returning the resulting model.
func drainMultiLogLines(t *testing.T, m Model, cmd tea.Cmd, n int) Model {
	t.Helper()
	seen := map[string]bool{}
	for i := 0; i < n*3 && len(seen) < n; i++ {
		require.NotNil(t, cmd, "stream ended after %d of %d expected lines", len(seen), n)
		msg := cmd()
		lm, ok := msg.(logLineMsg)
		if !ok || lm.done {
			continue
		}
		seen[lm.line] = true
		var next tea.Model
		next, cmd = m.Update(msg)
		m = next.(Model)
	}
	require.Len(t, seen, n, "expected a line from each of the %d streams", n)
	return m
}
