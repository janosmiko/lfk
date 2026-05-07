package app

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/app/bgtasks"
	"github.com/janosmiko/lfk/internal/model"
)

// --- StartupOptions.IsUnionMode / HasCLIOverrides ---

func TestIsUnionMode(t *testing.T) {
	tests := []struct {
		name string
		opts StartupOptions
		want bool
	}{
		{"zero value is not union", StartupOptions{}, false},
		{"empty slice is not union", StartupOptions{UnionContexts: []string{}}, false},
		{"single context is union", StartupOptions{UnionContexts: []string{"a"}}, true},
		{"multiple contexts is union", StartupOptions{UnionContexts: []string{"a", "b"}}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.opts.IsUnionMode())
		})
	}
}

func TestHasCLIOverrides_IncludesUnionContexts(t *testing.T) {
	opts := StartupOptions{UnionContexts: []string{"a", "b"}}
	assert.True(t, opts.HasCLIOverrides(),
		"union contexts alone should count as a CLI override so session restore is skipped")
}

// --- ValidateUnionOptions ---

// nKubeContexts returns ["c0", "c1", ..., "c(n-1)"] for boundary-cap tests.
func nKubeContexts(n int) []string {
	out := make([]string, n)
	for i := range n {
		out[i] = fmt.Sprintf("c%d", i)
	}
	return out
}

