package app

import (
	"testing"
	"time"
)

func TestActiveWatchInterval(t *testing.T) {
	const fg = 2 * time.Second
	const bg = 30 * time.Second
	now := time.Now()
	tests := []struct {
		name        string
		focused     bool
		lastInputAt time.Time
		idleTimeout time.Duration
		want        time.Duration
	}{
		{"focused active", true, now, 120 * time.Second, fg},
		{"background", false, now, 120 * time.Second, bg},
		{"focused idle past timeout", true, now.Add(-200 * time.Second), 120 * time.Second, bg},
		{"focused idle but disabled", true, now.Add(-200 * time.Second), 0, fg},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				watchThrottle:           true,
				focused:                 tt.focused,
				lastInputAt:             tt.lastInputAt,
				watchInterval:           fg,
				backgroundWatchInterval: bg,
				foregroundIdleTimeout:   tt.idleTimeout,
			}
			if got := m.activeWatchInterval(); got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestActiveWatchIntervalThrottleDisabled(t *testing.T) {
	// watchThrottle=false: always watch_interval regardless of focus/idle.
	m := Model{
		watchThrottle:           false,
		focused:                 false,
		lastInputAt:             time.Now().Add(-time.Hour),
		watchInterval:           2 * time.Second,
		backgroundWatchInterval: 30 * time.Second,
		foregroundIdleTimeout:   120 * time.Second,
	}
	if got := m.activeWatchInterval(); got != 2*time.Second {
		t.Fatalf("disabled throttle must return watch_interval, got %v", got)
	}
}
