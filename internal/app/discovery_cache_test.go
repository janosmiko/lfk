package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// withKubeCacheDir points KUBECACHEDIR at a fresh temp dir so each test
// gets an isolated discovery cache without touching the user's home
// directory or the kubectl-shared layout. Returns the dir for assertions.
func withKubeCacheDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("KUBECACHEDIR", dir)
	return dir
}

func TestDiscoveryCacheRoundTripPerHost(t *testing.T) {
	withKubeCacheDir(t)

	host := "https://dev.example.com:6443"
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
			PrinterColumns: []model.PrinterColumn{
				{Name: "Sync Status", Type: "string", JSONPath: ".status.sync.status"},
			},
		},
	}

	require.NoError(t, saveDiscoveryCacheForHost(host, entries))

	loaded := loadDiscoveryCacheForHost(host)
	require.NotNil(t, loaded)
	assert.Equal(t, host, loaded.Host)
	require.Len(t, loaded.Entries, 2)
	assert.Equal(t, "Pod", loaded.Entries[0].Kind)
	assert.Equal(t, "□", loaded.Entries[0].Icon.Unicode)
	assert.Equal(t, "argoproj.io", loaded.Entries[1].APIGroup)
	require.Len(t, loaded.Entries[1].PrinterColumns, 1)
	assert.Equal(t, ".status.sync.status", loaded.Entries[1].PrinterColumns[0].JSONPath)
}

func TestDiscoveryCacheLivesUnderKubeCacheDir(t *testing.T) {
	dir := withKubeCacheDir(t)

	host := "https://prod.example.com:6443"
	require.NoError(t, saveDiscoveryCacheForHost(host, []model.ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods"},
	}))

	// Layout must match kubectl/k9s so `kubectl api-resources --invalidate-cache`
	// wipes lfk's snapshot too. Path: <KUBECACHEDIR>/discovery/<hostHash>/lfk-enriched.yaml
	expected := filepath.Join(dir, "discovery", k8s.CacheHostDir(host), "lfk-enriched.yaml")
	_, err := os.Stat(expected)
	assert.NoError(t, err, "cache file must live under <KUBECACHEDIR>/discovery/<host>/, got path: %s", expected)
}

func TestDiscoveryCacheMultipleHostsAreIndependent(t *testing.T) {
	withKubeCacheDir(t)

	require.NoError(t, saveDiscoveryCacheForHost("https://a.example:6443", []model.ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods"},
	}))
	require.NoError(t, saveDiscoveryCacheForHost("https://b.example:6443", []model.ResourceTypeEntry{
		{Kind: "Service", APIGroup: "", APIVersion: "v1", Resource: "services"},
		{Kind: "ConfigMap", APIGroup: "", APIVersion: "v1", Resource: "configmaps"},
	}))

	a := loadDiscoveryCacheForHost("https://a.example:6443")
	require.NotNil(t, a)
	require.Len(t, a.Entries, 1)
	assert.Equal(t, "Pod", a.Entries[0].Kind)

	b := loadDiscoveryCacheForHost("https://b.example:6443")
	require.NotNil(t, b)
	require.Len(t, b.Entries, 2)
	assert.Equal(t, "Service", b.Entries[0].Kind)
}

func TestLoadDiscoveryCacheMissingFile(t *testing.T) {
	withKubeCacheDir(t)
	assert.Nil(t, loadDiscoveryCacheForHost("https://nope.example:6443"),
		"missing file should return nil, not error")
}

func TestLoadDiscoveryCacheCorruptFile(t *testing.T) {
	withKubeCacheDir(t)

	host := "https://corrupt.example:6443"
	path := discoveryCacheFilePathForHost(host)
	require.NotEmpty(t, path)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("not: [valid: yaml"), 0o644))

	assert.Nil(t, loadDiscoveryCacheForHost(host),
		"corrupt file should return nil so the app starts fresh")
}

func TestLoadDiscoveryCacheRejectsFutureSchemaVersion(t *testing.T) {
	withKubeCacheDir(t)

	host := "https://future.example:6443"
	path := discoveryCacheFilePathForHost(host)
	require.NotEmpty(t, path)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	// schema_version 99 is from the future — must be ignored gracefully.
	require.NoError(t, os.WriteFile(path, []byte("schema_version: 99\nhost: x\nentries: []\n"), 0o644))

	assert.Nil(t, loadDiscoveryCacheForHost(host),
		"unknown schema version must be ignored, not crash")
}

