package app

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/model"
)

var errPluginExit = errors.New("getting credentials: exec: executable kubelogin failed with exit code 1")

func discoveryFailure(t *testing.T, m Model, ctx string) Model {
	t.Helper()
	mdl, cmd := m.updateAPIResourceDiscovery(apiResourceDiscoveryMsg{context: ctx, err: errPluginExit})
	require.NotNil(t, cmd)
	require.True(t, mdl.execAuthTried[ctx], "guard must be set for %s", ctx)
	return mdl
}

func TestExecAuth_DiscoveryErrorSuspendsOnce(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes

	m = discoveryFailure(t, m, "test-ctx")

	assert.Nil(t, m.maybeExecAuth("test-ctx", errPluginExit))
}

func TestExecAuth_NoSuspend(t *testing.T) {
	tests := []struct {
		name string
		mod  func(m *Model)
		ctx  string
		err  error
	}{
		{"cluster list hover", func(m *Model) { m.nav.Level = model.LevelClusters }, "test-ctx", errPluginExit},
		{"other context", nil, "other-ctx", errPluginExit},
		{"union sentinel", func(m *Model) { m.nav.Context = UnionContextSentinel }, UnionContextSentinel, errPluginExit},
		{"executable missing", nil, "test-ctx", errors.New("exec: executable kubelogin not found")},
		{"demo", func(m *Model) { m.demoMode = true }, "test-ctx", errPluginExit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := basePush80Model()
			m.nav.Level = model.LevelResourceTypes
			if tt.mod != nil {
				tt.mod(&m)
			}
			assert.Nil(t, m.maybeExecAuth(tt.ctx, tt.err))
			assert.False(t, m.execAuthTried[tt.ctx])
		})
	}
}

func TestExecAuth_HooksIgnoreOtherContextAndClusterLevel(t *testing.T) {
	m := basePush80Model()
	m.requestGen = 1
	m.nav.Level = model.LevelResourceTypes

	mdl, _ := m.updateResourcesLoaded(resourcesLoadedMsg{gen: 1, context: "other-ctx", err: errPluginExit})
	assert.Empty(t, mdl.(Model).execAuthTried)

	m.nav.Level = model.LevelClusters
	mdl, _ = m.updateNamespacesLoaded(namespacesLoadedMsg{context: "test-ctx", err: errPluginExit})
	assert.Empty(t, mdl.(Model).execAuthTried)
}

func TestExecAuth_HooksSetGuardForCurrentContext(t *testing.T) {
	base := basePush80Model()
	base.requestGen = 1
	base.nav.Level = model.LevelResourceTypes

	m, _ := base.updateAPIResourceDiscovery(apiResourceDiscoveryMsg{context: "test-ctx", err: errPluginExit})
	assert.True(t, m.execAuthTried["test-ctx"], "discovery hook")

	mdl, _ := base.updateResourcesLoaded(resourcesLoadedMsg{gen: 1, context: "test-ctx", err: errPluginExit})
	assert.True(t, mdl.(Model).execAuthTried["test-ctx"], "resources hook")

	mdl, _ = base.updateNamespacesLoaded(namespacesLoadedMsg{context: "test-ctx", err: errPluginExit})
	assert.True(t, mdl.(Model).execAuthTried["test-ctx"], "namespaces hook")
}

func TestExecAuthDone_RepeatedFailureDoesNotLoop(t *testing.T) {
	m := basePush80Model()
	m.scheduler = scheduler.New(0)
	m.nav.Level = model.LevelResourceTypes
	require.NotNil(t, m.maybeExecAuth("test-ctx", errPluginExit))

	got, _ := m.updateExecAuthDone(execAuthDoneMsg{context: "test-ctx"})
	m = got.(Model)
	assert.Nil(t, m.maybeExecAuth("test-ctx", errPluginExit))
}

func TestExecAuthDone_OtherContextOnlySetsStatus(t *testing.T) {
	m := basePush80Model()
	m.scheduler = scheduler.New(0)
	gen := m.requestGen
	got, _ := m.updateExecAuthDone(execAuthDoneMsg{context: "other-ctx"})
	assert.Equal(t, gen, got.(Model).requestGen, "no refresh for a context that is not current")
	assert.Empty(t, got.(Model).scheduler.QueueSnapshot())
}

