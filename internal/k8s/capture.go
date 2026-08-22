package k8s

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/paths"
)

// CaptureBackend identifies which capture engine the user picked.
type CaptureBackend string

const (
	BackendKubectlDebug CaptureBackend = "kubectl-debug"
	BackendKubeshark    CaptureBackend = "kubeshark"
)

// CaptureStatus is the lifecycle state of a single capture.
type CaptureStatus string

const (
	CaptureStarting CaptureStatus = "Starting"
	CaptureRunning  CaptureStatus = "Running"
	CaptureStopped  CaptureStatus = "Stopped"
	CaptureFailed   CaptureStatus = "Failed"
)

// CaptureRequest is the user's intent: what to capture and how.
type CaptureRequest struct {
	Backend   CaptureBackend // BackendKubectlDebug only (Kubeshark has its own code path)
	Context   string
	Namespace string
	PodName   string
	Container string // optional. Passed as --target to kubectl debug
	Interface string // default "any"
	SnapLen   int    // default 65535
	BPFFilter string // optional
	OutputDir string // default: "captures" in the lfk state directory
}

// CaptureEntry is one running or completed capture.
type CaptureEntry struct {
	ID         int
	Request    CaptureRequest
	Status     CaptureStatus
	StartedAt  time.Time
	StoppedAt  *time.Time
	OutputPath string
	LastError  string

	// The decoder goroutine writes these outside m.mu, so a plain copy of the
	// entry would tear a counter. atomic.Int64 embeds noCopy, which makes
	// `copy := *entry` a go vet copylocks failure instead of a silent tear.
	// CaptureSnapshot is the shape callers read.
	PacketCount atomic.Int64
	ByteCount   atomic.Int64

	cmd     *exec.Cmd
	cancel  context.CancelFunc
	decoder *packetDecoder
}

// cloneTime copies the pointed-to timestamp so a caller mutating
// snapshot.StoppedAt cannot reach back into the manager's entry.
func cloneTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	c := *t
	return &c
}

// CaptureSnapshot is one entry as callers outside the manager see it: the
// counters resolved to plain values and the lifecycle handles left behind.
type CaptureSnapshot struct {
	ID          int
	Request     CaptureRequest
	Status      CaptureStatus
	StartedAt   time.Time
	StoppedAt   *time.Time
	PacketCount int64
	ByteCount   int64
	OutputPath  string
	LastError   string
}

// CaptureManager tracks running captures across the lfk session.
type CaptureManager struct {
	mu      sync.Mutex
	entries []*CaptureEntry
	nextID  int
	// listener carries one token per change. The manager sends while holding
	// m.mu, so no SetUpdateListener can retire the listener between the choice
	// of it and the delivery to it.
	listener updateListener

	// backendFactory builds backend implementations. Defaults to
	// defaultBackendFactory. Tests inject fakes via SetBackendFactory.
	backendFactory func(CaptureBackend) (captureBackend, error)
}

// NewCaptureManager returns an empty manager.
func NewCaptureManager() *CaptureManager {
	return &CaptureManager{nextID: 1, backendFactory: defaultBackendFactory}
}

// SetBackendFactory replaces the backend factory. Intended for tests so they
// can swap in a fakeBackend without spawning kubectl.
func (m *CaptureManager) SetBackendFactory(f func(CaptureBackend) (captureBackend, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.backendFactory = f
}

// SetUpdateListener registers ch as the single listener for entry changes. The
// returned channel closes when a later call replaces this listener, so the
// superseded one stops waiting instead of leaking.
func (m *CaptureManager) SetUpdateListener(ch chan<- struct{}) <-chan struct{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.listener.setLocked(ch)
}

// notifyLocked delivers one update to the current listener. Callers must hold
// m.mu.
func (m *CaptureManager) notifyLocked() {
	m.listener.notifyLocked()
}

// Entries returns a snapshot of all entries (running and stopped). The
// internal control handles (cmd, cancel, decoder) are left behind so callers
// can't accidentally drive lifecycle from a stale copy.
func (m *CaptureManager) Entries() []CaptureSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]CaptureSnapshot, len(m.entries))
	for i, e := range m.entries {
		out[i] = CaptureSnapshot{
			ID:          e.ID,
			Request:     e.Request,
			Status:      e.Status,
			StartedAt:   e.StartedAt,
			StoppedAt:   cloneTime(e.StoppedAt),
			PacketCount: e.PacketCount.Load(),
			ByteCount:   e.ByteCount.Load(),
			OutputPath:  e.OutputPath,
			LastError:   e.LastError,
		}
	}
	return out
}

// ActiveCount returns the number of entries in Starting or Running state.
func (m *CaptureManager) ActiveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, e := range m.entries {
		if e.Status == CaptureRunning || e.Status == CaptureStarting {
			n++
		}
	}
	return n
}

