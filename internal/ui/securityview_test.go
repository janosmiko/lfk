package ui

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/security"
)

// -----------------------------------------------------------------------------
// F1 — state helpers
// -----------------------------------------------------------------------------

func TestSecurityViewStateSeverityCounts(t *testing.T) {
	state := SecurityViewState{
		Findings: []security.Finding{
			{Severity: security.SeverityCritical, Category: security.CategoryVuln},
			{Severity: security.SeverityCritical, Category: security.CategoryVuln},
			{Severity: security.SeverityHigh, Category: security.CategoryVuln},
			{Severity: security.SeverityMedium, Category: security.CategoryMisconfig},
			{Severity: security.SeverityLow, Category: security.CategoryMisconfig},
		},
	}
	counts := state.severityCounts()
	assert.Equal(t, 2, counts[security.SeverityCritical])
	assert.Equal(t, 1, counts[security.SeverityHigh])
	assert.Equal(t, 1, counts[security.SeverityMedium])
	assert.Equal(t, 1, counts[security.SeverityLow])
}

func TestSecurityViewStateFilterByCategory(t *testing.T) {
	state := SecurityViewState{
		ActiveCategory: security.CategoryVuln,
		Findings: []security.Finding{
			{ID: "1", Category: security.CategoryVuln},
			{ID: "2", Category: security.CategoryMisconfig},
			{ID: "3", Category: security.CategoryVuln},
		},
	}
	visible := state.visibleFindings()
	assert.Len(t, visible, 2)
	assert.Equal(t, "1", visible[0].ID)
	assert.Equal(t, "3", visible[1].ID)
}

func TestSecurityViewStateEmpty(t *testing.T) {
	state := SecurityViewState{}
	assert.Empty(t, state.visibleFindings())
	assert.Equal(t, 0, state.severityCounts()[security.SeverityCritical])
}

func TestSecurityViewStateUpdatedAgo(t *testing.T) {
	state := SecurityViewState{LastFetch: time.Now().Add(-5 * time.Second)}
	s := state.updatedAgo()
	assert.Contains(t, s, "5s")
}

func TestSecurityViewStateUpdatedAgoNever(t *testing.T) {
	assert.Equal(t, "never", SecurityViewState{}.updatedAgo())
}

func TestSecurityViewStateFilterByResource(t *testing.T) {
	ref := &security.ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "api"}
	state := SecurityViewState{
		ResourceFilter: ref,
		Findings: []security.Finding{
			{ID: "1", Resource: security.ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "api"}},
			{ID: "2", Resource: security.ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "web"}},
		},
	}
	visible := state.visibleFindings()
	assert.Len(t, visible, 1)
	assert.Equal(t, "1", visible[0].ID)
}

func TestSecurityViewStateTextFilter(t *testing.T) {
	state := SecurityViewState{
		Filter: "openssl",
		Findings: []security.Finding{
			{ID: "1", Title: "CVE-2024-1", Summary: "openssl 3.0.7"},
			{ID: "2", Title: "CVE-2024-2", Summary: "glibc 2.36"},
			{ID: "3", Title: "OpenSSL something", Summary: "x"},
		},
	}
	visible := state.visibleFindings()
	assert.Len(t, visible, 2, "case-insensitive substring match on openssl")
}

// -----------------------------------------------------------------------------
// F2 — tiles
// -----------------------------------------------------------------------------

func TestRenderSeverityTiles(t *testing.T) {
	state := SecurityViewState{
		Findings: []security.Finding{
			{Severity: security.SeverityCritical},
			{Severity: security.SeverityCritical},
			{Severity: security.SeverityHigh},
			{Severity: security.SeverityLow},
		},
	}
	out := renderSeverityTiles(state, 80)
	assert.Contains(t, out, "CRIT")
	assert.Contains(t, out, "2")
	assert.Contains(t, out, "HIGH")
	assert.Contains(t, out, "1")
	assert.Contains(t, out, "LOW")
	assert.Contains(t, out, "MED")
	assert.Contains(t, out, "0")
}

