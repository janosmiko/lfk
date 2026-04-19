package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Tokyonight Storm color palette default values. These back the mutable
// Color* variables below; no-color mode blanks the variables so inline
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
func ThemeColor(spec string) lipgloss.TerminalColor {
	if ConfigNoColor {
		return lipgloss.NoColor{}
	}
	return lipgloss.Color(spec)
}

// Theme color slots used by inline lipgloss.Color(ColorX) calls throughout
// the codebase. Overwritten by applyNoColorTheme (blanked) and by the
// color branch of ApplyTheme (restored to defaults). They stay as package
// variables so code that already references them keeps compiling
// unchanged.
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

	NormalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorFile))

	DimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorDimmed))

	// BarDimStyle is DimStyle but with bar background (for status bar hints).
	BarDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorDimmed))

	// BarNormalStyle is NormalStyle but with bar background (for status bar text).
	BarNormalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorFile))

	// Category header in resource type list.
	CategoryStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorDimmed)).
			Bold(true).
			Italic(true)

	// Category header rendered as a full-width "bar" with a distinct
	// background, used by explorer columns to separate groups.
	CategoryBarStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorPrimary)).
				Bold(true)

	// Resource type icon style.
	IconStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorPrimary))

	// Status colors: Green=running, Blue=progressing, Red=error, Grey=completed/other.
	StatusRunning     = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary))
	StatusProgressing = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary))
	StatusFailed      = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError))
	StatusOther       = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDimmed))

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

	// Column header with underline and icon.
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorPrimary)).
			Underline(true)

	HeaderIconStyle = lipgloss.NewStyle().
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

	YamlValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorFile))

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

	// SelectedSearchHighlightStyle highlights search matches on the selected (cursor) item.
	// Uses a contrasting color visible against the selection background.
	SelectedSearchHighlightStyle = lipgloss.NewStyle().
					Background(lipgloss.Color(ColorWarning)).
					Foreground(lipgloss.Color(ColorSelectedBg)).
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
func FillLinesBg(content string, width int, bg lipgloss.TerminalColor) string {
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

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		// Prepend bg at start of line and after every ANSI reset.
		line = bgSeq + strings.ReplaceAll(line, reset, reset+bgSeq)
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

// StatusStyle returns the appropriate style for a resource status string.
func StatusStyle(status string) lipgloss.Style {
	switch status {
	case "default":
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary)).Background(BaseBg)
	case "Running", "Active", "Bound", "Available", "Ready",
		"Healthy", "Healthy/Synced", "Synced",
		"Deployed",
		"SecretSynced", "Created", "Updated", "Valid":
		return StatusRunning
	case "Succeeded", "Completed",
		"Superseded":
		return StatusOther
	case "Pending", "ContainerCreating", "PodInitializing", "Terminating",
		"Waiting", "Init", "NotReady",
		"Progressing", "Progressing/Synced", "Progressing/OutOfSync",
		"Missing", "Suspended", "Unknown", "Reconciling",
		"Healthy/OutOfSync", "Missing/OutOfSync", "Suspended/OutOfSync",
		"OutOfSync",
		"Pending-install", "Pending-upgrade", "Pending-rollback", "Uninstalling":
		return StatusProgressing
	case "Warning":
		return StatusProgressing
	case "Normal":
		return DimStyle
	case "Failed", "CrashLoopBackOff", "Error", "ImagePullBackOff", "Terminated",
		"Degraded", "Degraded/Synced", "Degraded/OutOfSync",
		"Missing/Synced",
		"OOMKilled", "ErrImagePull", "CreateContainerConfigError",
		"SecretSyncedError", "SecretMissing", "MissingProviderSecret",
		"UpdateFailed", "FailedScheduling",
		"InvalidStoreConfiguration", "InvalidProviderConfig", "ValidationFailed":
		return StatusFailed
	default:
		if status == "" {
			return NormalStyle
		}
		return StatusOther
	}
}
