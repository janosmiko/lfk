//go:build e2e

// Package e2e drives the built lfk binary under --demo inside a real pty
// and asserts on what it renders. Unit tests exercise app.Model and the
// demo fake cluster in isolation; this is the only test that launches the
// process the way a user actually would, so it catches startup-path
// panics unit tests structurally cannot reach -- a --demo startup crash
// once shipped past a fully green unit suite.
//
// teatest (github.com/charmbracelet/x/exp/teatest) was considered first,
// but it pins github.com/charmbracelet/bubbletea v1, a different module
// than this repo's charm.land/bubbletea/v2 -- its tea.Program type isn't
// assignable to ours, so it can't drive this app. A pty-driven subprocess
// works against any bubbletea version because it only depends on the
// binary's terminal output.
package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
)

const (
	startupTimeout  = 15 * time.Second
	shutdownTimeout = 5 * time.Second
)

// buildBinary compiles lfk from the repo root into a temp dir so
// `go test -tags e2e ./e2e/...` is self-sufficient and never runs against
// a stale ./lfk left over from a previous `make build`.
func buildBinary(t *testing.T) string {
	t.Helper()

	repoRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolving repo root: %v", err)
	}

	binPath := filepath.Join(t.TempDir(), "lfk-e2e")
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	return binPath
}

// output is a goroutine-safe buffer that accumulates everything read from
// the pty, polled by waitFor.
type output struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (o *output) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.buf.Write(p)
}

func (o *output) String() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.buf.String()
}

// waitFor polls out for substr until it appears or timeout elapses.
func waitFor(t *testing.T, out *output, substr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(out.String(), substr) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out after %s waiting for %q; captured output:\n%s", timeout, substr, out.String())
}

// TestDemoModeStartup launches `lfk --demo` under a pty with HOME and
// KUBECONFIG pointed at an empty temp dir, then checks it renders the DEMO
// badge and a seeded workload without panicking.
func TestDemoModeStartup(t *testing.T) {
	binPath := buildBinary(t)

	home := t.TempDir()
	kubeconfig := filepath.Join(home, "kubeconfig")

	cmd := exec.Command(binPath, "--demo")
	// An explicit, minimal env -- not os.Environ() plus overrides -- so a
	// developer's own XDG_CONFIG_HOME, LFK_CONFIG_DIR, or KUBECONFIG_DIR
	// can never leak a real config or kubeconfig into the demo run.
	cmd.Env = []string{
		"HOME=" + home,
		"KUBECONFIG=" + kubeconfig,
		"PATH=" + os.Getenv("PATH"),
		"TERM=xterm-256color",
	}

	stderr := &output{}
	cmd.Stderr = stderr // stdout must stay on the pty, or bubbletea's terminal handshake hangs

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 40, Cols: 200})
	if err != nil {
		t.Fatalf("starting lfk under pty: %v", err)
	}
	defer func() { _ = ptmx.Close() }()

	out := &output{}
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				_, _ = out.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	// Always kill the process, however the test ends -- it's a TUI that
	// otherwise sits waiting for input forever.
	defer func() {
		_ = cmd.Process.Kill()

		waitErr := make(chan error, 1)
		go func() { waitErr <- cmd.Wait() }()
		select {
		case <-waitErr:
		case <-time.After(shutdownTimeout):
			t.Log("lfk process did not exit after kill within shutdownTimeout")
		}

		select {
		case <-readDone:
		case <-time.After(shutdownTimeout):
			t.Log("pty reader goroutine did not finish within shutdownTimeout")
		}

		if s := stderr.String(); s != "" {
			t.Logf("lfk stderr:\n%s", s)
		}
	}()

	waitFor(t, out, "DEMO", startupTimeout)
	if strings.Contains(out.String(), "panic:") {
		t.Fatalf("lfk panicked during startup:\n%s", out.String())
	}

	// Drill into the (only) context, then jump to the Deployments resource
	// type via search rather than counting arrow-key presses: the
	// resource-type list grows as source discovery completes, so a fixed
	// key count is order-dependent and flaky.
	sendKeys(t, ptmx, "\r")
	time.Sleep(300 * time.Millisecond)
	sendKeys(t, ptmx, "/Deployments\r")
	time.Sleep(300 * time.Millisecond)
	sendKeys(t, ptmx, "\r")

	waitFor(t, out, "web", startupTimeout)

	if strings.Contains(out.String(), "panic:") || strings.Contains(stderr.String(), "panic:") {
		t.Fatalf("lfk panicked; pty output:\n%s\nstderr:\n%s", out.String(), stderr.String())
	}
}

func sendKeys(t *testing.T, f *os.File, s string) {
	t.Helper()
	if _, err := f.Write([]byte(s)); err != nil {
		t.Fatalf("writing to pty: %v", err)
	}
}
