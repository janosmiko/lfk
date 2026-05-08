package app

import (
	"context"
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/k8s/localcluster"
)

// fakeProvider is a minimal localcluster.LifecycleProvider for tests
// of the start/stop/delete/create cmds. Start/Stop methods make this
// satisfy LifecycleProvider too — tests pass it to
// {start,stop}LocalClusterCmd which require the narrower interface.
type fakeProvider struct {
	name      string
	installed bool
	listFn    func(context.Context) ([]localcluster.Cluster, error)
}

func (f fakeProvider) Name() string    { return f.name }
func (f fakeProvider) Installed() bool { return f.installed }
func (f fakeProvider) List(ctx context.Context) ([]localcluster.Cluster, error) {
	return f.listFn(ctx)
}
func (fakeProvider) Create(context.Context, localcluster.CreateSpec) error { return nil }
func (fakeProvider) Delete(context.Context, string) error                  { return nil }
func (fakeProvider) Start(context.Context, string) error                   { return nil }
func (fakeProvider) Stop(context.Context, string) error                    { return nil }

func TestDetectLocalClustersCmd_AggregatesProviders(t *testing.T) {
	provs := []localcluster.Provider{
		fakeProvider{
			name: "kind", installed: true,
			listFn: func(context.Context) ([]localcluster.Cluster, error) {
				return []localcluster.Cluster{{Provider: "kind", Name: "dev", ContextName: "kind-dev", Status: localcluster.ClusterStatusRunning}}, nil
			},
		},
		fakeProvider{
			name: "k3d", installed: true,
			listFn: func(context.Context) ([]localcluster.Cluster, error) {
				return nil, errors.New("k3d boom")
			},
		},
	}
	cmd := detectLocalClustersCmd(7, provs)
	msg, ok := cmd().(localClustersDetectedMsg)
	if !ok {
		t.Fatalf("expected localClustersDetectedMsg, got %T", cmd())
	}
	if msg.gen != 7 {
		t.Fatalf("gen = %d, want 7", msg.gen)
	}
	if len(msg.clusters) != 1 {
		t.Fatalf("expected 1 cluster (kind), got %v", msg.clusters)
	}
	if msg.providerErrors["k3d"] == "" {
		t.Fatalf("expected k3d error captured, got %v", msg.providerErrors)
	}
}

func TestDetectLocalClustersCmd_NoProvidersInstalled(t *testing.T) {
	cmd := detectLocalClustersCmd(1, []localcluster.Provider{
		fakeProvider{name: "kind", installed: false},
	})
	msg := cmd().(localClustersDetectedMsg)
	if len(msg.clusters) != 0 {
		t.Fatalf("expected 0 clusters")
	}
	if len(msg.providerErrors) != 0 {
		t.Fatalf("expected no provider errors, got %v", msg.providerErrors)
	}
}

func TestDetectLocalClustersCmdReturnsTeaCmd(t *testing.T) {
	// Compile-fence that the cmd's static type is tea.Cmd.
	cmd := teaCmdAssert(detectLocalClustersCmd(0, nil))
	if cmd == nil {
		t.Fatal("nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(localClustersDetectedMsg); !ok {
		t.Fatalf("msg type = %T, want localClustersDetectedMsg", msg)
	}
}

// teaCmdAssert exists only to assert at compile time that
// detectLocalClustersCmd returns a tea.Cmd. Inlining the type via
// `var x tea.Cmd = ...` triggers staticcheck QF1011 (omit inferred
// type); a typed argument keeps the assertion without redundancy.
func teaCmdAssert(c tea.Cmd) tea.Cmd { return c }

func TestCreateLocalClusterCmd_ReturnsMsg(t *testing.T) {
	prov := fakeProvider{
		name: "kind", installed: true,
		listFn: func(context.Context) ([]localcluster.Cluster, error) { return nil, nil },
	}
	cmd := createLocalClusterCmd(context.Background(), 7, prov, localcluster.CreateSpec{Name: "dev"})
	msg := cmd().(localClusterCreatedMsg)
	if msg.gen != 7 || msg.provider != "kind" || msg.name != "dev" {
		t.Fatalf("msg = %+v", msg)
	}
}

func TestStartLocalClusterCmd(t *testing.T) {
	prov := fakeProvider{name: "k3d", installed: true}
	cmd := startLocalClusterCmd(context.Background(), 1, prov, "staging")
	msg := cmd().(localClusterMutatedMsg)
	if msg.gen != 1 || msg.verb != "started" {
		t.Fatalf("msg = %+v, want gen=1 verb=started", msg)
	}
}

func TestStopLocalClusterCmd(t *testing.T) {
	prov := fakeProvider{name: "k3d", installed: true}
	cmd := stopLocalClusterCmd(context.Background(), 2, prov, "staging")
	msg := cmd().(localClusterMutatedMsg)
	if msg.gen != 2 || msg.verb != "stopped" {
		t.Fatalf("msg = %+v, want gen=2 verb=stopped", msg)
	}
}

func TestDeleteLocalClusterCmd(t *testing.T) {
	prov := fakeProvider{name: "kind", installed: true}
	cmd := deleteLocalClusterCmd(context.Background(), 3, prov, "dev")
	msg := cmd().(localClusterMutatedMsg)
	if msg.gen != 3 || msg.verb != "deleted" {
		t.Fatalf("msg = %+v, want gen=3 verb=deleted", msg)
	}
}
