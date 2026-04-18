package app

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// genTimestampedLine returns a single log line with the given RFC3339
// timestamp. Keeps test bodies short — every histogram test fixture
// boils down to "a slice of these strings".
func genTimestampedLine(ts time.Time, body string) string {
	return ts.Format(time.RFC3339Nano) + " " + body
}

// TestBuildLogHistogramBucketStepLadder exercises the step-picking
// ladder: the smallest ladder step must be chosen such that
// totalRange / step <= numBuckets-1. Each sub-test pins a known
// time range and asserts the picked step.
func TestBuildLogHistogramBucketStepLadder(t *testing.T) {
	base := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name        string
		span        time.Duration
		numBuckets  int
		wantStepLow time.Duration // step must be >= this …
		wantStepHi  time.Duration // … and <= this. Allows for ladder-step variance.
	}{
		{
			name:        "1 minute span fits in 1s or 5s buckets",
			span:        1 * time.Minute,
			numBuckets:  60,
			wantStepLow: 1 * time.Second,
			wantStepHi:  5 * time.Second,
		},
		{
			name:        "10 minute span needs at least 30s or 1m buckets",
			span:        10 * time.Minute,
			numBuckets:  20,
			wantStepLow: 30 * time.Second,
			wantStepHi:  1 * time.Minute,
		},
		{
			name:        "1 hour span fits in 5m or 15m buckets",
			span:        1 * time.Hour,
			numBuckets:  20,
			wantStepLow: 5 * time.Minute,
			wantStepHi:  15 * time.Minute,
		},
		{
			name:        "1 day span fits in 1h or 6h buckets",
			span:        24 * time.Hour,
			numBuckets:  40,
			wantStepLow: 1 * time.Hour,
			wantStepHi:  6 * time.Hour,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lines := []string{
				genTimestampedLine(base, "start"),
				genTimestampedLine(base.Add(tc.span), "end"),
			}
			h := BuildLogHistogram(lines, tc.numBuckets)
			assert.GreaterOrEqual(t, h.BucketStep, tc.wantStepLow,
				"step too small for span; would overflow bucket count")
			assert.LessOrEqual(t, h.BucketStep, tc.wantStepHi,
				"step larger than necessary — should pick smallest from ladder")
			// The number of buckets used must never exceed numBuckets.
			assert.LessOrEqual(t, len(h.Counts), tc.numBuckets,
				"bucket count must be capped at numBuckets")
			// And cover at least one bucket so the strip is renderable.
			assert.GreaterOrEqual(t, len(h.Counts), 1)
		})
	}
}

// TestBuildLogHistogramCounts feeds three timestamps that each fall
// into a known bucket and asserts the counts slice matches exactly.
// Uses a fixed step (5s) so the bucketing math is hand-verifiable.
func TestBuildLogHistogramCounts(t *testing.T) {
	base := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)

	// Create lines spread across a 25-second range so 5 buckets of
	// 5s each are needed. With numBuckets=5, the ladder picks 5s.
	lines := []string{
		genTimestampedLine(base, "a"),                     // bucket 0
		genTimestampedLine(base.Add(2*time.Second), "b"),  // bucket 0
		genTimestampedLine(base.Add(6*time.Second), "c"),  // bucket 1
		genTimestampedLine(base.Add(12*time.Second), "d"), // bucket 2
		genTimestampedLine(base.Add(13*time.Second), "e"), // bucket 2
		genTimestampedLine(base.Add(14*time.Second), "f"), // bucket 2
		genTimestampedLine(base.Add(20*time.Second), "g"), // bucket 4 (last)
	}

	h := BuildLogHistogram(lines, 5)
	require.Equal(t, 5*time.Second, h.BucketStep, "expected 5s ladder step for 20s range / 5 buckets")

	// 5 buckets covering 0–25s. The 20s line falls into bucket 4.
	expectedCounts := []int{2, 1, 3, 0, 1}
	assert.Equal(t, expectedCounts, h.Counts)
}

// TestBuildLogHistogramSkipsLinesWithoutTimestamp confirms that lines
// missing a parseable RFC3339 prefix are silently dropped from the
// counts. The histogram is built only from the lines that DO have a
// timestamp, and the bucket boundaries reflect their range, not the
// position of the dropped lines.
func TestBuildLogHistogramSkipsLinesWithoutTimestamp(t *testing.T) {
	base := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)

	lines := []string{
		"plain log without timestamp",                     // dropped
		genTimestampedLine(base, "a"),                     // bucket 0
		"another plain line",                              // dropped
		genTimestampedLine(base.Add(10*time.Second), "b"), // bucket N
		"yet another",                                     // dropped
	}

	h := BuildLogHistogram(lines, 10)
	totalCounted := 0
	for _, c := range h.Counts {
		totalCounted += c
	}
	assert.Equal(t, 2, totalCounted, "only timestamped lines should be counted")
}

