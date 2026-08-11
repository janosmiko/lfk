package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// TestRenderObjectExplorerTreeView_SanitizesHostileLabelsAndValues guards
// the middle-column tree rows: labels come from object field keys (label
// and annotation keys) and f.Preview from field values, both cluster-
// controlled. A hostile CSI/OSC-52/bidi payload in either must not reach
// the rendered tree, whether the row is the cursor row or not.
func TestRenderObjectExplorerTreeView_SanitizesHostileLabelsAndValues(t *testing.T) {
	cases := []struct {
		name    string
		key     string
		val     string
		mustNot []string // exact hostile byte sequences that must not survive
	}{
		{"bidi override", "annotation\u202ekey", "value\u202ehere", []string{"\u202e"}},
		{"raw csi", "annotation\x1b[31mkey", "value\x1b[2Jhere", []string{"\x1b[31m", "\x1b[2J"}},
		{"osc52 clipboard write", "annotation\x1b]52;c;aGF4\x07key", "value\x1b]52;c;aGF4\x07here", []string{"\x1b]52;c;aGF4\x07"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows := []model.ObjectTreeRow{
				{Field: model.ObjectField{Key: tc.key, Type: "<string>", Preview: tc.val}, Segs: []string{tc.key}, Depth: 0},
			}
			for _, cursor := range []int{-1, 0} {
				out := RenderObjectExplorerTreeView(
					rows, false, cursor, 0, "Object Explorer: x", nil, 0,
					"", 0, "", "hint", 160, 30,
				)
				for _, hostile := range tc.mustNot {
					assert.NotContains(t, out, hostile, "cursor=%d", cursor)
				}
			}
		})
	}
}

// TestRenderObjectExplorerTreeView_SanitizesFilteredPath covers the
// filtered-mode label path (model.FormatObjectPath), which bypasses the
// plain-key branch of objectTreeLabels.
func TestRenderObjectExplorerTreeView_SanitizesFilteredPath(t *testing.T) {
	rows := []model.ObjectTreeRow{
		{Field: model.ObjectField{Key: "key", Type: "<string>", Preview: "val"}, Segs: []string{"spec\x1b[31m", "annot\x1b]52;c;aGF4\x07ation\u202e"}, Depth: 1},
	}
	out := RenderObjectExplorerTreeView(
		rows, true, 0, 0, "Object Explorer: x", nil, 0,
		"", 0, "annot", "hint", 160, 30,
	)
	assert.NotContains(t, out, "\x1b[31m")
	assert.NotContains(t, out, "\x1b]52;c;aGF4\x07")
	assert.NotContains(t, out, "\u202e")
}
