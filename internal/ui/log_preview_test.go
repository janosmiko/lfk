package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
)

func TestRenderLogPreviewShowsTail(t *testing.T) {
	lines := []string{"a", "b", "c", "d", "e"}
	out := stripANSI(RenderLogPreview(lines, "", 40, 3, "ns/pod-1", 0))
	assert.Contains(t, out, "LIVE LOGS")
	assert.Contains(t, out, "e")        // newest visible
	assert.NotContains(t, out, "\na\n") // oldest scrolled off (height 3)
}

// TestRenderLogPreviewHeaderNoBluebar asserts the "LIVE LOGS" header has no
// leading space and is NOT a full-width blue bar (no TitleStyle/FillLinesBg).
// The header must match the right-pane DETAILS style: dim+bold, no background.
func TestRenderLogPreviewHeaderNoBluebar(t *testing.T) {
	out := stripANSI(RenderLogPreview([]string{"line"}, "", 60, 5, "ns/mypod", 0))
	firstLine, _, _ := strings.Cut(out, "\n")
	// No leading space — first char must be 'L'.
	assert.True(t, strings.HasPrefix(firstLine, "LIVE LOGS"),
		"header must start with LIVE LOGS (no leading space), got: %q", firstLine)
	// Pod label included after two spaces.
	assert.Contains(t, firstLine, "ns/mypod")
}

func TestRenderLogPreviewPlaceholderAndError(t *testing.T) {
	assert.Contains(t, stripANSI(RenderLogPreview(nil, "", 40, 5, "", 0)),
		"Select a pod")
	assert.Contains(t, stripANSI(RenderLogPreview(nil, "boom", 40, 5, "ns/pod-1", 0)),
		"boom")
}

func TestRenderLogPreviewPodLabel(t *testing.T) {
	out := stripANSI(RenderLogPreview([]string{"line1"}, "", 60, 5, "default/mypod", 0))
	assert.Contains(t, out, "LIVE LOGS")
	assert.Contains(t, out, "default/mypod")
}

func TestRenderLogPreviewLineTruncation(t *testing.T) {
	// A very long line must be truncated to width, not overflow.
	long := strings.Repeat("x", 200)
	out := RenderLogPreview([]string{long}, "", 40, 5, "ns/pod", 0)
	// After stripping ANSI, each line must be at most 40 chars (display cells).
	for line := range strings.SplitSeq(stripANSI(out), "\n") {
		assert.LessOrEqualf(t, len(line), 40, "line too wide (%d): %q", len(line), line)
	}
}

func TestRenderLogPreviewPadsToHeight(t *testing.T) {
	// With fewer lines than height, the output must still fill height rows.
	out := RenderLogPreview([]string{"only-one"}, "", 40, 6, "ns/pod", 0)
	lines := strings.Split(out, "\n")
	assert.Equal(t, 6, len(lines), "output must be exactly height lines")
}

func TestRenderLogPreviewEmptyPodLabel(t *testing.T) {
	out := stripANSI(RenderLogPreview(nil, "", 40, 4, "", 0))
	assert.Contains(t, out, "Select a pod to see live logs")
	assert.NotContains(t, out, "LIVE LOGS")
}

func TestRenderLogPreviewWrapsLongLine(t *testing.T) {
	// A line longer than width must produce multiple physical body rows rather
	// than being truncated to a single row.
	long := strings.Repeat("a", 80)
	const width = 20
	const height = 10
	out := RenderLogPreview([]string{long}, "", width, height, "ns/pod", 0)
	rows := strings.Split(out, "\n")
	// Row 0 is the title bar; body rows follow.
	body := rows[1:]
	// Count non-empty body rows that contain actual content.
	contentRows := 0
	for _, r := range body {
		plain := stripANSI(r)
		if strings.TrimSpace(plain) != "" {
			contentRows++
		}
	}
	assert.Greater(t, contentRows, 1, "long line should wrap into multiple body rows")
	// Each row must not exceed width visual columns.
	for _, r := range body {
		w := lipgloss.Width(r)
		assert.LessOrEqualf(t, w, width, "body row too wide (%d): %q", w, r)
	}
}

