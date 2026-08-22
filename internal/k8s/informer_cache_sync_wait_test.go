package k8s

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// An informer whose watch cannot finish its initial stream never closes
// `synced`. Waiting the full timeout on every call stalls one dashboard section
// per tick for as long as the session runs (issue #646).
func TestWaitForSyncGivesUpOncePerEntry(t *testing.T) {
	entry := &informerEntry{
		stopCh: make(chan struct{}),
		synced: make(chan struct{}),
	}

	start := time.Now()
	require.Error(t, waitForSync(t.Context(), entry, 50*time.Millisecond),
		"the first wait must report the timeout")
	require.GreaterOrEqual(t, time.Since(start), 50*time.Millisecond,
		"the first wait must actually wait")

	start = time.Now()
	require.Error(t, waitForSync(t.Context(), entry, time.Hour),
		"an entry that already timed out must not be waited on again")
	assert.Less(t, time.Since(start), 50*time.Millisecond,
		"the second wait must return without blocking")
}

// The informer keeps warming in the background, so a later sync must put the
// cache back in service.
func TestWaitForSyncRecoversAfterTheInformerSyncs(t *testing.T) {
	entry := &informerEntry{
		stopCh: make(chan struct{}),
		synced: make(chan struct{}),
	}
	require.Error(t, waitForSync(t.Context(), entry, time.Millisecond))

	close(entry.synced)

	assert.NoError(t, waitForSync(t.Context(), entry, time.Millisecond),
		"a synced informer must serve the cache again")
}

// denyGVR stops the watch for the session. Always-mode ignored that and sent
// every list back through the cache branch, where getOrStart restarted the
// informer and the caller paid the sync wait again.
func TestShouldUseCacheHonoursADeniedGVRInAlwaysMode(t *testing.T) {
	ic := newInformerCache(nil)
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "nodes"}
	ic.denyGVR("test-ctx", gvr)

	assert.False(t, shouldUseCache(InformerCacheAlways, ic, "test-ctx", gvr),
		"a denied GVR must take the direct path in every mode")
}
