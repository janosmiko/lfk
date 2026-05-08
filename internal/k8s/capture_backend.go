package k8s

import (
	"context"
	"io"
	"os/exec"
	"sync"
)

// captureBackend is implemented by kubectl-debug (the only streaming backend).
// Kubeshark hand-off does NOT implement this interface — see capture_backend_kubeshark.go.
type captureBackend interface {
	// Start spawns the capture process. Returns:
	//   - the pcap byte stream (caller closes via cancel),
	//   - a cancel func that terminates the capture,
	//   - the *exec.Cmd for diagnostics + lifecycle (CaptureManager calls cmd.Wait),
	//   - a *stderrBuffer that accumulates the child's stderr so CaptureManager
	//     can surface it as LastError on a non-zero exit,
	//   - or an error.
	Start(ctx context.Context, req CaptureRequest) (
		stream io.ReadCloser,
		cancel context.CancelFunc,
		cmd *exec.Cmd,
		stderr *stderrBuffer,
		err error,
	)
}

// maxStderrBufferSize caps the in-memory tail of a backend's stderr that
// the buffer will retain. We only need enough bytes to (a) feed the
// translateKubectlDebugErr pattern matchers and (b) populate the trimmed
// LastError shown in the overlay (captureLastErrorMaxLen = 240 chars). A
// pathological backend that spews unbounded stderr (a kubectl misconfig
// loop, a tcpdump emitting "X errors" forever) would otherwise grow the
// buffer for the entire capture lifetime.
const maxStderrBufferSize = 8 * 1024

// stderrBuffer is a goroutine-safe stderr sink shared by all streaming
// capture backends. The child process writes via the io.Writer interface;
// CaptureManager reads via String() once cmd.Wait returns. Bytes past
// maxStderrBufferSize are dropped from the head — keeping the most-recent
// tail is the right tradeoff because translateKubectlDebugErr keys off
// patterns that appear at exit time and the LastError display also shows
// the trailing slice.
type stderrBuffer struct {
	mu  sync.Mutex
	buf []byte
}

func (s *stderrBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Fast path for the common case (< cap).
	if len(s.buf)+len(p) <= maxStderrBufferSize {
		s.buf = append(s.buf, p...)
		return len(p), nil
	}
	// Either the new chunk alone is bigger than cap, or appending it would
	// overflow. Reserve cap bytes; populate with the trailing window of
	// (old | new). io.Writer contract: report bytes accepted, not bytes
	// retained — return len(p) so the producer doesn't see partial-write.
	combined := len(s.buf) + len(p)
	tailStart := combined - maxStderrBufferSize
	out := make([]byte, maxStderrBufferSize)
	if tailStart < len(s.buf) {
		// Old buffer contributes its trailing slice [tailStart:].
		copyN := copy(out, s.buf[tailStart:])
		copy(out[copyN:], p)
	} else {
		// Old buffer is entirely discarded; p contributes its trailing slice.
		copy(out, p[tailStart-len(s.buf):])
	}
	s.buf = out
	return len(p), nil
}

func (s *stderrBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return string(s.buf)
}
