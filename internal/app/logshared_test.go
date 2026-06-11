package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestKubectlPodLogArgs_Follow asserts the streaming variant carries the flags
// needed to survive init-container transitions (-f, --max-log-requests,
// --ignore-errors) plus the common all-containers/prefix/timestamps set.
func TestKubectlPodLogArgs_Follow(t *testing.T) {
	got := kubectlPodLogArgs("mypod", "myns", "myctx", true, 50, "")
	assert.Equal(t, []string{
		"logs", "-f", "mypod",
		"-n", "myns",
		"--context", "myctx",
		"--all-containers=true",
		"--prefix",
		"--max-log-requests=20",
		"--ignore-errors",
		"--tail=50",
		"--timestamps",
	}, got)
}

// TestKubectlPodLogArgs_Snapshot asserts the one-shot variant (lazy history
// fetch) omits -f and the follow-only flags but keeps --tail/--timestamps.
func TestKubectlPodLogArgs_Snapshot(t *testing.T) {
	got := kubectlPodLogArgs("mypod", "myns", "myctx", false, 200, "")
	assert.Equal(t, []string{
		"logs", "mypod",
		"-n", "myns",
		"--context", "myctx",
		"--all-containers=true",
		"--prefix",
		"--tail=200",
		"--timestamps",
	}, got)
}

// TestKubectlPodLogArgs_Container asserts the single-container variant scopes
// the stream with -c and drops the all-containers flags (--all-containers,
// --prefix, --max-log-requests) that only make sense for multi-stream tails.
func TestKubectlPodLogArgs_Container(t *testing.T) {
	got := kubectlPodLogArgs("mypod", "myns", "myctx", true, 50, "sidecar")
	assert.Equal(t, []string{
		"logs", "-f", "mypod",
		"-n", "myns",
		"--context", "myctx",
		"-c", "sidecar",
		"--ignore-errors",
		"--tail=50",
		"--timestamps",
	}, got)

	snapshot := kubectlPodLogArgs("mypod", "myns", "myctx", false, 200, "sidecar")
	assert.Equal(t, []string{
		"logs", "mypod",
		"-n", "myns",
		"--context", "myctx",
		"-c", "sidecar",
		"--tail=200",
		"--timestamps",
	}, snapshot)
}

// TestKubectlPodLogArgs_NoTail asserts a negative tail omits the --tail flag.
func TestKubectlPodLogArgs_NoTail(t *testing.T) {
	got := kubectlPodLogArgs("p", "n", "c", false, -1, "")
	assert.NotContains(t, got, "--tail=-1")
	for _, a := range got {
		assert.NotContains(t, a, "--tail", "no --tail flag when tail < 0")
	}
}

// TestMergeOlderLogLines_OverlapTrimsDuplicates: the fetched batch is the last
// N lines of the pod; the current buffer's oldest 3 lines appear partway
// through it. Only the genuinely-older prefix before the overlap is returned.
func TestMergeOlderLogLines_OverlapTrimsDuplicates(t *testing.T) {
	current := []string{"c1", "c2", "c3", "c4"}
	fetched := []string{"o1", "o2", "c1", "c2", "c3", "c4"}
	got := mergeOlderLogLines(current, fetched)
	assert.Equal(t, []string{"o1", "o2"}, got)
}

// TestMergeOlderLogLines_NoOverlapPrependsAll: when no overlap is found (logs
// rotated), the entire fetched batch is treated as older.
func TestMergeOlderLogLines_NoOverlapPrependsAll(t *testing.T) {
	current := []string{"c1", "c2", "c3"}
	fetched := []string{"x1", "x2", "x3", "x4"}
	got := mergeOlderLogLines(current, fetched)
	assert.Equal(t, fetched, got)
}

// TestMergeOlderLogLines_OverlapAtStartReturnsEmpty: when the first fetched
// line is already the oldest current line, there is nothing older to add.
func TestMergeOlderLogLines_OverlapAtStartReturnsEmpty(t *testing.T) {
	current := []string{"c1", "c2", "c3"}
	fetched := []string{"c1", "c2", "c3"}
	got := mergeOlderLogLines(current, fetched)
	assert.Empty(t, got)
}

// TestMergeOlderLogLines_SingleLineFallback: with fewer than 3 lines on either
// side, the single-line fallback matches current[0] in the fetched batch.
func TestMergeOlderLogLines_SingleLineFallback(t *testing.T) {
	current := []string{"c1"}
	fetched := []string{"o1", "o2", "c1"}
	got := mergeOlderLogLines(current, fetched)
	assert.Equal(t, []string{"o1", "o2"}, got)
}

// TestMergeOlderLogLines_EmptyInputs: empty fetched yields nothing; empty
// current with non-empty fetched prepends all (no overlap possible).
func TestMergeOlderLogLines_EmptyInputs(t *testing.T) {
	assert.Empty(t, mergeOlderLogLines([]string{"c1"}, nil))
	assert.Equal(t, []string{"o1", "o2"}, mergeOlderLogLines(nil, []string{"o1", "o2"}))
}

// TestCapLines_TrimsToMax asserts capLines keeps exactly the newest max lines
// and reports the drop count.
func TestCapLines_TrimsToMax(t *testing.T) {
	buf := []string{"a", "b", "c", "d", "e"}
	out, drop := capLines(buf, 2)
	assert.Equal(t, []string{"d", "e"}, out)
	assert.Equal(t, 3, drop)
}

// TestCapLines_NoTrimUnderMax is a no-op when the buffer already fits.
func TestCapLines_NoTrimUnderMax(t *testing.T) {
	buf := []string{"a", "b"}
	out, drop := capLines(buf, 5)
	assert.Equal(t, buf, out)
	assert.Equal(t, 0, drop)
}

// TestCapLines_DisabledWhenMaxNonPositive returns the buffer untouched.
func TestCapLines_DisabledWhenMaxNonPositive(t *testing.T) {
	buf := []string{"a", "b", "c"}
	out, drop := capLines(buf, 0)
	assert.Equal(t, buf, out)
	assert.Equal(t, 0, drop)
}