func TestRenderSeverityTilesCompact(t *testing.T) {
	state := SecurityViewState{}
	out := renderSeverityTiles(state, 30)
	// Compact form, no styled boxes.
	assert.Contains(t, out, "CRIT 0")
	assert.Contains(t, out, "HIGH 0")
	assert.Contains(t, out, "MED 0")
	assert.Contains(t, out, "LOW 0")
}

// -----------------------------------------------------------------------------
// F3 — tabs
// -----------------------------------------------------------------------------

func TestRenderTabStripShowsAllAvailable(t *testing.T) {
	state := SecurityViewState{
		AvailableCategories: []security.Category{security.CategoryVuln, security.CategoryMisconfig},
		ActiveCategory:      security.CategoryVuln,
		Findings: []security.Finding{
			{Category: security.CategoryVuln},
			{Category: security.CategoryMisconfig},
			{Category: security.CategoryMisconfig},
		},
	}
	out := renderTabStrip(state)
	assert.Contains(t, out, "Vulns 1")
	assert.Contains(t, out, "Misconf 2")
	assert.NotContains(t, out, "Policies", "policies tab must be hidden when not available")
}

func TestRenderTabStripEmptyWhenSingleCategory(t *testing.T) {
	state := SecurityViewState{
		AvailableCategories: []security.Category{security.CategoryMisconfig},
		ActiveCategory:      security.CategoryMisconfig,
	}
	out := renderTabStrip(state)
	assert.Empty(t, out, "single-category dashboards skip the tab strip")
}

// -----------------------------------------------------------------------------
// F4 — table
// -----------------------------------------------------------------------------

func TestRenderFindingsTable(t *testing.T) {
	state := SecurityViewState{
		AvailableCategories: []security.Category{security.CategoryVuln},
		ActiveCategory:      security.CategoryVuln,
		Cursor:              1,
		Findings: []security.Finding{
			{
				ID: "1", Severity: security.SeverityCritical, Source: "trivy-op",
				Title: "CVE-2024-1234", Category: security.CategoryVuln,
				Resource: security.ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "api"},
			},
			{
				ID: "2", Severity: security.SeverityHigh, Source: "trivy-op",
				Title: "CVE-2024-5678", Category: security.CategoryVuln,
				Resource: security.ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "web"},
			},
		},
	}
	out := renderFindingsTable(state, 80, 10)
	assert.Contains(t, out, "deploy/api")
	assert.Contains(t, out, "deploy/web")
	assert.Contains(t, out, "CVE-2024-1234")
	assert.Contains(t, out, "> ", "cursor should highlight row 1")
}

func TestRenderFindingsTableEmpty(t *testing.T) {
	state := SecurityViewState{
		AvailableCategories: []security.Category{security.CategoryVuln},
		ActiveCategory:      security.CategoryVuln,
	}
	out := renderFindingsTable(state, 80, 10)
	assert.Contains(t, out, "No security findings")
}

func TestRenderFindingsTableNarrow(t *testing.T) {
	state := SecurityViewState{
		AvailableCategories: []security.Category{security.CategoryVuln},
		ActiveCategory:      security.CategoryVuln,
		Findings: []security.Finding{
			{
				Severity: security.SeverityCritical, Source: "trivy-op", Title: "X",
				Resource: security.ResourceRef{Namespace: "p", Kind: "Deployment", Name: "api"},
				Category: security.CategoryVuln,
			},
		},
	}
	out := renderFindingsTable(state, 50, 10)
	// At width 50, SOURCE column must be hidden.
	assert.NotContains(t, out, "trivy-op")
	assert.Contains(t, out, "deploy/api")
}

// -----------------------------------------------------------------------------
// F5 — details + status
// -----------------------------------------------------------------------------

