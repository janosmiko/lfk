package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/require"
)

// assertGolden compares got to testdata/<name>. Run with UPDATE_GOLDEN=1 to
// rewrite the file.
func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if os.Getenv("UPDATE_GOLDEN") != "" {
		require.NoError(t, os.MkdirAll("testdata", 0o750))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o600))
		return
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, string(want), got)
}

func themeStyleTable() []struct {
	name  string
	style *lipgloss.Style
} {
	return []struct {
		name  string
		style *lipgloss.Style
	}{
		{"ActiveColumnStyle", &ActiveColumnStyle},
		{"InactiveColumnStyle", &InactiveColumnStyle},
		{"SelectedStyle", &SelectedStyle},
		{"NormalStyle", &NormalStyle},
		{"DimStyle", &DimStyle},
		{"BarDimStyle", &BarDimStyle},
		{"BarNormalStyle", &BarNormalStyle},
		{"CategoryStyle", &CategoryStyle},
		{"CategoryBarStyle", &CategoryBarStyle},
		{"IconStyle", &IconStyle},
		{"StatusRunning", &StatusRunning},
		{"StatusProgressing", &StatusProgressing},
		{"StatusFailed", &StatusFailed},
		{"StatusOther", &StatusOther},
		{"StatusWarning", &StatusWarning},
		{"RowTintFailedFg", &RowTintFailedFg},
		{"RowTintProgressingFg", &RowTintProgressingFg},
		{"RowTintFailedBg", &RowTintFailedBg},
		{"RowTintProgressingBg", &RowTintProgressingBg},
		{"RowTintFailedCursorBg", &RowTintFailedCursorBg},
		{"RowTintProgressingCursorBg", &RowTintProgressingCursorBg},
		{"TitleBarStyle", &TitleBarStyle},
		{"TitleBreadcrumbStyle", &TitleBreadcrumbStyle},
		{"TitleStyle", &TitleStyle},
		{"NamespaceBadgeStyle", &NamespaceBadgeStyle},
		{"ReadOnlyBadgeStyle", &ReadOnlyBadgeStyle},
		{"DemoBadgeStyle", &DemoBadgeStyle},
		{"ReadOnlyMarkerStyle", &ReadOnlyMarkerStyle},
		{"HeaderStyle", &HeaderStyle},
		{"HeaderIconStyle", &HeaderIconStyle},
		{"SortActiveHeaderStyle", &SortActiveHeaderStyle},
		{"NamespaceStyle", &NamespaceStyle},
		{"YamlViewStyle", &YamlViewStyle},
		{"YamlKeyStyle", &YamlKeyStyle},
		{"YamlValueStyle", &YamlValueStyle},
		{"YamlPunctuationStyle", &YamlPunctuationStyle},
		{"YamlCommentStyle", &YamlCommentStyle},
		{"YamlStringStyle", &YamlStringStyle},
		{"YamlNumberStyle", &YamlNumberStyle},
		{"YamlBoolStyle", &YamlBoolStyle},
		{"YamlNullStyle", &YamlNullStyle},
		{"YamlAnchorStyle", &YamlAnchorStyle},
		{"YamlTagStyle", &YamlTagStyle},
		{"YamlBlockScalarStyle", &YamlBlockScalarStyle},
		{"StatusBarBgStyle", &StatusBarBgStyle},
		{"StatusBarStyle", &StatusBarStyle},
		{"HelpKeyStyle", &HelpKeyStyle},
		{"FieldDocHeaderStyle", &FieldDocHeaderStyle},
		{"FieldDocTextStyle", &FieldDocTextStyle},
		{"FieldDocErrorStyle", &FieldDocErrorStyle},
		{"WhichKeyKeyStyle", &WhichKeyKeyStyle},
		{"WhichKeyDescStyle", &WhichKeyDescStyle},
		{"WhichKeyActionsStyle", &WhichKeyActionsStyle},
		{"WhichKeyViewsStyle", &WhichKeyViewsStyle},
		{"WhichKeyFilterStyle", &WhichKeyFilterStyle},
		{"WhichKeySelectionStyle", &WhichKeySelectionStyle},
		{"WhichKeySortStyle", &WhichKeySortStyle},
		{"WhichKeySettingsStyle", &WhichKeySettingsStyle},
		{"ErrorStyle", &ErrorStyle},
		{"CurrentMarkerStyle", &CurrentMarkerStyle},
		{"OverlayStyle", &OverlayStyle},
		{"OverlayTitleStyle", &OverlayTitleStyle},
		{"OverlaySelectedStyle", &OverlaySelectedStyle},
		{"OverlayNormalStyle", &OverlayNormalStyle},
		{"OverlayFilterStyle", &OverlayFilterStyle},
		{"OverlayDimStyle", &OverlayDimStyle},
		{"OverlayWarningStyle", &OverlayWarningStyle},
		{"OverlayInputStyle", &OverlayInputStyle},
		{"ParentHighlightStyle", &ParentHighlightStyle},
		{"StatusMessageOkStyle", &StatusMessageOkStyle},
		{"StatusMessageErrStyle", &StatusMessageErrStyle},
		{"SearchHighlightStyle", &SearchHighlightStyle},
		{"SelectedSearchHighlightStyle", &SelectedSearchHighlightStyle},
		{"SelectionMarkerStyle", &SelectionMarkerStyle},
		{"SelectionCountStyle", &SelectionCountStyle},
		{"YamlCursorIndicatorStyle", &YamlCursorIndicatorStyle},
		{"DeprecationStyle", &DeprecationStyle},
		{"innerPanelStyle", &innerPanelStyle},
		{"crashTabSeparatorStyle", &crashTabSeparatorStyle},
		{"crashSectionStyle", &crashSectionStyle},
		{"crashHeaderStyle", &crashHeaderStyle},
	}
}

func dumpThemeStyles() string {
	var b strings.Builder
	for _, s := range themeStyleTable() {
		fmt.Fprintf(&b, "%s %q\n", s.name, s.style.Render("x"))
	}
	fmt.Fprintf(&b, "BaseBg %T\nBarBg %T\nSurfaceBg %T\n", BaseBg, BarBg, SurfaceBg)
	slots := []string{
		ColorPrimary, ColorSecondary, ColorFile, ColorSelectedFg, ColorSelectedBg,
		ColorBorder, ColorDimmed, ColorError, ColorWarning, ColorPurple,
		ColorOrange, ColorCyan, ColorBase, ColorBarBg, ColorSurface,
	}
	fmt.Fprintf(&b, "slots %q\n", slots)
	return b.String()
}

func TestThemeStylesGolden(t *testing.T) {
	prevNoColor, prevTransparent, prevContrast := ConfigNoColor, ConfigTransparentBg, ConfigMinContrastRatio
	t.Cleanup(func() {
		ConfigNoColor, ConfigTransparentBg, ConfigMinContrastRatio = prevNoColor, prevTransparent, prevContrast
		ApplyTheme(DefaultTheme())
	})

	cases := []struct {
		golden      string
		noColor     bool
		transparent bool
		contrast    float64
	}{
		{"theme_styles_default.golden", false, false, 0},
		{"theme_styles_nocolor.golden", true, false, 0},
		{"theme_styles_transparent_contrast.golden", false, true, 0.5},
	}
	for _, tc := range cases {
		t.Run(tc.golden, func(t *testing.T) {
			ConfigNoColor, ConfigTransparentBg, ConfigMinContrastRatio = tc.noColor, tc.transparent, tc.contrast
			ApplyTheme(DefaultTheme())
			assertGolden(t, tc.golden, dumpThemeStyles())
		})
	}
}
