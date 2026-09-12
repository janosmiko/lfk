package ui

import (
	"charm.land/lipgloss/v2"
)

// applyNoColorTheme rebuilds the styles from an empty Theme. An empty color
// string is NoColor{} in lipgloss, so no style can keep a color by being missed.
func applyNoColorTheme() {
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

	buildThemeStyles(Theme{})

	// FillLinesBg skips the fill only for the NoColor type itself.
	none := lipgloss.NoColor{}
	BaseBg = none
	BarBg = none
	SurfaceBg = none

	ActiveColumnStyle = ActiveColumnStyle.Bold(true)
	SelectedStyle = SelectedStyle.Reverse(true)
	DimStyle = DimStyle.Faint(true)
	BarDimStyle = BarDimStyle.Faint(true)
	CategoryBarStyle = CategoryBarStyle.Underline(true)

	StatusFailed = StatusFailed.Bold(true)
	StatusOther = StatusOther.Faint(true)
	StatusWarning = StatusWarning.Italic(true)

	// Row tint degrades to attribute cues without color: failed rows bold,
	// progressing rows italic. Both variants share the cue.
	RowTintFailedFg = RowTintFailedFg.Bold(true)
	RowTintProgressingFg = RowTintProgressingFg.Italic(true)
	RowTintFailedBg = RowTintFailedBg.Bold(true)
	RowTintProgressingBg = RowTintProgressingBg.Italic(true)

	NamespaceBadgeStyle = NamespaceBadgeStyle.Reverse(true)
	ReadOnlyBadgeStyle = ReadOnlyBadgeStyle.Reverse(true)
	DemoBadgeStyle = DemoBadgeStyle.Reverse(true)

	SortActiveHeaderStyle = SortActiveHeaderStyle.Underline(true)

	YamlPunctuationStyle = YamlPunctuationStyle.Faint(true)
	YamlCommentStyle = YamlCommentStyle.Faint(true)

	StatusBarBgStyle = StatusBarBgStyle.Faint(true)
	StatusBarStyle = StatusBarStyle.Faint(true)

	// Without color, bold is the only thing left to mark an error apart from
	// the description it replaces.
	FieldDocErrorStyle = FieldDocErrorStyle.Bold(true)

	// Which-key groups get no substitute: the group accent is purely a color
	// cue, six groups would need six legible attributes, and a half-applied
	// scheme reads as noise. Only the category hint is lost.

	OverlaySelectedStyle = OverlaySelectedStyle.Reverse(true)
	OverlayDimStyle = OverlayDimStyle.Faint(true)

	ParentHighlightStyle = ParentHighlightStyle.Underline(true)

	StatusMessageErrStyle = StatusMessageErrStyle.Reverse(true)

	SearchHighlightStyle = SearchHighlightStyle.Reverse(true)
	SelectedSearchHighlightStyle = SelectedSearchHighlightStyle.Reverse(true)

	SelectionCountStyle = SelectionCountStyle.Reverse(true)

	DeprecationStyle = DeprecationStyle.Faint(true).Italic(true)

	YamlCursorIndicatorStyle = YamlCursorIndicatorStyle.Bold(true)
}
