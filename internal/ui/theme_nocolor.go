package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// originalColorProfile captures the renderer's profile before no-color mode
// first forced it to ANSI so returning to color mode can restore whatever
// termenv originally detected (TrueColor, ANSI256, Ascii...).
var (
	originalColorProfile      termenv.Profile
	originalColorProfileSaved bool
)

// applyNoColorTheme rebuilds every style global without foreground or
// background colors. The approach:
//
//  1. Force the renderer to the ANSI profile so SGR attribute codes (bold,
//     reverse, underline, faint) still reach the terminal.
//  2. Blank every Color* variable so inline lipgloss.Color(ColorX) calls
//     scattered through the codebase resolve to an empty hex string, which
//     lipgloss treats as NoColor{} and emits no color SGR.
//  3. Rebuild each theme style global without Foreground/Background.
//
// Selection visibility is preserved by Bold+Reverse on SelectedStyle and
// OverlaySelectedStyle — terminal-native inverse video, which is visible
// in any terminal and does not shift row layout.
func applyNoColorTheme() {
	r := lipgloss.DefaultRenderer()
	if !originalColorProfileSaved {
		originalColorProfile = r.ColorProfile()
		originalColorProfileSaved = true
	}
	r.SetColorProfile(termenv.ANSI)

	// Blank every theme color slot so inline Foreground(lipgloss.Color(...))
	// calls emit no color SGR (empty string → NoColor{}).
	ColorPrimary = ""
	ColorSecondary = ""
	ColorFile = ""
	ColorSelectedFg = ""
	ColorSelectedBg = ""
	ColorBorder = ""
	ColorDimmed = ""
	ColorError = ""
	ColorWarning = ""
	ColorPurple = ""
	ColorOrange = ""
	ColorCyan = ""
	ColorBase = ""
	ColorBarBg = ""
	ColorSurface = ""

	none := lipgloss.NoColor{}
	BaseBg = none
	BarBg = none
	SurfaceBg = none

	ActiveColumnStyle = lipgloss.NewStyle().
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		Bold(true)

	InactiveColumnStyle = lipgloss.NewStyle().
		Padding(0, 1).
		Border(lipgloss.RoundedBorder())

	SelectedStyle = lipgloss.NewStyle().
		Bold(true).
		Reverse(true)

	NormalStyle = lipgloss.NewStyle()
	DimStyle = lipgloss.NewStyle().Faint(true)
	BarDimStyle = lipgloss.NewStyle().Faint(true)
	BarNormalStyle = lipgloss.NewStyle()

	CategoryStyle = lipgloss.NewStyle().Bold(true).Italic(true)
	CategoryBarStyle = lipgloss.NewStyle().Bold(true).Underline(true)

	IconStyle = lipgloss.NewStyle()
	StatusRunning = lipgloss.NewStyle()
	StatusProgressing = lipgloss.NewStyle()
	StatusFailed = lipgloss.NewStyle().Bold(true)
	StatusOther = lipgloss.NewStyle().Faint(true)

	TitleBarStyle = lipgloss.NewStyle().Padding(0, 1)
	TitleBreadcrumbStyle = lipgloss.NewStyle().Bold(true)
	TitleStyle = lipgloss.NewStyle().Bold(true).Padding(0, 1)

	NamespaceBadgeStyle = lipgloss.NewStyle().
		Bold(true).
		Reverse(true).
		Padding(0, 1)

	ReadOnlyBadgeStyle = lipgloss.NewStyle().
		Bold(true).
		Reverse(true).
		Padding(0, 1)

	ReadOnlyMarkerStyle = lipgloss.NewStyle().Bold(true)

	HeaderStyle = lipgloss.NewStyle().Bold(true).Underline(true)
	HeaderIconStyle = lipgloss.NewStyle()
	NamespaceStyle = lipgloss.NewStyle().Bold(true).Padding(0, 1)

	YamlViewStyle = lipgloss.NewStyle().Padding(1, 2)
	YamlKeyStyle = lipgloss.NewStyle().Bold(true)
	YamlValueStyle = lipgloss.NewStyle()
	YamlPunctuationStyle = lipgloss.NewStyle().Faint(true)
	YamlCommentStyle = lipgloss.NewStyle().Faint(true).Italic(true)
	YamlStringStyle = lipgloss.NewStyle()
	YamlNumberStyle = lipgloss.NewStyle()
	YamlBoolStyle = lipgloss.NewStyle().Bold(true)
	YamlNullStyle = lipgloss.NewStyle().Italic(true)
	YamlAnchorStyle = lipgloss.NewStyle().Bold(true)
	YamlTagStyle = lipgloss.NewStyle()
	YamlBlockScalarStyle = lipgloss.NewStyle().Bold(true)

	StatusBarBgStyle = lipgloss.NewStyle().Faint(true).Padding(0, 1)
	StatusBarStyle = lipgloss.NewStyle().Faint(true).Padding(0, 1)
	HelpKeyStyle = lipgloss.NewStyle().Bold(true)

	ErrorStyle = lipgloss.NewStyle().Bold(true)
	CurrentMarkerStyle = lipgloss.NewStyle().Bold(true)

	OverlayStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2)

	innerPanelStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		Padding(0, 1)

	OverlayTitleStyle = lipgloss.NewStyle().Bold(true).Padding(0, 0, 1, 0)
	OverlaySelectedStyle = lipgloss.NewStyle().
		Bold(true).
		Reverse(true)
	OverlayNormalStyle = lipgloss.NewStyle()
	OverlayFilterStyle = lipgloss.NewStyle().Bold(true)
	OverlayDimStyle = lipgloss.NewStyle().Faint(true)
	OverlayWarningStyle = lipgloss.NewStyle().Bold(true)
	OverlayInputStyle = lipgloss.NewStyle().Bold(true).Underline(true)

	ParentHighlightStyle = lipgloss.NewStyle().Bold(true).Underline(true)

	StatusMessageOkStyle = lipgloss.NewStyle().Bold(true)
	StatusMessageErrStyle = lipgloss.NewStyle().Bold(true).Reverse(true)

	SearchHighlightStyle = lipgloss.NewStyle().Bold(true).Reverse(true)
	SelectedSearchHighlightStyle = lipgloss.NewStyle().Bold(true).Reverse(true).Underline(true)

	SelectionMarkerStyle = lipgloss.NewStyle().Bold(true)
	SelectionCountStyle = lipgloss.NewStyle().Bold(true).Reverse(true)

	DeprecationStyle = lipgloss.NewStyle().Faint(true).Italic(true)

	YamlCursorIndicatorStyle = lipgloss.NewStyle().Bold(true)
}
