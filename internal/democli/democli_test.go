package democli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/k8s/demo"
)

func TestRun_LogsVerb(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(t.Context(), []string{"logs", "web-7d8f9c6b5-9k2pl", "-n", "demo", "--tail=2", "--timestamps"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.Count(stdout.String(), "\n") != 2 {
		t.Errorf("stdout = %q, want 2 lines", stdout.String())
	}
}

func TestRun_GetVerb(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(t.Context(), []string{"get", "deployment/" + demo.DeploymentWeb, "-n", demo.NamespaceDemo, "--context", "demo", "-o", "json"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(stdout.String(), `"matchLabels"`) {
		t.Errorf("stdout = %q, want matchLabels selector JSON", stdout.String())
	}
}

func TestRun_DescribeVerb(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(t.Context(), []string{"describe", "pod", demo.PodWebCrashLoop, "-n", demo.NamespaceDemo, "--context", "demo"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "CrashLoopBackOff") {
		t.Errorf("stdout = %q, want CrashLoopBackOff for the known crash-loop pod", stdout.String())
	}
}

// TestRun_RefusesUnsupportedVerbs guards acceptance criterion 5: every verb
// this package does not implement must refuse clearly and exit non-zero
// (via a non-nil error) rather than silently doing nothing or falling
// through to a real binary.
func TestRun_RefusesUnsupportedVerbs(t *testing.T) {
	for _, verb := range []string{"exec", "port-forward", "drain", "cordon", "debug", "apply", "delete"} {
		t.Run(verb, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := Run(t.Context(), []string{verb, "pod/x", "-n", "demo"}, &stdout, &stderr)
			if err == nil {
				t.Fatalf("Run(%q) error = nil, want non-nil (refusal must exit non-zero)", verb)
			}
			if !strings.Contains(stderr.String(), "not available in demo mode") {
				t.Errorf("stderr = %q, want a clear demo-mode refusal message", stderr.String())
			}
		})
	}
}

func TestRun_NoArgsRefuses(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := Run(t.Context(), nil, &stdout, &stderr); err == nil {
		t.Fatal("Run(nil) error = nil, want non-nil")
	}
}
