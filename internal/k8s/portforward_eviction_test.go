package k8s

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- EntriesForDisplay: terminal-state eviction ---
// A terminal entry survives its first display, and any display still
// within the grace period, and is gone once the grace period has passed.

func TestEntriesForDisplay_StoppedSurvivesBackToBackCallsThenEvicts(t *testing.T) {
	mgr := NewPortForwardManager()
	current := time.Now()
	mgr.SetClockForTest(func() time.Time { return current })

	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{
		{ID: 1, Status: PortForwardStopped, StartedAt: current},
		{ID: 2, Status: PortForwardRunning, StartedAt: current},
	}
	mgr.mu.Unlock()

	first := mgr.EntriesForDisplay()
	assert.Len(t, first, 2, "stopped entry must be visible on first display")

	second := mgr.EntriesForDisplay()
	require.Len(t, second, 2, "an immediate second display must not evict — coalesced Updates can skip a paint")

	current = current.Add(portForwardEvictionGrace)
	third := mgr.EntriesForDisplay()
	require.Len(t, third, 1, "stopped entry must be evicted once the grace period has passed")
	assert.Equal(t, 2, third[0].ID, "the running entry must remain")
}

func TestEntriesForDisplay_FailedSurvivesBackToBackCallsThenEvicts(t *testing.T) {
	mgr := NewPortForwardManager()
	current := time.Now()
	mgr.SetClockForTest(func() time.Time { return current })

	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{
		{ID: 1, Status: PortForwardFailed, Error: "boom", StartedAt: current},
	}
	mgr.mu.Unlock()

	first := mgr.EntriesForDisplay()
	require.Len(t, first, 1)
	assert.Equal(t, "boom", first[0].Error, "the failure reason must be visible on first display")

	second := mgr.EntriesForDisplay()
	require.Len(t, second, 1, "an immediate second display must not evict — coalesced Updates can skip a paint")

	current = current.Add(portForwardEvictionGrace)
	third := mgr.EntriesForDisplay()
	assert.Empty(t, third, "failed entry must be evicted once the grace period has passed")
}

func TestEntriesForDisplay_NeverEvictsActiveEntries(t *testing.T) {
	mgr := NewPortForwardManager()
	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{
		{ID: 1, Status: PortForwardRunning, StartedAt: time.Now()},
		{ID: 2, Status: PortForwardStarting, StartedAt: time.Now()},
	}
	mgr.mu.Unlock()

	for i := range 3 {
		entries := mgr.EntriesForDisplay()
		assert.Lenf(t, entries, 2, "call %d: running/starting entries must never be evicted", i)
	}
}
