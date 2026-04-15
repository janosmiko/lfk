package k8s

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/security"
)

func TestFindingGroupKey(t *testing.T) {
	cases := []struct {
		name string
		f    security.Finding
		want string
	}{
		{
			name: "heuristic with check label",
			f: security.Finding{
				Labels: map[string]string{"check": "privileged"},
			},
			want: "privileged",
		},
		{
			name: "trivy vuln with cve label",
			f: security.Finding{
				Labels: map[string]string{"cve": "CVE-2024-1234"},
			},
			want: "CVE-2024-1234",
		},
		{
			name: "trivy misconfig with check_id label",
			f: security.Finding{
				Labels: map[string]string{"check_id": "KSV001"},
			},
			want: "KSV001",
		},
		{
			name: "no labels falls back to title",
			f: security.Finding{
				Title: "Foo",
			},
			want: "Foo",
		},
		{
			name: "no labels no title falls back to ID",
			f: security.Finding{
				ID: "abc",
			},
			want: "abc",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, findingGroupKey(tc.f))
		})
	}
}

func TestGroupFindings(t *testing.T) {
	findings := []security.Finding{
		{
			ID: "1", Source: "heuristic", Title: "privileged",
			Severity: security.SeverityCritical,
			Category: security.CategoryMisconfig,
			Labels:   map[string]string{"check": "privileged"},
			Resource: security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "pod-a"},
		},
		{
			ID: "2", Source: "heuristic", Title: "privileged",
			Severity: security.SeverityCritical,
			Category: security.CategoryMisconfig,
			Labels:   map[string]string{"check": "privileged"},
			Resource: security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "pod-b"},
		},
		{
			ID: "3", Source: "heuristic", Title: "host_pid",
			Severity: security.SeverityHigh,
			Category: security.CategoryMisconfig,
			Labels:   map[string]string{"check": "host_pid"},
			Resource: security.ResourceRef{Namespace: "default", Kind: "Pod", Name: "pod-c"},
		},
		{
			ID: "4", Source: "heuristic", Title: "privileged",
			Severity: security.SeverityCritical,
			Category: security.CategoryMisconfig,
			Labels:   map[string]string{"check": "privileged"},
			Resource: security.ResourceRef{Namespace: "kube-system", Kind: "Pod", Name: "pod-d"},
		},
	}

	groups := groupFindings(findings, "heuristic")

	assert.Len(t, groups, 2)

	// Sorted by severity desc, then title asc.
	// Both are Critical/High so "privileged" (Critical) comes first.
	assert.Equal(t, "privileged", groups[0].Key)
	assert.Equal(t, 3, groups[0].Count)
	assert.Equal(t, security.SeverityCritical, groups[0].Severity)

	assert.Equal(t, "host_pid", groups[1].Key)
	assert.Equal(t, 1, groups[1].Count)
	assert.Equal(t, security.SeverityHigh, groups[1].Severity)
}

func TestGroupFindingsHighestSeverityWins(t *testing.T) {
	findings := []security.Finding{
		{
			ID: "1", Source: "heuristic", Title: "some-check",
			Severity: security.SeverityLow,
			Labels:   map[string]string{"check": "some-check"},
			Resource: security.ResourceRef{Namespace: "ns", Kind: "Pod", Name: "a"},
		},
		{
			ID: "2", Source: "heuristic", Title: "some-check",
			Severity: security.SeverityCritical,
			Labels:   map[string]string{"check": "some-check"},
			Resource: security.ResourceRef{Namespace: "ns", Kind: "Pod", Name: "b"},
		},
		{
			ID: "3", Source: "heuristic", Title: "some-check",
			Severity: security.SeverityMedium,
			Labels:   map[string]string{"check": "some-check"},
			Resource: security.ResourceRef{Namespace: "ns", Kind: "Pod", Name: "c"},
		},
	}

	groups := groupFindings(findings, "heuristic")

	assert.Len(t, groups, 1)
	assert.Equal(t, security.SeverityCritical, groups[0].Severity)
}

func TestGroupFindingsFiltersSource(t *testing.T) {
	findings := []security.Finding{
		{
			ID: "1", Source: "heuristic", Title: "privileged",
			Severity: security.SeverityCritical,
			Labels:   map[string]string{"check": "privileged"},
			Resource: security.ResourceRef{Namespace: "ns", Kind: "Pod", Name: "a"},
		},
		{
			ID: "2", Source: "trivy-operator", Title: "CVE-2024-1234",
			Severity: security.SeverityHigh,
			Labels:   map[string]string{"cve": "CVE-2024-1234"},
			Resource: security.ResourceRef{Namespace: "ns", Kind: "Deployment", Name: "api"},
		},
		{
			ID: "3", Source: "heuristic", Title: "host_pid",
			Severity: security.SeverityHigh,
			Labels:   map[string]string{"check": "host_pid"},
			Resource: security.ResourceRef{Namespace: "ns", Kind: "Pod", Name: "b"},
		},
	}

	groups := groupFindings(findings, "heuristic")

	assert.Len(t, groups, 2)
	for _, g := range groups {
		assert.Equal(t, "heuristic", g.Source)
	}
}

