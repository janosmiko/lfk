package k8s

import (
	"sync"
	"testing"
)

// ReloadKubeconfig swaps c.contexts under configMu while running on a
// background goroutine (kubeconfig refresh, cluster create/delete). Every
// reader of c.contexts must take configMu.RLock or the swap races. These three
// accessors funnel under essentially every background API call / log stream, so
// the race is live in normal use. Run with -race.
func TestKubeconfigAccessorsRaceWithReload(t *testing.T) {
	c := newTestClient(t)
	const iters = 300

	var wg sync.WaitGroup

	wg.Go(func() {
		for range iters {
			_ = c.ReloadKubeconfig()
		}
	})

	reader := func() {
		for range iters {
			_ = c.KubeconfigPathForContext("test-context")
			_ = c.OriginalContextName("test-context")
			_, _ = c.restConfigForContext("test-context")
		}
	}
	for range 3 {
		wg.Go(reader)
	}

	wg.Wait()
}
