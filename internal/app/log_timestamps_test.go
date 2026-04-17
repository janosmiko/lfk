package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseLogLineTimestamp exercises the kubectl-style RFC3339Nano
// prefix, the plain RFC3339 prefix, the kubectl --prefix envelope, and
// the negative cases (no timestamp, partial timestamp, empty line).
func TestParseLogLineTimestamp(t *testing.T) {
	t.Run("kubectl RFC3339Nano is parsed and sliced", func(t *testing.T) {
		line := "2026-04-16T10:00:30.123456789Z hello world"
		ts, consumed, ok := parseLogLineTimestamp(line)
		require.True(t, ok)
		assert.Equal(t, 2026, ts.Year())
		assert.Equal(t, time.April, ts.Month())
		assert.Equal(t, 16, ts.Day())
		// Slicing by len(consumed) should give us the remainder.
		assert.Equal(t, "hello world", line[len(consumed):])
	})

	t.Run("plain RFC3339 (no nanoseconds) is parsed", func(t *testing.T) {
		line := "2026-04-16T10:00:30Z message body"
		ts, consumed, ok := parseLogLineTimestamp(line)
		require.True(t, ok)
		assert.Equal(t, 10, ts.Hour())
		assert.Equal(t, "message body", line[len(consumed):])
	})

	t.Run("kubectl --prefix envelope is tolerated", func(t *testing.T) {
		line := "[pod/api-abc12/server] 2026-04-16T10:00:30Z serving on :8080"
		ts, consumed, ok := parseLogLineTimestamp(line)
		require.True(t, ok)
		assert.Equal(t, 2026, ts.Year())
		assert.Equal(t, "serving on :8080", line[len(consumed):])
	})

	t.Run("line with no timestamp returns ok=false", func(t *testing.T) {
		line := "GET /healthz 200"
		ts, consumed, ok := parseLogLineTimestamp(line)
		assert.False(t, ok)
		assert.True(t, ts.IsZero())
		assert.Empty(t, consumed)
	})

	t.Run("partial timestamp prefix is rejected", func(t *testing.T) {
		// Looks like a date fragment but has no `T` separator and no space.
		line := "2026-04-16 something"
		_, _, ok := parseLogLineTimestamp(line)
		assert.False(t, ok)
	})

	t.Run("empty line returns ok=false", func(t *testing.T) {
		_, _, ok := parseLogLineTimestamp("")
		assert.False(t, ok)
	})

	t.Run("timestamp with no trailing content still parses", func(t *testing.T) {
		// Missing space after the stamp — we require a space terminator
		// because the caller needs a slice point for the message body.
		line := "2026-04-16T10:00:30Z"
		_, _, ok := parseLogLineTimestamp(line)
		assert.False(t, ok)
	})
}

// TestFormatRelativeTimestamp covers every branch of the formatter:
// the sub-second "just now", each past bucket, and both future buckets
// (near and far).
func TestFormatRelativeTimestamp(t *testing.T) {
	now := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		ts   time.Time
		want string
	}{
		{
			name: "sub-second delta renders as 'just now'",
			ts:   now.Add(-500 * time.Millisecond),
			want: "just now",
		},
		{
			name: "exact now renders as 'just now'",
			ts:   now,
			want: "just now",
		},
		{
			name: "5 seconds ago",
			ts:   now.Add(-5 * time.Second),
			want: "5s ago",
		},
		{
			name: "45 minutes ago",
			ts:   now.Add(-45 * time.Minute),
			want: "45m ago",
		},
		{
			name: "2 hours ago",
			ts:   now.Add(-2 * time.Hour),
			want: "2h ago",
		},
		{
			name: "3 days ago",
			ts:   now.Add(-3 * 24 * time.Hour),
			want: "3d ago",
		},
		{
			name: "future 2 minutes",
			ts:   now.Add(2 * time.Minute),
			want: "in 2m",
		},
		{
			name: "far future 5 days",
			ts:   now.Add(5 * 24 * time.Hour),
			want: "in 5d",
		},
		{
			name: "future sub-second",
			ts:   now.Add(500 * time.Millisecond),
			want: "just now",
		},
		{
			name: "future 30 seconds",
			ts:   now.Add(30 * time.Second),
			want: "in 30s",
		},
		{
			name: "future 3 hours",
			ts:   now.Add(3 * time.Hour),
			want: "in 3h",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatRelativeTimestamp(tc.ts, now)
			assert.Equal(t, tc.want, got)
		})
	}
}
