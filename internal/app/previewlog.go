// Package app — previewlog.go
// Right-pane live-log preview: a bounded buffer fed by a dedicated kubectl
// logs follow-stream, isolated from the fullscreen logView so the two can
// never clobber each other.
package app

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/ui"
)

// previewLogCap is the initial live-stream ring size: the pane only ever shows
// the last screenful on fresh open, so a small value keeps memory flat.
// Lazy history loading can grow the buffer up to previewLogMaxLines.
const previewLogCap = 100

// previewLogMaxLines is the total buffer bound once lazy history has been
// prepended. Older lines beyond this cap are dropped and hasMoreHistory is set
// to false.
const previewLogMaxLines = 1000

// previewLogHistoryBatch is the number of additional lines fetched per
// lazy-history request (the --tail increment for each older-batch fetch).
const previewLogHistoryBatch = 100

// previewLogViewportHeightDefault is the fallback historyTail value used in
// tests where previewLogViewportHeight() cannot be computed (zero-sized model).
// It mirrors previewLogMinViewportHeight so tests stay self-consistent.
const previewLogViewportHeightDefault = previewLogMinViewportHeight

// previewLogCacheMax caps the number of per-pod buffers kept in the LRU cache.
// Oldest entry is evicted once the cap is exceeded.
const previewLogCacheMax = 5

// previewLogMinViewportHeight is the floor applied when computing the initial
// --tail for a fresh stream from the visible pane height.
const previewLogMinViewportHeight = 10

// previewLogCacheEntry holds a snapshot of the preview buffer for one pod.
type previewLogCacheEntry struct {
	lines          []string
	fromBottom     int
	hasMoreHistory bool
	historyTail    int
}

type previewLogState struct {
	lines                []string
	ch                   chan string
	cancel               context.CancelFunc
	podKey               string       // ctx/ns/name currently streaming
	readerInFlight       *atomic.Bool // single-reader guard across model copies
	err                  string
	autoReconnectAttempt int // consecutive done-reconnect attempts; reset to 0 on a line
	// fromBottom is the scroll offset for the preview pane.
	// 0 = auto-follow (show newest); N = show N rows up from the bottom.
	// K scrolls back (fromBottom++), J scrolls forward (fromBottom--).
	fromBottom int

	// Lazy history loading state.
	// hasMoreHistory is true when there may be older lines not yet loaded.
	// Set to true on fresh stream start; cleared when an empty overlap is found
	// or the buffer cap is reached.
	hasMoreHistory bool
	// loadingHistory is true while a one-shot history fetch is in flight.
	loadingHistory bool
	// historyTail is the --tail value for the NEXT older-batch kubectl fetch.
	// Starts at previewLogViewportHeight() (the initial tail) and grows by
	// previewLogHistoryBatch with each successful prepend.
	historyTail int
}

// appendPreviewLogLine adds a line to the preview buffer. The buffer may grow
// beyond previewLogCap when lazy history has been prepended; it is only capped
// at previewLogMaxLines to bound total memory. The GC-friendly tail-copy is
// performed at the higher bound so the dropped head can be collected.
func (m *Model) appendPreviewLogLine(line string) {
	m.previewLog.lines = append(m.previewLog.lines, line)
	m.previewLog.lines, _ = capLines(m.previewLog.lines, previewLogMaxLines)
}

// stopPreviewStream sends a cancel signal to the active kubectl goroutine and
// clears the ch/cancel pair, but leaves lines, podKey, autoReconnectAttempt,
// and fromBottom untouched. Used by the reconnect path so the buffer is
// preserved across retries.
func (m *Model) stopPreviewStream() {
	if m.previewLog.cancel != nil {
		m.previewLog.cancel()
		m.previewLog.cancel = nil
	}
	m.previewLog.ch = nil
}

