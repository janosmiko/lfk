package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/janosmiko/lfk/internal/logger"
)

// netshootImage is the pinned image used for the ephemeral debug container.
//
// Pinned by tag rather than digest to keep the build self-contained; for
// hardened deployments swap to a digest pin (e.g.
// "nicolaka/netshoot@sha256:...") so registry tampering is detected.
// Bumping this image needs a security review — the container runs with
// NET_ADMIN/NET_RAW in the target pod's network namespace.
const netshootImage = "nicolaka/netshoot:v0.13"

type kubectlDebugBackend struct{}

// debugContainerCounter feeds the suffix on the per-capture debug-container
// name (lfk-trafcap-<n>). Atomic so concurrent Start calls don't collide on
// the same name within a single process lifetime; combined with the
// nanosecond timestamp prefix, collisions across lfk runs are also bounded.
var debugContainerCounter atomic.Uint64

// makeDebugContainerName produces a DNS-label-safe name lfk uses both for
// `kubectl debug -c` (so we can find the container later) and for `kubectl
// exec -c` (so we can terminate its tcpdump on stop). Format:
// `lfk-trafcap-<unix-nanos>-<counter>`. Truncated to 63 chars, lowercase,
// alphanumeric + dash.
func makeDebugContainerName(now time.Time) string {
	n := debugContainerCounter.Add(1)
	name := fmt.Sprintf("lfk-trafcap-%d-%d", now.UnixNano(), n)
	if len(name) > 63 {
		name = name[:63]
	}
	return name
}

// remoteCaptureKiller is the kubectl-exec helper invoked from Start's
// returned cancel func. Pulled out as a package-level variable so tests
// can stub it without spawning kubectl.
var remoteCaptureKiller = terminateRemoteCapture

func (kubectlDebugBackend) Start(ctx context.Context, req CaptureRequest) (io.ReadCloser, context.CancelFunc, *exec.Cmd, *stderrBuffer, error) {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("kubectl not in PATH: %w", err)
	}
	debugContainer := makeDebugContainerName(time.Now())
	cctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(cctx, kubectlPath, kubectlDebugArgv(req, debugContainer)...)
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, nil, nil, nil, fmt.Errorf("kubectl-debug stdout pipe: %w", err)
	}

	stderrBuf := &stderrBuffer{}
	cmd.Stderr = stderrBuf

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, nil, nil, nil, fmt.Errorf("kubectl-debug start: %w", err)
	}

	wrappedCancel := wrapCancelWithRemoteKill(req, kubectlPath, debugContainer, cancel)

	// kubectl debug occasionally emits informational messages on stdout
	// ("Defaulting debug container name to debugger-XXX.") before the
	// ephemeral container's stdout — i.e., our pcap stream — starts flowing.
	// Wrap the stream so we discard everything before the pcap magic so the
	// decoder doesn't trip on `Unknown magic 0x44656661` (ASCII "Defa").
	//
	// dumpPath: when the preamble overflows without a pcap magic, the skipper
	// writes the buffered bytes to a sibling file so the user can hexdump
	// kubectl's actual stdout for diagnosis (the in-overlay error truncates).
	dumpPath := ""
	if req.OutputDir != "" {
		dumpPath = filepath.Join(req.OutputDir, fmt.Sprintf("%s--%s--%s--debug-preamble.bin",
			safeName(req.Context), safeName(req.Namespace), safeName(req.PodName)))
	}
	return &pcapPreambleSkipper{rc: stdout, dumpPath: dumpPath}, wrappedCancel, cmd, stderrBuf, nil
}

// wrapCancelWithRemoteKill returns a context.CancelFunc that, on first
// invocation, dispatches the remote-kill goroutine before calling inner.
// Subsequent invocations just call inner — the sync.Once guard prevents
// duplicate kubectl-exec calls when Stop / StopAll fire close together
// (or when the watchdog also calls cancel after the user-initiated stop).
//
// Without this wrapping the ephemeral container keeps running after
// kubectl disconnects: containerd holds the stdout pipe open, drains it
// into /var/log/pods/.../0.log, and disk pressure builds up on every node
// we've ever captured from.
func wrapCancelWithRemoteKill(req CaptureRequest, kubectlPath, debugContainer string, inner context.CancelFunc) context.CancelFunc {
	var once sync.Once
	return func() {
		once.Do(func() {
			go remoteCaptureKiller(req, kubectlPath, debugContainer)
		})
		if inner != nil {
			inner()
		}
	}
}

