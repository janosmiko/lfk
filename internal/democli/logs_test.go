package democli

import (
	"bytes"
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/janosmiko/lfk/internal/logagg"
)

// TestRunLogs_RoundTripsThroughLogagg is the criterion-4 proof: generated log
// lines, once the app strips the pod prefix and timestamp (as
// logTopSample/logTopParseInto do), must parse as ProfileJSON and carry
// fields the Top view groups by (method, path, status, duration_ms).
func TestRunLogs_RoundTripsThroughLogagg(t *testing.T) {
	var buf bytes.Buffer
	args := []string{
		"web-7d8f9c6b5-9k2pl", "-n", "demo", "--context", "demo",
		"--all-containers=true", "--prefix", "--tail=30", "--timestamps",
	}

	if err := runLogs(t.Context(), args, &buf); err != nil {
		t.Fatalf("runLogs() error = %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 30 {
		t.Fatalf("got %d lines, want 30", len(lines))
	}

	sample := make([]string, len(lines))
	for i, line := range lines {
		sample[i] = stripPrefixAndTimestamp(t, line)
	}

	kind := logagg.DetectKind(sample)
	if kind != logagg.ProfileJSON {
		t.Fatalf("DetectKind() = %v, want %v", kind, logagg.ProfileJSON)
	}

	parser := logagg.ParserFor(kind)
	var saw4xx, saw5xx bool
	for _, body := range sample {
		f, ok := parser.Parse(body)
		if !ok {
			t.Fatalf("parser.Parse(%q) failed", body)
		}
		for _, key := range []string{logagg.FieldMethod, logagg.FieldPath, logagg.FieldStatus, logagg.FieldDurationMS} {
			if f[key] == "" {
				t.Errorf("line %q missing field %q after parse", body, key)
			}
		}
		if logagg.IsHTTPError(f) {
			if strings.HasPrefix(f[logagg.FieldStatus], "4") {
				saw4xx = true
			}
			if strings.HasPrefix(f[logagg.FieldStatus], "5") {
				saw5xx = true
			}
		}
	}

	// Build an aggregation the same way the app's Log Top view does, and
	// confirm the generated lines actually group.
	agg := logagg.NewAggregation([]string{logagg.FieldMethod, logagg.FieldPath}, nil, logagg.IsHTTPError)
	for _, body := range sample {
		if f, ok := parser.Parse(body); ok {
			agg.Add(f)
		}
	}
	if agg.Total() != 30 {
		t.Fatalf("aggregation total = %d, want 30", agg.Total())
	}
	if len(agg.Rows(logagg.SortReq)) == 0 {
		t.Fatalf("aggregation produced no rows")
	}
	if !saw4xx && !saw5xx {
		t.Logf("no 4xx/5xx in this 30-line sample; acceptable but worth widening tail if flaky")
	}
}

// stripPrefixAndTimestamp mirrors ui.StripPodPrefix + logagg.SplitTimestamp
// so the test exercises the exact transformation the app performs before
// handing a line to the parser.
func stripPrefixAndTimestamp(t *testing.T, line string) string {
	t.Helper()
	if strings.HasPrefix(line, "[") {
		_, rest, ok := strings.Cut(line, "] ")
		if !ok {
			t.Fatalf("line %q has a bracket prefix with no closing '] '", line)
		}
		line = rest
	}
	if _, rest, ok := logagg.SplitTimestamp(line); ok {
		return rest
	}
	t.Fatalf("line %q has no parseable leading RFC3339Nano timestamp", line)
	return ""
}

func TestRunLogs_DeterministicPerPod(t *testing.T) {
	run := func(target string) string {
		var buf bytes.Buffer
		args := []string{target, "-n", "demo", "--context", "demo", "--tail=10", "--timestamps"}
		if err := runLogs(t.Context(), args, &buf); err != nil {
			t.Fatalf("runLogs() error = %v", err)
		}
		// Timestamps are wall-clock and vary run to run; compare only the
		// JSON body (after the timestamp) for stability.
		var bodies []string
		for line := range strings.SplitSeq(strings.TrimRight(buf.String(), "\n"), "\n") {
			if _, rest, ok := logagg.SplitTimestamp(line); ok {
				bodies = append(bodies, rest)
			}
		}
		return strings.Join(bodies, "\n")
	}

	first := run("web-7d8f9c6b5-9k2pl")
	second := run("web-7d8f9c6b5-9k2pl")
	if first != second {
		t.Errorf("same pod produced different streams:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}

	other := run("web-7d8f9c6b5-x7bwn")
	if first == other {
		t.Errorf("different pods produced identical streams")
	}
}

func TestRunLogs_PrefixFormat(t *testing.T) {
	var buf bytes.Buffer
	args := []string{
		"web-7d8f9c6b5-9k2pl", "-n", "demo", "--context", "demo",
		"--all-containers=true", "--prefix", "--tail=1", "--timestamps",
	}
	if err := runLogs(t.Context(), args, &buf); err != nil {
		t.Fatalf("runLogs() error = %v", err)
	}
	line := strings.TrimRight(buf.String(), "\n")
	want := "[pod/web-7d8f9c6b5-9k2pl/app] "
	if !strings.HasPrefix(line, want) {
		t.Errorf("line = %q, want prefix %q", line, want)
	}
}

func TestRunLogs_ContainerFlagOmitsPrefix(t *testing.T) {
	var buf bytes.Buffer
	args := []string{
		"web-7d8f9c6b5-9k2pl", "-n", "demo", "--context", "demo",
		"-c", "web", "--tail=1", "--timestamps",
	}
	if err := runLogs(t.Context(), args, &buf); err != nil {
		t.Fatalf("runLogs() error = %v", err)
	}
	line := strings.TrimRight(buf.String(), "\n")
	if strings.HasPrefix(line, "[pod/") {
		t.Errorf("line = %q, want no bracket prefix when -c is set without --prefix", line)
	}
}

// TestRunLogs_FollowTerminatesOnContextCancel is the leak-guard proof:
// follow mode must return promptly when the caller's context is cancelled,
// and must not leave any goroutine of its own running afterward.
func TestRunLogs_FollowTerminatesOnContextCancel(t *testing.T) {
	baseline := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(t.Context())
	args := []string{
		"web-7d8f9c6b5-9k2pl", "-n", "demo", "--context", "demo",
		"-f", "--tail=0", "--timestamps",
	}

	done := make(chan error, 1)
	go func() {
		var buf bytes.Buffer
		done <- runLogs(ctx, args, &buf)
	}()

	// Give the goroutine a moment to enter the follow select loop, then cancel.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("runLogs() error = %v, want nil on context cancel", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runLogs() did not return within 2s of context cancellation")
	}

	// Allow the runtime a brief window to finish tearing down the goroutine's
	// stack before comparing counts.
	deadline := time.Now().Add(time.Second)
	for runtime.NumGoroutine() > baseline && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := runtime.NumGoroutine(); got > baseline {
		t.Errorf("goroutine count after cancel = %d, want <= baseline %d (leak)", got, baseline)
	}
}
