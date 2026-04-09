package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/security"
)

func TestLoadSecurityDashboardNilManager(t *testing.T) {
	m := Model{}
	cmd := m.loadSecurityDashboard()
	assert.Nil(t, cmd)
}

func TestLoadSecurityDashboardDispatchesFetch(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{
		NameStr:       "fake",
		Available:     true,
		CategoriesVal: []security.Category{security.CategoryVuln},
		Findings: []security.Finding{
			{ID: "1", Category: security.CategoryVuln, Severity: security.SeverityHigh, Title: "x"},
		},
	})

	m := Model{securityManager: mgr}
	m.nav.Context = "kctx"

	cmd := m.loadSecurityDashboard()
	require.NotNil(t, cmd)
	msg := cmd()

	loaded, ok := msg.(securityFindingsLoadedMsg)
	require.True(t, ok, "got %T", msg)
	assert.Equal(t, "kctx", loaded.context)
	assert.Len(t, loaded.result.Findings, 1)
}

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
	assert.True(t, loaded.available)
	assert.Equal(t, "kctx", loaded.context)
}
