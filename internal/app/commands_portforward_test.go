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
// period even if the manager never reports another update.
func TestWaitForPortForwardUpdate_FiresOnEvictionDeadlineWithoutAnUpdate(t *testing.T) {
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
		t.Fatal("waitForPortForwardUpdate did not fire on the eviction deadline without a manager update")
	}
}

// select picks pseudo-randomly among ready cases, so with ch and superseded
// both ready every call, a version that returns nil on superseded drops the
// pending update about half the time. 4000 iterations makes that certain.
func TestWaitForPortForwardSignal_SupersededDoesNotDropPendingUpdate(t *testing.T) {
	superseded := make(chan struct{})
	close(superseded)

	for i := range 4000 {
		ch := make(chan struct{}, 1)
		ch <- struct{}{}

		msg := waitForPortForwardSignal(ch, nil, superseded)

		require.NotNilf(t, msg, "iteration %d: pending update on ch was lost when superseded also fired", i)
		assert.IsType(t, portForwardUpdateMsg{}, msg)
	}
}

// A superseded waiter tears down after its replacement took over, so its
// teardown must leave the replacement's eviction deadline alone. Otherwise
// the terminal entry never gets the render that evicts it.
func TestWaitForPortForwardUpdate_SupersededTeardownKeepsTheEvictionDeadline(t *testing.T) {
	m := basePush80Model()
	current := time.Now()
	mgr := k8s.NewPortForwardManagerWithClock(func() time.Time { return current })
	mgr.SeedTerminalEntryForTest(1, current.Add(-2750*time.Millisecond))
	m.portForwardMgr = mgr

	older := m.waitForPortForwardUpdate()
	require.NotNil(t, older)
	olderDone := make(chan tea.Msg, 1)
	go func() { olderDone <- older() }()

	// Give the older waiter time to arm the deadline before it is superseded.
	time.Sleep(30 * time.Millisecond)

	newer := m.waitForPortForwardUpdate()
	require.NotNil(t, newer)
	newerDone := make(chan tea.Msg, 1)
	go func() { newerDone <- newer() }()

	select {
	case <-olderDone:
	case <-time.After(2 * time.Second):
		t.Fatal("the superseded waitForPortForwardUpdate waiter did not exit")
	}

	select {
	case msg := <-newerDone:
		assert.IsType(t, portForwardUpdateMsg{}, msg)
	case <-time.After(3 * time.Second):
		t.Fatal("the newer waiter never fired: the superseded waiter's teardown took the eviction deadline with it")
	}
}

// SetUpdateListener keeps only one slot, so an older listener must be
// released rather than blocking forever once a newer one is armed.
func TestWaitForPortForwardUpdate_SupersededListenerExits(t *testing.T) {
	m := basePush80Model()
	m.portForwardMgr = k8s.NewPortForwardManager()

	firstCmd := m.waitForPortForwardUpdate()
	require.NotNil(t, firstCmd)

	firstDone := make(chan tea.Msg, 1)
	go func() { firstDone <- firstCmd() }()

	// Give the first goroutine time to reach its select before it is superseded.
	time.Sleep(20 * time.Millisecond)

	secondCmd := m.waitForPortForwardUpdate()
	require.NotNil(t, secondCmd)

	select {
	case <-firstDone:
	case <-time.After(2 * time.Second):
		t.Fatal("superseded waitForPortForwardUpdate listener did not exit when a new listener was armed")
	}
}
