package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func saveSecurityGlobals(t *testing.T) {
	t.Helper()
	pe, ps := ConfigSecurityEnabled, ConfigSecuritySources
	pce, pcs := ConfigClusterSecurityEnabled, ConfigClusterSecuritySources
	t.Cleanup(func() {
		ConfigSecurityEnabled = pe
		ConfigSecuritySources = ps
		ConfigClusterSecurityEnabled = pce
		ConfigClusterSecuritySources = pcs
	})
	ConfigSecurityEnabled = true
	ConfigSecuritySources = map[string]bool{}
	ConfigClusterSecurityEnabled = map[string]bool{}
	ConfigClusterSecuritySources = map[string]map[string]bool{}
}

func TestResolveSecurityEnabled(t *testing.T) {
	t.Run("default enabled", func(t *testing.T) {
		saveSecurityGlobals(t)
		assert.True(t, ResolveSecurityEnabled("any"))
	})

	t.Run("global disable", func(t *testing.T) {
		saveSecurityGlobals(t)
		ConfigSecurityEnabled = false
		assert.False(t, ResolveSecurityEnabled("any"))
	})

	t.Run("per-cluster overrides global", func(t *testing.T) {
		saveSecurityGlobals(t)
		ConfigSecurityEnabled = false
		ConfigClusterSecurityEnabled = map[string]bool{"prod": true}
		assert.True(t, ResolveSecurityEnabled("prod"), "per-cluster true wins over global false")
		assert.False(t, ResolveSecurityEnabled("dev"), "other contexts fall back to global")

		ConfigSecurityEnabled = true
		ConfigClusterSecurityEnabled = map[string]bool{"prod": false}
		assert.False(t, ResolveSecurityEnabled("prod"), "per-cluster false wins over global true")
		assert.True(t, ResolveSecurityEnabled("dev"))
	})
}

func TestResolveSecuritySourceEnabled(t *testing.T) {
	t.Run("default enabled", func(t *testing.T) {
		saveSecurityGlobals(t)
		assert.True(t, ResolveSecuritySourceEnabled("ctx", "trivy-operator"))
		assert.True(t, ResolveSecuritySourceEnabled("ctx", "heuristic"))
	})

	t.Run("global disable via friendly alias", func(t *testing.T) {
		saveSecurityGlobals(t)
		// Internal id "trivy-operator" disabled by its friendly key "trivy".
		ConfigSecuritySources = map[string]bool{"trivy": false, "kyverno": false}
		assert.False(t, ResolveSecuritySourceEnabled("ctx", "trivy-operator"))
		assert.False(t, ResolveSecuritySourceEnabled("ctx", "policy-report"))
		assert.True(t, ResolveSecuritySourceEnabled("ctx", "heuristic"))
	})

	t.Run("global disable via internal id", func(t *testing.T) {
		saveSecurityGlobals(t)
		ConfigSecuritySources = map[string]bool{"falco": false}
		assert.False(t, ResolveSecuritySourceEnabled("ctx", "falco"))
	})

	t.Run("per-cluster source override wins over global", func(t *testing.T) {
		saveSecurityGlobals(t)
		ConfigSecuritySources = map[string]bool{"trivy": false}
		ConfigClusterSecuritySources = map[string]map[string]bool{
			"prod": {"trivy": true},
		}
		assert.True(t, ResolveSecuritySourceEnabled("prod", "trivy-operator"), "per-cluster true wins")
		assert.False(t, ResolveSecuritySourceEnabled("dev", "trivy-operator"), "falls back to global false")
	})
}
