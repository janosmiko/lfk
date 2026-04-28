package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

func TestDiscoveryCacheRoundTrip(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	entries := []model.ResourceTypeEntry{
		{
			DisplayName: "Pods",
			Kind:        "Pod",
			APIGroup:    "",
			APIVersion:  "v1",
			Resource:    "pods",
			Namespaced:  true,
			Verbs:       []string{"get", "list", "watch"},
			Icon:        model.Icon{Unicode: "□", Simple: "[Po]"},
		},
		{
			DisplayName: "ArgoCD Applications",
			Kind:        "Application",
			APIGroup:    "argoproj.io",
			APIVersion:  "v1alpha1",
			Resource:    "applications",
			Namespaced:  true,
			Verbs:       []string{"get", "list", "watch"},
			PrinterColumns: []model.PrinterColumn{
				{Name: "Sync Status", Type: "string", JSONPath: ".status.sync.status"},
			},
		},
		{
			DisplayName:    "ReplicationControllers",
			Kind:           "ReplicationController",
			APIGroup:       "",
			APIVersion:     "v1",
			Resource:       "replicationcontrollers",
			Namespaced:     true,
			Deprecated:     true,
			DeprecationMsg: "use Deployments",
		},
	}

	require.NoError(t, updateDiscoveryCacheContext("dev-cluster", entries))
	require.NoError(t, updateDiscoveryCacheContext("prod-cluster", entries[:1]))

	loaded := loadDiscoveryCache()
	require.NotNil(t, loaded)
	require.Contains(t, loaded.Contexts, "dev-cluster")
	require.Contains(t, loaded.Contexts, "prod-cluster")

	dev := loaded.Contexts["dev-cluster"]
	require.Len(t, dev.Entries, 3)
	assert.Equal(t, "Pod", dev.Entries[0].Kind)
	assert.Equal(t, "v1", dev.Entries[0].APIVersion)
	assert.True(t, dev.Entries[0].Namespaced)
	assert.Equal(t, []string{"get", "list", "watch"}, dev.Entries[0].Verbs)
	assert.Equal(t, "□", dev.Entries[0].Icon.Unicode)

	assert.Equal(t, "argoproj.io", dev.Entries[1].APIGroup)
	require.Len(t, dev.Entries[1].PrinterColumns, 1)
	assert.Equal(t, "Sync Status", dev.Entries[1].PrinterColumns[0].Name)
	assert.Equal(t, ".status.sync.status", dev.Entries[1].PrinterColumns[0].JSONPath)

	assert.True(t, dev.Entries[2].Deprecated)
	assert.Equal(t, "use Deployments", dev.Entries[2].DeprecationMsg)

	assert.Len(t, loaded.Contexts["prod-cluster"].Entries, 1)
}

func TestDiscoveryCacheUpdatePreservesOtherContexts(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	require.NoError(t, updateDiscoveryCacheContext("a", []model.ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods"},
	}))
	require.NoError(t, updateDiscoveryCacheContext("b", []model.ResourceTypeEntry{
		{Kind: "Service", APIGroup: "", APIVersion: "v1", Resource: "services"},
	}))

	// Updating "a" must not erase "b".
	require.NoError(t, updateDiscoveryCacheContext("a", []model.ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods"},
		{Kind: "ConfigMap", APIGroup: "", APIVersion: "v1", Resource: "configmaps"},
	}))

	loaded := loadDiscoveryCache()
	require.NotNil(t, loaded)
	assert.Len(t, loaded.Contexts["a"].Entries, 2, "context a was updated")
	assert.Len(t, loaded.Contexts["b"].Entries, 1, "context b must survive untouched")
	assert.Equal(t, "Service", loaded.Contexts["b"].Entries[0].Kind)
}

func TestLoadDiscoveryCacheMissingFile(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	loaded := loadDiscoveryCache()
	assert.Nil(t, loaded, "missing file should return nil, not error")
}

func TestLoadDiscoveryCacheCorruptFile(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)

	path := discoveryCacheFilePath()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("not: [valid: yaml"), 0o644))

	loaded := loadDiscoveryCache()
	assert.Nil(t, loaded, "corrupt file should return nil so the app starts fresh")
}

func TestNavigateClusterFiresDiscoveryWhenCachePrefillIsStale(t *testing.T) {
	// Stale-while-revalidate: cache prefills discoveredResources, but the
	// first navigation/hover within the session must still kick off a live
	// discovery so the user sees fresh data after the brief stale window.
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	require.NoError(t, updateDiscoveryCacheContext("dev-cluster", []model.ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods"},
	}))

	client := newTestClientForOptions(t)
	m := NewModel(client, StartupOptions{})

	require.NotEmpty(t, m.discoveredResources["dev-cluster"], "cache prefill missing")
	require.False(t, m.discoveryRefreshedContexts["dev-cluster"], "cache prefill must not pose as fresh")
	require.False(t, m.discoveringContexts["dev-cluster"], "no discovery should be in flight yet")

	assert.True(t, m.shouldFireDiscoveryFor("dev-cluster"),
		"a context that exists only in the disk cache must still trigger live discovery")
}

func TestNavigateClusterSkipsDiscoveryAfterRefresh(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	m := NewModel(newTestClientForOptions(t), StartupOptions{})
	m.discoveryRefreshedContexts["dev-cluster"] = true

	assert.False(t, m.shouldFireDiscoveryFor("dev-cluster"),
		"a context refreshed earlier this session must not re-fire on subsequent hovers")
}

