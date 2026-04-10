package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
)

func TestLoadSecurityAvailabilityNilManager(t *testing.T) {
	m := Model{}
	assert.Nil(t, m.loadSecurityAvailability())
}

func TestLoadSecurityAvailabilityDispatches(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{NameStr: "fake", Available: true})

	m := Model{securityManager: mgr}
	m.nav.Context = "kctx"

	cmd := m.loadSecurityAvailability()
	require.NotNil(t, cmd)
	msg := cmd()

	loaded, ok := msg.(securityAvailabilityLoadedMsg)
	require.True(t, ok)
	assert.Equal(t, "kctx", loaded.context)
	assert.True(t, loaded.availability["fake"], "fake source should be available")
}

func TestSecurityAvailabilityLoadedMsgUpdatesModel(t *testing.T) {
	m := Model{securityAvailabilityByName: make(map[string]bool)}
	m.nav.Context = "kctx"
	msg := securityAvailabilityLoadedMsg{
		context: "kctx",
		availability: map[string]bool{
			"trivy-operator": true,
			"heuristic":      true,
		},
	}
	updated := m.handleSecurityAvailabilityLoaded(msg)
	assert.True(t, updated.securityAvailabilityByName["trivy-operator"])
	assert.True(t, updated.securityAvailabilityByName["heuristic"])
}

func TestSecurityAvailabilityLoadedStaleContextDiscarded(t *testing.T) {
	m := Model{securityAvailabilityByName: make(map[string]bool)}
	m.nav.Context = "current"
	msg := securityAvailabilityLoadedMsg{
		context:      "stale",
		availability: map[string]bool{"trivy-operator": true},
	}
	updated := m.handleSecurityAvailabilityLoaded(msg)
	assert.False(t, updated.securityAvailabilityByName["trivy-operator"])
}

func TestRenderRightResourcesShowsFindingDetails(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResources
	m.middleItems = []model.Item{
		{
			Name: "CVE-2024-1234",
			Kind: "__security_finding__",
			Columns: []model.KeyValue{
				{Key: "Severity", Value: "CRIT"},
				{Key: "Title", Value: "CVE-2024-1234"},
				{Key: "Resource", Value: "deploy/api"},
			},
		},
	}
	out := m.renderRightResources(80, 20)
	assert.Contains(t, out, "CRIT")
	assert.Contains(t, out, "CVE-2024-1234")
	assert.Contains(t, out, "deploy/api")
}