// FindByPod returns the ID of the most-recent active capture matching the pod, if any.
func (m *CaptureManager) FindByPod(kubectx, ns, pod string) (int, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.entries {
		if e.Status != CaptureRunning && e.Status != CaptureStarting {
			continue
		}
		if e.Request.Context == kubectx && e.Request.Namespace == ns && e.Request.PodName == pod {
			return e.ID, true
		}
	}
	return 0, false
}

// defaultBackendFactory returns the production backend implementation.
//
// Stays a free function rather than a method so tests can also mint a manager
// directly without calling NewCaptureManager.
func defaultBackendFactory(b CaptureBackend) (captureBackend, error) {
	switch b {
	case BackendKubectlDebug:
		return kubectlDebugBackend{}, nil
	default:
		return nil, fmt.Errorf("unsupported backend %q for streaming capture", b)
	}
}

// captureRunningFlipDelay is the wait before a Starting entry flips to
// Running. atomic.Int64 (nanoseconds) so tests can shrink it without racing
// the concurrent flip-goroutine readers.
var captureRunningFlipDelay atomic.Int64

func init() {
	captureRunningFlipDelay.Store(int64(100 * time.Millisecond))
}

// Start spawns a streaming capture. Not used for kubeshark hand-off.
func (m *CaptureManager) Start(ctx context.Context, req CaptureRequest, onPacket func(PacketSummary)) (int, error) {
	if req.Backend == BackendKubeshark {
		return 0, fmt.Errorf("kubeshark hand-off does not use CaptureManager")
	}
	if req.OutputDir == "" {
		req.OutputDir = defaultCaptureDir()
	}
	if req.Interface == "" {
		req.Interface = "any"
	}
	if req.SnapLen == 0 {
		req.SnapLen = 65535
	}

	if err := os.MkdirAll(req.OutputDir, 0o700); err != nil {
		return 0, fmt.Errorf("creating output dir: %w", err)
	}
	// Nanosecond suffix prevents same-second collisions when a user restarts
	// quickly. O_EXCL refuses to follow a pre-existing symlink at the target
	// path (avoiding TOCTOU writes through an attacker-placed link) and also
	// flags any remaining collision instead of silently truncating.
	outPath := filepath.Join(req.OutputDir, fmt.Sprintf("%s--%s--%s--%s.pcap",
		safeName(req.Context), safeName(req.Namespace), safeName(req.PodName),
		time.Now().UTC().Format("20060102-150405.000000000")))
	f, err := os.OpenFile(outPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return 0, fmt.Errorf("opening pcap file: %w", err)
	}

	m.mu.Lock()
	factory := m.backendFactory
	m.mu.Unlock()
	if factory == nil {
		factory = defaultBackendFactory
	}
	backend, err := factory(req.Backend)
	if err != nil {
		_ = f.Close()
		_ = os.Remove(outPath)
		return 0, err
	}

	stream, cancel, cmd, stderrBuf, err := backend.Start(ctx, req)
	if err != nil {
		_ = f.Close()
		_ = os.Remove(outPath)
		return 0, fmt.Errorf("backend start: %w", err)
	}

	m.mu.Lock()
	id := m.nextID
	m.nextID++
	entry := &CaptureEntry{
		ID:         id,
		Request:    req,
		Status:     CaptureStarting,
		StartedAt:  time.Now(),
		OutputPath: outPath,
		cmd:        cmd,
		cancel:     cancel,
	}
	d := &packetDecoder{
		file:        f,
		packetCount: &entry.PacketCount,
		byteCount:   &entry.ByteCount,
		onPacket:    onPacket,
	}
	entry.decoder = d
	m.entries = append(m.entries, entry)
	m.notifyLocked()
	m.mu.Unlock()

	// Decoder goroutine: tees pcap to file + decodes. After the stream
	// closes, reap the child process and surface any non-zero exit + stderr
	// as LastError so the user sees WHY the capture stopped.
	go func() {
		runErr := d.Run(ctx, stream)
		_ = stream.Close()
		_ = f.Close()

		// Reap the child. Wait blocks until the process truly exits even
		// if Run already returned EOF (which can happen if the backend
		// crashed before producing pcap header bytes).
		var waitErr error
		if cmd != nil {
			waitErr = cmd.Wait()
		}
		stderrTxt := ""
		if stderrBuf != nil {
			stderrTxt = strings.TrimSpace(stderrBuf.String())
		}

		now := time.Now()
		m.mu.Lock()
		entry.Status, entry.LastError = classifyCaptureExit(req.Backend, runErr, waitErr, stderrTxt, ctx.Err())
		entry.StoppedAt = &now
		m.notifyLocked()
		m.mu.Unlock()
	}()

	// Flip Starting -> Running once the decoder has had a chance to consume the pcap header.
	go func() {
		select {
		case <-time.After(time.Duration(captureRunningFlipDelay.Load())):
			m.markCaptureRunning(entry)
		case <-ctx.Done():
		}
	}()

	return id, nil
}

