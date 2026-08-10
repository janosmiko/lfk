package democli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/k8s/demo"
)

// TestRun_DescribeDBMigratePod guards against describe.go drifting out of
// sync with the seed data in internal/k8s/demo/jobs.go, which seeds
// PodDBMigrate as PodFailed with a Terminated{Reason: "Error", ExitCode: 1}
// container -- a failed migration, not a completed one.
func TestRun_DescribeDBMigratePod(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(t.Context(), []string{"describe", "pod", demo.PodDBMigrate, "-n", demo.NamespaceJobs, "--context", "demo"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	out := stdout.String()
	for _, want := range []string{"Failed", "Reason:         Error", "Exit Code:      1"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout = %q, want it to contain %q", out, want)
		}
	}
	if strings.Contains(out, "Succeeded") {
		t.Errorf("stdout = %q, must not report Succeeded for a failed migration pod", out)
	}
	if strings.Contains(out, "Completed") {
		t.Errorf("stdout = %q, must not report Completed for a failed migration pod", out)
	}
}
