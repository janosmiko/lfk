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
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

var errPluginExit = errors.New("getting credentials: exec: executable kubelogin failed with exit code 1")

func discoveryFailure(t *testing.T, m Model, ctx string) Model {
	t.Helper()
	mdl, cmd := m.updateAPIResourceDiscovery(apiResourceDiscoveryMsg{context: ctx, err: errPluginExit})
	require.NotNil(t, cmd)
	require.True(t, mdl.execAuth.tried[ctx], "guard must be set for %s", ctx)
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
			assert.False(t, m.execAuth.tried[tt.ctx])
		})
	}
}

func TestExecAuth_HooksIgnoreOtherContextAndClusterLevel(t *testing.T) {
	m := basePush80Model()
	m.requestGen = 1
	m.nav.Level = model.LevelResourceTypes

	mdl, _ := m.updateResourcesLoaded(resourcesLoadedMsg{gen: 1, context: "other-ctx", err: errPluginExit})
	assert.Empty(t, mdl.(Model).execAuth.tried)

	m.nav.Level = model.LevelClusters
	mdl, _ = m.updateNamespacesLoaded(namespacesLoadedMsg{context: "test-ctx", err: errPluginExit})
	assert.Empty(t, mdl.(Model).execAuth.tried)
}

func TestExecAuth_HooksSetGuardForCurrentContext(t *testing.T) {
	base := basePush80Model()
	base.requestGen = 1
	base.nav.Level = model.LevelResourceTypes

	m, _ := base.updateAPIResourceDiscovery(apiResourceDiscoveryMsg{context: "test-ctx", err: errPluginExit})
	assert.True(t, m.execAuth.tried["test-ctx"], "discovery hook")

	mdl, _ := base.updateResourcesLoaded(resourcesLoadedMsg{gen: 1, context: "test-ctx", err: errPluginExit})
	assert.True(t, mdl.(Model).execAuth.tried["test-ctx"], "resources hook")

	mdl, _ = base.updateNamespacesLoaded(namespacesLoadedMsg{context: "test-ctx", err: errPluginExit})
	assert.True(t, mdl.(Model).execAuth.tried["test-ctx"], "namespaces hook")
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
	assert.True(t, mdl.(Model).execAuth.tried["test-ctx"])
}

func TestExecAuth_DiscoverySuccessKeepsGuardRefreshClears(t *testing.T) {
	m := basePush80Model()
	m.execAuth.tried = map[string]bool{"test-ctx": true}
	mdl, _ := m.updateAPIResourceDiscovery(apiResourceDiscoveryMsg{context: "test-ctx"})
	assert.True(t, mdl.execAuth.tried["test-ctx"])

	m.execAuth.tried = map[string]bool{"test-ctx": true}
	got, _, _ := m.handleExplorerDirectActionKeys(tea.KeyPressMsg{Code: 'R', Text: "R"})
	assert.False(t, got.(Model).execAuth.tried["test-ctx"])
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
	m.execAuth.tried = map[string]bool{"test-ctx": true}
	got, _ := m.updateExecAuthDone(execAuthDoneMsg{context: "test-ctx", err: errors.New("boom")})
	assert.True(t, got.(Model).execAuth.tried["test-ctx"])
	assert.True(t, got.(Model).statusMessageErr)
}

func kubectlLogin() loginCommand {
	return resolveLoginCommand("ctx-a", "ctx-a", k8s.ExecPluginInfo{}, false, nil)
}

func TestExecAuthCmd_RunSuccess(t *testing.T) {
	argvPath := fakeKubectl(t, "echo 'Password: ' >&2")
	var out bytes.Buffer
	c := &execAuthCmd{context: "ctx-a", login: kubectlLogin()}
	c.SetStdin(strings.NewReader(""))
	c.SetStdout(&out)
	require.NoError(t, c.Run())
	assert.Contains(t, out.String(), "lfk: logging in to ctx-a: kubectl get --raw /version --context ctx-a")
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
	c := &execAuthCmd{context: "ctx-a", login: kubectlLogin()}
	c.SetStdin(in)
	c.SetStdout(&out)
	require.Error(t, c.Run())
	assert.Contains(t, out.String(), "Press Enter to return to lfk")
}

// countingScript counts its runs in a file and exits 1 until run number succeedOn.
// The failure line mimics kubectl output for a failed exec credential plugin.
func countingScript(counter string, succeedOn int, failLine string) string {
	return `c="` + counter + `"
n=$(cat "$c" 2>/dev/null || echo 0)
n=$((n+1))
echo $n > "$c"
if [ "$n" -lt ` + strconv.Itoa(succeedOn) + ` ]; then echo "` + failLine + `" >&2; exit 1; fi`
}

