package app

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Per-pod buffer cache tests (TDD: written before implementation) ---

// TestPreviewCacheRestoreOnReturn asserts that navigating from pod A to pod B
// (via enterPreviewLogPod) caches A's buffer, and returning to A restores it
// instantly instead of re-fetching.
func TestPreviewCacheRestoreOnReturn(t *testing.T) {
	m := baseModelWithFakeClient()
	m.previewLog.readerInFlight = &atomic.Bool{}
	m.previewLogCache = make(map[string]previewLogCacheEntry)
	m.fullLogPreview = true

	// Enter pod A (fresh start — no cache).
	m = withSelectedPod(t, m, "ns", "pod-a")
	m, _ = m.enterPreviewLogPod(podRef{context: "test-ctx", namespace: "ns", name: "pod-a"})

	// Simulate some lines arriving.
	m.previewLog.lines = []string{"line-1", "line-2", "line-3"}

	// Navigate to pod B — A's buffer should be cached.
	m = withSelectedPod(t, m, "ns", "pod-b")
	m, _ = m.enterPreviewLogPod(podRef{context: "test-ctx", namespace: "ns", name: "pod-b"})

	keyA := "test-ctx/ns/pod-a"
	assert.Contains(t, m.previewLogCache, keyA, "pod-a buffer must be cached on leave")
	assert.Equal(t, []string{"line-1", "line-2", "line-3"}, m.previewLogCache[keyA].lines)

	// Return to pod A — buffer must be restored.
	m = withSelectedPod(t, m, "ns", "pod-a")
	m, _ = m.enterPreviewLogPod(podRef{context: "test-ctx", namespace: "ns", name: "pod-a"})

	assert.Equal(t, "test-ctx/ns/pod-a", m.previewLog.podKey, "podKey must be pod-a after restore")
	assert.Equal(t, []string{"line-1", "line-2", "line-3"}, m.previewLog.lines,
		"lines must be restored from cache on return to pod-a")
}

// TestPreviewCacheToggleOffOnSamePod asserts that disable (cache+cancel) then
// re-enable on the same pod restores the buffer without a fresh fetch.
func TestPreviewCacheToggleOffOnSamePod(t *testing.T) {
	m := baseModelWithFakeClient()
	m.previewLog.readerInFlight = &atomic.Bool{}
	m.previewLogCache = make(map[string]previewLogCacheEntry)
	m.fullLogPreview = true
	m = withSelectedPod(t, m, "ns", "pod-a")

	// Enter pod A.
	m, _ = m.enterPreviewLogPod(podRef{context: "test-ctx", namespace: "ns", name: "pod-a"})
	m.previewLog.lines = []string{"a", "b", "c"}

	// Toggle off: cache then cancel.
	m.cachePreviewLog()
	m.cancelPreviewLogStream()

	assert.Empty(t, m.previewLog.lines, "cancel must clear lines")
	assert.Contains(t, m.previewLogCache, "test-ctx/ns/pod-a", "buffer must be cached on disable")

	// Toggle on again: restore from cache.
	m, _ = m.enterPreviewLogPod(podRef{context: "test-ctx", namespace: "ns", name: "pod-a"})
	assert.Equal(t, []string{"a", "b", "c"}, m.previewLog.lines,
		"re-enabling must restore from cache")
	assert.Equal(t, "test-ctx/ns/pod-a", m.previewLog.podKey)
}

// TestPreviewCacheLRUEvicts asserts that visiting previewLogCacheMax+2 distinct
// pods keeps the cache size at or below previewLogCacheMax.
func TestPreviewCacheLRUEvicts(t *testing.T) {
	m := baseModelWithFakeClient()
	m.previewLog.readerInFlight = &atomic.Bool{}
	m.previewLogCache = make(map[string]previewLogCacheEntry)
	m.fullLogPreview = true

	// Visit previewLogCacheMax+2 distinct pods.
	for i := range previewLogCacheMax + 2 {
		podName := fmt.Sprintf("pod-%d", i)
		m = withSelectedPod(t, m, "ns", podName)
		// Set current pod lines before switching away so cachePreviewLog has content.
		m.previewLog.podKey = "test-ctx/ns/" + podName
		m.previewLog.lines = []string{"line-from-" + podName}
		m, _ = m.enterPreviewLogPod(podRef{context: "test-ctx", namespace: "ns", name: fmt.Sprintf("pod-%d", i+1)})
	}

	assert.LessOrEqual(t, len(m.previewLogCache), previewLogCacheMax,
		"cache must not exceed previewLogCacheMax entries")
}

