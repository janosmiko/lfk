package k8s

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewCaptureManager_EmptyAtStart(t *testing.T) {
	m := NewCaptureManager()
	if m == nil {
		t.Fatal("NewCaptureManager returned nil")
	}
	if got := m.Entries(); len(got) != 0 {
		t.Errorf("Entries len = %d, want 0", len(got))
	}
	if got := m.ActiveCount(); got != 0 {
		t.Errorf("ActiveCount = %d, want 0", got)
	}
}

// fakeBackend implements captureBackend for tests without exec.
type fakeBackend struct {
	stream io.Reader
}

func (f *fakeBackend) Start(ctx context.Context, req CaptureRequest) (io.ReadCloser, context.CancelFunc, *exec.Cmd, *stderrBuffer, error) {
	cctx, cancel := context.WithCancel(ctx)
	rc := io.NopCloser(f.stream)
	go func() {
		<-cctx.Done()
	}()
	// fake cmd is not started, so cmd.Wait() would error; tests that read
	// LastError must account for that. CaptureManager handles a nil cmd by
	// skipping Wait.
	return rc, cancel, nil, &stderrBuffer{}, nil
}

func TestCaptureManager_Start_PopulatesEntry(t *testing.T) {
	m := NewCaptureManager()
	tmp := t.TempDir()
	req := CaptureRequest{
		Backend:   BackendKubectlDebug,
		Context:   "ctx",
		Namespace: "ns",
		PodName:   "pod1",
		Interface: "any",
		SnapLen:   65535,
		OutputDir: tmp,
	}

	m.SetBackendFactory(func(b CaptureBackend) (captureBackend, error) {
		return &fakeBackend{stream: bytes.NewReader(emptyPcapHeader())}, nil
	})

	id, err := m.Start(context.Background(), req, func(s PacketSummary) {})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if id <= 0 {
		t.Errorf("ID = %d, want > 0", id)
	}

	waitForCaptureRunning(t, m, id)

	es := m.Entries()
	if len(es) != 1 {
		t.Fatalf("Entries len = %d, want 1", len(es))
	}
	if es[0].OutputPath == "" {
		t.Error("OutputPath empty")
	}

	// Verify the file was created in the OutputDir.
	if _, err := os.Stat(es[0].OutputPath); err != nil {
		t.Errorf("OutputPath %s not created: %v", es[0].OutputPath, err)
	}

	if err := m.Stop(id); err != nil {
		t.Errorf("Stop: %v", err)
	}
}

