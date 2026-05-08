package k8s

import (
	"io"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestKubectlDebugBackend_Argv_FullSpec(t *testing.T) {
	req := CaptureRequest{
		Context: "ctx", Namespace: "ns", PodName: "pod1",
		Container: "app", Interface: "any", SnapLen: 65535, BPFFilter: "port 443",
	}
	want := []string{
		"debug",
		"-n", "ns",
		"--context", "ctx",
		"pod/pod1",
		"--image=" + netshootImage,
		"--attach=true",
		"-c", "lfk-trafcap-test",
		"--target=app",
		"--profile=netadmin",
		"--",
		"sh", "-c",
		"sleep 1; exec timeout 30m tcpdump -U -nn -i 'any' -s 65535 -w - 'port 443'",
	}
	got := kubectlDebugArgv(req, "lfk-trafcap-test")
	if !reflect.DeepEqual(got, want) {
		t.Errorf("argv =\n%v\nwant\n%v", got, want)
	}
}

func TestKubectlDebugBackend_Argv_NoFilterNoContainer(t *testing.T) {
	req := CaptureRequest{Context: "ctx", Namespace: "ns", PodName: "pod1", Interface: "any", SnapLen: 4096}
	got := kubectlDebugArgv(req, "lfk-trafcap-test")
	shellCmd := got[len(got)-1]

	// -s <snaplen> must be emitted so the UI control actually constrains tcpdump.
	if !strings.Contains(shellCmd, " -s 4096 ") {
		t.Errorf("shell cmd missing `-s 4096`; got %q", shellCmd)
	}
	// No --target=... should appear when Container is empty.
	for _, a := range got {
		if strings.HasPrefix(a, "--target=") {
			t.Errorf("unexpected --target arg %q when Container is empty", a)
		}
	}
	// Trailing position: the shell cmd should end in `-w -` (no BPF appended).
	if !strings.HasSuffix(shellCmd, "-w -") {
		t.Errorf("shell cmd should end with `-w -` when no BPF filter; got %q", shellCmd)
	}
}

// TestKubectlDebugBackend_Argv_AlwaysAttaches guards a regression where
// dropping --attach=true makes kubectl create the ephemeral container and
// exit without streaming pcap, surfacing as "opening pcap stream: EOF" in
// the user-visible LastError.
func TestKubectlDebugBackend_Argv_AlwaysAttaches(t *testing.T) {
	req := CaptureRequest{Context: "ctx", Namespace: "ns", PodName: "pod1", Interface: "any"}
	got := kubectlDebugArgv(req, "lfk-trafcap-test")
	for _, a := range got {
		if a == "--attach=true" || a == "-i" || a == "--stdin" {
			return
		}
	}
	t.Errorf("argv must include --attach=true (or equivalent) so kubectl streams pcap; got %v", got)
}

// TestKubectlDebugBackend_Argv_AttachRaceDelay guards the leading sleep that
// defeats the kubectl-debug attach race (tcpdump's pcap file header is
// dropped by the CRI runtime if it lands before kubectl's attach
// connection establishes, breaking the decoder).
func TestKubectlDebugBackend_Argv_AttachRaceDelay(t *testing.T) {
	req := CaptureRequest{Context: "ctx", Namespace: "ns", PodName: "pod1", Interface: "any"}
	got := kubectlDebugArgv(req, "lfk-trafcap-test")
	shellCmd := got[len(got)-1]
	if !strings.HasPrefix(shellCmd, "sleep ") {
		t.Errorf("shell cmd must begin with `sleep N` to defeat the attach race; got %q", shellCmd)
	}
	if !strings.Contains(shellCmd, "exec timeout") {
		t.Errorf("shell cmd should `exec timeout <D> tcpdump` after the sleep; got %q", shellCmd)
	}
}

// TestKubectlDebugBackend_Argv_NamedDebugContainer guards that we always
// pass `-c <debugContainer>` so Stop / StopAll can find the container later
// to terminate its tcpdump via `kubectl exec -c <name> -- kill 1`. Without
// the named container the on-node tcpdump keeps running after kubectl
// disconnects, and the runtime keeps draining stdout to /var/log/pods,
// building disk pressure (the bug users hit on long-lived clusters).
func TestKubectlDebugBackend_Argv_NamedDebugContainer(t *testing.T) {
	req := CaptureRequest{Context: "ctx", Namespace: "ns", PodName: "pod1", Interface: "any"}
	got := kubectlDebugArgv(req, "lfk-trafcap-42")
	for i, a := range got {
		if a == "-c" && i+1 < len(got) && got[i+1] == "lfk-trafcap-42" {
			return
		}
	}
	t.Errorf("argv must include `-c lfk-trafcap-42` so Stop can target the debug container; got %v", got)
}

// TestKubectlDebugBackend_Argv_TimeoutSafetyNet guards the `timeout 30m`
// wrapper around tcpdump. This is a safety net for the case where lfk
// crashes / loses cluster connectivity before it can run the kubectl-exec
// kill — without it the orphan tcpdump runs forever, the runtime keeps
// logging it to disk, and the node fills up.
func TestKubectlDebugBackend_Argv_TimeoutSafetyNet(t *testing.T) {
	req := CaptureRequest{Context: "ctx", Namespace: "ns", PodName: "pod1", Interface: "any"}
	got := kubectlDebugArgv(req, "lfk-trafcap-test")
	shellCmd := got[len(got)-1]
	if !strings.Contains(shellCmd, "timeout 30m tcpdump") {
		t.Errorf("shell cmd must wrap tcpdump in `timeout 30m` so orphans self-terminate; got %q", shellCmd)
	}
}

// TestWrapCancelWithRemoteKill_DispatchesOnce stubs remoteCaptureKiller and
// asserts that calling the wrapped cancel triggers the remote-kill goroutine
// exactly once even on repeated invocations. The wrapped cancel must also
// always call the inner cancel (idempotent — safe to call many times).
func TestWrapCancelWithRemoteKill_DispatchesOnce(t *testing.T) {
	prev := remoteCaptureKiller
	defer func() { remoteCaptureKiller = prev }()

	var killCount atomic.Int32
	killed := make(chan struct{}, 4)
	remoteCaptureKiller = func(req CaptureRequest, kubectlPath, debugContainer string) {
		killCount.Add(1)
		killed <- struct{}{}
	}

	innerCalls := 0
	innerCancel := func() { innerCalls++ }

	cancel := wrapCancelWithRemoteKill(
		CaptureRequest{Namespace: "ns", PodName: "pod", Context: "ctx"},
		"/usr/bin/kubectl",
		"lfk-trafcap-7",
		innerCancel,
	)

	// First call must dispatch the kill goroutine and call inner cancel.
	cancel()
	select {
	case <-killed:
	case <-time.After(time.Second):
		t.Fatal("wrapped cancel did not dispatch remote kill within 1s")
	}
	if innerCalls != 1 {
		t.Errorf("inner cancel calls = %d, want 1 after first cancel", innerCalls)
	}

	// Second + third calls must NOT dispatch additional kills (sync.Once).
	cancel()
	cancel()
	// Inner cancel still gets every call so context cancellation is
	// idempotent at all callers — the only de-duplicated side effect is
	// the remote kubectl-exec.
	if innerCalls != 3 {
		t.Errorf("inner cancel calls = %d, want 3 after triple cancel", innerCalls)
	}
	// Drain a moment to give any extra goroutine a chance to fire.
	select {
	case <-killed:
		t.Errorf("remote kill fired more than once; sync.Once guard regressed")
	case <-time.After(50 * time.Millisecond):
	}
	if got := killCount.Load(); got != 1 {
		t.Errorf("killCount = %d, want 1", got)
	}
}

func TestMakeDebugContainerName_DNSCompliant(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 6, time.UTC)
	got := makeDebugContainerName(now)
	if len(got) > 63 {
		t.Errorf("name length = %d, want <= 63 (DNS-label limit)", len(got))
	}
	if !strings.HasPrefix(got, "lfk-trafcap-") {
		t.Errorf("name %q must start with the lfk-trafcap- prefix", got)
	}
	for _, r := range got {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
		if !ok {
			t.Errorf("name %q contains invalid DNS-label char %q", got, r)
		}
	}
}

