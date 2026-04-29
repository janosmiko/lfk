package ui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- FormatAge ---

func TestFormatAge(t *testing.T) {
	tests := []struct {
		name     string
		offset   time.Duration
		expected string
	}{
		{"30 seconds ago", 30 * time.Second, "30s"},
		{"59 seconds ago", 59 * time.Second, "59s"},
		{"1 minute ago", 1 * time.Minute, "1m"},
		{"5 minutes ago", 5*time.Minute + 30*time.Second, "5m"},
		{"59 minutes ago", 59*time.Minute + 59*time.Second, "59m"},
		{"1 hour ago", 1 * time.Hour, "1h"},
		{"12 hours ago", 12 * time.Hour, "12h"},
		{"23 hours ago", 23*time.Hour + 59*time.Minute, "23h"},
		{"1 day ago", 24 * time.Hour, "1d"},
		{"7 days ago", 7 * 24 * time.Hour, "7d"},
		{"30 days ago", 30 * 24 * time.Hour, "30d"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatAge(time.Now().Add(-tt.offset))
			assert.Equal(t, tt.expected, result)
		})
	}

	t.Run("zero time returns dash", func(t *testing.T) {
		assert.Equal(t, "-", FormatAge(time.Time{}))
	})
}
