//go:build e2e

package e2e

import (
	"testing"
	"time"
)

// The pty buffer is never cleared, so a marker rendered during startup would
// satisfy every later wait for free. That is not hypothetical: the cluster
// dashboard lists an UnexpectedJob warning naming the CronJob, which is the
// same string the navigation waits on.
func TestWaitForAfter_IgnoresOutputBeforeOffset(t *testing.T) {
	out := &output{}
	if _, err := out.Write([]byte("UnexpectedJob: CronJob/nightly-backup")); err != nil {
		t.Fatal(err)
	}
	offset := len(out.String())

	done := make(chan struct{})
	go func() {
		defer close(done)
		waitForAfter(t, out, offset, "nightly-backup", 2*time.Second)
	}()

	select {
	case <-done:
		t.Fatal("returned on a marker that only appears before the offset")
	case <-time.After(300 * time.Millisecond):
	}

	if _, err := out.Write([]byte("\nrow: nightly-backup")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("did not return once the marker appeared after the offset")
	}
}
