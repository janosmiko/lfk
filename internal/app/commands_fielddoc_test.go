package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// fakeKubectl puts a stub kubectl first on PATH and returns the file the stub
// writes its argv to. The stub is the only way to exercise the subprocess path
// without a cluster: it lets the test assert on the arguments the app builds
// and choose what kubectl "prints" back.
func fakeKubectl(t *testing.T, script string) (argvPath string) {
	t.Helper()

	dir := t.TempDir()
	argvPath = filepath.Join(dir, "argv")
	body := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argvPath + "\n" + script + "\n"

	require.NoError(t, os.WriteFile(filepath.Join(dir, "kubectl"), []byte(body), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argvPath
}

func fieldDocExecModel(t *testing.T) Model {
	t.Helper()
	m := basePush80Model()
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true,
	}
	m.fieldDoc.cache = newFieldDocCache()
	m.reqCtx = t.Context()
	return m
}

func TestExecKubectlExplainFieldBuildsTheArguments(t *testing.T) {
	argvPath := fakeKubectl(t, `cat <<'OUT'
KIND:       Deployment
VERSION:    apps/v1

FIELD: replicas <integer>

DESCRIPTION:
    Number of desired pods.
OUT`)
	m := fieldDocExecModel(t)
	key := fieldDocKey{
		context: "test-ctx", apiVersion: "apps/v1", resource: "deployments", path: "spec.replicas",
	}

	msg := m.execKubectlExplainField(t.Context(), 1, key)()

	got, ok := msg.(fieldDocLoadedMsg)
	require.True(t, ok)
	require.NoError(t, got.err)

	argv, err := os.ReadFile(argvPath)
	require.NoError(t, err)
	args := strings.Fields(string(argv))

	assert.Equal(t, "explain", args[0])
	assert.Contains(t, args, "deployments.spec.replicas", "the target joins the resource and the field path")
	assert.Contains(t, args, "--api-version")
	assert.Contains(t, args, "apps/v1")
	assert.Contains(t, args, "--context")
}

// A core resource has no group/version, so the flag must be left off entirely
// rather than passed empty.
func TestExecKubectlExplainFieldOmitsEmptyAPIVersion(t *testing.T) {
	argvPath := fakeKubectl(t, `echo "FIELD: dnsPolicy <string>"`)
	m := fieldDocExecModel(t)

	m.execKubectlExplainField(t.Context(), 1,
		fieldDocKey{context: "test-ctx", resource: "pods", path: "spec.dnsPolicy"})()

	argv, err := os.ReadFile(argvPath)
	require.NoError(t, err)
	assert.NotContains(t, string(argv), "--api-version")
}

func TestExecKubectlExplainFieldParsesDescriptionAndType(t *testing.T) {
	fakeKubectl(t, `cat <<'OUT'
KIND:       Pod
VERSION:    v1

FIELD: dnsPolicy <string>

DESCRIPTION:
    Set DNS policy for the pod. Defaults to "ClusterFirst".
OUT`)
	m := fieldDocExecModel(t)

	msg := m.execKubectlExplainField(t.Context(), 4,
		fieldDocKey{context: "test-ctx", resource: "pods", path: "spec.dnsPolicy"})()

	got := msg.(fieldDocLoadedMsg)
	require.NoError(t, got.err)
	assert.Equal(t, uint64(4), got.req, "the reply carries the request it answers")
	assert.Equal(t, "<string>", got.entry.fieldType)
	assert.Contains(t, got.entry.desc, "Set DNS policy for the pod.")
}

// The reported bug, end to end through the subprocess: kubectl exits non-zero
// after printing the preamble, and only the error line may reach the pane.
func TestExecKubectlExplainFieldTrimsTheErrorOutput(t *testing.T) {
	fakeKubectl(t, `cat >&2 <<'OUT'
KIND:       Pod
VERSION:    v1

error: field "checksum/config" does not exist
OUT
exit 1`)
	m := fieldDocExecModel(t)

	msg := m.execKubectlExplainField(t.Context(), 1, fieldDocKey{
		context: "test-ctx", resource: "pods", path: "metadata.annotations.checksum/config",
	})()

	got := msg.(fieldDocLoadedMsg)
	require.Error(t, got.err)
	assert.Equal(t, `field "checksum/config" does not exist`, got.err.Error())
	assert.NotContains(t, got.err.Error(), "exit status")
	assert.NotContains(t, got.err.Error(), "VERSION:")
}

func TestExecKubectlExplainFieldReportsMissingKubectl(t *testing.T) {
	m := fieldDocExecModel(t)
	t.Setenv("PATH", "/nonexistent")

	msg := m.execKubectlExplainField(t.Context(), 1,
		fieldDocKey{context: "test-ctx", resource: "pods", path: "spec"})()

	got := msg.(fieldDocLoadedMsg)
	require.Error(t, got.err)
	assert.Contains(t, got.err.Error(), "kubectl not found")
}

// A cancelled request must end the process, not wait out its deadline. The stub
// sleeps far longer than the test would tolerate if cancellation did not work.
//
// The sleep runs as a CHILD of the stub shell on purpose. Killing the shell
// leaves that child holding the output pipe, which is what a credential plugin
// does in the real failure, and CombinedOutput reads to EOF. Only cmd.WaitDelay
// gets the worker back. Without it this hangs for the full sleep.
func TestExecKubectlExplainFieldStopsOnCancel(t *testing.T) {
	argvPath := fakeKubectl(t, "sleep 60 &\nwait")
	m := fieldDocExecModel(t)
	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan tea.Msg, 1)
	go func() {
		done <- m.execKubectlExplainField(ctx, 1,
			fieldDocKey{context: "test-ctx", resource: "pods", path: "spec"})()
	}()

	// The stub writes its argv first, so the file appearing means the process
	// is really running. Cancelling before that would only prove that an
	// already-cancelled context never spawns one.
	require.Eventually(t, func() bool {
		_, err := os.Stat(argvPath)
		return err == nil
	}, 10*time.Second, 20*time.Millisecond, "the stub kubectl never started")

	cancelledAt := time.Now()
	cancel()

	select {
	case res := <-done:
		got, ok := res.(fieldDocLoadedMsg)
		require.True(t, ok)
		assert.Error(t, got.err, "a killed process reports an error, which the pane then ignores")
		// Well under fieldDocFetchTimeout, so a fetch that ignored cancellation
		// and merely ran out its 15 second deadline cannot pass this. The
		// allowance covers cmd.WaitDelay, which is what unblocks the read when
		// the backgrounded sleep keeps holding the pipe.
		assert.Less(t, time.Since(cancelledAt), fieldDocFetchTimeout/2,
			"cancelling must end the fetch, not leave it to the deadline")
	case <-time.After(fieldDocFetchTimeout / 2):
		t.Fatal("cancelling the request did not stop the kubectl process")
	}
}

// isContextCanceled is what keeps a cancelled fetch from painting an error into
// the pane, so the two halves have to agree on what cancellation looks like.
func TestFieldDocCancelledFetchLeavesThePaneClean(t *testing.T) {
	m := fieldDocExecModel(t)
	m.fieldDoc.on = true
	m.fieldDoc.loading = true
	m.fieldDoc.req = 1

	got := m.updateFieldDocLoaded(fieldDocLoadedMsg{
		req: 1, key: fieldDocKey{path: "spec"}, err: context.Canceled,
	})

	assert.Empty(t, got.fieldDoc.err)
	assert.False(t, got.fieldDoc.loading)
}
