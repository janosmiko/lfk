package k8s

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// portForwardSettleTimeout is how long the test waits for the fake binary to
// spawn, write to stderr, and exit. It is generous on purpose: a full
// `go test -race ./...` run competes for cores, and the first exec of a file
// just written to a temp directory can take seconds on a loaded machine.
const portForwardSettleTimeout = 30 * time.Second

// The port-forward monitor reads the captured stderr after cmd.Wait(). The
// stderr capture must be synchronized with that read — a bare bytes.Buffer
// written by a detached io.Copy goroutine races the post-Wait read. Run with
// -race: this fails before the fix and passes after. It also asserts the
// captured stderr still reaches entry.Error.
func TestPortForwardStderrCaptureNoRace(t *testing.T) {
	// Fake kubectl: ignore args, emit to stderr, fail. Drives the failure
	// branch that reads the captured stderr.
	dir := t.TempDir()
	fake := filepath.Join(dir, "fake-kubectl")
	script := "#!/bin/sh\necho 'unable to listen on port' >&2\nexit 1\n"
	require.NoError(t, os.WriteFile(fake, []byte(script), 0o700))

	mgr := NewPortForwardManager()
	// The callback runs on the monitor goroutine. Counting it keeps that
	// concurrent write under the race detector without letting a slow reader
	// block the monitor, which a channel would.
	var updates atomic.Int64
	mgr.SetUpdateCallback(func() { updates.Add(1) })

	_, err := mgr.Start(fake, "/dev/null", "Pod", "p", "ns", "ctx", "ctx", "8080", "80")
	require.NoError(t, err, "Start should launch the fake binary")

	// Poll the status itself. Waiting on the callback instead would tie the
	// test to how many times the monitor happens to report. EventuallyWithT
	// reports the last status it saw, so a timeout names the state it got
	// stuck in.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		entries := mgr.Entries()
		if assert.Len(c, entries, 1) {
			assert.Equal(c, PortForwardFailed, entries[0].Status)
		}
	}, portForwardSettleTimeout, 10*time.Millisecond)

	// Failed is terminal, so this second read sees the same entry.
	entries := mgr.Entries()
	require.Len(t, entries, 1)
	assert.Contains(t, entries[0].Error, "unable to listen on port",
		"captured stderr must surface in entry.Error")
	// Start reports once before it returns, the monitor once more after
	// cmd.Wait. Reading the count here would be a race the other way: the
	// monitor calls back after it releases the lock, so the status can be
	// visible before the report is.
	require.Eventually(t, func() bool { return updates.Load() >= 2 },
		portForwardSettleTimeout, 10*time.Millisecond,
		"the manager must report the move to Failed, not only the start")
}
