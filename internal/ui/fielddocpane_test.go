package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fieldDocLines(t *testing.T, s string) []string {
	t.Helper()
	return strings.Split(stripANSI(s), "\n")
}

// The pane is joined horizontally next to the viewer, so it has to claim
// exactly the height it was given or lipgloss pads the shorter side.
func TestRenderFieldDocPaneClaimsGivenHeight(t *testing.T) {
	got := RenderFieldDocPane(40, 20, FieldDocPane{
		Path: "spec.dnsPolicy", FieldType: "<string>", Desc: "Set DNS policy for the pod.",
	}, false)

	assert.Len(t, fieldDocLines(t, got), 20)
}

// omitFooter drops the empty status row so a caller can put one full-width
// footer under both panes, the way the log viewer does.
func TestRenderFieldDocPaneOmitFooterIsOneShorter(t *testing.T) {
	withFooter := RenderFieldDocPane(40, 20, FieldDocPane{Path: "spec", Desc: "text"}, false)
	without := RenderFieldDocPane(40, 20, FieldDocPane{Path: "spec", Desc: "text"}, true)

	assert.Len(t, fieldDocLines(t, withFooter), 20)
	assert.Len(t, fieldDocLines(t, without), 19)
}

func TestRenderFieldDocPaneClaimsGivenWidth(t *testing.T) {
	got := RenderFieldDocPane(40, 12, FieldDocPane{
		Path: "spec.containers.image", FieldType: "<string>", Desc: strings.Repeat("word ", 100),
	}, false)

	for i, l := range fieldDocLines(t, got) {
		assert.Equal(t, 40, ansi.StringWidth(l), "row %d must fill the pane width exactly", i)
	}
}

func TestRenderFieldDocPaneShowsPathAndType(t *testing.T) {
	got := stripANSI(RenderFieldDocPane(60, 12, FieldDocPane{
		Path: "spec.dnsPolicy", FieldType: "<string>", Desc: "Set DNS policy for the pod.",
	}, false))

	assert.Contains(t, got, "spec.dnsPolicy")
	assert.Contains(t, got, "<string>")
	assert.Contains(t, got, "Set DNS policy for the pod.")
}

// A long path keeps its tail: the leaf is what names the field being read.
// The description deliberately omits the leaf, or this passes on the body.
func TestRenderFieldDocPaneKeepsPathTail(t *testing.T) {
	got := RenderFieldDocPane(30, 12, FieldDocPane{
		Path: "spec.template.spec.containers.livenessProbe.httpGet.port",
		Desc: "Which one to probe.",
	}, false)

	titleRow := fieldDocLines(t, got)[0]

	assert.Contains(t, titleRow, "port", "the title must keep the leaf segment")
	assert.NotContains(t, stripANSI(got), "Which one to probe.\nport",
		"guard against the assertion passing on the description")
}

func TestRenderFieldDocPaneEmptyState(t *testing.T) {
	got := stripANSI(RenderFieldDocPane(60, 12, FieldDocPane{Path: "spec.opaque", FieldType: "<string>"}, false))

	assert.Contains(t, got, "No description")
}

func TestRenderFieldDocPaneLoadingState(t *testing.T) {
	got := stripANSI(RenderFieldDocPane(60, 12, FieldDocPane{Path: "spec.dnsPolicy", Loading: true}, false))

	assert.Contains(t, strings.ToLower(got), "loading")
}

func TestRenderFieldDocPaneErrorState(t *testing.T) {
	got := stripANSI(RenderFieldDocPane(60, 12, FieldDocPane{
		Path: "spec.nope", Err: `field "nope" does not exist`,
	}, false))

	assert.Contains(t, got, "does not exist")
}

func TestRenderFieldDocPaneErrorBeatsDesc(t *testing.T) {
	got := stripANSI(RenderFieldDocPane(60, 12, FieldDocPane{
		Path: "spec.nope", Desc: "stale text", Err: "boom",
	}, false))

	assert.NotContains(t, got, "stale text")
	assert.Contains(t, got, "boom")
}

// A tall pane fits a long description without cutting it, which is the point
// of moving from a 6-line footnote to a full-height side pane.
func TestRenderFieldDocPaneTallPaneShowsMoreText(t *testing.T) {
	desc := strings.Repeat("sentence ", 60)

	short := stripANSI(RenderFieldDocPane(40, 8, FieldDocPane{Path: "spec", Desc: desc}, false))
	tall := stripANSI(RenderFieldDocPane(40, 30, FieldDocPane{Path: "spec", Desc: desc}, false))

	assert.Greater(t, strings.Count(tall, "sentence"), strings.Count(short, "sentence"))
}

func TestRenderFieldDocPaneDegenerateSizes(t *testing.T) {
	for _, tc := range []struct{ w, h int }{{0, 0}, {1, 1}, {5, 2}, {-3, -3}} {
		got := RenderFieldDocPane(tc.w, tc.h, FieldDocPane{Path: "spec", Desc: "x"}, false)
		require.NotEmpty(t, got, "w=%d h=%d must still render", tc.w, tc.h)
	}
}
