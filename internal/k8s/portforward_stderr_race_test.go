package k8s

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	done := make(chan struct{}, 16)
	mgr.SetUpdateCallback(func() { done <- struct{}{} })

	_, err := mgr.Start(fake, "/dev/null", "Pod", "p", "ns", "ctx", "ctx", "8080", "80")
	require.NoError(t, err, "Start should launch the fake binary")

	// Wait for the monitor goroutine to reach a terminal status.
	deadline := time.After(5 * time.Second)
	for {
		entries := mgr.Entries()
		if len(entries) == 1 && entries[0].Status == PortForwardFailed {
			assert.Contains(t, entries[0].Error, "unable to listen on port",
				"captured stderr must surface in entry.Error")
			return
		}
		select {
		case <-done:
		case <-deadline:
			t.Fatalf("port-forward did not reach Failed status; entries=%+v", mgr.Entries())
		}
	}
}
