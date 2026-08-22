package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// explainExecModel is a model with the API Explorer open on pods, ready to run
// a real subprocess against the stub kubectl that fakeKubectl puts on PATH.
func explainExecModel(t *testing.T) Model {
	t.Helper()
	m := basePush80Model()
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true,
	}
	m.reqCtx = t.Context()
	m.beginExplainSession()
	m.explainResource = "pods"
	return m
}

// Leaving the API Explorer must end the explain processes still running. The
// stub sleeps far longer than the deadline the fetch would otherwise wait out.
//
// The sleep runs as a CHILD of the stub shell on purpose. Killing the shell
// leaves that child holding the output pipe, which is what a credential plugin
// does in the real failure, and CombinedOutput reads to EOF. Only cmd.WaitDelay
// gets the worker back. Without it this hangs for the full sleep.
func TestExplainFetchesStopWhenTheViewCloses(t *testing.T) {
	cases := []struct {
		name string
		run  func(m Model) tea.Cmd
	}{
		{"explain", func(m Model) tea.Cmd { return m.execKubectlExplain("pods", "v1", "spec") }},
		{"recursive", func(m Model) tea.Cmd { return m.execKubectlExplainRecursive("pods", "v1", "") }},
		{"tree", func(m Model) tea.Cmd { return m.execKubectlExplainTree("pods", "v1", "spec") }},
		{"treeDesc", func(m Model) tea.Cmd { return m.execKubectlExplainTreeDesc("pods", "v1", "spec") }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			argvPath := fakeKubectl(t, "sleep 60 &\nwait")
			m := explainExecModel(t)
			cmd := tc.run(m)
			require.NotNil(t, cmd)

			done := make(chan tea.Msg, 1)
			go func() { done <- cmd() }()

			// The stub writes its argv first, so the file appearing means the
			// process is really running. Closing the view before that would
			// only prove that a cancelled context never spawns one.
			//
			// Watch done alongside the file. A spawn that fails outright (a
			// PATH miss, ETXTBSY on the freshly written script) sends its
			// error there immediately, and polling on alone would spend the
			// whole ceiling before reporting "never started" without saying
			// why. That message was the whole content of two flakes in a
			// loaded ./... run that no one could reproduce afterwards.
			waitForStub(t, argvPath, done)

			closedAt := time.Now()
			m.exitExplainView()

			select {
			case <-done:
				// Measured from the close, and well under explainFetchTimeout,
				// so a fetch that ignored cancellation and merely ran out its
				// deadline cannot pass this. The allowance covers
				// cmd.WaitDelay, which is what unblocks the read while the
				// backgrounded sleep still holds the pipe.
				assert.Less(t, time.Since(closedAt), explainFetchTimeout/2,
					"closing the API Explorer must end the fetch, not leave it to the deadline")
			case <-time.After(explainFetchTimeout / 2):
				t.Fatal("closing the API Explorer did not stop the kubectl process")
			}
		})
	}
}

// An auth failure prints whatever the credential plugin had to say. That text
// must not reach the log; the target and the exit status are what identify the
// failure.
func TestExplainFailureLogsNoCommandOutput(t *testing.T) {
	cases := []struct {
		name   string
		run    func(m Model) tea.Cmd
		target string
	}{
		{"explain", func(m Model) tea.Cmd { return m.execKubectlExplain("pods", "v1", "spec") }, "pods.spec"},
		{"recursive", func(m Model) tea.Cmd { return m.execKubectlExplainRecursive("pods", "v1", "") }, "pods"},
		{"tree", func(m Model) tea.Cmd { return m.execKubectlExplainTree("pods", "v1", "spec") }, "pods.spec"},
		{"treeDesc", func(m Model) tea.Cmd { return m.execKubectlExplainTreeDesc("pods", "v1", "spec") }, "pods.spec"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeKubectl(t, `echo "token=s3cret-from-the-credential-plugin" >&2
exit 1`)
			buf := captureLogger(t)
			m := explainExecModel(t)

			tc.run(m)()

			assert.NotContains(t, buf.String(), "s3cret-from-the-credential-plugin")
			assert.Contains(t, buf.String(), "kubectl explain failed")
			assert.Contains(t, buf.String(), tc.target, "the log still names the target that failed")
			assert.Contains(t, buf.String(), "exit status 1")
		})
	}
}

