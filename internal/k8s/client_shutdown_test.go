package k8s

import (
	"testing"
	"time"
)

// A reflector stuck in an exec credential plugin never finishes, so Shutdown
// must give up waiting instead of hanging the quit.
func TestShutdown_BoundedWhenInformerGoroutineStuck(t *testing.T) {
	old := shutdownStopTimeout
	shutdownStopTimeout = 200 * time.Millisecond
	t.Cleanup(func() { shutdownStopTimeout = old })

	ic := newInformerCache(nil)
	ic.wg.Add(1) // never Done: simulates the stuck reflector
	c := &Client{informers: ic, informerMode: InformerCacheAlways}

	// Quit calls Shutdown twice (drain and main defer): wait once, not twice.
	limits := []time.Duration{shutdownStopTimeout + 300*time.Millisecond, 100 * time.Millisecond}
	for i, limit := range limits {
		start := time.Now()
		done := make(chan struct{})
		go func() {
			c.Shutdown()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("Shutdown call %d did not return within the cap", i+1)
		}
		if got := time.Since(start); got > limit {
			t.Fatalf("Shutdown call %d took %v, want under %v", i+1, got, limit)
		}
	}
}
