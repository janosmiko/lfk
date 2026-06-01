package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/ui"
)

func TestNamespaceFromResourceKey(t *testing.T) {
	assert.Equal(t, "default", namespaceFromResourceKey("default/Pod/web"))
	assert.Equal(t, "kube-system", namespaceFromResourceKey("kube-system/Deployment/dns"))
	assert.Equal(t, "", namespaceFromResourceKey("/ClusterRole/admin")) // cluster-scoped
	assert.Equal(t, "", namespaceFromResourceKey(""))
}

func TestIsNamespaceIgnored(t *testing.T) {
	state := &SecurityIgnoreState{Contexts: make(map[string][]SecurityIgnoreRule)}
	state = addSecurityIgnore(state, "prod", SecurityIgnoreRule{
		Source: "heuristic", GroupKey: "no-limits", Namespace: "monitoring",
	})

	assert.True(t, isNamespaceIgnored(state, "prod", "heuristic", "no-limits", "monitoring"))
	assert.False(t, isNamespaceIgnored(state, "prod", "heuristic", "no-limits", "default"))
	assert.False(t, isNamespaceIgnored(state, "prod", "heuristic", "no-limits", ""), "empty ns never matches")
	assert.False(t, isNamespaceIgnored(state, "staging", "heuristic", "no-limits", "monitoring"))
}

// A namespace-scoped rule must NOT make the whole group count as ignored;
// the group row stays visible and only the scoped namespace is filtered.
func TestIsGroupIgnoredIgnoresNamespaceScopedRules(t *testing.T) {
	state := &SecurityIgnoreState{Contexts: make(map[string][]SecurityIgnoreRule)}
	state = addSecurityIgnore(state, "prod", SecurityIgnoreRule{
		Source: "heuristic", GroupKey: "no-limits", Namespace: "monitoring",
	})
	assert.False(t, isGroupIgnored(state, "prod", "heuristic", "no-limits"))

	// Adding a cluster-wide rule for the same group flips it.
	state = addSecurityIgnore(state, "prod", SecurityIgnoreRule{Source: "heuristic", GroupKey: "no-limits"})
	assert.True(t, isGroupIgnored(state, "prod", "heuristic", "no-limits"))
}

func TestIsResourceIgnoredNamespaceScope(t *testing.T) {
	state := &SecurityIgnoreState{Contexts: make(map[string][]SecurityIgnoreRule)}
	state = addSecurityIgnore(state, "prod", SecurityIgnoreRule{
		Source: "heuristic", GroupKey: "no-limits", Namespace: "monitoring",
	})

	// Resources in the ignored namespace are hidden...
	assert.True(t, isResourceIgnored(state, "prod", "heuristic", "no-limits", "monitoring/Pod/grafana"))
	// ...resources in other namespaces are not.
	assert.False(t, isResourceIgnored(state, "prod", "heuristic", "no-limits", "default/Pod/web"))
	// ...and a cluster-scoped finding (empty ns) is not caught by the ns rule.
	assert.False(t, isResourceIgnored(state, "prod", "heuristic", "no-limits", "/ClusterRole/admin"))
}

func TestAddSecurityIgnoreNamespaceDedupAndDistinct(t *testing.T) {
	state := &SecurityIgnoreState{Contexts: make(map[string][]SecurityIgnoreRule)}

	// Same (source, group, namespace) dedups.
	state = addSecurityIgnore(state, "prod", SecurityIgnoreRule{Source: "h", GroupKey: "g", Namespace: "ns1", Comment: "a"})
	state = addSecurityIgnore(state, "prod", SecurityIgnoreRule{Source: "h", GroupKey: "g", Namespace: "ns1", Comment: "b"})
	require.Len(t, state.Contexts["prod"], 1)
	assert.Equal(t, "b", state.Contexts["prod"][0].Comment)

	// Different namespace is a distinct rule.
	state = addSecurityIgnore(state, "prod", SecurityIgnoreRule{Source: "h", GroupKey: "g", Namespace: "ns2"})
	require.Len(t, state.Contexts["prod"], 2)

	// A namespace rule and a cluster-wide rule for the same group are distinct.
	state = addSecurityIgnore(state, "prod", SecurityIgnoreRule{Source: "h", GroupKey: "g"})
	require.Len(t, state.Contexts["prod"], 3)
}

