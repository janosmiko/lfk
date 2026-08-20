package app

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCov80RestorePortForwardsNoKubectl(t *testing.T) {
	m := basePush80Model()
	m.portForwardMgr = k8s.NewPortForwardManager()
	m.pendingPortForwards = &PortForwardStates{
		PortForwards: []PortForwardState{
			{ResourceKind: "svc", ResourceName: "my-svc", Namespace: "default", Context: "test-ctx", LocalPort: "8080", RemotePort: "80"},
		},
	}
	t.Setenv("PATH", "/nonexistent")
	cmds := m.restorePortForwards()
	assert.Nil(t, cmds)
}

func TestCov80RestorePortForwardsEmpty(t *testing.T) {
	m := basePush80Model()
	m.portForwardMgr = k8s.NewPortForwardManager()
	m.pendingPortForwards = &PortForwardStates{}
	cmds := m.restorePortForwards()
	assert.Empty(t, cmds)
}

func TestCovSaveCurrentPortForwards(t *testing.T) {
	m := baseModelCov()
	m.portForwardMgr = k8s.NewPortForwardManager()
	// Should not panic with no entries.
	m.saveCurrentPortForwards()
}

func TestCovRestorePortForwards(t *testing.T) {
	m := baseModelCov()
	m.portForwardMgr = k8s.NewPortForwardManager()
	m.client = k8s.NewTestClient(nil, nil)
	m.pendingPortForwards = &PortForwardStates{
		PortForwards: []PortForwardState{},
	}
	cmds := m.restorePortForwards()
	// No port forwards to restore, and kubectl may not be available.
	_ = cmds
}

// TestRestartLocalPort verifies that restarting a port forward reuses the
// local port the forward already had, rather than picking a new random one.
func TestRestartLocalPort(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"resolved random port is reused", "54321", "54321"},
		{"user-specified port is reused", "8080", "8080"},
		{"unresolved zero stays random", "0", "0"},
		{"empty falls back to random", "", "0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := restartLocalPort(tc.in); got != tc.want {
				t.Errorf("restartLocalPort(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// A terminal port-forward, once shown, must be evicted after its grace
// period even if the manager never fires another update callback.
func TestWaitForPortForwardUpdate_FiresOnEvictionDeadlineWithoutCallback(t *testing.T) {
	m := basePush80Model()
	current := time.Now()
	mgr := k8s.NewPortForwardManagerWithClock(func() time.Time { return current })
	mgr.SeedTerminalEntryForTest(1, current.Add(-4*time.Second))
	m.portForwardMgr = mgr

	cmd := m.waitForPortForwardUpdate()
	require.NotNil(t, cmd)

	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()

	select {
	case msg := <-done:
		assert.IsType(t, portForwardUpdateMsg{}, msg)
	case <-time.After(2 * time.Second):
		t.Fatal("waitForPortForwardUpdate did not fire on the eviction deadline without a manager callback")
	}
}