// markCaptureRunning flips a still-starting capture to Running. A capture the
// watchdog already finished stays as it is, and reports nothing: that terminal
// transition announced itself.
func (m *CaptureManager) markCaptureRunning(entry *CaptureEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if entry.Status != CaptureStarting {
		return
	}
	entry.Status = CaptureRunning
	m.notifyLocked()
}

// classifyCaptureExit folds the multiple error sources the watchdog sees
// (decoder error, child Wait error, stderr buffer, parent ctx cancellation)
// into a single (Status, LastError) pair the renderer can show.
//
// Backend-specific stderr translation (e.g., kubectl-debug's "ephemeral
// containers disabled" pattern) runs here so the friendly message ends up
// in LastError instead of the raw exit-code text.
func classifyCaptureExit(backend CaptureBackend, runErr, waitErr error, stderrTxt string, ctxErr error) (CaptureStatus, string) {
	// User-cancelled (Stop button or lfk shutdown) — not a failure.
	if errors.Is(ctxErr, context.Canceled) || errors.Is(runErr, context.Canceled) {
		return CaptureStopped, ""
	}

	// Translate stderr first. If a backend-specific message matches, prefer
	// it over the raw exit-code text.
	friendly := ""
	if backend == BackendKubectlDebug && stderrTxt != "" {
		friendly = translateKubectlDebugErr(stderrTxt)
	}

	exited := waitErr != nil
	failed := exited
	// Decoder errors that aren't simple EOF count as failures too.
	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		failed = true
	}

	if !failed {
		return CaptureStopped, ""
	}

	switch {
	case friendly != "":
		return CaptureFailed, friendly
	case stderrTxt != "":
		// Trim and bound the stderr text so a flood doesn't break layout AND
		// to limit incidental leakage of kubeconfig paths or API server URLs
		// that kubectl errors sometimes embed. The user can re-run with
		// kubectl directly to see the full message.
		// Redact before the trim: trimming a secret first leaves a tail no
		// pattern matches any more.
		txt := logger.Redact(stderrTxt)
		if len(txt) > captureLastErrorMaxLen {
			txt = "…" + txt[len(txt)-captureLastErrorMaxLen:]
		}
		// Prepend a hint about which side failed when both errors exist.
		switch {
		case runErr != nil && waitErr != nil:
			return CaptureFailed, fmt.Sprintf("%s\n%s\n%s", waitErr.Error(), runErr.Error(), txt)
		case waitErr != nil:
			return CaptureFailed, fmt.Sprintf("%s\n%s", waitErr.Error(), txt)
		default:
			return CaptureFailed, fmt.Sprintf("%s\n%s", runErr.Error(), txt)
		}
	case waitErr != nil && runErr != nil:
		return CaptureFailed, strings.TrimSpace(waitErr.Error() + ": " + runErr.Error())
	case waitErr != nil:
		return CaptureFailed, waitErr.Error()
	case runErr != nil:
		return CaptureFailed, runErr.Error()
	default:
		return CaptureFailed, "capture exited with no output"
	}
}

// Stop cancels a running capture by ID. Closes pcap file via the watchdog goroutine.
func (m *CaptureManager) Stop(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.entries {
		if e.ID == id {
			if e.cancel != nil {
				e.cancel()
			}
			return nil
		}
	}
	return fmt.Errorf("capture id %d not found", id)
}

// StopAll cancels every entry. Used on lfk shutdown.
func (m *CaptureManager) StopAll() {
	m.mu.Lock()
	for _, e := range m.entries {
		if e.cancel != nil {
			e.cancel()
		}
	}
	m.mu.Unlock()
}

// defaultCaptureDir returns the user-default capture directory.
// Returns "" when the state directory cannot be resolved.
func defaultCaptureDir() string {
	dir, err := paths.StateDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "captures")
}

// captureLastErrorMaxLen bounds the stderr substring stored in
// CaptureEntry.LastError. Reduced from the original 800 to limit incidental
// disclosure of kubeconfig paths / API server addresses kubectl sometimes
// embeds in its error output.
const captureLastErrorMaxLen = 240

// safeNameMaxLen bounds each filename component to the Kubernetes name
// segment limit. Combined into the pcap filename four times (ctx, ns, pod,
// timestamp), this keeps the filename safely under filesystem name limits.
const safeNameMaxLen = 63

// safeName replaces non-alphanumeric / non-[-_.] runes with underscores and
// truncates to safeNameMaxLen so the resulting filename is portable across
// filesystems and stays under the 255-byte filename limit common on Linux.
func safeName(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if len(out) >= safeNameMaxLen {
			break
		}
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			out = append(out, r)
		case r == '-' || r == '_' || r == '.':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}
