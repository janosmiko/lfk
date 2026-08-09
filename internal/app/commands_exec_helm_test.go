package app

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/k8s"
)

// TestResolveHelmPath_RefusesInDemoMode guards the last hole in acceptance
// criterion 5: helm shells out via exec.LookPath directly (KubectlPath does
// not cover it), so demo mode must be checked explicitly before ever
// resolving a real helm binary.
func TestResolveHelmPath_RefusesInDemoMode(t *testing.T) {
	k8s.SetDemoMode(true)
	defer k8s.SetDemoMode(false)

	path, err := resolveHelmPath()
	if err == nil {
		t.Fatalf("resolveHelmPath() = (%q, nil), want an error in demo mode", path)
	}
	if !strings.Contains(err.Error(), "demo mode") {
		t.Errorf("resolveHelmPath() error = %v, want it to mention demo mode", err)
	}
	if path != "" {
		t.Errorf("resolveHelmPath() path = %q, want empty on refusal", path)
	}
}

func TestResolveHelmPath_NormalModeMatchesLookPath(t *testing.T) {
	k8s.SetDemoMode(false)

	want, wantErr := exec.LookPath("helm")
	got, err := resolveHelmPath()

	if wantErr != nil {
		if err == nil {
			t.Fatalf("resolveHelmPath() = (%q, nil); want error when helm is absent from PATH", got)
		}
		if !errors.Is(err, exec.ErrNotFound) && !strings.Contains(err.Error(), "not found") {
			t.Errorf("resolveHelmPath() error = %v, want it to surface the LookPath failure", err)
		}
		return
	}

	if err != nil {
		t.Fatalf("resolveHelmPath() unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("resolveHelmPath() = %q, want %q", got, want)
	}
}
