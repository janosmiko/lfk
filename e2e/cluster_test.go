//go:build e2e

package e2e

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// clusterStartupTimeout is longer than startupTimeout: a real apiserver
// connection and RBAC self-check are slower than the demo fake's in-memory
// client.
const clusterStartupTimeout = 90 * time.Second

// TestClusterModeCronJobLogs opens the seeded CronJob's logs (TASK-896)
// against a real cluster, unlike TestDemoModeCronJobLogs's fake selectors.
// Skipped without one - .github/workflows/e2e-cluster.yml provisions it.
func TestClusterModeCronJobLogs(t *testing.T) {
	if os.Getenv("LFK_E2E_REAL_CLUSTER") == "" {
		t.Skip("LFK_E2E_REAL_CLUSTER not set; this test needs a real cluster, see .github/workflows/e2e-cluster.yml")
	}
	marker := os.Getenv("LFK_E2E_LOG_MARKER")
	if marker == "" {
		t.Fatal("LFK_E2E_LOG_MARKER must be set alongside LFK_E2E_REAL_CLUSTER")
	}

	binPath := buildBinary(t)

	cmd := exec.Command(binPath)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color") // KUBECONFIG comes from the CI job's environment

	ptmx, out, stderr := runUnderPty(t, cmd)

	// Same navigation as TestDemoModeCronJobLogs: select the (only) context,
	// jump to the CronJobs resource type, select the (only) row, open logs.
	sendKeys(t, ptmx, "\r")
	time.Sleep(300 * time.Millisecond)
	sendKeys(t, ptmx, "/CronJobs\r")
	time.Sleep(300 * time.Millisecond)
	sendKeys(t, ptmx, "\r")
	time.Sleep(300 * time.Millisecond)
	sendKeys(t, ptmx, "\x0c") // ctrl+l: open the fullscreen log viewer

	waitFor(t, out, marker, clusterStartupTimeout)

	if strings.Contains(out.String(), "panic:") || strings.Contains(stderr.String(), "panic:") {
		t.Fatalf("lfk panicked; pty output:\n%s\nstderr:\n%s", out.String(), stderr.String())
	}
	if strings.Contains(out.String(), "No runs yet") {
		t.Fatalf("CronJob log resolution reported no runs; pty output:\n%s", out.String())
	}
}