func TestExecAuth_ResourcesLoadedErrorSuspends(t *testing.T) {
	m := basePush80Model()
	m.requestGen = 1
	mdl, cmd := m.updateResourcesLoaded(resourcesLoadedMsg{gen: 1, context: "test-ctx", err: errPluginExit})
	assert.NotNil(t, cmd)
	assert.True(t, mdl.(Model).execAuthTried["test-ctx"])
}

func TestExecAuth_DiscoverySuccessKeepsGuardRefreshClears(t *testing.T) {
	m := basePush80Model()
	m.execAuthTried = map[string]bool{"test-ctx": true}
	mdl, _ := m.updateAPIResourceDiscovery(apiResourceDiscoveryMsg{context: "test-ctx"})
	assert.True(t, mdl.execAuthTried["test-ctx"])

	m.execAuthTried = map[string]bool{"test-ctx": true}
	got, _, _ := m.handleExplorerDirectActionKeys(tea.KeyPressMsg{Code: 'R', Text: "R"})
	assert.False(t, got.(Model).execAuthTried["test-ctx"])
}

func TestExecAuth_CachedDiscoveryDoesNotRearmPrompt(t *testing.T) {
	m := basePush80Model()
	m.scheduler = scheduler.New(0)
	m.nav.Level = model.LevelResourceTypes
	m.requestGen = 1
	mdl, cmd := m.updateResourcesLoaded(resourcesLoadedMsg{gen: 1, context: "test-ctx", err: errPluginExit})
	require.NotNil(t, cmd)
	m = mdl.(Model)

	m, _ = m.updateAPIResourceDiscovery(apiResourceDiscoveryMsg{context: "test-ctx"})
	got, _ := m.updateExecAuthDone(execAuthDoneMsg{context: "test-ctx"})
	m = got.(Model)

	assert.Nil(t, m.maybeExecAuth("test-ctx", errPluginExit))
}

func TestExecAuthDone_SuccessQueuesDiscovery(t *testing.T) {
	m := basePush80Model()
	m.scheduler = scheduler.New(0)
	got, cmd := m.updateExecAuthDone(execAuthDoneMsg{context: "test-ctx"})
	require.NotNil(t, cmd)
	found := false
	for _, e := range got.(Model).scheduler.QueueSnapshot() {
		if e.Kind == scheduler.KindAPIDiscovery {
			found = true
		}
	}
	assert.True(t, found, "discovery must be queued after successful auth")
}

func TestExecAuthDone_ErrorKeepsGuard(t *testing.T) {
	m := basePush80Model()
	m.execAuthTried = map[string]bool{"test-ctx": true}
	got, _ := m.updateExecAuthDone(execAuthDoneMsg{context: "test-ctx", err: errors.New("boom")})
	assert.True(t, got.(Model).execAuthTried["test-ctx"])
	assert.True(t, got.(Model).statusMessageErr)
}

func TestExecAuthCmd_RunSuccess(t *testing.T) {
	argvPath := fakeKubectl(t, "echo 'Password: ' >&2")
	var out bytes.Buffer
	c := &execAuthCmd{context: "ctx-a", kubectlContext: "ctx-a"}
	c.SetStdin(strings.NewReader(""))
	c.SetStdout(&out)
	require.NoError(t, c.Run())
	assert.Contains(t, out.String(), "lfk: ctx-a auth plugin needs input")
	assert.Contains(t, out.String(), "Password:", "plugin stderr must reach the stdout writer")
	assert.NotContains(t, out.String(), "Press Enter")
	argv, err := os.ReadFile(argvPath)
	require.NoError(t, err)
	assert.Contains(t, string(argv), "--request-timeout=5m", "a stalled API server must not keep lfk suspended forever")
}

