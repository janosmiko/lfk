package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseLogSinceDuration covers the supported inputs (Go duration
// syntax plus the "d" shorthand for days), and the rejected cases
// (empty, nonsense, non-positive).  It is the contract the overlay and
// the stream plumbing rely on.
func TestParseLogSinceDuration(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		wantDur     time.Duration
		wantDisplay string
		wantErr     bool
	}{
		{
			name:        "seconds",
			input:       "30s",
			wantDur:     30 * time.Second,
			wantDisplay: "30s",
		},
		{
			name:        "minutes",
			input:       "5m",
			wantDur:     5 * time.Minute,
			wantDisplay: "5m",
		},
		{
			name:        "composite hour and minute",
			input:       "1h30m",
			wantDur:     90 * time.Minute,
			wantDisplay: "1h30m",
		},
		{
			name:        "hours",
			input:       "2h",
			wantDur:     2 * time.Hour,
			wantDisplay: "2h",
		},
		{
			name:        "days suffix converts to hours",
			input:       "2d",
			wantDur:     48 * time.Hour,
			wantDisplay: "2d",
		},
		{
			name:        "days suffix single digit",
			input:       "7d",
			wantDur:     7 * 24 * time.Hour,
			wantDisplay: "7d",
		},
		{
			name:        "leading whitespace is trimmed for display",
			input:       "  15m  ",
			wantDur:     15 * time.Minute,
			wantDisplay: "15m",
		},
		{
			name:    "empty input rejected",
			input:   "",
			wantErr: true,
		},
		{
			name:    "whitespace only rejected",
			input:   "   ",
			wantErr: true,
		},
		{
			name:    "non-duration text rejected",
			input:   "bogus",
			wantErr: true,
		},
		{
			name:    "alpha-only text rejected",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "zero duration rejected",
			input:   "0s",
			wantErr: true,
		},
		{
			name:    "negative duration rejected",
			input:   "-5m",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dur, display, err := parseLogSinceDuration(tc.input)
			if tc.wantErr {
				require.Error(t, err, "expected error for input %q", tc.input)
				return
			}
			require.NoError(t, err, "unexpected error for input %q", tc.input)
			assert.Equal(t, tc.wantDur, dur, "duration mismatch")
			assert.Equal(t, tc.wantDisplay, display, "display mismatch")
		})
	}
}