// terminateRemoteCapture sends SIGTERM (with SIGKILL fallback) to pid 1 of
// the given debug container so the on-node tcpdump exits and the runtime
// stops capturing its stdout to /var/log/pods/.../0.log. Best-effort: a
// 5-second timeout caps the call and any error is logged but otherwise
// swallowed because (a) the container may already be gone, (b) tcpdump is
// also wrapped in `timeout 30m` for the case where this exec can't reach
// the cluster.
func terminateRemoteCapture(req CaptureRequest, kubectlPath, debugContainer string) {
	if debugContainer == "" || kubectlPath == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	args := []string{
		"exec",
		"-n", req.Namespace,
		"--context", req.Context,
		"-c", debugContainer,
		"pod/" + req.PodName,
		"--",
		"sh", "-c",
		// Send TERM, give tcpdump a moment to flush its final write, then
		// hard-kill if it didn't exit. `2>/dev/null || true` covers the
		// case where the process is already gone.
		"kill -TERM 1 2>/dev/null || true; sleep 1; kill -KILL 1 2>/dev/null || true",
	}
	cmd := exec.CommandContext(ctx, kubectlPath, args...)
	if err := cmd.Run(); err != nil {
		logger.Error("remote capture termination failed",
			"namespace", req.Namespace, "pod", req.PodName,
			"debugContainer", debugContainer, "error", err.Error())
	}
}

// kubectlDebugArgv builds the kubectl debug argv. Extracted so tests can
// verify without spawning kubectl.
//
// Notes:
//   - `--attach=true` is REQUIRED. kubectl debug's `--attach` defaults to
//     false unless `-i`/`--stdin` is set; without it, kubectl creates the
//     ephemeral container and exits cleanly, leaving our pcap reader
//     observing an immediate EOF on kubectl's stdout. We don't want `-i`
//     because it keeps stdin open on the container — irrelevant for tcpdump
//     and an extra resource we'd rather not hold.
//   - `-c <debugContainer>` pins the ephemeral container name we generate
//     ourselves so Stop / StopAll can `kubectl exec -c <name> -- kill 1`
//     to terminate the remote tcpdump. Without this, the runtime keeps
//     draining tcpdump's stdout to /var/log/pods/ after kubectl
//     disconnects, building up disk pressure on every node we capture
//     from.
//   - `--profile=netadmin` is required for tcpdump to open AF_PACKET. This
//     was added to kubectl in 1.30. Older versions surface "unknown flag:
//     --profile" via stderr; translateKubectlDebugErr maps that to a
//     friendly message.
//   - tcpdump runs inside `sh -c "sleep 1; exec timeout 30m tcpdump ..."`.
//     The leading sleep is REQUIRED to defeat kubectl-debug's attach race:
//     kubectl creates and starts the ephemeral container, and the
//     container's entrypoint (tcpdump) starts running BEFORE kubectl's
//     attach connection is established. Output produced before the attach
//     is discarded by the CRI runtime, so without the sleep the first ~24
//     bytes of pcap (the file header containing the magic, version and
//     linktype) are lost.
//     The `timeout 30m` is a safety net: if lfk crashes or otherwise can't
//     reach the cluster to send the kill exec, the container self-
//     terminates after 30 minutes regardless. Bounds worst-case disk
//     pressure on the node.
//   - The pcapPreambleSkipper handles kubectl's pre-pcap stdout messages
//     ("Defaulting debug container name to debugger-XXX.") so we don't
//     need a kubectl flag to suppress them.
func kubectlDebugArgv(req CaptureRequest, debugContainer string) []string {
	args := []string{
		"debug",
		"-n", req.Namespace,
		"--context", req.Context,
		"pod/" + req.PodName,
		"--image=" + netshootImage,
		"--attach=true",
		"-c", debugContainer,
	}
	if req.Container != "" {
		args = append(args, "--target="+req.Container)
	}
	args = append(args, "--profile=netadmin", "--", "sh", "-c", buildTcpdumpShellCmd(req))
	return args
}

