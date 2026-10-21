package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func rememberTestModel(t *testing.T, enabled bool) Model {
	t.Helper()
	t.Setenv("LFK_STATE_DIR", t.TempDir())
	origRemember := ui.ConfigRememberNamespaces
	origAll, origSet := ui.ConfigAllNamespaces, ui.ConfigAllNamespacesSet
	t.Cleanup(func() {
		ui.ConfigRememberNamespaces = origRemember
		ui.ConfigAllNamespaces, ui.ConfigAllNamespacesSet = origAll, origSet
	})
	ui.ConfigRememberNamespaces = enabled
	m := baseModelWithFakeClient()
	m.client.AddTestContextWithNamespace("staging", "https://staging.example.local:6443", "pinned")
	m.client.AddTestContextWithNamespace("prod", "https://prod.example.local:6443", "")
	return m
}

func rememberedFileExists(t *testing.T) bool {
	t.Helper()
	path := stateFilePath(rememberedNamespacesFile)
	require.NotEmpty(t, path)
	_, err := os.Stat(path)
	return err == nil
}

func TestRememberedScope_RoundTrip(t *testing.T) {
	m := rememberTestModel(t, true)
	m.namespace = "team-a"
	m.allNamespaces = false
	m.selectedNamespaces = map[string]bool{"b": true, "a": true}
	m.nsSelectionNegated = true

	m.rememberScope("test-ctx")

	got, ok := loadRememberedScope("test-ctx")
	require.True(t, ok)
	assert.Equal(t, "team-a", got.namespace)
	assert.False(t, got.allNamespaces)
	assert.Equal(t, map[string]bool{"a": true, "b": true}, got.selectedNamespaces)
	assert.True(t, got.nsSelectionNegated)
}

func TestLoadRememberedScope_MissingAndCorrupt(t *testing.T) {
	_ = rememberTestModel(t, true)

	_, ok := loadRememberedScope("nope")
	assert.False(t, ok, "missing file")

	require.NoError(t, os.MkdirAll(filepath.Dir(stateFilePath(rememberedNamespacesFile)), 0o700))
	require.NoError(t, os.WriteFile(stateFilePath(rememberedNamespacesFile), []byte("contexts: [not a map"), 0o600))
	_, ok = loadRememberedScope("nope")
	assert.False(t, ok, "corrupt file")
}

func TestRememberedScope_UnknownContextIsMiss(t *testing.T) {
	m := rememberTestModel(t, true)
	m.rememberScope("test-ctx")
	_, ok := loadRememberedScope("other")
	assert.False(t, ok)
}

func TestRescope_RestoresRememberedScopePerContext(t *testing.T) {
	m := rememberTestModel(t, true)
	m.namespace = "team-a"
	m.rememberScope("test-ctx")

	m.rescopeNamespaceForContext("staging")
	assert.Equal(t, "pinned", m.namespace, "no memory for staging falls back to the pin")

	m.rescopeNamespaceForContext("test-ctx")
	assert.Equal(t, "team-a", m.namespace)
	assert.False(t, m.allNamespaces)
}

func TestRescope_RememberedBeatsPinAndAllNamespacesConfig(t *testing.T) {
	m := rememberTestModel(t, true)
	ui.ConfigAllNamespaces, ui.ConfigAllNamespacesSet = true, true
	m.namespace = "team-a"
	m.rememberScope("staging")

	m.allNamespaces = true
	m.rescopeNamespaceForContext("staging")

	assert.Equal(t, "team-a", m.namespace)
	assert.False(t, m.allNamespaces)
}

func TestRescope_RememberedAllNamespacesBeatsPin(t *testing.T) {
	m := rememberTestModel(t, true)
	m.allNamespaces = true
	m.namespace = ""
	m.rememberScope("staging")

	m.allNamespaces = false
	m.namespace = "x"
	m.rescopeNamespaceForContext("staging")

	assert.True(t, m.allNamespaces)
	assert.Equal(t, "", m.namespace)
}

func TestRemember_ToggleOffChangesNothingAndWritesNoFile(t *testing.T) {
	m := rememberTestModel(t, false)
	m.namespace = "team-a"
	m.rememberScope("test-ctx")
	m.rememberAllTabScopes()
	assert.False(t, rememberedFileExists(t))

	m.rescopeNamespaceForContext("staging")
	assert.Equal(t, "pinned", m.namespace)
}

func TestRemember_ToggleOffIgnoresExistingFile(t *testing.T) {
	m := rememberTestModel(t, true)
	m.namespace = "team-a"
	m.rememberScope("staging")

	ui.ConfigRememberNamespaces = false
	m.rescopeNamespaceForContext("staging")
	assert.Equal(t, "pinned", m.namespace)
}

func TestRemember_UnionModeAndEmptyContextSkipped(t *testing.T) {
	m := rememberTestModel(t, true)
	m.namespace = "team-a"
	m.rememberScope("")
	assert.False(t, rememberedFileExists(t), "empty context")

	m.unionMode = true
	m.rememberScope("test-ctx")
	m.rememberAllTabScopes()
	assert.False(t, rememberedFileExists(t), "union mode")
}

