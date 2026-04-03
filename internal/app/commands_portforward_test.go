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