// cancelPreviewLogStream stops the active preview stream, if any, and resets
// all related state so the pane shows its placeholder. Use this on pod switch,
// toggle-off, tab switch, and context change — anywhere the buffer must be
// cleared. The reconnect path uses stopPreviewStream instead.
func (m *Model) cancelPreviewLogStream() {
	m.stopPreviewStream()
	m.previewLog.podKey = ""
	m.previewLog.lines = nil
	m.previewLog.err = ""
	m.previewLog.autoReconnectAttempt = 0
	m.previewLog.fromBottom = 0
	m.previewLog.hasMoreHistory = false
	m.previewLog.loadingHistory = false
	m.previewLog.historyTail = 0
}

// startPreviewLogStream starts a new preview log stream for ref.
//
// When reconnect is false (initial open, pod switch, toggle-on) the full
// cancelPreviewLogStream is called first: lines, podKey, err, attempt, and
// fromBottom are all cleared and --tail=<previewLogCap> is used.
//
// When reconnect is true (same pod, stream ended, retrying) only
// stopPreviewStream is called: lines, podKey, autoReconnectAttempt, and
// fromBottom are PRESERVED so there is no buffer-clear flicker, and --tail=0
// is used so a terminated pod yields nothing (the attempt counter then
// increments to the max and stops).
//
// The cancel-of-previous and podKey bookkeeping are performed before any
// kubectl invocation, making them safe to assert in unit tests regardless of
// whether kubectl is present in the test environment.
//
// Returns the updated model and a tea.Cmd that reads the first line from the
// new stream (via waitForPreviewLogLine). If kubectl is not found, the
// function records the error in previewLog.err and returns a nil cmd.
func (m Model) startPreviewLogStream(ref podRef, reconnect bool) (Model, tea.Cmd) {
	// Stop / clear the previous stream before touching kubectl. The full cancel
	// clears the buffer; the light stop preserves it for reconnects.
	if reconnect {
		m.stopPreviewStream()
	} else {
		m.cancelPreviewLogStream()
	}

	// Initialise the reader-in-flight guard if not already set (shared across
	// model copies via pointer — allocate exactly once per stream session).
	if m.previewLog.readerInFlight == nil {
		m.previewLog.readerInFlight = &atomic.Bool{}
	} else {
		// Reset so the first arm on the new stream is not blocked.
		m.previewLog.readerInFlight.Store(false)
	}

	// Book-keep the new pod key immediately so the test can assert it without
	// needing kubectl to be present.
	m.previewLog.podKey = ref.key()

	kubectlPath, err := k8s.KubectlPath()
	if err != nil {
		m.previewLog.err = fmt.Sprintf("kubectl not found: %v", err)
		return m, nil
	}

	kubeconfigPaths := m.client.KubeconfigPathForContext(ref.context)

	// --tail=0 on reconnect/resume: a terminated pod yields nothing, so the
	// attempt counter increments to max and stops without re-dumping history.
	// Fresh starts use the visible pane height as the initial tail so the pane
	// fills on first open rather than fetching an arbitrary fixed count.
	initialTail := m.previewLogViewportHeight()
	tail := initialTail
	if reconnect {
		tail = 0
	}

	// Set history state for fresh starts only. On reconnect the existing state
	// is preserved (the user may already have scrolled back into loaded history).
	if !reconnect {
		m.previewLog.hasMoreHistory = true
		m.previewLog.loadingHistory = false
		m.previewLog.historyTail = initialTail
	}

	args := kubectlPodLogArgs(ref.name, ref.namespace, m.kubectlContext(ref.context), true, tail, ref.container)

	ctx, cancel := context.WithCancel(context.Background())
	m.previewLog.cancel = cancel

	ch := make(chan string, 64)
	m.previewLog.ch = ch

	go func() {
		defer close(ch)

		cmd := exec.CommandContext(ctx, kubectlPath, k8s.DemoKubectlArgs(args)...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
		logger.Info("Starting preview kubectl logs", "args", args, "kubeconfig", kubeconfigPaths)

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			logger.Error("Failed to create preview stdout pipe", "error", err)
			return
		}
		cmd.Stderr = cmd.Stdout

		if err := cmd.Start(); err != nil {
			logger.Error("Failed to start preview kubectl logs", "error", err)
			select {
			case ch <- fmt.Sprintf("[error] %v", err):
			case <-ctx.Done():
			}
			return
		}
		defer cmd.Wait() //nolint:errcheck

		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
		for scanner.Scan() {
			select {
			case ch <- scanner.Text():
			case <-ctx.Done():
				return
			}
		}
	}()

	return m, m.waitForPreviewLogLine()
}