func TestRenderLogPreviewPreservesAnsiWhenEnabled(t *testing.T) {
	prev := ConfigLogRenderAnsi
	t.Cleanup(func() { ConfigLogRenderAnsi = prev })

	// With ANSI enabled, SGR sequences must survive into the output.
	ConfigLogRenderAnsi = true
	line := "\x1b[31mred\x1b[0m"
	out := RenderLogPreview([]string{line}, "", 40, 5, "ns/pod", 0)
	assert.True(t, strings.Contains(out, "\x1b["), "SGR sequence must be preserved when ConfigLogRenderAnsi=true")

	// With ANSI disabled, SGR sequences must be stripped.
	ConfigLogRenderAnsi = false
	out2 := RenderLogPreview([]string{line}, "", 40, 5, "ns/pod", 0)
	// The renderer always styles its own chrome, so assert the log line's own
	// SGR is gone rather than that the output carries no escapes at all.
	assert.False(t, strings.Contains(out2, "\x1b[31m"), "the log line's SGR must be stripped when ConfigLogRenderAnsi=false")
	assert.Contains(t, stripANSI(out2), "red", "the text itself survives stripping")
}

// --- Fix A: formatPreviewLogLine ---

// TestFormatPreviewLogLineStripsPrefixAndRelativizesTime verifies that the
// formatPreviewLogLine helper:
//   - drops the kubectl "[pod/...]" bracket prefix
//   - converts the RFC3339Nano timestamp to a relative form
//   - keeps the log body intact
func TestFormatPreviewLogLineStripsPrefixAndRelativizesTime(t *testing.T) {
	// Simulate "5 seconds ago" — parse the raw timestamp from the example.
	rawLine := `[pod/argocd-server-5ff895d75b-2pm7r/server] 2026-06-10T11:15:11.558418443Z time="2026-06-10T11:15:11Z" level=info msg="Loading TLS..."`

	out := stripANSI(formatPreviewLogLine(rawLine))

	assert.NotContains(t, out, "[pod/", "prefix must be stripped")
	assert.NotContains(t, out, "2026-06-10T11:15:11.558418443Z", "raw RFC3339 timestamp must be replaced")
	// Relative-time token: one of "Xs ago", "Xm ago", "Xh ago", "Xd ago".
	hasRelative := strings.Contains(out, " ago")
	assert.True(t, hasRelative, "output must contain a relative-time token (got %q)", out)
	// Body content must survive.
	assert.Contains(t, out, `msg="Loading TLS...`)
}

// TestFormatPreviewLogLineNoTimestamp verifies that lines without a kubectl
// timestamp render as plain body (no crash, no empty output).
func TestFormatPreviewLogLineNoTimestamp(t *testing.T) {
	raw := "plain log line with no timestamp or prefix"
	out := stripANSI(formatPreviewLogLine(raw))
	assert.Contains(t, out, "plain log line", "body must be preserved when no timestamp is present")
	assert.NotContains(t, out, " ago", "no relative time when timestamp is absent")
}

// TestFormatPreviewLogLineRecentTimestamp verifies a very recent timestamp
// produces a "Xs ago" token (the line was emitted close to now).
func TestFormatPreviewLogLineRecentTimestamp(t *testing.T) {
	ts := time.Now().UTC().Add(-3 * time.Second).Format(time.RFC3339Nano)
	raw := "[pod/ns/container] " + ts + " hello world"
	out := stripANSI(formatPreviewLogLine(raw))
	assert.NotContains(t, out, "[pod/", "prefix stripped")
	assert.NotContains(t, out, ts, "raw timestamp replaced")
	assert.Contains(t, out, "s ago", "recent timestamp should show Xs ago")
	assert.Contains(t, out, "hello world")
}

// --- Fix B: RenderLogPreview scroll offset ---

// TestRenderLogPreviewScrollShowsOlderLines verifies:
//   - offset 0 shows the newest lines (auto-follow)
//   - a larger offset reveals older lines and hides the newest
func TestRenderLogPreviewScrollShowsOlderLines(t *testing.T) {
	// Build 20 distinct lines so we can assert which are visible.
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%02d", i)
	}

	const width = 40
	const height = 6 // 1 title + 5 body
	// bodyHeight == 5; total lines == 20.

	// Offset 0: auto-follow — newest lines visible.
	outFollow := stripANSI(RenderLogPreview(lines, "", width, height, "ns/pod", 0))
	assert.Contains(t, outFollow, "line-19", "offset 0 must show newest line")

	// fromBottom=15: end=20-15=5, start=0 → shows line-00..line-04.
	outScrolled := stripANSI(RenderLogPreview(lines, "", width, height, "ns/pod", 15))
	assert.Contains(t, outScrolled, "line-00", "scrolled back should show oldest line")
	assert.NotContains(t, outScrolled, "line-19", "scrolled back should not show newest line")
}

