package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
)

// withIsolatedActiveState saves and restores global ui.Active* state touched by
// these tests so parallel/sequential tests do not leak state.
func withIsolatedActiveState(t *testing.T, idx *security.FindingIndex, available bool) {
	t.Helper()
	origIdx := ActiveSecurityIndex
	origAvail := ActiveSecurityAvailable
	origHidden := ActiveSecurityBadgesHidden
	origQuery := ActiveHighlightQuery
	origMS := ActiveMiddleScroll
	origSel := ActiveSelectedItems

	ActiveSecurityIndex = idx
	ActiveSecurityAvailable = available
	ActiveSecurityBadgesHidden = false
	ActiveHighlightQuery = ""
	ActiveMiddleScroll = -1
	ActiveSelectedItems = nil

	t.Cleanup(func() {
		ActiveSecurityIndex = origIdx
		ActiveSecurityAvailable = origAvail
		ActiveSecurityBadgesHidden = origHidden
		ActiveHighlightQuery = origQuery
		ActiveMiddleScroll = origMS
		ActiveSelectedItems = origSel
	})
}

func buildIndex(t *testing.T, findings ...security.Finding) *security.FindingIndex {
	t.Helper()
	return security.BuildFindingIndex(findings)
}

// --- securityBadgeFor ---

func TestSecurityBadgeForWithFindings(t *testing.T) {
	idx := buildIndex(t,
		security.Finding{
			Title:    "CVE-2024-3001",
			Severity: security.SeverityCritical,
			Resource: security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "api"},
		},
		security.Finding{
			Title:    "CVE-2024-3002",
			Severity: security.SeverityHigh,
			Resource: security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "api"},
		},
		security.Finding{
			Title:    "CVE-2024-3003",
			Severity: security.SeverityLow,
			Resource: security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "api"},
		},
	)

	got := securityBadgeFor(idx, security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "api"})
	plain := ansi.Strip(got)

	// 1 Critical (plus 1 High, 1 Low) -> the badge shows only the worst-severity
	// count so a red badge can never be misread as a multi-severity total.
	assert.NotEmpty(t, got, "badge must not be empty when findings exist")
	assert.Contains(t, plain, "\u25cf", "critical badge must use the filled circle symbol")
	assert.Equal(t, StatusFailed.Render("\u25cf1"), got,
		"critical badge must show only the critical count in StatusFailed style")
}

// TestSecurityBadgeForShowsOnlyWorstCount locks the user-facing rule for the
// case that triggered this change: a resource with a few criticals among many
// lower-severity findings must badge "\u25cf3" (3 criticals), never "\u25cf119".
func TestSecurityBadgeForShowsOnlyWorstCount(t *testing.T) {
	findings := []security.Finding{}
	ref := security.ResourceRef{Namespace: "argo-cd", Kind: "Pod", Name: "redis"}
	add := func(sev security.Severity, n int, prefix string) {
		for i := range n {
			findings = append(findings, security.Finding{
				Title:    prefix + string(rune('a'+i)),
				Severity: sev,
				Resource: ref,
			})
		}
	}
	add(security.SeverityCritical, 3, "crit-")
	add(security.SeverityHigh, 6, "high-")
	add(security.SeverityLow, 110, "low-")

	idx := buildIndex(t, findings...)
	plain := ansi.Strip(securityBadgeFor(idx, ref))

	assert.Equal(t, "\u25cf3", plain,
		"badge must show 3 criticals only, not the 119 all-severity total")
}

func TestSecurityBadgeForHighUsesOrange(t *testing.T) {
	idx := buildIndex(t,
		security.Finding{
			Title:    "CVE-2024-4001",
			Severity: security.SeverityHigh,
			Resource: security.ResourceRef{Namespace: "ns", Kind: "Deployment", Name: "web"},
		},
		security.Finding{
			Title:    "CVE-2024-4002",
			Severity: security.SeverityMedium,
			Resource: security.ResourceRef{Namespace: "ns", Kind: "Deployment", Name: "web"},
		},
	)

	got := securityBadgeFor(idx, security.ResourceRef{Namespace: "ns", Kind: "Deployment", Name: "web"})
	plain := ansi.Strip(got)

	assert.Contains(t, plain, "\u25d0", "high badge must use the half circle symbol")
	assert.Equal(t, DeprecationStyle.Render("\u25d01"), got,
		"high badge must show only the high count in DeprecationStyle")
}

func TestSecurityBadgeForMediumUsesProgressing(t *testing.T) {
	idx := buildIndex(t,
		security.Finding{
			Severity: security.SeverityMedium,
			Resource: security.ResourceRef{Namespace: "ns", Kind: "Pod", Name: "x"},
		},
	)

	got := securityBadgeFor(idx, security.ResourceRef{Namespace: "ns", Kind: "Pod", Name: "x"})
	plain := ansi.Strip(got)

	assert.Contains(t, plain, "\u25cb", "medium badge must use the empty circle symbol")
	assert.Contains(t, plain, "1", "badge must include the total finding count")
	assert.Equal(t, StatusProgressing.Render("\u25cb1"), got,
		"medium/low badge must use StatusProgressing style")
}