// waitForPreviewLogLine returns a tea.Cmd that reads exactly one line from
// previewLog.ch. The readerInFlight guard (Swap-on-arm) ensures that only one
// reader is in flight at a time even when model copies are made during a pod
// switch; if a reader is already in flight the cmd is a no-op.
func (m Model) waitForPreviewLogLine() tea.Cmd {
	ch := m.previewLog.ch
	if ch == nil {
		return nil
	}
	guard := m.previewLog.readerInFlight
	if guard == nil {
		return nil
	}
	// Arm only when no reader is currently in flight.
	if !guard.Swap(true) {
		return func() tea.Msg {
			line, ok := <-ch
			if !ok {
				return previewLogLineMsg{done: true, ch: ch}
			}
			return previewLogLineMsg{line: line, ch: ch}
		}
	}
	return nil
}

// updatePreviewLogLine handles a line arriving from the live-log preview stream.
// Stale messages (from a superseded stream) are silently dropped. After
// consuming a line the reader guard is reset to idle and exactly one new reader
// is armed so the stream continues without concurrent readers.
//
// On done: if fullLogPreview is on, the selected pod matches the streaming
// podKey, and autoReconnectAttempt < logAutoReconnectMaxAttempts, a restart is
// scheduled (mirroring the fullscreen log viewer's auto-reconnect for pod
// restarts / ContainerCreating transitions). On a normal line the attempt
// counter is reset to 0.
func (m Model) updatePreviewLogLine(msg previewLogLineMsg) (tea.Model, tea.Cmd) {
	// Drop lines that belong to a superseded stream.
	if msg.ch != m.previewLog.ch {
		return m, nil
	}

	// The reader that produced this msg has exited; mark it idle so the next
	// arm in waitForPreviewLogLine is not blocked by a stale true value.
	if m.previewLog.readerInFlight != nil {
		m.previewLog.readerInFlight.Store(false)
	}

	if msg.done {
		// Stream ended (channel closed by the producer goroutine). Schedule an
		// auto-reconnect when the preview is still active and the selected pod
		// hasn't changed — this handles ContainerCreating / init-container
		// transitions identically to the fullscreen log viewer.
		m.previewLog.err = ""
		if m.fullLogPreview && m.previewLog.autoReconnectAttempt < logAutoReconnectMaxAttempts {
			if ref, ok := m.selectedPodForLogPreview(); ok {
				if ref.key() == m.previewLog.podKey {
					m.previewLog.autoReconnectAttempt++
					return m, schedulePreviewLogRestart(msg.ch)
				}
			}
		}
		return m, nil
	}

	// A line arrived — reset the reconnect backoff counter.
	if m.previewLog.autoReconnectAttempt > 0 {
		m.previewLog.autoReconnectAttempt = 0
	}
	// Keep a scrolled-back view anchored to the content the user is reading.
	// fromBottom is a bottom-anchored physical-line offset, so appending a line
	// would otherwise shift the window toward the newest line. When the user has
	// scrolled back (fromBottom > 0) advance it by the new line's physical
	// (wrapped) height so the same lines stay visible; fromBottom == 0 keeps
	// auto-following the newest line. The single-line measure is O(1).
	if m.previewLog.fromBottom > 0 {
		m.previewLog.fromBottom += ui.PreviewLogPhysicalCount([]string{msg.line}, m.previewLogInnerWidth())
	}
	m.appendPreviewLogLine(msg.line)
	return m, m.waitForPreviewLogLine()
}

// schedulePreviewLogRestart returns a tea.Cmd that waits for
// logAutoReconnectDelay and then emits a previewLogRestartMsg carrying the
// channel of the preview stream that just ended. The Update handler correlates
// on that channel to ignore stale restarts after a pod switch or toggle-off.
func schedulePreviewLogRestart(ch chan string) tea.Cmd {
	return tea.Tick(logAutoReconnectDelay, func(_ time.Time) tea.Msg {
		return previewLogRestartMsg{ch: ch}
	})
}