const pluginFailLine = "Unable to connect to the server: getting credentials: exec: executable kubelogin failed with exit code 1"

func runCount(t *testing.T, counter string) string {
	t.Helper()
	got, err := os.ReadFile(counter)
	require.NoError(t, err)
	return strings.TrimSpace(string(got))
}

func TestExecAuthCmd_RunSingleAttempt(t *testing.T) {
	counter := filepath.Join(t.TempDir(), "count")
	fakeKubectl(t, countingScript(counter, 100, pluginFailLine))
	var out bytes.Buffer
	c := &execAuthCmd{context: "ctx-a", login: kubectlLogin()}
	c.SetStdin(strings.NewReader("\n"))
	c.SetStdout(&out)
	require.Error(t, c.Run())
	assert.Equal(t, "1", runCount(t, counter))
	assert.Contains(t, out.String(), pluginFailLine)
	assert.Contains(t, out.String(), "Press Enter to return to lfk")
}

func TestExecAuthCmd_RunPassesLoginEnvAndStdout(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "login")
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\necho \"code $1 $AWS_PROFILE\"\n"), 0o755))
	var out bytes.Buffer
	c := &execAuthCmd{context: "ctx-a", login: loginCommand{
		display: "login", argv: []string{bin, "ABCD"}, env: []string{"AWS_PROFILE=dev"},
	}}
	c.SetStdin(strings.NewReader(""))
	c.SetStdout(&out)
	require.NoError(t, c.Run())
	assert.Contains(t, out.String(), "code ABCD dev", "a login prints its device code on stdout")
}

// openLoginBox drives a plugin failure on the current context through the
// background resolve and feeds the result back, as the program would.
func openLoginBox(t *testing.T, m Model, ctx string) Model {
	t.Helper()
	cmd := m.maybeExecAuth(ctx, errPluginExit)
	require.NotNil(t, cmd)
	msg := cmd()
	require.IsType(t, execAuthPromptMsg{}, msg, "must ask first, not suspend")
	got, _ := m.Update(msg)
	return got.(Model)
}

func loginBoxText(m Model) string {
	return strings.Join(strings.Fields(confirmBox(m)), " ")
}

func TestExecAuth_OpensConfirmInsteadOfSuspending(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m = openLoginBox(t, m, "test-ctx")

	assert.Equal(t, overlayConfirm, m.overlay)
	assert.Equal(t, execAuthLoginAction, m.pendingAction)
	view := stripANSI(m.View().Content)
	assert.Contains(t, view, "Login required")
	assert.Contains(t, view, "test-ctx")
	assert.Contains(t, loginBoxText(m), "test-ctx needs login. Run: kubectl get --raw /version --context test-ctx")
	assert.NotContains(t, loginBoxText(m), "attempt")
}

func TestExecAuth_SkipsWhenOverlayOpen(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	cmd := m.maybeExecAuth("test-ctx", errPluginExit)
	require.NotNil(t, cmd)
	m.overlay = overlayConfirm
	m.pendingAction = "Delete"
	got, _ := m.Update(cmd())
	m = got.(Model)
	assert.Equal(t, "Delete", m.pendingAction)
	assert.False(t, m.execAuth.tried["test-ctx"], "the next load must ask again")
	assert.Empty(t, m.execAuth.context)
}

func TestExecAuth_EscCancels(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m = openLoginBox(t, m, "test-ctx")

	got, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = got.(Model)
	assert.Equal(t, overlayNone, m.overlay)
	assert.Empty(t, m.pendingAction)
	assert.Empty(t, m.execAuth.context)
	assert.True(t, m.execAuth.tried["test-ctx"])
	assert.Contains(t, m.statusMessage, "R")
}

func TestExecAuth_CtrlCCancelsLogin(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m = openLoginBox(t, m, "test-ctx")

	got, _ := m.Update(keyMsg("ctrl+c"))
	m = got.(Model)
	assert.Equal(t, overlayNone, m.overlay)
	assert.Empty(t, m.execAuth.context)
	assert.Empty(t, m.pendingAction)
	assert.Empty(t, m.confirmTitle)
	assert.Empty(t, m.confirmQuestion)

	got, _, _ = m.handleExplorerDirectActionKeys(tea.KeyPressMsg{Code: 'R', Text: "R"})
	m = got.(Model)
	m = openLoginBox(t, m, "test-ctx")
	assert.Equal(t, overlayConfirm, m.overlay, "R must bring the login back")
	assert.Equal(t, execAuthLoginAction, m.pendingAction)
}

