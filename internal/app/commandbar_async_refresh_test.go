package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// argoCommandBar switches to "argo" before its discovery lands and opens the
// command bar with input already typed.
func argoCommandBar(t *testing.T, input string) Model {
	t.Helper()
	withKubeCacheDir(t)
	m := ctxSwitchModel()
	delete(m.discoveredResources, "argo")
	got, _ := m.executeContextCommand("argo")
	m = got.(Model)
	m.client = k8s.NewTestClient(nil, nil) // a nil client makes every name fetch a nil cmd
	m.commandBarActive = true
	m.commandBarInput.Value = input
	m, _ = m.commandBarRefreshSuggestions()
	return m
}

func argoDiscoveryMsg() apiResourceDiscoveryMsg {
	return apiResourceDiscoveryMsg{
		context: "argo",
		entries: append(model.SeedResources(), model.ResourceTypeEntry{
			Kind: "Application", APIGroup: "argoproj.io", APIVersion: "v1alpha1",
			Resource: "applications", Namespaced: true,
		}),
	}
}

func TestUpdateAPIResourceDiscovery_KeepsEscDismissedSuggestionsClosed(t *testing.T) {
	m := argoCommandBar(t, "co")
	require.NotEmpty(t, m.commandBarSuggestions)
	got, _ := m.commandBarHandleEsc()
	m = got.(Model)

	m, _ = m.updateAPIResourceDiscovery(argoDiscoveryMsg())

	assert.Empty(t, m.commandBarSuggestions, "Enter must run the input, not accept a reopened suggestion")
}

func TestUpdateAPIResourceDiscovery_KeepsAcceptedSuggestionsClosed(t *testing.T) {
	m := argoCommandBar(t, "co")
	require.NotEmpty(t, m.commandBarSuggestions)
	got, _ := m.commandBarEnter()
	m = got.(Model)
	require.True(t, m.commandBarActive)

	m, _ = m.updateAPIResourceDiscovery(argoDiscoveryMsg())

	assert.Empty(t, m.commandBarSuggestions)
}

func TestUpdateAPIResourceDiscovery_KeepsCycledSelection(t *testing.T) {
	m := argoCommandBar(t, "co")
	require.Greater(t, m.commandBarActionableSuggestionCount(), 1)
	got, _ := m.commandBarCycleForward("tab")
	m = got.(Model)
	picked := m.commandBarPreview
	require.NotEmpty(t, picked)

	m, _ = m.updateAPIResourceDiscovery(argoDiscoveryMsg())

	assert.Equal(t, picked, m.commandBarPreview)
	require.Less(t, m.commandBarSelectedSuggestion, len(m.commandBarSuggestions))
	assert.Equal(t, picked, m.commandBarSuggestions[m.commandBarSelectedSuggestion].Text)
}

func TestUpdateAPIResourceDiscovery_DispatchesStartedNameFetch(t *testing.T) {
	m := argoCommandBar(t, "kubectl get configmaps ")
	m.commandBarNameLoading = "" // the resolved name changed, so no fetch is in flight for it

	m, cmd := m.updateAPIResourceDiscovery(argoDiscoveryMsg())

	require.NotEmpty(t, m.commandBarNameLoading)
	assert.True(t, hasSuggestion(m.commandBarSuggestions, "loading..."))
	assert.NotNil(t, cmd)
}

func TestCommandBarNamesFetched_KeepsDismissedSuggestionsClosed(t *testing.T) {
	m := argoCommandBar(t, "kubectl get configmaps ")
	key := m.commandBarNameLoading
	require.NotEmpty(t, key)
	got, _ := m.commandBarHandleEsc()
	m = got.(Model)

	ret, _ := m.Update(commandBarNamesFetchedMsg{cacheKey: key, names: []string{"web-1", "web-2"}})
	m = ret.(Model)

	assert.Empty(t, m.commandBarSuggestions)
}

func TestCommandBarNamesFetched_KeepsSelectionInRange(t *testing.T) {
	m := argoCommandBar(t, "kubectl get configmaps ")
	key := m.commandBarNameLoading
	m.commandBarSuggestions = []ui.Suggestion{{Text: "a"}, {Text: "b"}, {Text: "c"}, {Text: "d"}}
	m.commandBarSelectedSuggestion = 3

	ret, _ := m.Update(commandBarNamesFetchedMsg{cacheKey: key, names: []string{"web-1"}})
	m = ret.(Model)

	assert.Less(t, m.commandBarSelectedSuggestion, len(m.commandBarSuggestions))
}

func TestCommandBarNamesFetched_OlderFetchKeepsNewerInFlight(t *testing.T) {
	m := argoCommandBar(t, "kubectl get configmaps ")
	m.discoveredResources["argo"] = argoDiscoveryMsg().entries
	newer := m.commandBarNameLoading
	require.NotEmpty(t, newer)

	ret, cmd := m.updateImpl(commandBarNamesFetchedMsg{cacheKey: "argo/default/secrets", names: []string{"s-1"}})
	m = ret.(Model)

	assert.Equal(t, newer, m.commandBarNameLoading)
	assert.Nil(t, cmd, "the in-flight fetch must not start again")
}
