package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// TestRenderColumnLastItemReachableAtAnyHeight is a regression guard
// for the compound scroll bug user-reported on a 245-item / 30-
// category list: when decrementing startEntry past an earlier entry
// with 2-3 display lines (category header + blank separator + item),
// (1) the "don't leave empty space" loop shifted startEntry too far
// back so displayLines > height and the end-loop dropped tail items,
// and (2) the render loop flipped `first=false` in the elided-sep
// branch, adding a phantom leading blank row that pushed the last
// entry out of the visible area. The net effect: the final 1-2
// resource types were unreachable by scrolling with `j`/`G`.
//
// The fix is in two places: VimScrollOff (and the fallback inline
// scroll) now check the resulting displayLines BEFORE decrementing,
// and the render loop no longer flips `first` when it elides the
// sep for the viewport's first visible entry.
func TestRenderColumnLastItemReachableAtAnyHeight(t *testing.T) {
	// 50 items in 10 categories (5 per cat) = 69 total display
	// lines with separators. Heights below 69 must still render
	// the last item when the cursor is on it.
	const numCats = 10
	const itemsPerCat = 5
	items := make([]model.Item, 0, numCats*itemsPerCat)
	for c := range numCats {
		catName := fmt.Sprintf("Cat%02d", c)
		for i := range itemsPerCat {
			items = append(items, model.Item{
				Name:     fmt.Sprintf("c%02d-item%d", c, i),
				Category: catName,
			})
		}
	}

	lastIdx := len(items) - 1
	lastName := items[lastIdx].Name

	for _, height := range []int{11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 30, 40, 50, 60, 68, 69} {
		t.Run(fmt.Sprintf("h=%d", height), func(t *testing.T) {
			ActiveMiddleScroll = 0
			defer func() { ActiveMiddleScroll = -1 }()

			out := RenderColumn("", items, lastIdx, 40, height, true, false, "", "")
			lines := strings.Split(out, "\n")
			assert.LessOrEqual(t, len(lines), height,
				"rendered %d lines must fit in height=%d", len(lines), height)
			assert.Contains(t, out, lastName,
				"cursor=%d (%s) must be visible at height=%d", lastIdx, lastName, height)
		})
	}
}

// TestRenderColumnCategoriesSeparatorsAndScroll verifies the blank
// separator rows between groups and that scrolling reveals the list
// tail. With 8 items in 3 categories: 3 headers + 2 seps + 8 items
// = 13 rows. Height = 11 cannot fit everything, so scroll is
// required to see the last items — but cursor-at-last MUST shift
// the viewport so the last item is visible.
func TestRenderColumnCategoriesSeparatorsAndScroll(t *testing.T) {
	items := []model.Item{
		{Name: "a", Category: "Cat1"},
		{Name: "b", Category: "Cat1"},
		{Name: "c", Category: "Cat1"},
		{Name: "d", Category: "Cat2"},
		{Name: "e", Category: "Cat2"},
		{Name: "f", Category: "Cat2"},
		{Name: "g", Category: "Cat3"},
		{Name: "h", Category: "Cat3"},
	}

	t.Run("cursor at top shows leading items and first separator", func(t *testing.T) {
		ActiveMiddleScroll = 0
		defer func() { ActiveMiddleScroll = -1 }()

		out := RenderColumn("", items, 0, 30, 11, true, false, "", "")
		lines := strings.Split(out, "\n")
		assert.LessOrEqual(t, len(lines), 11)

		// First items must be visible.
		assert.Contains(t, out, "a")
		assert.Contains(t, out, "b")
		assert.Contains(t, out, "c")

		// At least one blank row between groups must be present.
		blanks := 0
		for _, l := range lines {
			if strings.TrimSpace(l) == "" {
				blanks++
			}
		}
		assert.GreaterOrEqual(t, blanks, 1, "blank separator row between groups")
	})

	t.Run("cursor at last item scrolls viewport to show it", func(t *testing.T) {
		ActiveMiddleScroll = 0
		defer func() { ActiveMiddleScroll = -1 }()

		out := RenderColumn("", items, 7, 30, 11, true, false, "", "")
		lines := strings.Split(out, "\n")
		assert.LessOrEqual(t, len(lines), 11)

		// The cursor's item MUST be visible — this is the key
		// assertion. Without a correct scroll, 'h' would be clipped
		// and the user would have no way to reach it.
		assert.Contains(t, out, "h", "cursor-at-last must be visible after scroll")
		assert.Contains(t, out, "g")
	})
}

