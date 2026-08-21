package ui

import (
	"image/color"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

// Tokyonight Storm color palette default values. These back the mutable
// Color* variables below. No-color mode blanks the variables so inline
// lipgloss.Color(ColorX) calls scattered across the codebase yield
// NoColor{} without touching any call site.
const (
	defaultColorPrimary    = "#7aa2f7" // Blue - borders, headers, breadcrumbs
	defaultColorSecondary  = "#9ece6a" // Green - help keys, running status, success
	defaultColorFile       = "#c0caf5" // Light purple - normal text
	defaultColorSelectedFg = "#24283b" // Dark - selected item foreground
	defaultColorSelectedBg = "#7aa2f7" // Blue - selected item background
	defaultColorBorder     = "#4e5575" // Dark blue - inactive borders
	defaultColorDimmed     = "#4e5575" // Muted purple - help text, placeholders
	defaultColorError      = "#f7768e" // Red/Pink - errors, failures
	defaultColorWarning    = "#e0af68" // Orange/Yellow - warnings, pending
	defaultColorPurple     = "#bb9af7" // Purple - special values
	defaultColorOrange     = "#ff9e64" // Orange - high usage warning
	defaultColorCyan       = "#73daca" // Cyan - very new resources (< 1h)
	defaultColorBase       = "#24283b" // Dark background base
	defaultColorBarBg      = "#313446" // Slightly lighter bar background
	defaultColorSurface    = "#2a2e40" // Surface background for overlays
)

// ThemeColor returns a lipgloss color for the given spec when colors are
// enabled, or NoColor{} when ConfigNoColor is active. Accepts any format
// lipgloss.Color understands: hex ("#f7768e"), ANSI 256 number ("62"), or
// 16-color ANSI number ("2"). Use this helper for inline styles that
// reference raw color literals (not the Color* slots) so they also respect
// no-color mode.
func ThemeColor(spec string) color.Color {
	if ConfigNoColor {
		return lipgloss.NoColor{}
	}
	return lipgloss.Color(spec)
}

// Theme color slots used by inline lipgloss.Color(ColorX) calls throughout
// the codebase. ApplyTheme rewrites them from the active theme so inline
// call sites track theme changes. applyNoColorTheme blanks them so inline
// foreground calls resolve to NoColor{}. They stay as package variables
// so code that already references them keeps compiling unchanged.
//
// Initial values are the Tokyonight Storm defaults so any early call site
// that fires before ApplyTheme still emits a valid color.
var (
	ColorPrimary    = defaultColorPrimary
	ColorSecondary  = defaultColorSecondary
	ColorFile       = defaultColorFile
	ColorSelectedFg = defaultColorSelectedFg
	ColorSelectedBg = defaultColorSelectedBg
	ColorBorder     = defaultColorBorder
	ColorDimmed     = defaultColorDimmed
	ColorError      = defaultColorError
	ColorWarning    = defaultColorWarning
	ColorPurple     = defaultColorPurple
	ColorOrange     = defaultColorOrange
	ColorCyan       = defaultColorCyan
	ColorBase       = defaultColorBase
	ColorBarBg      = defaultColorBarBg
	ColorSurface    = defaultColorSurface
)

var (
	// Column styles.
	ActiveColumnStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(ColorPrimary))

	InactiveColumnStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(ColorBorder))

	// Item styles.
	SelectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorSelectedFg)).
			Background(lipgloss.Color(ColorSelectedBg))

	NormalStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFile))

	DimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDimmed))

	// BarDimStyle is DimStyle but with bar background (for status bar hints).
	BarDimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDimmed))

	// BarNormalStyle is NormalStyle but with bar background (for status bar text).
	BarNormalStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFile))

	// Category header in resource type list.
	CategoryStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorDimmed)).
			Bold(true).
			Italic(true)

	// Category header rendered as a full-width "bar" with a distinct background, used by explorer columns to separate groups.
	CategoryBarStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorPrimary)).
				Bold(true)

	// Resource type icon style.
	IconStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary))

	// Status colors: Green=running, Blue=progressing, Red=error, Grey=completed/other, Amber=warning.
	StatusRunning     = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary))
	StatusProgressing = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary))
	StatusFailed      = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError))
	StatusOther       = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDimmed))
	StatusWarning     = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorWarning))

	// Whole-row status tint (issue #540). Colors mirror the Status cell (StatusFailed = error, StatusProgressing = primary) so
	// the row tint and the cell never disagree. Fg variants recolor the row text. Bg variants lay a muted severity background.
	RowTintFailedFg      = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError))
	RowTintProgressingFg = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary))
	RowTintFailedBg      = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFile)).
				Background(lipgloss.Color(blendHexToward(defaultColorBase, defaultColorError, rowTintBgBlend)))
	RowTintProgressingBg = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFile)).
				Background(lipgloss.Color(blendHexToward(defaultColorBase, defaultColorPrimary, rowTintBgBlend)))
	// Cursor row in background mode: the status background blended toward the
	// selection color so the cursor stays visible on a tinted row (#540 UAT).
	RowTintFailedCursorBg = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFile)).Bold(true).
				Background(lipgloss.Color(blendHexToward(blendHexToward(defaultColorBase, defaultColorError, rowTintBgBlend), defaultColorSelectedBg, rowTintCursorBlend)))
	RowTintProgressingCursorBg = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFile)).Bold(true).
					Background(lipgloss.Color(blendHexToward(blendHexToward(defaultColorBase, defaultColorPrimary, rowTintBgBlend), defaultColorSelectedBg, rowTintCursorBlend)))

	// Title bar (full-width background).
	TitleBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(ColorBarBg)).
			Foreground(lipgloss.Color(ColorFile)).
			Padding(0, 1)

	TitleBreadcrumbStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(ColorPrimary)).
				Background(lipgloss.Color(ColorBarBg))

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorPrimary)).
			Padding(0, 1)

	// Namespace badge in title bar.
	NamespaceBadgeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorSelectedFg)).
				Background(lipgloss.Color(ColorPrimary)).
				Bold(true).
				Padding(0, 1)

	// Read-only badge in title bar. Warning color so it's hard to miss.
	ReadOnlyBadgeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorSelectedFg)).
				Background(lipgloss.Color(ColorWarning)).
				Bold(true).
				Padding(0, 1)

	// Demo-mode badge: ReadOnlyBadgeStyle with a distinguishing purple background.
	DemoBadgeStyle = ReadOnlyBadgeStyle.Background(lipgloss.Color(ColorPurple))

	// Subtle [RO] marker for list rows. Foreground-only so it doesn't
	// compete with the row content the way a solid-background badge does.
	// Same visual weight as CurrentMarkerStyle (* prefix).
	ReadOnlyMarkerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorWarning)).
				Bold(true)

	// Column header with underline and icon.
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorPrimary)).
			Underline(true)

	HeaderIconStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorPrimary))

	// SortActiveHeaderStyle highlights the header label and sort arrow of the
	// column the table is currently sorted by, so the active sort column reads
	// distinctly against the dim of the inactive headers.
	SortActiveHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(ColorPrimary))

	// Namespace indicator in top-right (kept for compat).
	NamespaceStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorWarning)).
			Bold(true).
			Padding(0, 1)

	// Full screen YAML view.
	YamlViewStyle = lipgloss.NewStyle().
			Padding(1, 2)

	// YAML key highlighting.
	YamlKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorPrimary)).
			Bold(true)

	YamlValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFile))

	YamlPunctuationStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorDimmed))

	YamlCommentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorDimmed)).
				Italic(true)

	// YAML syntax highlighting: value types.
	YamlStringStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSecondary))

	YamlNumberStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorOrange))

	YamlBoolStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorOrange)).
			Bold(true)

	YamlNullStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorPurple)).
			Italic(true)

	YamlAnchorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorCyan)).
			Bold(true)

	YamlTagStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorPurple))

	YamlBlockScalarStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorPurple)).
				Bold(true)

	// Status bar (full-width background).
	StatusBarBgStyle = lipgloss.NewStyle().
				Background(lipgloss.Color(ColorBarBg)).
				Foreground(lipgloss.Color(ColorDimmed)).
				Padding(0, 1)

	StatusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorDimmed)).
			Padding(0, 1)

	// Help key style (for status bar hints).
	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSecondary)).
			Bold(true)

	// Schema side pane (ctrl+k): the header names the field and carries
	// the accent. The description is prose to read, so it stays plain.
	FieldDocHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Bold(true)
	FieldDocTextStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFile))
	FieldDocErrorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError))

	// Which-key panel. The panel draws one flat list with no section
	// headers, so the description's color is the only thing left that says
	// which family an entry belongs to. The key keeps one accent throughout.
	// Red is deliberately unused: it is the app's failure/destructive signal,
	// and a red description reads as an error message rather than as a
	// category — a risk the panel's most destructive entries (Delete, Force
	// delete) would run head-first into.
	//
	// The group styles are not bold: bold is what marks the key, and six bold
	// colored sentences per panel is noise, not emphasis.
	//
	// WhichKeyDescStyle is the ungrouped default: the g-prefix goto popup has
	// no groups and renders entirely through it. No group may reuse its color,
	// nor the key's, or the two halves of a row collapse into one
	// (TestWhichKeyGroupStyles_NeverMatchThePlainDescriptionOrTheKey). The key
	// keeps Secondary because HelpKeyStyle draws every hint-bar hotkey in that
	// same green bold. Actions is what moved, having held Secondary since the
	// accent sat on the KEY rather than on the description.
	WhichKeyKeyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Bold(true)
	WhichKeyDescStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFile))
	// Actions is the largest group, so it stays neutral and the five smaller
	// groups carry the accents. Sharing WhichKeyDescStyle is deliberate: exactly
	// one group may be neutral, pinned by the group-style guard.
	WhichKeyActionsStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFile))
	WhichKeyViewsStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary))
	WhichKeyFilterStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan))
	WhichKeySelectionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPurple))
	WhichKeySortStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorWarning))
	WhichKeySettingsStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorOrange))

	// Error style.
	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorError)).
			Bold(true)

	// Current context marker.
	CurrentMarkerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorSecondary)).
				Bold(true)

	// Overlay styles (namespace selector, action menu).
	OverlayStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorPrimary)).
			Padding(1, 2)

	OverlayTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(ColorPrimary)).
				Padding(0, 0, 1, 0)

	OverlaySelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(ColorSelectedFg)).
				Background(lipgloss.Color(ColorSelectedBg))

	OverlayNormalStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorFile))

	OverlayFilterStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorSecondary)).
				Bold(true)

	OverlayDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorDimmed))

	// Confirm overlay styles.
	OverlayWarningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorError)).
				Bold(true)

	// Scale input style.
	OverlayInputStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorFile)).
				Bold(true).
				Underline(true)

	// Parent highlight style (dimmer than active selection).
	ParentHighlightStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(ColorFile)).
				Background(lipgloss.Color(ColorBorder))

	// Status message style (temporary success/error in status bar).
	StatusMessageOkStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorSecondary)).
				Bold(true)

	StatusMessageErrStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorError)).
				Bold(true)

	// SearchHighlightStyle highlights search/filter matches in item names.
	SearchHighlightStyle = lipgloss.NewStyle().
				Background(lipgloss.Color(ColorWarning)).
				Foreground(lipgloss.Color(ColorBase)).
				Bold(true)

	// SelectedSearchHighlightStyle highlights the currently selected search
	// match (the one n/N steps to). The distinct purple background (vs the
	// warning-yellow used by regular matches) and underline together make the
	// current match visually unambiguous across themes. Foreground stays the
	// same dark base color so text remains legible on both backgrounds.
	SelectedSearchHighlightStyle = lipgloss.NewStyle().
					Background(lipgloss.Color(ColorPurple)).
					Foreground(lipgloss.Color(ColorBase)).
					Bold(true).
					Underline(true)

	// SelectionMarkerStyle styles the checkmark shown on multi-selected items.
	SelectionMarkerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorSecondary)).
				Bold(true)

	// SelectionCountStyle styles the selection count badge in the status bar.
	SelectionCountStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorSelectedFg)).
				Background(lipgloss.Color(ColorSecondary)).
				Bold(true)

	// YamlCursorIndicatorStyle styles the gutter indicator on the YAML cursor line.
	YamlCursorIndicatorStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color(ColorPrimary))

	// DeprecationStyle styles the deprecation warning indicator on resource type items.
	DeprecationStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorWarning))
)