// updatePreviewLogRestart fires when a scheduled preview auto-reconnect is due.
// The restart is silently dropped if the preview was toggled off, the pod
// changed (ch mismatch — startPreviewLogStream creates a new ch on every
// restart), or no pod is currently selected.
func (m Model) updatePreviewLogRestart(msg previewLogRestartMsg) (tea.Model, tea.Cmd) {
	// Guard: preview must still be on and the stream must not have been
	// superseded (pod switch / toggle creates a new ch, making this stale).
	if !m.fullLogPreview || m.previewLog.ch != msg.ch {
		return m, nil
	}
	ref, ok := m.selectedPodForLogPreview()
	if !ok {
		return m, nil
	}
	m, cmd := m.startPreviewLogStream(ref, true)
	return m, cmd
}

// handleExplorerToggleLogPreview toggles the right-pane live-log preview,
// mirroring handleExplorerTogglePreview for the YAML preview. It clears
// mutually-exclusive modes (fullYAMLPreview, mapView, resourceTree) on enable
// and cancels the stream on disable.
func (m Model) handleExplorerToggleLogPreview() (tea.Model, tea.Cmd) {
	m.fullLogPreview = !m.fullLogPreview
	// Log preview is mutually exclusive with YAML preview and resource map.
	m.fullYAMLPreview = false
	m.mapView = false
	m.resourceTree = nil

	var readerCmd tea.Cmd
	if m.fullLogPreview {
		if ref, ok := m.selectedPodForLogPreview(); ok {
			m, readerCmd = m.enterPreviewLogPod(ref)
		}
		m.setStatusMessage("Live logs preview", false)
	} else {
		m.cachePreviewLog()
		m.cancelPreviewLogStream()
		m.setStatusMessage("Details preview", false)
	}

	return m, tea.Batch(m.loadPreview(), readerCmd, scheduleStatusClear())
}

// maybeRestartPreviewLog checks whether the currently-streaming pod differs
// from ref. If they are the same (matching ctx/ns/name key) it is a no-op
// and returns (m, nil). If they differ it cancels the old stream and starts a
// new one for ref, returning the updated model and the first-line reader cmd.
//
// This is the pod-switch debounce guard: callers must check fullLogPreview
// before calling, and should batch the returned cmd with any other cmds they
// are returning to the Bubble Tea runtime.
func (m Model) maybeRestartPreviewLog(ref podRef) (Model, tea.Cmd) {
	if m.previewLog.podKey == ref.key() {
		// Same pod — nothing to do.
		return m, nil
	}
	return m.enterPreviewLogPod(ref)
}

// maybeRestartOrCancelPreviewLog is the high-level hook for selection-change
// events. When fullLogPreview is on it resolves the selected pod and either
// restarts the stream (new pod) or cancels it (no pod). Returns the updated
// model and any reader cmd to batch.
func (m Model) maybeRestartOrCancelPreviewLog() (Model, tea.Cmd) {
	if !m.fullLogPreview {
		return m, nil
	}
	ref, ok := m.selectedPodForLogPreview()
	if !ok {
		// No pod selected — cache the buffer then clear the stream.
		m.cachePreviewLog()
		m.cancelPreviewLogStream()
		return m, nil
	}
	return m.maybeRestartPreviewLog(ref)
}

// previewLogPodLabel returns "namespace/name" for the currently-selected pod
// ("namespace/name/container" for a container stream), or "" when no pod
// resolves (so RenderLogPreview shows its placeholder).
func (m Model) previewLogPodLabel() string {
	ref, ok := m.selectedPodForLogPreview()
	if !ok {
		return ""
	}
	label := ref.namespace + "/" + ref.name
	if ref.container != "" {
		label += "/" + ref.container
	}
	return label
}