func TestValidateUnionOptions(t *testing.T) {
	existing := map[string]bool{"blue": true, "green": true, "yellow": true}
	for _, c := range nKubeContexts(MaxUnionContexts + 2) {
		existing[c] = true
	}
	exists := func(name string) bool { return existing[name] }

	tests := []struct {
		name    string
		opts    StartupOptions
		wantErr string // substring; empty means no error
	}{
		{
			name: "no union contexts: no-op",
			opts: StartupOptions{},
		},
		{
			name:    "union without namespace is rejected",
			opts:    StartupOptions{UnionContexts: []string{"blue"}},
			wantErr: "namespace",
		},
		{
			name: "union with multiple namespaces is rejected",
			opts: StartupOptions{
				UnionContexts: []string{"blue"},
				Namespaces:    []string{"ns-a", "ns-b"},
			},
			wantErr: "exactly one",
		},
		{
			name: "union with empty namespace is rejected",
			opts: StartupOptions{
				UnionContexts: []string{"blue"},
				Namespaces:    []string{""},
			},
			wantErr: "non-empty",
		},
		{
			name: "union with --context is mutually exclusive",
			opts: StartupOptions{
				UnionContexts: []string{"blue"},
				Context:       "blue",
				Namespaces:    []string{"ns"},
			},
			wantErr: "mutually exclusive",
		},
		{
			name: "missing kubeconfig context is rejected",
			opts: StartupOptions{
				UnionContexts: []string{"blue", "missing"},
				Namespaces:    []string{"ns"},
			},
			wantErr: `"missing" not found`,
		},
		{
			name: "duplicate contexts are rejected",
			opts: StartupOptions{
				UnionContexts: []string{"blue", "blue"},
				Namespaces:    []string{"ns"},
			},
			wantErr: "specified more than once",
		},
		{
			name: "sentinel name is reserved",
			opts: StartupOptions{
				UnionContexts: []string{UnionContextSentinel},
				Namespaces:    []string{"ns"},
			},
			wantErr: "reserved",
		},
		{
			name: "valid single context passes",
			opts: StartupOptions{
				UnionContexts: []string{"blue"},
				Namespaces:    []string{"ns"},
			},
		},
		{
			name: "valid multiple contexts pass",
			opts: StartupOptions{
				UnionContexts: []string{"blue", "green", "yellow"},
				Namespaces:    []string{"ns"},
			},
		},
		{
			name: "exactly MaxUnionContexts is allowed",
			opts: StartupOptions{
				UnionContexts: nKubeContexts(MaxUnionContexts),
				Namespaces:    []string{"ns"},
			},
		},
		{
			name: "more than MaxUnionContexts is rejected",
			opts: StartupOptions{
				UnionContexts: nKubeContexts(MaxUnionContexts + 1),
				Namespaces:    []string{"ns"},
			},
			wantErr: "at most",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateUnionOptions(tc.opts, exists)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// --- ResolveUnionSet ---

func TestResolveUnionSet(t *testing.T) {
	// One named set the lookup will return when asked. Anything else returns
	// ok=false so the "not defined in config" branch fires.
	lookup := UnionSetLookup(func(name string) (contexts []string, namespace string, colors map[string]string, ok bool) {
		if name == "ski-staging-west" {
			return []string{"green", "yellow", "blue"},
				"kube-policies",
				map[string]string{"green": "green", "yellow": "yellow", "blue": "blue"},
				true
		}
		return nil, "", nil, false
	})

	tests := []struct {
		name     string
		opts     StartupOptions
		wantSet  []string
		wantNS   []string
		wantErr  string
		wantNoOp bool // true: opts must come back unchanged
	}{
		{
			name:     "no --union-set is no-op",
			opts:     StartupOptions{},
			wantNoOp: true,
		},
		{
			name: "named set expands into UnionContexts and namespace",
			opts: StartupOptions{UnionSet: "ski-staging-west"},
			// Set's namespace fills in because no CLI namespace was provided.
			wantSet: []string{"green", "yellow", "blue"},
			wantNS:  []string{"kube-policies"},
		},
		{
			name: "CLI --namespace overrides set's namespace",
			opts: StartupOptions{
				UnionSet:   "ski-staging-west",
				Namespaces: []string{"override-ns"},
			},
			wantSet: []string{"green", "yellow", "blue"},
			wantNS:  []string{"override-ns"},
		},
		{
			name:    "unknown set name is rejected",
			opts:    StartupOptions{UnionSet: "does-not-exist"},
			wantErr: "not defined in config",
		},
		{
			name: "mutex with --context",
			opts: StartupOptions{
				UnionSet: "ski-staging-west",
				Context:  "blue",
			},
			wantErr: "mutually exclusive",
		},
		{
			name: "mutex with --union-context",
			opts: StartupOptions{
				UnionSet:      "ski-staging-west",
				UnionContexts: []string{"red"},
			},
			wantErr: "mutually exclusive",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveUnionSet(tc.opts, lookup)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			if tc.wantNoOp {
				assert.Equal(t, tc.opts, got, "opts must be unchanged when --union-set is empty")
				return
			}
			assert.Equal(t, tc.wantSet, got.UnionContexts)
			assert.Equal(t, tc.wantNS, got.Namespaces)
		})
	}

	// Nil-lookup branch: avoids a panic and surfaces a useful message instead
	// of silently treating the set as missing.
	t.Run("nil lookup", func(t *testing.T) {
		_, err := ResolveUnionSet(StartupOptions{UnionSet: "ski-staging-west"}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no union_sets configured")
	})
}

func TestResolveUnionSet_PreservesCLIArgsWhenSetHasNoNamespace(t *testing.T) {
	// A set with no namespace must NOT clobber a CLI-provided namespace,
	// and must NOT inject an empty namespace that would later look like
	// "user provided --namespace ''".
	lookup := UnionSetLookup(func(name string) (contexts []string, namespace string, colors map[string]string, ok bool) {
		if name == "ns-less" {
			return []string{"a", "b"}, "", nil, true
		}
		return nil, "", nil, false
	})

	got, err := ResolveUnionSet(
		StartupOptions{UnionSet: "ns-less", Namespaces: []string{"cli-ns"}},
		lookup,
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, got.UnionContexts)
	assert.Equal(t, []string{"cli-ns"}, got.Namespaces)

	// And: ns-less set with no CLI namespace leaves Namespaces empty.
	// ValidateUnionOptions then catches this with the "namespace required"
	// error — that contract is exercised in TestValidateUnionOptions.
	got, err = ResolveUnionSet(StartupOptions{UnionSet: "ns-less"}, lookup)
	require.NoError(t, err)
	assert.Empty(t, got.Namespaces)
}

// --- NewModel with union options ---

func TestNewModel_UnionInitialises(t *testing.T) {
	client := newTestClientForOptions(t)

	opts := StartupOptions{
		UnionContexts: []string{"blue", "green"},
		Namespaces:    []string{"cloud-cd"},
	}
	m := NewModel(client, opts)

	assert.True(t, m.unionMode)
	assert.Equal(t, []string{"blue", "green"}, m.unionContexts)
	assert.Equal(t, UnionContextSentinel, m.nav.Context,
		"nav.Context should be the sentinel so loadResources knows to fan out")
	assert.False(t, m.allNamespaces, "union requires a specific namespace")
	assert.Equal(t, "cloud-cd", m.namespace)

	require.NotNil(t, m.pendingSession, "session-restore scaffolding is built off the first union context")
	assert.Equal(t, "blue", m.pendingSession.Context)
}

// --- isUnionSentinel / effectiveContext ---

func TestIsUnionSentinel(t *testing.T) {
	m := Model{unionMode: true, nav: model.NavigationState{Context: UnionContextSentinel}}
	assert.True(t, m.isUnionSentinel())

	// Post-drill-down: flag still true, but nav.Context is a real cluster.
	m.nav.Context = "blue"
	assert.False(t, m.isUnionSentinel(), "drill-down replaces the sentinel with the real cluster")

	// Non-union mode always returns false, even if something sets the sentinel string.
	m = Model{nav: model.NavigationState{Context: UnionContextSentinel}}
	assert.False(t, m.isUnionSentinel(), "unionMode=false short-circuits even when the sentinel is present")
}

func TestEffectiveContext_ResolvesSentinelToHoveredItemsCluster(t *testing.T) {
	// Two rows with identical Name/Kind/Extra/Namespace differing only by ClusterName.
	items := []model.Item{
		{Name: "my-pod", Kind: "Pod", Namespace: "cloud-cd", ClusterName: "blue"},
		{Name: "my-pod", Kind: "Pod", Namespace: "cloud-cd", ClusterName: "green"},
	}
	m := Model{
		unionMode:     true,
		unionContexts: []string{"blue", "green"},
		nav:           model.NavigationState{Level: model.LevelResources, Context: UnionContextSentinel},
		middleItems:   items,
		cursors:       [5]int{},
	}

	m.setCursor(0)
	assert.Equal(t, "blue", m.effectiveContext(),
		"cursor on row 0 should route to blue")

	m.setCursor(1)
	assert.Equal(t, "green", m.effectiveContext(),
		"cursor on row 1 should route to green — this is the disambiguation fix")
}

func TestEffectiveContext_NonUnionReturnsNavContext(t *testing.T) {
	m := Model{nav: model.NavigationState{Context: "production"}}
	assert.Equal(t, "production", m.effectiveContext())
}

func TestEffectiveContext_SentinelWithoutHoveredClusterFallsBackToFirstUnionContext(t *testing.T) {
	// At LevelResourceTypes (sidebar items have no ClusterName), at the
	// Overview/Monitoring pseudo-rows, or any other sentinel-level path
	// where the hovered item isn't a union row — effectiveContext must
	// return a real cluster name, not the sentinel. Without this fallback
	// the raw "__union__" string leaks into restConfigForContext and
	// surfaces as "context \"__union__\" does not exist" in the log.
	items := []model.Item{{Name: "Pods", Kind: "Pod"}} // no ClusterName
	m := Model{
		unionMode:     true,
		unionContexts: []string{"blue", "green"},
		nav:           model.NavigationState{Level: model.LevelResourceTypes, Context: UnionContextSentinel},
		middleItems:   items,
		cursors:       [5]int{},
	}
	assert.Equal(t, "blue", m.effectiveContext(),
		"sentinel + no per-row cluster must fall back to unionContexts[0]")
}

func TestEffectiveContext_PostDrillDownReturnsNavContext(t *testing.T) {
	m := Model{
		unionMode:     true,
		unionContexts: []string{"blue", "green"},
		nav:           model.NavigationState{Level: model.LevelOwned, Context: "blue"},
	}
	assert.Equal(t, "blue", m.effectiveContext(),
		"post-drill-down nav.Context is already the real cluster; effectiveContext must not touch it")
}

// --- activeContext sentinel resolution ---

func TestActiveContext_UnionSentinelResolvesToFirstCluster(t *testing.T) {
	// activeContext feeds GetNamespaces, the namespace cache key, and the
	// command-bar completion store. All five call-sites need a real
	// kubeconfig context — passing the literal "__union__" sentinel
	// fails GetNamespaces and corrupts the cache. The resolution lives
	// inside activeContext so every caller benefits in one place.
	m := Model{
		unionMode:     true,
		unionContexts: []string{"blue", "green", "yellow"},
		nav:           model.NavigationState{Level: model.LevelResources, Context: UnionContextSentinel},
	}
	assert.Equal(t, "blue", m.activeContext(),
		"sentinel must resolve to unionContexts[0] so namespace listing targets a real cluster")
}

func TestActiveContext_UnionSentinelWithoutContextsReturnsEmpty(t *testing.T) {
	m := Model{
		unionMode: true,
		nav:       model.NavigationState{Level: model.LevelResources, Context: UnionContextSentinel},
	}
	assert.Empty(t, m.activeContext(), "sentinel must not leak as a Kubernetes context when no union contexts are configured")
}

func TestActiveContext_PostDrillDownReturnsNavContext(t *testing.T) {
	// Once the user drills into a specific resource, nav.Context is the
	// real cluster name and activeContext must return it as-is (the
	// sentinel resolution short-circuits before the nav.Context branch).
	m := Model{
		unionMode:     true,
		unionContexts: []string{"blue", "green"},
		nav:           model.NavigationState{Level: model.LevelOwned, Context: "green"},
	}
	assert.Equal(t, "green", m.activeContext())
}

func TestActiveContext_NonUnionUnaffected(t *testing.T) {
	// The sentinel branch only fires when unionMode is true. A regular
	// session must keep its existing nav.Context-then-current-context
	// fallback chain untouched.
	m := Model{nav: model.NavigationState{Context: "production"}}
	assert.Equal(t, "production", m.activeContext())
}

// --- selectedMiddleItem disambiguation in union mode ---

func TestSelectedMiddleItem_UnionDisambiguatesByClusterName(t *testing.T) {
	items := []model.Item{
		{Name: "my-pod", Kind: "Pod", Namespace: "cloud-cd", ClusterName: "blue"},
		{Name: "my-pod", Kind: "Pod", Namespace: "cloud-cd", ClusterName: "green"},
	}
	m := Model{
		nav:         model.NavigationState{Level: model.LevelResources},
		middleItems: items,
		cursors:     [5]int{},
	}

	m.setCursor(1)
	sel := m.selectedMiddleItem()
	require.NotNil(t, sel)
	assert.Equal(t, "green", sel.ClusterName,
		"cursor on row 1 must return the green row — pre-fix this returned blue because ClusterName was not part of the match")
}

// --- Read-only guards in union mode ---

func TestBookmarkToSlot_BlockedAtUnionSentinel(t *testing.T) {
	m := Model{
		unionMode: true,
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: UnionContextSentinel,
		},
		cursors: [5]int{},
	}
	mdl, _ := m.bookmarkToSlot("a")
	result, ok := mdl.(Model)
	require.True(t, ok)
	assert.Empty(t, result.bookmarks, "no bookmark should be persisted at the union sentinel")
	assert.Contains(t, result.statusMessage, "union view",
		"user should see a clear message about why the bookmark was refused")
}

func TestToggleSelect_AllowedAtUnionSentinel(t *testing.T) {
	// Multi-select must work at the sentinel so the user can run bulk
	// delete/restart against pods/deploys spread across clusters. The
	// dispatcher (executeBulkAction) still gates the action label against
	// the union allowlist, so opening the door for selection itself is safe.
	items := []model.Item{
		{Name: "my-pod", Kind: "Pod", Namespace: "cloud-cd", ClusterName: "blue"},
		{Name: "my-pod", Kind: "Pod", Namespace: "cloud-cd", ClusterName: "green"},
	}
	m := Model{
		unionMode: true,
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: UnionContextSentinel,
		},
		middleItems:     items,
		cursors:         [5]int{},
		selectedItems:   make(map[string]bool),
		selectionAnchor: -1,
	}
	m.setCursor(1)
	result := m.handleKeyToggleSelect()
	assert.Len(t, result.selectedItems, 1, "the green row should be selected")
	// selectionKey must include ClusterName so the green selection does not
	// collide with the blue one when they share name+namespace.
	assert.True(t, result.selectedItems["green:cloud-cd/my-pod"],
		"selection key must encode ClusterName; got: %v", result.selectedItems)
}

// --- selectionKey ---

func TestSelectionKey_NonUnionFormat(t *testing.T) {
	// Non-union rows have empty ClusterName; the legacy "namespace/name"
	// format must be preserved so existing readonly_test/bulk tests and
	// session files keep working.
	got := selectionKey(model.Item{Name: "my-pod", Namespace: "default"})
	assert.Equal(t, "default/my-pod", got)

	// Cluster-scoped (no namespace) keeps the bare name.
	got = selectionKey(model.Item{Name: "my-node"})
	assert.Equal(t, "my-node", got)
}

func TestSelectionKey_UnionPrependsCluster(t *testing.T) {
	// Same name+namespace in two clusters must hash to two distinct keys
	// — otherwise selecting "my-pod" in blue silently marks "my-pod" in
	// green as selected too.
	blue := selectionKey(model.Item{Name: "my-pod", Namespace: "cloud-cd", ClusterName: "blue"})
	green := selectionKey(model.Item{Name: "my-pod", Namespace: "cloud-cd", ClusterName: "green"})
	assert.NotEqual(t, blue, green)
	assert.Equal(t, "blue:cloud-cd/my-pod", blue)
	assert.Equal(t, "green:cloud-cd/my-pod", green)
}

// --- Union action allowlist ---

func TestIsUnionAllowedAction(t *testing.T) {
	// Allowlist: Delete + the two force escalations + Restart. Everything
	// else mutating must be blocked. Read-only labels (Logs, Describe,
	// Refresh, Events) pass through unconditionally.
	allowed := []string{"Delete", "Force Delete", "Force Finalize", "Restart"}
	for _, label := range allowed {
		assert.True(t, isUnionAllowedAction(label), "%q must be allowed at the union sentinel", label)
	}
	blocked := []string{"Edit", "Scale", "Drain", "Cordon", "Exec", "Shell", "Port Forward", "Secret Editor", "Sync", "Upgrade"}
	for _, label := range blocked {
		assert.False(t, isUnionAllowedAction(label), "%q must NOT be allowed at the union sentinel", label)
	}
	// Non-mutating labels are always allowed because they aren't in
	// mutatingActions to begin with — verify the helper returns true for
	// a few representative read-only labels.
	for _, label := range []string{"Logs", "Describe", "Events", "Refresh"} {
		assert.True(t, isUnionAllowedAction(label), "non-mutating %q must pass through", label)
	}
}

// --- expandGroupedItems carries ClusterName ---

func TestExpandGroupedItems_PropagatesClusterName(t *testing.T) {
	// Bulk dispatcher relies on GroupedRef.ClusterName to route per-item.
	// A flat (non-grouped) item must produce a ref whose ClusterName equals
	// the parent. A grouped item must propagate the parent's cluster onto
	// each child ref that doesn't already carry one, while preserving refs
	// that already carry their own cluster.
	in := []model.Item{
		{Name: "p1", Namespace: "ns", ClusterName: "blue"},
		{
			Name:        "g1",
			Namespace:   "ns",
			ClusterName: "green",
			GroupedRefs: []model.GroupedRef{
				{Name: "child-a", Namespace: "ns"},
				{Name: "child-b", Namespace: "ns", ClusterName: "yellow"}, // already set; preserved
			},
		},
	}
	got := expandGroupedItems(in)
	require.Len(t, got, 3)
	assert.Equal(t, "blue", got[0].ClusterName)
	assert.Equal(t, "green", got[1].ClusterName, "child-a must inherit parent's cluster")
	assert.Equal(t, "yellow", got[2].ClusterName, "child-b's existing cluster must be preserved")
}

// --- Cluster color tile stamping on union rows ---

func TestUpdateResourcesLoadedMain_StampsClusterColorOnUnionRows(t *testing.T) {
	// Union rows must have ClusterColor populated from the per-set
	// unionContextColors map so the table renderer can paint a 1-cell
	// tile per row. The per-set map is sourced from union_sets.contexts
	// `color:` fields at startup, separate from the global cluster_colors
	// state file (which feeds the cluster picker only).
	m := Model{
		client:                     nil,
		unionMode:                  true,
		unionContexts:              []string{"blue", "green", "yellow"},
		unionContextColors:         map[string]string{"blue": "blue", "green": "green"}, // yellow intentionally unset
		discoveredResources:        make(map[string][]model.ResourceTypeEntry),
		discoveryRefreshedContexts: make(map[string]bool),
		discoveringContexts:        make(map[string]bool),
		itemCache:                  make(map[string][]model.Item),
		cacheFingerprints:          make(map[string]string),
		nav: model.NavigationState{
			Level:        model.LevelResources,
			Context:      UnionContextSentinel,
			ResourceType: model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true},
		},
		cursors:       [5]int{},
		selectedItems: make(map[string]bool),
	}
	msg := resourcesLoadedMsg{
		items: []model.Item{
			{Name: "p1", Kind: "Pod", Namespace: "ns", ClusterName: "blue"},
			{Name: "p2", Kind: "Pod", Namespace: "ns", ClusterName: "green"},
			{Name: "p3", Kind: "Pod", Namespace: "ns", ClusterName: "yellow"},
		},
		gen: m.requestGen,
		rt:  m.nav.ResourceType,
	}
	updated, _ := m.updateResourcesLoadedMain(msg)
	result, ok := updated.(Model)
	require.True(t, ok)
	require.Len(t, result.middleItems, 3)

	byName := make(map[string]model.Item)
	for _, item := range result.middleItems {
		byName[item.Name] = item
	}
	assert.Equal(t, "blue", byName["p1"].ClusterColor, "p1 in blue cluster should pick up the blue color")
	assert.Equal(t, "green", byName["p2"].ClusterColor, "p2 in green cluster should pick up the green color")
	assert.Empty(t, byName["p3"].ClusterColor, "yellow cluster has no configured color; ClusterColor must remain empty so the tile cell stays blank for that row")
}