func TestExecAuthCmd_RunFailureWaitsForEnter(t *testing.T) {
	fakeKubectl(t, "exit 1")
	var out bytes.Buffer
	in := strings.NewReader("\nrest")
	c := &execAuthCmd{context: "ctx-a", kubectlContext: "ctx-a"}
	c.SetStdin(in)
	c.SetStdout(&out)
	require.Error(t, c.Run())
	assert.Contains(t, out.String(), "Press Enter to return to lfk")
}

// countingScript counts its runs in a file and exits 1 until run number succeedOn.
func countingScript(counter string, succeedOn int) string {
	return `n=$(cat ` + counter + ` 2>/dev/null || echo 0)
n=$((n+1))
echo $n > ` + counter + `
if [ "$n" -lt ` + strconv.Itoa(succeedOn) + ` ]; then echo "bad password" >&2; exit 1; fi`
}

func TestExecAuthCmd_RunRetriesUntilSuccess(t *testing.T) {
	counter := filepath.Join(t.TempDir(), "count")
	fakeKubectl(t, countingScript(counter, 3))
	var out bytes.Buffer
	c := &execAuthCmd{context: "ctx-a", kubectlContext: "ctx-a"}
	c.SetStdin(strings.NewReader(""))
	c.SetStdout(&out)
	require.NoError(t, c.Run())
	assert.Contains(t, out.String(), "Attempt 2/3")
	assert.Contains(t, out.String(), "Attempt 3/3")
	assert.NotContains(t, out.String(), "Press Enter")
}

func TestExecAuthCmd_RunFailsAfterMaxAttempts(t *testing.T) {
	counter := filepath.Join(t.TempDir(), "count")
	fakeKubectl(t, countingScript(counter, 100))
	var out bytes.Buffer
	c := &execAuthCmd{context: "ctx-a", kubectlContext: "ctx-a"}
	c.SetStdin(strings.NewReader("\n"))
	c.SetStdout(&out)
	require.Error(t, c.Run())
	got, err := os.ReadFile(counter)
	require.NoError(t, err)
	assert.Equal(t, "3", strings.TrimSpace(string(got)))
	assert.Contains(t, out.String(), "Press Enter to return to lfk")
}

func TestExecAuth_NilGuardMapSetOnFailedLoad(t *testing.T) {
	m := basePush80Model()
	m.execAuthTried = nil
	m.requestGen = 1
	mdl, _ := m.updateResourcesLoaded(resourcesLoadedMsg{gen: 1, context: "test-ctx", err: errPluginExit})
	assert.True(t, mdl.(Model).execAuthTried["test-ctx"])

	m.execAuthTried = nil
	m.nav.Level = model.LevelResourceTypes
	mdl, _ = m.updateNamespacesLoaded(namespacesLoadedMsg{context: "test-ctx", err: errPluginExit})
	assert.True(t, mdl.(Model).execAuthTried["test-ctx"])
}

func TestExecAuth_LoadSuccessKeepsGuard(t *testing.T) {
	m := basePush80Model()
	m.requestGen = 1
	m.execAuthTried = map[string]bool{"test-ctx": true}
	mdl, _ := m.updateResourcesLoaded(resourcesLoadedMsg{gen: 1, context: "test-ctx"})
	assert.True(t, mdl.(Model).execAuthTried["test-ctx"])
}

func TestExecAuth_NavigateChildClusterClearsGuard(t *testing.T) {
	m := basePush80Model()
	m.execAuthTried = map[string]bool{"cluster-a": true}
	got, _ := m.navigateChildCluster(&model.Item{Name: "cluster-a"})
	assert.False(t, got.(Model).execAuthTried["cluster-a"])
}

func TestExecAuthCmd_NoKubeconfigKeepsInheritedEnv(t *testing.T) {
	t.Setenv("KUBECONFIG", "/inherited/kubeconfig")
	out := filepath.Join(t.TempDir(), "env")
	fakeKubectl(t, `printf '%s' "$KUBECONFIG" > `+out)
	c := &execAuthCmd{context: "ctx-a", kubectlContext: "ctx-a"}
	c.SetStdin(strings.NewReader(""))
	c.SetStdout(io.Discard)
	require.NoError(t, c.Run())
	got, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Equal(t, "/inherited/kubeconfig", string(got))
}