// TestPreviewCacheClearedOnContextSwitch asserts that navigateChildCluster clears
// the whole previewLogCache (different cluster — stale cross-context entries).
func TestPreviewCacheClearedOnContextSwitch(t *testing.T) {
	m := baseModelWithFakeClient()
	m.previewLogCache = map[string]previewLogCacheEntry{
		"ctx1/ns/pod-a": {lines: []string{"old"}},
		"ctx1/ns/pod-b": {lines: []string{"old2"}},
	}
	m.previewLogCacheOrder = []string{"ctx1/ns/pod-a", "ctx1/ns/pod-b"}

	sel := &model.Item{Name: "other-ctx"}
	mm, _ := m.navigateChildCluster(sel)
	rm := mm.(Model)

	assert.Empty(t, rm.previewLogCache, "context switch must clear the preview log cache")
	assert.Empty(t, rm.previewLogCacheOrder, "context switch must clear the cache order slice")
}

// TestPreviewCacheNoAlias guards against the cached slice being aliased to the
// live buffer — mutations to the live buffer must not corrupt the cache.
func TestPreviewCacheNoAlias(t *testing.T) {
	m := baseModelWithFakeClient()
	m.previewLog.readerInFlight = &atomic.Bool{}
	m.previewLogCache = make(map[string]previewLogCacheEntry)
	m.previewLog.podKey = "test-ctx/ns/pod-a"
	m.previewLog.lines = []string{"original"}

	m.cachePreviewLog()

	// Mutate the live buffer.
	m.previewLog.lines[0] = "mutated"

	// Cache must still hold the original value.
	assert.Equal(t, "original", m.previewLogCache["test-ctx/ns/pod-a"].lines[0],
		"cached slice must be a copy, not an alias of the live buffer")
}

// TestPreviewViewportHeightClamped verifies previewLogViewportHeight always
// returns a value in [10, previewLogCap].
func TestPreviewViewportHeightClamped(t *testing.T) {
	m := baseModelCov()
	m.width = 200
	m.height = 30
	h := m.previewLogViewportHeight()
	assert.GreaterOrEqual(t, h, 10, "viewport height must be at least 10")
	assert.LessOrEqual(t, h, previewLogCap, "viewport height must not exceed previewLogCap")
}

// TestPreviewViewportHeightVerySmallTerminal verifies clamping on a tiny
// terminal where the computed height would be below the minimum.
func TestPreviewViewportHeightVerySmallTerminal(t *testing.T) {
	m := baseModelCov()
	m.width = 20
	m.height = 5
	h := m.previewLogViewportHeight()
	assert.GreaterOrEqual(t, h, 10, "viewport height must still be at least 10 on tiny terminals")
}

// TestAppendPreviewLogLineBounds verifies that the live-stream ring caps at
// previewLogMaxLines (not the smaller previewLogCap, which is only the initial
// --tail size for a fresh stream).
func TestAppendPreviewLogLineBounds(t *testing.T) {
	m := baseModelCov()
	// Append more than previewLogMaxLines to exercise the cap.
	for range previewLogMaxLines*2 + 10 {
		m.appendPreviewLogLine("line")
	}
	assert.LessOrEqual(t, len(m.previewLog.lines), previewLogMaxLines,
		"preview log buffer must cap at previewLogMaxLines")
	assert.NotEmpty(t, m.previewLog.lines)
}

// --- Lazy older-history loading tests ---

// TestAppendPreviewLogGrowsToMaxLines verifies that appendPreviewLogLine
// allows the buffer to grow past previewLogCap (for lazily-loaded history)
// up to previewLogMaxLines, and caps there.
func TestAppendPreviewLogGrowsToMaxLines(t *testing.T) {
	m := baseModelCov()

	// Append more than previewLogCap but fewer than previewLogMaxLines.
	// The buffer must keep all of them (not capped at previewLogCap).
	target := previewLogCap + 50
	for i := range target {
		m.appendPreviewLogLine(fmt.Sprintf("line-%d", i))
	}
	assert.Equal(t, target, len(m.previewLog.lines),
		"buffer must keep all lines when under previewLogMaxLines")

	// Now append until we exceed previewLogMaxLines — it must cap there.
	for i := range previewLogMaxLines*2 - target {
		m.appendPreviewLogLine(fmt.Sprintf("overflow-%d", i))
	}
	assert.Equal(t, previewLogMaxLines, len(m.previewLog.lines),
		"buffer must cap at previewLogMaxLines")
}

// TestUpdatePreviewLogHistoryPrependsOlder verifies the happy-path merge:
// fetched lines contain an overlap with the current buffer; only the genuinely
// older delta is prepended.
func TestUpdatePreviewLogHistoryPrependsOlder(t *testing.T) {
	m := baseModelWithFakeClient()
	m.fullLogPreview = true
	m.previewLog.podKey = "ctx/ns/pod-1"
	m.previewLog.lines = []string{"c", "d"}
	m.previewLog.loadingHistory = true
	m.previewLog.hasMoreHistory = true
	m.previewLog.historyTail = previewLogViewportHeightDefault

	msg := previewLogHistoryMsg{
		podKey: "ctx/ns/pod-1",
		lines:  []string{"a", "b", "c", "d"},
	}
	mm := m.updatePreviewLogHistory(msg)
	rm := mm.(Model)

	assert.Equal(t, []string{"a", "b", "c", "d"}, rm.previewLog.lines,
		"older lines a,b must be prepended before the current buffer")
	assert.False(t, rm.previewLog.loadingHistory, "loadingHistory must be cleared")
	assert.Equal(t, previewLogViewportHeightDefault+previewLogHistoryBatch, rm.previewLog.historyTail,
		"historyTail must advance by one batch")
}

