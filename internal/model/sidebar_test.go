package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSidebarItems_CategorizesBuiltIns(t *testing.T) {
	discovered := []ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
		{Kind: "Deployment", APIGroup: "apps", APIVersion: "v1", Resource: "deployments", Namespaced: true},
		{Kind: "Service", APIGroup: "", APIVersion: "v1", Resource: "services", Namespaced: true},
		{Kind: "StorageClass", APIGroup: "storage.k8s.io", APIVersion: "v1", Resource: "storageclasses", Namespaced: false},
	}

	items := BuildSidebarItems(discovered)

	// Assert the four built-ins appear with their metadata applied.
	cats := collectByDisplay(items)
	require.Contains(t, cats, "Pods")
	assert.Equal(t, "Workloads", cats["Pods"].Category)
	assert.Equal(t, "□", cats["Pods"].Icon.Unicode)

	require.Contains(t, cats, "Deployments")
	assert.Equal(t, "Workloads", cats["Deployments"].Category)

	require.Contains(t, cats, "Services")
	assert.Equal(t, "Networking", cats["Services"].Category)

	require.Contains(t, cats, "StorageClasses")
	assert.Equal(t, "Storage", cats["StorageClasses"].Category)
}

// collectByDisplay indexes items by their display Name for assertions.
func collectByDisplay(items []Item) map[string]Item {
	out := make(map[string]Item, len(items))
	for _, it := range items {
		out[it.Name] = it
	}
	return out
}

func TestBuildSidebarItems_HidesUnknownCoreBuiltIns(t *testing.T) {
	discovered := []ResourceTypeEntry{
		// In BuiltInMetadata
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
		// Core K8s group, not in BuiltInMetadata — must be hidden
		{Kind: "TokenReview", APIGroup: "authentication.k8s.io", APIVersion: "v1", Resource: "tokenreviews", Namespaced: false},
		{Kind: "Binding", APIGroup: "", APIVersion: "v1", Resource: "bindings", Namespaced: true},
		{Kind: "ComponentStatus", APIGroup: "", APIVersion: "v1", Resource: "componentstatuses", Namespaced: false},
	}

	items := BuildSidebarItems(discovered)
	names := make(map[string]bool, len(items))
	for _, it := range items {
		names[it.Name] = true
	}

	assert.True(t, names["Pods"], "known built-in must appear")
	assert.False(t, names["TokenReview"], "uncategorized authentication.k8s.io resource must be hidden")
	assert.False(t, names["Binding"], "uncategorized core resource must be hidden")
	assert.False(t, names["ComponentStatus"], "uncategorized core resource must be hidden")
}

func TestBuildSidebarItems_ShowsCRDsAsGenericEntries(t *testing.T) {
	discovered := []ResourceTypeEntry{
		// An unknown CRD (not in BuiltInMetadata, group is not core).
		{Kind: "Widget", APIGroup: "example.com", APIVersion: "v1", Resource: "widgets", Namespaced: true},
	}

	items := BuildSidebarItems(discovered)

	var widget *Item
	for i := range items {
		if items[i].Kind == "Widget" {
			widget = &items[i]
			break
		}
	}
	require.NotNil(t, widget)
	assert.Equal(t, "example.com", widget.Category)
	assert.Equal(t, "⧫", widget.Icon.Unicode)
	assert.Equal(t, "Widgets", widget.Name)
}

func TestBuildSidebarItems_InjectsPseudoCategories(t *testing.T) {
	items := BuildSidebarItems(nil)

	names := make(map[string]bool, len(items))
	for _, it := range items {
		names[it.Name] = true
	}
	// Dashboard items are injected statically even with a nil discovered set.
	assert.True(t, names["Cluster"], "Dashboards/Cluster must be injected")
	assert.True(t, names["Monitoring"], "Dashboards/Monitoring must be injected")
	// Helm/Releases and Port Forwards are delivered via the discovered set
	// (PseudoResources), so they do NOT appear when discovered is nil.
	assert.False(t, names["Releases"], "Releases should not appear without discovered set")
	assert.False(t, names["Port Forwards"], "Port Forwards should not appear without discovered set")
}