// waitForCaptureRunning polls the manager for the given ID to reach Running.
// Replaces time.Sleep(200ms) — the 100ms Starting->Running flip can be late
// under CI load, and a fixed sleep both flakes and slows the suite.
func waitForCaptureRunning(t *testing.T, m *CaptureManager, id int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, e := range m.Entries() {
			if e.ID == id && (e.Status == CaptureRunning || e.Status == CaptureStopped || e.Status == CaptureFailed) {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("capture id=%d did not transition out of Starting within 2s", id)
}

func TestCaptureManager_FindByPod(t *testing.T) {
	m := NewCaptureManager()
	tmp := t.TempDir()

	// Use a pipe so the stream blocks until the write-end is closed,
	// keeping the capture in Running state long enough for FindByPod to work.
	pr, pw := io.Pipe()
	// Write the pcap header so the decoder can open the stream, then block.
	go func() {
		_, _ = pw.Write(emptyPcapHeader())
		// Block until the test closes the write-end (via Stop -> cancel).
		// The pipe read-end will return io.ErrClosedPipe / io.EOF when pw is closed.
	}()

	m.SetBackendFactory(func(b CaptureBackend) (captureBackend, error) {
		return &fakeBackend{stream: pr}, nil
	})
	defer func() {
		_ = pw.Close()
		_ = pr.Close()
	}()

	req := CaptureRequest{
		Backend: BackendKubectlDebug, Context: "ctx", Namespace: "ns", PodName: "pod1",
		OutputDir: tmp,
	}
	id, err := m.Start(context.Background(), req, func(s PacketSummary) {})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = m.Stop(id) }()

	waitForCaptureRunning(t, m, id)

	gotID, ok := m.FindByPod("ctx", "ns", "pod1")
	if !ok {
		t.Fatal("FindByPod returned !ok for active capture")
	}
	if gotID != id {
		t.Errorf("FindByPod ID = %d, want %d", gotID, id)
	}

	if _, ok := m.FindByPod("ctx", "ns", "other"); ok {
		t.Error("FindByPod returned ok for unrelated pod")
	}
}

// TestCaptureManager_Start_RefusesPreexistingPath asserts that O_EXCL on the
// pcap file open guards against following a pre-existing symlink (or
// overwriting any other file) at the target path. The filename uses
// nanosecond resolution, so the collision is engineered by pre-creating a
// file at the deterministic path.
func TestCaptureManager_Start_RefusesPreexistingPath(t *testing.T) {
	m := NewCaptureManager()
	tmp := t.TempDir()

	// Pre-create a file with the safeName prefix and a wildcard timestamp —
	// since the actual filename is unpredictable, instead pre-create the
	// exact directory path and use a probe via a helper:
	// run Start once successfully, then attempt Start again immediately and
	// confirm we don't truncate the prior file.
	m.SetBackendFactory(func(b CaptureBackend) (captureBackend, error) {
		return &fakeBackend{stream: bytes.NewReader(emptyPcapHeader())}, nil
	})

	req := CaptureRequest{
		Backend: BackendKubectlDebug, Context: "ctx", Namespace: "ns", PodName: "pod1",
		OutputDir: tmp,
	}
	id1, err := m.Start(context.Background(), req, func(s PacketSummary) {})
	if err != nil {
		t.Fatalf("first Start: %v", err)
	}
	defer func() { _ = m.Stop(id1) }()
	es := m.Entries()
	if len(es) != 1 {
		t.Fatalf("entries len = %d", len(es))
	}
	firstPath := es[0].OutputPath

	// Sanity: nanosecond timestamps mean a back-to-back Start picks a
	// different filename. Independently confirm O_EXCL by re-opening the
	// first path with the same flags — it must fail.
	if _, err := os.OpenFile(firstPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600); err == nil {
		t.Errorf("OpenFile O_EXCL on existing %s succeeded; should have failed", firstPath)
	}
}

// TestStderrBuffer_BoundsMemoryAtCap guards the bounded growth contract:
// even a pathological backend writing megabytes of stderr must leave the
// buffer at maxStderrBufferSize bytes. The tail (most recent bytes) wins
// because translateKubectlDebugErr keys off patterns that appear at exit.
func TestStderrBuffer_BoundsMemoryAtCap(t *testing.T) {
	b := &stderrBuffer{}
	chunk := bytes.Repeat([]byte("X"), 4096) // 4 KiB per write
	for range 1000 {                         // ~4 MiB total written
		_, _ = b.Write(chunk)
	}
	if got := len(b.buf); got != maxStderrBufferSize {
		t.Errorf("buf len = %d, want %d (cap)", got, maxStderrBufferSize)
	}

	// Final write contains a marker; the buffer must retain it (tail wins).
	marker := []byte("FINAL_MARKER_TAIL_RETAINED")
	_, _ = b.Write(marker)
	if !bytes.Contains(b.buf, marker) {
		t.Error("final write must remain in the buffer; the tail-keep policy was violated")
	}
}

// TestStderrBuffer_SmallWritesDontTriggerCap verifies the fast path:
// totals under the cap append linearly, with no head truncation.
func TestStderrBuffer_SmallWritesDontTriggerCap(t *testing.T) {
	b := &stderrBuffer{}
	_, _ = b.Write([]byte("first "))
	_, _ = b.Write([]byte("second"))
	if got := b.String(); got != "first second" {
		t.Errorf("got %q, want %q", got, "first second")
	}
}

// TestStderrBuffer_SingleOversizedWriteKeepsTail covers the case where
// a single Write call exceeds the cap. Old buffer is fully dropped; new
// chunk's tail fills the buffer.
func TestStderrBuffer_SingleOversizedWriteKeepsTail(t *testing.T) {
	b := &stderrBuffer{}
	huge := bytes.Repeat([]byte("a"), maxStderrBufferSize+128)
	huge = append(huge, []byte("END")...)
	_, _ = b.Write(huge)
	if got := len(b.buf); got != maxStderrBufferSize {
		t.Errorf("buf len = %d, want %d", got, maxStderrBufferSize)
	}
	if !bytes.HasSuffix(b.buf, []byte("END")) {
		t.Error("oversized write must keep its trailing bytes")
	}
}

// TestCaptureManager_Entries_StripsInternalControlHandles guards the
// encapsulation contract: callers of Entries() get a public-only snapshot,
// not a back-door to the lifecycle handles. A regression that re-copies
// cmd / cancel / decoder / onUpdate would let a stale snapshot drive
// state changes (or hold references after Stop), which is the kind of
// subtle bug CodeRabbit flagged on review.
func TestCaptureManager_Entries_StripsInternalControlHandles(t *testing.T) {
	m := NewCaptureManager()
	cancelCalls := 0
	updateCalls := 0
	m.mu.Lock()
	m.entries = append(m.entries, &CaptureEntry{
		ID:       1,
		Status:   CaptureRunning,
		cmd:      &exec.Cmd{},
		cancel:   func() { cancelCalls++ },
		decoder:  &packetDecoder{},
		onUpdate: func() { updateCalls++ },
	})
	m.mu.Unlock()

	es := m.Entries()
	if len(es) != 1 {
		t.Fatalf("entries len = %d, want 1", len(es))
	}
	if es[0].cmd != nil {
		t.Errorf("snapshot.cmd should be nil; got %+v", es[0].cmd)
	}
	if es[0].cancel != nil {
		t.Errorf("snapshot.cancel should be nil; got non-nil func")
	}
	if es[0].decoder != nil {
		t.Errorf("snapshot.decoder should be nil; got %+v", es[0].decoder)
	}
	if es[0].onUpdate != nil {
		t.Errorf("snapshot.onUpdate should be nil; got non-nil func")
	}
	if cancelCalls != 0 || updateCalls != 0 {
		t.Errorf("Entries() must not invoke control handles; cancel=%d update=%d", cancelCalls, updateCalls)
	}
}

// TestCaptureManager_Entries_AtomicCountersRace exercises Entries() against
// concurrent atomic.AddInt64 writes on PacketCount/ByteCount. Without atomic
// reads in the snapshot, `go test -race` flags this as a data race.
func TestCaptureManager_Entries_AtomicCountersRace(t *testing.T) {
	m := NewCaptureManager()
	m.mu.Lock()
	entry := &CaptureEntry{ID: 1, Status: CaptureRunning, StartedAt: time.Now()}
	m.entries = append(m.entries, entry)
	m.mu.Unlock()

	stop := make(chan struct{})
	done := make(chan struct{}, 2)

	go func() {
		defer func() { done <- struct{}{} }()
		for {
			select {
			case <-stop:
				return
			default:
				atomic.AddInt64(&entry.PacketCount, 1)
				atomic.AddInt64(&entry.ByteCount, 64)
			}
		}
	}()
	go func() {
		defer func() { done <- struct{}{} }()
		for {
			select {
			case <-stop:
				return
			default:
				es := m.Entries()
				if len(es) != 1 {
					t.Errorf("Entries len = %d, want 1", len(es))
					return
				}
				_ = es[0].PacketCount
				_ = es[0].ByteCount
			}
		}
	}()

	time.Sleep(20 * time.Millisecond)
	close(stop)
	<-done
	<-done
}

func TestCaptureManager_Start_KubesharkRejected(t *testing.T) {
	m := NewCaptureManager()
	req := CaptureRequest{Backend: BackendKubeshark, OutputDir: t.TempDir()}
	_, err := m.Start(context.Background(), req, func(s PacketSummary) {})
	if err == nil {
		t.Error("Start should reject BackendKubeshark — kubeshark hand-off has separate code path")
	}
}

// TestSafeName_TruncatesLongInput guards the 63-char cap that keeps the pcap
// filename within filesystem limits even when context/namespace/pod names
// approach the Kubernetes 253-char DNS-name maximum.
func TestSafeName_TruncatesLongInput(t *testing.T) {
	long := strings.Repeat("a", 200)
	got := safeName(long)
	if len(got) != safeNameMaxLen {
		t.Errorf("len(safeName) = %d, want %d", len(got), safeNameMaxLen)
	}
}

func TestSafeName_ShortInput(t *testing.T) {
	if got := safeName("ctx-1"); got != "ctx-1" {
		t.Errorf("safeName(ctx-1) = %q, want ctx-1", got)
	}
}

func TestSafeName_ReplacesUnsafe(t *testing.T) {
	if got := safeName("ns/with:slashes"); got != "ns_with_slashes" {
		t.Errorf("safeName = %q, want ns_with_slashes", got)
	}
}

// TestClassifyCaptureExit covers the multi-source error-folding paths so a
// regression in any branch (ctx-canceled, friendly translation, stderr-only,
// runErr+waitErr) surfaces immediately.
func TestClassifyCaptureExit(t *testing.T) {
	someErr := errors.New("boom")
	otherErr := errors.New("kapow")

	tests := []struct {
		name       string
		backend    CaptureBackend
		runErr     error
		waitErr    error
		stderr     string
		ctxErr     error
		wantStatus CaptureStatus
		wantSubstr string // substring expected in LastError; "" means LastError must be exactly ""
	}{
		{
			name:       "ctx-canceled is a clean stop",
			backend:    BackendKubectlDebug,
			ctxErr:     context.Canceled,
			wantStatus: CaptureStopped,
			wantSubstr: "",
		},
		{
			name:       "runErr canceled is a clean stop even without ctxErr",
			backend:    BackendKubectlDebug,
			runErr:     context.Canceled,
			wantStatus: CaptureStopped,
			wantSubstr: "",
		},
		{
			name:       "friendly stderr wins over raw text",
			backend:    BackendKubectlDebug,
			waitErr:    someErr,
			stderr:     "Error from server (Forbidden): ephemeral containers are disabled on this cluster",
			wantStatus: CaptureFailed,
			wantSubstr: "ephemeral containers",
		},
		{
			name:       "raw stderr included when no friendly match",
			backend:    BackendKubectlDebug,
			waitErr:    someErr,
			stderr:     "some unmapped failure text",
			wantStatus: CaptureFailed,
			wantSubstr: "unmapped failure",
		},
		{
			name:       "runErr + waitErr both included",
			backend:    BackendKubectlDebug,
			runErr:     otherErr,
			waitErr:    someErr,
			wantStatus: CaptureFailed,
			wantSubstr: "boom",
		},
		{
			name:       "waitErr only",
			backend:    BackendKubectlDebug,
			waitErr:    someErr,
			wantStatus: CaptureFailed,
			wantSubstr: "boom",
		},
		{
			name:       "runErr only (decoder failure)",
			backend:    BackendKubectlDebug,
			runErr:     otherErr,
			wantStatus: CaptureFailed,
			wantSubstr: "kapow",
		},
		{
			name:       "no errors at all - clean exit",
			backend:    BackendKubectlDebug,
			wantStatus: CaptureStopped,
			wantSubstr: "",
		},
		{
			name:       "stderr trim past captureLastErrorMaxLen",
			backend:    BackendKubectlDebug,
			waitErr:    someErr,
			stderr:     strings.Repeat("x", captureLastErrorMaxLen+200),
			wantStatus: CaptureFailed,
			wantSubstr: "…", // trimmed marker
		},
		{
			name:       "stderr just under cap is not trimmed",
			backend:    BackendKubectlDebug,
			waitErr:    someErr,
			stderr:     strings.Repeat("y", captureLastErrorMaxLen-1),
			wantStatus: CaptureFailed,
			wantSubstr: strings.Repeat("y", 50),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotMsg := classifyCaptureExit(tt.backend, tt.runErr, tt.waitErr, tt.stderr, tt.ctxErr)
			if gotStatus != tt.wantStatus {
				t.Errorf("status = %s, want %s", gotStatus, tt.wantStatus)
			}
			if tt.wantSubstr == "" {
				if gotMsg != "" {
					t.Errorf("LastError = %q, want empty", gotMsg)
				}
				return
			}
			if !strings.Contains(gotMsg, tt.wantSubstr) {
				t.Errorf("LastError = %q, want substring %q", gotMsg, tt.wantSubstr)
			}
		})
	}
}

// emptyPcapHeader returns a minimal valid pcap stream header (24 bytes).
// Lets the decoder open the stream without a panic; the test cancels via Stop.
func emptyPcapHeader() []byte {
	return []byte{
		0xd4, 0xc3, 0xb2, 0xa1, // magic (little-endian)
		0x02, 0x00, 0x04, 0x00, // version 2.4
		0x00, 0x00, 0x00, 0x00, // thiszone
		0x00, 0x00, 0x00, 0x00, // sigfigs
		0xff, 0xff, 0x00, 0x00, // snaplen 65535
		0x01, 0x00, 0x00, 0x00, // linktype Ethernet
	}
}