func TestSecurityBadgeForLowUsesProgressing(t *testing.T) {
	idx := buildIndex(t,
		security.Finding{
			Severity: security.SeverityLow,
			Resource: security.ResourceRef{Namespace: "ns", Kind: "Pod", Name: "y"},
		},
	)

	got := securityBadgeFor(idx, security.ResourceRef{Namespace: "ns", Kind: "Pod", Name: "y"})
	plain := ansi.Strip(got)

	assert.Contains(t, plain, "\u25cb")
	assert.Contains(t, plain, "1")
}

// --- securityBadgeFor empty / nil cases ---

func TestSecurityBadgeForEmpty(t *testing.T) {
	idx := security.BuildFindingIndex(nil)
	got := securityBadgeFor(idx, security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "no-finds"})
	assert.Empty(t, got, "empty index must yield empty badge")
}

func TestSecurityBadgeForNilIndex(t *testing.T) {
	got := securityBadgeFor(nil, security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "whatever"})
	assert.Empty(t, got, "nil index must yield empty badge")
}

func TestSecurityBadgeForResourceWithoutFindings(t *testing.T) {
	idx := buildIndex(t,
		security.Finding{
			Severity: security.SeverityCritical,
			Resource: security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "api"},
		},
	)
	got := securityBadgeFor(idx, security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "other"})
	assert.Empty(t, got, "resource without findings must yield empty badge")
}

// --- RenderTable gating ---

func TestRenderTableHidesSecurityBadgeWhenUnavailable(t *testing.T) {
	idx := buildIndex(t,
		security.Finding{
			Severity: security.SeverityCritical,
			Resource: security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "nginx"},
		},
	)
	// Even though an index is populated, ActiveSecurityAvailable=false must
	// suppress the badge so the explorer looks identical to clusters with no
	// security sources.
	withIsolatedActiveState(t, idx, false)

	items := []model.Item{
		{Name: "nginx", Namespace: "default", Kind: "Pod", Status: "Running"},
	}
	result := RenderTable("NAME", items, 0, 80, 20, false, "", "")
	plain := ansi.Strip(result)

	assert.NotContains(t, plain, "\u25cf", "badge must be absent when security is unavailable")
	assert.NotContains(t, plain, "\u25d0")
	assert.NotContains(t, plain, "\u25cb")
}

func TestRenderTableShowsSecurityBadgeWhenAvailable(t *testing.T) {
	idx := buildIndex(t,
		security.Finding{
			Severity: security.SeverityCritical,
			Resource: security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "nginx"},
		},
	)
	withIsolatedActiveState(t, idx, true)

	items := []model.Item{
		{Name: "nginx", Namespace: "default", Kind: "Pod", Status: "Running"},
	}
	result := RenderTable("NAME", items, 0, 80, 20, false, "", "")
	plain := ansi.Strip(result)

	// The row with findings must carry the critical badge and its count.
	assert.True(t,
		strings.Contains(plain, "\u25cf1"),
		"expected critical badge with count=1 in rendered table, got:\n%s", plain)
}

func TestRenderTableHidesSecurityBadgeWhenToggledOff(t *testing.T) {
	idx := buildIndex(t,
		security.Finding{
			Severity: security.SeverityCritical,
			Resource: security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "nginx"},
		},
	)
	// Source available and findings indexed, but the user toggled badges off.
	withIsolatedActiveState(t, idx, true)
	ActiveSecurityBadgesHidden = true

	items := []model.Item{
		{Name: "nginx", Namespace: "default", Kind: "Pod", Status: "Running"},
	}
	result := RenderTable("NAME", items, 0, 80, 20, false, "", "")
	plain := ansi.Strip(result)

	assert.NotContains(t, plain, "●", "badge must be hidden when toggled off")
	assert.NotContains(t, plain, "◐")
	assert.NotContains(t, plain, "○")
}

func TestRenderTableShowsNoBadgeForResourceWithoutFindings(t *testing.T) {
	idx := buildIndex(t,
		security.Finding{
			Severity: security.SeverityCritical,
			Resource: security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "nginx"},
		},
	)
	withIsolatedActiveState(t, idx, true)

	items := []model.Item{
		// A different pod — no matching findings.
		{Name: "redis", Namespace: "default", Kind: "Pod", Status: "Running"},
	}
	result := RenderTable("NAME", items, 0, 80, 20, false, "", "")
	plain := ansi.Strip(result)

	assert.NotContains(t, plain, "\u25cf", "no badge expected for a pod with zero findings")
}
