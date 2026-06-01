package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/security"
)

// The findings cache read used to run synchronously inside
// refreshSecuritySources on the Bubble Tea Update goroutine. On clusters with
// many findings the per-host cache file is tens of MB and decoding it took
// seconds, freezing the UI (static spinner, unresponsive quit) at startup and
// on context/tab switch. The read is now deferred into securityFindingsSeedCmd
// and applied via securityFindingsSeedMsg. These tests guard the
// stale-while-revalidate ordering of that handler.

func seedRef() security.ResourceRef {
	return security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "nginx"}
}

func seedIndex() *security.FindingIndex {
	return security.BuildFindingIndex([]security.Finding{
		{ID: "F1", Title: "crit", Severity: security.SeverityCritical, Resource: seedRef()},
	})
}

func TestSecurityFindingsSeedAppliesWhenIndexNil(t *testing.T) {
	m := newTestModelWithSecurity(t, security.NewManager(), "kctx")
	require.Nil(t, m.securityIndex)

	updated := m.updateSecurityFindingsSeed(securityFindingsSeedMsg{
		context: "kctx",
		index:   seedIndex(),
	})

	require.NotNil(t, updated.securityIndex,
		"disk-cached seed must populate the badge index when none is loaded yet")
	assert.Equal(t, 1, updated.securityIndex.For(seedRef()).Critical)
}

func TestSecurityFindingsSeedDoesNotClobberLiveScan(t *testing.T) {
	m := newTestModelWithSecurity(t, security.NewManager(), "kctx")
	live := security.BuildFindingIndex(nil)
	m.securityIndex = live

	updated := m.updateSecurityFindingsSeed(securityFindingsSeedMsg{
		context: "kctx",
		index:   seedIndex(),
	})

	assert.Same(t, live, updated.securityIndex,
		"a live scan result already landed; the older disk seed must not overwrite it")
}

func TestSecurityFindingsSeedStaleContextDiscarded(t *testing.T) {
	m := newTestModelWithSecurity(t, security.NewManager(), "current")

	updated := m.updateSecurityFindingsSeed(securityFindingsSeedMsg{
		context: "other-cluster",
		index:   seedIndex(),
	})

	assert.Nil(t, updated.securityIndex,
		"a seed for a different context must not paint badges on the active cluster")
}

func TestSecurityFindingsSeedNilIndexIgnored(t *testing.T) {
	m := newTestModelWithSecurity(t, security.NewManager(), "kctx")

	updated := m.updateSecurityFindingsSeed(securityFindingsSeedMsg{
		context: "kctx",
		index:   nil, // cache miss
	})

	assert.Nil(t, updated.securityIndex)
}
