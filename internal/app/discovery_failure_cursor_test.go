package app

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// Sibling of TestDiscoveryAfterCursorMovePreservesCursor (which guards the
// success branch). The failure branch must apply the same wasInitial guard:
// when middleItems is already populated and a discovery retry fails (e.g.,
// kind cluster mid-teardown returning errors on every 2s watch tick), the
// user's live cursor must be preserved instead of snapped back to
// cursorMemory.
//
// Reproduction: cursorMemory[ctx] is non-zero (set by session restore or a
// prior drill-in), the user has scrolled to a different position, and a
// watch-tick re-discovery fails. Pre-fix, restoreCursor() ran
// unconditionally and snapped the cursor back. Post-fix, clampCursor()
// runs because middleItems was already populated.
func TestDiscoveryFailureAfterCursorMovePreservesCursor(t *testing.T) {
	t.Parallel()
	m := baseModelCov()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "test-ctx"
	m.allGroupsExpanded = true

	// Middle pane already populated with the seed list (prior failed
	// discovery left this state, or the user navigated in via the
	// previewItems branch of navigateChildCluster).
	m.middleItems = model.BuildSidebarItems(model.SeedResources())
	require.NotEmpty(t, m.middleItems)
	m.setCursor(5) // user scrolled here

	// Cursor memory points to a different position — set by, e.g., a prior
	// session restore that landed on Deployments. restoreCursor() would
	// snap to this index and undo the user's scroll.
	m.cursorMemory[m.navKey()] = 2

	// invalidatePreviewForCursorChange just ran from the user's j/k press.
	m.loading = true

	updated, _ := m.updateAPIResourceDiscovery(apiResourceDiscoveryMsg{
		context: "test-ctx",
		err:     errors.New("the server is currently unable to handle the request"),
	})

	assert.Equal(t, 5, updated.cursor(),
		"failure-path discovery retry must not snap cursor back to "+
			"cursorMemory when middleItems is already populated — "+
			"otherwise watch-tick failures undo the user's scroll on "+
			"every interval")
	assert.False(t, updated.loading,
		"failure handler must clear m.loading so the spinner doesn't linger")
}

// Initial-discovery failure (no items rendered yet) must still restore
// cursor from cursorMemory — that's what session restore depends on so the
// user lands on their saved resource type even when the cluster's first
// /api roundtrip fails. wasInitial=true here because middleItems is nil.
func TestDiscoveryFailureOnInitialLoadRestoresCursorFromMemory(t *testing.T) {
	t.Parallel()
	m := baseModelCov()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "test-ctx"
	m.allGroupsExpanded = true

	// Initial state: middle pane not yet populated (navigateChildCluster's
	// default branch ran with discoveredResources empty and no preview).
	m.middleItems = nil
	m.setCursor(0)

	// Session restore wrote the saved resource type's index here.
	m.cursorMemory[m.navKey()] = 4

	// m.loading=true is the original signal that the failure handler keys
	// off — and is correct on the initial discovery.
	m.loading = true

	updated, _ := m.updateAPIResourceDiscovery(apiResourceDiscoveryMsg{
		context: "test-ctx",
		err:     errors.New("connection refused"),
	})

	assert.Equal(t, 4, updated.cursor(),
		"initial-discovery failure must still restoreCursor from "+
			"cursorMemory so session restore lands on the saved resource type")
	assert.NotEmpty(t, updated.middleItems,
		"failure handler must populate middleItems with the seed list so "+
			"the user can navigate even when the cluster's discovery is broken")
}

// Production case: allGroupsExpanded is false by default. The accordion
// makes only the user's current category visible alongside collapsed
// placeholders for the others. The earlier tests run with
// allGroupsExpanded=true, which short-circuits syncExpandedGroup and
// hides whether the wasInitial guard works under realistic conditions.
//
// This test mirrors TestDiscoveryFailureAfterCursorMovePreservesCursor
// but with the production accordion active and the cursor parked on a
// real item inside the expanded group. The cursor must stay where the
// user scrolled — not snap to cursorMemory, and not get re-anchored to
// the first item of expandedGroup by syncExpandedGroup.
func TestDiscoveryFailureAfterCursorMove_AccordionEnabled_PreservesCursor(t *testing.T) {
	t.Parallel()
	m := baseModelCov()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "test-ctx"
	m.allGroupsExpanded = false
	m.expandedGroup = "Workloads"

	m.middleItems = model.BuildSidebarItems(model.SeedResources())
	require.NotEmpty(t, m.middleItems)

	// Park the cursor on a real Workloads item that's NOT the first one
	// in the group. syncExpandedGroup snaps to the first matching item
	// when expandedGroup changes — picking a non-first index makes the
	// regression visible if that path ever fires here.
	visible := m.visibleMiddleItems()
	deploymentsIdx, replicaSetsIdx := -1, -1
	for i, item := range visible {
		switch item.Kind {
		case "Deployment":
			deploymentsIdx = i
		case "ReplicaSet":
			replicaSetsIdx = i
		}
	}
	require.GreaterOrEqual(t, deploymentsIdx, 0, "seed list must include Deployments")
	require.Greater(t, replicaSetsIdx, deploymentsIdx,
		"ReplicaSet must come after Deployment for this test to exercise non-first-item preservation")
	m.setCursor(replicaSetsIdx)

	// cursorMemory points at Deployments (e.g., the last drill-in target
	// from a prior session). restoreCursor() would snap back here; the
	// fix must clamp instead.
	m.cursorMemory[m.navKey()] = deploymentsIdx

	// invalidatePreviewForCursorChange just ran from the user's j press.
	m.loading = true

	updated, _ := m.updateAPIResourceDiscovery(apiResourceDiscoveryMsg{
		context: "test-ctx",
		err:     errors.New("the server is currently unable to handle the request"),
	})

	assert.Equal(t, replicaSetsIdx, updated.cursor(),
		"with the production accordion (allGroupsExpanded=false), the failure "+
			"handler must keep the cursor where the user parked it — not snap "+
			"back to cursorMemory[ctx] (=Deployments) via restoreCursor")
	assert.Equal(t, "Workloads", updated.expandedGroup,
		"expandedGroup must remain Workloads — syncExpandedGroup runs after "+
			"clampCursor and would change it only if the cursor's category "+
			"diverged, which it didn't here")
}

// A discovery failure for a non-current context (e.g., a stale hover
// preview from the cluster picker that the user has already left) must
// not touch cursor or middleItems.
func TestDiscoveryFailureForOtherContextIsNoOp(t *testing.T) {
	t.Parallel()
	m := baseModelCov()
	m.nav.Level = model.LevelResourceTypes
	m.nav.Context = "current-ctx"
	m.allGroupsExpanded = true

	originalItems := model.BuildSidebarItems(model.SeedResources())
	m.middleItems = originalItems
	m.setCursor(5)
	m.cursorMemory[m.navKey()] = 2
	m.loading = true

	updated, _ := m.updateAPIResourceDiscovery(apiResourceDiscoveryMsg{
		context: "other-ctx", // not the user's current context
		err:     errors.New("forbidden"),
	})

	assert.Equal(t, 5, updated.cursor(),
		"failure for non-current context must not affect cursor")
	assert.True(t, updated.loading,
		"failure for non-current context must not clear m.loading "+
			"(the loader belongs to the current context's in-flight work)")
}
