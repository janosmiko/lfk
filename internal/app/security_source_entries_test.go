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

func TestBuildSecuritySourceEntriesShowsAllRegistered(t *testing.T) {
	// All registered sources are shown regardless of availability — missing
	// external dependencies surface at fetch time, not at category-build time.
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{NameStr: "heuristic", Available: true})
	mgr.Register(&security.FakeSource{NameStr: "trivy-operator", Available: false})
	avail := map[string]bool{
		"heuristic":      true,
		"trivy-operator": false,
	}

	entries := buildSecuritySourceEntries(mgr, avail)
	require.Len(t, entries, 2,
		"registered sources should always appear, even when availability probe is false")
	names := map[string]bool{}
	for _, e := range entries {
		names[e.SourceName] = true
	}
	assert.True(t, names["heuristic"])
	assert.True(t, names["trivy-operator"])
}

func TestBuildSecuritySourceEntriesFallbackDisplayName(t *testing.T) {
	mgr := security.NewManager()
	mgr.Register(&security.FakeSource{NameStr: "custom-scanner", Available: true})
	avail := map[string]bool{"custom-scanner": true}

	entries := buildSecuritySourceEntries(mgr, avail)
	require.Len(t, entries, 1)
	assert.Equal(t, "custom-scanner", entries[0].DisplayName)
	assert.Equal(t, "●", entries[0].Icon)
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
