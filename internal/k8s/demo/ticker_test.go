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

// gcd and lcm let tests derive an exact cycle length from the ticker's own
// per-field cycle constants, instead of hardcoding a value that would go
// stale silently if those constants change.
func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func lcm(a, b int) int {
	return a / gcd(a, b) * b
}

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

// podEffectivelyRunning reports whether a pod's Phase is Running and its
// Ready condition is True, mirroring the app's pod-health classification
// closely enough to catch the crashlooping pod (Phase Running, Ready False)
// and a Pending flip (Phase Pending) as not-running.
func podEffectivelyRunning(obj map[string]any) bool {
	phase, _, _ := unstructured.NestedString(obj, "status", "phase")
	if phase != "Running" {
		return false
	}
	conditions, _, _ := unstructured.NestedSlice(obj, "status", "conditions")
	for _, c := range conditions {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if cm["type"] == "Ready" {
			return cm["status"] == "True"
		}
	}
	return false
}

// TestTicker_FullCycleInvariants drives the ticker across a full cycle and
// asserts both demo-polish invariants: Running pods stay a strict majority at
// every tick, and the web Deployment reports fully ready for a majority of the
// cycle.
//
// Pods are counted in NamespaceDemo only. The ticker mutates nothing outside
// it, so the fixed Jobs-namespace pods would dilute a ratio they cannot move.
func TestTicker_FullCycleInvariants(t *testing.T) {
	dyn := NewDynamicClient()
	tk := NewTicker(dyn, time.Hour)
	ctx := t.Context()

	// cycleLen must stay the LCM of every per-field cycle length so the loop
	// below always covers a full cycle, even if those constants change.
	cycleLen := lcm(lcm(healthyPodFlipEvery, len(waitingReasons)), len(deploymentReadyReplicaCycle))

	deployClient := dyn.Resource(deploymentGVR).Namespace(NamespaceDemo)
	deployObj, err := deployClient.Get(ctx, DeploymentWeb, metav1.GetOptions{})
	require.NoError(t, err)
	desired, _, err := unstructured.NestedInt64(deployObj.Object, "spec", "replicas")
	require.NoError(t, err)
	require.Positive(t, desired)

	readyTicks := 0
	for i := range cycleLen {
		require.NoError(t, tk.Tick(ctx))

		podList, err := dyn.Resource(podGVR).Namespace(NamespaceDemo).List(ctx, metav1.ListOptions{})
		require.NoError(t, err)

		var running int
		for _, item := range podList.Items {
			if podEffectivelyRunning(item.Object) {
				running++
			}
		}
		total := len(podList.Items)
		assert.Greater(t, running*2, total,
			"tick %d: expected Running pods to be a strict majority (%d running of %d total)", i, running, total)

		deployObj, err := deployClient.Get(ctx, DeploymentWeb, metav1.GetOptions{})
		require.NoError(t, err)
		ready, _, err := unstructured.NestedInt64(deployObj.Object, "status", "readyReplicas")
		require.NoError(t, err)
		if ready >= desired {
			readyTicks++
		}
	}

	assert.Greater(t, readyTicks*2, cycleLen,
		"expected the web Deployment to be fully ready for a majority of the cycle (%d/%d ticks ready)", readyTicks, cycleLen)
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

// TestTicker_EventTimestampsAreDeterministic drives two independent Tickers
// through the same tick and asserts they produce identical Warning Event
// timestamps -- the ticker's doc comment promises every mutation is a
// deterministic function of the tick count, which time.Now() would break.
func TestTicker_EventTimestampsAreDeterministic(t *testing.T) {
	ctx := t.Context()

	tickEvent := func(dyn *dynamicfake.FakeDynamicClient) map[string]any {
		t.Helper()
		// restartCrashLoopPod's tick-0 event is always named "web-crash-restart-0".
		obj, err := dyn.Resource(eventGVR).Namespace(NamespaceDemo).Get(ctx, "web-crash-restart-0", metav1.GetOptions{})
		require.NoError(t, err)
		return obj.Object
	}

	dynA := NewDynamicClient()
	tkA := NewTicker(dynA, time.Hour)
	require.NoError(t, tkA.Tick(ctx))
	firstA, _, err := unstructured.NestedString(tickEvent(dynA), "firstTimestamp")
	require.NoError(t, err)

	time.Sleep(1100 * time.Millisecond) // cross a second boundary; metav1.Time truncates to seconds

	dynB := NewDynamicClient()
	tkB := NewTicker(dynB, time.Hour)
	require.NoError(t, tkB.Tick(ctx))
	firstB, _, err := unstructured.NestedString(tickEvent(dynB), "firstTimestamp")
	require.NoError(t, err)

	assert.Equal(t, firstA, firstB, "tick 0 on two fresh tickers must produce identical event timestamps")
}

// TestTicker_TickClearsActionLogAndStaysBounded drives many ticks and
// asserts the dynamic fake client's Actions() log stays bounded instead of
// growing linearly with tick count. Each tick performs roughly eight
// Get/Update/Create/Delete calls against the fake, all recorded into
// client-go's testing.Fake (embedded in FakeDynamicClient) with nothing to
// clear it; left unbounded, a long demo session grows into the multi-GB
// range (see TASK-865 finding 1).
func TestTicker_TickClearsActionLogAndStaysBounded(t *testing.T) {
	dyn := NewDynamicClient()
	tk := NewTicker(dyn, time.Hour)
	ctx := t.Context()

	const manyTicks = 500
	for range manyTicks {
		require.NoError(t, tk.Tick(ctx))
	}

	actions := dyn.Actions()
	assert.Less(t, len(actions), 20,
		"expected the fake dynamic client's action log to be cleared each tick, not grow with tick count (%d actions after %d ticks)",
		len(actions), manyTicks)
}

// TestTicker_TickClearsExtraClientActionLogs verifies NewTicker's extra
// clearers (e.g. the typed demo clientset, which shares the same
// unbounded-Actions() problem since the app's own List/Get calls run
// against it too) get their action log cleared on the same cadence as the
// dynamic client, independent of whether the ticker itself touched them.
func TestTicker_TickClearsExtraClientActionLogs(t *testing.T) {
	dyn := NewDynamicClient()
	cs := NewClientset()
	tk := NewTicker(dyn, time.Hour, cs)
	ctx := t.Context()

	_, err := cs.CoreV1().Pods(NamespaceDemo).List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, cs.Actions(), "test setup: expected the List call to be recorded")

	require.NoError(t, tk.Tick(ctx))

	assert.Empty(t, cs.Actions(), "expected the extra clearer's action log to be cleared after a tick")
}