// TestBuildLogHistogramCursorBucket verifies CursorBucket returns the
// bucket index that contains the cursor line's timestamp. We pick a
// line that we know falls into bucket 2 and assert.
func TestBuildLogHistogramCursorBucket(t *testing.T) {
	base := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)

	lines := []string{
		genTimestampedLine(base, "a"),                     // bucket 0
		genTimestampedLine(base.Add(5*time.Second), "b"),  // bucket 1
		genTimestampedLine(base.Add(12*time.Second), "c"), // bucket 2
		genTimestampedLine(base.Add(20*time.Second), "d"), // bucket 4
	}
	h := BuildLogHistogram(lines, 5)
	require.Equal(t, 5*time.Second, h.BucketStep)

	// Line c is at +12s → bucket index 12/5 = 2.
	assert.Equal(t, 2, h.CursorBucket(lines[2]),
		"+12s line should land in bucket 2")

	// Line a is at +0 → bucket 0.
	assert.Equal(t, 0, h.CursorBucket(lines[0]))

	// A plain line (no timestamp) returns -1.
	assert.Equal(t, -1, h.CursorBucket("plain line"))

	// A timestamp before the histogram start clamps to -1 so the
	// renderer doesn't draw a phantom indicator below bucket 0.
	earlier := genTimestampedLine(base.Add(-1*time.Hour), "way before")
	assert.Equal(t, -1, h.CursorBucket(earlier))
}

// TestBuildLogHistogramEmpty exercises three flavours of empty input:
// nil, zero-length, and "all lines lack timestamps". Each should
// return the zero LogHistogram, and CursorBucket on it must return -1.
func TestBuildLogHistogramEmpty(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		h := BuildLogHistogram(nil, 80)
		assert.Empty(t, h.Counts)
		assert.Equal(t, time.Duration(0), h.BucketStep)
		assert.Equal(t, -1, h.CursorBucket("anything"))
	})

	t.Run("zero numBuckets", func(t *testing.T) {
		h := BuildLogHistogram([]string{"2026-04-17T12:00:00Z hi"}, 0)
		assert.Empty(t, h.Counts)
	})

	t.Run("no timestamped lines", func(t *testing.T) {
		h := BuildLogHistogram([]string{"plain", "also plain"}, 80)
		assert.Empty(t, h.Counts)
		assert.Equal(t, time.Duration(0), h.BucketStep)
		assert.Equal(t, -1, h.CursorBucket("anything"))
	})
}

// TestBuildLogHistogramKubectlPrefix confirms that lines wearing the
// kubectl `--prefix` envelope (e.g. `[pod/foo container] 2026-... msg`)
// are still bucketed correctly — the envelope must not interfere with
// the timestamp parser used by the histogram builder.
func TestBuildLogHistogramKubectlPrefix(t *testing.T) {
	base := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	lines := []string{
		fmt.Sprintf("[pod/foo container] %s message a", base.Format(time.RFC3339Nano)),
		fmt.Sprintf("[pod/foo container] %s message b", base.Add(1*time.Second).Format(time.RFC3339Nano)),
	}
	h := BuildLogHistogram(lines, 5)
	totalCounted := 0
	for _, c := range h.Counts {
		totalCounted += c
	}
	assert.Equal(t, 2, totalCounted, "prefix-wrapped lines should still be counted")
}

// TestFormatBucketStepHumanReadable spot-checks the chip formatter:
// each ladder step should round-trip into a one-or-two-character
// string consistent with the user's mental model ("5m", "1h", "1d").
func TestFormatBucketStepHumanReadable(t *testing.T) {
	cases := []struct {
		step time.Duration
		want string
	}{
		{1 * time.Second, "1s"},
		{5 * time.Second, "5s"},
		{30 * time.Second, "30s"},
		{1 * time.Minute, "1m"},
		{15 * time.Minute, "15m"},
		{1 * time.Hour, "1h"},
		{6 * time.Hour, "6h"},
		{24 * time.Hour, "1d"},
	}
	for _, tc := range cases {
		h := LogHistogram{BucketStep: tc.step, Counts: []int{1}}
		assert.Equal(t, tc.want, h.FormatBucketStep(),
			"unexpected chip formatting for step %s", tc.step)
	}

	// Zero histogram renders no chip.
	assert.Equal(t, "", LogHistogram{}.FormatBucketStep())
}
