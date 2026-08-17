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
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/janosmiko/lfk/internal/k8s/demo"
)

const (
	// A pty wait is wall-clock, so a loaded machine eats the budget rather than
	// the app being slow. 15s failed on a laptop running three test suites at
	// once, and a shared CI runner is no less contended.
	startupTimeout  = 45 * time.Second
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

// waitForThenSend waits for substr before sending keys - a fixed sleep
// between keystrokes is a timing guess that only holds on a fast, unloaded
// machine (TASK-896: it failed against a real apiserver's slower startup).
//
// It searches only output produced after the previous step, because the buffer
// is never cleared and a marker that appeared once would satisfy every later
// wait for free. A real case: the startup dashboard lists an UnexpectedJob
// warning naming the CronJob, so a whole-buffer wait for that name passes
// before any key is sent.
func waitForThenSend(t *testing.T, out *output, substr string, f *os.File, keys string, timeout time.Duration) int {
	t.Helper()
	return waitForNewThenSend(t, out, 0, substr, f, keys, timeout)
}

// waitForNewThenSend is waitForThenSend scoped to output after offset. It
// returns the buffer length at the moment the keys were sent, to pass as the
// next step's offset.
func waitForNewThenSend(t *testing.T, out *output, offset int, substr string, f *os.File, keys string, timeout time.Duration) int {
	t.Helper()
	waitForAfter(t, out, offset, substr, timeout)
	sent := len(out.String())
	sendKeys(t, f, keys)
	return sent
}

// waitForAfter polls for substr in the output written after offset.
func waitForAfter(t *testing.T, out *output, offset int, substr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if s := out.String(); len(s) > offset && strings.Contains(s[offset:], substr) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out after %s waiting for %q in output past byte %d; captured output:\n%s",
		timeout, substr, offset, out.String())
}

// runUnderPty starts cmd inside a pty and registers cleanup that kills it -
// lfk is a TUI that otherwise sits waiting for input forever.
func runUnderPty(t *testing.T, cmd *exec.Cmd) (ptmx *os.File, out, stderr *output) {
	t.Helper()
	stderr = &output{}
	cmd.Stderr = stderr // stdout must stay on the pty, or bubbletea's terminal handshake hangs

	var err error
	ptmx, err = pty.StartWithSize(cmd, &pty.Winsize{Rows: 40, Cols: 200})
	if err != nil {
		t.Fatalf("starting lfk under pty: %v", err)
	}

	out = &output{}
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

	t.Cleanup(func() {
		_ = ptmx.Close()
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
	})

	return ptmx, out, stderr
}

// startDemoUnderPty builds lfk and launches it under --demo. Startup is
// already confirmed (DEMO badge rendered, no panic) by the time it returns.
func startDemoUnderPty(t *testing.T) (ptmx *os.File, out, stderr *output) {
	t.Helper()
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

	ptmx, out, stderr = runUnderPty(t, cmd)

	waitFor(t, out, "DEMO", startupTimeout)
	if strings.Contains(out.String(), "panic:") {
		t.Fatalf("lfk panicked during startup:\n%s", out.String())
	}

	return ptmx, out, stderr
}