// TestBuildSidebarItems_PseudoResourcesCategorized verifies that the LFK
// pseudo-resources (helm releases, port forwards) produced by
// PseudoResources() are surfaced as sidebar items with their correct
// category and icon via the BuiltInMetadata overlay.
func TestBuildSidebarItems_PseudoResourcesCategorized(t *testing.T) {
	items := BuildSidebarItems(PseudoResources())

	cats := make(map[string]Item, len(items))
	for _, it := range items {
		cats[it.Name] = it
	}

	require.Contains(t, cats, "Releases")
	assert.Equal(t, "Helm", cats["Releases"].Category)
	assert.Equal(t, "HelmRelease", cats["Releases"].Kind)
	assert.Equal(t, "_helm/v1/releases", cats["Releases"].Extra)
	assert.Equal(t, "⎈", cats["Releases"].Icon.Unicode)

	require.Contains(t, cats, "Port Forwards")
	assert.Equal(t, "Networking", cats["Port Forwards"].Category)
	assert.Equal(t, "__port_forwards__", cats["Port Forwards"].Kind)
	assert.Equal(t, "_portforward/v1/portforwards", cats["Port Forwards"].Extra)
}

func TestBuildSidebarItems_PinnedGroupsOrdering(t *testing.T) {
	defer func(orig []string) { PinnedGroups = orig }(PinnedGroups)
	PinnedGroups = []string{"example.com"}

	discovered := []ResourceTypeEntry{
		{Kind: "Widget", APIGroup: "example.com", APIVersion: "v1", Resource: "widgets"},
		{Kind: "Gadget", APIGroup: "zzz.com", APIVersion: "v1", Resource: "gadgets"},
	}

	items := BuildSidebarItems(discovered)

	// Find the first non-core category item — it should come from example.com.
	coreCats := map[string]bool{
		"Dashboards": true, "Cluster": true, "Workloads": true, "Config": true,
		"Networking": true, "Storage": true, "Access Control": true,
		"Helm": true, "API and CRDs": true,
	}
	var firstNonCore *Item
	for i := range items {
		if !coreCats[items[i].Category] {
			firstNonCore = &items[i]
			break
		}
	}
	require.NotNil(t, firstNonCore)
	assert.Equal(t, "example.com", firstNonCore.Category, "pinned group must appear before unpinned")
}

// TestBuildSidebarItems_RareResourcesHiddenByDefault verifies that entries
// marked Rare in BuiltInMetadata are skipped from the default sidebar and
// only surface when ShowRareResources is true. Also verifies that
// uncategorized core Kubernetes resources are hidden by default and
// appear under the "Advanced" category when the toggle is on.
func TestBuildSidebarItems_RareResourcesHiddenByDefault(t *testing.T) {
	defer func(orig bool) { ShowRareResources = orig }(ShowRareResources)

	discovered := []ResourceTypeEntry{
		// Non-rare entry: always visible.
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
		// Rare curated entry.
		{Kind: "CSIDriver", APIGroup: "storage.k8s.io", APIVersion: "v1", Resource: "csidrivers", Namespaced: false},
		// Uncategorized core K8s resource.
		{Kind: "TokenReview", APIGroup: "authentication.k8s.io", APIVersion: "v1", Resource: "tokenreviews", Namespaced: false},
	}

	// Default (ShowRareResources = false): rare entries hidden.
	ShowRareResources = false
	defaultItems := BuildSidebarItems(discovered)
	defaultNames := collectByDisplay(defaultItems)
	assert.Contains(t, defaultNames, "Pods", "Pod must always appear")
	assert.NotContains(t, defaultNames, "CSIDrivers", "rare curated entry must be hidden by default")
	assert.NotContains(t, defaultNames, "Tokenreviews", "uncategorized core resource must be hidden by default")

	// With toggle ON: rare curated surfaces in its category, uncategorized
	// core resources surface under "Advanced".
	ShowRareResources = true
	toggleItems := BuildSidebarItems(discovered)
	toggleNames := collectByDisplay(toggleItems)
	require.Contains(t, toggleNames, "CSIDrivers")
	assert.Equal(t, "Storage", toggleNames["CSIDrivers"].Category)

	require.Contains(t, toggleNames, "Tokenreviews")
	assert.Equal(t, AdvancedCategory, toggleNames["Tokenreviews"].Category)
	assert.Equal(t, "TokenReview", toggleNames["Tokenreviews"].Kind)
}