// TestUpdatePreviewLogHistoryNoNewStopsLoading verifies that when the fetched
// lines fully overlap the current buffer (no older lines), hasMoreHistory is
// set to false.
func TestUpdatePreviewLogHistoryNoNewStopsLoading(t *testing.T) {
	m := baseModelWithFakeClient()
	m.fullLogPreview = true
	m.previewLog.podKey = "ctx/ns/pod-1"
	m.previewLog.lines = []string{"a", "b", "c", "d"}
	m.previewLog.loadingHistory = true
	m.previewLog.hasMoreHistory = true

	// Fetched set is identical to current — no new older content.
	msg := previewLogHistoryMsg{
		podKey: "ctx/ns/pod-1",
		lines:  []string{"a", "b", "c", "d"},
	}
	mm := m.updatePreviewLogHistory(msg)
	rm := mm.(Model)

	assert.False(t, rm.previewLog.hasMoreHistory, "no new lines must stop lazy loading")
	assert.False(t, rm.previewLog.loadingHistory)
}

// TestUpdatePreviewLogHistoryStaleDropped verifies that a msg whose podKey
// doesn't match the current streaming pod is silently ignored.
func TestUpdatePreviewLogHistoryStaleDropped(t *testing.T) {
	m := baseModelWithFakeClient()
	m.fullLogPreview = true
	m.previewLog.podKey = "ctx/ns/pod-1"
	m.previewLog.lines = []string{"x"}
	m.previewLog.loadingHistory = true

	msg := previewLogHistoryMsg{
		podKey: "ctx/ns/pod-OTHER",
		lines:  []string{"a", "b", "x"},
	}
	mm := m.updatePreviewLogHistory(msg)
	rm := mm.(Model)

	// Model state must be completely unchanged (lines intact).
	assert.Equal(t, []string{"x"}, rm.previewLog.lines, "stale msg must leave lines unchanged")
	assert.True(t, rm.previewLog.loadingHistory, "stale msg must leave loadingHistory unchanged")
}

// TestPreviewCacheRestoresHistoryState verifies that cachePreviewLog stores
// hasMoreHistory and historyTail, and restorePreviewLogFromCache restores them.
func TestPreviewCacheRestoresHistoryState(t *testing.T) {
	m := baseModelWithFakeClient()
	m.previewLog.readerInFlight = &atomic.Bool{}
	m.previewLogCache = make(map[string]previewLogCacheEntry)
	m.previewLog.podKey = "ctx/ns/pod-a"
	m.previewLog.lines = []string{"a", "b", "c"}
	m.previewLog.hasMoreHistory = true
	m.previewLog.historyTail = 250

	m.cachePreviewLog()

	// Wipe state to verify restoration.
	m.previewLog.hasMoreHistory = false
	m.previewLog.historyTail = 0
	m.previewLog.lines = nil

	ok := m.restorePreviewLogFromCache("ctx/ns/pod-a")
	require.True(t, ok)
	assert.True(t, m.previewLog.hasMoreHistory, "hasMoreHistory must be restored from cache")
	assert.Equal(t, 250, m.previewLog.historyTail, "historyTail must be restored from cache")
}

// TestMaybeLoadMorePreviewHistoryTriggersAtTop verifies that
// maybeLoadMorePreviewHistory returns a non-nil cmd and sets loadingHistory
// when the user is scrolled to the top (fromBottom at max) and hasMoreHistory.
func TestMaybeLoadMorePreviewHistoryTriggersAtTop(t *testing.T) {
	m := baseModelWithFakeClient()
	m.fullLogPreview = true
	m.width = 200
	m.height = 30
	// Populate more lines than fit in the viewport so maxFromBottom > 0.
	for i := range 80 {
		m.previewLog.lines = append(m.previewLog.lines, fmt.Sprintf("line-%d", i))
	}
	m.previewLog.podKey = "ctx/ns/pod-1"
	m.previewLog.hasMoreHistory = true
	m.previewLog.loadingHistory = false
	m.previewLog.historyTail = 30
	m = withSelectedPod(t, m, "ns", "pod-1")
	m.nav.Context = "ctx"

	// Scroll to the very top.
	m.clampPreviewScroll()
	usable := m.width - 6
	rightW := max(10, usable-max(10, usable*12/100)-max(10, usable*51/100))
	innerW := rightW - 2
	colHeight := max(m.height-4, 3)
	bodyHeight := max(colHeight-1, 1)
	total := ui.PreviewLogPhysicalCount(m.previewLog.lines, innerW)
	maxFromBottom := max(total-bodyHeight, 0)
	m.previewLog.fromBottom = maxFromBottom

	mm, cmd := m.maybeLoadMorePreviewHistory()
	assert.NotNil(t, cmd, "must return a fetch cmd when scrolled to the top with hasMoreHistory")
	assert.True(t, mm.previewLog.loadingHistory, "loadingHistory must be set to true when fetch is triggered")
}

