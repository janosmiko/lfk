package k8s

import (
	"errors"
	"os"
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

func TestDemoModeEnabled_DefaultsOff(t *testing.T) {
	if DemoModeEnabled() {
		t.Fatalf("DemoModeEnabled() = true by default, want false")
	}
}

func TestKubectlPath_DemoModeReturnsOwnExecutable(t *testing.T) {
	SetDemoMode(true)
	defer SetDemoMode(false)

	want, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error: %v", err)
	}

	got, err := KubectlPath()
	if err != nil {
		t.Fatalf("KubectlPath() unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("KubectlPath() = %q, want %q (this binary)", got, want)
	}
}

// TestKubectlPath_DemoModeOverridesKubectlBin proves the critical property:
// with demo mode on, no code path may resolve a kubectl on the user's PATH or
// via KUBECTL_BIN — only this binary.
func TestKubectlPath_DemoModeOverridesKubectlBin(t *testing.T) {
	t.Setenv("KUBECTL_BIN", "/opt/stub/kubectl")
	SetDemoMode(true)
	defer SetDemoMode(false)

	want, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error: %v", err)
	}

	got, err := KubectlPath()
	if err != nil {
		t.Fatalf("KubectlPath() unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("KubectlPath() = %q, want %q (demo mode must win over KUBECTL_BIN)", got, want)
	}
}

// TestDemoKubectlArgs_PrependsSubcommandOnlyInDemoMode guards the fix for
// the finding that every kubectl exec.Command call site built kubectl-shaped
// argv without the __demo-kubectl prefix: in demo mode KubectlPath() returns
// this process's own executable, so the re-exec'd binary must route into the
// hidden subcommand instead of parsing "edit"/"describe"/etc. as its own
// root-command argv (which would launch a second TUI instead of the demo
// kubectl emulation).
func TestDemoKubectlArgs_PrependsSubcommandOnlyInDemoMode(t *testing.T) {
	in := []string{"edit", "pod/web", "-n", "demo", "--context", "demo"}

	SetDemoMode(true)
	got := DemoKubectlArgs(in)
	SetDemoMode(false)
	want := append([]string{"__demo-kubectl"}, in...)
	if len(got) != len(want) {
		t.Fatalf("DemoKubectlArgs(%v) = %v, want %v", in, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("DemoKubectlArgs(%v)[%d] = %q, want %q", in, i, got[i], want[i])
		}
	}

	got = DemoKubectlArgs(in)
	if len(got) != len(in) {
		t.Fatalf("DemoKubectlArgs(%v) outside demo mode = %v, want args unchanged", in, got)
	}
	for i := range in {
		if got[i] != in[i] {
			t.Errorf("DemoKubectlArgs(%v)[%d] outside demo mode = %q, want %q", in, i, got[i], in[i])
		}
	}
}

func TestKubectlPath_DemoModeOffRestoresNormalResolution(t *testing.T) {
	SetDemoMode(true)
	SetDemoMode(false)

	t.Setenv("KUBECTL_BIN", "/opt/stub/kubectl")
	got, err := KubectlPath()
	if err != nil {
		t.Fatalf("KubectlPath() unexpected error: %v", err)
	}
	if got != "/opt/stub/kubectl" {
		t.Errorf("KubectlPath() = %q, want %q after demo mode disabled", got, "/opt/stub/kubectl")
	}
}