func TestNavigateClusterSkipsDiscoveryWhileInFlight(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	m := NewModel(newTestClientForOptions(t), StartupOptions{})
	m.discoveringContexts["dev-cluster"] = true

	assert.False(t, m.shouldFireDiscoveryFor("dev-cluster"),
		"deduplication must keep an in-flight discovery from being kicked off twice")
}

func TestUpdateAPIResourceDiscoveryWritesCache(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	m := Model{
		discoveredResources:        make(map[string][]model.ResourceTypeEntry),
		discoveringContexts:        map[string]bool{"dev-cluster": true},
		discoveryRefreshedContexts: make(map[string]bool),
		// nav.Context = "" so this test exercises the write path even when
		// the user isn't currently looking at the cluster.
	}

	entries := []model.ResourceTypeEntry{
		{Kind: "Application", APIGroup: "argoproj.io", APIVersion: "v1alpha1", Resource: "applications", Namespaced: true},
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
	}
	msg := apiResourceDiscoveryMsg{context: "dev-cluster", entries: entries}
	updated, _ := m.updateAPIResourceDiscovery(msg)

	assert.True(t, updated.discoveryRefreshedContexts["dev-cluster"],
		"successful discovery must mark context as freshly refreshed this session")

	cache := loadDiscoveryCache()
	require.NotNil(t, cache, "discovery success must write a cache file")
	require.Contains(t, cache.Contexts, "dev-cluster")

	cached := cache.Contexts["dev-cluster"]
	require.Len(t, cached.Entries, 2)
	// Pseudo-resources (helm-releases, port-forwards) must NOT bleed into
	// the persisted snapshot — those are prepended at runtime.
	for _, e := range cached.Entries {
		assert.NotEqual(t, "_helm", e.APIGroup, "pseudo-resource leaked into cache")
		assert.NotEqual(t, "_portforward", e.APIGroup, "pseudo-resource leaked into cache")
	}
}

func TestUpdateAPIResourceDiscoveryFailureDoesNotWriteCache(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	// Pre-populate a cache so we can verify the failed call leaves it alone.
	require.NoError(t, updateDiscoveryCacheContext("dev-cluster", []model.ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods"},
	}))
	before := loadDiscoveryCache()
	require.NotNil(t, before)
	beforeUpdated := before.Contexts["dev-cluster"].UpdatedAt

	m := Model{
		discoveredResources:        make(map[string][]model.ResourceTypeEntry),
		discoveringContexts:        map[string]bool{"dev-cluster": true},
		discoveryRefreshedContexts: make(map[string]bool),
	}

	msg := apiResourceDiscoveryMsg{
		context: "dev-cluster",
		err:     assertSimpleError("discovery failed: forbidden"),
	}
	updated, _ := m.updateAPIResourceDiscovery(msg)

	assert.False(t, updated.discoveryRefreshedContexts["dev-cluster"],
		"failed discovery must NOT mark the context as refreshed")

	after := loadDiscoveryCache()
	require.NotNil(t, after)
	assert.Equal(t, beforeUpdated, after.Contexts["dev-cluster"].UpdatedAt,
		"failed discovery must not overwrite the previous good snapshot")
}

// assertSimpleError returns a basic error value for tests.
type assertSimpleError string

func (e assertSimpleError) Error() string { return string(e) }

func TestNewModelPrefillsDiscoveredResourcesFromCache(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	require.NoError(t, updateDiscoveryCacheContext("dev-cluster", []model.ResourceTypeEntry{
		{Kind: "Application", APIGroup: "argoproj.io", APIVersion: "v1alpha1", Resource: "applications", Namespaced: true},
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
	}))

	client := newTestClientForOptions(t)
	m := NewModel(client, StartupOptions{})

	// Sidebar must paint immediately from the cache rather than wait for a
	// fresh discovery round-trip — that's the entire point of stale-while-
	// revalidate.
	entries, ok := m.discoveredResources["dev-cluster"]
	require.True(t, ok, "cached context must populate discoveredResources at NewModel time")

	// Pseudo-resources (helm releases, port forwards) are prepended at
	// runtime; the cache stores the cluster-reported set verbatim, so the
	// loaded length is pseudo + cached.
	require.GreaterOrEqual(t, len(entries), 2)

	kinds := make(map[string]bool)
	for _, e := range entries {
		kinds[e.Kind] = true
	}
	assert.True(t, kinds["Application"], "ArgoCD Application from cache must be present")
	assert.True(t, kinds["Pod"], "Pod from cache must be present")

	// discoveryRefreshedContexts stays empty: the data is from disk, not
	// from a live API call this session.
	assert.False(t, m.discoveryRefreshedContexts["dev-cluster"],
		"cached prefill must not be marked as refreshed — live discovery still has to run")
}

func TestLoadDiscoveryCacheRejectsFutureSchemaVersion(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)

	path := discoveryCacheFilePath()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	// schema_version 99 is from the future — must be ignored gracefully.
	require.NoError(t, os.WriteFile(path, []byte("schema_version: 99\ncontexts: {}\n"), 0o644))

	loaded := loadDiscoveryCache()
	assert.Nil(t, loaded, "unknown schema version must be ignored, not crash")
}
