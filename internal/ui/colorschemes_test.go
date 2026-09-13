package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuiltinSchemes(t *testing.T) {
	schemes := BuiltinSchemes()
	assert.NotEmpty(t, schemes)
	assert.Greater(t, len(schemes), 400, "should have 400+ generated themes")

	// Verify each scheme has all fields populated.
	for name, theme := range schemes {
		assert.NotEmpty(t, theme.Primary, "scheme %s missing Primary", name)
		assert.NotEmpty(t, theme.Secondary, "scheme %s missing Secondary", name)
		assert.NotEmpty(t, theme.Text, "scheme %s missing Text", name)
		assert.NotEmpty(t, theme.SelectedFg, "scheme %s missing SelectedFg", name)
		assert.NotEmpty(t, theme.SelectedBg, "scheme %s missing SelectedBg", name)
		assert.NotEmpty(t, theme.Border, "scheme %s missing Border", name)
		assert.NotEmpty(t, theme.Dimmed, "scheme %s missing Dimmed", name)
		assert.NotEmpty(t, theme.Error, "scheme %s missing Error", name)
		assert.NotEmpty(t, theme.Warning, "scheme %s missing Warning", name)
		assert.NotEmpty(t, theme.Purple, "scheme %s missing Purple", name)
		assert.NotEmpty(t, theme.Base, "scheme %s missing Base", name)
		assert.NotEmpty(t, theme.BarBg, "scheme %s missing BarBg", name)
		assert.NotEmpty(t, theme.Surface, "scheme %s missing Surface", name)
	}
}

func TestBuiltinSchemesSnapshot(t *testing.T) {
	schemes := BuiltinSchemes()
	assert.Len(t, schemes, 462)

	assert.Equal(t, Theme{
		Primary:    "#bd93f9",
		Secondary:  "#50fa7b",
		Text:       "#f8f8f2",
		SelectedFg: "#282a36",
		SelectedBg: "#bd93f9",
		Border:     "#6272a4",
		Dimmed:     "#6272a4",
		Error:      "#ff5555",
		Warning:    "#f1fa8c",
		Purple:     "#ff79c6",
		Base:       "#282a36",
		BarBg:      "#343642",
		Surface:    "#2e303c",
	}, schemes["dracula"])
	assert.False(t, IsLightScheme("dracula"))

	assert.Equal(t, Theme{
		Primary:    "#2e7de9",
		Secondary:  "#587539",
		Text:       "#3760bf",
		SelectedFg: "#e1e2e7",
		SelectedBg: "#2e7de9",
		Border:     "#a1a6c5",
		Dimmed:     "#a1a6c5",
		Error:      "#f52a65",
		Warning:    "#8c6c3e",
		Purple:     "#9854f1",
		Base:       "#e1e2e7",
		BarBg:      "#d3d4d9",
		Surface:    "#dadbe0",
	}, schemes["tokyonight-day"])
	assert.True(t, IsLightScheme("tokyonight-day"))

	assert.Equal(t, Theme{
		Primary:    "#89b4fa",
		Secondary:  "#a6e3a1",
		Text:       "#cdd6f4",
		SelectedFg: "#1e1e2e",
		SelectedBg: "#89b4fa",
		Border:     "#585b70",
		Dimmed:     "#585b70",
		Error:      "#f38ba8",
		Warning:    "#f9e2af",
		Purple:     "#f5c2e7",
		Base:       "#1e1e2e",
		BarBg:      "#2b2b3a",
		Surface:    "#242434",
	}, schemes["catppuccin-mocha"])
	assert.False(t, IsLightScheme("catppuccin-mocha"))
}

func TestBuiltinSchemesContainExpectedThemes(t *testing.T) {
	schemes := BuiltinSchemes()
	expected := []string{
		"tokyonight", "tokyonight-storm", "tokyonight-day",
		"dracula", "nord",
		"gruvbox-dark", "gruvbox-light",
		"catppuccin-mocha", "catppuccin-latte",
		"rose-pine", "rose-pine-moon", "rose-pine-dawn",
	}
	for _, name := range expected {
		_, ok := schemes[name]
		assert.True(t, ok, "missing expected scheme: %s", name)
	}
}

func TestIsLightScheme(t *testing.T) {
	// Known light schemes (detected by background luminance).
	assert.True(t, IsLightScheme("tokyonight-day"))
	assert.True(t, IsLightScheme("gruvbox-light"))
	assert.True(t, IsLightScheme("catppuccin-latte"))
	assert.True(t, IsLightScheme("rose-pine-dawn"))

	// Known dark schemes.
	assert.False(t, IsLightScheme("tokyonight"))
	assert.False(t, IsLightScheme("dracula"))
	assert.False(t, IsLightScheme("nord"))
	assert.False(t, IsLightScheme("catppuccin-mocha"))

	// Unknown scheme.
	assert.False(t, IsLightScheme("nonexistent"))
}

func TestGroupedSchemeEntries(t *testing.T) {
	entries := GroupedSchemeEntries()
	assert.NotEmpty(t, entries)

	// First entry should be "Dark Themes" header.
	assert.True(t, entries[0].IsHeader)
	assert.Equal(t, "Dark Themes", entries[0].Name)

	// Find "Light Themes" header.
	lightHeaderIdx := -1
	for i, e := range entries {
		if e.IsHeader && e.Name == "Light Themes" {
			lightHeaderIdx = i
			break
		}
	}
	assert.Greater(t, lightHeaderIdx, 0, "Light Themes header should exist")

	// All entries before light header (except dark header) should be dark schemes.
	for i := 1; i < lightHeaderIdx; i++ {
		if !entries[i].IsHeader {
			assert.False(t, IsLightScheme(entries[i].Name),
				"scheme %s before Light Themes header should be dark", entries[i].Name)
		}
	}

	// All entries after light header should be light schemes.
	for i := lightHeaderIdx + 1; i < len(entries); i++ {
		if !entries[i].IsHeader {
			assert.True(t, IsLightScheme(entries[i].Name),
				"scheme %s after Light Themes header should be light", entries[i].Name)
		}
	}
}
