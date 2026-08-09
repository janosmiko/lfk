package ui

import (
	"charm.land/lipgloss/v2"
)

// applyNoColorTheme rebuilds every style global without foreground or
// background colors. The approach:
//
//  1. Blank every Color* variable so inline lipgloss.Color(ColorX) calls
//     scattered through the codebase resolve to an empty hex string, which
//     lipgloss treats as NoColor{} and emits no color SGR.
//  2. Rebuild each theme style global without Foreground/Background.
//
// Selection visibility is preserved by Bold+Reverse on SelectedStyle and
// OverlaySelectedStyle — terminal-native inverse video, which is visible
// in any terminal and does not shift row layout.
//
// lipgloss v2 has no global renderer, so there is no color profile to force
// here. Blanking the slots is the whole mechanism: SGR attribute codes (bold,
// reverse, underline, faint) are emitted independently of color.
func applyNoColorTheme() {
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
	StatusWarning = lipgloss.NewStyle().Italic(true)

	// Row tint degrades to attribute cues without color: failed rows bold,
	// progressing rows italic — both variants share the cue.
	RowTintFailedFg = lipgloss.NewStyle().Bold(true)
	RowTintProgressingFg = lipgloss.NewStyle().Italic(true)
	RowTintFailedBg = lipgloss.NewStyle().Bold(true)
	RowTintProgressingBg = lipgloss.NewStyle().Italic(true)
	// Cursor-blend backgrounds are never read in no-color mode (the cursor
	// falls back to the reverse-video selection), but define them so the
	// package globals never hold stale colors after a theme switch.
	RowTintFailedCursorBg = lipgloss.NewStyle().Bold(true)
	RowTintProgressingCursorBg = lipgloss.NewStyle().Bold(true)

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
	SortActiveHeaderStyle = lipgloss.NewStyle().Bold(true).Underline(true)
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

	// Schema side pane: without color the header keeps bold, which is the
	// only cue left that separates it from the description below it.
	FieldDocHeaderStyle = lipgloss.NewStyle().Bold(true)
	FieldDocTextStyle = lipgloss.NewStyle()
	// Without color, bold is the only thing left to mark an error apart from
	// the description it replaces.
	FieldDocErrorStyle = lipgloss.NewStyle().Bold(true)

	// Which-key: the group accent is purely a color cue on the description, so
	// without color every group collapses to the same plain description the
	// panel had before groups were colored at all. Faint/italic/underline are
	// not pressed into service as substitutes — six groups need six cues, there
	// are not six legible attributes, and a half-applied scheme reads as noise.
	// The entries stay perfectly usable; only the category hint is gone.
	WhichKeyKeyStyle = lipgloss.NewStyle().Bold(true)
	WhichKeyDescStyle = lipgloss.NewStyle()
	WhichKeyActionsStyle = lipgloss.NewStyle()
	WhichKeyViewsStyle = lipgloss.NewStyle()
	WhichKeyFilterStyle = lipgloss.NewStyle()
	WhichKeySelectionStyle = lipgloss.NewStyle()
	WhichKeySortStyle = lipgloss.NewStyle()
	WhichKeySettingsStyle = lipgloss.NewStyle()

	ErrorStyle = lipgloss.NewStyle().Bold(true)
	CurrentMarkerStyle = lipgloss.NewStyle().Bold(true)

	OverlayStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2)

	innerPanelStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		Padding(0, 1)

	// Crash investigator divider + section/header styles. Bold and
	// underline survive no-color mode; foreground and background colors
	// are stripped so emphasis remains via SGR attributes only.
	crashTabSeparatorStyle = lipgloss.NewStyle()
	crashSectionStyle = lipgloss.NewStyle().Bold(true).Underline(true)
	crashHeaderStyle = lipgloss.NewStyle().Bold(true)

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
