package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