// --- Action menu filtering at the union sentinel ---

func TestOpenActionMenu_UnionSentinel_PodFiltered(t *testing.T) {
	// At the union sentinel a Pod's action menu must drop every mutating
	// label except Delete and Force Delete. Read-only labels (Logs,
	// Describe, Events, Crash Investigator, etc.) pass through.
	m := Model{
		unionMode:     true,
		unionContexts: []string{"blue", "green"},
		nav: model.NavigationState{
			Level:        model.LevelResources,
			Context:      UnionContextSentinel,
			ResourceType: model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true},
		},
		middleItems: []model.Item{
			{Name: "my-pod", Kind: "Pod", Namespace: "cloud-cd", ClusterName: "blue"},
		},
		tabs:    []TabState{{}},
		cursors: [5]int{},
		width:   80, height: 40,
	}
	result := m.openActionMenu()
	assert.Equal(t, overlayAction, result.overlay)

	labels := make(map[string]bool)
	for _, item := range result.overlayItems {
		labels[item.Name] = true
	}

	// In-scope mutations are present.
	assert.True(t, labels["Delete"], "Delete must be in the union Pod menu")
	assert.True(t, labels["Force Delete"], "Force Delete must be in the union Pod menu")
	// Out-of-scope mutations are filtered out.
	for _, label := range []string{"Edit", "Exec", "Attach", "Debug", "Port Forward", "Shell"} {
		assert.False(t, labels[label], "%q must be filtered out at the union sentinel", label)
	}
}

