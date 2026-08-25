package k8s

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"
)

// A PreferCache call that finds no informer running reads nothing from the
// cache and lists directly. Starting the informer before that list puts its own
// initial LIST on the wire beside the list the user is waiting on. The cluster
// dashboard fans out eight such sections at once, so every first load turned
// eight lists into sixteen: on an EKS cluster the pod section went from about
// 1.2s to about 6s. The informer must start once our own list is done.
func TestGetResources_PreferCache_DefersInformerStartUntilTheListReturns(t *testing.T) {
	dc := newFakeDynClient(pod("api-1", "team-a"))
	c := NewTestClient(nil, dc)
	c.SetInformerCacheMode(InformerCacheAuto)
	t.Cleanup(c.Shutdown)
	withTunedThresholds(c, 1000, 500, 3)

	var once sync.Once
	var runningDuringList atomic.Bool
	dc.PrependReactor("list", "pods", func(clienttesting.Action) (bool, runtime.Object, error) {
		once.Do(func() { runningDuringList.Store(c.informers.informerRunning("", podGVR())) })
		return false, nil, nil // fall through to the tracker
	})

	items, err := c.GetResources(t.Context(), "", "team-a", podRT, PreferCache())
	require.NoError(t, err)
	require.Len(t, items, 1)

	assert.False(t, runningDuringList.Load(),
		"the informer must not be listing while the caller's own list is in flight")
	assert.True(t, c.informers.informerRunning("", podGVR()),
		"the informer must start once the list is done, so the next refresh reads the cache")
}
