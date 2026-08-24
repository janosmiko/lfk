package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// --- resolveStartupAllNamespaces precedence ---

func TestResolveStartupAllNamespaces_ContextPinsNamespace_ScopesToIt(t *testing.T) {
	orig, origSet := ui.ConfigAllNamespaces, ui.ConfigAllNamespacesSet
	defer func() { ui.ConfigAllNamespaces, ui.ConfigAllNamespacesSet = orig, origSet }()
	ui.ConfigAllNamespaces = true
	ui.ConfigAllNamespacesSet = false

	client := k8s.NewTestClient(nil, nil)
	client.AddTestContextWithNamespace("prod", "https://prod.example.local:6443", "billing")

	got := resolveStartupAllNamespaces(client, "prod")
	assert.False(t, got, "a pinned context namespace must scope the startup view when all_namespaces is unset")
}

func TestResolveStartupAllNamespaces_ContextPinsNoNamespace_AllNamespaces(t *testing.T) {
	orig, origSet := ui.ConfigAllNamespaces, ui.ConfigAllNamespacesSet
	defer func() { ui.ConfigAllNamespaces, ui.ConfigAllNamespacesSet = orig, origSet }()
	ui.ConfigAllNamespaces = true
	ui.ConfigAllNamespacesSet = false

	client := k8s.NewTestClient(nil, nil)
	client.AddTestContextWithNamespace("dev", "https://dev.example.local:6443", "")

	got := resolveStartupAllNamespaces(client, "dev")
	assert.True(t, got, "a context with no pinned namespace must fall back to all-namespaces")
}

func TestResolveStartupAllNamespaces_ConfigTrueWinsOverPinnedNamespace(t *testing.T) {
	orig, origSet := ui.ConfigAllNamespaces, ui.ConfigAllNamespacesSet
	defer func() { ui.ConfigAllNamespaces, ui.ConfigAllNamespacesSet = orig, origSet }()
	ui.ConfigAllNamespaces = true
	ui.ConfigAllNamespacesSet = true

	client := k8s.NewTestClient(nil, nil)
	client.AddTestContextWithNamespace("prod", "https://prod.example.local:6443", "billing")

	got := resolveStartupAllNamespaces(client, "prod")
	assert.True(t, got, "an explicit all_namespaces: true must win over a pinned context namespace")
}

func TestResolveStartupAllNamespaces_ConfigFalseScopesToContextNamespace(t *testing.T) {
	orig, origSet := ui.ConfigAllNamespaces, ui.ConfigAllNamespacesSet
	defer func() { ui.ConfigAllNamespaces, ui.ConfigAllNamespacesSet = orig, origSet }()
	ui.ConfigAllNamespaces = false
	ui.ConfigAllNamespacesSet = true

	client := k8s.NewTestClient(nil, nil)
	client.AddTestContextWithNamespace("dev", "https://dev.example.local:6443", "")

	got := resolveStartupAllNamespaces(client, "dev")
	assert.False(t, got, "an explicit all_namespaces: false must scope even without a pinned namespace")
}

// --- NewModel / CLI flag precedence ---

func TestNewModel_CLINamespaceOverridesConfigAndKubeconfig(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	orig, origSet := ui.ConfigAllNamespaces, ui.ConfigAllNamespacesSet
	defer func() { ui.ConfigAllNamespaces, ui.ConfigAllNamespacesSet = orig, origSet }()
	ui.ConfigAllNamespaces = true
	ui.ConfigAllNamespacesSet = false

	client := k8s.NewTestClient(nil, nil)
	client.AddTestContextWithNamespace("prod", "https://prod.example.local:6443", "billing")

	m := NewModel(client, StartupOptions{Context: "prod", Namespaces: []string{"checkout"}})

	require.NotNil(t, m.pendingSession)
	require.Len(t, m.pendingSession.Tabs, 1)
	tab := m.pendingSession.Tabs[0]
	assert.False(t, tab.AllNamespaces)
	assert.Equal(t, "checkout", tab.Namespace, "--namespace must win over both config and the kubeconfig-pinned namespace")
}

// TestNewModel_CLIContextOnlyHonoursConfigAndKubeconfigNamespace is the
// regression test for the --context (no --namespace) startup path: it must
// not force all-namespaces and drop the kubeconfig-pinned scope.
func TestNewModel_CLIContextOnlyHonoursConfigAndKubeconfigNamespace(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	orig, origSet := ui.ConfigAllNamespaces, ui.ConfigAllNamespacesSet
	defer func() { ui.ConfigAllNamespaces, ui.ConfigAllNamespacesSet = orig, origSet }()
	ui.ConfigAllNamespaces = true
	ui.ConfigAllNamespacesSet = false

	client := k8s.NewTestClient(nil, nil)
	client.AddTestContextWithNamespace("prod", "https://prod.example.local:6443", "billing")

	m := NewModel(client, StartupOptions{Context: "prod"})

	require.NotNil(t, m.pendingSession)
	require.Len(t, m.pendingSession.Tabs, 1)
	tab := m.pendingSession.Tabs[0]
	assert.False(t, tab.AllNamespaces, "--context alone must not force all-namespaces over a pinned kubeconfig namespace")
	assert.Equal(t, "billing", tab.Namespace)
}

// --- rescopeNamespaceForContext ---

