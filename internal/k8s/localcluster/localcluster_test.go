package localcluster

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"
)

func TestErrNotSupportedIsSentinel(t *testing.T) {
	concat := errors.New("kind: " + ErrNotSupported.Error())
	if errors.Is(concat, ErrNotSupported) {
		t.Fatal("plain string concat should not satisfy errors.Is — guard against accidental wrap")
	}
	wrapped := fmt.Errorf("kind start dev: %w", ErrNotSupported)
	if !errors.Is(wrapped, ErrNotSupported) {
		t.Fatal("fmt.Errorf %w wrap must satisfy errors.Is")
	}
}

func TestCreateSpecZeroValueIsValid(t *testing.T) {
	// Constructing a CreateSpec with all zero values should compile and not panic.
	// Real validation lives in the wizard; this is a compile-fence smoke test
	// so refactors don't accidentally remove a field that callers depend on.
	var spec CreateSpec
	if spec.Nodes != 0 {
		t.Fatal("zero CreateSpec must have Nodes == 0")
	}
}

func TestClusterFieldsAccessible(t *testing.T) {
	c := Cluster{
		Provider:    "kind",
		Name:        "dev",
		ContextName: "kind-dev",
		Status:      ClusterStatusRunning,
		K8sVersion:  "v1.30.0",
		Nodes:       1,
		Age:         "2d",
	}
	if c.ContextName != "kind-dev" {
		t.Fatalf("ContextName not set")
	}
}

func TestAllReturnsAllKnownProviders(t *testing.T) {
	got := All()
	if len(got) != 3 {
		t.Fatalf("All() returned %d providers, want 3", len(got))
	}
	names := map[string]bool{}
	for _, p := range got {
		names[p.Name()] = true
	}
	for _, want := range []string{"kind", "k3d", "minikube"} {
		if !names[want] {
			t.Fatalf("All() missing %q", want)
		}
	}
}

func TestInstalledFiltersByLookPath(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(name string) (string, error) {
			if name == "kind" {
				return "/usr/bin/kind", nil
			}
			return "", exec.ErrNotFound
		},
	}
	// Build providers with the fake runner so LookPath is hermetic.
	provs := []Provider{
		newKindProvider(fake),
		newK3dProvider(fake),
		newMinikubeProvider(fake),
	}
	got := installedFrom(provs)
	if len(got) != 1 || got[0].Name() != "kind" {
		t.Fatalf("Installed() = %v, want [kind]", got)
	}
}

func TestByNameReturnsMatching(t *testing.T) {
	// ByName is installation-agnostic — it returns a Provider struct
	// regardless of whether the underlying CLI is on $PATH. Do not add
	// Installed() filtering here; that would make this test depend on
	// the test host's environment.
	if ByName("kind") == nil {
		t.Fatal("ByName(kind) returned nil")
	}
	if ByName("does-not-exist") != nil {
		t.Fatal("ByName(does-not-exist) should be nil")
	}
}

func TestInstalledDelegates(t *testing.T) {
	// Installed() is a thin delegation to installedFrom(All()) using
	// realRunner. We can't assert which providers are present (depends
	// on $PATH on the test host), but we can guarantee:
	//   1. the call doesn't panic
	//   2. it returns a non-nil slice (zero installed → empty, not nil)
	got := Installed()
	if got == nil {
		t.Fatal("Installed() must return non-nil slice; callers range over it")
	}
}

func TestValidateConfigFile(t *testing.T) {
	tmp, err := os.CreateTemp("", "lfk-validate-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(tmp.Name()) })
	_ = tmp.Close()

	cases := []struct {
		name   string
		path   string
		wantOK bool
	}{
		{"empty is allowed", "", true},
		{"absolute existing file", tmp.Name(), true},
		{"relative path rejected", "config.yaml", false},
		{"traversal segments rejected", "/tmp/../etc/passwd", false},
		{"non-existent path rejected", "/tmp/lfk-does-not-exist-" + tmp.Name(), false},
		{"directory rejected", "/tmp", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateConfigFile(tc.path)
			if tc.wantOK && err != nil {
				t.Fatalf("expected ok, got %v", err)
			}
			if !tc.wantOK {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !errors.Is(err, ErrInvalidConfigFile) {
					t.Fatalf("error = %v, want wrap of ErrInvalidConfigFile", err)
				}
			}
		})
	}
}
