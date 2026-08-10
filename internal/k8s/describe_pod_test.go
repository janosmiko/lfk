package k8s

import (
	"strings"
	"testing"
)

// TestDescribePod_FailsClosedWhenKubectlNotFound guards TASK-865 finding 5:
// DescribePod was the only KubectlPath caller in the module that swallowed
// resolution failure and fell back to the bare "kubectl" name, letting
// exec.CommandContext resolve it on PATH at fork time instead of surfacing
// the error the other 31 call sites all return. It must fail closed like
// every other KubectlPath call site.
func TestDescribePod_FailsClosedWhenKubectlNotFound(t *testing.T) {
	emptyPath := t.TempDir()
	t.Setenv("KUBECTL_BIN", "")
	t.Setenv("PATH", emptyPath)

	c := NewTestClient(nil, nil)
	_, err := c.DescribePod(t.Context(), "test-ctx", "default", "web-1")
	if err == nil {
		t.Fatal("DescribePod() error = nil, want an error when kubectl is not on PATH")
	}
	if !strings.Contains(err.Error(), "kubectl not found") {
		t.Errorf("DescribePod() error = %v, want it to mention \"kubectl not found\"", err)
	}
}