// TestMaybeLoadMorePreviewHistoryNoTriggerWhenNotAtTop verifies no fetch is
// triggered when the user is not scrolled to the top.
func TestMaybeLoadMorePreviewHistoryNoTriggerWhenNotAtTop(t *testing.T) {
	m := baseModelWithFakeClient()
	m.fullLogPreview = true
	m.width = 200
	m.height = 30
	for i := range 80 {
		m.previewLog.lines = append(m.previewLog.lines, fmt.Sprintf("line-%d", i))
	}
	m.previewLog.podKey = "ctx/ns/pod-1"
	m.previewLog.hasMoreHistory = true
	m.previewLog.loadingHistory = false
	m.previewLog.historyTail = 30
	m = withSelectedPod(t, m, "ns", "pod-1")
	m.nav.Context = "ctx"

	// Scroll only partway up (not at the top).
	m.previewLog.fromBottom = 3

	_, cmd := m.maybeLoadMorePreviewHistory()
	assert.Nil(t, cmd, "must not trigger when not scrolled to the top")
}

// withSelectedPod sets the middle pane to a single Pod item and places the
// cursor on it, mirroring what real navigation does for a resource list of pods.
func withSelectedPod(_ *testing.T, m Model, ns, name string) Model { //nolint:unparam
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	m.middleItems = []model.Item{{Name: name, Namespace: ns, Kind: "Pod"}}
	m.setCursor(0)
	return m
}

// withSelectedResource sets the middle pane to a single non-pod resource item.
func withSelectedResource(_ *testing.T, m Model, kind, ns, name string) Model {
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: kind, Resource: "services", Namespaced: true}
	m.middleItems = []model.Item{{Name: name, Namespace: ns, Kind: kind}}
	m.setCursor(0)
	return m
}

// withSelectedContainer positions the model at LevelContainers inside pod
// (ns/podName) with the cursor on a single Container row, mirroring what
// navigateChildResource does when drilling into a Pod.
func withSelectedContainer(_ *testing.T, m Model, ns, podName, container string) Model {
	m.nav.Level = model.LevelContainers
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true}
	m.nav.Namespace = ns
	m.nav.OwnedName = podName
	m.middleItems = []model.Item{{Name: container, Kind: "Container", Status: "Running"}}
	m.setCursor(0)
	return m
}

func TestUpdatePreviewLogLineAppendsForCurrentStream(t *testing.T) {
	m := baseModelWithFakeClient()
	m.previewLog.readerInFlight = &atomic.Bool{}
	ch := make(chan string, 1)
	m.previewLog.ch = ch
	mm, cmd := m.updatePreviewLogLine(previewLogLineMsg{line: "hello", ch: ch})
	m = mm.(Model)
	assert.Equal(t, []string{"hello"}, m.previewLog.lines)
	assert.NotNil(t, cmd, "must re-arm the reader")
}

// TestUpdatePreviewLogLineSticksWhenScrolledBack guards the scroll-anchor bug:
// while the user is scrolled back (fromBottom > 0) a newly arrived line must
// advance fromBottom by its physical height so the same content stays in view,
// instead of the bottom-anchored window drifting toward the newest line.
func TestUpdatePreviewLogLineSticksWhenScrolledBack(t *testing.T) {
	m := baseModelWithFakeClient()
	m.fullLogPreview = true
	m.width = 200
	m.height = 30
	m.previewLog.readerInFlight = &atomic.Bool{}
	ch := make(chan string, 1)
	m.previewLog.ch = ch
	for i := range 80 {
		m.previewLog.lines = append(m.previewLog.lines, fmt.Sprintf("line-%d", i))
	}
	m.previewLog.fromBottom = 10

	mm, _ := m.updatePreviewLogLine(previewLogLineMsg{line: "fresh-line", ch: ch})
	m = mm.(Model)

	// One short line wraps to exactly one physical row, so fromBottom advances by 1.
	delta := ui.PreviewLogPhysicalCount([]string{"fresh-line"}, m.previewLogInnerWidth())
	assert.Equal(t, 1, delta, "sanity: a short line is one physical row")
	assert.Equal(t, 10+delta, m.previewLog.fromBottom,
		"scrolled-back view must stick when new lines arrive")
}

