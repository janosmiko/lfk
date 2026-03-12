package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPortForwardManager(t *testing.T) {
	mgr := NewPortForwardManager()
	assert.NotNil(t, mgr)
	assert.Equal(t, 0, mgr.ActiveCount())
	assert.Empty(t, mgr.Entries())
}

func TestPortForwardManagerStopAll(t *testing.T) {
	mgr := NewPortForwardManager()
	// StopAll on empty manager should not panic.
	mgr.StopAll()
	assert.Equal(t, 0, mgr.ActiveCount())
}

func TestPortForwardManagerRemoveNonexistent(t *testing.T) {
	mgr := NewPortForwardManager()
	// Remove non-existent entry should not panic.
	mgr.Remove(999)
	assert.Equal(t, 0, mgr.ActiveCount())
}

func TestPortForwardManagerStopNonexistent(t *testing.T) {
	mgr := NewPortForwardManager()
	err := mgr.Stop(999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPortForwardManagerUpdateCallback(t *testing.T) {
	mgr := NewPortForwardManager()
	called := false
	mgr.SetUpdateCallback(func() {
		called = true
	})
	// Remove triggers callback path but does nothing since entry doesn't exist.
	mgr.Remove(1)
	// StopAll triggers callback path.
	mgr.StopAll()
	assert.False(t, called) // No actual entries to trigger on.
}
