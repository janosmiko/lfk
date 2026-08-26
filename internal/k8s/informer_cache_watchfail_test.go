package k8s

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/watch"
	clienttesting "k8s.io/client-go/testing"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// A transport error, not an API status error, so denyGVR's status checks
// never match it (issue #694).
var errReset = errors.New("read tcp 10.0.0.1:52000->10.0.0.2:443: read: connection reset by peer")

// Issue #694: every request to one apiserver shares a single HTTP/2
// connection, so a watch retried forever takes the plain lists down with it.
func TestInformerCache_WatchNetworkErrorsDemote(t *testing.T) {
	logger.ResetDedupForTest()
	drainUIChan()

	dc := newFakeDynClient(pod("api-1", "team-a"))
	dc.PrependWatchReactor("pods", func(clienttesting.Action) (bool, watch.Interface, error) {
		return true, nil, errReset
	})
	c := NewTestClient(nil, dc)
	c.SetInformerCacheMode(InformerCacheAlways)
	t.Cleanup(c.Shutdown)
	// One failure, because the reflector backs off to roughly 0.22 QPS and
	// three real retries would park the suite for ~15s. The threshold
	// itself is covered by TestInformerCache_WatchSuccessResetsFailureCount.
	withWatchFailureTuning(c, 1, time.Hour)
	gvr := podGVR()

	deadline := time.Now().Add(10 * time.Second)
	var items []model.Item
	var err error
	for time.Now().Before(deadline) {
		items, err = c.GetResources(t.Context(), "", "team-a", podRT)
		require.NoError(t, err)
		if c.informers.cacheBlocked("", gvr) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	require.True(t, c.informers.cacheBlocked("", gvr),
		"a watch failing repeatedly on the network must stop being retried")
	assert.False(t, c.informers.isPromoted("", gvr))
	assert.False(t, c.informers.getAutoState("", gvr).isDenied(),
		"a network error is transient, so it must not deny the GVR for the whole session")
	assert.False(t, c.informers.hasEntry("", gvr), "the informer must be torn down")

	require.Len(t, items, 1, "the direct list must keep working while the watch is down")
	assert.Equal(t, "api-1", items[0].Name)
}

// TestInformerCache_WatchFailureCooldownExpires covers the other half: the
// block is a cooldown, not a life sentence. Once it lapses the GVR may open
// a watch again, so a network blip does not cost the cache for the session.
func TestInformerCache_WatchFailureCooldownExpires(t *testing.T) {
	c := NewTestClient(nil, newFakeDynClient(pod("api-1", "team-a")))
	c.SetInformerCacheMode(InformerCacheAuto)
	t.Cleanup(c.Shutdown)
	withWatchFailureTuning(c, 1, 20*time.Millisecond)
	gvr := podGVR()

	c.informers.noteWatchFailure("", gvr, errReset)
	require.True(t, c.informers.cacheBlocked("", gvr))

	time.Sleep(40 * time.Millisecond)
	assert.False(t, c.informers.cacheBlocked("", gvr), "the cooldown must lapse")
}

// TestInformerCache_WatchSuccessResetsFailureCount guards the "consecutive"
// in the threshold. A watch that recovers between failures must not
// accumulate them across the recovery, or a long session on a slightly
// lossy link would eventually demote every GVR.
func TestInformerCache_WatchSuccessResetsFailureCount(t *testing.T) {
	c := NewTestClient(nil, newFakeDynClient(pod("api-1", "team-a")))
	c.SetInformerCacheMode(InformerCacheAuto)
	t.Cleanup(c.Shutdown)
	withWatchFailureTuning(c, 3, time.Hour)
	gvr := podGVR()

	c.informers.noteWatchFailure("", gvr, errReset)
	c.informers.noteWatchFailure("", gvr, errReset)
	c.informers.noteWatchSuccess("", gvr)
	c.informers.noteWatchFailure("", gvr, errReset)
	c.informers.noteWatchFailure("", gvr, errReset)

	assert.False(t, c.informers.cacheBlocked("", gvr),
		"two failures after a recovery must not trip a threshold of three")

	c.informers.noteWatchFailure("", gvr, errReset)
	assert.True(t, c.informers.cacheBlocked("", gvr))
}

// TestInformerCache_WatchFailureLogsOnce keeps the reflector's retry storm
// out of the user's log. The give-up is worth one line, not one per retry.
func TestInformerCache_WatchFailureLogsOnce(t *testing.T) {
	logger.ResetDedupForTest()
	drainUIChan()

	c := NewTestClient(nil, newFakeDynClient(pod("api-1", "team-a")))
	c.SetInformerCacheMode(InformerCacheAuto)
	t.Cleanup(c.Shutdown)
	withWatchFailureTuning(c, 1, time.Hour)
	gvr := podGVR()

	for range 5 {
		c.informers.noteWatchFailure("", gvr, errReset)
	}

	warnings := 0
	for {
		select {
		case e := <-logger.UIChan():
			if e.Level == "WRN" {
				warnings++
			}
		case <-time.After(200 * time.Millisecond):
			assert.Equal(t, 1, warnings, "the give-up should surface exactly one warning")
			return
		}
	}
}
