package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/stretchr/testify/assert"
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
