package app

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// Entering a context fires discovery, and the user can drill into a resource
// type before the apiserver answers. A failure arriving then must not paint the
// seed resource types over the resource list, nor cache them as "test-ctx/pods".
func TestDiscoveryFailureAtResourceLevelLeavesMiddleColumnAlone(t *testing.T) {
	t.Parallel()

	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.nav.Context = "test-ctx"
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods"}
	m.allGroupsExpanded = true

	pods := []model.Item{
		{Name: "api-0", Kind: "Pod"},
		{Name: "api-1", Kind: "Pod"},
		{Name: "api-2", Kind: "Pod"},
	}
	m.setMiddleItems(pods)
	m.setCursor(2)
	// The pod list is still being fetched, so the loader is up.
	m.loading = true

	updated, _ := m.updateAPIResourceDiscovery(apiResourceDiscoveryMsg{
		context: "test-ctx",
		err:     errors.New("the server is currently unable to handle the request"),
	})

	assert.Equal(t, pods, updated.middleItems,
		"a discovery failure while the user is at LevelResources must not "+
			"replace the resource list with the resource-type seed list")
	assert.Equal(t, 2, updated.cursor(),
		"the cursor belongs to the resource list and must survive an "+
			"unrelated discovery failure")
	_, cached := updated.itemCache[updated.navKey()]
	assert.False(t, cached,
		"the fallback must not write resource-type items into the item "+
			"cache under the resource-level navKey (test-ctx/pods)")

	view := stripANSI(updated.View().Content)
	assert.Contains(t, view, "api-0")
	assert.Contains(t, view, "api-1")
	assert.Contains(t, view, "api-2")
	assert.NotContains(t, view, "Deployments",
		"a discovery failure at LevelResources must not paint the resource-type "+
			"sidebar into the middle column")
}

// The fallback belongs to LevelResourceTypes: there it must still paint the
// seed list and cache it under the context key.
func TestDiscoveryFailureAtResourceTypesLevelSeedsAndCachesUnderContext(t *testing.T) {
	t.Parallel()

	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "test-ctx"
	m.allGroupsExpanded = true
	m.setMiddleItems(nil)
	m.loading = true

	updated, _ := m.updateAPIResourceDiscovery(apiResourceDiscoveryMsg{
		context: "test-ctx",
		err:     errors.New("connection refused"),
	})

	require.NotEmpty(t, updated.middleItems,
		"the seed list must render so the user can navigate a cluster whose "+
			"discovery is broken")
	assert.Equal(t, updated.middleItems, updated.itemCache["test-ctx"],
		"the seed list must be cached under the resource-types key")
	assert.False(t, updated.loading)

	view := stripANSI(updated.View().Content)
	assert.Contains(t, view, "Deployments")
	assert.Contains(t, view, "ConfigMaps")
}
