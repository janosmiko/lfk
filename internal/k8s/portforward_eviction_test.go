package k8s

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- EntriesForDisplay: terminal-state eviction ---
// A terminal entry survives its first display and is gone by the second.

func TestEntriesForDisplay_EvictsStoppedAfterSecondCall(t *testing.T) {
	mgr := NewPortForwardManager()
	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{
		{ID: 1, Status: PortForwardStopped, StartedAt: time.Now()},
		{ID: 2, Status: PortForwardRunning, StartedAt: time.Now()},
	}
	mgr.mu.Unlock()

	first := mgr.EntriesForDisplay()
	assert.Len(t, first, 2, "stopped entry must still be visible on first display")

	second := mgr.EntriesForDisplay()
	require.Len(t, second, 1, "stopped entry must be evicted after being shown once")
	assert.Equal(t, 2, second[0].ID, "the running entry must remain")
}

func TestEntriesForDisplay_EvictsFailedAfterSecondCall(t *testing.T) {
	mgr := NewPortForwardManager()
	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{
		{ID: 1, Status: PortForwardFailed, Error: "boom", StartedAt: time.Now()},
	}
	mgr.mu.Unlock()

	first := mgr.EntriesForDisplay()
	require.Len(t, first, 1)
	assert.Equal(t, "boom", first[0].Error, "the failure reason must be visible on first display")

	second := mgr.EntriesForDisplay()
	assert.Empty(t, second, "failed entry must be evicted after being shown once")
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
