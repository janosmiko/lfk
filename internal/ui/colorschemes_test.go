package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuiltinSchemes(t *testing.T) {
	schemes := BuiltinSchemes()
	assert.NotEmpty(t, schemes)

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

func TestBuiltinSchemesContainExpectedThemes(t *testing.T) {
	schemes := BuiltinSchemes()
	expected := []string{
		"tokyonight", "tokyonight-storm", "tokyonight-day",
		"kanagawa-wave", "kanagawa-dragon",
		"bluloco-dark", "bluloco-light",
		"nord",
		"gruvbox-dark", "gruvbox-light",
		"dracula",
		"catppuccin-mocha", "catppuccin-macchiato", "catppuccin-frappe", "catppuccin-latte",
	}
	for _, name := range expected {
		_, ok := schemes[name]
		assert.True(t, ok, "missing expected scheme: %s", name)
	}
}

func TestIsLightScheme(t *testing.T) {
	// Known light schemes.
	assert.True(t, IsLightScheme("tokyonight-day"))
	assert.True(t, IsLightScheme("bluloco-light"))
	assert.True(t, IsLightScheme("gruvbox-light"))
	assert.True(t, IsLightScheme("catppuccin-latte"))

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

