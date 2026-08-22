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
	cronJob := os.Getenv("LFK_CRONJOB")
	if cronJob == "" {
		cronJob = "nightly-backup"
	}

	binPath := buildBinary(t)

	cmd := exec.Command(binPath)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color") // KUBECONFIG comes from the CI job's environment

	ptmx, out, stderr := runUnderPty(t, cmd)

	// Same navigation as TestDemoModeCronJobLogs, but each keystroke waits
	// for its target's own text first - a real apiserver's context and
	// discovery lists populate too slowly for a fixed sleep to outlast.
	at := waitForThenSend(t, out, "kind-lfk-e2e", ptmx, "\r", clusterStartupTimeout)
	at = waitForNewThenSend(t, out, at, "CronJobs", ptmx, "/CronJobs\r", clusterStartupTimeout)
	at = waitForNewThenSend(t, out, at, cronJob, ptmx, "\r", clusterStartupTimeout)
	// Wait for the Job row rather than the CronJob's own name. The CronJob name
	// prefixes the Job's, so the shorter string is already on screen in the
	// details pane and a repaint would satisfy the wait before the drilled-in
	// list exists, sending ctrl+l at a selection that resolves to no pod.
	at = waitForNewThenSend(t, out, at, cronJob+"-run", ptmx, "\x0c", clusterStartupTimeout)

	// "half page" belongs to the fullscreen viewer's footer and nothing on the
	// way here draws it, so this separates "ctrl+l did not land" from "the logs
	// never arrived". The failure that prompted this reported a missing marker
	// while the app was still in the explorer.
	//
	// Both waits run from the offset ctrl+l was sent at. The viewer paints its
	// body and its footer in one frame, so advancing past the footer would step
	// over the log line this is looking for.
	waitForAfter(t, out, at, "half page", clusterStartupTimeout)
	waitForAfter(t, out, at, marker, clusterStartupTimeout)

	if strings.Contains(out.String(), "panic:") || strings.Contains(stderr.String(), "panic:") {
		t.Fatalf("lfk panicked; pty output:\n%s\nstderr:\n%s", out.String(), stderr.String())
	}
	if strings.Contains(out.String(), "No runs yet") {
		t.Fatalf("CronJob log resolution reported no runs; pty output:\n%s", out.String())
	}
}
