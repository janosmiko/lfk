package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The Log Top column-visibility overlay's hint bar advertises the
// fuzzy/literal mode chord alongside the filter key.
func TestOverlayHintBarLogTop_ColumnsShowsSearchModeHint(t *testing.T) {
	m := Model{overlay: overlayLogTopColumns, width: 120}
	got := stripANSI(m.overlayHintBar())
	assert.Contains(t, got, "~: fuzzy", "column-visibility hint bar must advertise the fuzzy chord")
}

// The column-visibility list's filter row is mode-aware: a ~ query shows
// the fuzzy label instead of the plain "/" prompt.
func TestRenderLogTopColumnsOverlay_FilterShowsModeLabel(t *testing.T) {
	m := basePush80Model()
	m.mode = modeLogTop
	m.logView.rawLines = []string{
		`2026-06-18T10:00:00Z {"RequestMethod":"GET","RequestPath":"/api/users","DownstreamStatus":200}`,
	}
	m.logTopResetAndParse()
	m.logTop.colFilterActive = true
	m.logTop.colFilter = "~re"

	content, _, _ := m.renderLogTopColumnsOverlay()
	assert.Contains(t, stripANSI(content), "[fuzzy]", "column filter must show the fuzzy mode label for a ~ query")
}
