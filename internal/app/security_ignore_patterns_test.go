package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/ui"
)

func TestGlobMatch(t *testing.T) {
	cases := []struct {
		pattern, s string
		want       bool
	}{
		{"", "anything", true},    // empty = any
		{"*", "anything", true},   // bare star = any
		{"", "", true},            // empty matches empty
		{"exact", "exact", true},  // exact match
		{"exact", "other", false}, // exact mismatch
		{"CVE-2024-*", "CVE-2024-1234", true},
		{"CVE-2024-*", "CVE-2023-1234", false},
		{"*-system", "kube-system", true},
		{"*-system", "kube-public", false},
		{"kube-*", "kube-system", true},
		{"kube-?", "kube-1", true},
		{"kube-?", "kube-12", false},   // ? = exactly one
		{"a*b*c", "axxbyyc", true},     // multiple stars
		{"a*b*c", "axxbyy", false},     // missing trailing c
		{"ns/*/*", "ns/Pod/web", true}, // slash-agnostic (unlike path.Match)
		{"prod-*", "staging-1", false},
		{"*", "", true},
		{"?", "", false}, // ? needs one char
	}
	for _, c := range cases {
		assert.Equalf(t, c.want, globMatch(c.pattern, c.s),
			"globMatch(%q, %q)", c.pattern, c.s)
	}
}

func TestPatternIsEmpty(t *testing.T) {
	assert.True(t, patternIsEmpty(ui.SecurityIgnorePattern{}))
	assert.True(t, patternIsEmpty(ui.SecurityIgnorePattern{Comment: "note only"}))
	assert.False(t, patternIsEmpty(ui.SecurityIgnorePattern{Source: "trivy-operator"}))
	assert.False(t, patternIsEmpty(ui.SecurityIgnorePattern{Namespace: "kube-system"}))
}

func TestPatternIgnoresResource(t *testing.T) {
	patterns := []ui.SecurityIgnorePattern{
		{Source: "trivy-operator", Group: "CVE-2024-*", Namespace: "kube-system"},
		{Cluster: "prod-*", Source: "heuristic", Group: "no-resource-limits"},
		{}, // empty -> must be skipped, never matches
	}

	// All non-empty fields match.
	assert.True(t, patternIgnoresResource(patterns, "any-ctx", "trivy-operator", "CVE-2024-9", "kube-system"))
	// Namespace differs -> no match.
	assert.False(t, patternIgnoresResource(patterns, "any-ctx", "trivy-operator", "CVE-2024-9", "default"))
	// Group glob differs -> no match.
	assert.False(t, patternIgnoresResource(patterns, "any-ctx", "trivy-operator", "CVE-2023-9", "kube-system"))
	// Cluster glob gates the second pattern (empty namespace = any).
	assert.True(t, patternIgnoresResource(patterns, "prod-eu", "heuristic", "no-resource-limits", "whatever"))
	assert.False(t, patternIgnoresResource(patterns, "staging", "heuristic", "no-resource-limits", "whatever"))
	// Empty pattern must never match.
	assert.False(t, patternIgnoresResource([]ui.SecurityIgnorePattern{{}}, "c", "s", "g", "n"))
}

// Cluster-scoped findings (ClusterRole, etc.) reach patternIgnoresResource with
// an empty namespace (namespaceFromResourceKey("/ClusterRole/admin") == ""). An
// any-namespace pattern must match them; a namespace-specific pattern must not.
func TestPatternIgnoresResourceClusterScoped(t *testing.T) {
	anyNS := []ui.SecurityIgnorePattern{{Source: "heuristic", Group: "check-x"}}
	assert.True(t, patternIgnoresResource(anyNS, "ctx", "heuristic", "check-x", ""),
		"empty-namespace pattern matches a cluster-scoped finding")

	specificNS := []ui.SecurityIgnorePattern{{Source: "heuristic", Group: "check-x", Namespace: "kube-system"}}
	assert.False(t, patternIgnoresResource(specificNS, "ctx", "heuristic", "check-x", ""),
		"namespace-specific pattern must NOT match a cluster-scoped finding")
}

func TestPatternIgnoresGroup(t *testing.T) {
	patterns := []ui.SecurityIgnorePattern{
		{Source: "heuristic", Group: "no-resource-limits"},   // any namespace -> whole group
		{Source: "trivy-operator", Namespace: "kube-system"}, // namespace-scoped -> NOT whole group
		{Source: "falco", Group: "*", Namespace: "*"},        // explicit any-namespace
	}

	// Any-namespace pattern hides the whole group.
	assert.True(t, patternIgnoresGroup(patterns, "ctx", "heuristic", "no-resource-limits"))
	// Explicit "*" namespace also counts as whole-group.
	assert.True(t, patternIgnoresGroup(patterns, "ctx", "falco", "any-rule"))
	// Namespace-scoped pattern does NOT hide the whole group.
	assert.False(t, patternIgnoresGroup(patterns, "ctx", "trivy-operator", "CVE-1"))
	// Non-matching source.
	assert.False(t, patternIgnoresGroup(patterns, "ctx", "policy-report", "x"))
}
