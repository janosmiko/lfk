package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/janosmiko/lfk/internal/app"
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

// validatePprofAddr must refuse every bind-all or non-loopback form of
// LFK_PPROF_ADDR: the pprof endpoint exposes process internals (heap,
// goroutines, env strings) and must never be reachable off-box.
func TestValidatePprofAddr(t *testing.T) {
	valid := []string{
		"localhost:6060",
		"127.0.0.1:6060",
		"[::1]:6060",
	}
	for _, addr := range valid {
		if err := validatePprofAddr(addr); err != nil {
			t.Errorf("validatePprofAddr(%q) = %v, want nil", addr, err)
		}
	}

	invalid := []struct {
		addr, wantContains string
	}{
		{":6060", "loopback"},
		{"0.0.0.0:6060", "loopback"},
		{"[::]:6060", "loopback"},
		{"10.0.0.1:6060", "loopback"},
		{"example.com:6060", "not an IP"},
		{"127.0.0.1", "invalid host:port"},
		{"", "invalid host:port"},
	}
	for _, tt := range invalid {
		err := validatePprofAddr(tt.addr)
		if err == nil {
			t.Errorf("validatePprofAddr(%q) = nil, want error", tt.addr)
			continue
		}
		if !strings.Contains(err.Error(), tt.wantContains) {
			t.Errorf("validatePprofAddr(%q) error = %q, want it to contain %q", tt.addr, err, tt.wantContains)
		}
	}
}

// --demo must work with no kubeconfig on disk at all (acceptance criterion
// 1), and must skip --context validation entirely — a bogus --context
// would otherwise fail against the real kubeconfig path.
func TestResolveStartupClient_DemoSkipsKubeconfigResolution(t *testing.T) {
	emptyDir := t.TempDir()
	t.Setenv("HOME", emptyDir)
	t.Setenv("KUBECONFIG", filepath.Join(emptyDir, "does-not-exist"))
	t.Setenv("KUBECONFIG_DIR", "")

	client, err := resolveStartupClient(app.StartupOptions{Demo: true, Context: "no-such-context"})
	if err != nil {
		t.Fatalf("resolveStartupClient() error = %v, want nil", err)
	}
	if client == nil {
		t.Fatal("resolveStartupClient() returned nil client")
	}
	if !client.IsDemo() {
		t.Error("resolveStartupClient() client.IsDemo() = false, want true")
	}
}
