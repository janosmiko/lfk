// commands_localcluster wraps localcluster.Provider operations as
// tea.Cmds so the manager overlay can dispatch them through bubbletea
// without blocking the UI. The bare *Cmd factory functions are the
// testable inner closures (used directly by commands_localcluster_test);
// the (Model).<verb>LocalClusterCmd methods register the work with the
// bgtasks Registry so it surfaces in :tasks and Ctrl+C can cancel
// long-running creates the same way it cancels other mutations.
package app

import (
	"context"
	"fmt"
	"sync"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/app/bgtasks"
	"github.com/janosmiko/lfk/internal/k8s/localcluster"
)

// createLocalClusterCmd is the bare cmd factory: dispatches Provider.Create
// and emits localClusterCreatedMsg with the result. ctx must be the
// cancellable context registered with bgtasks so the user-cancel path
// can interrupt the underlying CLI.
func createLocalClusterCmd(ctx context.Context, gen uint64, p localcluster.Provider, spec localcluster.CreateSpec) tea.Cmd {
	return func() tea.Msg {
		err := p.Create(ctx, spec)
		return localClusterCreatedMsg{
			gen:      gen,
			provider: p.Name(),
			name:     spec.Name,
			err:      err,
		}
	}
}

// startLocalClusterCmd dispatches LifecycleProvider.Start and emits
// the result as localClusterMutatedMsg{verb: "started"}. Takes the
// narrower interface so the type system enforces that only providers
// supporting start/stop reach this code path.
func startLocalClusterCmd(ctx context.Context, gen uint64, p localcluster.LifecycleProvider, name string) tea.Cmd {
	return func() tea.Msg {
		err := p.Start(ctx, name)
		return localClusterMutatedMsg{gen: gen, provider: p.Name(), name: name, verb: "started", err: err}
	}
}

// stopLocalClusterCmd dispatches LifecycleProvider.Stop and emits the
// result as localClusterMutatedMsg{verb: "stopped"}.
func stopLocalClusterCmd(ctx context.Context, gen uint64, p localcluster.LifecycleProvider, name string) tea.Cmd {
	return func() tea.Msg {
		err := p.Stop(ctx, name)
		return localClusterMutatedMsg{gen: gen, provider: p.Name(), name: name, verb: "stopped", err: err}
	}
}

// deleteLocalClusterCmd dispatches Provider.Delete and emits the result
// as localClusterMutatedMsg{verb: "deleted"}.
func deleteLocalClusterCmd(ctx context.Context, gen uint64, p localcluster.Provider, name string) tea.Cmd {
	return func() tea.Msg {
		err := p.Delete(ctx, name)
		return localClusterMutatedMsg{gen: gen, provider: p.Name(), name: name, verb: "deleted", err: err}
	}
}

// detectLocalClustersCmd returns a tea.Cmd that calls List() on every
// installed provider concurrently and packages the union of results
// (and any per-provider errors) into a localClustersDetectedMsg. gen
// is echoed back on the message so the handler can drop stale results
// from a previous open / refresh.
func detectLocalClustersCmd(gen uint64, provs []localcluster.Provider) tea.Cmd {
	return func() tea.Msg {
		var (
			mu       sync.Mutex
			clusters []localcluster.Cluster
			errs     = map[string]string{}
			wg       sync.WaitGroup
		)
		for _, p := range provs {
			if !p.Installed() {
				continue
			}
			wg.Add(1)
			go func(p localcluster.Provider) {
				defer wg.Done()
				cs, err := p.List(context.Background())
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					errs[p.Name()] = err.Error()
					return
				}
				clusters = append(clusters, cs...)
			}(p)
		}
		wg.Wait()
		return localClustersDetectedMsg{
			gen:            gen,
			clusters:       clusters,
			providerErrors: errs,
		}
	}
}

// dispatchCreateLocalCluster registers a cancellable bgtask, then
// returns a tea.Cmd that runs the create under that task's context.
// On completion the bgtask is finished and the resulting message lands
// in updateLocalClusterCreated.
func (m Model) dispatchCreateLocalCluster(p localcluster.Provider, spec localcluster.CreateSpec) tea.Cmd {
	registry := m.bgtasks
	ctx, cancel := context.WithCancel(context.Background())
	id := registry.StartCancellable(
		bgtasks.KindMutation,
		fmt.Sprintf("Create %s/%s", p.Name(), spec.Name),
		"",
		cancel,
	)
	gen := m.localClusterState.gen
	return wrapWithFinish(registry, id, createLocalClusterCmd(ctx, gen, p, spec))
}

// dispatchStartLocalCluster / dispatchStopLocalCluster / dispatchDeleteLocalCluster
// are the bgtasks-tracked wrappers for the three mutation cmds. Each
// one registers a cancellable bgtask so the user can hit Ctrl+C to
// abort a wedged op the same way other mutations are cancelled.
func (m Model) dispatchStartLocalCluster(p localcluster.LifecycleProvider, name string) tea.Cmd {
	registry := m.bgtasks
	ctx, cancel := context.WithCancel(context.Background())
	id := registry.StartCancellable(
		bgtasks.KindMutation,
		fmt.Sprintf("Start %s/%s", p.Name(), name),
		"",
		cancel,
	)
	gen := m.localClusterState.gen
	return wrapWithFinish(registry, id, startLocalClusterCmd(ctx, gen, p, name))
}

func (m Model) dispatchStopLocalCluster(p localcluster.LifecycleProvider, name string) tea.Cmd {
	registry := m.bgtasks
	ctx, cancel := context.WithCancel(context.Background())
	id := registry.StartCancellable(
		bgtasks.KindMutation,
		fmt.Sprintf("Stop %s/%s", p.Name(), name),
		"",
		cancel,
	)
	gen := m.localClusterState.gen
	return wrapWithFinish(registry, id, stopLocalClusterCmd(ctx, gen, p, name))
}

func (m Model) dispatchDeleteLocalCluster(p localcluster.Provider, name string) tea.Cmd {
	registry := m.bgtasks
	ctx, cancel := context.WithCancel(context.Background())
	id := registry.StartCancellable(
		bgtasks.KindMutation,
		fmt.Sprintf("Delete %s/%s", p.Name(), name),
		"",
		cancel,
	)
	gen := m.localClusterState.gen
	return wrapWithFinish(registry, id, deleteLocalClusterCmd(ctx, gen, p, name))
}

// dispatchDetectLocalClusters wraps detectLocalClustersCmd with bgtasks
// tracking so the per-2s watch-mode refresh (when watching at LevelClusters)
// surfaces in the title-bar indicator like every other resource list.
func (m Model) dispatchDetectLocalClusters(gen uint64, provs []localcluster.Provider) tea.Cmd {
	return m.trackBgTask(
		bgtasks.KindResourceList,
		"Detect local clusters",
		"",
		detectLocalClustersCmd(gen, provs),
	)
}

// wrapWithFinish takes a cmd that will produce a Msg, and returns a
// new cmd that finishes the bgtask before the message is delivered.
// Defer ensures Finish runs even if the inner cmd panics.
func wrapWithFinish(registry *bgtasks.Registry, id uint64, inner tea.Cmd) tea.Cmd {
	return func() tea.Msg {
		defer registry.Finish(id)
		if inner == nil {
			return nil
		}
		return inner()
	}
}
