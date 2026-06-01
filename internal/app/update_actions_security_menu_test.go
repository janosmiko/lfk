package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// securityMenuModel builds a Model positioned on a __security_affected_resource__
// row of the heuristic source in context "prod", namespace "monitoring".
func securityMenuModel(t *testing.T) Model {
	t.Helper()
	m := Model{}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__security_heuristic__"}
	m.nav.Context = "prod"
	m.securityIgnores = &SecurityIgnoreState{Contexts: map[string][]SecurityIgnoreRule{}}
	m.middleItems = []model.Item{{
		Kind:      "__security_affected_resource__",
		Name:      "pod/grafana",
		Namespace: "monitoring",
		Extra:     "no-limits", // group key
		Columns:   []model.KeyValue{{Key: "__resource_key__", Value: "monitoring/Pod/grafana"}},
	}}
	m.setCursor(0)
	return m
}

func menuItemNames(items []model.Item) []string {
	names := make([]string, 0, len(items))
	for _, it := range items {
		names = append(names, it.Name)
	}
	return names
}

func TestSecurityActionMenuOffersNamespaceIgnore(t *testing.T) {
	m := securityMenuModel(t)
	m = m.openSecurityActionMenu()

	names := menuItemNames(m.overlayItems)
	assert.Contains(t, names, "Ignore (Group)")
	assert.Contains(t, names, "Ignore (Namespace)")
	assert.Contains(t, names, "Ignore (This Resource)")
	assert.NotContains(t, names, "Un-ignore", "nothing ignored yet")
}

func TestSecurityActionMenuNamespaceIgnoreAddsRule(t *testing.T) {
	m := securityMenuModel(t)
	res, _ := m.executeSecurityIgnoreAction("Ignore (Namespace)")
	rm := res.(Model)

	rules := rm.securityIgnores.Contexts["prod"]
	require.Len(t, rules, 1)
	assert.Equal(t, "heuristic", rules[0].Source)
	assert.Equal(t, "no-limits", rules[0].GroupKey)
	assert.Equal(t, "monitoring", rules[0].Namespace)
	assert.Empty(t, rules[0].Resource, "namespace rule must not pin a resource")
	assert.Contains(t, rm.statusMessage, "monitoring")
}

// Once the namespace is ignored, the menu drops the namespace + resource
// options (already covered) and offers Un-ignore instead.
func TestSecurityActionMenuAfterNamespaceIgnore(t *testing.T) {
	m := securityMenuModel(t)
	m.securityIgnores = addSecurityIgnore(m.securityIgnores, "prod", SecurityIgnoreRule{
		Source: "heuristic", GroupKey: "no-limits", Namespace: "monitoring",
	})
	m = m.openSecurityActionMenu()

	names := menuItemNames(m.overlayItems)
	assert.NotContains(t, names, "Ignore (Namespace)", "already namespace-ignored")
	assert.NotContains(t, names, "Ignore (This Resource)", "namespace ignore already covers the resource")
	assert.Contains(t, names, "Ignore (Group)", "escalating to whole-cluster is still allowed")
	assert.Contains(t, names, "Un-ignore")
}

// Un-ignore peels the most specific rule first: with both a resource rule and
// a namespace rule present, it removes the resource rule and leaves the
// namespace rule intact.
func TestSecurityActionMenuUnignorePrefersResourceRule(t *testing.T) {
	m := securityMenuModel(t)
	m.securityIgnores = addSecurityIgnore(m.securityIgnores, "prod", SecurityIgnoreRule{
		Source: "heuristic", GroupKey: "no-limits", Namespace: "monitoring",
	})
	m.securityIgnores = addSecurityIgnore(m.securityIgnores, "prod", SecurityIgnoreRule{
		Source: "heuristic", GroupKey: "no-limits", Resource: "monitoring/Pod/grafana",
	})

	res, _ := m.executeSecurityIgnoreAction("Un-ignore")
	rm := res.(Model)

	rules := rm.securityIgnores.Contexts["prod"]
	require.Len(t, rules, 1, "only the resource rule should be removed")
	assert.Equal(t, "monitoring", rules[0].Namespace)
	assert.Empty(t, rules[0].Resource)
}