func TestRenderDetailsPane(t *testing.T) {
	state := SecurityViewState{
		ActiveCategory: security.CategoryVuln,
		ShowDetail:     true,
		Findings: []security.Finding{
			{
				ID: "1", Title: "CVE-2024-1234", Severity: security.SeverityCritical,
				Summary: "openssl 3.0.7", Details: "Fixed in 3.0.13\n\nA flaw was found...",
				References: []string{"https://nvd.nist.gov/vuln/detail/CVE-2024-1234"},
				Category:   security.CategoryVuln,
				Resource:   security.ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "api"},
			},
		},
	}
	out := renderDetailsPane(state, 80)
	assert.Contains(t, out, "CVE-2024-1234")
	assert.Contains(t, out, "Fixed in 3.0.13")
	assert.Contains(t, out, "https://nvd.nist.gov/vuln/detail/CVE-2024-1234")
}

func TestRenderDetailsPaneHidden(t *testing.T) {
	state := SecurityViewState{ShowDetail: false}
	assert.Empty(t, renderDetailsPane(state, 80))
}

func TestRenderLoadingState(t *testing.T) {
	state := SecurityViewState{Loading: true}
	out := renderSecurityStatusOverlay(state)
	assert.Contains(t, out, "Loading security findings")
}

func TestRenderErrorState(t *testing.T) {
	state := SecurityViewState{LastError: errors.New("probe failed")}
	out := renderSecurityStatusOverlay(state)
	assert.Contains(t, out, "probe failed")
	assert.Contains(t, out, "Press r to retry")
}

func TestRenderNoSourcesAvailable(t *testing.T) {
	state := SecurityViewState{AvailableCategories: nil, Loading: false, LastError: nil}
	out := renderSecurityStatusOverlay(state)
	assert.Contains(t, out, "No security sources available")
}

// -----------------------------------------------------------------------------
// F6 — composition
// -----------------------------------------------------------------------------

func TestRenderSecurityDashboardComposition(t *testing.T) {
	state := SecurityViewState{
		AvailableCategories: []security.Category{security.CategoryVuln, security.CategoryMisconfig},
		ActiveCategory:      security.CategoryVuln,
		LastFetch:           time.Now().Add(-12 * time.Second),
		Findings: []security.Finding{
			{
				ID: "1", Category: security.CategoryVuln, Severity: security.SeverityCritical,
				Title: "CVE-2024-1", Source: "trivy-op",
				Resource: security.ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "api"},
			},
		},
	}
	out := RenderSecurityDashboard(state, 80, 20)
	assert.Contains(t, out, "Security Dashboard")
	assert.Contains(t, out, "CRIT")
	assert.Contains(t, out, "Vulns 1")
	assert.Contains(t, out, "CVE-2024-1")
	assert.Contains(t, out, "ago")
}

func TestRenderSecurityDashboardLoading(t *testing.T) {
	state := SecurityViewState{Loading: true}
	out := RenderSecurityDashboard(state, 80, 20)
	assert.Contains(t, out, "Loading security findings")
}

func TestRenderSecurityDashboardWithFilter(t *testing.T) {
	ref := &security.ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "api"}
	state := SecurityViewState{
		AvailableCategories: []security.Category{security.CategoryVuln},
		ActiveCategory:      security.CategoryVuln,
		ResourceFilter:      ref,
	}
	out := RenderSecurityDashboard(state, 80, 20)
	assert.Contains(t, out, "Security: prod/Deployment/api")
	assert.Contains(t, out, "clear filter")
}

// -----------------------------------------------------------------------------
// F7 — width adaptation
// -----------------------------------------------------------------------------

func TestRenderSecurityDashboardCompactWidth(t *testing.T) {
	state := SecurityViewState{
		AvailableCategories: []security.Category{security.CategoryVuln},
		ActiveCategory:      security.CategoryVuln,
		Findings: []security.Finding{
			{
				Severity: security.SeverityCritical, Title: "X",
				Resource: security.ResourceRef{Namespace: "p", Kind: "Deployment", Name: "api"},
				Category: security.CategoryVuln,
			},
		},
	}
	for _, w := range []int{30, 50, 70, 120} {
		out := RenderSecurityDashboard(state, w, 20)
		assert.NotEmpty(t, out, "width %d should still render", w)
		assert.Contains(t, out, "CRIT", "width %d should include severity", w)
	}
}