func TestRemoveSecurityIgnoreNamespace(t *testing.T) {
	state := &SecurityIgnoreState{Contexts: make(map[string][]SecurityIgnoreRule)}
	state = addSecurityIgnore(state, "prod", SecurityIgnoreRule{Source: "h", GroupKey: "g", Namespace: "ns1"})
	state = addSecurityIgnore(state, "prod", SecurityIgnoreRule{Source: "h", GroupKey: "g"}) // cluster-wide
	require.Len(t, state.Contexts["prod"], 2)

	// Removing the namespace rule leaves the cluster-wide rule intact.
	state = removeSecurityIgnore(state, "prod", "h", "g", "ns1", "")
	require.Len(t, state.Contexts["prod"], 1)
	assert.Equal(t, "", state.Contexts["prod"][0].Namespace)
}

func TestSecurityIgnoresNamespaceRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	state := &SecurityIgnoreState{Contexts: make(map[string][]SecurityIgnoreRule)}
	state = addSecurityIgnore(state, "prod", SecurityIgnoreRule{
		Source: "heuristic", GroupKey: "no-limits", Namespace: "monitoring", Comment: "noisy in monitoring",
	})
	require.NoError(t, saveSecurityIgnores(state))

	loaded := loadSecurityIgnores()
	require.Len(t, loaded.Contexts["prod"], 1)
	assert.Equal(t, "monitoring", loaded.Contexts["prod"][0].Namespace)
	assert.Empty(t, loaded.Contexts["prod"][0].Resource)
}

// TestModelIgnoreCheckerCombinesStateAndConfig verifies the checker honors
// BOTH the interactive YAML rules and the config-file glob patterns.
func TestModelIgnoreCheckerCombinesStateAndConfig(t *testing.T) {
	saved := ui.ConfigSecurityIgnorePatterns
	t.Cleanup(func() { ui.ConfigSecurityIgnorePatterns = saved })

	ui.ConfigSecurityIgnorePatterns = []ui.SecurityIgnorePattern{
		{Cluster: "prod", Source: "trivy-operator", Group: "CVE-2024-*", Namespace: "kube-system"},
		{Source: "falco", Group: "*"}, // any-namespace -> whole group
	}

	state := &SecurityIgnoreState{Contexts: map[string][]SecurityIgnoreRule{
		"prod": {{Source: "heuristic", GroupKey: "manual-ignore"}}, // cluster-wide YAML rule
	}}
	c := newModelIgnoreChecker(state, "prod")

	// YAML rule still works.
	assert.True(t, c.IsGroupIgnored("heuristic", "manual-ignore"))
	assert.True(t, c.IsResourceIgnored("heuristic", "manual-ignore", "default/Pod/x"))

	// Config namespace-scoped pattern: hides the resource in kube-system only,
	// and does NOT mark the whole group ignored.
	assert.True(t, c.IsResourceIgnored("trivy-operator", "CVE-2024-1", "kube-system/Pod/x"))
	assert.False(t, c.IsResourceIgnored("trivy-operator", "CVE-2024-1", "default/Pod/x"))
	assert.False(t, c.IsGroupIgnored("trivy-operator", "CVE-2024-1"))

	// Config any-namespace pattern hides the whole falco group, including
	// cluster-scoped findings (empty namespace from "/ClusterRole/admin").
	assert.True(t, c.IsGroupIgnored("falco", "any-rule"))
	assert.True(t, c.IsResourceIgnored("falco", "any-rule", "default/Pod/x"))
	assert.True(t, c.IsResourceIgnored("falco", "any-rule", "/ClusterRole/admin"))

	// Cluster glob gates: a different context is unaffected by the prod pattern.
	other := newModelIgnoreChecker(state, "staging")
	assert.False(t, other.IsResourceIgnored("trivy-operator", "CVE-2024-1", "kube-system/Pod/x"))
}
