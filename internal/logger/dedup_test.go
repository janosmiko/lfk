package logger

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestShouldEmit_FirstCallEmits(t *testing.T) {
	ResetDedupForTest()
	emit, supp := ShouldEmit("tag", "ctx")
	assert.True(t, emit)
	assert.Equal(t, 0, supp)
}

// Regression guard for the EKS metrics-server case (node-metrics-load
// failing every 2s): repeats inside the window are silently suppressed
// so neither the on-disk log nor the in-app overlay drown in identical
// lines.
func TestShouldEmit_SuppressesInsideWindow(t *testing.T) {
	ResetDedupForTest()
	SetDedupWindowForTest(5 * time.Minute)
	now := time.Now()
	SetDedupClockForTest(func() time.Time { return now })
	defer SetDedupClockForTest(time.Now)

	emit, _ := ShouldEmit("tag", "ctx")
	assert.True(t, emit, "first call should emit")

	for range 10 {
		emit, _ = ShouldEmit("tag", "ctx")
		assert.False(t, emit, "repeats inside window must be suppressed")
	}
}

// After the window expires, the next call emits and reports the count
// of events suppressed during that window — the rate is preserved even
// though the lines were dropped.
func TestShouldEmit_ReEmitsAfterWindowWithSuppressedCount(t *testing.T) {
	ResetDedupForTest()
	SetDedupWindowForTest(5 * time.Minute)
	current := time.Now()
	SetDedupClockForTest(func() time.Time { return current })
	defer SetDedupClockForTest(time.Now)

	ShouldEmit("tag", "ctx")
	for range 7 {
		ShouldEmit("tag", "ctx")
	}
	current = current.Add(6 * time.Minute)

	emit, supp := ShouldEmit("tag", "ctx")
	assert.True(t, emit)
	assert.Equal(t, 7, supp, "must report the 7 suppressed events")
}

// Different (tag, contextKey) pairs dedup independently. One cluster's
// outage must not mask another cluster's identical-tag failure.
func TestShouldEmit_DifferentKeysIndependent(t *testing.T) {
	ResetDedupForTest()

	emitA, _ := ShouldEmit("metrics", "cluster-a")
	emitB, _ := ShouldEmit("metrics", "cluster-b")
	assert.True(t, emitA)
	assert.True(t, emitB, "cluster-b must emit even though cluster-a just did")

	emitA2, _ := ShouldEmit("metrics", "cluster-a")
	assert.False(t, emitA2, "cluster-a repeat is suppressed")
}

func TestShouldEmit_ConcurrentSafe(t *testing.T) {
	ResetDedupForTest()
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			ShouldEmit("tag", "ctx")
		})
	}
	wg.Wait()
	// No assertion needed: -race catches any concurrent map write.
}

// WarnOnce/ErrorOnce publish to UIChan on emit only; suppressed
// repeats must not push anything to the UI overlay either.
func TestWarnOnce_PublishesOnEmitNotOnSuppress(t *testing.T) {
	ResetDedupForTest()
	drainUIChan()

	WarnOnce("tag", "ctx", "boom", "k", "v")
	select {
	case e := <-UIChan():
		assert.Equal(t, "WRN", e.Level)
		assert.Equal(t, "boom", e.Message)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected UIChan entry on first emit")
	}

	WarnOnce("tag", "ctx", "boom", "k", "v")
	select {
	case e := <-UIChan():
		t.Fatalf("did not expect a UIChan entry for suppressed repeat: %+v", e)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestErrorOnce_PublishesAtErrorLevel(t *testing.T) {
	ResetDedupForTest()
	drainUIChan()

	ErrorOnce("err-tag", "ctx", "broken")
	select {
	case e := <-UIChan():
		assert.Equal(t, "ERR", e.Level)
		assert.Equal(t, "broken", e.Message)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected UIChan entry on first emit")
	}
}

// dedupState is pruned periodically (every dedupPruneEvery calls).
// Without this, high-cardinality keys (per-pod context keys across
// many namespaces, varying stderr lines from a flapping plugin) would
// accumulate indefinitely in a long-running session.
func TestShouldEmit_EvictsStaleEntries(t *testing.T) {
	ResetDedupForTest()
	SetDedupWindowForTest(5 * time.Minute)
	SetDedupPruneEveryForTest(4)
	defer SetDedupPruneEveryForTest(1024)
	current := time.Now()
	SetDedupClockForTest(func() time.Time { return current })
	defer SetDedupClockForTest(time.Now)

	ShouldEmit("tag", "stale-a")
	ShouldEmit("tag", "stale-b")

	current = current.Add(15 * time.Minute) // past 2x window

	// Two more calls to hit the prune threshold (dedupOps reaches 4).
	ShouldEmit("tag", "fresh-c")
	ShouldEmit("tag", "fresh-d")

	dedupMu.Lock()
	_, staleAStill := dedupState["tag\x00stale-a"]
	_, staleBStill := dedupState["tag\x00stale-b"]
	_, freshCStill := dedupState["tag\x00fresh-c"]
	dedupMu.Unlock()

	assert.False(t, staleAStill, "stale entry must be evicted past 2x window")
	assert.False(t, staleBStill, "stale entry must be evicted past 2x window")
	assert.True(t, freshCStill, "fresh entry must survive prune")
}

// A pruned entry's re-occurrence emits as a fresh first-occurrence —
// not as a suppressed-count emission — since prune already cleared its
// state. Guards against losing the suppressed-count side channel for
// frequently-recurring keys (those have suppressed>0 and are not
// evicted).
func TestShouldEmit_PrunedEntryEmitsFresh(t *testing.T) {
	ResetDedupForTest()
	SetDedupWindowForTest(1 * time.Minute)
	SetDedupPruneEveryForTest(2)
	defer SetDedupPruneEveryForTest(1024)
	current := time.Now()
	SetDedupClockForTest(func() time.Time { return current })
	defer SetDedupClockForTest(time.Now)

	ShouldEmit("tag", "abandoned")
	current = current.Add(3 * time.Minute)
	ShouldEmit("tag", "trigger-prune") // bumps dedupOps to 2 -> prune

	current = current.Add(1 * time.Second)
	emit, supp := ShouldEmit("tag", "abandoned")
	assert.True(t, emit)
	assert.Equal(t, 0, supp, "pruned entry should look like a first emission")
}

func drainUIChan() {
	for {
		select {
		case <-UIChan():
		default:
			return
		}
	}
}