// FillLinesBg post-processes a multi-line string so that the background color
// is continuous across each line. It does two things:
//  1. Re-establishes the background after every ANSI reset (\x1b[0m) within a line,
//     so gaps between styled segments get the background.
//  2. Pads each line to the given width with the background color.
//
// bg should be BaseBg, BarBg, or SurfaceBg.
func FillLinesBg(content string, width int, bg color.Color) string {
	if _, ok := bg.(lipgloss.NoColor); ok {
		return content // transparent mode, nothing to fill
	}
	// Extract the raw ANSI background sequence from a styled render.
	// lipgloss.NewStyle().Background(bg).Render("X") produces "<bg>X<reset>".
	// We extract everything before "X".
	sample := lipgloss.NewStyle().Background(bg).Render("X")
	idx := strings.Index(sample, "X")
	if idx <= 0 {
		return content // cannot extract bg sequence
	}
	bgSeq := sample[:idx]

	fill := lipgloss.NewStyle().Background(bg)
	reset := "\x1b[0m"
	// lipgloss/reflow emit the parameterless SGR reset (ESC[m) at word-wrap
	// boundaries inside a styled span, while a style's own closing reset is the
	// full ESC[0m. Both clear the background, so both must be followed by bgSeq
	// — otherwise padding after a wrap-induced reset renders with the terminal
	// default (black under non-black themes), "tearing" the panel (issue #293).
	shortReset := "\x1b[m"

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		// Prepend bg at start of line and after every ANSI reset.
		line = bgSeq + strings.ReplaceAll(line, reset, reset+bgSeq)
		line = strings.ReplaceAll(line, shortReset, shortReset+bgSeq)
		// Pad to full width.
		w := lipgloss.Width(line)
		if w < width {
			line += fill.Render(strings.Repeat(" ", width-w))
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

// AgeStyle returns a color style based on the age string of a resource.
// Very new resources (< 1h) are cyan, recent (< 24h) are green,
// normal (1-7d) are default dim, and old (> 7d) are extra dim.
func AgeStyle(age string) lipgloss.Style {
	if age == "" {
		return DimStyle
	}

	// Parse the numeric prefix and unit suffix (e.g., "5m", "2h", "3d", "14d", "1y").
	unit := age[len(age)-1]
	numStr := strings.TrimRight(age[:len(age)-1], " ")
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return DimStyle
	}

	switch unit {
	case 's', 'm':
		// Seconds or minutes: less than 1 hour — very new.
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Background(BaseBg)
	case 'h':
		// Hours: less than 24 hours — recent.
		if num < 24 {
			return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Background(BaseBg)
		}
		return DimStyle
	case 'd':
		// Days: 1-7 days is normal dim, > 7 days is extra dim.
		if num > 7 {
			return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBorder)).Background(BaseBg)
		}
		return DimStyle
	case 'y':
		// Years: old.
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBorder)).Background(BaseBg)
	default:
		return DimStyle
	}
}

// BoxWidth sets style's width so its rendered CONTENT is w columns wide.
//
// lipgloss v2 counts the border inside Width(). v1 counted it outside. Every
// caller computes a content width and budgets the border columns separately,
// so the border has to be added back or each box renders two columns narrow —
// and nested boxes compound the loss.
func BoxWidth(style lipgloss.Style, w int) lipgloss.Style {
	return style.Width(w + style.GetHorizontalBorderSize())
}

// BoxHeight sets style's height so its rendered CONTENT is h rows tall.
// Counterpart to BoxWidth: lipgloss v2 counts the border inside Height() too.
func BoxHeight(style lipgloss.Style, h int) lipgloss.Style {
	return style.Height(h + style.GetVerticalBorderSize())
}
