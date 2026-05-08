package localcluster

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

const k3dListJSONOneRunning = `[
  {
    "name": "staging",
    "serversRunning": 1,
    "serversCount": 1,
    "agentsRunning": 1,
    "agentsCount": 1
  }
]`

const k3dListJSONOneStopped = `[
  {
    "name": "stopped-one",
    "serversRunning": 0,
    "serversCount": 1,
    "agentsRunning": 0,
    "agentsCount": 0
  }
]`

func TestK3dList_ParsesRunning(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/k3d", nil },
		RunFn: func(_ context.Context, _ string, _ ...string) (string, string, int, error) {
			return k3dListJSONOneRunning, "", 0, nil
		},
	}
	p := newK3dProvider(fake)
	got, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []Cluster{{
		Provider: "k3d", Name: "staging", ContextName: "k3d-staging",
		Status: ClusterStatusRunning, Nodes: 2,
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestK3dList_ParsesStopped(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/k3d", nil },
		RunFn: func(_ context.Context, _ string, _ ...string) (string, string, int, error) {
			return k3dListJSONOneStopped, "", 0, nil
		},
	}
	p := newK3dProvider(fake)
	got, _ := p.List(context.Background())
	if len(got) != 1 || got[0].Status != ClusterStatusStopped {
		t.Fatalf("got %+v", got)
	}
}

func TestK3dList_EmptyArray(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/k3d", nil },
		RunFn: func(_ context.Context, _ string, _ ...string) (string, string, int, error) {
			return "[]", "", 0, nil
		},
	}
	p := newK3dProvider(fake)
	got, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil empty slice for [] input; callers may range without nil-checking")
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 clusters, got %v", got)
	}
}

func TestK3dList_MalformedJSONErrors(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/k3d", nil },
		RunFn: func(_ context.Context, _ string, _ ...string) (string, string, int, error) {
			return "{not json", "", 0, nil
		},
	}
	p := newK3dProvider(fake)
	_, err := p.List(context.Background())
	if err == nil || !strings.Contains(err.Error(), "k3d:") {
		t.Fatalf("expected wrapped 'k3d:' parse error, got %v", err)
	}
}

func TestK3dList_NonZeroExitErrors(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/k3d", nil },
		RunFn: func(_ context.Context, _ string, _ ...string) (string, string, int, error) {
			return "", "k3d not initialised", 1, errors.New("exit 1")
		},
	}
	p := newK3dProvider(fake)
	_, err := p.List(context.Background())
	if err == nil {
		t.Fatal("expected error on non-zero exit")
	}
}

func TestK3dCreate_NameOnly(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/k3d", nil },
		RunFn:      func(context.Context, string, ...string) (string, string, int, error) { return "", "", 0, nil },
	}
	p := newK3dProvider(fake)
	if err := p.Create(context.Background(), CreateSpec{Name: "staging"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got := strings.Join(fake.CallsSnapshot()[0].Args, " ")
	want := "cluster create staging"
	if got != want {
		t.Fatalf("argv = %q, want %q", got, want)
	}
}

func TestK3dCreate_WithVersionAndAgents(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/k3d", nil },
		RunFn:      func(context.Context, string, ...string) (string, string, int, error) { return "", "", 0, nil },
	}
	p := newK3dProvider(fake)
	_ = p.Create(context.Background(), CreateSpec{Name: "x", K8sVersion: "v1.30.0", Nodes: 3})
	got := strings.Join(fake.CallsSnapshot()[0].Args, " ")
	if !strings.Contains(got, "--image rancher/k3s:v1.30.0-k3s1") {
		t.Fatalf("argv = %q, want --image rancher/k3s:v1.30.0-k3s1", got)
	}
	if !strings.Contains(got, "--agents 2") {
		t.Fatalf("argv = %q, want --agents 2 (Nodes-1)", got)
	}
}

func TestK3dDelete(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/k3d", nil },
		RunFn:      func(context.Context, string, ...string) (string, string, int, error) { return "", "", 0, nil },
	}
	p := newK3dProvider(fake)
	if err := p.Delete(context.Background(), "staging"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if strings.Join(fake.CallsSnapshot()[0].Args, " ") != "cluster delete staging" {
		t.Fatalf("argv = %q", fake.CallsSnapshot()[0].Args)
	}
}

func TestK3dStartStop(t *testing.T) {
	fake := &FakeRunner{
		LookPathFn: func(string) (string, error) { return "/usr/bin/k3d", nil },
		RunFn:      func(context.Context, string, ...string) (string, string, int, error) { return "", "", 0, nil },
	}
	p := newK3dProvider(fake)
	lp, ok := p.(LifecycleProvider)
	if !ok {
		t.Fatal("k3dProvider must satisfy LifecycleProvider")
	}
	if err := lp.Start(context.Background(), "staging"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := lp.Stop(context.Background(), "staging"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	calls := fake.CallsSnapshot()
	if strings.Join(calls[0].Args, " ") != "cluster start staging" {
		t.Fatalf("Start argv = %q", calls[0].Args)
	}
	if strings.Join(calls[1].Args, " ") != "cluster stop staging" {
		t.Fatalf("Stop argv = %q", calls[1].Args)
	}
}