func TestTicker_StartStopExitsGoroutineCleanly(t *testing.T) {
	baseline := runtime.NumGoroutine()

	dyn := NewDynamicClient()
	tk := NewTicker(dyn, time.Millisecond)

	tk.Start(t.Context())
	// Check running, not a NumGoroutine snapshot: a goroutine from an
	// earlier test exiting between the baseline and the check masks the
	// new one and flakes on slow CI runners.
	tk.mu.Lock()
	started := tk.running
	tk.mu.Unlock()
	require.True(t, started, "expected Start to launch the ticker")

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

// TestTicker_ExternalContextCancelResetsRunningState guards TASK-865
// finding 6: if the context passed to Start is cancelled by anything other
// than Stop, the goroutine exits but running stayed true forever (only
// Stop ever cleared it), so a later Start call would silently no-op instead
// of actually restarting the ticker.
func TestTicker_ExternalContextCancelResetsRunningState(t *testing.T) {
	dyn := NewDynamicClient()
	tk := NewTicker(dyn, time.Millisecond)

	ctx, cancel := context.WithCancel(t.Context())
	tk.Start(ctx)
	cancel()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		tk.mu.Lock()
		running := tk.running
		tk.mu.Unlock()
		if !running {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	tk.mu.Lock()
	running := tk.running
	tk.mu.Unlock()
	require.False(t, running, "expected running to reset to false after external context cancellation, not just after Stop")

	// running was just proven false, so Start's no-op guard cannot fire:
	// running flipping back to true means the relaunch path ran.
	tk.Start(t.Context())
	defer tk.Stop()
	tk.mu.Lock()
	relaunched := tk.running
	tk.mu.Unlock()
	assert.True(t, relaunched,
		"expected Start to relaunch the ticker after external cancellation, not silently no-op")
}

func TestTicker_ContextCancelEndsGoroutine(t *testing.T) {
	baseline := runtime.NumGoroutine()

	dyn := NewDynamicClient()
	tk := NewTicker(dyn, time.Millisecond)

	ctx, cancel := context.WithCancel(t.Context())
	tk.Start(ctx)
	tk.mu.Lock()
	started := tk.running
	tk.mu.Unlock()
	require.True(t, started, "expected Start to launch the ticker")

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