func TestOpenActionMenu_UnionSentinel_DeploymentAllowsRestart(t *testing.T) {
	// A Deployment's union menu must keep Restart and drop Scale/Edit/
	// Rollback so the sentinel-level write surface stays exactly two ops.
	m := Model{
		unionMode:     true,
		unionContexts: []string{"blue", "green"},
		nav: model.NavigationState{
			Level:        model.LevelResources,
			Context:      UnionContextSentinel,
			ResourceType: model.ResourceTypeEntry{Kind: "Deployment", Resource: "deployments", Namespaced: true, APIGroup: "apps", APIVersion: "v1"},
		},
		middleItems: []model.Item{
			{Name: "my-deploy", Kind: "Deployment", Namespace: "cloud-cd", ClusterName: "blue"},
		},
		tabs:    []TabState{{}},
		cursors: [5]int{},
		width:   80, height: 40,
	}
	result := m.openActionMenu()
	assert.Equal(t, overlayAction, result.overlay)

	labels := make(map[string]bool)
	for _, item := range result.overlayItems {
		labels[item.Name] = true
	}
	assert.True(t, labels["Restart"], "Restart must be in the union Deployment menu")
	assert.True(t, labels["Delete"], "Delete must be in the union Deployment menu")
	for _, label := range []string{"Scale", "Edit", "Rollback"} {
		assert.False(t, labels[label], "%q must be filtered out at the union sentinel", label)
	}
}

