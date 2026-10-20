package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

func TestHoverStyleNoColorDiffersFromSelected(t *testing.T) {
	prev := ConfigNoColor
	t.Cleanup(func() {
		ConfigNoColor = prev
		ApplyTheme(DefaultTheme())
	})
	ConfigNoColor = true
	ApplyTheme(DefaultTheme())

	assert.NotEqual(t, SelectedStyle.Render("x"), HoverStyle.Render("x"))
	assert.NotEqual(t, "x", HoverStyle.Render("x"))
}

func TestDerivedHoverBg(t *testing.T) {
	th := DefaultTheme()
	got := derivedHoverBg(th)
	assert.NotEmpty(t, got)
	assert.NotEqual(t, th.Base, got)
	assert.NotEqual(t, th.SelectedBg, got)
	assert.Empty(t, derivedHoverBg(Theme{}))
}

func TestRenderTableHoverRow(t *testing.T) {
	prevHover, prevScroll, prevQuery := ActiveHoverRow, ActiveMiddleScroll, ActiveHighlightQuery
	t.Cleanup(func() {
		ActiveHoverRow, ActiveMiddleScroll, ActiveHighlightQuery = prevHover, prevScroll, prevQuery
	})
	ActiveMiddleScroll, ActiveHighlightQuery = -1, ""
	items := []model.Item{
		{Name: "alpha", Status: "Running", Ready: "1/1", Age: "5m"},
		{Name: "bravo", Status: "Running", Ready: "1/1", Age: "5m"},
		{Name: "charlie", Status: "Running", Ready: "1/1", Age: "5m"},
	}
	line := func(out string, n int) string { return strings.Split(out, "\n")[n] }

	ActiveHoverRow = -1
	plain := RenderTable("NAME", items, 0, 100, 20, false, "", "")
	ActiveHoverRow = 2
	hovered := RenderTable("NAME", items, 0, 100, 20, false, "", "")

	assert.Equal(t, line(plain, 1), line(hovered, 1), "cursor row keeps its style")
	assert.Equal(t, line(plain, 2), line(hovered, 2), "other rows are untouched")
	assert.NotEqual(t, line(plain, 3), line(hovered, 3), "hovered row is restyled")
	assert.Equal(t, ansi.Strip(line(plain, 3)), ansi.Strip(line(hovered, 3)))

	ActiveHoverRow = 0
	onCursor := RenderTable("NAME", items, 0, 100, 20, false, "", "")
	assert.Equal(t, plain, onCursor, "hover on the cursor row changes nothing")
}

func TestColumnHoverRowKeepsSearchHighlight(t *testing.T) {
	prevHover, prevQuery := ActiveHoverRow, ActiveHighlightQuery
	t.Cleanup(func() { ActiveHoverRow, ActiveHighlightQuery = prevHover, prevQuery })
	items := []model.Item{{Name: "alpha"}, {Name: "bravo"}, {Name: "charlie"}}
	render := func() string {
		return RenderColumn("H", items, 0, 40, 10, true, false, "", "")
	}

	ActiveHoverRow, ActiveHighlightQuery = 1, ""
	noQuery := render()
	ActiveHighlightQuery = "rav"
	withQuery := render()

	hoveredLine := func(out string) string { return strings.Split(out, "\n")[2] }
	assert.NotEqual(t, hoveredLine(noQuery), hoveredLine(withQuery), "match highlight must reach the hovered row")
}
