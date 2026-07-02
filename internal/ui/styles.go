package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
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
// the codebase. ApplyTheme rewrites them from the active theme so inline
// call sites track theme changes; applyNoColorTheme blanks them so inline
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

	// Status colors: Green=running, Blue=progressing, Red=error, Grey=completed/other, Amber=warning.
	StatusRunning     = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary))
	StatusProgressing = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary))
	StatusFailed      = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError))
	StatusOther       = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDimmed))
	StatusWarning     = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorWarning))

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

// statusSev classifies a resource status string into a severity bucket. It is
// the single source of truth shared by StatusStyle (which maps it to a color)
// and StatusSeverityRank (which maps it to an ordering), so coloring and
// severity ordering can never drift — and the ordering stays correct in
// no-color mode, where the styles carry no distinguishing foreground.
type statusSev int

const (
	sevUnknown     statusSev = iota // unknown non-empty status -> dimmed
	sevBlank                        // "" -> neutral
	sevDefault                      // literal "default" -> primary
	sevNormal                       // "Normal" event type -> dim
	sevRunning                      // healthy / ready / synced
	sevDone                         // succeeded / completed
	sevProgressing                  // pending / progressing / out-of-sync / warning
	sevFailed                       // failed / degraded / errored
)

func statusSeverity(status string) statusSev {
	switch status {
	case "default":
		return sevDefault
	case "Running", "Active", "Bound", "Available", "Ready",
		"Healthy", "Healthy/Synced", "Synced",
		"Deployed",
		"SecretSynced", "Created", "Updated", "Valid",
		"Established":
		return sevRunning
	case "Succeeded", "Completed",
		"Superseded":
		return sevDone
	case "Pending", "ContainerCreating", "PodInitializing", "Terminating",
		"Waiting", "Init", "NotReady",
		"Progressing", "Progressing/Synced", "Progressing/OutOfSync",
		"Missing", "Suspended", "Unknown", "Reconciling",
		"Healthy/OutOfSync", "Missing/OutOfSync", "Suspended/OutOfSync",
		"OutOfSync",
		"Pending-install", "Pending-upgrade", "Pending-rollback", "Uninstalling",
		"Warning":
		return sevProgressing
	case "Normal":
		return sevNormal
	case "Failed", "CrashLoopBackOff", "Error", "ImagePullBackOff", "Terminated",
		"Degraded", "Degraded/Synced", "Degraded/OutOfSync",
		"Missing/Synced",
		"OOMKilled", "ErrImagePull", "CreateContainerConfigError",
		"SecretSyncedError", "SecretMissing", "MissingProviderSecret",
		model.MissingRefStatus,
		"UpdateFailed", "FailedScheduling",
		"InvalidStoreConfiguration", "InvalidProviderConfig", "ValidationFailed":
		return sevFailed
	default:
		if status == "" {
			return sevBlank
		}
		return sevUnknown
	}
}

// StatusStyle returns the appropriate style for a resource status string.
func StatusStyle(status string) lipgloss.Style {
	switch statusSeverity(status) {
	case sevDefault:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary)).Background(BaseBg)
	case sevRunning:
		return StatusRunning
	case sevDone:
		return StatusOther
	case sevProgressing:
		return StatusProgressing
	case sevNormal:
		return DimStyle
	case sevFailed:
		return StatusFailed
	case sevBlank:
		return NormalStyle
	default: // sevUnknown
		return StatusOther
	}
}

// StatusSeverityRank orders a status string worst-first (0 = most severe) for
// status rollups and summaries. Derived from statusSeverity so it tracks
// StatusStyle and works regardless of color profile.
func StatusSeverityRank(status string) int {
	switch statusSeverity(status) {
	case sevFailed:
		return 0
	case sevProgressing:
		return 2
	case sevRunning:
		return 3
	default: // done / normal / default / blank / unknown
		return 4
	}
}

// condPolarity classifies how a status condition's type relates to health.
type condPolarity int

const (
	condInfo    condPolarity = iota // neutral marker (Issuing, Progressing, Reconciling)
	condReady                       // good when True, bad when False (Ready, Available, Synced)
	condError                       // bad when True / present (Error, Failed, Degraded, Stalled)
	condWarning                     // always amber (any "*Warning" type)
)