// podRef identifies a single pod — or one container of it — for the live-log
// preview stream. An empty container means "all containers" (the whole-pod
// stream).
type podRef struct{ context, namespace, name, container string }

// key returns the cache/dedup key for this ref ("ctx/ns/name", with a
// "/container" suffix for single-container streams so a container switch is
// seen as a stream change). Kubernetes name validation forbids "/" in
// namespace, pod, and container names, so the separator is unambiguous.
func (r podRef) key() string {
	k := r.context + "/" + r.namespace + "/" + r.name
	if r.container != "" {
		k += "/" + r.container
	}
	return k
}

// cachePreviewLog saves the current preview buffer under its podKey so it can
// be restored when the user returns to the same pod. A copy of the slice is
// stored (not an alias) so later mutations to the live buffer cannot corrupt
// the cache. No-op when podKey is empty or the buffer is empty.
// LRU eviction: the key is moved to most-recent in previewLogCacheOrder; if
// the cache exceeds previewLogCacheMax the oldest key is dropped.
func (m *Model) cachePreviewLog() {
	if m.previewLog.podKey == "" || len(m.previewLog.lines) == 0 {
		return
	}
	if m.previewLogCache == nil {
		m.previewLogCache = make(map[string]previewLogCacheEntry)
	}
	key := m.previewLog.podKey

	// Move key to most-recent position in the order slice.
	m.previewLogCacheOrder = removeCacheOrderKey(m.previewLogCacheOrder, key)
	m.previewLogCacheOrder = append(m.previewLogCacheOrder, key)

	// Store a copy of the lines so the live buffer and cache do not alias.
	m.previewLogCache[key] = previewLogCacheEntry{
		lines:          append([]string(nil), m.previewLog.lines...),
		fromBottom:     m.previewLog.fromBottom,
		hasMoreHistory: m.previewLog.hasMoreHistory,
		historyTail:    m.previewLog.historyTail,
	}

	// Evict the oldest entry when over cap.
	for len(m.previewLogCache) > previewLogCacheMax && len(m.previewLogCacheOrder) > 0 {
		oldest := m.previewLogCacheOrder[0]
		m.previewLogCacheOrder = m.previewLogCacheOrder[1:]
		delete(m.previewLogCache, oldest)
	}
}

// restorePreviewLogFromCache restores the buffer for podKey from the cache.
// Returns true when an entry was found and applied. The entry's LRU recency is
// bumped on hit. Returns false when the cache is empty or has no entry for
// podKey.
func (m *Model) restorePreviewLogFromCache(podKey string) bool {
	if m.previewLogCache == nil {
		return false
	}
	entry, ok := m.previewLogCache[podKey]
	if !ok {
		return false
	}
	// Restore a copy so future live-buffer mutations don't reach the cache.
	m.previewLog.lines = append([]string(nil), entry.lines...)
	m.previewLog.fromBottom = entry.fromBottom
	m.previewLog.hasMoreHistory = entry.hasMoreHistory
	m.previewLog.historyTail = entry.historyTail
	// Bump LRU recency.
	m.previewLogCacheOrder = removeCacheOrderKey(m.previewLogCacheOrder, podKey)
	m.previewLogCacheOrder = append(m.previewLogCacheOrder, podKey)
	return true
}

// removeCacheOrderKey returns a new slice with key removed (preserving order).
func removeCacheOrderKey(order []string, key string) []string {
	out := order[:0:len(order)]
	for _, k := range order {
		if k != key {
			out = append(out, k)
		}
	}
	return out
}