// TestUpdatePreviewLogLineFollowsAtBottom verifies that at the bottom
// (fromBottom == 0) the preview keeps auto-following the newest line.
func TestUpdatePreviewLogLineFollowsAtBottom(t *testing.T) {
	m := baseModelWithFakeClient()
	m.fullLogPreview = true
	m.width = 200
	m.height = 30
	m.previewLog.readerInFlight = &atomic.Bool{}
	ch := make(chan string, 1)
	m.previewLog.ch = ch
	m.previewLog.lines = []string{"a", "b"}
	m.previewLog.fromBottom = 0

	mm, _ := m.updatePreviewLogLine(previewLogLineMsg{line: "c", ch: ch})
	m = mm.(Model)
	assert.Equal(t, 0, m.previewLog.fromBottom, "at the bottom the view keeps following newest")
}

func TestUpdatePreviewLogLineDropsStaleStream(t *testing.T) {
	m := baseModelWithFakeClient()
	m.previewLog.ch = make(chan string)
	other := make(chan string)
	mm, _ := m.updatePreviewLogLine(previewLogLineMsg{line: "x", ch: other})
	m = mm.(Model)
	assert.Empty(t, m.previewLog.lines, "lines from a superseded stream are ignored")
}

func TestStartPreviewLogStreamReplacesPrevious(t *testing.T) {
	m := baseModelWithFakeClient()
	m.previewLog.readerInFlight = &atomic.Bool{}

	cancelled := false
	m.previewLog.cancel = func() { cancelled = true }
	m.previewLog.podKey = "ctx/ns/old"

	m, _ = m.startPreviewLogStream(podRef{context: "ctx", namespace: "ns", name: "new"}, false)
	assert.True(t, cancelled, "switching pods must cancel the previous stream")
	assert.Equal(t, "ctx/ns/new", m.previewLog.podKey)
}

// TestPreviewReconnectPreservesBufferAndCounter asserts that startPreviewLogStream
// with reconnect=true stops the old kubectl but preserves lines,
// autoReconnectAttempt, and fromBottom — no buffer clear, no flicker.
func TestPreviewReconnectPreservesBufferAndCounter(t *testing.T) {
	m := baseModelWithFakeClient()
	m.previewLog.readerInFlight = &atomic.Bool{}

	cancelled := false
	m.previewLog.cancel = func() { cancelled = true }
	m.previewLog.lines = []string{"a", "b"}
	m.previewLog.autoReconnectAttempt = 3
	m.previewLog.fromBottom = 2
	m.previewLog.podKey = "ctx/ns/pod-1"

	m, _ = m.startPreviewLogStream(podRef{context: "ctx", namespace: "ns", name: "pod-1"}, true)

	assert.True(t, cancelled, "reconnect must cancel the previous kubectl stream")
	assert.Equal(t, []string{"a", "b"}, m.previewLog.lines, "reconnect must preserve existing lines")
	assert.Equal(t, 3, m.previewLog.autoReconnectAttempt, "reconnect must not reset attempt counter")
	assert.Equal(t, 2, m.previewLog.fromBottom, "reconnect must not reset fromBottom")
	assert.Equal(t, "ctx/ns/pod-1", m.previewLog.podKey)
}

// TestPreviewSwitchClearsBuffer asserts that startPreviewLogStream with
// reconnect=false (pod switch / initial open) clears lines and resets
// the attempt counter — preserving the original cancel-and-clear behavior.
func TestPreviewSwitchClearsBuffer(t *testing.T) {
	m := baseModelWithFakeClient()
	m.previewLog.readerInFlight = &atomic.Bool{}

	m.previewLog.cancel = func() {}
	m.previewLog.lines = []string{"old-line"}
	m.previewLog.autoReconnectAttempt = 5
	m.previewLog.fromBottom = 3
	m.previewLog.podKey = "ctx/ns/pod-1"

	m, _ = m.startPreviewLogStream(podRef{context: "ctx", namespace: "ns", name: "pod-2"}, false)

	assert.Empty(t, m.previewLog.lines, "pod switch must clear the buffer")
	assert.Equal(t, 0, m.previewLog.autoReconnectAttempt, "pod switch must reset attempt counter")
	assert.Equal(t, 0, m.previewLog.fromBottom, "pod switch must reset fromBottom")
	assert.Equal(t, "ctx/ns/pod-2", m.previewLog.podKey)
}

