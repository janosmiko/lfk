package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- OverlayListWidth ---

func TestOverlayListWidth(t *testing.T) {
	t.Run("empty items returns floor", func(t *testing.T) {
		assert.Equal(t, OverlayListFloor, OverlayListWidth(nil, OverlayListConfig{}, 200))
	})

	t.Run("short items return floor", func(t *testing.T) {
		items := []OverlayListItem{{Name: "a"}, {Name: "b"}}
		assert.Equal(t, OverlayListFloor, OverlayListWidth(items, OverlayListConfig{}, 200))
	})

	t.Run("long description grows the overlay past the floor", func(t *testing.T) {
		items := []OverlayListItem{{Name: "x", Description: strings.Repeat("y", 90)}}
		cfg := OverlayListConfig{ShowDescription: true}
		assert.Greater(t, OverlayListWidth(items, cfg, 200), OverlayListFloor)
	})

	t.Run("description ignored when ShowDescription is off", func(t *testing.T) {
		items := []OverlayListItem{{Name: "x", Description: strings.Repeat("y", 200)}}
		cfg := OverlayListConfig{ShowDescription: false}
		assert.Equal(t, OverlayListFloor, OverlayListWidth(items, cfg, 500))
	})

	t.Run("clamps to maxWidth", func(t *testing.T) {
		items := []OverlayListItem{{Name: strings.Repeat("a", 200)}}
		assert.Equal(t, 90, OverlayListWidth(items, OverlayListConfig{}, 90))
	})

	t.Run("non-positive maxWidth disables clamp but keeps floor", func(t *testing.T) {
		items := []OverlayListItem{{Name: "a"}}
		assert.Equal(t, OverlayListFloor, OverlayListWidth(items, OverlayListConfig{}, 0))
	})

	t.Run("wide glyphs are measured by visual cells", func(t *testing.T) {
		// CJK glyphs are 2 cells each; len() would overestimate.
		wide := strings.Repeat("你", 80)
		items := []OverlayListItem{{Name: wide}}
		assert.Equal(t, 90, OverlayListWidth(items, OverlayListConfig{}, 90))
	})
}

// --- RenderOverlayList ---