// enterPreviewLogPod centralises the "switch to this pod" transition:
//  1. Cache the buffer of the pod we are leaving.
//  2. If the pod being entered has a cached buffer, restore it and start the
//     stream in resume mode (--tail=0, new-lines only).
//  3. Otherwise, start a fresh stream using the visible pane height as --tail.
func (m Model) enterPreviewLogPod(ref podRef) (Model, tea.Cmd) {
	// 1. Save the departing pod's buffer.
	m.cachePreviewLog()

	// 2. Try to restore from cache.
	if m.restorePreviewLogFromCache(ref.key()) {
		// Buffer restored — start the stream in resume mode: preserve the
		// restored lines, set podKey, use --tail=0 (new lines only).
		m.stopPreviewStream()
		// Reset the reader guard for the new stream.
		if m.previewLog.readerInFlight == nil {
			m.previewLog.readerInFlight = &atomic.Bool{}
		} else {
			m.previewLog.readerInFlight.Store(false)
		}
		m.previewLog.podKey = ref.key()
		// Delegate to startPreviewLogStream with reconnect=true so it builds
		// args with --tail=0 and starts the goroutine without clearing the buffer.
		return m.startPreviewLogStream(ref, true)
	}

	// 3. No cache hit — fresh start (clears buffer, uses fill-pane --tail).
	return m.startPreviewLogStream(ref, false)
}

// previewLogInnerWidth returns the content width (in cells) of the live-log
// right pane, matching the rightW-2 computation in clampPreviewScroll and
// renderRightColumn so the scroll math and the renderer agree on wrap width.
func (m Model) previewLogInnerWidth() int {
	usable := m.width - 6
	rightW := max(10, usable-max(10, usable*12/100)-max(10, usable*51/100))
	return rightW - 2
}

// previewLogViewportHeight returns the number of body lines visible in the
// live-log right pane, clamped to [previewLogMinViewportHeight, previewLogCap].
// This is used as the initial --tail value for fresh streams so the pane fills
// on the first open instead of fetching an arbitrary fixed count.
//
// The calculation mirrors clampPreviewScroll / renderRightColumn so the result
// is always consistent with what the renderer actually shows.
func (m Model) previewLogViewportHeight() int {
	usable := m.width - 6
	rightW := max(10, usable-max(10, usable*12/100)-max(10, usable*51/100))
	_ = rightW // inner width is used only for content wrap — not needed for height
	colHeight := max(m.height-4, 3)
	if len(m.tabs) > 1 {
		colHeight--
	}
	// The body area inside the log pane excludes the title bar (1 line).
	bodyHeight := max(colHeight-1, 1)
	if bodyHeight < previewLogMinViewportHeight {
		return previewLogMinViewportHeight
	}
	if bodyHeight > previewLogCap {
		return previewLogCap
	}
	return bodyHeight
}

// selectedPodForLogPreview returns the pod to tail when the current selection
// resolves to exactly one pod (a Pod row at LevelResources, or a Pod selected
// after drilling into a parent at LevelOwned) or one container of a pod (a
// Container row at LevelContainers — the parent pod comes from nav.OwnedName,
// mirroring buildActionCtx). Returns ok=false for multi-pod parents and
// non-pod resources.
func (m Model) selectedPodForLogPreview() (podRef, bool) {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return podRef{}, false
	}
	name, container := sel.Name, ""
	if sel.Kind == "Container" {
		if m.nav.OwnedName == "" {
			return podRef{}, false
		}
		name, container = m.nav.OwnedName, sel.Name
	} else if sel.Kind != "Pod" {
		return podRef{}, false
	}
	// Mirror the namespace resolution from buildActionCtx.
	ns := sel.Namespace
	if ns == "" {
		ns = m.nav.Namespace
	}
	if ns == "" {
		ns = m.namespace
	}
	return podRef{
		context:   m.effectiveContext(),
		namespace: ns,
		name:      name,
		container: container,
	}, true
}

// fetchOlderPreviewLogs returns a tea.Cmd that runs a one-shot kubectl logs
// --tail=<tail> for ref (no -f), capturing combined output, and emits a
// previewLogHistoryMsg. The fetch runs in the background; the model correlates
// the result by podKey to drop stale responses.
func (m Model) fetchOlderPreviewLogs(ref podRef, tail int) tea.Cmd {
	kubectlPath, err := k8s.KubectlPath()
	if err != nil {
		return func() tea.Msg {
			return previewLogHistoryMsg{podKey: ref.key(), err: err}
		}
	}

	kubeconfigPaths := m.client.KubeconfigPathForContext(ref.context)
	podKey := ref.key()

	args := kubectlPodLogArgs(ref.name, ref.namespace, m.kubectlContext(ref.context), false, tail, ref.container)

	return func() tea.Msg {
		cmd := exec.Command(kubectlPath, k8s.DemoKubectlArgs(args)...) //nolint:gosec
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
		logger.Info("Fetching older preview log history", "args", args)

		output, err := cmd.CombinedOutput()
		if err != nil {
			return previewLogHistoryMsg{podKey: podKey, err: err}
		}

		var lines []string
		for line := range strings.SplitSeq(string(output), "\n") {
			if line != "" {
				lines = append(lines, line)
			}
		}
		return previewLogHistoryMsg{podKey: podKey, lines: lines}
	}
}