// Un-ignore on an affected-resource row with ONLY a namespace rule peels that
// namespace rule (the middle precedence rung).
func TestSecurityActionMenuUnignoreNamespaceOnly(t *testing.T) {
	m := securityMenuModel(t)
	m.securityIgnores = addSecurityIgnore(m.securityIgnores, "prod", SecurityIgnoreRule{
		Source: "heuristic", GroupKey: "no-limits", Namespace: "monitoring",
	})

	res, _ := m.executeSecurityIgnoreAction("Un-ignore")
	rm := res.(Model)

	assert.Empty(t, rm.securityIgnores.Contexts["prod"], "the namespace rule should be removed")
}

// Un-ignore from a finding-group row removes the cluster-wide group rule
// (sel.Kind != affected_resource, so namespace/resource stay empty).
func TestSecurityActionMenuUnignoreFromGroupRow(t *testing.T) {
	m := securityMenuModel(t)
	m.middleItems = []model.Item{{
		Kind:  "__security_finding_group__",
		Name:  "no-limits",
		Extra: "no-limits",
	}}
	m.setCursor(0)
	m.securityIgnores = addSecurityIgnore(m.securityIgnores, "prod", SecurityIgnoreRule{
		Source: "heuristic", GroupKey: "no-limits",
	})

	res, _ := m.executeSecurityIgnoreAction("Un-ignore")
	rm := res.(Model)

	assert.Empty(t, rm.securityIgnores.Contexts["prod"], "the cluster-wide group rule should be removed")
}

// When a broader rule still hides the row after un-ignore, the status message
// says so instead of a misleading bare "Un-ignored".
func TestSecurityActionMenuUnignoreWarnsWhenBroaderRuleRemains(t *testing.T) {
	m := securityMenuModel(t)
	// Both a namespace rule and a cluster-wide group rule (e.g. hand-edited YAML).
	m.securityIgnores = addSecurityIgnore(m.securityIgnores, "prod", SecurityIgnoreRule{
		Source: "heuristic", GroupKey: "no-limits", Namespace: "monitoring",
	})
	m.securityIgnores = addSecurityIgnore(m.securityIgnores, "prod", SecurityIgnoreRule{
		Source: "heuristic", GroupKey: "no-limits",
	})

	res, _ := m.executeSecurityIgnoreAction("Un-ignore")
	rm := res.(Model)

	assert.Contains(t, rm.statusMessage, "still hidden by a broader rule")
}

// dispatchSecurityActionIfApplicable must NOT intercept labels off a security
// view, so a generic "Refresh" elsewhere keeps its normal semantics.
func TestDispatchSecurityActionGatedOffSecurityView(t *testing.T) {
	m := Model{}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}

	_, _, handled := m.dispatchSecurityActionIfApplicable("Refresh")
	assert.False(t, handled, "off a security view the security dispatch must not fire")
}

func TestDispatchSecurityActionOnSecurityView(t *testing.T) {
	for _, label := range []string{"Ignore (Group)", "Ignore (Namespace)", "Ignore (This Resource)", "Un-ignore", "Refresh"} {
		m := securityMenuModel(t)
		_, _, handled := m.dispatchSecurityActionIfApplicable(label)
		assert.Truef(t, handled, "label %q must be handled on a security view", label)
	}
}

// openSecurityActionMenuIfApplicable also fires via the selected-item fallback
// when the nav resource type is a normal kind but a security row is selected.
func TestOpenSecurityActionMenuFallbackBySelectedItem(t *testing.T) {
	m := Model{}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"} // normal kind
	m.securityIgnores = &SecurityIgnoreState{Contexts: map[string][]SecurityIgnoreRule{}}
	m.middleItems = []model.Item{{
		Kind:      "__security_affected_resource__",
		Name:      "pod/grafana",
		Namespace: "monitoring",
		Extra:     "no-limits",
		Columns:   []model.KeyValue{{Key: "__resource_key__", Value: "monitoring/Pod/grafana"}},
	}}
	m.setCursor(0)

	_, ok := m.openSecurityActionMenuIfApplicable()
	assert.True(t, ok, "a selected security row must open the security menu even under a normal nav kind")
}