// TestPreviewReconnectStopsAfterMaxAttempts asserts that when
// autoReconnectAttempt is already at the max, the done-path in
// updatePreviewLogLine returns nil cmd (no further schedule).
func TestPreviewReconnectStopsAfterMaxAttempts(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Context = "ctx"
	m = withSelectedPod(t, m, "ns", "pod-1")
	m.fullLogPreview = true
	m.previewLog.readerInFlight = &atomic.Bool{}
	ch := make(chan string)
	m.previewLog.ch = ch
	m.previewLog.podKey = "ctx/ns/pod-1"
	m.previewLog.lines = []string{"existing", "lines"}
	m.previewLog.autoReconnectAttempt = logAutoReconnectMaxAttempts

	mm, cmd := m.updatePreviewLogLine(previewLogLineMsg{done: true, ch: ch})
	rm := mm.(Model)

	assert.Equal(t, logAutoReconnectMaxAttempts, rm.previewLog.autoReconnectAttempt,
		"attempt must not exceed max")
	assert.Nil(t, cmd, "no restart cmd when max attempts reached")
	// Buffer must be untouched — we didn't restart, so no clear occurred.
	assert.Equal(t, []string{"existing", "lines"}, rm.previewLog.lines,
		"buffer must be preserved when max attempts reached")
}

func TestSelectedPodForLogPreview(t *testing.T) {
	t.Run("single pod resolves", func(t *testing.T) {
		m := baseModelWithFakeClient()
		m = withSelectedPod(t, m, "ns", "pod-1")
		ref, ok := m.selectedPodForLogPreview()
		assert.True(t, ok)
		assert.Equal(t, "pod-1", ref.name)
		assert.Equal(t, "ns", ref.namespace)
	})
	t.Run("non-pod resource does not resolve", func(t *testing.T) {
		m := baseModelWithFakeClient()
		m = withSelectedResource(t, m, "Service", "ns", "svc-1")
		_, ok := m.selectedPodForLogPreview()
		assert.False(t, ok)
	})
	t.Run("container row resolves to its pod plus container", func(t *testing.T) {
		m := baseModelWithFakeClient()
		m = withSelectedContainer(t, m, "ns", "pod-1", "sidecar")
		ref, ok := m.selectedPodForLogPreview()
		assert.True(t, ok)
		assert.Equal(t, "pod-1", ref.name, "pod name comes from nav.OwnedName")
		assert.Equal(t, "ns", ref.namespace)
		assert.Equal(t, "sidecar", ref.container)
	})
	t.Run("container row without owned pod does not resolve", func(t *testing.T) {
		m := baseModelWithFakeClient()
		m = withSelectedContainer(t, m, "ns", "", "sidecar")
		_, ok := m.selectedPodForLogPreview()
		assert.False(t, ok)
	})
}

// Two containers of the same pod must stream and cache independently: the
// podRef key embeds the container so a container switch restarts the stream.
func TestPodRefKeyDistinguishesContainers(t *testing.T) {
	pod := podRef{context: "ctx", namespace: "ns", name: "pod-1"}
	app := podRef{context: "ctx", namespace: "ns", name: "pod-1", container: "app"}
	sidecar := podRef{context: "ctx", namespace: "ns", name: "pod-1", container: "sidecar"}

	assert.Equal(t, "ctx/ns/pod-1", pod.key(), "whole-pod key keeps its historical shape")
	assert.NotEqual(t, app.key(), sidecar.key())
	assert.NotEqual(t, pod.key(), app.key())
}

// The pane title shows which container is being tailed.
func TestPreviewLogPodLabelIncludesContainer(t *testing.T) {
	m := baseModelWithFakeClient()
	m = withSelectedContainer(t, m, "ns", "pod-1", "app")
	assert.Equal(t, "ns/pod-1/app", m.previewLogPodLabel())
}

func TestToggleLogPreviewMutualExclusivity(t *testing.T) {
	m := baseModelWithFakeClient()
	m.fullYAMLPreview = true

	mm, _ := m.handleExplorerToggleLogPreview()
	m = mm.(Model)
	assert.True(t, m.fullLogPreview, "first toggle enables log preview")
	assert.False(t, m.fullYAMLPreview, "enabling log preview turns off YAML preview")

	mm, _ = m.handleExplorerToggleLogPreview()
	m = mm.(Model)
	assert.False(t, m.fullLogPreview, "second toggle turns it off")
}

// TestContextSwitchCancelsLogPreview asserts that navigateChildCluster cancels
// any running live-log preview stream so stale goroutines cannot outlive the
// context they were opened against.
func TestContextSwitchCancelsLogPreview(t *testing.T) {
	m := baseModelWithFakeClient()
	cancelled := false
	m.previewLog.cancel = func() { cancelled = true }
	m.fullLogPreview = true
	sel := &model.Item{Name: "other-ctx"}
	_, _ = m.navigateChildCluster(sel)
	assert.True(t, cancelled, "leaving a context must cancel the live-log preview")
}

