package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

const crdSessionHost = "https://dev-envs.example:6443"

// writeSessionFile puts a saved session on disk where loadStartupSession finds it.
func writeSessionFile(t *testing.T, sess SessionState) {
	t.Helper()
	data, err := yaml.Marshal(sess)
	require.NoError(t, err)
	path := sessionFilePath()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, data, 0o600))
}

// crdSessionClient is a client whose dev-envs context resolves to a host with a
// cached discovery snapshot that carries the Argo WorkflowTemplate CRD.
func crdSessionClient(t *testing.T) *k8s.Client {
	t.Helper()
	require.NoError(t, saveDiscoveryCacheForHost(crdSessionHost, []model.ResourceTypeEntry{
		{Kind: "Pod", APIVersion: "v1", Resource: "pods", Namespaced: true},
		{
			Kind: "WorkflowTemplate", APIGroup: "argoproj.io", APIVersion: "v1alpha1",
			Resource: "workflowtemplates", Namespaced: true,
		},
	}))
	client := k8s.NewTestClient(nil, nil)
	client.AddTestContext("dev-envs", crdSessionHost)
	return client
}

func TestNewModel_SeedsTheSavedContextFromTheDiscoveryCache(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	withKubeCacheDir(t)
	client := crdSessionClient(t)
	writeSessionFile(t, SessionState{
		Context:      "dev-envs",
		ResourceType: "argoproj.io/v1alpha1/workflowtemplates",
	})

	m := NewModel(client, StartupOptions{})

	_, ok := model.FindResourceTypeIn("argoproj.io/v1alpha1/workflowtemplates", m.discoveredResources["dev-envs"])
	assert.True(t, ok, "the saved CRD must be resolvable before the first frame")
}

func TestRestoreSession_CRDLandsOnTheListWithoutPassingThroughTheTypeBrowser(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	withKubeCacheDir(t)
	client := crdSessionClient(t)
	writeSessionFile(t, SessionState{
		Context:      "dev-envs",
		ResourceType: "argoproj.io/v1alpha1/workflowtemplates",
		CursorName:   "backup-db",
	})

	m := NewModel(client, StartupOptions{})
	m.width, m.height = 200, 50

	mdl, _ := m.updateContextsLoaded(contextsLoadedMsg{
		items: []model.Item{{Name: "dev-envs", IsContext: true}},
	})
	got := mdl.(Model)

	assert.Equal(t, model.LevelResources, got.nav.Level,
		"the restore must go straight to the CRD list, not park on the resource-type browser")
	assert.Equal(t, "workflowtemplates", got.nav.ResourceType.Resource)
	assert.Equal(t, "backup-db", got.pendingTarget)
	assert.Empty(t, got.sessionResourceTypeAwaitingDiscovery,
		"nothing should be waiting on discovery once the cache resolved the type")
}

func TestNewModel_ReadsOnlyTheSavedContextsCache(t *testing.T) {
	// A kubeconfig can hold thousands of contexts, and resolving a host costs
	// one clientcmd.ClientConfig() call each. Only the context the session
	// opens may be read at startup.
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	withKubeCacheDir(t)
	client := crdSessionClient(t)
	otherHost := "https://other.example:6443"
	require.NoError(t, saveDiscoveryCacheForHost(otherHost, []model.ResourceTypeEntry{
		{Kind: "Service", APIVersion: "v1", Resource: "services", Namespaced: true},
	}))
	client.AddTestContext("other", otherHost)
	writeSessionFile(t, SessionState{Context: "dev-envs", ResourceType: "argoproj.io/v1alpha1/workflowtemplates"})

	m := NewModel(client, StartupOptions{})

	assert.NotEmpty(t, m.discoveredResources["dev-envs"])
	assert.Empty(t, m.discoveredResources["other"],
		"contexts the session does not open stay for the async preload")
}

func TestNewModel_NoSavedSessionReadsNoCache(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	withKubeCacheDir(t)
	client := crdSessionClient(t)

	m := NewModel(client, StartupOptions{})

	assert.Empty(t, m.discoveredResources["dev-envs"])
}

func TestNewModel_MultiTabSeedsTheActiveTabsContext(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	withKubeCacheDir(t)
	client := crdSessionClient(t)
	// buildSessionState mirrors the active tab's context into the legacy
	// top-level field, so a real file carries both.
	writeSessionFile(t, SessionState{
		Context:   "dev-envs",
		ActiveTab: 1,
		Tabs: []SessionTab{
			{Context: "other", ResourceType: "/v1/pods"},
			{Context: "dev-envs", ResourceType: "argoproj.io/v1alpha1/workflowtemplates"},
		},
	})

	m := NewModel(client, StartupOptions{})

	_, ok := model.FindResourceTypeIn("argoproj.io/v1alpha1/workflowtemplates", m.discoveredResources["dev-envs"])
	assert.True(t, ok, "the active tab decides which context is worth a synchronous read")
	assert.Empty(t, m.discoveredResources["other"], "background tabs wait for the async preload")
}

func TestSessionRestoreContext_PrefersTheActiveTabOverTheLegacyField(t *testing.T) {
	sess := &SessionState{
		Context:   "stale-legacy-value",
		ActiveTab: 1,
		Tabs:      []SessionTab{{Context: "first"}, {Context: "second"}},
	}

	assert.Equal(t, "second", sessionRestoreContext(sess))
	assert.Equal(t, "only", sessionRestoreContext(&SessionState{Context: "only"}))
	assert.Equal(t, "first", sessionRestoreContext(&SessionState{
		ActiveTab: 9, Tabs: []SessionTab{{Context: "first"}},
	}), "an out-of-range index falls back to the first tab")
	assert.Empty(t, sessionRestoreContext(nil))
}
