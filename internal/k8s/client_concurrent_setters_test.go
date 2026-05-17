package k8s

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestSetSecretLazyLoadingConcurrentWithReads exercises the race between
// SetSecretLazyLoading and the readers in client_resources.go that consult
// the flag from tea.Cmd goroutines. Before the atomic conversion the
// bool field was written without synchronization while concurrent readers
// touched it, which the race detector flagged when those readers ran in
// parallel with a startup write (or any future runtime-config-reload path).
//
// Run with `go test -race`.
func TestSetSecretLazyLoadingConcurrentWithReads(t *testing.T) {
	c := NewTestClient(nil, nil)
	stop := make(chan struct{})
	var wg sync.WaitGroup

	for range 16 {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
					_ = c.secretLazyLoading.Load()
				}
			}
		})
	}
	for i := range 8 {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
					c.SetSecretLazyLoading(i%2 == 0)
				}
			}
		})
	}

	time.Sleep(20 * time.Millisecond)
	close(stop)
	wg.Wait()
}

// TestSetKubesharkNamespaceConcurrentWithReads is the kubesharkNamespaceOverride
// counterpart to the SecretLazyLoading race test.
func TestSetKubesharkNamespaceConcurrentWithReads(t *testing.T) {
	c := NewTestClient(nil, nil)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	values := []string{"kubeshark", "trafcap", "obs-net", ""}

	for range 16 {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
					_ = c.kubesharkNamespace()
				}
			}
		})
	}
	for i := range 8 {
		wg.Go(func() {
			j := 0
			for {
				select {
				case <-stop:
					return
				default:
					c.SetKubesharkNamespace(values[(i+j)%len(values)])
					j++
				}
			}
		})
	}

	time.Sleep(20 * time.Millisecond)
	close(stop)
	wg.Wait()

	// Sanity assertion to keep the test from being marked as no-op.
	assert.NotPanics(t, func() { _ = c.kubesharkNamespace() })
}
