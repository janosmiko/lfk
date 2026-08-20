package ui

import (
	"image/color"
	"slices"

	"charm.land/lipgloss/v2"
)

// ClusterColorNames lists every colour name accepted by saveClusterColors and
// rendered by ClusterColorTitleBarStyle. Order is the canonical order used in
// the picker overlay (top-to-bottom).
//
// Four of the names are theme-mapped (red / yellow / green / blue → the
// existing Error / Warning / Secondary / Primary theme tokens) so they
// adapt as the user switches lfk colorschemes. The remaining four are
// palette-relative ANSI bright codes — recognisable across every
// terminal scheme but unaffected by lfk's theme.
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

// ansiCodeForClusterColor holds the ANSI bright codes for the four
// non-theme-mapped colours. red/yellow/green/blue are intentionally
// absent — they resolve via clusterColorBg's theme path.
var ansiCodeForClusterColor = map[string]string{
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
	return slices.Contains(ClusterColorNames, name)
}

// clusterColorBg resolves the named colour to a lipgloss background.
// Theme-mapped names (red/yellow/green/blue) read directly from
// ActiveTheme so a colorscheme switch propagates on the next render —
// the package-level Color* slots are stuck on Tokyo Night defaults
// (they only branch between "default" and "no-color blanked" inside
// ApplyTheme. They never receive the active theme's actual values), so
// using them here would have left the cluster tints frozen on the
// boot-time palette regardless of which theme the user picked.
//
// The rest map to ANSI bright codes that follow the terminal palette.
// ThemeColor wraps both paths with the no-color check, so this also
// no-ops automatically when ConfigNoColor is set.
func clusterColorBg(name string) color.Color {
	switch name {
	case "red":
		return ThemeColor(ActiveTheme.Error)
	case "yellow":
		return ThemeColor(ActiveTheme.Warning)
	case "green":
		return ThemeColor(ActiveTheme.Secondary)
	case "blue":
		return ThemeColor(ActiveTheme.Primary)
	}
	if code, ok := ansiCodeForClusterColor[name]; ok {
		return ThemeColor(code)
	}
	return nil
}

// clusterColorFg picks a contrasting foreground for the named colour.
// Theme-mapped names get ActiveTheme.SelectedFg (designed to contrast
// with the theme's accent backgrounds). ANSI-mapped names get ANSI
// black, which is universally legible on every bright ANSI background.
func clusterColorFg(name string) color.Color {
	switch name {
	case "red", "yellow", "green", "blue":
		return ThemeColor(ActiveTheme.SelectedFg)
	case "magenta", "cyan", "white", "gray":
		return ThemeColor("0")
	}
	return nil
}

// ClusterColorTitleBarStyle returns a lipgloss style for tinting the title
// bar background to the named colour. Empty / unknown name returns
// the zero style so the caller can compose unconditionally.
func ClusterColorTitleBarStyle(name string) lipgloss.Style {
	bg := clusterColorBg(name)
	if bg == nil {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().
		Background(bg).
		Foreground(clusterColorFg(name)).
		Bold(true)
}

// ClusterColorSwatchBg returns a 2-cell coloured block rendered as a
// background tint on whitespace, intended for use inside rows that may
// be wrapped in a selection-highlight style. Background-as-colour wins
// over the outer style's foreground so the colour stays visible
// whether or not the row is selected.
//
// Empty / unknown name returns two regular spaces (no background), so
// rows without a colour add no visual noise.
func ClusterColorSwatchBg(name string) string {
	bg := clusterColorBg(name)
	if bg == nil {
		return "  "
	}
	return lipgloss.NewStyle().Background(bg).Render("  ")
}

// ClusterColorTileBg is the single-cell variant of ClusterColorSwatchBg used
// by the union-view row prefix: every union row gets a 1-cell colored tile
// at the leading edge so the source context is identifiable at a glance
// without scanning the textual Context column. One cell trades visibility
// for a smaller width tax than the 2-cell picker swatch — appropriate for
// per-row use across hundreds of rows.
//
// Empty / unknown name returns one regular space so rows without a colour
// stay aligned with rows that have one (the column-width calc subtracts a
// fixed tileW regardless of which rows actually have colours set).
func ClusterColorTileBg(name string) string {
	bg := clusterColorBg(name)
	if bg == nil {
		return " "
	}
	return lipgloss.NewStyle().Background(bg).Render(" ")
}

// ClusterColorTileBgOver is ClusterColorTileBg for a row wrapped by an outer
// style — specifically the cursor row's selection highlight. The colored
// tile ends with an SGR reset. Left alone, that reset cancels the outer
// style for every cell after the tile, so the cursor highlight vanishes for
// the rest of the row. Re-emitting the outer style's open codes right after
// the tile restores it. Mirrors the HighlightMatchStyledOver "restore"
// pattern.
//
// An uncolored tile is a plain space with no embedded reset, so it is
// returned unchanged — nothing needs restoring.
func ClusterColorTileBgOver(name string, outer lipgloss.Style) string {
	tile := ClusterColorTileBg(name)
	if clusterColorBg(name) == nil {
		return tile
	}
	return tile + styleOpenCodes(outer)
}
