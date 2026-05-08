package localcluster

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestKindList_Empty(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/kind", nil },
		RunFn: func(_ context.Context, name string, args ...string) (string, string, int, error) {
			return "No kind clusters found.\n", "", 0, nil
		},
	}
	p := newKindProvider(fake)
	got, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 clusters, got %v", got)
	}
}

func TestKindList_OneRunningCluster(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/kind", nil },
		RunFn: func(_ context.Context, name string, args ...string) (string, string, int, error) {
			switch name {
			case "kind":
				return "dev\n", "", 0, nil
			case "docker":
				return "running\n", "", 0, nil
			}
			t.Fatalf("unexpected cmd %q", name)
			return "", "", 0, nil
		},
	}
	p := newKindProvider(fake)
	got, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []Cluster{{Provider: "kind", Name: "dev", ContextName: "kind-dev", Status: ClusterStatusRunning}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("List = %+v\nwant %+v", got, want)
	}
}

func TestKindList_DockerMissingMarksUnknown(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/kind", nil },
		RunFn: func(_ context.Context, name string, _ ...string) (string, string, int, error) {
			if name == "docker" {
				return "", "docker: command not found", 127, errors.New("not found")
			}
			return "dev\n", "", 0, nil
		},
	}
	p := newKindProvider(fake)
	got, _ := p.List(context.Background())
	if len(got) != 1 || got[0].Status != ClusterStatusUnknown {
		t.Fatalf("expected single cluster with unknown status, got %+v", got)
	}
}

func TestKindList_KindCLIError(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/kind", nil },
		RunFn: func(_ context.Context, name string, _ ...string) (string, string, int, error) {
			return "", "permission denied", 1, errors.New("exit status 1")
		},
	}
	p := newKindProvider(fake)
	_, err := p.List(context.Background())
	if err == nil || !strings.Contains(err.Error(), "kind:") {
		t.Fatalf("expected wrapped 'kind:' error, got %v", err)
	}
}

func TestKindList_MultilineFiltered(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/kind", nil },
		RunFn: func(_ context.Context, name string, _ ...string) (string, string, int, error) {
			if name == "kind" {
				return "dev\n\nstaging\n", "", 0, nil
			}
			return "running\n", "", 0, nil
		},
	}
	p := newKindProvider(fake)
	got, _ := p.List(context.Background())
	if len(got) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(got))
	}
}

func TestKindCreate_BuildsArgvNameOnly(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/kind", nil },
		RunFn:      func(context.Context, string, ...string) (string, string, int, error) { return "", "", 0, nil },
	}
	p := newKindProvider(fake)
	if err := p.Create(context.Background(), CreateSpec{Name: "dev"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	calls := fake.CallsSnapshot()
	if len(calls) != 1 || calls[0].Name != "kind" {
		t.Fatalf("calls = %+v", calls)
	}
	got := strings.Join(calls[0].Args, " ")
	want := "create cluster --name dev"
	if got != want {
		t.Fatalf("argv = %q, want %q", got, want)
	}
}

func TestKindCreate_BuildsArgvWithVersion(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/kind", nil },
		RunFn:      func(context.Context, string, ...string) (string, string, int, error) { return "", "", 0, nil },
	}
	p := newKindProvider(fake)
	_ = p.Create(context.Background(), CreateSpec{Name: "dev", K8sVersion: "v1.30.0"})
	got := strings.Join(fake.CallsSnapshot()[0].Args, " ")
	want := "create cluster --name dev --image kindest/node:v1.30.0"
	if got != want {
		t.Fatalf("argv = %q, want %q", got, want)
	}
}

func TestKindCreate_BuildsArgvWithMultipleNodes(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/kind", nil },
		RunFn:      func(context.Context, string, ...string) (string, string, int, error) { return "", "", 0, nil },
	}
	p := newKindProvider(fake)
	_ = p.Create(context.Background(), CreateSpec{Name: "dev", Nodes: 3})
	got := strings.Join(fake.CallsSnapshot()[0].Args, " ")
	if !strings.Contains(got, "--config") {
		t.Fatalf("argv = %q, want --config flag for multi-node", got)
	}
}

func TestKindCreate_PropagatesError(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/kind", nil },
		RunFn: func(_ context.Context, _ string, _ ...string) (string, string, int, error) {
			return "", "image pull failed", 1, errors.New("exit 1")
		},
	}
	p := newKindProvider(fake)
	err := p.Create(context.Background(), CreateSpec{Name: "dev"})
	if err == nil || !strings.Contains(err.Error(), "kind:") {
		t.Fatalf("expected wrapped 'kind:' error, got %v", err)
	}
}

func TestKindDelete_BuildsArgv(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/kind", nil },
		RunFn:      func(context.Context, string, ...string) (string, string, int, error) { return "", "", 0, nil },
	}
	p := newKindProvider(fake)
	if err := p.Delete(context.Background(), "dev"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got := strings.Join(fake.CallsSnapshot()[0].Args, " ")
	want := "delete cluster --name dev"
	if got != want {
		t.Fatalf("argv = %q, want %q", got, want)
	}
}

func TestKindDoesNotImplementLifecycleProvider(t *testing.T) {
	fake := &FakeRunner{LookPathFn: func(string) (string, error) { return "/usr/bin/kind", nil }}
	p := newKindProvider(fake)
	if _, ok := p.(LifecycleProvider); ok {
		t.Fatal("kindProvider must NOT satisfy LifecycleProvider — kind has no native start/stop")
	}
}

func TestKindContainerStatusMapping(t *testing.T) {
	cases := []struct {
		dockerStdout string
		want         ClusterStatus
	}{
		{"running\n", ClusterStatusRunning},
		{"exited\n", ClusterStatusStopped},
		{"paused\n", ClusterStatusStopped},
		{"dead\n", ClusterStatusStopped},
		{"created\n", ClusterStatusUnknown}, // unknown docker state
		{"restarting\n", ClusterStatusUnknown},
		{"\n", ClusterStatusUnknown},
	}
	for _, tc := range cases {
		t.Run(strings.TrimSpace(tc.dockerStdout), func(t *testing.T) {
			fake := &FakeRunner{
				RunFn: func(_ context.Context, _ string, _ ...string) (string, string, int, error) {
					return tc.dockerStdout, "", 0, nil
				},
			}
			got := kindContainerStatus(context.Background(), fake, "dev")
			if got != tc.want {
				t.Fatalf("status for %q = %v, want %v", tc.dockerStdout, got, tc.want)
			}
		})
	}
}
