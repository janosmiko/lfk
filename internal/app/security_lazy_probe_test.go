package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
)

// TestMaybeProbeSecurityOnFocus covers the lazy security-probe trigger: it
// fires once when the user focuses the Security category at LevelResourceTypes
// and stays quiet otherwise, so opening a cluster never probes (and never hits
// the aws credential plugin) until the user actually looks at security.
func TestMaybeProbeSecurityOnFocus(t *testing.T) {
	newModel := func() Model {
		m := baseModelBoost2()
		m.securityManager = security.NewManager()
		m.nav.Level = model.LevelResourceTypes
		m.nav.Context = "test-ctx"
		return m
	}

	t.Run("probes once when Security focused", func(t *testing.T) {
		m := newModel()
		m.expandedGroup = "Security"
		assert.NotNil(t, m.maybeProbeSecurityOnFocus(), "focusing Security must trigger a probe")
		assert.Equal(t, "test-ctx", m.securityProbedContext)
		assert.Nil(t, m.maybeProbeSecurityOnFocus(), "must not re-probe the same context")
	})

	t.Run("no probe when not focused on Security", func(t *testing.T) {
		m := newModel()
		m.expandedGroup = "Workloads"
		assert.Nil(t, m.maybeProbeSecurityOnFocus())
		assert.Empty(t, m.securityProbedContext, "guard must stay unset when not focused")
	})

	t.Run("no probe outside resource-types level", func(t *testing.T) {
		m := newModel()
		m.expandedGroup = "Security"
		m.nav.Level = model.LevelResources
		assert.Nil(t, m.maybeProbeSecurityOnFocus())
	})

	t.Run("re-probes after guard reset (context switch)", func(t *testing.T) {
		m := newModel()
		m.expandedGroup = "Security"
		assert.NotNil(t, m.maybeProbeSecurityOnFocus())
		m.securityProbedContext = "" // simulates refreshSecuritySources on switch
		assert.NotNil(t, m.maybeProbeSecurityOnFocus(), "must re-probe after the guard is reset")
	})

	t.Run("focuses via selected item category (all-groups-expanded)", func(t *testing.T) {
		m := newModel()
		m.expandedGroup = "Workloads" // not Security
		m.middleItems = []model.Item{{Name: "Trivy", Category: "Security"}}
		m.setCursor(0)
		assert.NotNil(t, m.maybeProbeSecurityOnFocus(), "a selected Security item must trigger the probe")
	})
}