func TestExecAuth_EnterRunsLogin(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m = openLoginBox(t, m, "test-ctx")

	got, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = got.(Model)
	require.NotNil(t, cmd)
	assert.Equal(t, overlayNone, m.overlay)
	assert.Empty(t, m.pendingAction)
	assert.Equal(t, "test-ctx", m.execAuth.context)
	assert.Equal(t, 1, m.execAuth.attempt)
}

func TestExecAuthDone_FailureReopensBoxUntilMax(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m = openLoginBox(t, m, "test-ctx")

	for _, want := range []string{"attempt 2/3", "attempt 3/3"} {
		got, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		got, _ = got.(Model).updateExecAuthDone(execAuthDoneMsg{context: "test-ctx", err: errors.New("exit status 1")})
		m = got.(Model)
		require.Equal(t, overlayConfirm, m.overlay, want)
		assert.Contains(t, loginBoxText(m), "Login failed. Retry? ("+want+")")
	}

	got, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got, _ = got.(Model).updateExecAuthDone(execAuthDoneMsg{context: "test-ctx", err: errors.New("exit status 1")})
	m = got.(Model)
	assert.Equal(t, overlayNone, m.overlay)
	assert.Contains(t, m.statusMessage, "Auth failed: ")
	assert.True(t, m.statusMessageErr)
	assert.Empty(t, m.execAuth.context)
}

func TestExecAuthDone_ConfirmAfterContextSwitch(t *testing.T) {
	m := basePush80Model()
	m.scheduler = scheduler.New(0)
	m.nav.Context = "a"
	m.nav.Level = model.LevelResourceTypes
	m = openLoginBox(t, m, "a")
	m.nav.Context = "b"

	got, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	gen := got.(Model).requestGen
	got, _ = got.(Model).updateExecAuthDone(execAuthDoneMsg{context: "a"})
	m = got.(Model)
	assert.Contains(t, m.statusMessage, "Authenticated to a")
	assert.Equal(t, gen, m.requestGen)
	assert.Empty(t, m.scheduler.QueueSnapshot(), "no refresh for b")
	assert.Empty(t, m.execAuth.context)
}

func TestExecAuth_NilGuardMapSetOnFailedLoad(t *testing.T) {
	m := basePush80Model()
	m.execAuth.tried = nil
	m.requestGen = 1
	mdl, _ := m.updateResourcesLoaded(resourcesLoadedMsg{gen: 1, context: "test-ctx", err: errPluginExit})
	assert.True(t, mdl.(Model).execAuth.tried["test-ctx"])

	m.execAuth.tried = nil
	m.nav.Level = model.LevelResourceTypes
	mdl, _ = m.updateNamespacesLoaded(namespacesLoadedMsg{context: "test-ctx", err: errPluginExit})
	assert.True(t, mdl.(Model).execAuth.tried["test-ctx"])
}

func TestExecAuth_LoadSuccessKeepsGuard(t *testing.T) {
	m := basePush80Model()
	m.requestGen = 1
	m.execAuth.tried = map[string]bool{"test-ctx": true}
	mdl, _ := m.updateResourcesLoaded(resourcesLoadedMsg{gen: 1, context: "test-ctx"})
	assert.True(t, mdl.(Model).execAuth.tried["test-ctx"])
}

func TestExecAuth_NavigateChildClusterClearsGuard(t *testing.T) {
	m := basePush80Model()
	m.execAuth.tried = map[string]bool{"cluster-a": true}
	got, _ := m.navigateChildCluster(&model.Item{Name: "cluster-a"})
	assert.False(t, got.(Model).execAuth.tried["cluster-a"])
}

func TestExecAuthCmd_NoKubeconfigKeepsInheritedEnv(t *testing.T) {
	t.Setenv("KUBECONFIG", "/inherited/kubeconfig")
	out := filepath.Join(t.TempDir(), "env")
	fakeKubectl(t, `printf '%s' "$KUBECONFIG" > `+out)
	c := &execAuthCmd{context: "ctx-a", login: kubectlLogin()}
	c.SetStdin(strings.NewReader(""))
	c.SetStdout(io.Discard)
	require.NoError(t, c.Run())
	got, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Equal(t, "/inherited/kubeconfig", string(got))
}