// TestPreviewLogSwitchesOnlyOnPodChange verifies that maybeRestartPreviewLog
// is a no-op when the selection resolves to the same pod key, and restarts
// the stream (updating podKey) when it resolves to a different pod.
func TestPreviewLogSwitchesOnlyOnPodChange(t *testing.T) {
	t.Run("same pod is a no-op", func(t *testing.T) {
		m := baseModelWithFakeClient()
		m.fullLogPreview = true
		m.previewLog.podKey = "ctx/ns/pod-1"
		m2, cmd := m.maybeRestartPreviewLog(podRef{context: "ctx", namespace: "ns", name: "pod-1"})
		assert.Equal(t, "ctx/ns/pod-1", m2.previewLog.podKey, "same pod must not change podKey")
		assert.Nil(t, cmd, "same pod must return nil cmd (no-op)")
	})

	t.Run("different pod restarts the stream", func(t *testing.T) {
		m := baseModelWithFakeClient()
		m.previewLog.readerInFlight = &atomic.Bool{}
		m.fullLogPreview = true
		m.previewLog.podKey = "ctx/ns/pod-1"
		m2, _ := m.maybeRestartPreviewLog(podRef{context: "ctx", namespace: "ns", name: "pod-2"})
		assert.Equal(t, "ctx/ns/pod-2", m2.previewLog.podKey, "different pod must update podKey")
	})
}

// TestMaybeRestartPreviewLogCancelsWhenNoPod verifies that when fullLogPreview is
// on but no pod is resolved, cancelPreviewLogStream is called and the state is cleared.
func TestMaybeRestartPreviewLogCancelsWhenNoPod(t *testing.T) {
	m := baseModelWithFakeClient()
	m.fullLogPreview = true
	cancelled := false
	m.previewLog.cancel = func() { cancelled = true }
	m.previewLog.podKey = "ctx/ns/pod-1"
	m.cancelPreviewLogStream()
	assert.True(t, cancelled)
	assert.Empty(t, m.previewLog.podKey)
}

// --- Auto-reconnect tests (TDD: written before implementation) ---

// TestPreviewLogReconnectsOnDoneWhileActive asserts that when fullLogPreview is
// on, a pod is selected, and the stream's done msg arrives for the current ch,
// the handler increments autoReconnectAttempt and returns a non-nil cmd.
func TestPreviewLogReconnectsOnDoneWhileActive(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Context = "ctx"
	m = withSelectedPod(t, m, "ns", "pod-1")
	m.fullLogPreview = true
	m.previewLog.readerInFlight = &atomic.Bool{}
	ch := make(chan string)
	m.previewLog.ch = ch
	m.previewLog.podKey = "ctx/ns/pod-1" // matches selected pod key

	mm, cmd := m.updatePreviewLogLine(previewLogLineMsg{done: true, ch: ch})
	rm := mm.(Model)
	assert.Equal(t, 1, rm.previewLog.autoReconnectAttempt, "attempt must be incremented on done")
	require.NotNil(t, cmd, "a scheduled restart cmd must be returned")
}

// TestPreviewLogStopsReconnectWhenToggledOff asserts that when fullLogPreview
// is false, a done msg produces no retry (nil cmd) and attempt stays at 0.
func TestPreviewLogStopsReconnectWhenToggledOff(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Context = "ctx"
	m = withSelectedPod(t, m, "ns", "pod-1")
	m.fullLogPreview = false
	m.previewLog.readerInFlight = &atomic.Bool{}
	ch := make(chan string)
	m.previewLog.ch = ch
	m.previewLog.podKey = "ctx/ns/pod-1"

	mm, cmd := m.updatePreviewLogLine(previewLogLineMsg{done: true, ch: ch})
	rm := mm.(Model)
	assert.Equal(t, 0, rm.previewLog.autoReconnectAttempt, "attempt must stay 0 when preview is off")
	assert.Nil(t, cmd, "no restart cmd when preview is toggled off")
}

// TestPreviewLogLineResetsReconnectAttempt asserts that a normal line arriving
// resets autoReconnectAttempt to 0.
func TestPreviewLogLineResetsReconnectAttempt(t *testing.T) {
	m := baseModelWithFakeClient()
	m.fullLogPreview = true
	m.previewLog.readerInFlight = &atomic.Bool{}
	ch := make(chan string)
	m.previewLog.ch = ch
	m.previewLog.podKey = "ctx/ns/pod-1"
	m.previewLog.autoReconnectAttempt = 5

	mm, _ := m.updatePreviewLogLine(previewLogLineMsg{line: "hello", ch: ch})
	rm := mm.(Model)
	assert.Equal(t, 0, rm.previewLog.autoReconnectAttempt, "a line must reset the attempt counter to 0")
}

