package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fieldDocLines(t *testing.T, s string) []string {
	t.Helper()
	return strings.Split(stripANSI(s), "\n")
}

func TestRenderFieldDocPaneHeight(t *testing.T) {
	got := RenderFieldDocPane(60, FieldDocPane{
		Path: "spec.dnsPolicy", FieldType: "<string>", Desc: "Set DNS policy for the pod.",
	})

	assert.Len(t, fieldDocLines(t, got), FieldDocPaneHeight,
		"the pane must always claim the same height so the layout does not jump")
}

func TestRenderFieldDocPaneShowsPathAndType(t *testing.T) {
	got := stripANSI(RenderFieldDocPane(60, FieldDocPane{
		Path: "spec.dnsPolicy", FieldType: "<string>", Desc: "Set DNS policy for the pod.",
	}))

	assert.Contains(t, got, "spec.dnsPolicy")
	assert.Contains(t, got, "<string>")
	assert.Contains(t, got, "Set DNS policy for the pod.")
}

func TestRenderFieldDocPaneWrapsLongText(t *testing.T) {
	long := strings.Repeat("word ", 200)

	got := RenderFieldDocPane(40, FieldDocPane{Path: "spec", Desc: long})

	lines := fieldDocLines(t, got)
	require.Len(t, lines, FieldDocPaneHeight)
	for i, l := range lines {
		assert.LessOrEqual(t, len([]rune(l)), 40, "line %d must stay inside the width", i)
	}
}

// A field the schema documents nowhere must say so. A blank pane reads as a
// bug, not as an answer.
func TestRenderFieldDocPaneEmptyState(t *testing.T) {
	got := stripANSI(RenderFieldDocPane(60, FieldDocPane{Path: "spec.opaque", FieldType: "<string>"}))

	assert.Contains(t, got, "No description")
	assert.Contains(t, got, "spec.opaque", "the empty state still names the field")
}

func TestRenderFieldDocPaneLoadingState(t *testing.T) {
	got := stripANSI(RenderFieldDocPane(60, FieldDocPane{Path: "spec.dnsPolicy", Loading: true}))

	assert.Contains(t, strings.ToLower(got), "loading")
}

func TestRenderFieldDocPaneErrorState(t *testing.T) {
	got := stripANSI(RenderFieldDocPane(60, FieldDocPane{
		Path: "spec.nope", Err: `field "nope" does not exist`,
	}))

	assert.Contains(t, got, "does not exist")
}

// An error wins over a stale description: showing old text next to a failed
// lookup would read as if the text belonged to the field that failed.
func TestRenderFieldDocPaneErrorBeatsDesc(t *testing.T) {
	got := stripANSI(RenderFieldDocPane(60, FieldDocPane{
		Path: "spec.nope", Desc: "stale text", Err: "boom",
	}))

	assert.NotContains(t, got, "stale text")
	assert.Contains(t, got, "boom")
}

// The root of a document has no field path. The pane still renders rather than
// collapsing the layout.
func TestRenderFieldDocPaneEmptyPath(t *testing.T) {
	got := RenderFieldDocPane(60, FieldDocPane{Path: "", Desc: "Pod is a collection of containers."})

	assert.Len(t, fieldDocLines(t, got), FieldDocPaneHeight)
}

func TestRenderFieldDocPaneNarrowWidth(t *testing.T) {
	got := RenderFieldDocPane(8, FieldDocPane{Path: "spec.containers.image", Desc: "The container image name."})

	lines := fieldDocLines(t, got)
	assert.Len(t, lines, FieldDocPaneHeight)
	for i, l := range lines {
		assert.LessOrEqual(t, len([]rune(l)), 8, "line %d must stay inside a narrow width", i)
	}
}