// TestBuildSidebarItems_CuratedOrderWithinCategory verifies that items in
// a core category follow BuiltInOrderRank (the curated declaration order)
// rather than alphabetical by name. This is the regression guard for the
// order change: Pods must come before Deployments, not after CronJobs.
func TestBuildSidebarItems_CuratedOrderWithinCategory(t *testing.T) {
	discovered := []ResourceTypeEntry{
		// Deliberately pass in reverse/alphabetical order so the sort has
		// to reorder them via BuiltInOrderRank.
		{Kind: "CronJob", APIGroup: "batch", APIVersion: "v1", Resource: "cronjobs", Namespaced: true},
		{Kind: "DaemonSet", APIGroup: "apps", APIVersion: "v1", Resource: "daemonsets", Namespaced: true},
		{Kind: "Deployment", APIGroup: "apps", APIVersion: "v1", Resource: "deployments", Namespaced: true},
		{Kind: "Job", APIGroup: "batch", APIVersion: "v1", Resource: "jobs", Namespaced: true},
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
		{Kind: "ReplicaSet", APIGroup: "apps", APIVersion: "v1", Resource: "replicasets", Namespaced: true},
		{Kind: "StatefulSet", APIGroup: "apps", APIVersion: "v1", Resource: "statefulsets", Namespaced: true},
	}

	items := BuildSidebarItems(discovered)

	// Collect the display names of the Workloads-category items in the
	// order they appear in the sidebar.
	var workloads []string
	for _, it := range items {
		if it.Category == "Workloads" {
			workloads = append(workloads, it.Name)
		}
	}

	expected := []string{"Pods", "Deployments", "ReplicaSets", "StatefulSets", "DaemonSets", "Jobs", "CronJobs"}
	assert.Equal(t, expected, workloads, "workloads must follow curated BuiltInOrderRank order")
}

// TestBuildSidebarItems_InjectsSecuritySources verifies that entries from
// SecuritySourcesFn are injected under the Security category with a synthetic
// APIGroup/Kind and that Count is reflected in the display name.
func TestBuildSidebarItems_InjectsSecuritySources(t *testing.T) {
	defer func(fn func() []SecuritySourceEntry) { SecuritySourcesFn = fn }(SecuritySourcesFn)

	SecuritySourcesFn = func() []SecuritySourceEntry {
		return []SecuritySourceEntry{
			{DisplayName: "Trivy", SourceName: "trivy-operator", Icon: Icon{Unicode: "◈"}, Count: 5},
			{DisplayName: "Heuristic", SourceName: "heuristic", Icon: Icon{Unicode: "◉"}, Count: 12},
		}
	}

	items := BuildSidebarItems(nil)

	var trivy, heuristic *Item
	for i := range items {
		switch items[i].Kind {
		case "__security_trivy-operator__":
			trivy = &items[i]
		case "__security_heuristic__":
			heuristic = &items[i]
		}
	}
	require.NotNil(t, trivy, "Trivy security entry must be injected")
	require.NotNil(t, heuristic, "Heuristic security entry must be injected")

	assert.Equal(t, "Trivy (5)", trivy.Name)
	assert.Equal(t, "Security", trivy.Category)
	assert.Equal(t, "◈", trivy.Icon.Unicode)
	assert.Equal(t, SecurityVirtualAPIGroup+"/v1/findings-trivy-operator", trivy.Extra)

	assert.Equal(t, "Heuristic (12)", heuristic.Name)
	assert.Equal(t, "Security", heuristic.Category)

	// Each entry's Extra must be unique so navigation dispatches to the
	// right source when the user drills in.
	assert.NotEqual(t, trivy.Extra, heuristic.Extra,
		"security entries must have unique Extra values")
}

// TestBuildSidebarItems_NoSecuritySourcesWhenHookNil verifies that the
// Security category remains empty (no items injected) when SecuritySourcesFn
// is unset, and no panic occurs.
func TestBuildSidebarItems_NoSecuritySourcesWhenHookNil(t *testing.T) {
	defer func(fn func() []SecuritySourceEntry) { SecuritySourcesFn = fn }(SecuritySourcesFn)
	SecuritySourcesFn = nil

	items := BuildSidebarItems(nil)

	for _, it := range items {
		assert.NotEqual(t, "Security", it.Category,
			"no Security items should be present when hook is nil")
	}
}