func TestLoadAllDiscoveryCachesMapsContextsToHostFiles(t *testing.T) {
	withKubeCacheDir(t)

	hostA := "https://a.example:6443"
	hostB := "https://b.example:6443"

	// Two contexts share host A — both must populate from the single file.
	// One context points at host B — gets its own file.
	require.NoError(t, saveDiscoveryCacheForHost(hostA, []model.ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods"},
	}))
	require.NoError(t, saveDiscoveryCacheForHost(hostB, []model.ResourceTypeEntry{
		{Kind: "Service", APIGroup: "", APIVersion: "v1", Resource: "services"},
	}))

	client := k8s.NewTestClient(nil, nil)
	client.AddTestContext("alpha", hostA)
	client.AddTestContext("beta", hostA) // same host as alpha
	client.AddTestContext("gamma", hostB)

	loaded := loadAllDiscoveryCaches(client)
	require.Len(t, loaded["alpha"], 1)
	require.Len(t, loaded["beta"], 1)
	require.Len(t, loaded["gamma"], 1)
	assert.Equal(t, "Pod", loaded["alpha"][0].Kind)
	assert.Equal(t, "Pod", loaded["beta"][0].Kind, "two contexts on same host share the file")
	assert.Equal(t, "Service", loaded["gamma"][0].Kind)
}

func TestLoadAllDiscoveryCachesSkipsContextsWithUnresolvableHost(t *testing.T) {
	withKubeCacheDir(t)

	// orphan-ctx exists but has no host registered — must not crash and
	// must not appear in the result map.
	client := k8s.NewTestClient(nil, nil)
	client.AddTestContext("alpha", "https://a.example:6443")
	require.NoError(t, saveDiscoveryCacheForHost("https://a.example:6443", []model.ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods"},
	}))

	loaded := loadAllDiscoveryCaches(client)
	require.NotNil(t, loaded["alpha"])
	assert.NotContains(t, loaded, "test-ctx",
		"the default test-ctx had no cached host file — must not show up here")
}

func TestNewModelPrefillsDiscoveredResourcesFromCache(t *testing.T) {
	withKubeCacheDir(t)

	host := "https://dev.example:6443"
	require.NoError(t, saveDiscoveryCacheForHost(host, []model.ResourceTypeEntry{
		{Kind: "Application", APIGroup: "argoproj.io", APIVersion: "v1alpha1", Resource: "applications", Namespaced: true},
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
	}))

	client := newTestClientForOptions(t)
	client.AddTestContext("dev-cluster", host)

	m := NewModel(client, StartupOptions{})

	entries, ok := m.discoveredResources["dev-cluster"]
	require.True(t, ok, "cached context must populate discoveredResources at NewModel time")
	require.GreaterOrEqual(t, len(entries), 2)

	kinds := make(map[string]bool)
	for _, e := range entries {
		kinds[e.Kind] = true
	}
	assert.True(t, kinds["Application"], "ArgoCD Application from cache must be present")
	assert.True(t, kinds["Pod"], "Pod from cache must be present")

	assert.False(t, m.discoveryRefreshedContexts["dev-cluster"],
		"cached prefill must not be marked as refreshed — live discovery still has to run")
}

func TestUpdateAPIResourceDiscoveryWritesCache(t *testing.T) {
	withKubeCacheDir(t)

	host := "https://dev.example:6443"
	client := k8s.NewTestClient(nil, nil)
	client.AddTestContext("dev-cluster", host)

	m := Model{
		client:                     client,
		discoveredResources:        make(map[string][]model.ResourceTypeEntry),
		discoveringContexts:        map[string]bool{"dev-cluster": true},
		discoveryRefreshedContexts: make(map[string]bool),
	}

	entries := []model.ResourceTypeEntry{
		{Kind: "Application", APIGroup: "argoproj.io", APIVersion: "v1alpha1", Resource: "applications", Namespaced: true},
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true},
	}
	msg := apiResourceDiscoveryMsg{context: "dev-cluster", entries: entries}
	updated, _ := m.updateAPIResourceDiscovery(msg)

	assert.True(t, updated.discoveryRefreshedContexts["dev-cluster"],
		"successful discovery must mark context as freshly refreshed this session")

	cache := loadDiscoveryCacheForHost(host)
	require.NotNil(t, cache, "discovery success must write a cache file at the host's path")
	assert.Equal(t, host, cache.Host)
	require.Len(t, cache.Entries, 2)
	for _, e := range cache.Entries {
		assert.NotEqual(t, "_helm", e.APIGroup, "pseudo-resource leaked into cache")
		assert.NotEqual(t, "_portforward", e.APIGroup, "pseudo-resource leaked into cache")
	}
}

func TestUpdateAPIResourceDiscoveryFailureDoesNotWriteCache(t *testing.T) {
	withKubeCacheDir(t)

	host := "https://dev.example:6443"
	client := k8s.NewTestClient(nil, nil)
	client.AddTestContext("dev-cluster", host)

	// Pre-populate so we can verify the failed call leaves it alone.
	require.NoError(t, saveDiscoveryCacheForHost(host, []model.ResourceTypeEntry{
		{Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods"},
	}))
	before := loadDiscoveryCacheForHost(host)
	require.NotNil(t, before)
	beforeUpdated := before.UpdatedAt

	m := Model{
		client:                     client,
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

	after := loadDiscoveryCacheForHost(host)
	require.NotNil(t, after)
	assert.Equal(t, beforeUpdated, after.UpdatedAt,
		"failed discovery must not overwrite the previous good snapshot")
}

// assertSimpleError returns a basic error value for tests.
type assertSimpleError string

func (e assertSimpleError) Error() string { return string(e) }
