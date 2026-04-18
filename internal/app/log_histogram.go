package app

import (
	"time"
)

// histogramBucketLadder enumerates the candidate bucket widths that
// BuildLogHistogram can pick from. The smallest step where
// totalRange / step <= numBuckets wins, so the histogram never has
// more columns than the renderer asked for. Hand-picked to match the
// units a human would naturally talk about ("5 seconds", "1 minute"),
// rather than arbitrary values.
var histogramBucketLadder = []time.Duration{
	1 * time.Second,
	5 * time.Second,
	15 * time.Second,
	30 * time.Second,
	1 * time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	30 * time.Minute,
	1 * time.Hour,
	6 * time.Hour,
	24 * time.Hour,
}

// LogHistogram summarises a buffer of log lines by time bucket.
//
// All three fields are zero-valued when the input contained no
// parseable timestamps; callers can treat IsZero() on Start (or a
// nil Counts slice) as "no histogram available, render nothing".
type LogHistogram struct {
	BucketStep time.Duration // width of each bucket
	Start      time.Time     // lower bound of bucket 0
	Counts     []int         // len == numBuckets; counts[i] is line count in [Start+i*Step, Start+(i+1)*Step)
}

// BuildLogHistogram scans lines, extracts RFC3339 timestamps, and
// returns a histogram with numBuckets columns spanning the parse-
// succeeded time range. Lines without a parseable timestamp are
// skipped. Returns a zero LogHistogram when no lines have timestamps.
//
// BucketStep is chosen from histogramBucketLadder to be the smallest
// step where totalRange / step <= numBuckets. When the time range is
// smaller than the smallest ladder step (1s) we still pick 1s — sub-
// second ranges produce a single-bucket histogram, which is correct
// even if visually compressed.
func BuildLogHistogram(lines []string, numBuckets int) LogHistogram {
	if numBuckets <= 0 || len(lines) == 0 {
		return LogHistogram{}
	}

	// Walk once to find the time range. We deliberately do NOT keep the
	// per-line timestamps around — that would double the memory cost on
	// large buffers, and bucketing only needs (min, max, then a second
	// pass to count). The second pass amortises the parse cost.
	var (
		minTime time.Time
		maxTime time.Time
		seen    int
	)
	for _, line := range lines {
		ts, _, ok := parseLogLineTimestamp(line)
		if !ok {
			continue
		}
		if seen == 0 || ts.Before(minTime) {
			minTime = ts
		}
		if seen == 0 || ts.After(maxTime) {
			maxTime = ts
		}
		seen++
	}
	if seen == 0 {
		return LogHistogram{}
	}

	totalRange := maxTime.Sub(minTime)
	step := pickBucketStep(totalRange, numBuckets)

	// Compute the bucket count needed to cover the full range. When
	// totalRange is zero (one timestamp, or every line at the same
	// instant) we still want exactly one bucket.
	bucketCount := int(totalRange/step) + 1
	if bucketCount > numBuckets {
		bucketCount = numBuckets
	}
	if bucketCount < 1 {
		bucketCount = 1
	}

	counts := make([]int, bucketCount)
	for _, line := range lines {
		ts, _, ok := parseLogLineTimestamp(line)
		if !ok {
			continue
		}
		idx := int(ts.Sub(minTime) / step)
		if idx < 0 {
			idx = 0
		}
		if idx >= bucketCount {
			idx = bucketCount - 1
		}
		counts[idx]++
	}

	return LogHistogram{
		BucketStep: step,
		Start:      minTime,
		Counts:     counts,
	}
}

// pickBucketStep returns the smallest step from histogramBucketLadder
// where totalRange / step <= numBuckets. When totalRange is zero or
// negative (single-instant buffer) it returns the smallest step so
// the resulting bucket count math still yields exactly one bucket.
func pickBucketStep(totalRange time.Duration, numBuckets int) time.Duration {
	if totalRange <= 0 {
		return histogramBucketLadder[0]
	}
	for _, step := range histogramBucketLadder {
		if int(totalRange/step) <= numBuckets-1 {
			return step
		}
	}
	// Beyond 24h ranges: keep widening by integer days so the histogram
	// still fits in numBuckets columns. We deliberately stay on the
	// "round number of days" axis so the bucket boundaries look natural
	// in the strip's mouseover/footnote.
	step := histogramBucketLadder[len(histogramBucketLadder)-1]
	for int(totalRange/step) > numBuckets-1 {
		step += 24 * time.Hour
	}
	return step
}

// CursorBucket returns the bucket index containing the cursor's source
// line timestamp, or -1 when the cursor line has no timestamp or the
// histogram is empty. The caller is expected to have already extracted
// the source line at the cursor's position.
func (h LogHistogram) CursorBucket(cursorLine string) int {
	if len(h.Counts) == 0 || h.BucketStep <= 0 {
		return -1
	}
	ts, _, ok := parseLogLineTimestamp(cursorLine)
	if !ok {
		return -1
	}
	idx := int(ts.Sub(h.Start) / h.BucketStep)
	if idx < 0 {
		return -1
	}
	if idx >= len(h.Counts) {
		idx = len(h.Counts) - 1
	}
	return idx
}

// FormatBucketStep returns a compact human-friendly representation of
// the histogram's bucket width, suitable for surfacing in a title-bar
// chip ("[HIST: 5m]"). Returns the empty string for a zero histogram.
func (h LogHistogram) FormatBucketStep() string {
	if h.BucketStep <= 0 {
		return ""
	}
	return formatBucketStepDuration(h.BucketStep)
}

// formatBucketStepDuration mirrors the human spelling used on the
// histogramBucketLadder. Anything beyond the ladder's largest step is
// formatted as a plain "{n}d" since pickBucketStep only widens by
// whole days past 24h.
func formatBucketStepDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return formatInt(int(d/time.Second)) + "s"
	case d < time.Hour:
		return formatInt(int(d/time.Minute)) + "m"
	case d < 24*time.Hour:
		return formatInt(int(d/time.Hour)) + "h"
	default:
		return formatInt(int(d/(24*time.Hour))) + "d"
	}
}

// formatInt is a small helper that avoids pulling in fmt for a
// hot-path formatter. Histograms are recomputed every render frame, so
// allocating a *fmt.pp for every chip would add up.
func formatInt(n int) string {
	if n == 0 {
		return "0"
	}
	negative := false
	if n < 0 {
		negative = true
		n = -n
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
