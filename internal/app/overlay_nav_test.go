package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClampOverlayCursor(t *testing.T) {
	tests := []struct {
		name   string
		cursor int
		delta  int
		maxIdx int
		want   int
	}{
		// Normal movement within bounds.
		{name: "move down by 1 within bounds", cursor: 0, delta: 1, maxIdx: 9, want: 1},
		{name: "move up by 1 within bounds", cursor: 5, delta: -1, maxIdx: 9, want: 4},
		{name: "move down by 10 within bounds", cursor: 0, delta: 10, maxIdx: 19, want: 10},
		{name: "move up by 10 within bounds", cursor: 15, delta: -10, maxIdx: 19, want: 5},

		// Clamping at upper bound.
		{name: "clamp at upper bound with delta 1", cursor: 9, delta: 1, maxIdx: 9, want: 9},
		{name: "clamp at upper bound with delta 10", cursor: 5, delta: 10, maxIdx: 9, want: 9},
		{name: "clamp at upper bound with delta 20", cursor: 0, delta: 20, maxIdx: 9, want: 9},

		// Clamping at lower bound.
		{name: "clamp at lower bound with delta -1", cursor: 0, delta: -1, maxIdx: 9, want: 0},
		{name: "clamp at lower bound with delta -10", cursor: 5, delta: -10, maxIdx: 9, want: 0},
		{name: "clamp at lower bound with delta -20", cursor: 3, delta: -20, maxIdx: 9, want: 0},

		// Empty list (maxIdx = -1).
		{name: "empty list returns 0", cursor: 0, delta: 1, maxIdx: -1, want: 0},
		{name: "empty list with negative delta returns 0", cursor: 0, delta: -1, maxIdx: -1, want: 0},
		{name: "empty list with large positive delta returns 0", cursor: 5, delta: 10, maxIdx: -1, want: 0},

		// Large delta exceeding bounds.
		{name: "large positive delta exceeding bounds", cursor: 0, delta: 1000, maxIdx: 50, want: 50},
		{name: "large negative delta exceeding bounds", cursor: 50, delta: -1000, maxIdx: 50, want: 0},

		// Zero delta (no change).
		{name: "zero delta no change", cursor: 5, delta: 0, maxIdx: 9, want: 5},
		{name: "zero delta at zero", cursor: 0, delta: 0, maxIdx: 9, want: 0},
		{name: "zero delta at max", cursor: 9, delta: 0, maxIdx: 9, want: 9},

		// Single-item list.
		{name: "single item list move down", cursor: 0, delta: 1, maxIdx: 0, want: 0},
		{name: "single item list move up", cursor: 0, delta: -1, maxIdx: 0, want: 0},
		{name: "single item list no change", cursor: 0, delta: 0, maxIdx: 0, want: 0},

		// Edge: cursor already beyond maxIdx (shouldn't happen, but verify clamping).
		{name: "cursor beyond maxIdx clamped", cursor: 20, delta: 0, maxIdx: 9, want: 9},
		{name: "cursor negative clamped", cursor: -5, delta: 0, maxIdx: 9, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clampOverlayCursor(tt.cursor, tt.delta, tt.maxIdx)
			if got != tt.want {
				t.Errorf("clampOverlayCursor(%d, %d, %d) = %d, want %d",
					tt.cursor, tt.delta, tt.maxIdx, got, tt.want)
			}
		})
	}
}

func TestCovClampOverlayCursorExhaustive(t *testing.T) {
	tests := []struct {
		name     string
		cursor   int
		delta    int
		maxIdx   int
		expected int
	}{
		{"move down", 0, 1, 5, 1},
		{"move up", 3, -1, 5, 2},
		{"clamp at max", 5, 1, 5, 5},
		{"clamp at zero", 0, -1, 5, 0},
		{"empty list", 0, 1, -1, 0},
		{"big jump", 0, 100, 10, 10},
		{"big negative", 5, -100, 10, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, clampOverlayCursor(tt.cursor, tt.delta, tt.maxIdx))
		})
	}
}
