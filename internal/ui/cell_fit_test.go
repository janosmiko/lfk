package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

const styledDot = "\x1b[31m●\x1b[m"

func TestCellPad(t *testing.T) {
	cases := []struct {
		name      string
		s         string
		w         int
		wantRight string
		wantLeft  string
	}{
		{"plain", "ab", 5, "ab   ", "   ab"},
		{"ansi styled", styledDot, 3, styledDot + "  ", "  " + styledDot},
		{"wide", "中", 4, "中  ", "  中"},
		{"too long", "hello world", 5, "hello world", "hello world"},
		{"exact width", "hello", 5, "hello", "hello"},
		{"zero width", "ab", 0, "ab", "ab"},
		{"negative width", "ab", -1, "ab", "ab"},
		{"empty", "", 3, "   ", "   "},
		{"tab", "a\tb", 6, "a\tb    ", "    a\tb"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.wantRight, padRight(tc.s, tc.w))
			assert.Equal(t, tc.wantLeft, padLeft(tc.s, tc.w))
		})
	}
}

func TestCellTruncate(t *testing.T) {
	cases := []struct {
		name string
		s    string
		w    int
		want string
	}{
		{"plain", "hello world", 5, "hell…"},
		{"ansi styled", "\x1b[31mhello\x1b[m", 3, "\x1b[31mhe…\x1b[m"},
		{"wide", "中文字", 4, "中…"},
		{"fits", "hi", 5, "hi"},
		{"exact width", "hello", 5, "hello"},
		{"one cell", "hello", 1, "…"},
		{"zero width", "hello", 0, ""},
		{"negative width", "hello", -1, ""},
		{"empty", "", 3, ""},
		{"tab", "a\tbcdef", 4, "a\tbc…"},
		{"multibyte runes", "αβγδε", 4, "αβγ…"},
		{"long key", "extremely_long_key_name", 18, "extremely_long_ke…"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ansi.Truncate(tc.s, tc.w, "…"))
		})
	}
}

func TestCellTruncateDots(t *testing.T) {
	cases := []struct {
		s    string
		w    int
		want string
	}{
		{"hello", 5, "hello"},
		{"hi", 5, "hi"},
		{"hello world", 8, "hello..."},
		{"abcdef", 4, "a..."},
		{"", 5, ""},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, ansi.Truncate(tc.s, tc.w, "..."))
	}
}

func TestCanIGroupsLongCursorRowStaysOneLine(t *testing.T) {
	lines := renderCanIGroups([]string{strings.Repeat("g", 50)}, 0, 0, 20, 1)
	assert.Len(t, lines, 1)
	assert.NotContains(t, lines[0], "\n")
	assert.Equal(t, 20, lipgloss.Width(lines[0]))
}

func TestClusterPickerTrailingSwatchRightAligned(t *testing.T) {
	swatch := ClusterColorSwatchBg("red")
	assert.Contains(t, swatch, "\x1b[")
	got := clusterPickerTrailing(model.Item{ClusterColor: "red"})
	want := strings.Repeat(" ", clusterPickerDefColW+clusterPickerStatusColW+clusterPickerColorColW-2) + swatch
	assert.Equal(t, want, got)
}

func TestFinalizerOverlayLongNameEndsWithDots(t *testing.T) {
	name := strings.Repeat("n", 60)
	out := RenderFinalizerSearchOverlay(
		[]FinalizerMatchEntry{{Name: name, Namespace: "default", Kind: "Pod", Matched: "kubernetes.io/pvc-protection", Age: "1d"}},
		0, map[string]bool{}, "pvc", "", false, false, 80, 20,
	)
	assert.Contains(t, ansi.Strip(out), strings.Repeat("n", 18)+"... ")
}

func TestTrafficCaptureLongReasonEndsWithEllipsis(t *testing.T) {
	out := buildCaptureConfig(CaptureOverlayEntry{
		Backends: []CaptureBackendChip{{Label: "ephemeral", Reason: strings.Repeat("x", 40)}},
	}, 120)
	assert.Contains(t, ansi.Strip(out), "ephemeral ("+strings.Repeat("x", 29)+"…)")
}

func TestRightsizingNarrowNameTruncated(t *testing.T) {
	name := "a-very-long-container-name-that-overflows"
	layout := rsLayoutFor(false, 60)
	w := layout.widths[0]
	cells := rightsizingDataRow(name, "cpu", model.ResourceRec{}, false, layout)
	assert.Less(t, w, len(name))
	assert.Equal(t, " "+name[:w-1]+"… ", ansi.Strip(cells[0]))
}