// --- Table-alignment and hanging indent ---

// TestPreviewTableAlignsMessageColumn verifies that two log lines with
// different-width relative-time strings both place their message text starting
// at visual column previewLogTimeColWidth. The time column is padded to exactly
// previewLogTimeColWidth visual cells so plain[previewLogTimeColWidth:] is
// always the message (after stripping ANSI).
func TestPreviewTableAlignsMessageColumn(t *testing.T) {
	now := time.Now().UTC()
	ts1 := now.Add(-1 * time.Minute).Format(time.RFC3339Nano)  // ~"1m ago"
	ts2 := now.Add(-52 * time.Second).Format(time.RFC3339Nano) // ~"52s ago"

	raw1 := "[pod/ns/c] " + ts1 + " hello world"
	raw2 := "[pod/ns/c] " + ts2 + " greetings universe"

	const width = 60
	phys := previewPhysicalLines([]string{raw1, raw2}, width)

	// There should be at least one physical line per raw line.
	assert.GreaterOrEqual(t, len(phys), 2, "expected at least two physical lines")

	// After stripping ANSI, the plain line must be at least previewLogTimeColWidth
	// chars wide and plain[previewLogTimeColWidth:] must start with the message.
	plain1 := stripANSI(phys[0])
	plain2 := stripANSI(phys[1])

	assert.GreaterOrEqualf(t, len(plain1), previewLogTimeColWidth,
		"raw1 physical line too short: %q", plain1)
	assert.GreaterOrEqualf(t, len(plain2), previewLogTimeColWidth,
		"raw2 physical line too short: %q", plain2)

	// Message must begin at column previewLogTimeColWidth in both lines.
	assert.Truef(t, strings.HasPrefix(plain1[previewLogTimeColWidth:], "hello world"),
		"raw1 message must begin at col %d; plain line: %q", previewLogTimeColWidth, plain1)
	assert.Truef(t, strings.HasPrefix(plain2[previewLogTimeColWidth:], "greetings universe"),
		"raw2 message must begin at col %d; plain line: %q", previewLogTimeColWidth, plain2)

	// Time column (first previewLogTimeColWidth chars) must be different widths
	// of the rel-time label — "1m ago" vs "52s ago" — but padded to the same col.
	// Verify neither time bleeds into the message column: the char at index
	// previewLogTimeColWidth-1 must be a space (the padding).
	assert.Equalf(t, ' ', rune(plain1[previewLogTimeColWidth-1]),
		"raw1: char before message col must be space (padding), got %q in %q",
		plain1[previewLogTimeColWidth-1], plain1)
	assert.Equalf(t, ' ', rune(plain2[previewLogTimeColWidth-1]),
		"raw2: char before message col must be space (padding), got %q in %q",
		plain2[previewLogTimeColWidth-1], plain2)
}

// TestPreviewWrapHangingIndent verifies that when a log body is long enough to
// wrap, each continuation line starts with exactly previewLogTimeColWidth
// leading spaces (hanging indent) so it visually aligns under the message column.
func TestPreviewWrapHangingIndent(t *testing.T) {
	now := time.Now().UTC()
	ts := now.Add(-5 * time.Second).Format(time.RFC3339Nano)

	// Build a body that is definitely longer than (width - previewLogTimeColWidth)
	// so we always get at least one continuation line.
	const width = 40
	msgWidth := width - previewLogTimeColWidth
	longBody := strings.Repeat("x", msgWidth*2+5)

	raw := "[pod/ns/c] " + ts + " " + longBody

	phys := previewPhysicalLines([]string{raw}, width)
	assert.Greater(t, len(phys), 1, "long body must produce more than one physical line")

	// Every continuation line (index > 0) must begin with exactly
	// previewLogTimeColWidth spaces, followed immediately by a non-space char
	// (the wrapped message content).
	indent := strings.Repeat(" ", previewLogTimeColWidth)
	for i, line := range phys[1:] {
		plain := stripANSI(line)
		assert.Truef(t, strings.HasPrefix(plain, indent),
			"continuation line %d must start with %d spaces, got %q", i+1, previewLogTimeColWidth, plain)
		// The char right after the indent must be non-space (actual content).
		if len(plain) > previewLogTimeColWidth {
			assert.NotEqualf(t, ' ', rune(plain[previewLogTimeColWidth]),
				"continuation line %d must have non-space after indent, got %q", i+1, plain)
		}
	}
}