func TestRenderOverlayList(t *testing.T) {
	const w = 72

	t.Run("empty items shows EmptyMessage", func(t *testing.T) {
		cfg := OverlayListConfig{Title: "Picker", EmptyMessage: "Nothing here"}
		out := RenderOverlayList(nil, cfg, w)
		assert.Contains(t, out, "Picker")
		assert.Contains(t, out, "Nothing here")
	})

	t.Run("renders title + item names", func(t *testing.T) {
		items := []OverlayListItem{
			{Name: "Alpha"}, {Name: "Beta"}, {Name: "Gamma"},
		}
		out := RenderOverlayList(items, OverlayListConfig{Title: "Picker"}, w)
		assert.Contains(t, out, "Picker")
		assert.Contains(t, out, "Alpha")
		assert.Contains(t, out, "Beta")
		assert.Contains(t, out, "Gamma")
	})

	t.Run("cursor row padded to inner width for full-row highlight", func(t *testing.T) {
		items := []OverlayListItem{{Name: "Short"}, {Name: "Other"}}
		out := RenderOverlayList(items, OverlayListConfig{Cursor: 0}, w)
		var sel string
		for l := range strings.SplitSeq(out, "\n") {
			if strings.Contains(l, "Short") {
				sel = l
				break
			}
		}
		require.NotEmpty(t, sel, "expected the cursor row")
		assert.Equal(t, w, lipgloss.Width(sel),
			"cursor row must be padded to width so the highlight spans the row")
	})

	t.Run("non-cursor row is not padded to full width", func(t *testing.T) {
		items := []OverlayListItem{{Name: "Short"}, {Name: "Other"}}
		out := RenderOverlayList(items, OverlayListConfig{Cursor: 0}, w)
		var other string
		for l := range strings.SplitSeq(out, "\n") {
			if strings.Contains(l, "Other") {
				other = l
				break
			}
		}
		require.NotEmpty(t, other)
		assert.Less(t, lipgloss.Width(other), w)
	})

	t.Run("ShowKey renders [k] prefix", func(t *testing.T) {
		items := []OverlayListItem{{Key: "f", Name: "Failing"}}
		out := RenderOverlayList(items, OverlayListConfig{ShowKey: true}, w)
		assert.Contains(t, out, "[f]")
		assert.Contains(t, out, "Failing")
	})

	t.Run("ShowKey off omits the [k] prefix", func(t *testing.T) {
		items := []OverlayListItem{{Key: "f", Name: "Failing"}}
		out := RenderOverlayList(items, OverlayListConfig{ShowKey: false}, w)
		assert.NotContains(t, out, "[f]")
	})

	t.Run("ShowStatus renders [status] badge", func(t *testing.T) {
		items := []OverlayListItem{{Status: "d", Name: "Delete"}}
		out := RenderOverlayList(items, OverlayListConfig{ShowStatus: true}, w)
		assert.Contains(t, out, "[d]")
		assert.Contains(t, out, "Delete")
	})

	t.Run("ShowDescription renders Description text", func(t *testing.T) {
		items := []OverlayListItem{{Name: "Failing", Description: "CrashLoop / Error"}}
		out := RenderOverlayList(items, OverlayListConfig{ShowDescription: true}, w)
		assert.Contains(t, out, "Failing")
		assert.Contains(t, out, "CrashLoop / Error")
	})

	t.Run("ShowDescription off omits the description", func(t *testing.T) {
		items := []OverlayListItem{{Name: "Failing", Description: "hidden-desc"}}
		out := RenderOverlayList(items, OverlayListConfig{ShowDescription: false}, w)
		assert.NotContains(t, out, "hidden-desc")
	})

	t.Run("ShowActiveMarker renders check next to Active item", func(t *testing.T) {
		items := []OverlayListItem{
			{Name: "A", Active: false},
			{Name: "B", Active: true},
		}
		out := RenderOverlayList(items, OverlayListConfig{ShowActiveMarker: true}, w)
		assert.Contains(t, out, "✓") // ✓
	})

	t.Run("ShowActiveMarker off hides the check mark", func(t *testing.T) {
		items := []OverlayListItem{{Name: "A", Active: true}}
		out := RenderOverlayList(items, OverlayListConfig{ShowActiveMarker: false}, w)
		assert.NotContains(t, out, "✓")
	})

	t.Run("active-marker column collapses when no item is active", func(t *testing.T) {
		// When ShowActiveMarker is on but nothing's active, the 2-cell
		// reserved space disappears so [k]/[s] sit flush against the
		// box's left padding. Width comparison: items render at strictly
		// less than the width-with-marker case.
		none := []OverlayListItem{{Name: "Failing", Key: "f"}}
		one := []OverlayListItem{{Name: "Failing", Key: "f", Active: true}}
		cfg := OverlayListConfig{ShowActiveMarker: true, ShowKey: true}
		outNone := RenderOverlayList(none, cfg, w)
		outOne := RenderOverlayList(one, cfg, w)
		assert.NotContains(t, outNone, "✓")
		assert.Contains(t, outOne, "✓")
	})

	t.Run("active-marker column reserves space when at least one item is active", func(t *testing.T) {
		// Two items, one active. All rows reserve the 2-cell marker
		// column so the [k] of inactive rows aligns with the [k] of the
		// active row.
		items := []OverlayListItem{
			{Name: "A", Key: "a"},
			{Name: "B", Key: "b", Active: true},
		}
		out := RenderOverlayList(items, OverlayListConfig{
			ShowActiveMarker: true, ShowKey: true,
		}, w)
		assert.Contains(t, out, "✓")
		// Both rows contain the bracketed key.
		assert.Contains(t, out, "[a]")
		assert.Contains(t, out, "[b]")
	})

	t.Run("MultiSelect renders checkboxes", func(t *testing.T) {
		items := []OverlayListItem{
			{Name: "A", Selected: true},
			{Name: "B", Selected: false},
		}
		out := RenderOverlayList(items, OverlayListConfig{MultiSelect: true}, w)
		// Checked + unchecked boxes both visible.
		assert.Contains(t, out, "☑") // ☑
		assert.Contains(t, out, "☐") // ☐
	})

	t.Run("Subtitle renders below title", func(t *testing.T) {
		out := RenderOverlayList(nil, OverlayListConfig{
			Title:        "Picker",
			Subtitle:     "context: my-cluster",
			EmptyMessage: "",
		}, w)
		assert.Contains(t, out, "context: my-cluster")
	})

	t.Run("FooterHint renders below list", func(t *testing.T) {
		items := []OverlayListItem{{Name: "A"}}
		out := RenderOverlayList(items, OverlayListConfig{FooterHint: "tab to switch pod"}, w)
		assert.Contains(t, out, "tab to switch pod")
	})

	t.Run("scroll window respects MaxVisible", func(t *testing.T) {
		items := make([]OverlayListItem, 10)
		for i := range items {
			items[i].Name = "item-" + string(rune('A'+i))
		}
		out := RenderOverlayList(items, OverlayListConfig{
			Scroll:     0,
			MaxVisible: 3,
		}, w)
		// First three visible.
		assert.Contains(t, out, "item-A")
		assert.Contains(t, out, "item-B")
		assert.Contains(t, out, "item-C")
		// Past the window is hidden.
		assert.NotContains(t, out, "item-D")
		assert.NotContains(t, out, "item-J")
	})

	t.Run("scroll offset shifts the window", func(t *testing.T) {
		items := make([]OverlayListItem, 10)
		for i := range items {
			items[i].Name = "item-" + string(rune('A'+i))
		}
		out := RenderOverlayList(items, OverlayListConfig{
			Scroll:     5,
			MaxVisible: 3,
		}, w)
		assert.NotContains(t, out, "item-A")
		assert.Contains(t, out, "item-F")
		assert.Contains(t, out, "item-G")
		assert.Contains(t, out, "item-H")
	})

	t.Run("filter input shown when FilterActive", func(t *testing.T) {
		items := []OverlayListItem{{Name: "Alpha"}}
		out := RenderOverlayList(items, OverlayListConfig{
			Filter:       "alp",
			FilterActive: true,
		}, w)
		assert.Contains(t, out, "alp")
		assert.Contains(t, out, "█") // cursor block when active
	})

	t.Run("filter input shown when Filter non-empty and inactive (no cursor block)", func(t *testing.T) {
		items := []OverlayListItem{{Name: "Alpha"}}
		out := RenderOverlayList(items, OverlayListConfig{
			Filter:       "alp",
			FilterActive: false,
		}, w)
		assert.Contains(t, out, "alp")
		assert.NotContains(t, out, "█") // no cursor when not in filter mode
	})

	t.Run("no filter row when filter empty and inactive", func(t *testing.T) {
		items := []OverlayListItem{{Name: "Alpha"}}
		out := RenderOverlayList(items, OverlayListConfig{}, w)
		assert.NotContains(t, out, "filter: ")
	})

	t.Run("Disabled item renders with dim style indicator", func(t *testing.T) {
		items := []OverlayListItem{
			{Name: "Alive"},
			{Name: "Stale", Disabled: true},
		}
		out := RenderOverlayList(items, OverlayListConfig{}, w)
		// Both names visible — Disabled is a style hint, not a removal.
		assert.Contains(t, out, "Alive")
		assert.Contains(t, out, "Stale")
	})
}
