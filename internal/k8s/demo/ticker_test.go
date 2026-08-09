package demo

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func crashLoopContainerStatus(t *testing.T, dyn *dynamicfake.FakeDynamicClient) map[string]any {
	t.Helper()
	obj, err := dyn.Resource(podGVR).Namespace(NamespaceDemo).Get(t.Context(), PodWebCrashLoop, metav1.GetOptions{})
	require.NoError(t, err)

	statuses, found, err := unstructured.NestedSlice(obj.Object, "status", "containerStatuses")
	require.NoError(t, err)
	require.True(t, found)
	require.Len(t, statuses, 1)

	cs, ok := statuses[0].(map[string]any)
	require.True(t, ok)
	return cs
}

func TestTicker_TickRaisesRestartCountByOnePerCall(t *testing.T) {
	dyn := NewDynamicClient()
	tk := NewTicker(dyn, time.Hour)
	ctx := t.Context()

	before, _, err := unstructured.NestedInt64(crashLoopContainerStatus(t, dyn), "restartCount")
	require.NoError(t, err)

	const n = 5
	for range n {
		require.NoError(t, tk.Tick(ctx))
	}

	after, _, err := unstructured.NestedInt64(crashLoopContainerStatus(t, dyn), "restartCount")
	require.NoError(t, err)
	assert.Equal(t, before+n, after, "expected restartCount to rise by exactly one per Tick call")
}

func TestTicker_TickRotatesWaitingReason(t *testing.T) {
	dyn := NewDynamicClient()
	tk := NewTicker(dyn, time.Hour)
	ctx := t.Context()

	seen := make([]string, 0, len(waitingReasons)*2)
	for range len(waitingReasons) * 2 {
		require.NoError(t, tk.Tick(ctx))
		reason, found, err := unstructured.NestedString(crashLoopContainerStatus(t, dyn), "state", "waiting", "reason")
		require.NoError(t, err)
		require.True(t, found)
		seen = append(seen, reason)
	}

	for i, reason := range seen {
		assert.Equal(t, waitingReasons[i%len(waitingReasons)], reason, "reason at tick %d", i)
	}
}

func TestTicker_EachTickAppendsWarningEventAndStaysUnderCap(t *testing.T) {
	dyn := NewDynamicClient()
	tk := NewTicker(dyn, time.Hour)
	ctx := t.Context()

	countEvents := func() int {
		list, err := dyn.Resource(eventGVR).Namespace(NamespaceDemo).List(ctx, metav1.ListOptions{})
		require.NoError(t, err)
		return len(list.Items)
	}

	before := countEvents()

	require.NoError(t, tk.Tick(ctx))
	afterOne := countEvents()
	assert.Equal(t, before+1, afterOne, "expected exactly one new event after one Tick")

	const manyTicks = 200
	for range manyTicks {
		require.NoError(t, tk.Tick(ctx))
	}

	afterMany := countEvents()
	assert.LessOrEqual(t, afterMany, before+maxTickerEvents,
		"event count must stay bounded even after many ticks")
	assert.Greater(t, afterMany, before, "ticks should still be producing events")
}

func TestTicker_StartStopExitsGoroutineCleanly(t *testing.T) {
	baseline := runtime.NumGoroutine()

	dyn := NewDynamicClient()
	tk := NewTicker(dyn, time.Millisecond)

	tk.Start(t.Context())
	assert.Greater(t, runtime.NumGoroutine(), baseline, "expected a new goroutine after Start")

	tk.Stop()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= baseline+1 { // +1 tolerance for runtime jitter
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("Stop did not drain the ticker goroutine: baseline=%d, after=%d", baseline, runtime.NumGoroutine())
}

func TestTicker_ContextCancelEndsGoroutine(t *testing.T) {
	baseline := runtime.NumGoroutine()

	dyn := NewDynamicClient()
	tk := NewTicker(dyn, time.Millisecond)

	ctx, cancel := context.WithCancel(t.Context())
	tk.Start(ctx)
	assert.Greater(t, runtime.NumGoroutine(), baseline, "expected a new goroutine after Start")

	cancel()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= baseline+1 {
			tk.Stop() // idempotent cleanup so the test itself leaves nothing running
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("context cancellation did not end the ticker goroutine: baseline=%d, after=%d", baseline, runtime.NumGoroutine())
}
