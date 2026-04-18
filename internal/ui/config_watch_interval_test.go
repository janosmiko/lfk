package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClampWatchInterval(t *testing.T) {
	tests := []struct {
		name string
		in   time.Duration
		want time.Duration
	}{
		{"zero returns zero (treated as unset)", 0, 0},
		{"below 500ms clamps up to 500ms", 100 * time.Millisecond, 500 * time.Millisecond},
		{"exactly 500ms passes through", 500 * time.Millisecond, 500 * time.Millisecond},
		{"1 second passes through", 1 * time.Second, 1 * time.Second},
		{"2 seconds (default) passes through", 2 * time.Second, 2 * time.Second},
		{"5 minutes passes through", 5 * time.Minute, 5 * time.Minute},
		{"exactly 10 minutes passes through", 10 * time.Minute, 10 * time.Minute},
		{"above 10 minutes clamps down to 10 minutes", 15 * time.Minute, 10 * time.Minute},
		{"negative returns zero (treated as unset)", -1 * time.Second, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ClampWatchInterval(tc.in)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestLoadConfig_WatchInterval(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		want    time.Duration
		wantOK  bool
		comment string
	}{
		{
			name:    "valid 5s duration",
			yaml:    "watch_interval: 5s\n",
			want:    5 * time.Second,
			wantOK:  true,
			comment: "parses cleanly",
		},
		{
			name:    "valid 500ms duration",
			yaml:    "watch_interval: 500ms\n",
			want:    500 * time.Millisecond,
			wantOK:  true,
			comment: "min allowed",
		},
		{
			name:    "100ms is clamped to 500ms",
			yaml:    "watch_interval: 100ms\n",
			want:    500 * time.Millisecond,
			wantOK:  true,
			comment: "below min clamps up",
		},
		{
			name:    "30m is clamped to 10m",
			yaml:    "watch_interval: 30m\n",
			want:    10 * time.Minute,
			wantOK:  true,
			comment: "above max clamps down",
		},
		{
			name:    "invalid duration falls back to default 2s",
			yaml:    "watch_interval: 30d\n",
			want:    2 * time.Second,
			wantOK:  false,
			comment: "30d is not a valid Go duration",
		},
		{
			name:    "missing field uses default 2s",
			yaml:    "colorscheme: dracula\n",
			want:    2 * time.Second,
			wantOK:  false,
			comment: "unset fallback",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			origInterval := ConfigWatchInterval
			t.Cleanup(func() { ConfigWatchInterval = origInterval })
			ConfigWatchInterval = 2 * time.Second

			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(path, []byte(tc.yaml), 0o600); err != nil {
				t.Fatal(err)
			}

			LoadConfig(path)
			assert.Equal(t, tc.want, ConfigWatchInterval, tc.comment)
		})
	}
}
