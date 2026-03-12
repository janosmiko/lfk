package ui

import "sort"

// BuiltinSchemes returns all built-in color schemes keyed by display name.
func BuiltinSchemes() map[string]Theme {
	return map[string]Theme{
		// --- Tokyonight family ---
		"tokyonight": {
			Primary:    "#7aa2f7",
			Secondary:  "#9ece6a",
			Text:       "#c0caf5",
			SelectedFg: "#1a1b26",
			SelectedBg: "#7aa2f7",
			Border:     "#3b4261",
			Dimmed:     "#565f89",
			Error:      "#f7768e",
			Warning:    "#e0af68",
			Purple:     "#bb9af7",
			Base:       "#1a1b26",
			BarBg:      "#24283b",
			Surface:    "#1f2335",
		},
		"tokyonight-storm": {
			Primary:    "#7aa2f7",
			Secondary:  "#9ece6a",
			Text:       "#c0caf5",
			SelectedFg: "#24283b",
			SelectedBg: "#7aa2f7",
			Border:     "#3b4261",
			Dimmed:     "#565f89",
			Error:      "#f7768e",
			Warning:    "#e0af68",
			Purple:     "#bb9af7",
			Base:       "#24283b",
			BarBg:      "#292e42",
			Surface:    "#24283b",
		},
		"tokyonight-day": {
			Primary:    "#2e7de9",
			Secondary:  "#587539",
			Text:       "#3760bf",
			SelectedFg: "#e1e2e7",
			SelectedBg: "#2e7de9",
			Border:     "#a8aecb",
			Dimmed:     "#6172b0",
			Error:      "#f52a65",
			Warning:    "#8c6c3e",
			Purple:     "#9854f1",
			Base:       "#e1e2e7",
			BarBg:      "#d0d5e3",
			Surface:    "#d4d6e4",
		},

		// --- Kanagawa family ---
		"kanagawa-wave": {
			Primary:    "#7e9cd8",
			Secondary:  "#98bb6c",
			Text:       "#dcd7ba",
			SelectedFg: "#1f1f28",
			SelectedBg: "#7e9cd8",
			Border:     "#54546d",
			Dimmed:     "#727169",
			Error:      "#e82424",
			Warning:    "#e6c384",
			Purple:     "#957fb8",
			Base:       "#1f1f28",
			BarBg:      "#2a2a37",
			Surface:    "#223249",
		},
		"kanagawa-dragon": {
			Primary:    "#7fb4ca",
			Secondary:  "#87a987",
			Text:       "#c5c9c5",
			SelectedFg: "#181616",
			SelectedBg: "#7fb4ca",
			Border:     "#504945",
			Dimmed:     "#737c73",
			Error:      "#c4746e",
			Warning:    "#c4b28a",
			Purple:     "#8992a7",
			Base:       "#181616",
			BarBg:      "#212121",
			Surface:    "#1d1c19",
		},

		// --- Bluloco family ---
		"bluloco-dark": {
			Primary:    "#3691ff",
			Secondary:  "#3fc56b",
			Text:       "#abb2bf",
			SelectedFg: "#282c34",
			SelectedBg: "#3691ff",
			Border:     "#444c56",
			Dimmed:     "#636d83",
			Error:      "#ff6480",
			Warning:    "#f9c859",
			Purple:     "#ce9887",
			Base:       "#282c34",
			BarBg:      "#2c313a",
			Surface:    "#21252b",
		},
		"bluloco-light": {
			Primary:    "#275fe4",
			Secondary:  "#23974a",
			Text:       "#383a42",
			SelectedFg: "#f9f9f9",
			SelectedBg: "#275fe4",
			Border:     "#c8c8c8",
			Dimmed:     "#7a82da",
			Error:      "#d52753",
			Warning:    "#df631c",
			Purple:     "#823ff1",
			Base:       "#f9f9f9",
			BarBg:      "#eaeaeb",
			Surface:    "#f0f0f0",
		},

		// --- Nord ---
		"nord": {
			Primary:    "#88c0d0",
			Secondary:  "#a3be8c",
			Text:       "#d8dee9",
			SelectedFg: "#2e3440",
			SelectedBg: "#88c0d0",
			Border:     "#4c566a",
			Dimmed:     "#616e88",
			Error:      "#bf616a",
			Warning:    "#ebcb8b",
			Purple:     "#b48ead",
			Base:       "#2e3440",
			BarBg:      "#3b4252",
			Surface:    "#353b49",
		},

		// --- Gruvbox family ---
		"gruvbox-dark": {
			Primary:    "#458588",
			Secondary:  "#98971a",
			Text:       "#ebdbb2",
			SelectedFg: "#282828",
			SelectedBg: "#458588",
			Border:     "#504945",
			Dimmed:     "#928374",
			Error:      "#cc241d",
			Warning:    "#d79921",
			Purple:     "#b16286",
			Base:       "#282828",
			BarBg:      "#3c3836",
			Surface:    "#32302f",
		},
		"gruvbox-light": {
			Primary:    "#076678",
			Secondary:  "#79740e",
			Text:       "#3c3836",
			SelectedFg: "#fbf1c7",
			SelectedBg: "#076678",
			Border:     "#bdae93",
			Dimmed:     "#928374",
			Error:      "#9d0006",
			Warning:    "#b57614",
			Purple:     "#8f3f71",
			Base:       "#fbf1c7",
			BarBg:      "#ebdbb2",
			Surface:    "#f2e5bc",
		},

		// --- Dracula ---
		"dracula": {
			Primary:    "#8be9fd",
			Secondary:  "#50fa7b",
			Text:       "#f8f8f2",
			SelectedFg: "#282a36",
			SelectedBg: "#8be9fd",
			Border:     "#44475a",
			Dimmed:     "#6272a4",
			Error:      "#ff5555",
			Warning:    "#f1fa8c",
			Purple:     "#bd93f9",
			Base:       "#282a36",
			BarBg:      "#343746",
			Surface:    "#2d2f3f",
		},

		// --- Catppuccin family ---
		"catppuccin-mocha": {
			Primary:    "#89b4fa",
			Secondary:  "#a6e3a1",
			Text:       "#cdd6f4",
			SelectedFg: "#1e1e2e",
			SelectedBg: "#89b4fa",
			Border:     "#45475a",
			Dimmed:     "#6c7086",
			Error:      "#f38ba8",
			Warning:    "#f9e2af",
			Purple:     "#cba6f7",
			Base:       "#1e1e2e",
			BarBg:      "#313244",
			Surface:    "#292940",
		},
		"catppuccin-macchiato": {
			Primary:    "#8aadf4",
			Secondary:  "#a6da95",
			Text:       "#cad3f5",
			SelectedFg: "#24273a",
			SelectedBg: "#8aadf4",
			Border:     "#494d64",
			Dimmed:     "#6e738d",
			Error:      "#ed8796",
			Warning:    "#eed49f",
			Purple:     "#c6a0f6",
			Base:       "#24273a",
			BarBg:      "#363a4f",
			Surface:    "#2e3247",
		},
		"catppuccin-frappe": {
			Primary:    "#8caaee",
			Secondary:  "#a6d189",
			Text:       "#c6d0f5",
			SelectedFg: "#303446",
			SelectedBg: "#8caaee",
			Border:     "#51576d",
			Dimmed:     "#737994",
			Error:      "#e78284",
			Warning:    "#e5c890",
			Purple:     "#ca9ee6",
			Base:       "#303446",
			BarBg:      "#414559",
			Surface:    "#393d52",
		},
		"catppuccin-latte": {
			Primary:    "#1e66f5",
			Secondary:  "#40a02b",
			Text:       "#4c4f69",
			SelectedFg: "#eff1f5",
			SelectedBg: "#1e66f5",
			Border:     "#bcc0cc",
			Dimmed:     "#8c8fa1",
			Error:      "#d20f39",
			Warning:    "#df8e1d",
			Purple:     "#8839ef",
			Base:       "#eff1f5",
			BarBg:      "#dce0e8",
			Surface:    "#e6e9ef",
		},
	}
}

