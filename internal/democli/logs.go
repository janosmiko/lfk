package democli

import (
	"bufio"
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"math/rand"
	"time"

	"github.com/janosmiko/lfk/internal/ui"
)

// defaultTailLines is how many lines to emit when the caller passed no
// --tail (kubectl's own default backlog size).
const defaultTailLines = 20

// demoLineInterval paces follow-mode line emission so a streamed session
// looks live instead of dumping instantly.
const demoLineInterval = 150 * time.Millisecond

// demoPaths and demoMethods are the synthetic request shapes mixed into the
// generated access log so Log Top has varied paths/methods to rank.
var demoPaths = []string{
	"/api/users", "/api/users/42", "/api/orders", "/api/orders/1183",
	"/api/products", "/api/products/7", "/healthz", "/readyz", "/api/cart", "/api/checkout",
}

var demoMethods = []string{"GET", "GET", "GET", "POST", "PUT", "DELETE"}

// runLogs emulates `kubectl logs`. Non-follow requests emit `tail` (or
// defaultTailLines) historical lines and return. Follow requests replay the
// same backlog, then keep emitting new lines on demoLineInterval until ctx is
// cancelled — mirroring a real `-f` stream that only ends when the caller
// tears down the process.
func runLogs(ctx context.Context, args []string, stdout io.Writer) error {
	a := parseLogArgs(args)
	pod := a.podName()
	rng := rand.New(rand.NewSource(podSeed(a.namespace, pod, a.container))) //nolint:gosec // deterministic demo data, not security-sensitive

	w := bufio.NewWriter(stdout)
	defer w.Flush() //nolint:errcheck

	tail := a.tail
	if tail < 0 {
		tail = defaultTailLines
	}

	now := time.Now()
	for i := range tail {
		ts := now.Add(-time.Duration(tail-i) * demoLineInterval)
		if err := writeLogLine(w, rng, a, pod, ts); err != nil {
			return err
		}
	}
	if !a.follow {
		return w.Flush()
	}
	if err := w.Flush(); err != nil {
		return err
	}

	ticker := time.NewTicker(demoLineInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case t := <-ticker.C:
			if err := writeLogLine(w, rng, a, pod, t); err != nil {
				return nil // consumer closed the read end. Not an error to report
			}
			if err := w.Flush(); err != nil {
				return nil
			}
		}
	}
}

// writeLogLine renders and writes one line, adding the "[pod/name/container]"
// prefix kubectl's --prefix flag adds when the app streams all containers.
//
// pod is attacker-controllable: the demo apply path builds it from
// metadata.name in arbitrary user-supplied YAML with no RFC1123 validation
// (see Client.ApplyManifest), so it can carry terminal escape sequences.
// Sanitized here (not just at the write side) as defense in depth, the same
// way listsummary and yamlblame guard their own sinks against
// cluster-controlled strings.
func writeLogLine(w io.Writer, rng *rand.Rand, a logArgs, pod string, ts time.Time) error {
	line := generateLine(rng, pod, ts)
	if a.prefix {
		container := a.container
		if container == "" {
			container = "app"
		}
		line = fmt.Sprintf("[pod/%s/%s] %s", ui.SanitizeTerminalText(pod), ui.SanitizeTerminalText(container), line)
	}
	_, err := fmt.Fprintln(w, line)
	return err
}

// podSeed derives a stable RNG seed from a pod's identity so the same pod
// always yields the same stream and different pods deterministically diverge.
func podSeed(namespace, pod, container string) int64 {
	h := fnv.New64a()
	_, _ = io.WriteString(h, namespace+"/"+pod+"/"+container)
	return int64(h.Sum64()) //nolint:gosec // truncation is fine, only used as a PRNG seed
}

// generateLine renders one synthetic JSON access-log line timestamped ts,
// prefixed with kubectl's own RFC3339Nano --timestamps format (which is what
// every real call site requests). The JSON body uses logagg's normalized
// field names directly (method/path/status/duration_ms/host) so
// logagg.ParserFor(logagg.ProfileJSON) groups it without any aliasing.
func generateLine(rng *rand.Rand, pod string, ts time.Time) string {
	method := demoMethods[rng.Intn(len(demoMethods))]
	path := demoPaths[rng.Intn(len(demoPaths))]
	status := statusOutcome(rng)
	durationMS := 2 + rng.Float64()*40
	switch {
	case status >= 500:
		durationMS += 200 + rng.Float64()*300
	case status >= 400:
		durationMS += 20
	}
	return fmt.Sprintf(
		`%s {"level":"info","method":%q,"path":%q,"status":%d,"duration_ms":%.2f,"host":%q,"msg":"request handled"}`,
		ts.Format(time.RFC3339Nano), method, path, status, durationMS, pod,
	)
}

// statusOutcome picks an HTTP status weighted mostly-200 with a handful of
// 4xx/5xx so the Log Top aggregation has real errors to rank.
func statusOutcome(rng *rand.Rand) int {
	switch roll := rng.Intn(100); {
	case roll < 85:
		return []int{200, 200, 200, 201, 204}[rng.Intn(5)]
	case roll < 95:
		return []int{400, 401, 404, 404}[rng.Intn(4)]
	default:
		return []int{500, 502, 503}[rng.Intn(3)]
	}
}