// TestRenderColumnHeaderDoesNotWrap is a regression guard for the
// overflow that cut off the last row of the explorer middle column
// when only one tab was open. The left column uses a 10-char header
// like "KUBECONFIG" but its inner width is 8 (leftW=10 minus 1 cell
// of padding on each side). Without truncation, lipgloss wraps the
// header onto a second line, making the left column 1 row taller
// than the middle/right columns — which pushes the overall view 1
// row past m.height and the terminal clips the bottom row.
//
// Fix: RenderColumn truncates the header to `width` so it always
// occupies exactly 1 line and the column-height math stays correct.
func TestRenderColumnHeaderDoesNotWrap(t *testing.T) {
	items := []model.Item{
		{Name: "row-1"},
		{Name: "row-2"},
		{Name: "row-3"},
	}
	// header "KUBECONFIG" is 10 chars; width=8 forces truncation.
	result := RenderColumn("KUBECONFIG", items, 0, 8, 10, true, false, "", "")
	// The result must contain exactly one header line followed by the
	// items. Specifically: splitting on "\n" should yield 4 lines — one
	// header (truncated to width) and three rows.
	lines := strings.Split(result, "\n")
	assert.Len(t, lines, 4, "one header line + three item lines, no wrap")
	// First line must contain the truncated header (runes up to width).
	assert.Contains(t, lines[0], "KUBECON", "header truncated to fit width")
	assert.NotContains(t, lines[0], "KUBECONFIG", "full header must not fit in width=8")
}

// --- RenderColumn ---

func TestRenderColumn(t *testing.T) {
	t.Run("empty items with loading shows spinner", func(t *testing.T) {
		result := RenderColumn("Header", nil, 0, 40, 10, true, true, ">", "")
		assert.Contains(t, result, "Header")
		assert.Contains(t, result, "Loading...")
	})

	t.Run("empty items with error shows error", func(t *testing.T) {
		result := RenderColumn("Header", nil, 0, 40, 10, true, false, "", "connection refused")
		assert.Contains(t, result, "Header")
		assert.Contains(t, result, "connection refused")
	})

	t.Run("empty items no loading no error shows no items", func(t *testing.T) {
		result := RenderColumn("Header", nil, 0, 40, 10, true, false, "", "")
		assert.Contains(t, result, "Header")
		assert.Contains(t, result, "No items")
	})

	t.Run("empty header skips header line", func(t *testing.T) {
		items := []model.Item{{Name: "pod1", Status: "Running"}}
		result := RenderColumn("", items, 0, 40, 10, true, false, "", "")
		assert.Contains(t, result, "pod1")
	})

	t.Run("items rendered with names", func(t *testing.T) {
		items := []model.Item{
			{Name: "pod-1", Status: "Running"},
			{Name: "pod-2", Status: "Pending"},
		}
		// Reset global state that might interfere.
		origQuery := ActiveHighlightQuery
		ActiveHighlightQuery = ""
		defer func() { ActiveHighlightQuery = origQuery }()

		origCollapsed := ActiveCollapsedCategories
		ActiveCollapsedCategories = nil
		defer func() { ActiveCollapsedCategories = origCollapsed }()

		// Use inactive scroll to avoid relying on ActiveMiddleScroll.
		origMS := ActiveMiddleScroll
		ActiveMiddleScroll = -1
		origLS := ActiveLeftScroll
		ActiveLeftScroll = -1
		defer func() {
			ActiveMiddleScroll = origMS
			ActiveLeftScroll = origLS
		}()

		result := RenderColumn("Pods", items, 0, 60, 10, true, false, "", "")
		assert.Contains(t, result, "pod-1")
		assert.Contains(t, result, "pod-2")
		assert.Contains(t, result, "Running")
	})

	t.Run("cursor on item selects it", func(t *testing.T) {
		items := []model.Item{
			{Name: "item-a"},
			{Name: "item-b"},
		}
		origQuery := ActiveHighlightQuery
		ActiveHighlightQuery = ""
		defer func() { ActiveHighlightQuery = origQuery }()

		origCollapsed := ActiveCollapsedCategories
		ActiveCollapsedCategories = nil
		defer func() { ActiveCollapsedCategories = origCollapsed }()

		origMS := ActiveMiddleScroll
		ActiveMiddleScroll = -1
		origLS := ActiveLeftScroll
		ActiveLeftScroll = -1
		defer func() {
			ActiveMiddleScroll = origMS
			ActiveLeftScroll = origLS
		}()

		result := RenderColumn("Col", items, 1, 60, 10, true, false, "", "")
		assert.Contains(t, result, "item-b")
	})

	t.Run("inactive column renders", func(t *testing.T) {
		items := []model.Item{{Name: "svc-1"}}
		origCollapsed := ActiveCollapsedCategories
		ActiveCollapsedCategories = nil
		defer func() { ActiveCollapsedCategories = origCollapsed }()

		origMS := ActiveMiddleScroll
		ActiveMiddleScroll = -1
		origLS := ActiveLeftScroll
		ActiveLeftScroll = -1
		defer func() {
			ActiveMiddleScroll = origMS
			ActiveLeftScroll = origLS
		}()

		result := RenderColumn("Services", items, 0, 40, 10, false, false, "", "")
		assert.Contains(t, result, "svc-1")
	})

	t.Run("items with categories show category headers", func(t *testing.T) {
		items := []model.Item{
			{Name: "pod-1", Category: "Workloads"},
			{Name: "svc-1", Category: "Networking"},
		}
		origCollapsed := ActiveCollapsedCategories
		ActiveCollapsedCategories = nil
		defer func() { ActiveCollapsedCategories = origCollapsed }()

		origMS := ActiveMiddleScroll
		ActiveMiddleScroll = -1
		origLS := ActiveLeftScroll
		ActiveLeftScroll = -1
		defer func() {
			ActiveMiddleScroll = origMS
			ActiveLeftScroll = origLS
		}()

		result := RenderColumn("", items, 0, 60, 20, true, false, "", "")
		assert.Contains(t, result, "Workloads")
		assert.Contains(t, result, "Networking")
	})
}