func TestFindingGroupToItem(t *testing.T) {
	g := findingGroup{
		Key:          "privileged",
		Title:        "privileged",
		Severity:     security.SeverityCritical,
		Category:     security.CategoryMisconfig,
		Source:       "heuristic",
		Count:        3,
		FindingCount: 5,
		Summary:      "Container running as privileged",
		Details:      "Allows full host access",
		References: []string{
			"https://example.com/ref1",
			"https://example.com/ref2",
		},
	}

	item := findingGroupToItem(g)

	assert.Equal(t, "__security_finding_group__", item.Kind)
	assert.Equal(t, "privileged", item.Name)
	assert.Equal(t, "privileged", item.Extra)
	assert.Empty(t, item.Status, "status dot should not be set — severity column handles coloring")

	assert.Equal(t, "CRIT", item.ColumnValue("Severity"))
	assert.Equal(t, "5", item.ColumnValue("Affected"))
	assert.Equal(t, "misconfig", item.ColumnValue("Category"))
	assert.Equal(t, "heuristic", item.ColumnValue("Source"))
	assert.Empty(t, item.ColumnValue("Title"), "Title removed — Name already shows it")
	assert.Equal(t, "Container running as privileged\n\nAllows full host access",
		item.ColumnValue("Description"))
	assert.Equal(t, "https://example.com/ref1\nhttps://example.com/ref2",
		item.ColumnValue("References"))
}

func TestAffectedResourceToItem(t *testing.T) {
	ref := security.ResourceRef{
		Namespace: "prod",
		Kind:      "Deployment",
		Name:      "api",
	}

	findings := []security.Finding{
		{
			ID: "1", Source: "heuristic", Title: "privileged",
			Severity: security.SeverityCritical,
			Labels:   map[string]string{"check": "privileged"},
			Resource: security.ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "api"},
		},
		{
			ID: "2", Source: "heuristic", Title: "privileged",
			Severity: security.SeverityHigh,
			Labels:   map[string]string{"check": "privileged"},
			Resource: security.ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "api", Container: "sidecar"},
		},
		{
			// Different group key -- should be excluded.
			ID: "3", Source: "heuristic", Title: "host_pid",
			Severity: security.SeverityCritical,
			Labels:   map[string]string{"check": "host_pid"},
			Resource: security.ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "api"},
		},
	}

	item := affectedResourceToItem(ref, "privileged", findings)

	assert.Equal(t, "__security_affected_resource__", item.Kind)
	assert.Equal(t, "deploy/api", item.Name)
	assert.Equal(t, "prod", item.Namespace)
	assert.Empty(t, item.Status)
	assert.Equal(t, "privileged", item.Extra)

	assert.Equal(t, "CRIT", item.ColumnValue("Severity"))
	assert.Equal(t, "deploy/api", item.ColumnValue("Resource"))
	assert.Equal(t, "Deployment", item.ColumnValue("ResourceKind"))
	assert.Equal(t, "prod", item.ColumnValue("Namespace"))
	assert.Equal(t, "2", item.ColumnValue("FindingCount"))
}

func TestGroupFindingsUniqueResources(t *testing.T) {
	// Same finding title on the same pod (two containers) should produce
	// a single unique resource because ResourceRef.Key() omits Container.
	findings := []security.Finding{
		{
			ID: "1", Source: "heuristic", Title: "privileged",
			Severity: security.SeverityCritical,
			Labels:   map[string]string{"check": "privileged"},
			Resource: security.ResourceRef{
				Namespace: "default", Kind: "Pod", Name: "web",
				Container: "app",
			},
		},
		{
			ID: "2", Source: "heuristic", Title: "privileged",
			Severity: security.SeverityCritical,
			Labels:   map[string]string{"check": "privileged"},
			Resource: security.ResourceRef{
				Namespace: "default", Kind: "Pod", Name: "web",
				Container: "sidecar",
			},
		},
	}

	groups := groupFindings(findings, "heuristic")

	assert.Len(t, groups, 1)
	assert.Equal(t, 1, groups[0].Count, "same pod with two containers should count as 1 unique resource")
	assert.Len(t, groups[0].Resources, 1)
}

func TestFindingGroupToItemResourcesColumnForFiltering(t *testing.T) {
	// Simulate 3 different checks all affecting the same pod "bad".
	findings := []security.Finding{
		{
			Source: "heuristic", Title: "Privileged Container", Severity: security.SeverityCritical,
			Labels:   map[string]string{"check": "privileged"},
			Resource: security.ResourceRef{Namespace: "prod", Kind: "Pod", Name: "bad"},
		},
		{
			Source: "heuristic", Title: "Run As Root", Severity: security.SeverityHigh,
			Labels:   map[string]string{"check": "run_as_root"},
			Resource: security.ResourceRef{Namespace: "prod", Kind: "Pod", Name: "bad"},
		},
		{
			Source: "heuristic", Title: "Missing Limits", Severity: security.SeverityMedium,
			Labels:   map[string]string{"check": "missing_resource_limits"},
			Resource: security.ResourceRef{Namespace: "prod", Kind: "Pod", Name: "bad"},
		},
		// Another pod affected by "privileged" only.
		{
			Source: "heuristic", Title: "Privileged Container", Severity: security.SeverityCritical,
			Labels:   map[string]string{"check": "privileged"},
			Resource: security.ResourceRef{Namespace: "prod", Kind: "Pod", Name: "other"},
		},
	}

	groups := groupFindings(findings, "heuristic")
	assert.Len(t, groups, 3, "3 distinct checks = 3 groups")

	// Convert to items and verify ALL groups affecting "bad" have the
	// pod name in the __resources__ column.
	matchCount := 0
	for _, g := range groups {
		item := findingGroupToItem(g)
		res := item.ColumnValue("__resources__")
		assert.NotEmpty(t, res, "group %q must have __resources__ column", g.Key)
		if strings.Contains(res, "bad") {
			matchCount++
		}
	}
	assert.Equal(t, 3, matchCount,
		"all 3 groups affect pod 'bad' and must be filterable by that name")
}
