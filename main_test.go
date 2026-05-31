package main

import (
	"testing"
	"time"
)

// armForceQuit must, after the grace period, kill the program first (to
// restore the terminal) and only then exit — and it must not act before
// the grace elapses.
func TestArmForceQuit_KillsThenExitsAfterGrace(t *testing.T) {
	order := make(chan string, 2)
	notify := armForceQuit(100*time.Millisecond,
		func() { order <- "kill" },
		func() { order <- "exit" },
	)

	notify()

	// Nothing should fire before the grace period. The pre-check window is
	// kept a clear fraction below the grace so a descheduled goroutine on a
	// busy CI runner does not flake this assertion.
	select {
	case got := <-order:
		t.Fatalf("watchdog fired %q before grace elapsed", got)
	case <-time.After(20 * time.Millisecond):
	}

	got := make([]string, 0, 2)
	for range 2 {
		select {
		case s := <-order:
			got = append(got, s)
		case <-time.After(time.Second):
			t.Fatalf("watchdog did not fire within timeout, got %v", got)
		}
	}

	if got[0] != "kill" || got[1] != "exit" {
		t.Fatalf("expected kill then exit, got %v", got)
	}
}
