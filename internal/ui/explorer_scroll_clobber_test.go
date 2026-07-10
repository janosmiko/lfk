package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// Issue #524 (class of #398): a cursor-less render (cursor < 0) is always a
// preview/measure render — the right pane's children table, a context list, a
// measurement pass. Those renders must never write the persistent middle/left
// pane scroll positions or the middle-pane click/sort layout globals:
// VimScrollOff with cursor=-1 returns 0, so an unguarded write resets the
// neighbouring pane's viewport and it visibly jumps. The #399 fix parked the
// globals around the then-known call sites; these tests pin the invariant in
// the renderers themselves so no future unguarded caller can clobber state.

// saveRenderGlobals pins the pane-scroll and layout globals to known values
// and restores the originals on cleanup.
func saveRenderGlobals(t *testing.T) {
	t.Helper()
	origMiddle, origLeft := ActiveMiddleScroll, ActiveLeftScroll
	origLineMap := ActiveMiddleLineMap
	origLayout := ActiveMiddleColumnLayout
	origSortable := ActiveSortableColumns
	origSortableCount := ActiveSortableColumnCount
	origExtraKeys := ActiveExtraColumnKeys
	t.Cleanup(func() {
		ActiveMiddleScroll, ActiveLeftScroll = origMiddle, origLeft
		ActiveMiddleLineMap = origLineMap
		ActiveMiddleColumnLayout = origLayout
		ActiveSortableColumns = origSortable
		ActiveSortableColumnCount = origSortableCount
		ActiveExtraColumnKeys = origExtraKeys
	})
}

func clobberTestItems() []model.Item {
	items := make([]model.Item, 8)
	for i := range items {
		items[i] = model.Item{Name: "item-" + string(rune('a'+i)), Kind: "Pod", Namespace: "default", Status: "Running"}
	}
	return items
}

func TestRenderTableCursorlessKeepsGlobals(t *testing.T) {
	saveRenderGlobals(t)
	ActiveMiddleScroll, ActiveLeftScroll = 7, 4
	ActiveMiddleLineMap = []int{0, 1, 2}
	ActiveMiddleColumnLayout = []MiddleColumnRegion{{Key: "Name", StartX: 0, EndX: 20}}
	ActiveSortableColumns = []string{"Name", "Age"}
	ActiveSortableColumnCount = 2
	ActiveExtraColumnKeys = []string{"IP"}

	// Cursor-less render, as used for the right-pane children/preview tables.
	RenderTable("POD", clobberTestItems(), -1, 60, 5, false, "", "")

	assert.Equal(t, 7, ActiveMiddleScroll, "cursor-less render must not clobber the middle pane scroll")
	assert.Equal(t, 4, ActiveLeftScroll, "cursor-less render must not clobber the left pane scroll")
	assert.Equal(t, []int{0, 1, 2}, ActiveMiddleLineMap, "cursor-less render must not rebuild the middle click map")
	assert.Equal(t, []MiddleColumnRegion{{Key: "Name", StartX: 0, EndX: 20}}, ActiveMiddleColumnLayout, "cursor-less render must not rebuild the middle column layout")
	assert.Equal(t, []string{"Name", "Age"}, ActiveSortableColumns, "cursor-less render must not rebuild the sortable columns")
	assert.Equal(t, 2, ActiveSortableColumnCount)
	assert.Equal(t, []string{"IP"}, ActiveExtraColumnKeys, "cursor-less render must not rebuild the extra column keys")
}

func TestRenderTableWithCursorStillPersistsScroll(t *testing.T) {
	saveRenderGlobals(t)
	ActiveMiddleScroll, ActiveLeftScroll = 0, 4

	// Real middle-column render: 8 items, viewport of 3 rows, cursor at the
	// bottom — the persistent scroll must follow the cursor as before.
	RenderTable("POD", clobberTestItems(), 7, 60, 4, false, "", "")

	assert.Positive(t, ActiveMiddleScroll, "cursor-ful render must keep maintaining the persistent scroll")
	assert.Equal(t, 4, ActiveLeftScroll)
}

func TestRenderColumnCursorlessKeepsGlobals(t *testing.T) {
	saveRenderGlobals(t)

	t.Run("inactive column (left/preview) with cursor=-1", func(t *testing.T) {
		ActiveMiddleScroll, ActiveLeftScroll = 7, 4
		ActiveMiddleLineMap = []int{0, 1, 2}

		// Right-pane context/resource-type list preview renders with
		// isActive=false and cursor=-1 (renderRightClusters), and the left
		// pane renders with cursor=-1 whenever parentIndex has no match.
		RenderColumn("CONTEXT", clobberTestItems(), -1, 40, 5, false, false, "", "")

		assert.Equal(t, 7, ActiveMiddleScroll, "cursor-less render must not clobber the middle pane scroll")
		assert.Equal(t, 4, ActiveLeftScroll, "cursor-less render must not clobber the left pane scroll")
		assert.Equal(t, []int{0, 1, 2}, ActiveMiddleLineMap, "inactive render must not rebuild the middle click map")
	})

	t.Run("active column with cursor=-1", func(t *testing.T) {
		ActiveMiddleScroll, ActiveLeftScroll = 7, 4
		ActiveMiddleLineMap = []int{0, 1, 2}

		RenderColumn("HEADER", clobberTestItems(), -1, 40, 5, true, false, "", "")

		assert.Equal(t, 7, ActiveMiddleScroll, "cursor-less active render must not clobber the middle pane scroll")
		assert.Equal(t, []int{0, 1, 2}, ActiveMiddleLineMap, "cursor-less active render must not rebuild the middle click map")
	})
}

func TestRenderColumnWithCursorStillPersistsScroll(t *testing.T) {
	saveRenderGlobals(t)
	ActiveMiddleScroll, ActiveLeftScroll = 0, 0

	// Active middle column: cursor at the bottom of a list taller than the
	// viewport — the persistent scroll must follow the cursor as before.
	RenderColumn("HEADER", clobberTestItems(), 7, 40, 3, true, false, "", "")
	assert.Positive(t, ActiveMiddleScroll, "cursor-ful active render must keep maintaining the persistent scroll")

	// Inactive left column with a valid parent highlight keeps its viewport.
	ActiveLeftScroll = 0
	RenderColumn("PARENT", clobberTestItems(), 7, 40, 3, false, false, "", "")
	assert.Positive(t, ActiveLeftScroll, "cursor-ful inactive render must keep maintaining the persistent scroll")
}