// kubectlDebugAttachDelay is the inline `sleep` value (in seconds) that runs
// in the ephemeral container before tcpdump starts. See the kubectlDebugArgv
// notes for why this is required. 1 second is enough for local clusters; if
// users on slow remote clusters report missing-header failures, bumping this
// is the dial.
const kubectlDebugAttachDelay = 1

// kubectlDebugMaxCaptureDuration is the safety-net `timeout` wrapping tcpdump
// so an orphaned ephemeral container (lfk crashed, network partition,
// SIGKILL of lfk before StopAll runs) self-terminates after this many
// minutes instead of running forever and filling /var/log/pods on the node.
// Long enough that legitimate captures aren't cut short under normal use.
const kubectlDebugMaxCaptureDuration = "30m"

// buildTcpdumpShellCmd constructs the shell command that runs inside the
// ephemeral container. Layout:
//
//	sleep <N>; exec timeout <D> tcpdump -U -nn -i <iface> [-s <snap>] -w - [<bpf>]
//
// User-provided fields (Interface, BPFFilter) are passed through
// shellSingleQuote so shell metacharacters cannot break out of the argument
// they belong to. Numeric fields (SnapLen) bypass the quoter — they're
// emitted with strconv.Itoa, which only ever produces digits. The
// `timeout` value is a hard-coded constant from this package.
func buildTcpdumpShellCmd(req CaptureRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "sleep %d; exec timeout %s tcpdump -U -nn -i %s",
		kubectlDebugAttachDelay, kubectlDebugMaxCaptureDuration, shellSingleQuote(req.Interface))
	if req.SnapLen > 0 {
		fmt.Fprintf(&b, " -s %d", req.SnapLen)
	}
	b.WriteString(" -w -")
	if req.BPFFilter != "" {
		b.WriteString(" ")
		b.WriteString(shellSingleQuote(req.BPFFilter))
	}
	return b.String()
}

// shellSingleQuote returns s wrapped in POSIX single quotes, with any
// embedded single quote escaped via the standard `'\”` idiom. Callers can
// drop the result into an `sh -c` string with no risk of metacharacter
// expansion in the quoted argument.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// translateKubectlDebugErr maps known kubectl-debug stderr patterns to friendly
// error messages. Returns "" if no pattern matches (caller falls through to
// the raw error).
func translateKubectlDebugErr(stderr string) string {
	switch {
	case strings.Contains(stderr, "ephemeral containers are disabled"):
		return "Cluster has ephemeral containers disabled. Enable the EphemeralContainers feature gate."
	case strings.Contains(stderr, "unknown flag: --profile"):
		return "kubectl < 1.30: --profile=netadmin not supported. Upgrade kubectl to 1.30+."
	case strings.Contains(stderr, "forbidden") && strings.Contains(stderr, "netadmin"):
		return "Pod Security Admission denied netadmin profile in this namespace. Try kubeshark or capture from a different namespace."
	case strings.Contains(stderr, "Non-root user is configured") || strings.Contains(stderr, "capabilities granted by debug profile are not effective"):
		return "Target pod runs as non-root, so netadmin profile capabilities (NET_ADMIN/NET_RAW) are not effective and tcpdump can't open AF_PACKET. Try kubeshark instead."
	default:
		return ""
	}
}

// pcapPreambleSkipper wraps an io.ReadCloser and discards bytes until it sees
// a pcap magic number (libpcap or pcapng). This protects the decoder from
// kubectl debug's "Defaulting debug container name to debugger-XXX." message
// landing on stdout before the ephemeral container's pcap stream starts.
//
// Once the magic is found, all subsequent reads pass through unchanged (and
// the magic itself is preserved at the start of the output stream so
// pcapgo.NewReader reads a valid header).
//
// The buffer is bounded — if no magic appears in the first 64 KiB we give up
// and return io.EOF, surfacing as a "no pcap magic seen" error in the
// CaptureManager classifier.
type pcapPreambleSkipper struct {
	rc       io.ReadCloser
	found    bool
	overflow []byte // bytes already read past the magic, waiting to flush
	window   []byte // accumulated bytes waiting for a magic match; persists across Read calls
	// dumpPath, if non-empty, receives the buffered window when the preamble
	// overflows without a pcap magic. Lets the user hexdump kubectl's actual
	// stdout (which the in-overlay error message necessarily truncates).
	dumpPath string
}