// TestPreviewLogStopsReconnectAtMaxAttempts asserts that once the attempt
// counter reaches logAutoReconnectMaxAttempts, no further restart is scheduled.
func TestPreviewLogStopsReconnectAtMaxAttempts(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Context = "ctx"
	m = withSelectedPod(t, m, "ns", "pod-1")
	m.fullLogPreview = true
	m.previewLog.readerInFlight = &atomic.Bool{}
	ch := make(chan string)
	m.previewLog.ch = ch
	m.previewLog.podKey = "ctx/ns/pod-1"
	m.previewLog.autoReconnectAttempt = logAutoReconnectMaxAttempts

	mm, cmd := m.updatePreviewLogLine(previewLogLineMsg{done: true, ch: ch})
	rm := mm.(Model)
	assert.Equal(t, logAutoReconnectMaxAttempts, rm.previewLog.autoReconnectAttempt, "attempt must not exceed max")
	assert.Nil(t, cmd, "no restart cmd when max attempts reached")
}

// TestUpdatePreviewLogRestartDropsSuperseded asserts that a previewLogRestartMsg
// whose ch does not match the current previewLog.ch is silently dropped.
func TestUpdatePreviewLogRestartDropsSuperseded(t *testing.T) {
	m := baseModelWithFakeClient()
	m.nav.Context = "ctx"
	m = withSelectedPod(t, m, "ns", "pod-1")
	m.fullLogPreview = true
	m.previewLog.readerInFlight = &atomic.Bool{}
	currentCh := make(chan string)
	staleCh := make(chan string)
	m.previewLog.ch = currentCh
	m.previewLog.podKey = "ctx/ns/pod-1"

	mm, cmd := m.updatePreviewLogRestart(previewLogRestartMsg{ch: staleCh})
	rm := mm.(Model)
	assert.Equal(t, currentCh, rm.previewLog.ch, "stale restart must not replace current ch")
	assert.Nil(t, cmd, "stale restart must produce no cmd")
}

// --- Fix B: J/K scroll-back and clamp ---

// TestPreviewLogScrollBackAndForward verifies K=older/J=newer semantics.
// The model is given many buffered lines so clampPreviewScroll does not
// immediately zero fromBottom.
func TestPreviewLogScrollBackAndForward(t *testing.T) {
	m := baseModelWithFakeClient()
	m.fullLogPreview = true
	// Populate enough lines that clamping does not immediately zero fromBottom.
	m.width = 200
	m.height = 30
	for i := range 50 {
		m.appendPreviewLogLine("line " + fmt.Sprintf("%02d", i))
	}
	m.previewLog.fromBottom = 0

	// K: scroll back into history (fromBottom increases).
	mm, _, _ := m.handleExplorerActionKeyPreviewUp()
	m = mm.(Model)
	assert.Equal(t, 1, m.previewLog.fromBottom, "K must increment fromBottom")

	// K again.
	mm, _, _ = m.handleExplorerActionKeyPreviewUp()
	m = mm.(Model)
	assert.Equal(t, 2, m.previewLog.fromBottom)

	// J: scroll toward newest (fromBottom decreases).
	mm, _, _ = m.handleExplorerActionKeyPreviewDown()
	m = mm.(Model)
	assert.Equal(t, 1, m.previewLog.fromBottom, "J must decrement fromBottom")

	// J at 0 must stay at 0 (no negative).
	m.previewLog.fromBottom = 0
	mm, _, _ = m.handleExplorerActionKeyPreviewDown()
	m = mm.(Model)
	assert.Equal(t, 0, m.previewLog.fromBottom, "fromBottom must not go below 0")
}

// TestPreviewLogNonLogPreviewScrollUnchanged verifies that when fullLogPreview
// is false, K decrements previewScroll (and J can only increment if content exists).
func TestPreviewLogNonLogPreviewScrollUnchanged(t *testing.T) {
	m := baseModelWithFakeClient()
	m.fullLogPreview = false
	m.previewScroll = 2

	// K must decrement previewScroll (no clamp interference needed for going down).
	mm, _, _ := m.handleExplorerActionKeyPreviewUp()
	m = mm.(Model)
	assert.Equal(t, 1, m.previewScroll, "K must decrement previewScroll when not in log preview")
}

// TestTabSwitchPreservesLogPreviewFlag mirrors
// TestTabSwitchPreservesYAMLViewerState for the fullLogPreview bool: the flag
// must round-trip through saveCurrentTab → loadTab without bleeding between
// tabs.
func TestTabSwitchPreservesLogPreviewFlag(t *testing.T) {
	m := Model{
		tabs: []TabState{
			{fullLogPreview: true},  // Tab A: has log preview on.
			{fullLogPreview: false}, // Tab B: log preview off.
		},
		activeTab:      0,
		fullLogPreview: true,
	}

	// Switch to Tab B: its persisted flag must become active.
	m.saveCurrentTab()
	m.loadTab(1)
	assert.False(t, m.fullLogPreview, "Tab B must restore its own fullLogPreview=false")

	// Switch back to Tab A: its persisted flag must round-trip intact.
	m.saveCurrentTab()
	m.loadTab(0)
	assert.True(t, m.fullLogPreview, "Tab A must restore its own fullLogPreview=true")
}
