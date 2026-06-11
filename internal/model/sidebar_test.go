package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDisplayNameFromKind(t *testing.T) {
	tests := []struct {
		name   string
		kind   string
		plural string
		want   string
	}{
		{"camel-case kind plus s", "ApplicationSet", "applicationsets", "ApplicationSets"},
		{"simple kind plus s", "Widget", "widgets", "Widgets"},
		{"kind plus es", "Ingress", "ingresses", "Ingresses"},
		{"camel kind plus es", "StorageClass", "storageclasses", "StorageClasses"},
		{"y to ies", "NetworkPolicy", "networkpolicies", "NetworkPolicies"},
		{"regular y plural keeps kind", "Gateway", "gateways", "Gateways"},
		{"irregular plural falls back to plural", "Endpoints", "endpoints", "Endpoints"},
		{"mismatched plural falls back", "Foo", "bars", "Bars"},
		{"empty kind falls back to plural", "", "widgets", "Widgets"},
		{"empty plural", "Widget", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, displayNameFromKind(tt.kind, tt.plural))
		})
	}
}

func TestBuildSidebarItems_CamelCasesCRDFromKind(t *testing.T) {
	discovered := []ResourceTypeEntry{
		// Custom CRD whose plural is the lowercased kind + "s". The display
		// name should recover the kind's camel case instead of "Applicationsets".
		{Kind: "ApplicationSet", APIGroup: "example.com", APIVersion: "v1", Resource: "applicationsets", Namespaced: true},
	}

	items := BuildSidebarItems(discovered)

	var got *Item
	for i := range items {
		if items[i].Kind == "ApplicationSet" {
			got = &items[i]
			break
		}
	}
	require.NotNil(t, got)
	assert.Equal(t, "ApplicationSets", got.Name)
}

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
	assert.NotContains(t, defaultNames, "TokenReviews", "uncategorized core resource must be hidden by default")

	// With toggle ON: rare curated surfaces in its category, uncategorized
	// core resources surface under "Advanced".
	ShowRareResources = true
	toggleItems := BuildSidebarItems(discovered)
	toggleNames := collectByDisplay(toggleItems)
	require.Contains(t, toggleNames, "CSIDrivers")
	assert.Equal(t, "Storage", toggleNames["CSIDrivers"].Category)

	require.Contains(t, toggleNames, "TokenReviews")
	assert.Equal(t, AdvancedCategory, toggleNames["TokenReviews"].Category)
	assert.Equal(t, "TokenReview", toggleNames["TokenReviews"].Kind)
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

	require.Contains(t, byName, "FutureNetResources",
		"unknown networking.k8s.io resource must appear via group fallback")
	assert.Equal(t, "Networking", byName["FutureNetResources"].Category)
	assert.Equal(t, "⧫", byName["FutureNetResources"].Icon.Unicode,
		"fallback items use the generic CRD glyph")

	require.Contains(t, byName, "FutureGatewayResources",
		"unknown gateway.networking.k8s.io resource must appear via group fallback")
	assert.Equal(t, "Networking", byName["FutureGatewayResources"].Category)
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
	idxFallback := indexOf(networking, "FutureGatewayResources")
	idxPortFwd := indexOf(networking, "Port Forwards")
	require.GreaterOrEqual(t, idxGateway, 0, "Gateways must appear")
	require.GreaterOrEqual(t, idxHTTPRoute, 0, "HTTPRoutes must appear")
	require.GreaterOrEqual(t, idxFallback, 0, "FutureGatewayResources must appear via fallback")
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
	assert.NotContains(t, names, "TokenReviews", "non-listable review API must be hidden")
	assert.NotContains(t, names, "Subjectaccessreviews", "non-listable review API must be hidden")
	assert.NotContains(t, names, "Selfsubjectreviews", "non-listable review API must be hidden")
	assert.NotContains(t, names, "Selfsubjectaccessreviews", "non-listable review API must be hidden")
	assert.NotContains(t, names, "Selfsubjectrulesreviews", "non-listable review API must be hidden")
	assert.NotContains(t, names, "Writeonlythings", "non-listable CRD must be hidden")
}