// TestBuildSidebarItems_SecuritySourcesHookReturnsEmpty verifies that an
// empty slice from SecuritySourcesFn is treated the same as nil.
func TestBuildSidebarItems_SecuritySourcesHookReturnsEmpty(t *testing.T) {
	defer func(fn func() []SecuritySourceEntry) { SecuritySourcesFn = fn }(SecuritySourcesFn)
	SecuritySourcesFn = func() []SecuritySourceEntry { return nil }

	items := BuildSidebarItems(nil)

	for _, it := range items {
		assert.NotEqual(t, "Security", it.Category,
			"no Security items should be present when hook returns empty slice")
	}
}

// TestBuildSidebarItems_GroupFallbackCategorizesUnknownNetworking verifies
// that discovered resources in networking.k8s.io or gateway.networking.k8s.io
// that are not yet curated in BuiltInMetadata still surface under the
// "Networking" category (with the generic CRD glyph) instead of being
// hidden. This is the safety net so a new upstream resource is visible
// without manual metadata maintenance.
func TestBuildSidebarItems_GroupFallbackCategorizesUnknownNetworking(t *testing.T) {
	discovered := []ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
		// Not in BuiltInMetadata, but networking.k8s.io is in the fallback.
		{Kind: "FutureNetResource", APIGroup: "networking.k8s.io", APIVersion: "v1alpha1", Resource: "futurenetresources", Namespaced: true},
		// Not in BuiltInMetadata, but gateway.networking.k8s.io is in the fallback.
		// Use a synthetic name so the test keeps exercising the fallback path
		// even if real upstream resources get curated into BuiltInMetadata.
		{Kind: "FutureGatewayResource", APIGroup: "gateway.networking.k8s.io", APIVersion: "v1alpha2", Resource: "futuregatewayresources", Namespaced: true},
	}

	items := BuildSidebarItems(discovered)
	byName := collectByDisplay(items)

	require.Contains(t, byName, "Futurenetresources",
		"unknown networking.k8s.io resource must appear via group fallback")
	assert.Equal(t, "Networking", byName["Futurenetresources"].Category)
	assert.Equal(t, "⧫", byName["Futurenetresources"].Icon.Unicode,
		"fallback items use the generic CRD glyph")

	require.Contains(t, byName, "Futuregatewayresources",
		"unknown gateway.networking.k8s.io resource must appear via group fallback")
	assert.Equal(t, "Networking", byName["Futuregatewayresources"].Category)
}

// TestBuildSidebarItems_GroupFallbackOrderedBeforePortForwards verifies
// that auto-categorized Networking items sort after curated Networking
// entries but before the "Port Forwards" pseudo-resource, so new resources
// slot into a sensible position without pushing the LFK-only tools around.
func TestBuildSidebarItems_GroupFallbackOrderedBeforePortForwards(t *testing.T) {
	discovered := append(PseudoResources(),
		// Curated Gateway API entries.
		ResourceTypeEntry{Kind: "Gateway", APIGroup: "gateway.networking.k8s.io", APIVersion: "v1", Resource: "gateways", Namespaced: true},
		ResourceTypeEntry{Kind: "HTTPRoute", APIGroup: "gateway.networking.k8s.io", APIVersion: "v1", Resource: "httproutes", Namespaced: true},
		// Unknown gateway API resource — must sort via group fallback.
		// Synthetic name so the test keeps exercising the fallback path
		// independent of future BuiltInMetadata additions.
		ResourceTypeEntry{Kind: "FutureGatewayResource", APIGroup: "gateway.networking.k8s.io", APIVersion: "v1alpha2", Resource: "futuregatewayresources", Namespaced: true},
	)

	items := BuildSidebarItems(discovered)

	var networking []string
	for _, it := range items {
		if it.Category == "Networking" {
			networking = append(networking, it.Name)
		}
	}

	// Known curated items must come first in their declared order.
	// The unknown resource must slot after them, before Port Forwards.
	idxGateway := indexOf(networking, "Gateways")
	idxHTTPRoute := indexOf(networking, "HTTPRoutes")
	idxFallback := indexOf(networking, "Futuregatewayresources")
	idxPortFwd := indexOf(networking, "Port Forwards")
	require.GreaterOrEqual(t, idxGateway, 0, "Gateways must appear")
	require.GreaterOrEqual(t, idxHTTPRoute, 0, "HTTPRoutes must appear")
	require.GreaterOrEqual(t, idxFallback, 0, "Futuregatewayresources must appear via fallback")
	require.GreaterOrEqual(t, idxPortFwd, 0, "Port Forwards must appear")

	assert.Less(t, idxGateway, idxFallback,
		"curated Gateways must come before the fallback entry")
	assert.Less(t, idxHTTPRoute, idxFallback,
		"curated HTTPRoutes must come before the fallback entry")
	assert.Less(t, idxFallback, idxPortFwd,
		"the fallback entry must come before Port Forwards")
}

