package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func TestEffectiveStartupResource(t *testing.T) {
	orig := ui.ConfigStartupResource
	defer func() { ui.ConfigStartupResource = orig }()

	ui.ConfigStartupResource = "pods"
	assert.Equal(t, "deploy", effectiveStartupResource(StartupOptions{Resource: "deploy"}),
		"the --resource flag must win over startup_resource in config")
	assert.Equal(t, "pods", effectiveStartupResource(StartupOptions{}),
		"config value applies when no flag is given")

	ui.ConfigStartupResource = ""
	assert.Empty(t, effectiveStartupResource(StartupOptions{}))
}

func TestResolveResourceTypeByName(t *testing.T) {
	origAbbr := ui.SearchAbbreviations
	defer func() { ui.SearchAbbreviations = origAbbr }()
	ui.SearchAbbreviations = map[string]string{"deploy": "deployment"}

	discovered := []model.ResourceTypeEntry{
		{Kind: "Deployment", APIGroup: "apps", APIVersion: "v1", Resource: "deployments", Namespaced: true},
		{Kind: "Widget", APIGroup: "example.io", APIVersion: "v1", Resource: "widgets", Namespaced: true},
		{Kind: "Widget", APIGroup: "other.io", APIVersion: "v1", Resource: "widgets", Namespaced: true},
	}

	cases := []struct {
		name    string
		query   string
		wantOK  bool
		wantRef string
	}{
		{"abbreviation", "deploy", true, "apps/v1/deployments"},
		{"kind", "Deployment", true, "apps/v1/deployments"},
		{"plural", "deployments", true, "apps/v1/deployments"},
		{"singular", "deployment", true, "apps/v1/deployments"},
		{"crd name.group", "widgets.example.io", true, "example.io/v1/widgets"},
		{"unknown", "does-not-exist", false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rt, ok := resolveResourceTypeByName(tc.query, discovered)
			assert.Equal(t, tc.wantOK, ok)
			if tc.wantOK {
				assert.Equal(t, tc.wantRef, rt.ResourceRef())
			}
		})
	}
}

func TestApplyStartupResource(t *testing.T) {
	t.Run("nil session builds a synthetic one-tab session", func(t *testing.T) {
		got := applyStartupResource(nil, "deploy", "my-ctx", "default", false)
		require.Len(t, got.Tabs, 1)
		assert.Equal(t, "my-ctx", got.Tabs[0].Context)
		assert.Equal(t, "deploy", got.Tabs[0].ResourceType)
		assert.Equal(t, "default", got.Tabs[0].Namespace)
	})

	t.Run("multi-tab session only replaces the active tab", func(t *testing.T) {
		sess := &SessionState{
			ActiveTab: 1,
			Tabs: []SessionTab{
				{Context: "ctx-a", Namespace: "team-a", ResourceType: "/v1/pods", Filter: "keep-me"},
				{Context: "ctx-b", Namespace: "team-b", ResourceType: "/v1/services", Filter: "stale", CursorName: "svc-1"},
			},
		}
		got := applyStartupResource(sess, "deploy", "ctx-b", "team-b", false)

		assert.Equal(t, "/v1/pods", got.Tabs[0].ResourceType, "other tabs are untouched")
		assert.Equal(t, "keep-me", got.Tabs[0].Filter)
		assert.Equal(t, "ctx-a", got.Tabs[0].Context)

		assert.Equal(t, "deploy", got.Tabs[1].ResourceType)
		assert.Equal(t, "ctx-b", got.Tabs[1].Context, "the active tab's context is kept")
		assert.Equal(t, "team-b", got.Tabs[1].Namespace, "the active tab's namespace is kept")
		assert.Empty(t, got.Tabs[1].Filter, "list-view state resets on the active tab")
		assert.Empty(t, got.Tabs[1].CursorName)
	})

	t.Run("legacy single-tab session replaces the top-level fields", func(t *testing.T) {
		sess := &SessionState{
			Context: "my-ctx", Namespace: "my-ns",
			ResourceType: "/v1/pods", Filter: "stale", CursorName: "pod-1",
		}
		got := applyStartupResource(sess, "deploy", "my-ctx", "my-ns", false)

		assert.Equal(t, "deploy", got.ResourceType)
		assert.Equal(t, "my-ctx", got.Context, "context is kept")
		assert.Equal(t, "my-ns", got.Namespace, "namespace is kept")
		assert.Empty(t, got.Filter)
		assert.Empty(t, got.CursorName)
	})
}

// Proves the user-decided semantics end to end: a saved session's context
// and namespace survive, only the active tab's ResourceType changes.
func TestNewModel_StartupResourceOverridesSavedSessionActiveTab(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	client := k8s.NewTestClient(nil, nil)
	client.AddTestContext("my-ctx", "https://my-ctx.example:6443")

	writeSessionFile(t, SessionState{
		Context: "my-ctx",
		Tabs: []SessionTab{
			{Context: "my-ctx", Namespace: "team-a", ResourceType: "/v1/pods", Filter: "old-filter", CursorName: "pod-1"},
		},
	})

	m := NewModel(client, StartupOptions{Resource: "deploy"})

	require.Len(t, m.pendingSession.Tabs, 1)
	tab := m.pendingSession.Tabs[0]
	assert.Equal(t, "my-ctx", tab.Context, "saved context is kept")
	assert.Equal(t, "team-a", tab.Namespace, "saved namespace is kept")
	assert.Equal(t, "deploy", tab.ResourceType)
	assert.Empty(t, tab.Filter, "the stale filter is cleared")
	assert.Empty(t, tab.CursorName)
}

