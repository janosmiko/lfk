package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/security"
)

func TestBuildSecuritySourceEntriesNilManager(t *testing.T) {
	entries := buildSecuritySourceEntries(nil, nil)
	assert.Nil(t, entries)
}

// TestBuildSecuritySourceEntriesProbeInFlight guards the loader-entry
// behaviour: previously, all registered sources were listed eagerly and
// the list "shrunk" to the available subset once the availability probe
// landed — confusing on clusters that only have Heuristic. The fix
// surfaces a single non-navigable loader entry while the probe is in
// flight (and there's no cached availability), so users see a clear
// "(probing sources...)" instead of either ghost-clickable real entries
// or a silently-empty Security category.
func TestBuildSecuritySourceEntriesProbeInFlight(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{NameStr: "heuristic", Available: true})
	mgr.Register(&security.FakeSource{NameStr: "trivy-operator", Available: false})
	mgr.Register(&security.FakeSource{NameStr: "falco", Available: false})
	mgr.Register(&security.FakeSource{NameStr: "policy-report", Available: false})
	// availability map empty -> probe still in flight.
	entries := buildSecuritySourceEntries(mgr, map[string]bool{})
	require.Len(t, entries, 1, "exactly one loader entry while probing")
	assert.Empty(t, entries[0].SourceName,
		"loader sentinel uses empty SourceName so injectSecuritySourceItems builds SecurityLoaderKind")
	assert.Contains(t, entries[0].DisplayName, "probing")

	entries = buildSecuritySourceEntries(mgr, nil)
	require.Len(t, entries, 1, "nil availability is the same in-flight state")
	assert.Empty(t, entries[0].SourceName)
}

func TestBuildSecuritySourceEntriesFiltersUnavailable(t *testing.T) {
	// Sources without a confirmed availability probe result are hidden so
	// clusters without Trivy Operator don't see a dead "Trivy (0)" entry.
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{NameStr: "heuristic", Available: true})
	mgr.Register(&security.FakeSource{NameStr: "trivy-operator", Available: false})
	avail := map[string]bool{
		"heuristic":      true,
		"trivy-operator": false,
	}

	entries := buildSecuritySourceEntries(mgr, avail)
	require.Len(t, entries, 1,
		"unavailable sources must be filtered out")
	assert.Equal(t, "heuristic", entries[0].SourceName)
}

func TestBuildSecuritySourceEntriesFallbackDisplayName(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{NameStr: "custom-scanner", Available: true})
	avail := map[string]bool{"custom-scanner": true}

	entries := buildSecuritySourceEntries(mgr, avail)
	require.Len(t, entries, 1)
	assert.Equal(t, "custom-scanner", entries[0].DisplayName)
	assert.Equal(t, "●", entries[0].Icon.Unicode)
}

func TestBuildSecuritySourceEntriesKnownSources(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{NameStr: "heuristic", Available: true})
	mgr.Register(&security.FakeSource{NameStr: "trivy-operator", Available: true})
	mgr.Register(&security.FakeSource{NameStr: "policy-report", Available: true})
	mgr.Register(&security.FakeSource{NameStr: "kube-bench", Available: true})
	avail := map[string]bool{
		"heuristic":      true,
		"trivy-operator": true,
		"policy-report":  true,
		"kube-bench":     true,
	}

	entries := buildSecuritySourceEntries(mgr, avail)
	require.Len(t, entries, 4)

	displays := map[string]string{}
	for _, e := range entries {
		displays[e.SourceName] = e.DisplayName
	}
	assert.Equal(t, "Heuristic", displays["heuristic"])
	assert.Equal(t, "Trivy", displays["trivy-operator"])
	assert.Equal(t, "Kyverno", displays["policy-report"])
	assert.Equal(t, "CIS", displays["kube-bench"])
}