func TestExecuteBulkAction_UnionSentinel_BlocksScale(t *testing.T) {
	// Defense-in-depth: even if a Scale label slips past the menu filter,
	// executeBulkAction refuses it at the sentinel and emits a status.
	m := Model{
		unionMode: true,
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: UnionContextSentinel,
		},
		bulkMode:  true,
		bulkItems: []model.Item{{Name: "d1", Kind: "Deployment", ClusterName: "blue"}},
		actionCtx: actionContext{
			kind:    "Deployment",
			context: "blue",
		},
		cursors: [5]int{},
	}
	mdl, _ := m.executeBulkAction("Scale")
	result, ok := mdl.(Model)
	require.True(t, ok)
	assert.Contains(t, result.statusMessage, "not available in union view",
		"Scale must be refused with the union-view message")
}

func TestExecuteAction_UnionUsesTargetContextReadOnly(t *testing.T) {
	m := Model{
		unionMode: true,
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: UnionContextSentinel,
		},
		actionCtx: actionContext{
			kind:    "Pod",
			name:    "my-pod",
			context: "green",
		},
		contextROOverrides: map[string]bool{"green": true},
	}

	mdl, _ := m.executeAction("Delete")
	result, ok := mdl.(Model)
	require.True(t, ok)
	assert.Equal(t, readOnlyBlockedMessage("Delete"), result.statusMessage)
	assert.True(t, result.statusMessageErr)
	assert.NotEqual(t, overlayConfirm, result.overlay, "delete confirmation must not open for a read-only target context")
}