func TestNewModel_StartupResourceWithNoSavedSessionBuildsSyntheticTab(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	client := k8s.NewTestClient(nil, nil)

	m := NewModel(client, StartupOptions{Resource: "deploy"})

	require.NotNil(t, m.pendingSession)
	require.Len(t, m.pendingSession.Tabs, 1)
	assert.Equal(t, "deploy", m.pendingSession.Tabs[0].ResourceType)
	assert.True(t, m.restoringSession, "restore guard must be armed when startup resource creates a synthetic session")
}

func TestNewModel_StartupResourceFlagBeatsConfig(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	origCfg := ui.ConfigStartupResource
	defer func() { ui.ConfigStartupResource = origCfg }()
	ui.ConfigStartupResource = "pods"

	client := k8s.NewTestClient(nil, nil)
	m := NewModel(client, StartupOptions{Resource: "deploy"})

	require.Len(t, m.pendingSession.Tabs, 1)
	assert.Equal(t, "deploy", m.pendingSession.Tabs[0].ResourceType, "the flag must win over config")
}

func TestNewModel_StartupResourceFromConfigWhenNoFlag(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	origCfg := ui.ConfigStartupResource
	defer func() { ui.ConfigStartupResource = origCfg }()
	ui.ConfigStartupResource = "pods"

	client := k8s.NewTestClient(nil, nil)
	m := NewModel(client, StartupOptions{})

	require.Len(t, m.pendingSession.Tabs, 1)
	assert.Equal(t, "pods", m.pendingSession.Tabs[0].ResourceType)
}

func TestNewModel_NoStartupResourceLeavesPendingSessionUnset(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	origCfg := ui.ConfigStartupResource
	defer func() { ui.ConfigStartupResource = origCfg }()
	ui.ConfigStartupResource = ""

	client := k8s.NewTestClient(nil, nil)
	m := NewModel(client, StartupOptions{})

	assert.Nil(t, m.pendingSession)
}

// TestRestoreSingleTabSession_UnknownStartupResourceShowsStatusMessage covers
// the synchronous path: the resource can be checked immediately against seed
// resources (no CRD discovery needed for this context).
func TestRestoreSingleTabSession_UnknownStartupResourceShowsStatusMessage(t *testing.T) {
	m := basePush80Model()
	// Discovery already ran and settled for this context, so the missing
	// resource is reported immediately instead of deferred to a discovery reply.
	m.discoveryRefreshedContexts = map[string]bool{"test-ctx": true}
	contexts := []model.Item{{Name: "test-ctx", IsContext: true}}

	mdl, _ := m.restoreSingleTabSession(&SessionState{
		Context: "test-ctx", ResourceType: "does-not-exist",
	}, contexts)
	got := mdl.(Model)

	assert.Equal(t, model.LevelResourceTypes, got.nav.Level, "falls back to the resource-type browser")
	assert.Equal(t, "Startup resource not found: does-not-exist", got.statusMessage)
	assert.True(t, got.statusMessageErr)
}

// TestResumeDeferredSessionRestore_CRDResolvedAfterDiscovery covers a
// "name.group" startup resource whose CRD only appears once real API
// discovery lands, exercising the awaiting-discovery path.
func TestResumeDeferredSessionRestore_CRDResolvedAfterDiscovery(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "ctx-a"
	m.restoringSession = true
	m.sessionResourceTypeAwaitingDiscovery = "widgets.example.io"

	got, cmd := m.updateAPIResourceDiscovery(apiResourceDiscoveryMsg{
		context: "ctx-a",
		entries: []model.ResourceTypeEntry{
			{Kind: "Widget", APIGroup: "example.io", APIVersion: "v1", Resource: "widgets", Namespaced: true},
		},
	})

	assert.Equal(t, model.LevelResources, got.nav.Level)
	assert.Equal(t, "widgets", got.nav.ResourceType.Resource)
	assert.Empty(t, got.sessionResourceTypeAwaitingDiscovery)
	assert.NotNil(t, cmd)
}

// TestResumeDeferredSessionRestore_UnknownStartupResourceShowsStatusMessage
// covers the async path: discovery lands and the requested name still
// resolves to nothing.
func TestResumeDeferredSessionRestore_UnknownStartupResourceShowsStatusMessage(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "ctx-a"
	m.restoringSession = true
	m.sessionResourceTypeAwaitingDiscovery = "does-not-exist"

	got, cmd := m.updateAPIResourceDiscovery(apiResourceDiscoveryMsg{
		context: "ctx-a",
		entries: []model.ResourceTypeEntry{
			{Kind: "Pod", APIVersion: "v1", Resource: "pods", Namespaced: true},
		},
	})

	assert.Equal(t, model.LevelResourceTypes, got.nav.Level)
	assert.Equal(t, "Startup resource not found: does-not-exist", got.statusMessage)
	assert.True(t, got.statusMessageErr)
	assert.False(t, got.restoringSession)
	assert.NotNil(t, cmd)
}
