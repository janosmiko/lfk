package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- ActiveMiddleLineMap: RenderColumn ---

func TestRenderColumnLineMap(t *testing.T) {
	setup := func() func() {
		origScroll := ActiveMiddleScroll
		origHighlight := ActiveHighlightQuery
		origCollapsed := ActiveCollapsedCategories
		origCounts := ActiveCategoryCounts
		origSelected := ActiveSelectedItems
		ActiveMiddleScroll = 0
		ActiveHighlightQuery = ""
		ActiveCollapsedCategories = nil
		ActiveCategoryCounts = nil
		ActiveSelectedItems = nil
		return func() {
			ActiveMiddleScroll = origScroll
			ActiveHighlightQuery = origHighlight
			ActiveCollapsedCategories = origCollapsed
			ActiveCategoryCounts = origCounts
			ActiveSelectedItems = origSelected
		}
	}

	t.Run("simple items without categories", func(t *testing.T) {
		defer setup()()

		items := []model.Item{
			{Name: "item-a"},
			{Name: "item-b"},
			{Name: "item-c"},
		}
		RenderColumn("HEADER", items, 0, 40, 20, true, false, "", "")

		// No categories: each item is one display line.
		assert.Equal(t, []int{0, 1, 2}, ActiveMiddleLineMap)
	})

	t.Run("items with categories include headers and blank separator rows", func(t *testing.T) {
		defer setup()()

		items := []model.Item{
			{Name: "Pods", Category: "Workloads"},
			{Name: "Deployments", Category: "Workloads"},
			{Name: "Services", Category: "Networking"},
		}
		RenderColumn("HEADER", items, 0, 40, 30, true, false, "", "")

		// Expected display lines:
		// 0: "Workloads" header -> -1
		// 1: Pods -> 0
		// 2: Deployments -> 1
		// 3: blank separator -> -1
		// 4: "Networking" header -> -1
		// 5: Services -> 2
		assert.Equal(t, []int{-1, 0, 1, -1, -1, 2}, ActiveMiddleLineMap)
	})

	t.Run("line map not built for inactive column", func(t *testing.T) {
		defer setup()()

		// Clear the line map first.
		ActiveMiddleLineMap = nil

		items := []model.Item{
			{Name: "item-a"},
		}
		RenderColumn("HEADER", items, 0, 40, 20, false, false, "", "")

		// Line map should not be built for inactive column.
		assert.Nil(t, ActiveMiddleLineMap)
	})

	t.Run("scrolled column maps to correct items", func(t *testing.T) {
		defer setup()()

		items := make([]model.Item, 20)
		for i := range items {
			items[i] = model.Item{Name: "item-" + string(rune('a'+i))}
		}

		// Set scroll so viewport starts at item 10.
		ActiveMiddleScroll = 10

		// Height of 5 means only ~5 items are visible (minus header).
		RenderColumn("HEADER", items, 12, 40, 6, true, false, "", "")

		// Line map should contain indices starting from the scroll position.
		assert.NotEmpty(t, ActiveMiddleLineMap)
		// First visible item should be near the scroll position (not 0).
		for _, idx := range ActiveMiddleLineMap {
			assert.GreaterOrEqual(t, idx, 0)
			assert.Less(t, idx, len(items))
		}
		assert.GreaterOrEqual(t, ActiveMiddleLineMap[0], 8,
			"first visible item should be near scroll position, not at the start")
	})
}

// --- ActiveMiddleLineMap: RenderTable ---

func TestRenderTableLineMap(t *testing.T) {
	setup := func() func() {
		origScroll := ActiveMiddleScroll
		origHighlight := ActiveHighlightQuery
		origFullscreen := ActiveFullscreenMode
		origSelected := ActiveSelectedItems
		ActiveMiddleScroll = 0
		ActiveHighlightQuery = ""
		ActiveFullscreenMode = false
		ActiveSelectedItems = nil
		return func() {
			ActiveMiddleScroll = origScroll
			ActiveHighlightQuery = origHighlight
			ActiveFullscreenMode = origFullscreen
			ActiveSelectedItems = origSelected
		}
	}

	t.Run("simple items without categories", func(t *testing.T) {
		defer setup()()

		items := []model.Item{
			{Name: "nginx", Status: "Running"},
			{Name: "redis", Status: "Running"},
			{Name: "postgres", Status: "Pending"},
		}
		RenderTable("NAME", items, 0, 80, 20, false, "", "")

		// No categories: each item is one display line.
		assert.Equal(t, []int{0, 1, 2}, ActiveMiddleLineMap)
	})

	t.Run("items with categories include headers and blank separator rows", func(t *testing.T) {
		defer setup()()

		items := []model.Item{
			{Name: "nginx", Category: "Workloads", Status: "Running"},
			{Name: "redis", Category: "Workloads", Status: "Running"},
			{Name: "my-svc", Category: "Networking", Status: "Active"},
		}
		RenderTable("NAME", items, 0, 80, 30, false, "", "")

		// Expected display lines:
		// 0: "Workloads" header -> -1
		// 1: nginx -> 0
		// 2: redis -> 1
		// 3: blank separator -> -1
		// 4: "Networking" header -> -1
		// 5: my-svc -> 2
		assert.Equal(t, []int{-1, 0, 1, -1, -1, 2}, ActiveMiddleLineMap)
	})

	t.Run("line map not built when ActiveMiddleScroll is -1", func(t *testing.T) {
		defer setup()()

		ActiveMiddleScroll = -1
		ActiveMiddleLineMap = nil

		items := []model.Item{
			{Name: "nginx", Status: "Running"},
		}
		RenderTable("NAME", items, 0, 80, 20, false, "", "")

		// Line map should not be built when scroll is -1 (right column rendering).
		assert.Nil(t, ActiveMiddleLineMap)
	})

	t.Run("scrolled table maps to correct items", func(t *testing.T) {
		defer setup()()

		items := make([]model.Item, 30)
		for i := range items {
			items[i] = model.Item{Name: "pod-" + string(rune('a'+i%26)), Status: "Running"}
		}

		// Set scroll so viewport starts near item 15.
		ActiveMiddleScroll = 15

		// Small height to force scrolling.
		RenderTable("NAME", items, 18, 80, 8, false, "", "")

		// Line map should only contain items from the visible viewport.
		assert.NotEmpty(t, ActiveMiddleLineMap)
		for _, idx := range ActiveMiddleLineMap {
			assert.GreaterOrEqual(t, idx, 0)
			assert.Less(t, idx, len(items))
		}
		// First line map entry should be near the scroll position (not 0).
		assert.GreaterOrEqual(t, ActiveMiddleLineMap[0], 10,
			"first visible item should be near scroll position, not at the start")
	})
}
