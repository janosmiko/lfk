package k8s

import (
	"errors"
	"os/exec"
	"testing"
)

func TestKubectlPath_HonoursKubectlBinOverride(t *testing.T) {
	t.Setenv("KUBECTL_BIN", "/opt/stub/kubectl")

	got, err := KubectlPath()
	if err != nil {
		t.Fatalf("KubectlPath() unexpected error: %v", err)
	}
	if got != "/opt/stub/kubectl" {
		t.Errorf("KubectlPath() = %q, want %q", got, "/opt/stub/kubectl")
	}
}

func TestKubectlPath_FallsBackToPathLookup(t *testing.T) {
	t.Setenv("KUBECTL_BIN", "")

	want, lookupErr := exec.LookPath("kubectl")

	got, err := KubectlPath()

	if lookupErr != nil {
		// kubectl isn't installed in this environment: KubectlPath must
		// surface the same not-found condition PATH lookup would.
		if err == nil {
			t.Fatalf("KubectlPath() = %q, nil; want error when kubectl is absent from PATH", got)
		}
		return
	}

	if err != nil {
		t.Fatalf("KubectlPath() unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("KubectlPath() = %q, want %q", got, want)
	}
}

func TestKubectlPath_ErrorWhenKubectlAbsent(t *testing.T) {
	emptyPath := t.TempDir()
	t.Setenv("KUBECTL_BIN", "")
	t.Setenv("PATH", emptyPath)

	got, err := KubectlPath()
	if err == nil {
		t.Fatalf("KubectlPath() = %q, nil; want error when kubectl is not on PATH", got)
	}
	if !errors.Is(err, exec.ErrNotFound) {
		t.Errorf("KubectlPath() error = %v, want wrapping exec.ErrNotFound", err)
	}
}