// A tab restored with the API Explorer open gets a session of its own, so
// closing the view still stops the fetch it started after the switch back.
func TestExplainFetchesStopAfterSwitchingBack(t *testing.T) {
	argvPath := fakeKubectl(t, "sleep 60 &\nwait")
	m := explainExecModel(t)
	m.mode = modeExplain
	m.tabs = []TabState{{}, {}}
	m.saveCurrentTab()
	m.loadTab(1)
	m.loadTab(0)
	require.Equal(t, modeExplain, m.mode, "the restored tab is still on the API Explorer")

	done := make(chan tea.Msg, 1)
	go func() { done <- m.execKubectlExplainTree("pods", "v1", "spec")() }()

	require.Eventually(t, func() bool {
		_, err := os.Stat(argvPath)
		return err == nil
	}, 10*time.Second, 20*time.Millisecond, "the stub kubectl never started")

	closedAt := time.Now()
	m.exitExplainView()

	select {
	case <-done:
		assert.Less(t, time.Since(closedAt), explainFetchTimeout/2,
			"a restored tab must be able to cancel its own fetches")
	case <-time.After(explainFetchTimeout / 2):
		t.Fatal("closing the API Explorer on a restored tab did not stop the kubectl process")
	}
}

// Switching tabs is the other way out of the API Explorer, and the reply of a
// fetch started on the old tab has no tab of its own to go back to.
func TestExplainFetchesStopOnTabSwitch(t *testing.T) {
	argvPath := fakeKubectl(t, "sleep 60 &\nwait")
	m := explainExecModel(t)
	m.tabs = []TabState{{}, {}}
	cmd := m.execKubectlExplain("pods", "v1", "spec")
	require.NotNil(t, cmd)

	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()

	require.Eventually(t, func() bool {
		_, err := os.Stat(argvPath)
		return err == nil
	}, 10*time.Second, 20*time.Millisecond, "the stub kubectl never started")

	switchedAt := time.Now()
	m.loadTab(1)

	select {
	case <-done:
		assert.Less(t, time.Since(switchedAt), explainFetchTimeout/2,
			"switching tabs must end the fetch, not leave it to the deadline")
	case <-time.After(explainFetchTimeout / 2):
		t.Fatal("switching tabs did not stop the kubectl process")
	}
}

// A session that was never opened still runs under m.reqCtx, so no explain
// call is ever left unbounded.
func TestExplainRequestCtxFallsBackToTheRequestContext(t *testing.T) {
	m := basePush80Model()
	m.reqCtx = t.Context()
	m.cancelExplainSession()

	assert.Equal(t, m.reqCtx, m.explainRequestCtx())
}

// waitForStub blocks until the stub kubectl has written its argv file, and
// fails with whatever the fetch returned if it gave up first.
func waitForStub(t *testing.T, argvPath string, done <-chan tea.Msg) {
	t.Helper()
	// The ceiling only bounds how long the stub may take to get scheduled. It
	// is not the thing under test: the assertion that closing the view ends the
	// fetch measures from the close and is unaffected. The one captured failure
	// had the fetch still running with no spawn error, which suggests delayed
	// scheduling without proving it, so the ceiling is generous. It costs
	// nothing on the passing path.
	deadline := time.After(30 * time.Second)
	for {
		if _, err := os.Stat(argvPath); err == nil {
			return
		}
		select {
		case msg := <-done:
			t.Fatalf("the fetch returned before the stub started: %#v", msg)
		case <-deadline:
			// KubectlPath returns KUBECTL_BIN unchanged and without an error
			// when it is set, so report the override and whether the path
			// exists rather than presenting the result as resolved.
			path, pathErr := k8s.KubectlPath()
			_, pathStat := os.Stat(path)
			_, stubStat := os.Stat(filepath.Join(filepath.Dir(argvPath), "kubectl"))
			t.Fatalf("the stub kubectl never started, and the fetch is still running "+
				"(kubectlPath=%q err=%v exists=%v KUBECTL_BIN=%q stubPresent=%v PATH=%s)",
				path, pathErr, pathStat == nil, os.Getenv("KUBECTL_BIN"),
				stubStat == nil, os.Getenv("PATH"))
		case <-time.After(20 * time.Millisecond):
		}
	}
}