func indexOf(xs []string, s string) int {
	for i, v := range xs {
		if v == s {
			return i
		}
	}
	return -1
}

// TestBuildSidebarItems_SkipsNonListableResources verifies that discovered
// resources whose server-reported Verbs lack "list" are omitted from the
// sidebar entirely — they cannot be listed, so surfacing them under
// Advanced, Networking fallback, or anywhere else just produces 405
// "method not allowed" errors when the user navigates to them.
// Review APIs (tokenreviews, subjectaccessreviews, selfsubject*reviews)
// are the canonical offenders.
func TestBuildSidebarItems_SkipsNonListableResources(t *testing.T) {
	defer func(orig bool) { ShowRareResources = orig }(ShowRareResources)
	ShowRareResources = true // force Advanced surfacing so we can assert it's still skipped

	discovered := []ResourceTypeEntry{
		// Listable resource — must appear.
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true, Verbs: []string{"get", "list", "watch"}},
		// Review APIs (create-only) — must be skipped everywhere.
		{Kind: "TokenReview", APIGroup: "authentication.k8s.io", APIVersion: "v1", Resource: "tokenreviews", Namespaced: false, Verbs: []string{"create"}},
		{Kind: "SubjectAccessReview", APIGroup: "authorization.k8s.io", APIVersion: "v1", Resource: "subjectaccessreviews", Namespaced: false, Verbs: []string{"create"}},
		{Kind: "SelfSubjectReview", APIGroup: "authentication.k8s.io", APIVersion: "v1", Resource: "selfsubjectreviews", Namespaced: false, Verbs: []string{"create"}},
		{Kind: "SelfSubjectAccessReview", APIGroup: "authorization.k8s.io", APIVersion: "v1", Resource: "selfsubjectaccessreviews", Namespaced: false, Verbs: []string{"create"}},
		{Kind: "SelfSubjectRulesReview", APIGroup: "authorization.k8s.io", APIVersion: "v1", Resource: "selfsubjectrulesreviews", Namespaced: false, Verbs: []string{"create"}},
		// Create-only CRD — should also be skipped.
		{Kind: "WriteOnlyThing", APIGroup: "example.com", APIVersion: "v1", Resource: "writeonlythings", Namespaced: true, Verbs: []string{"create"}},
		// Pseudo-resource style: empty Verbs — must still appear (LFK internal).
		{Kind: "HelmRelease", APIGroup: "_helm", APIVersion: "v1", Resource: "releases", Namespaced: true},
	}

	items := BuildSidebarItems(discovered)
	names := collectByDisplay(items)

	assert.Contains(t, names, "Pods", "listable resource must appear")
	assert.Contains(t, names, "Releases", "pseudo-resource with empty Verbs must appear")
	assert.NotContains(t, names, "Tokenreviews", "non-listable review API must be hidden")
	assert.NotContains(t, names, "Subjectaccessreviews", "non-listable review API must be hidden")
	assert.NotContains(t, names, "Selfsubjectreviews", "non-listable review API must be hidden")
	assert.NotContains(t, names, "Selfsubjectaccessreviews", "non-listable review API must be hidden")
	assert.NotContains(t, names, "Selfsubjectrulesreviews", "non-listable review API must be hidden")
	assert.NotContains(t, names, "Writeonlythings", "non-listable CRD must be hidden")
}

func TestTitleCaseFirst(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"widgets", "Widgets"},
		{"a", "A"},
		// Already-uppercase inputs are a no-op for the first char.
		{"Already", "Already"},
		// Non-letter inputs survive unchanged.
		{"123abc", "123abc"},
	}
	for _, tc := range cases {
		got := titleCaseFirst(tc.in)
		assert.Equal(t, tc.want, got, "titleCaseFirst(%q)", tc.in)
	}
}
