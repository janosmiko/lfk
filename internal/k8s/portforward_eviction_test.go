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
	current := time.Now()
	mgr := NewPortForwardManagerWithClock(func() time.Time { return current })

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
	current := time.Now()
	mgr := NewPortForwardManagerWithClock(func() time.Time { return current })

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

// Start()'s monitor goroutine mutates its own *PortForwardEntry on exit, so
// that identity must survive a display call for the transition to show.
func TestEntriesForDisplay_KeepsEntryPointerIdentityForLiveStateUpdates(t *testing.T) {
	mgr := NewPortForwardManager()
	entry := &PortForwardEntry{ID: 1, Status: PortForwardRunning, StartedAt: time.Now()}
	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{entry}
	mgr.mu.Unlock()

	first := mgr.EntriesForDisplay()
	require.Len(t, first, 1)
	assert.Equal(t, PortForwardRunning, first[0].Status)

	mgr.mu.Lock()
	entry.Status = PortForwardStopped
	mgr.mu.Unlock()

	second := mgr.EntriesForDisplay()
	require.Len(t, second, 1)
	assert.Equal(t, PortForwardStopped, second[0].Status,
		"a status mutation via the original entry pointer must survive a prior display call")
}

// --- ArmEvictionRefresh: scheduling a refresh at the eviction deadline ---
// A terminal port-forward that has been shown must be evicted once its
// grace period passes even if no further manager callback ever fires.

func TestArmEvictionRefresh_NothingPendingForActiveOrUnshownEntries(t *testing.T) {
	mgr := NewPortForwardManager()
	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{
		{ID: 1, Status: PortForwardRunning},
		{ID: 2, Status: PortForwardStopped}, // never displayed: shownAt is zero
	}
	mgr.mu.Unlock()

	_, _, ok := mgr.ArmEvictionRefresh()
	assert.False(t, ok, "a running or never-shown entry must not arm an eviction tick")
}

func TestArmEvictionRefresh_SchedulesAtTheEntrysDeadline(t *testing.T) {
	current := time.Now()
	mgr := NewPortForwardManagerWithClock(func() time.Time { return current })
	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{{ID: 1, Status: PortForwardStopped, shownAt: current.Add(-time.Second)}}
	mgr.mu.Unlock()

	d, _, ok := mgr.ArmEvictionRefresh()
	require.True(t, ok)
	assert.Equal(t, portForwardEvictionGrace-time.Second, d)
}

func TestArmEvictionRefresh_DedupesUntilDisarmed(t *testing.T) {
	current := time.Now()
	mgr := NewPortForwardManagerWithClock(func() time.Time { return current })
	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{{ID: 1, Status: PortForwardStopped, shownAt: current}}
	mgr.mu.Unlock()

	_, arm, ok := mgr.ArmEvictionRefresh()
	require.True(t, ok, "first call must arm")

	_, _, ok = mgr.ArmEvictionRefresh()
	assert.False(t, ok, "a repeat call before the armed deadline fires must not stack another timer")

	mgr.DisarmEvictionRefresh(arm)
	_, _, ok = mgr.ArmEvictionRefresh()
	assert.True(t, ok, "disarming must allow re-arming for the still-pending entry")
}

func TestArmEvictionRefresh_ReArmsForAnEarlierDeadline(t *testing.T) {
	current := time.Now()
	mgr := NewPortForwardManagerWithClock(func() time.Time { return current })
	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{{ID: 1, Status: PortForwardStopped, shownAt: current}}
	mgr.mu.Unlock()
	_, _, ok := mgr.ArmEvictionRefresh()
	require.True(t, ok)

	mgr.mu.Lock()
	mgr.entries = append(mgr.entries, &PortForwardEntry{ID: 2, Status: PortForwardFailed, shownAt: current.Add(-2 * time.Second)})
	mgr.mu.Unlock()

	d, _, ok := mgr.ArmEvictionRefresh()
	require.True(t, ok, "an earlier deadline must supersede the already-armed one")
	assert.Equal(t, portForwardEvictionGrace-2*time.Second, d)
}

// The waiter holding the listener slot owns the armed deadline. Its
// predecessor tears down afterwards and must not release it, or the entry
// waits for a tick nobody scheduled.
func TestArmEvictionRefresh_SupersededWaiterKeepsItsHandsOffTheNewArm(t *testing.T) {
	current := time.Now()
	mgr := NewPortForwardManagerWithClock(func() time.Time { return current })
	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{{ID: 1, Status: PortForwardStopped, shownAt: current}}
	mgr.mu.Unlock()

	mgr.SetUpdateListener(make(chan struct{}, 1))
	_, older, ok := mgr.ArmEvictionRefresh()
	require.True(t, ok)

	mgr.SetUpdateListener(make(chan struct{}, 1))
	d, newer, ok := mgr.ArmEvictionRefresh()
	require.True(t, ok, "the waiter taking over the listener slot must get a deadline of its own")
	assert.Equal(t, portForwardEvictionGrace, d)

	mgr.DisarmEvictionRefresh(older)
	_, _, ok = mgr.ArmEvictionRefresh()
	assert.False(t, ok, "a superseded waiter must not release the deadline its successor armed")

	mgr.DisarmEvictionRefresh(newer)
	_, _, ok = mgr.ArmEvictionRefresh()
	assert.True(t, ok, "the owner must still be able to release its own deadline")
}

// EntriesForDisplay only evicts on the next call. ArmEvictionRefresh is the
// signal that schedules that next call once the grace period elapses.
func TestPortForward_StaysVisibleWithoutArming(t *testing.T) {
	current := time.Now()
	mgr := NewPortForwardManagerWithClock(func() time.Time { return current })
	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{{ID: 1, Status: PortForwardStopped, shownAt: current}}
	mgr.mu.Unlock()

	current = current.Add(portForwardEvictionGrace + time.Second)

	d, _, ok := mgr.ArmEvictionRefresh()
	require.True(t, ok, "a shown, expired terminal entry must arm a refresh")
	assert.Equal(t, time.Duration(0), d, "a deadline already in the past must fire immediately, not be missed")
}