// lightSchemes is the set of schemes classified as light themes.
var lightSchemes = map[string]bool{
	"tokyonight-day":   true,
	"bluloco-light":    true,
	"gruvbox-light":    true,
	"catppuccin-latte": true,
}

// IsLightScheme returns true if the named scheme is a light theme.
func IsLightScheme(name string) bool {
	return lightSchemes[name]
}

// SchemeEntry represents a single entry in the grouped scheme list.
// If IsHeader is true, Name is the group label and not a selectable scheme.
type SchemeEntry struct {
	Name     string
	IsHeader bool
}

// GroupedSchemeEntries returns scheme entries grouped by dark/light with headers.
func GroupedSchemeEntries() []SchemeEntry {
	schemes := BuiltinSchemes()
	var dark, light []string
	for name := range schemes {
		if IsLightScheme(name) {
			light = append(light, name)
		} else {
			dark = append(dark, name)
		}
	}
	sort.Strings(dark)
	sort.Strings(light)

	entries := []SchemeEntry{{Name: "Dark Themes", IsHeader: true}}
	for _, n := range dark {
		entries = append(entries, SchemeEntry{Name: n})
	}
	entries = append(entries, SchemeEntry{Name: "Light Themes", IsHeader: true})
	for _, n := range light {
		entries = append(entries, SchemeEntry{Name: n})
	}
	return entries
}

// SchemeNames returns the sorted list of built-in color scheme names.
func SchemeNames() []string {
	schemes := BuiltinSchemes()
	names := make([]string, 0, len(schemes))
	for name := range schemes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
