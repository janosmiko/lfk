package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// rowContaining returns the rendered line that contains the given display name.
func rowContaining(t *testing.T, out, name string) string {
	t.Helper()
	for line := range strings.SplitSeq(out, "\n") {
		if strings.Contains(line, name) {
			return line
		}
	}
	t.Fatalf("row %q not found in output", name)
	return ""
}

// TestRenderColumn_HiddenRowDimNotBrokenByInnerReset guards the dim path for
// hidden resource types: the row must be wrapped in a single dim span with no
// interior reset (\x1b[0m) before the content ends. The pre-fix code wrapped
// already-styled fragments (IconStyle + NormalStyle, each with its own reset),
// so the dim was canceled after the icon and the name rendered un-dimmed.
//
// DimStyle and the color profile are pinned locally because they are mutable
// package globals other theme tests reassign; without pinning this test's
// result would depend on execution order.
func TestRenderColumn_HiddenRowDimNotBrokenByInnerReset(t *testing.T) {
	origProfile := lipgloss.DefaultRenderer().ColorProfile()
	origDim := DimStyle
	t.Cleanup(func() {
		lipgloss.DefaultRenderer().SetColorProfile(origProfile)
		DimStyle = origDim
	})
	lipgloss.DefaultRenderer().SetColorProfile(termenv.ANSI256)
	DimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	openSGR := strings.TrimSuffix(DimStyle.Render("X"), "X\x1b[0m")
	require.NotEmpty(t, openSGR, "DimStyle must emit an opening SGR under this setup")

	items := []model.Item{
		{Name: "Pods", Category: "Workloads", Icon: model.Icon{Unicode: "P"}},
		{Name: "Ingresses", Category: "Networking", Icon: model.Icon{Unicode: "I"}, Hidden: true},
	}

	for _, tc := range []struct {
		name     string
		isActive bool
	}{
		{"active column", true},
		{"parent column", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ActiveMiddleScroll, ActiveLeftScroll = 0, 0
			defer func() { ActiveMiddleScroll, ActiveLeftScroll = -1, -1 }()

			out := RenderColumn("", items, 0, 40, 20, tc.isActive, false, "", "")
			row := rowContaining(t, out, "Ingresses")

			assert.True(t, strings.HasPrefix(row, openSGR), "hidden row must open with the dim style")
			// One uniform dim span: a single reset, none interior. The buggy
			// per-fragment path produced a reset between the icon and the name.
			assert.Equal(t, 1, strings.Count(row, "\x1b[0m"),
				"dim must not be broken by interior resets from per-fragment styling")
		})
	}
}
