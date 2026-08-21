package k8s

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- Entries: edge cases ---

func TestPortForwardManagerEntries_WithEntries(t *testing.T) {
	mgr := NewPortForwardManager()
	_, cancel := context.WithCancel(t.Context())
	defer cancel()

	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{
		{
			ID:           1,
			ResourceKind: "pod",
			ResourceName: "nginx",
			Namespace:    "default",
			Context:      "test",
			LocalPort:    "8080",
			RemotePort:   "80",
			Status:       PortForwardRunning,
			StartedAt:    time.Now(),
			cancel:       cancel,
		},
		{
			ID:           2,
			ResourceKind: "svc",
			ResourceName: "redis",
			Namespace:    "cache",
			Context:      "test",
			LocalPort:    "6379",
			RemotePort:   "6379",
			Status:       PortForwardStopped,
			StartedAt:    time.Now(),
			cancel:       cancel,
		},
	}
	mgr.mu.Unlock()

	entries := mgr.Entries()
	assert.Len(t, entries, 2)
	assert.Equal(t, "nginx", entries[0].ResourceName)
	assert.Equal(t, "redis", entries[1].ResourceName)
	// Verify it returns copies (modifying the returned entry should not affect the manager).
	entries[0].ResourceName = "modified"
	original := mgr.Entries()
	assert.Equal(t, "nginx", original[0].ResourceName, "Entries should return copies")
}

// --- ActiveCount: various statuses ---

func TestPortForwardManagerActiveCount_MixedStatuses(t *testing.T) {
	mgr := NewPortForwardManager()
	_, cancel := context.WithCancel(t.Context())
	defer cancel()

	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{
		{ID: 1, Status: PortForwardRunning, cancel: cancel},
		{ID: 2, Status: PortForwardStarting, cancel: cancel},
		{ID: 3, Status: PortForwardStopped, cancel: cancel},
		{ID: 4, Status: PortForwardFailed, cancel: cancel},
		{ID: 5, Status: PortForwardRunning, cancel: cancel},
	}
	mgr.mu.Unlock()

	assert.Equal(t, 3, mgr.ActiveCount(), "should count Running and Starting entries")
}

func TestPortForwardManagerActiveCount_AllStopped(t *testing.T) {
	mgr := NewPortForwardManager()
	_, cancel := context.WithCancel(t.Context())
	defer cancel()

	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{
		{ID: 1, Status: PortForwardStopped, cancel: cancel},
		{ID: 2, Status: PortForwardFailed, cancel: cancel},
	}
	mgr.mu.Unlock()

	assert.Equal(t, 0, mgr.ActiveCount())
}

// --- Stop: test stopping a running port forward ---

func TestPortForwardManagerStop_RunningEntry(t *testing.T) {
	mgr := NewPortForwardManager()
	_, cancel := context.WithCancel(t.Context())

	updates := make(chan struct{}, 4)
	mgr.SetUpdateListener(updates)

	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{
		{
			ID:     1,
			Status: PortForwardRunning,
			cancel: cancel,
		},
	}
	mgr.mu.Unlock()

	err := mgr.Stop(1)
	assert.NoError(t, err)

	entry := mgr.Entries()
	assert.Equal(t, PortForwardStopped, entry[0].Status)
	assert.Len(t, updates, 1, "the listener should be notified on Stop")
}

func TestPortForwardManagerStop_StartingEntry(t *testing.T) {
	mgr := NewPortForwardManager()
	_, cancel := context.WithCancel(t.Context())

	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{
		{
			ID:     1,
			Status: PortForwardStarting,
			cancel: cancel,
		},
	}
	mgr.mu.Unlock()

	err := mgr.Stop(1)
	assert.NoError(t, err)
	assert.Equal(t, PortForwardStopped, mgr.Entries()[0].Status)
}

