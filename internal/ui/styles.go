package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Tokyonight-inspired color palette (matching the v project).
const (
	ColorPrimary    = "#7aa2f7" // Blue - borders, headers, breadcrumbs
	ColorSecondary  = "#9ece6a" // Green - help keys, running status, success
	ColorFile       = "#c0caf5" // Light purple - normal text
	ColorSelectedFg = "#1a1b26" // Dark - selected item foreground
	ColorSelectedBg = "#7aa2f7" // Blue - selected item background
	ColorBorder     = "#3b4261" // Dark blue - inactive borders
	ColorDimmed     = "#565f89" // Muted purple - help text, placeholders
	ColorError      = "#f7768e" // Red/Pink - errors, failures
	ColorWarning    = "#e0af68" // Orange/Yellow - warnings, pending
	ColorPurple     = "#bb9af7" // Purple - special values
	ColorOrange     = "#ff9e64" // Orange - high usage warning
	ColorCyan       = "#73daca" // Cyan - very new resources (< 1h)
	ColorBase       = "#1a1b26" // Dark background base
	ColorBarBg      = "#24283b" // Slightly lighter bar background
	ColorSurface    = "#1f2335" // Surface background for overlays
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

	// Category header in resource type list.
	CategoryStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorDimmed)).
			Bold(true).
			Italic(true)

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

// FillLinesBg post-processes a multi-line string so that every line is padded
// to the given width with the specified background color. This fills the gaps
// left by ANSI resets from inner styled text, ensuring the theme background is
// continuous across each line. bg should be BaseBg, BarBg, or SurfaceBg.
func FillLinesBg(content string, width int, bg lipgloss.TerminalColor) string {
	if _, ok := bg.(lipgloss.NoColor); ok {
		return content // transparent mode, nothing to fill
	}
	fill := lipgloss.NewStyle().Background(bg)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		w := lipgloss.Width(line)
		if w < width {
			lines[i] = line + fill.Render(strings.Repeat(" ", width-w))
		}
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
		"Deployed":
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
		"OOMKilled", "ErrImagePull", "CreateContainerConfigError":
		return StatusFailed
	default:
		if status == "" {
			return NormalStyle
		}
		return StatusOther
	}
}