func TestNavigateChildCluster_RemembersLeavingScopeAndRestores(t *testing.T) {
	m := rememberTestModel(t, true)
	m.namespace = "team-a"

	got, _ := m.navigateChildCluster(&model.Item{Name: "staging"})
	m = got.(Model)
	assert.Equal(t, "pinned", m.namespace)
	scope, ok := loadRememberedScope("test-ctx")
	require.True(t, ok)
	assert.Equal(t, "team-a", scope.namespace)

	m.namespace = "team-b"
	got, _ = m.navigateChildCluster(&model.Item{Name: "test-ctx"})
	m = got.(Model)
	assert.Equal(t, "team-a", m.namespace)
}

func TestExecuteContextCommand_RemembersLeavingScopeAndRestores(t *testing.T) {
	m := rememberTestModel(t, true)
	m.namespace = "team-a"

	got, _ := m.executeContextCommand("staging")
	m = got.(Model)
	assert.Equal(t, "pinned", m.namespace)

	got, _ = m.executeContextCommand("test-ctx")
	m = got.(Model)
	assert.Equal(t, "team-a", m.namespace)
}

func TestBeginShutdown_RemembersEveryTabContext(t *testing.T) {
	m := rememberTestModel(t, true)
	m.namespace = "team-a"
	m.tabs = []TabState{{}, {}}
	m.tabs[1].nav.Context = "staging"
	m.tabs[1].namespace = "team-s"
	m.tabs[1].selectedNamespaces = map[string]bool{"team-s": true}

	got, _ := m.beginShutdown()
	_ = got

	a, ok := loadRememberedScope("test-ctx")
	require.True(t, ok)
	assert.Equal(t, "team-a", a.namespace)
	b, ok := loadRememberedScope("staging")
	require.True(t, ok)
	assert.Equal(t, "team-s", b.namespace)
}

func TestBeginShutdown_ToggleOffWritesNoRememberedFile(t *testing.T) {
	m := rememberTestModel(t, false)
	m.beginShutdown() //nolint:errcheck // only the file side effect matters
	assert.False(t, rememberedFileExists(t))
}

func rememberedStaging(t *testing.T, m Model) Model {
	t.Helper()
	m.namespace = "remembered"
	m.selectedNamespaces = map[string]bool{"remembered": true, "other": true}
	m.rememberScope("staging")
	return m
}

func TestNewModel_ContextOnlyStartsOnRememberedScope(t *testing.T) {
	m := rememberedStaging(t, rememberTestModel(t, true))

	nm := NewModel(m.client, StartupOptions{Context: "staging"})

	require.NotNil(t, nm.pendingSession)
	tab := nm.pendingSession.Tabs[0]
	assert.Equal(t, "remembered", tab.Namespace, "remembered beats the kubeconfig pin")
	assert.False(t, tab.AllNamespaces)
	assert.Equal(t, []string{"other", "remembered"}, tab.SelectedNamespaces)
}

func TestNewModel_ContextOnlyToggleOffUsesPin(t *testing.T) {
	m := rememberedStaging(t, rememberTestModel(t, true))
	ui.ConfigRememberNamespaces = false

	nm := NewModel(m.client, StartupOptions{Context: "staging"})

	require.NotNil(t, nm.pendingSession)
	assert.Equal(t, "pinned", nm.pendingSession.Tabs[0].Namespace)
}

func TestNewModel_CLINamespaceBeatsRemembered(t *testing.T) {
	m := rememberedStaging(t, rememberTestModel(t, true))

	nm := NewModel(m.client, StartupOptions{Context: "staging", Namespaces: []string{"cli-ns"}})

	require.NotNil(t, nm.pendingSession)
	assert.Equal(t, "cli-ns", nm.pendingSession.Tabs[0].Namespace)
	assert.Equal(t, []string{"cli-ns"}, nm.pendingSession.Tabs[0].SelectedNamespaces)
}

func TestNewModel_SessionScopeBeatsRemembered(t *testing.T) {
	m := rememberedStaging(t, rememberTestModel(t, true))
	require.NoError(t, saveSession(SessionState{
		Context: "staging",
		Tabs:    []SessionTab{{Context: "staging", Namespace: "from-session"}},
	}))

	nm := NewModel(m.client, StartupOptions{})

	require.NotNil(t, nm.pendingSession)
	assert.Equal(t, "from-session", nm.pendingSession.Tabs[0].Namespace)
	assert.False(t, nm.pendingSession.Tabs[0].AllNamespaces)
}

func TestBookmarkJump_KeepsBookmarkNamespaceOverRemembered(t *testing.T) {
	m := rememberedStaging(t, rememberTestModel(t, true))
	m.bookmarkLoadNamespace = true
	bm := model.Bookmark{Namespace: "bm-ns"}
	target := bookmarkTarget{kind: bookmarkTargetContext, context: "staging"}

	oldCtx := m.nav.Context
	m.applyBookmarkContextSwitch(target)
	m.applyBookmarkNamespace(bm, target, oldCtx)

	assert.Equal(t, "bm-ns", m.namespace)
	assert.Equal(t, map[string]bool{"bm-ns": true}, m.selectedNamespaces)
}

func TestRememberScope_CorruptFileNotOverwritten(t *testing.T) {
	m := rememberTestModel(t, true)
	path := stateFilePath(rememberedNamespacesFile)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("contexts: [not a map"), 0o600))

	m.rememberScope("test-ctx")

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "contexts: [not a map", string(data))
}
