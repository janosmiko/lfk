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
	assert.False(t, patternIsEmpty(ui.SecurityIgnorePattern{Labels: map[string]string{"k8s-app": "cilium"}}),
		"a labels-only pattern is a real constraint, not a no-op")
}

func TestPatternIgnoresResource(t *testing.T) {
	patterns := []ui.SecurityIgnorePattern{
		{Source: "trivy-operator", Group: "CVE-2024-*", Namespace: "kube-system"},
		{Cluster: "prod-*", Source: "heuristic", Group: "no-resource-limits"},
		{}, // empty -> must be skipped, never matches
	}

	// All non-empty fields match.
	assert.True(t, patternIgnoresResource(patterns, "any-ctx", "trivy-operator", "CVE-2024-9", "kube-system", nil))
	// Namespace differs -> no match.
	assert.False(t, patternIgnoresResource(patterns, "any-ctx", "trivy-operator", "CVE-2024-9", "default", nil))
	// Group glob differs -> no match.
	assert.False(t, patternIgnoresResource(patterns, "any-ctx", "trivy-operator", "CVE-2023-9", "kube-system", nil))
	// Cluster glob gates the second pattern (empty namespace = any).
	assert.True(t, patternIgnoresResource(patterns, "prod-eu", "heuristic", "no-resource-limits", "whatever", nil))
	assert.False(t, patternIgnoresResource(patterns, "staging", "heuristic", "no-resource-limits", "whatever", nil))
	// Empty pattern must never match.
	assert.False(t, patternIgnoresResource([]ui.SecurityIgnorePattern{{}}, "c", "s", "g", "n", nil))
}

func TestPatternIgnoresResourceLabels(t *testing.T) {
	// A labels-only pattern hides any resource carrying matching labels,
	// regardless of source / group / namespace.
	cilium := []ui.SecurityIgnorePattern{{Labels: map[string]string{"k8s-app": "cilium"}}}
	ciliumPod := map[string]string{"k8s-app": "cilium", "pod-template-hash": "abc"}
	otherPod := map[string]string{"app": "web"}

	assert.True(t, patternIgnoresResource(cilium, "ctx", "heuristic", "privileged", "kube-system", ciliumPod),
		"label-only pattern hides a resource with the matching label")
	assert.False(t, patternIgnoresResource(cilium, "ctx", "heuristic", "privileged", "kube-system", otherPod),
		"resource without the matching label stays visible")
	assert.False(t, patternIgnoresResource(cilium, "ctx", "heuristic", "privileged", "kube-system", nil),
		"resource with no labels (unknown) is never hidden by a label pattern")

	// Glob on the label value.
	glob := []ui.SecurityIgnorePattern{{Labels: map[string]string{"app.kubernetes.io/name": "longhorn-*"}}}
	assert.True(t, patternIgnoresResource(glob, "ctx", "trivy-operator", "CVE-1", "longhorn-system",
		map[string]string{"app.kubernetes.io/name": "longhorn-manager"}))
	assert.False(t, patternIgnoresResource(glob, "ctx", "trivy-operator", "CVE-1", "longhorn-system",
		map[string]string{"app.kubernetes.io/name": "csi-attacher"}))

	// Multiple label entries are AND-ed: all must match.
	both := []ui.SecurityIgnorePattern{{Labels: map[string]string{"team": "infra", "tier": "system"}}}
	assert.True(t, patternIgnoresResource(both, "ctx", "heuristic", "x", "ns",
		map[string]string{"team": "infra", "tier": "system"}))
	assert.False(t, patternIgnoresResource(both, "ctx", "heuristic", "x", "ns",
		map[string]string{"team": "infra"}), "one of two label constraints unmet -> no match")

	// Labels combine (AND) with the other fields: a source mismatch wins.
	scoped := []ui.SecurityIgnorePattern{{Source: "heuristic", Labels: map[string]string{"k8s-app": "cilium"}}}
	assert.True(t, patternIgnoresResource(scoped, "ctx", "heuristic", "privileged", "kube-system", ciliumPod))
	assert.False(t, patternIgnoresResource(scoped, "ctx", "trivy-operator", "CVE-1", "kube-system", ciliumPod),
		"source field still gates a label pattern")

	// An empty value glob ("" = any, like the other fields) means "the key must
	// exist with any value" — it still requires the key to be present.
	keyExists := []ui.SecurityIgnorePattern{{Labels: map[string]string{"k8s-app": ""}}}
	assert.True(t, patternIgnoresResource(keyExists, "ctx", "heuristic", "x", "ns",
		map[string]string{"k8s-app": "anything"}), "empty value glob matches any value when the key exists")
	assert.False(t, patternIgnoresResource(keyExists, "ctx", "heuristic", "x", "ns",
		map[string]string{"other": "v"}), "empty value glob still requires the key to be present")
}

// Cluster-scoped findings (ClusterRole, etc.) reach patternIgnoresResource with
// an empty namespace (namespaceFromResourceKey("/ClusterRole/admin") == ""). An
// any-namespace pattern must match them; a namespace-specific pattern must not.
func TestPatternIgnoresResourceClusterScoped(t *testing.T) {
	anyNS := []ui.SecurityIgnorePattern{{Source: "heuristic", Group: "check-x"}}
	assert.True(t, patternIgnoresResource(anyNS, "ctx", "heuristic", "check-x", "", nil),
		"empty-namespace pattern matches a cluster-scoped finding")

	specificNS := []ui.SecurityIgnorePattern{{Source: "heuristic", Group: "check-x", Namespace: "kube-system"}}
	assert.False(t, patternIgnoresResource(specificNS, "ctx", "heuristic", "check-x", "", nil),
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

// An all-"*" namespace glob ("**", "***") means "any namespace" — same as
// globMatch's any-sentinel — so it must hide the whole group, consistent with
// patternIgnoresResource (regression for the "*"-only check).
func TestPatternIgnoresGroup_AllStarNamespaceIsWholeGroup(t *testing.T) {
	patterns := []ui.SecurityIgnorePattern{{Source: "falco", Group: "rule", Namespace: "**"}}
	assert.True(t, patternIgnoresGroup(patterns, "ctx", "falco", "rule"),
		"'**' namespace must count as whole-group")
	assert.True(t, patternIgnoresResource(patterns, "ctx", "falco", "rule", "any-ns", nil),
		"and still match a specific resource (already consistent)")

	// A specific namespace glob stays namespace-scoped (not whole-group).
	scoped := []ui.SecurityIgnorePattern{{Source: "falco", Group: "rule", Namespace: "kube-*"}}
	assert.False(t, patternIgnoresGroup(scoped, "ctx", "falco", "rule"))
}

// A label-bearing pattern is resource-scoped: it must never hide a whole group
// even when its namespace is "any", because labels vary per resource.
func TestPatternIgnoresGroup_LabelPatternIsResourceScoped(t *testing.T) {
	patterns := []ui.SecurityIgnorePattern{
		{Source: "heuristic", Group: "privileged", Labels: map[string]string{"k8s-app": "cilium"}},
	}
	assert.False(t, patternIgnoresGroup(patterns, "ctx", "heuristic", "privileged"),
		"a label pattern must not hide the whole group")
	assert.True(t, patternIgnoresResource(patterns, "ctx", "heuristic", "privileged", "kube-system",
		map[string]string{"k8s-app": "cilium"}),
		"but it still hides matching resources within the group")
}