func TestExecuteBulkAction_UnionBlocksReadOnlyTargetContext(t *testing.T) {
	m := Model{
		unionMode: true,
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: UnionContextSentinel,
		},
		bulkMode: true,
		bulkItems: []model.Item{
			{Name: "p1", Kind: "Pod", Namespace: "ns", ClusterName: "blue"},
			{Name: "p2", Kind: "Pod", Namespace: "ns", ClusterName: "green"},
		},
		actionCtx: actionContext{
			kind:    "Pod",
			context: "blue",
		},
		contextROOverrides: map[string]bool{"green": true},
	}

	mdl, _ := m.executeBulkAction("Delete")
	result, ok := mdl.(Model)
	require.True(t, ok)
	assert.Equal(t, readOnlyBlockedMessage("Delete"), result.statusMessage)
	assert.True(t, result.statusMessageErr)
	assert.NotEqual(t, overlayConfirm, result.overlay, "bulk delete confirmation must not open when any target context is read-only")
}

func TestNewTab_BlockedInUnionMode(t *testing.T) {
	m := Model{
		unionMode: true,
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: UnionContextSentinel,
		},
		tabs:    []TabState{{}},
		cursors: [5]int{},
	}
	mdl, _, handled := m.handleExplorerActionKeyNewTab()
	assert.True(t, handled)
	result, ok := mdl.(Model)
	require.True(t, ok)
	assert.Len(t, result.tabs, 1, "union mode must not open a new tab")
	assert.Contains(t, result.statusMessage, "union view")
}