// TestBuildSidebarItems_InjectsSecuritySources verifies that the
// SecuritySourcesFn hook produces one sidebar Item per registered source,
// using the _security virtual APIGroup so GetResources can dispatch on it.
// A nil hook keeps the Security category empty (the pseudo-header still
// renders because Security is in CoreCategories).
func TestBuildSidebarItems_InjectsSecuritySources(t *testing.T) {
	prev := SecuritySourcesFn
	t.Cleanup(func() { SecuritySourcesFn = prev })
	SecuritySourcesFn = func() []SecuritySourceEntry {
		return []SecuritySourceEntry{
			{DisplayName: "Trivy", SourceName: "trivy-operator"},
			{DisplayName: "Heuristic", SourceName: "heuristic"},
		}
	}

	items := BuildSidebarItems(nil)

	byName := make(map[string]Item, len(items))
	for _, it := range items {
		byName[it.Name] = it
	}
	require.Contains(t, byName, "Trivy")
	require.Contains(t, byName, "Heuristic")
	assert.Equal(t, "__security_trivy-operator__", byName["Trivy"].Kind)
	assert.Equal(t, "Security", byName["Trivy"].Category)
	assert.Equal(t, SecurityVirtualAPIGroup+"/v1/findings-trivy-operator", byName["Trivy"].Extra,
		"Extra encodes the virtual API group so the explorer dispatches to security.Manager")
}

// TestBuildSidebarItems_SecuritySourceOrder verifies that the built-in
// sources (Advisor, Heuristic, RBAC) sort before external scanner sources,
// which keep their alphabetical fallback order.
func TestBuildSidebarItems_SecuritySourceOrder(t *testing.T) {
	prev := SecuritySourcesFn
	t.Cleanup(func() { SecuritySourcesFn = prev })
	SecuritySourcesFn = func() []SecuritySourceEntry {
		return []SecuritySourceEntry{
			{DisplayName: "Trivy", SourceName: "trivy-operator"},
			{DisplayName: "CIS", SourceName: "kube-bench"},
			{DisplayName: "RBAC", SourceName: "rbac"},
			{DisplayName: "Advisor", SourceName: "advisor"},
			{DisplayName: "Falco", SourceName: "falco"},
			{DisplayName: "Heuristic", SourceName: "heuristic"},
		}
	}

	items := BuildSidebarItems(nil)
	var got []string
	for _, it := range items {
		if it.Category == "Security" {
			got = append(got, it.Name)
		}
	}
	assert.Equal(t, []string{"Advisor", "Heuristic", "RBAC", "CIS", "Falco", "Trivy"}, got,
		"built-in sources lead; external scanners stay alphabetical")
}

// TestBuildSidebarItems_NilSecurityHook ensures the Security category is
// still navigable as an empty header when no sources are registered. This
// matters during the brief window between cluster switch and probe completion.
func TestBuildSidebarItems_NilSecurityHook(t *testing.T) {
	prev := SecuritySourcesFn
	t.Cleanup(func() { SecuritySourcesFn = prev })
	SecuritySourcesFn = nil

	items := BuildSidebarItems(nil)
	for _, it := range items {
		assert.NotEqual(t, "Security", it.Category,
			"with nil hook, no Security entries must be injected (the category header lives in CoreCategories)")
	}
}

// TestBuildSidebarItems_SecuritySourceCount surfaces the (N) suffix when
// a source has at least one finding. Zero counts render as the bare display
// name so the user is not distracted by "(0)" decorations.
func TestBuildSidebarItems_SecuritySourceCount(t *testing.T) {
	prev := SecuritySourcesFn
	t.Cleanup(func() { SecuritySourcesFn = prev })
	SecuritySourcesFn = func() []SecuritySourceEntry {
		return []SecuritySourceEntry{
			{DisplayName: "Trivy", SourceName: "trivy-operator", Count: 7},
			{DisplayName: "Heuristic", SourceName: "heuristic", Count: 0},
		}
	}

	items := BuildSidebarItems(nil)
	names := make(map[string]bool, len(items))
	for _, it := range items {
		names[it.Name] = true
	}
	assert.True(t, names["Trivy (7)"], "non-zero count surfaces as '(N)' suffix")
	assert.True(t, names["Heuristic"], "zero count keeps the bare display name")
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
