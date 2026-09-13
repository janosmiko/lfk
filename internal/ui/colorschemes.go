package ui

import "sort"

//go:generate go run ../../cmd/themegen --input-dir=../../themes/ghostty --output=colorschemes.tsv

// BuiltinSchemes returns all built-in color schemes keyed by display name.
// All schemes are auto-generated from ghostty terminal themes.
func BuiltinSchemes() map[string]Theme {
	return generatedSchemes()
}

// IsLightScheme returns true if the named scheme is a light theme.
// Detection is based on background luminance during code generation.
func IsLightScheme(name string) bool {
	return generatedLightSchemes()[name]
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

	entries := make([]SchemeEntry, 0, 2+len(dark)+len(light))
	entries = append(entries, SchemeEntry{Name: "Dark Themes", IsHeader: true})
	for _, n := range dark {
		entries = append(entries, SchemeEntry{Name: n})
	}
	entries = append(entries, SchemeEntry{Name: "Light Themes", IsHeader: true})
	for _, n := range light {
		entries = append(entries, SchemeEntry{Name: n})
	}
	return entries
}