// conditionPolarities is a curated map of well-known custom-resource condition
// types to their polarity. It makes the supported CRs explicit and overrides
// the heuristic where the type name alone would misclassify (e.g. cert-manager
// "Issuing", whose False state is normal, not a failure). Keys are lowercase.
var conditionPolarities = map[string]condPolarity{
	// ArgoCD Application (status-less; the type's presence is the signal).
	"comparisonerror":         condError,
	"invalidspecerror":        condError,
	"unknownerror":            condError,
	"syncerror":               condError,
	"deletionerror":           condError,
	"sharedresourcewarning":   condWarning,
	"repeatedresourcewarning": condWarning,
	"excludedresourcewarning": condWarning,
	"orphanedresourcewarning": condWarning,
	// cert-manager.
	"ready":   condReady,
	"issuing": condInfo, // actively issuing; False is the normal idle state
	// external-secrets.
	"secretsynced": condReady,
	// FluxCD.
	"reconciling": condInfo,
	"stalled":     condError,
	"healthy":     condReady,
}

var (
	conditionErrorKeywords = []string{"error", "fail", "degrad", "invalid", "unhealthy", "pressure", "stalled", "lost", "backoff", "misconfig"}
	conditionReadyKeywords = []string{"ready", "available", "synced", "healthy", "succeeded", "complete", "established", "approved", "provisioned", "bound", "scheduled", "initialized"}
)

// conditionPolarity returns the polarity of a condition type, consulting the
// curated map first and falling back to a keyword heuristic.
func conditionPolarity(condType string) condPolarity {
	lower := strings.ToLower(condType)
	if p, ok := conditionPolarities[lower]; ok {
		return p
	}
	// Match keyword stems against whole camelCase tokens, not raw substrings,
	// so negated types do not invert: "Unbound" -> ["unbound"] does not start
	// with "bound", and "Incomplete" -> ["incomplete"] not with "complete".
	tokens := conditionTokens(condType)
	switch {
	case tokenHasPrefix(tokens, []string{"warn"}):
		return condWarning
	case tokenHasPrefix(tokens, conditionErrorKeywords):
		return condError
	case tokenHasPrefix(tokens, conditionReadyKeywords):
		return condReady
	default:
		return condInfo
	}
}

// conditionTokens splits a PascalCase/camelCase condition type into lowercase
// word tokens (e.g. "ContainersReady" -> ["containers", "ready"]). Runs of
// non-letters and case transitions are token boundaries.
func conditionTokens(condType string) []string {
	var tokens []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			tokens = append(tokens, strings.ToLower(string(cur)))
			cur = nil
		}
	}
	for _, r := range condType {
		switch {
		case r >= 'A' && r <= 'Z':
			flush()
			cur = append(cur, r)
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			cur = append(cur, r)
		default:
			flush()
		}
	}
	flush()
	return tokens
}

// tokenHasPrefix reports whether any token starts with one of the keyword stems.
func tokenHasPrefix(tokens, keywords []string) bool {
	for _, tok := range tokens {
		for _, kw := range keywords {
			if strings.HasPrefix(tok, kw) {
				return true
			}
		}
	}
	return false
}

// ConditionStyle returns the color style for a status condition, combining its
// type's polarity with its status value. Conditions without a True/False status
// (e.g. ArgoCD application conditions) are colored by type polarity alone.
func ConditionStyle(condType, status string) lipgloss.Style {
	if status == "Unknown" {
		return DimStyle
	}
	switch conditionPolarity(condType) {
	case condWarning:
		return StatusWarning
	case condError:
		if status == "False" {
			return StatusRunning // a failure condition that is False = healthy
		}
		return StatusFailed // True or status-less = problem
	case condReady:
		if status == "False" {
			return StatusFailed // a readiness condition that is False = problem
		}
		return StatusRunning // True or status-less = healthy
	default: // condInfo
		if status == "True" {
			return StatusProgressing // active / in progress
		}
		return DimStyle // False or status-less = neutral
	}
}