func TestUpdateResourcesLoaded_UnionPartialErrorKeepsItems(t *testing.T) {
	m := Model{
		unionMode:                  true,
		unionContexts:              []string{"blue", "green"},
		unionContextColors:         map[string]string{"blue": "blue"},
		discoveredResources:        make(map[string][]model.ResourceTypeEntry),
		discoveryRefreshedContexts: make(map[string]bool),
		discoveringContexts:        make(map[string]bool),
		itemCache:                  make(map[string][]model.Item),
		cacheFingerprints:          make(map[string]string),
		bgtasks:                    bgtasks.New(0),
		nav: model.NavigationState{
			Level:   model.LevelResources,
			Context: UnionContextSentinel,
			ResourceType: model.ResourceTypeEntry{
				Kind:       "__port_forwards__",
				Resource:   "",
				Namespaced: true,
			},
		},
		cursors: [5]int{},
	}
	msg := resourcesLoadedMsg{
		items: []model.Item{
			{Name: "p1", Kind: "Pod", Namespace: "ns", ClusterName: "blue"},
		},
		err: errors.New("green unavailable"),
		gen: m.requestGen,
		rt:  m.nav.ResourceType,
	}

	mdl, _ := m.updateResourcesLoaded(msg)
	result, ok := mdl.(Model)
	require.True(t, ok)
	require.Len(t, result.middleItems, 1)
	assert.Equal(t, "p1", result.middleItems[0].Name)
	assert.Equal(t, "blue", result.middleItems[0].ClusterColor)
	assert.Contains(t, result.statusMessage, "green unavailable")
	assert.True(t, result.statusMessageErr)
	assert.Error(t, result.err)
}