// TestShellSingleQuote_EscapesMetacharacters verifies user-provided fields
// (BPF filter, interface) cannot break out of the shell-quoted argument and
// run additional commands inside the privileged ephemeral container.
func TestShellSingleQuote_EscapesMetacharacters(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"port 443", `'port 443'`},
		{"", "''"},
		{`it's`, `'it'\''s'`},
		{"$(rm -rf /)", `'$(rm -rf /)'`},
		{"a\nb", "'a\nb'"},
		{`"; rm -rf /; echo "`, `'"; rm -rf /; echo "'`},
		{`'; rm -rf /; echo '`, `''\''; rm -rf /; echo '\'''`},
	}
	for _, tt := range tests {
		if got := shellSingleQuote(tt.in); got != tt.want {
			t.Errorf("shellSingleQuote(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestKubectlDebugBackend_Argv_BPFFilterShellEscaped guards that a BPF
// filter containing shell metacharacters survives shell quoting and isn't
// re-interpreted by the inner sh.
func TestKubectlDebugBackend_Argv_BPFFilterShellEscaped(t *testing.T) {
	req := CaptureRequest{
		Context: "ctx", Namespace: "ns", PodName: "pod1",
		Interface: "any", BPFFilter: "tcp port 443 and host 1.2.3.4",
	}
	got := kubectlDebugArgv(req, "lfk-trafcap-test")
	shellCmd := got[len(got)-1]
	if !strings.HasSuffix(shellCmd, " 'tcp port 443 and host 1.2.3.4'") {
		t.Errorf("BPF filter must be single-quoted at the end of the shell cmd; got %q", shellCmd)
	}

	// Adversarial filter: should be quoted, not interpreted.
	req.BPFFilter = `port 443; rm -rf /`
	got = kubectlDebugArgv(req, "lfk-trafcap-test")
	shellCmd = got[len(got)-1]
	if !strings.HasSuffix(shellCmd, " 'port 443; rm -rf /'") {
		t.Errorf("adversarial BPF filter not properly quoted; got %q", shellCmd)
	}
}

func TestKubectlDebugBackend_Argv_NetshootImagePinned(t *testing.T) {
	req := CaptureRequest{Context: "ctx", Namespace: "ns", PodName: "pod1", Interface: "any"}
	got := kubectlDebugArgv(req, "lfk-trafcap-test")
	var imageArg string
	for _, a := range got {
		if len(a) >= len("--image=") && a[:len("--image=")] == "--image=" {
			imageArg = a[len("--image="):]
			break
		}
	}
	if imageArg == "" {
		t.Fatal("argv missing --image= flag")
	}
	// Image must include either an explicit tag or digest. Reject the implicit
	// ":latest" that results from a bare repo name.
	hasTag := false
	for i := len(imageArg) - 1; i >= 0; i-- {
		if imageArg[i] == ':' || imageArg[i] == '@' {
			hasTag = true
			break
		}
		if imageArg[i] == '/' {
			break
		}
	}
	if !hasTag {
		t.Errorf("image %q is not pinned (no :tag or @digest)", imageArg)
	}
}

// TestPcapPreambleSkipper_SplitReads ensures the skipper accumulates preamble
// bytes across multiple Read calls — a regression guard for the previous bug
// where `window` was a per-call local and a Read split before the magic
// dropped the earlier bytes.
func TestPcapPreambleSkipper_SplitReads(t *testing.T) {
	pcapMagic := []byte{0xd4, 0xc3, 0xb2, 0xa1, 0x02, 0x00, 0x04, 0x00}
	preamble := []byte("Defaulting debug container name to debugger-abcd.\n")

	chunks := [][]byte{
		preamble[:20], // first half of preamble
		preamble[20:], // rest of preamble
		pcapMagic,     // magic appears in third Read
		{0x99, 0xaa},  // a couple of post-magic bytes
	}
	rc := io.NopCloser(&chunkedReader{chunks: chunks})
	skipper := &pcapPreambleSkipper{rc: rc}

	// Read repeatedly with a small buffer; magic should appear and the bytes
	// after the magic should pass through cleanly.
	var got []byte
	tmp := make([]byte, 4)
	for range 10 {
		n, err := skipper.Read(tmp)
		got = append(got, tmp[:n]...)
		if err != nil {
			break
		}
		if len(got) >= len(pcapMagic)+2 {
			break
		}
	}
	want := append(append([]byte{}, pcapMagic...), 0x99, 0xaa)
	if len(got) < len(want) || !bytesEqual(got[:len(want)], want) {
		t.Errorf("got %x, want prefix %x", got, want)
	}
}

// chunkedReader returns each chunk on its own Read call.
type chunkedReader struct {
	chunks [][]byte
	i      int
}

func (c *chunkedReader) Read(buf []byte) (int, error) {
	if c.i >= len(c.chunks) {
		return 0, io.EOF
	}
	n := copy(buf, c.chunks[c.i])
	c.i++
	return n, nil
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestTranslateKubectlDebugErr(t *testing.T) {
	tests := []struct {
		stderr   string
		wantSubs string // substring expected in friendly message; "" means we expect empty (no translation)
	}{
		{"error: ephemeral containers are disabled on this cluster", "ephemeral containers"},
		{"error: unknown flag: --profile", "kubectl < 1.30"},
		{"Error from server (Forbidden): pods \"pod1\" is forbidden: violates PodSecurity \"baseline\": netadmin", "Pod Security Admission"},
		{"some unrelated error", ""},
	}
	for _, tt := range tests {
		got := translateKubectlDebugErr(tt.stderr)
		if tt.wantSubs == "" {
			if got != "" {
				t.Errorf("stderr=%q: got friendly=%q, want empty", tt.stderr, got)
			}
			continue
		}
		if got == "" || !strings.Contains(got, tt.wantSubs) {
			t.Errorf("stderr=%q: friendly=%q, want substring %q", tt.stderr, got, tt.wantSubs)
		}
	}
}