// TestDemoModeStartup checks lfk --demo renders a seeded workload without
// panicking.
func TestDemoModeStartup(t *testing.T) {
	ptmx, out, stderr := startDemoUnderPty(t)

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

// TestDemoModeCronJobLogs opens the seeded CronJob's logs (TASK-896) - a
// wiring check only, since internal/democli hand-crafts its kubectl
// responses. Real label selector semantics need the e2e-cluster CI job.
func TestDemoModeCronJobLogs(t *testing.T) {
	ptmx, out, stderr := startDemoUnderPty(t)

	sendKeys(t, ptmx, "\r")
	at := waitForThenSend(t, out, "CronJobs", ptmx, "/CronJobs\r", startupTimeout)
	at = waitForNewThenSend(t, out, at, demo.CronJobNightlyBackup, ptmx, "\r", startupTimeout)
	// ctrl+l opens the fullscreen log viewer.
	at = waitForNewThenSend(t, out, at, demo.CronJobNightlyBackup, ptmx, "\x0c", startupTimeout)

	waitForAfter(t, out, at, demo.JobNightlyBackupRun, startupTimeout)

	if strings.Contains(out.String(), "panic:") || strings.Contains(stderr.String(), "panic:") {
		t.Fatalf("lfk panicked; pty output:\n%s\nstderr:\n%s", out.String(), stderr.String())
	}
	if strings.Contains(out.String(), "No runs yet") {
		t.Fatalf("CronJob log resolution reported no runs; pty output:\n%s", out.String())
	}
}

// waitForAfter2 is waitForAfter returning the new offset.
func waitForAfter2(t *testing.T, out *output, offset int, substr string, timeout time.Duration) int {
	t.Helper()
	waitForAfter(t, out, offset, substr, timeout)
	return len(out.String())
}

func sendKeys(t *testing.T, f *os.File, s string) {
	t.Helper()
	if _, err := f.Write([]byte(s)); err != nil {
		t.Fatalf("writing to pty: %v", err)
	}
}

// TestDemoModeChangedColumn drives the Changed column the way the user did when
// they found it empty. A unit test cannot catch that class of bug: it feeds
// hand-built items, so it never notices that real list rows carry no source
// for the value. This asserts the column is there without being asked for,
// and that it fills for more than one row.
func TestDemoModeChangedColumn(t *testing.T) {
	ptmx, out, stderr := startDemoUnderPty(t)

	sendKeys(t, ptmx, "\r")
	at := waitForThenSend(t, out, "Pods", ptmx, "/Pods\r", startupTimeout)
	at = waitForNewThenSend(t, out, at, demo.PodWebCrashLoop, ptmx, "\r", startupTimeout)
	// The column is a regular column now, so it renders before any sort.
	at = waitForAfter2(t, out, at, "CHANGED", startupTimeout)

	// The command bar reaches the sort key without counting > presses. The
	// first Enter accepts the autocomplete entry, the second runs the command.
	sendKeys(t, ptmx, ":sort Changed")
	time.Sleep(300 * time.Millisecond)
	sendKeys(t, ptmx, "\r")
	time.Sleep(300 * time.Millisecond)
	sendKeys(t, ptmx, "\r")

	waitForAfter(t, out, at, "Sort by Changed", startupTimeout)

	if strings.Contains(out.String(), "panic:") || strings.Contains(stderr.String(), "panic:") {
		t.Fatalf("lfk panicked; pty output:\n%s\nstderr:\n%s", out.String(), stderr.String())
	}

	// The bug: only the pod lfk watched restart had a value. Every seeded pod
	// carries condition transitions, so the rendered table must show an age
	// for several distinct pods.
	filled := podsWithFilledChangedCell(out.String())
	if len(filled) < 3 {
		t.Fatalf("Changed column filled for %d distinct pods %v, want at least 3; pty output:\n%s",
			len(filled), filled, out.String())
	}
}

// changedCellPattern matches the age shapes formatAge emits (45s, 5m, 2h, 3d, 1y).
var changedCellPattern = regexp.MustCompile(`\b\d+[smhdy]\b`)

// demoWebPodPattern matches the seeded web pod names.
var demoWebPodPattern = regexp.MustCompile(`web-7d8f9c6b5-[a-z0-9]+`)

// podsWithFilledChangedCell returns the sorted, deduplicated names of seeded
// pods whose row carries at least two age-shaped values. Every row already
// renders AGE, so a second one on the same line is the CHANGED cell.
//
// Deduplicating by name is the point. The pty stream holds every redraw frame,
// so counting matching lines would let one populated pod satisfy a
// three-row threshold on its own -- passing even when the column is filled for
// exactly the one pod whose restart lfk happened to witness, which is the
// regression this test exists to catch.
func podsWithFilledChangedCell(screen string) []string {
	seen := map[string]bool{}
	for line := range strings.SplitSeq(stripSGR(screen), "\n") {
		name := demoWebPodPattern.FindString(line)
		if name == "" {
			continue
		}
		if len(changedCellPattern.FindAllString(line, -1)) >= 2 {
			seen[name] = true
		}
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// stripSGR removes ANSI escape sequences so column values are matchable.
func stripSGR(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`)