func TestPortForwardManagerStop_AlreadyStopped(t *testing.T) {
	mgr := NewPortForwardManager()
	_, cancel := context.WithCancel(t.Context())

	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{
		{
			ID:     1,
			Status: PortForwardStopped,
			cancel: cancel,
		},
	}
	mgr.mu.Unlock()

	err := mgr.Stop(1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestPortForwardManagerStop_FailedEntry(t *testing.T) {
	mgr := NewPortForwardManager()
	_, cancel := context.WithCancel(t.Context())

	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{
		{
			ID:     1,
			Status: PortForwardFailed,
			cancel: cancel,
		},
	}
	mgr.mu.Unlock()

	err := mgr.Stop(1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestPortForwardManagerStop_NotFound(t *testing.T) {
	mgr := NewPortForwardManager()
	err := mgr.Stop(999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPortForwardManagerStop_NoCallback(t *testing.T) {
	mgr := NewPortForwardManager()
	_, cancel := context.WithCancel(t.Context())

	// No callback set.
	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{
		{ID: 1, Status: PortForwardRunning, cancel: cancel},
	}
	mgr.mu.Unlock()

	err := mgr.Stop(1)
	assert.NoError(t, err, "Stop without callback should not panic")
}

// --- Remove: test removing entries ---

func TestPortForwardManagerRemove_StoppedEntry(t *testing.T) {
	mgr := NewPortForwardManager()
	_, cancel := context.WithCancel(t.Context())

	updates := make(chan struct{}, 4)
	mgr.SetUpdateListener(updates)

	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{
		{ID: 1, Status: PortForwardStopped, cancel: cancel},
		{ID: 2, Status: PortForwardRunning, cancel: cancel},
	}
	mgr.mu.Unlock()

	mgr.Remove(1)
	assert.Len(t, mgr.Entries(), 1)
	assert.Equal(t, 2, mgr.Entries()[0].ID)
	assert.Len(t, updates, 1)
}

func TestPortForwardManagerRemove_RunningEntry(t *testing.T) {
	mgr := NewPortForwardManager()
	_, cancel := context.WithCancel(t.Context())

	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{
		{ID: 1, Status: PortForwardRunning, cancel: cancel},
	}
	mgr.mu.Unlock()

	// Removing a running entry should cancel it first.
	mgr.Remove(1)
	assert.Empty(t, mgr.Entries())
}

func TestPortForwardManagerRemove_StartingEntry(t *testing.T) {
	mgr := NewPortForwardManager()
	_, cancel := context.WithCancel(t.Context())

	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{
		{ID: 1, Status: PortForwardStarting, cancel: cancel},
	}
	mgr.mu.Unlock()

	mgr.Remove(1)
	assert.Empty(t, mgr.Entries())
}

func TestPortForwardManagerRemove_NoCallback(t *testing.T) {
	mgr := NewPortForwardManager()
	_, cancel := context.WithCancel(t.Context())

	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{
		{ID: 1, Status: PortForwardStopped, cancel: cancel},
	}
	mgr.mu.Unlock()

	mgr.Remove(1)
	assert.Empty(t, mgr.Entries(), "Remove without callback should not panic")
}

// --- GetEntry: test retrieving entries ---

func TestPortForwardManagerGetEntry_Found(t *testing.T) {
	mgr := NewPortForwardManager()
	_, cancel := context.WithCancel(t.Context())

	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{
		{
			ID:           1,
			ResourceKind: "pod",
			ResourceName: "nginx",
			Namespace:    "default",
			Status:       PortForwardRunning,
			cancel:       cancel,
		},
		{
			ID:           2,
			ResourceKind: "svc",
			ResourceName: "redis",
			Namespace:    "cache",
			Status:       PortForwardStopped,
			cancel:       cancel,
		},
	}
	mgr.mu.Unlock()

	entry := mgr.GetEntry(1)
	assert.NotNil(t, entry)
	assert.Equal(t, 1, entry.ID)
	assert.Equal(t, "nginx", entry.ResourceName)
	assert.Equal(t, PortForwardRunning, entry.Status)

	entry2 := mgr.GetEntry(2)
	assert.NotNil(t, entry2)
	assert.Equal(t, "redis", entry2.ResourceName)
}

func TestPortForwardManagerGetEntry_NotFound(t *testing.T) {
	mgr := NewPortForwardManager()
	entry := mgr.GetEntry(999)
	assert.Nil(t, entry)
}

func TestPortForwardManagerGetEntry_ReturnsCopy(t *testing.T) {
	mgr := NewPortForwardManager()
	_, cancel := context.WithCancel(t.Context())

	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{
		{ID: 1, ResourceName: "original", cancel: cancel},
	}
	mgr.mu.Unlock()

	entry := mgr.GetEntry(1)
	entry.ResourceName = "modified"

	original := mgr.GetEntry(1)
	assert.Equal(t, "original", original.ResourceName, "GetEntry should return a copy")
}

// --- StopAll: test stopping all entries ---

func TestPortForwardManagerStopAll_MixedStatuses(t *testing.T) {
	mgr := NewPortForwardManager()
	_, cancel1 := context.WithCancel(t.Context())
	_, cancel2 := context.WithCancel(t.Context())
	_, cancel3 := context.WithCancel(t.Context())

	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{
		{ID: 1, Status: PortForwardRunning, cancel: cancel1},
		{ID: 2, Status: PortForwardStarting, cancel: cancel2},
		{ID: 3, Status: PortForwardStopped, cancel: cancel3},
	}
	mgr.mu.Unlock()

	mgr.StopAll()

	entries := mgr.Entries()
	for _, e := range entries {
		assert.Equal(t, PortForwardStopped, e.Status,
			"entry %d should be stopped after StopAll", e.ID)
	}
}

func TestPortForwardManagerStopAll_AllRunning(t *testing.T) {
	mgr := NewPortForwardManager()
	_, cancel1 := context.WithCancel(t.Context())
	_, cancel2 := context.WithCancel(t.Context())

	mgr.mu.Lock()
	mgr.entries = []*PortForwardEntry{
		{ID: 1, Status: PortForwardRunning, cancel: cancel1},
		{ID: 2, Status: PortForwardRunning, cancel: cancel2},
	}
	mgr.mu.Unlock()

	mgr.StopAll()
	assert.Equal(t, 0, mgr.ActiveCount())
}