// updatePreviewLogHistory handles a previewLogHistoryMsg, prepending the
// genuinely-older delta lines to the preview buffer and advancing historyTail.
// Stale results (podKey mismatch or preview toggled off) are silently dropped
// without touching loadingHistory so the current pod's in-flight state is
// preserved.
func (m Model) updatePreviewLogHistory(msg previewLogHistoryMsg) tea.Model {
	// Drop stale or irrelevant results WITHOUT clearing loadingHistory: the
	// current pod may have its own fetch in flight whose result has not yet
	// arrived.
	if !m.fullLogPreview || msg.podKey != m.previewLog.podKey {
		return m
	}

	// From here the result is for the current pod — clear the in-flight flag.
	m.previewLog.loadingHistory = false

	if msg.err != nil || len(msg.lines) == 0 {
		m.previewLog.hasMoreHistory = false
		return m
	}

	// The fetched lines are the last <tail> lines of the pod; the current
	// buffer's oldest lines overlap the end of that set, so only the prefix
	// before the overlap is genuinely older.
	newOlder := mergeOlderLogLines(m.previewLog.lines, msg.lines)
	if len(newOlder) == 0 {
		m.previewLog.hasMoreHistory = false
		return m
	}

	// Prepend the older lines.
	m.previewLog.lines = append(newOlder, m.previewLog.lines...)

	// Cap total at previewLogMaxLines, trimming the oldest extras.
	if trimmed, drop := capLines(m.previewLog.lines, previewLogMaxLines); drop > 0 {
		m.previewLog.lines = trimmed
		m.previewLog.hasMoreHistory = false
	}

	// Advance the tail for the next fetch.
	m.previewLog.historyTail += previewLogHistoryBatch
	if m.previewLog.historyTail >= previewLogMaxLines {
		m.previewLog.hasMoreHistory = false
	}

	// fromBottom does not need adjustment: the scroll is bottom-anchored, so
	// prepending older lines increases the total but keeps the same newest
	// lines visible. The user can now scroll further up.

	return m
}

// maybeLoadMorePreviewHistory triggers a lazy history fetch when the user has
// scrolled to (or near) the top of the loaded buffer and there is more history
// to fetch. Returns (m, nil) when no fetch is needed.
func (m Model) maybeLoadMorePreviewHistory() (Model, tea.Cmd) {
	if !m.fullLogPreview || !m.previewLog.hasMoreHistory || m.previewLog.loadingHistory {
		return m, nil
	}

	ref, ok := m.selectedPodForLogPreview()
	if !ok || ref.key() != m.previewLog.podKey {
		return m, nil
	}

	// Compute the maximum fromBottom value using the same math as clampPreviewScroll.
	innerW := m.previewLogInnerWidth()
	colHeight := max(m.height-4, 3)
	if len(m.tabs) > 1 {
		colHeight--
	}
	bodyHeight := max(colHeight-1, 1)
	total := ui.PreviewLogPhysicalCount(m.previewLog.lines, innerW)
	maxFromBottom := max(total-bodyHeight, 0)

	if m.previewLog.fromBottom < maxFromBottom {
		// User is not at the top yet — don't fetch.
		return m, nil
	}

	// At the top: trigger fetch.
	m.previewLog.loadingHistory = true
	return m, m.fetchOlderPreviewLogs(ref, m.previewLog.historyTail+previewLogHistoryBatch)
}
