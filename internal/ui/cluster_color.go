package ui

import "github.com/charmbracelet/lipgloss"

// ClusterColorNames lists every colour name accepted by saveClusterColors and
// rendered by ClusterColorTitleBarStyle. Order is the canonical order used in
// the picker overlay (top-to-bottom).
//
// We deliberately use ANSI named colours instead of theme tokens or hex
// values: ANSI maps to the user's terminal palette so a "red" cluster looks
// red in every terminal scheme (Solarized, Tokyo Night, etc.) while staying
// recognisable across dark and light backgrounds. The bright variants
// (ANSI 9–15) keep the tint loud enough that "I'm in prod" is unmissable
// even when the title bar shares the row with other badges.
var ClusterColorNames = []string{
	"red",
	"yellow",
	"green",
	"blue",
	"magenta",
	"cyan",
	"white",
	"gray",
}

// ansiCodeForClusterColor maps each named colour to its bright ANSI code.
// Bright variants stand out against typical dark themes; gray uses ANSI 8
// (bright black) so a "neutral" cluster has a distinct but quiet badge.
var ansiCodeForClusterColor = map[string]string{
	"red":     "9",
	"yellow":  "11",
	"green":   "10",
	"blue":    "12",
	"magenta": "13",
	"cyan":    "14",
	"white":   "15",
	"gray":    "8",
}

// IsValidClusterColor reports whether the given name is one of the named
// colours that the persistence layer is allowed to store. Empty string is
// the sentinel for "no colour assigned" and is rejected here — callers must
// treat the absence of a key in the colors map as the unset state instead.
func IsValidClusterColor(name string) bool {
	_, ok := ansiCodeForClusterColor[name]
	return ok
}

// ClusterColorTitleBarStyle returns a lipgloss style for tinting the title
// bar background to the named colour. Foreground is forced to black so text
// stays legible on every bright background. Empty / unknown name returns
// the zero style so the caller can compose unconditionally.
func ClusterColorTitleBarStyle(name string) lipgloss.Style {
	code, ok := ansiCodeForClusterColor[name]
	if !ok {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().
		Background(lipgloss.Color(code)).
		Foreground(lipgloss.Color("0")). // ANSI black for contrast on every bright bg
		Bold(true)
}

// ClusterColorSwatch returns a 2-cell coloured block used inside the
// picker overlay rows where the surrounding style sets only a foreground
// (no competing background). Empty / unknown name returns two
// dim-coloured cells so rows without a colour stay aligned with rows
// that have one.
func ClusterColorSwatch(name string) string {
	code, ok := ansiCodeForClusterColor[name]
	if !ok {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDimmed)).Render("··")
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(code)).Render("██")
}

// ClusterColorSwatchBg returns a 2-cell coloured block rendered as a
// background tint on whitespace, intended for use inside the cluster
// picker rows where the row may be wrapped in a selection-highlight
// style. Background-as-colour wins over the outer style's foreground,
// so the colour stays visible whether or not the row is selected.
//
// Empty / unknown name returns two regular spaces (no background), so
// rows without a colour add no visual noise to the right edge.
func ClusterColorSwatchBg(name string) string {
	code, ok := ansiCodeForClusterColor[name]
	if !ok {
		return "  "
	}
	return lipgloss.NewStyle().Background(lipgloss.Color(code)).Render("  ")
}