// --- FormatItem ---

func TestFormatItem(t *testing.T) {
	t.Run("simple name no extras", func(t *testing.T) {
		item := model.Item{Name: "my-pod"}
		origQuery := ActiveHighlightQuery
		ActiveHighlightQuery = ""
		defer func() { ActiveHighlightQuery = origQuery }()

		result := FormatItem(item, 40)
		assert.Contains(t, result, "my-pod")
	})

	t.Run("item with namespace", func(t *testing.T) {
		item := model.Item{Name: "my-pod", Namespace: "default"}
		origQuery := ActiveHighlightQuery
		ActiveHighlightQuery = ""
		defer func() { ActiveHighlightQuery = origQuery }()

		result := FormatItem(item, 60)
		assert.Contains(t, result, "default/my-pod")
	})

	t.Run("item with status", func(t *testing.T) {
		item := model.Item{Name: "pod", Status: "Running"}
		origQuery := ActiveHighlightQuery
		ActiveHighlightQuery = ""
		defer func() { ActiveHighlightQuery = origQuery }()

		result := FormatItem(item, 40)
		assert.Contains(t, result, "pod")
		assert.Contains(t, result, "Running")
	})

	t.Run("item with ready and age", func(t *testing.T) {
		item := model.Item{Name: "pod", Ready: "1/1", Age: "5m", Status: "Running"}
		origQuery := ActiveHighlightQuery
		ActiveHighlightQuery = ""
		defer func() { ActiveHighlightQuery = origQuery }()

		result := FormatItem(item, 60)
		assert.Contains(t, result, "pod")
		assert.Contains(t, result, "1/1")
		assert.Contains(t, result, "5m")
	})

	t.Run("current context shows star", func(t *testing.T) {
		item := model.Item{Name: "my-context", Status: "current"}
		origQuery := ActiveHighlightQuery
		ActiveHighlightQuery = ""
		defer func() { ActiveHighlightQuery = origQuery }()

		result := FormatItem(item, 40)
		assert.Contains(t, result, "*")
		assert.Contains(t, result, "my-context")
	})

	t.Run("deprecated item shows warning", func(t *testing.T) {
		item := model.Item{Name: "old-resource", Deprecated: true}
		origQuery := ActiveHighlightQuery
		ActiveHighlightQuery = ""
		defer func() { ActiveHighlightQuery = origQuery }()

		result := FormatItem(item, 40)
		assert.Contains(t, result, "old-resource")
	})

	t.Run("item with icon", func(t *testing.T) {
		origMode := IconMode
		IconMode = "unicode"
		defer func() { IconMode = origMode }()

		origQuery := ActiveHighlightQuery
		ActiveHighlightQuery = ""
		defer func() { ActiveHighlightQuery = origQuery }()

		item := model.Item{Name: "pod", Icon: model.Icon{Unicode: "⬤"}}
		result := FormatItem(item, 40)
		assert.Contains(t, result, "pod")
	})

	t.Run("read-only context shows [RO] marker", func(t *testing.T) {
		// Regression test: a read-only context row must render the [RO]
		// marker in FormatItem (the non-cursor path), not just in
		// FormatItemPlain. Without this, j/k cursor moves would drop the
		// marker on the row that just lost focus.
		item := model.Item{Name: "prod", ReadOnly: true}
		result := FormatItem(item, 40)
		assert.Contains(t, result, "prod")
		assert.Contains(t, result, "[RO]")
	})

	t.Run("current read-only context shows star and [RO]", func(t *testing.T) {
		item := model.Item{Name: "prod", Status: "current", ReadOnly: true}
		result := FormatItem(item, 40)
		assert.Contains(t, result, "*")
		assert.Contains(t, result, "prod")
		assert.Contains(t, result, "[RO]")
	})
}

