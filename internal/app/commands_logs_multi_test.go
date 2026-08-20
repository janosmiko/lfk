package app

import (
	"os"
	"strings"
	"testing"

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