const pcapPreambleMaxBytes = 64 * 1024

// known pcap magic numbers (4 bytes, all endians + libpcap+pcapng):
// libpcap microsecond: a1 b2 c3 d4 (big-endian) / d4 c3 b2 a1 (little-endian)
// libpcap nanosecond:  a1 b2 3c 4d / 4d 3c b2 a1
// pcapng section header block magic: 0a 0d 0d 0a
var pcapMagicNumbers = [][]byte{
	{0xa1, 0xb2, 0xc3, 0xd4},
	{0xd4, 0xc3, 0xb2, 0xa1},
	{0xa1, 0xb2, 0x3c, 0x4d},
	{0x4d, 0x3c, 0xb2, 0xa1},
	{0x0a, 0x0d, 0x0d, 0x0a},
}

func (p *pcapPreambleSkipper) Read(buf []byte) (int, error) {
	if p.found {
		if len(p.overflow) > 0 {
			n := copy(buf, p.overflow)
			p.overflow = p.overflow[n:]
			return n, nil
		}
		return p.rc.Read(buf)
	}

	// Accumulate bytes across Read calls. p.window persists on the receiver
	// so that if a caller issues several reads before the magic appears, the
	// already-consumed preamble is not lost.
	scratch := make([]byte, 4096)
	for {
		n, err := p.rc.Read(scratch)
		if n > 0 {
			p.window = append(p.window, scratch[:n]...)
		}
		if idx := findPcapMagic(p.window); idx >= 0 {
			p.found = true
			tail := p.window[idx:]
			p.window = nil
			out := copy(buf, tail)
			if out < len(tail) {
				p.overflow = make([]byte, len(tail)-out)
				copy(p.overflow, tail[out:])
			}
			return out, nil
		}
		if err != nil {
			return 0, err
		}
		if len(p.window) > pcapPreambleMaxBytes {
			dumpHint := p.dumpPreambleOnOverflow()
			return 0, fmt.Errorf("no pcap magic in first %d bytes of capture stream (preamble: %q)%s",
				pcapPreambleMaxBytes, summarizePreamble(p.window), dumpHint)
		}
	}
}

// findPcapMagic scans buf for any known magic and returns the offset of the
// first match, or -1 if none is found. Search bounded by len(buf)-3 since the
// magic is 4 bytes.
func findPcapMagic(buf []byte) int {
	for _, magic := range pcapMagicNumbers {
		if i := bytes.Index(buf, magic); i >= 0 {
			return i
		}
	}
	return -1
}

// summarizePreamble returns the first ~120 bytes of `buf` for use in error
// messages, with non-printable characters escaped. encoding/binary import
// kept for future structured magic detection.
func summarizePreamble(buf []byte) string {
	const cap = 120
	if len(buf) > cap {
		buf = buf[:cap]
	}
	out := make([]byte, 0, len(buf))
	for _, b := range buf {
		if b >= 0x20 && b < 0x7f {
			out = append(out, b)
		} else {
			out = append(out, '.')
		}
	}
	return string(out)
}

func (p *pcapPreambleSkipper) Close() error { return p.rc.Close() }

// dumpPreambleOnOverflow writes p.window to p.dumpPath (best-effort) and
// returns a "; saved to <path>" suffix for the error message. Failures to
// write are logged but don't block the error path — the in-overlay summary
// is the primary diagnostic.
func (p *pcapPreambleSkipper) dumpPreambleOnOverflow() string {
	logger.Error("pcap preamble overflow",
		"size", len(p.window),
		"head_text", string(p.window[:min(512, len(p.window))]))
	if p.dumpPath == "" {
		return ""
	}
	if err := os.WriteFile(p.dumpPath, p.window, 0o600); err != nil {
		logger.Error("preamble dump failed", "path", p.dumpPath, "error", err.Error())
		return ""
	}
	return "; saved to " + p.dumpPath
}
