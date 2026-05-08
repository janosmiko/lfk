package localcluster

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

const minikubeProfileListJSON = `{
  "valid": [
    {"Name": "experiments", "Status": "Running", "Config": {"KubernetesConfig": {"KubernetesVersion": "v1.31.0"}, "Nodes": [{"Name": "m02"}]}},
    {"Name": "old", "Status": "Stopped", "Config": {"KubernetesConfig": {"KubernetesVersion": "v1.28.0"}, "Nodes": []}}
  ],
  "invalid": []
}`

func TestMinikubeList_ParsesValid(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/minikube", nil },
		RunFn: func(context.Context, string, ...string) (string, string, int, error) {
			return minikubeProfileListJSON, "", 0, nil
		},
	}
	p := newMinikubeProvider(fake)
	got, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []Cluster{
		{Provider: "minikube", Name: "experiments", ContextName: "experiments", Status: ClusterStatusRunning, K8sVersion: "v1.31.0", Nodes: 1},
		{Provider: "minikube", Name: "old", ContextName: "old", Status: ClusterStatusStopped, K8sVersion: "v1.28.0", Nodes: 0},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestMinikubeList_EmptyValid(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/minikube", nil },
		RunFn: func(context.Context, string, ...string) (string, string, int, error) {
			return `{"valid": [], "invalid": []}`, "", 0, nil
		},
	}
	p := newMinikubeProvider(fake)
	got, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 clusters, got %v", got)
	}
}

func TestMinikubeList_NonZeroExitErrors(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/minikube", nil },
		RunFn: func(context.Context, string, ...string) (string, string, int, error) {
			return "", "boom", 1, errors.New("exit 1")
		},
	}
	p := newMinikubeProvider(fake)
	_, err := p.List(context.Background())
	if err == nil || !strings.Contains(err.Error(), "minikube:") {
		t.Fatalf("expected wrapped 'minikube:' error, got %v", err)
	}
}

func TestMinikubeList_BadJSON(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/minikube", nil },
		RunFn: func(context.Context, string, ...string) (string, string, int, error) {
			return "garbage", "", 0, nil
		},
	}
	p := newMinikubeProvider(fake)
	_, err := p.List(context.Background())
	if err == nil {
		t.Fatal("expected error on bad JSON")
	}
}

func TestMinikubeStatusMapping(t *testing.T) {
	cases := []struct {
		raw  string
		want ClusterStatus
	}{
		{"Running", ClusterStatusRunning},
		{"Stopped", ClusterStatusStopped},
		{"running", ClusterStatusRunning},
		{"Paused", ClusterStatusUnknown},
		{"Stopping", ClusterStatusUnknown},
		{"Starting", ClusterStatusUnknown},
		{"Misconfigured", ClusterStatusUnknown},
		{"", ClusterStatusUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			if got := minikubeStatus(tc.raw); got != tc.want {
				t.Fatalf("minikubeStatus(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestMinikubeCreate_RejectsConfigFile(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/minikube", nil },
		RunFn:      func(context.Context, string, ...string) (string, string, int, error) { return "", "", 0, nil },
	}
	p := newMinikubeProvider(fake)
	err := p.Create(context.Background(), CreateSpec{Name: "exp", ConfigFile: "/tmp/foo.yaml"})
	if err == nil || !strings.Contains(err.Error(), "config-file is not supported") {
		t.Fatalf("expected ConfigFile-not-supported error, got %v", err)
	}
	if len(fake.CallsSnapshot()) != 0 {
		t.Fatal("Create must not invoke minikube CLI when ConfigFile is set")
	}
}

func TestMinikubeCreate_NameOnly(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/minikube", nil },
		RunFn:      func(context.Context, string, ...string) (string, string, int, error) { return "", "", 0, nil },
	}
	p := newMinikubeProvider(fake)
	if err := p.Create(context.Background(), CreateSpec{Name: "exp"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got := strings.Join(fake.CallsSnapshot()[0].Args, " ")
	want := "start --profile exp --interactive=false"
	if got != want {
		t.Fatalf("argv = %q, want %q", got, want)
	}
}

func TestMinikubeCreate_AlwaysPassesNonInteractive(t *testing.T) {
	// Critical: lfk hosts minikube under its TUI, so an interactive
	// prompt (driver picker, sudo) would silently block until the
	// 10-min timeout. --interactive=false forces minikube to error
	// instead, surfacing the failure to the user.
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/minikube", nil },
		RunFn:      func(context.Context, string, ...string) (string, string, int, error) { return "", "", 0, nil },
	}
	p := newMinikubeProvider(fake)
	_ = p.Create(context.Background(), CreateSpec{Name: "exp", K8sVersion: "v1.30.0", Nodes: 2})
	got := strings.Join(fake.CallsSnapshot()[0].Args, " ")
	if !strings.Contains(got, "--interactive=false") {
		t.Fatalf("argv = %q, must contain --interactive=false", got)
	}
}

func TestMinikubeCreate_WithVersionAndNodes(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/minikube", nil },
		RunFn:      func(context.Context, string, ...string) (string, string, int, error) { return "", "", 0, nil },
	}
	p := newMinikubeProvider(fake)
	_ = p.Create(context.Background(), CreateSpec{Name: "x", K8sVersion: "v1.30.0", Nodes: 3})
	got := strings.Join(fake.CallsSnapshot()[0].Args, " ")
	if !strings.Contains(got, "--kubernetes-version v1.30.0") {
		t.Fatalf("argv = %q, want --kubernetes-version v1.30.0", got)
	}
	if !strings.Contains(got, "--nodes 3") {
		t.Fatalf("argv = %q, want --nodes 3", got)
	}
}

func TestMinikubeDeleteStartStop(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/minikube", nil },
		RunFn:      func(context.Context, string, ...string) (string, string, int, error) { return "", "", 0, nil },
	}
	p := newMinikubeProvider(fake)
	lp, ok := p.(LifecycleProvider)
	if !ok {
		t.Fatal("minikubeProvider must satisfy LifecycleProvider")
	}
	if err := p.Delete(context.Background(), "exp"); err != nil {
		t.Fatal(err)
	}
	if err := lp.Start(context.Background(), "exp"); err != nil {
		t.Fatal(err)
	}
	if err := lp.Stop(context.Background(), "exp"); err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{
		"delete --profile exp",
		"start --profile exp",
		"stop --profile exp",
	}
	calls := fake.CallsSnapshot()
	for i, want := range wantArgs {
		got := strings.Join(calls[i].Args, " ")
		if got != want {
			t.Fatalf("call %d argv = %q, want %q", i, got, want)
		}
	}
}