// --- FormatItemPlain ---

func TestFormatItemPlain(t *testing.T) {
	t.Run("simple name no extras", func(t *testing.T) {
		item := model.Item{Name: "my-pod"}
		result := FormatItemPlain(item, 40)
		assert.Contains(t, result, "my-pod")
	})

	t.Run("item with namespace", func(t *testing.T) {
		item := model.Item{Name: "pod", Namespace: "kube-system"}
		result := FormatItemPlain(item, 60)
		assert.Contains(t, result, "kube-system/pod")
	})

	t.Run("item with status and details", func(t *testing.T) {
		item := model.Item{Name: "pod", Ready: "2/3", Restarts: "5", Age: "10m", Status: "Running"}
		result := FormatItemPlain(item, 60)
		assert.Contains(t, result, "pod")
		assert.Contains(t, result, "2/3")
		assert.Contains(t, result, "5")
		assert.Contains(t, result, "10m")
		assert.Contains(t, result, "Running")
	})

	t.Run("current context shows star", func(t *testing.T) {
		item := model.Item{Name: "ctx", Status: "current"}
		result := FormatItemPlain(item, 40)
		assert.Contains(t, result, "* ")
		assert.Contains(t, result, "ctx")
	})

	t.Run("deprecated item shows warning", func(t *testing.T) {
		item := model.Item{Name: "res", Deprecated: true}
		result := FormatItemPlain(item, 40)
		assert.Contains(t, result, "res")
	})

	t.Run("long name truncated", func(t *testing.T) {
		item := model.Item{Name: "a-very-long-pod-name-that-exceeds-max-width", Status: "Running"}
		result := FormatItemPlain(item, 30)
		assert.LessOrEqual(t, len(result), 40) // Rough check: plain text should be bounded.
		assert.Contains(t, result, "Running")
	})

	t.Run("icon in plain mode", func(t *testing.T) {
		origMode := IconMode
		IconMode = "unicode"
		defer func() { IconMode = origMode }()

		item := model.Item{Name: "pod", Icon: model.Icon{Unicode: "⬤"}}
		result := FormatItemPlain(item, 40)
		assert.Contains(t, result, "pod")
		// In plain mode, icon is plain text (no styled IconStyle).
		assert.Contains(t, result, "⬤")
	})
}
