package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/security"
)

// TestMaybeEagerSecurityScan covers the cluster-open background scan: it fires
// only when source availability is already known (seeded from the disk cache),
// so a previously-inspected cluster populates SEC badges without navigating to
// Security, while a first-ever visit (empty cache) stays fully lazy.
func TestMaybeEagerSecurityScan(t *testing.T) {
	// The eager-scan gate is enabled in production; this flips it on explicitly
	// so the test is independent of the package default. The disabled path is
	// covered separately by TestMaybeEagerSecurityScanGatedOff.
	withEagerScanEnabled(t)

	newModel := func() Model {
		m := baseModelBoost2()
		m.securityManager = security.NewManager()
		m.nav.Context = "test-ctx"
		return m
	}

	t.Run("scans when cached availability has an available source", func(t *testing.T) {
		m := newModel()
		m.securityAvailabilityByName = map[string]bool{"kubescape": true}
		assert.NotNil(t, m.maybeEagerSecurityScan(),
			"a known-available source must trigger the eager findings scan")
	})

	t.Run("no scan when every cached source is unavailable", func(t *testing.T) {
		m := newModel()
		m.securityAvailabilityByName = map[string]bool{"kubescape": false}
		assert.Nil(t, m.maybeEagerSecurityScan())
	})

	t.Run("no scan with empty cache (first-ever visit stays lazy)", func(t *testing.T) {
		m := newModel()
		m.securityAvailabilityByName = map[string]bool{}
		assert.Nil(t, m.maybeEagerSecurityScan())
	})

	t.Run("no scan when no manager is wired", func(t *testing.T) {
		m := baseModelBoost2()
		m.securityManager = nil
		m.securityAvailabilityByName = map[string]bool{"kubescape": true}
		assert.Nil(t, m.maybeEagerSecurityScan())
	})
}

// TestMaybeEagerSecurityScanGatedOff verifies the feature gate: with eager scan
// disabled (the non-default off path), it never fires even when a source is
// available.
func TestMaybeEagerSecurityScanGatedOff(t *testing.T) {
	orig := eagerSecurityScanEnabled
	eagerSecurityScanEnabled = false
	t.Cleanup(func() { eagerSecurityScanEnabled = orig })

	m := baseModelBoost2()
	m.securityManager = security.NewManager()
	m.nav.Context = "test-ctx"
	m.securityAvailabilityByName = map[string]bool{"kubescape": true}
	assert.Nil(t, m.maybeEagerSecurityScan(),
		"gated off: eager scan must not fire even with an available source")
}

// withEagerScanEnabled flips the eager-scan gate on for the duration of a test.
func withEagerScanEnabled(t *testing.T) {
	t.Helper()
	orig := eagerSecurityScanEnabled
	eagerSecurityScanEnabled = true
	t.Cleanup(func() { eagerSecurityScanEnabled = orig })
}

// TestSecurityBadgeToggleHandler verifies the badge-visibility hotkey flips the
// view flag without touching the manager, cache, or index.
func TestSecurityBadgeToggleHandler(t *testing.T) {
	m := baseModelBoost2()
	assert.False(t, m.hideSecurityBadges, "badges visible by default")

	res, _, handled := m.handleExplorerActionKeySecurityBadgeToggle()
	assert.True(t, handled, "toggle key must be consumed")
	assert.True(t, res.(Model).hideSecurityBadges, "first press hides badges")

	res2, _, _ := res.(Model).handleExplorerActionKeySecurityBadgeToggle()
	assert.False(t, res2.(Model).hideSecurityBadges, "second press shows badges again")
}