func TestRescopeNamespaceForContext_ScopesToPinnedNamespaceAndClearsSelection(t *testing.T) {
	orig, origSet := ui.ConfigAllNamespaces, ui.ConfigAllNamespacesSet
	defer func() { ui.ConfigAllNamespaces, ui.ConfigAllNamespacesSet = orig, origSet }()
	ui.ConfigAllNamespaces = true
	ui.ConfigAllNamespacesSet = false

	m := baseModelWithFakeClient()
	m.client.AddTestContextWithNamespace("staging", "https://staging.example.local:6443", "team-a")
	m.allNamespaces = true
	m.selectedNamespaces = map[string]bool{"old-ns": true}
	m.nsSelectionNegated = true

	m.rescopeNamespaceForContext("staging")

	assert.False(t, m.allNamespaces)
	assert.Equal(t, "team-a", m.namespace)
	assert.Nil(t, m.selectedNamespaces, "switching clusters must drop the prior cluster's multi-namespace selection")
	assert.False(t, m.nsSelectionNegated)
}

func TestRescopeNamespaceForContext_NoOpInUnionMode(t *testing.T) {
	m := baseModelWithFakeClient()
	m.client.AddTestContextWithNamespace("staging", "https://staging.example.local:6443", "team-a")
	m.unionMode = true
	m.allNamespaces = true
	m.namespace = "kept"
	m.selectedNamespaces = map[string]bool{"kept-ns": true}

	m.rescopeNamespaceForContext("staging")

	assert.True(t, m.allNamespaces)
	assert.Equal(t, "kept", m.namespace)
	assert.Equal(t, map[string]bool{"kept-ns": true}, m.selectedNamespaces)
}

// The all-namespaces stash (savedSelectedNamespaces) survives an A-toggle so a
// second press can restore the prior selection. It must not survive a context
// switch: restoring it in the new cluster scopes to a namespace that belongs to
// the old one. namespace_history.go drops the same pair on a previous-namespace
// jump for the same reason.
func TestRescopeNamespaceForContext_DropsAllNamespacesStash(t *testing.T) {
	orig, origSet := ui.ConfigAllNamespaces, ui.ConfigAllNamespacesSet
	defer func() { ui.ConfigAllNamespaces, ui.ConfigAllNamespacesSet = orig, origSet }()
	ui.ConfigAllNamespaces = true
	ui.ConfigAllNamespacesSet = false

	for _, tc := range []struct {
		name          string
		pinnedNs      string
		wantAllNs     bool
		wantNamespace string
	}{
		{name: "context pins a namespace", pinnedNs: "team-a", wantAllNs: false, wantNamespace: "team-a"},
		{name: "context pins nothing", pinnedNs: "", wantAllNs: true, wantNamespace: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := baseModelWithFakeClient()
			m.client.AddTestContextWithNamespace("staging", "https://staging.example.local:6443", tc.pinnedNs)
			m.namespace = "stale"
			m.allNamespaces = true
			// State the A-toggle leaves behind in the cluster we are leaving.
			m.savedSelectedNamespaces = map[string]bool{"team-a-ns": true}
			m.savedNsSelectionNegated = true
			m.nsSelectionModified = true
			m.previousNsScope = &nsScope{namespace: "old-ns"}

			m.rescopeNamespaceForContext("staging")

			assert.Equal(t, tc.wantAllNs, m.allNamespaces)
			assert.Equal(t, tc.wantNamespace, m.namespace)
			assert.Nil(t, m.savedSelectedNamespaces, "a later A-toggle would restore the old cluster's selection")
			assert.False(t, m.savedNsSelectionNegated)
			assert.False(t, m.nsSelectionModified)
			assert.Nil(t, m.previousNsScope, "a previous-namespace jump would land in the old cluster's namespace")
		})
	}
}

// A context switch into all-namespaces mode must clear m.namespace too. The
// A-toggle only falls back to the new context's default when m.namespace is
// empty (update_keys_actions.go), so a leftover value silently scopes the new
// cluster to a namespace from the old one.
func TestRescopeNamespaceForContext_AllNamespacesClearsStaleNamespace(t *testing.T) {
	orig, origSet := ui.ConfigAllNamespaces, ui.ConfigAllNamespacesSet
	defer func() { ui.ConfigAllNamespaces, ui.ConfigAllNamespacesSet = orig, origSet }()
	ui.ConfigAllNamespaces = true
	ui.ConfigAllNamespacesSet = false

	m := baseModelWithFakeClient()
	m.client.AddTestContextWithNamespace("staging", "https://staging.example.local:6443", "")
	m.nav.Level = model.LevelResourceTypes
	m.namespace = "old-cluster-ns"
	m.allNamespaces = false

	m.nav.Context = "staging"
	m.rescopeNamespaceForContext("staging")
	require.True(t, m.allNamespaces)
	require.Empty(t, m.namespace, "the old cluster's namespace must not survive the switch")

	// Toggle all-namespaces off in the new cluster.
	toggled, _, _ := m.handleExplorerActionKeyAllNamespaces()
	after, ok := toggled.(Model)
	require.True(t, ok)

	assert.False(t, after.allNamespaces)
	assert.Equal(t, "default", after.namespace, "the toggle must resolve the new context's default namespace")
}
