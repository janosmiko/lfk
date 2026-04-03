package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- parseHex ---

func TestParseHex(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantR   uint8
		wantG   uint8
		wantB   uint8
		wantErr bool
	}{
		{name: "black with hash", input: "#000000", wantR: 0, wantG: 0, wantB: 0},
		{name: "black without hash", input: "000000", wantR: 0, wantG: 0, wantB: 0},
		{name: "white", input: "#ffffff", wantR: 255, wantG: 255, wantB: 255},
		{name: "red", input: "#ff0000", wantR: 255, wantG: 0, wantB: 0},
		{name: "green", input: "#00ff00", wantR: 0, wantG: 255, wantB: 0},
		{name: "blue", input: "#0000ff", wantR: 0, wantG: 0, wantB: 255},
		{name: "mixed color", input: "#1a2b3c", wantR: 0x1a, wantG: 0x2b, wantB: 0x3c},
		{name: "uppercase hex", input: "#AABBCC", wantR: 0xaa, wantG: 0xbb, wantB: 0xcc},
		{name: "too short", input: "#fff", wantErr: true},
		{name: "too long", input: "#1234567", wantErr: true},
		{name: "empty string", input: "", wantErr: true},
		{name: "invalid chars", input: "#gghhii", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, g, b, err := parseHex(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantR, r, "red channel")
			assert.Equal(t, tt.wantG, g, "green channel")
			assert.Equal(t, tt.wantB, b, "blue channel")
		})
	}
}

// --- mustParseHex ---

func TestMustParseHex(t *testing.T) {
	tests := []struct {
		name  string
		input string
		wantR uint8
		wantG uint8
		wantB uint8
	}{
		{name: "valid color", input: "#ff8000", wantR: 255, wantG: 128, wantB: 0},
		{name: "invalid falls back to gray", input: "xyz", wantR: 128, wantG: 128, wantB: 128},
		{name: "empty falls back to gray", input: "", wantR: 128, wantG: 128, wantB: 128},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, g, b := mustParseHex(tt.input)
			assert.Equal(t, tt.wantR, r)
			assert.Equal(t, tt.wantG, g)
			assert.Equal(t, tt.wantB, b)
		})
	}
}

// --- toHex ---

func TestToHex(t *testing.T) {
	tests := []struct {
		name string
		r    uint8
		g    uint8
		b    uint8
		want string
	}{
		{name: "black", r: 0, g: 0, b: 0, want: "#000000"},
		{name: "white", r: 255, g: 255, b: 255, want: "#ffffff"},
		{name: "red", r: 255, g: 0, b: 0, want: "#ff0000"},
		{name: "mixed", r: 0x1a, g: 0x2b, b: 0x3c, want: "#1a2b3c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, toHex(tt.r, tt.g, tt.b))
		})
	}
}

// --- luminance ---

func TestLuminance(t *testing.T) {
	tests := []struct {
		name string
		r    uint8
		g    uint8
		b    uint8
		low  float64
		high float64
	}{
		{name: "black is zero", r: 0, g: 0, b: 0, low: 0.0, high: 0.001},
		{name: "white is one", r: 255, g: 255, b: 255, low: 0.999, high: 1.001},
		{name: "pure red", r: 255, g: 0, b: 0, low: 0.20, high: 0.22},
		{name: "pure green", r: 0, g: 255, b: 0, low: 0.71, high: 0.72},
		{name: "pure blue", r: 0, g: 0, b: 255, low: 0.07, high: 0.08},
		{name: "mid gray above threshold", r: 200, g: 200, b: 200, low: 0.5, high: 0.7},
		{name: "dark gray below threshold", r: 50, g: 50, b: 50, low: 0.0, high: 0.05},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := luminance(tt.r, tt.g, tt.b)
			assert.GreaterOrEqual(t, l, tt.low)
			assert.LessOrEqual(t, l, tt.high)
		})
	}
}

// --- blendColor ---

func TestBlendColor(t *testing.T) {
	tests := []struct {
		name string
		c1   string
		c2   string
		t    float64
		want string
	}{
		{name: "t=0 returns c1", c1: "#ff0000", c2: "#0000ff", t: 0.0, want: "#ff0000"},
		{name: "t=1 returns c2", c1: "#ff0000", c2: "#0000ff", t: 1.0, want: "#0000ff"},
		{name: "t=0.5 midpoint", c1: "#000000", c2: "#ffffff", t: 0.5, want: "#7f7f7f"},
		{name: "blend red and green", c1: "#ff0000", c2: "#00ff00", t: 0.5, want: "#7f7f00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, blendColor(tt.c1, tt.c2, tt.t))
		})
	}
}

// --- lighten / darken ---

func TestLighten(t *testing.T) {
	// Lighten blends toward white.
	result := lighten("#000000", 1.0)
	assert.Equal(t, "#ffffff", result, "fully lightened black should be white")

	result = lighten("#000000", 0.0)
	assert.Equal(t, "#000000", result, "lighten by 0 should be unchanged")

	result = lighten("#ff0000", 0.5)
	assert.Equal(t, "#ff7f7f", result, "half-lighten red")
}

func TestDarken(t *testing.T) {
	// Darken blends toward black.
	result := darken("#ffffff", 1.0)
	assert.Equal(t, "#000000", result, "fully darkened white should be black")

	result = darken("#ffffff", 0.0)
	assert.Equal(t, "#ffffff", result, "darken by 0 should be unchanged")

	result = darken("#00ff00", 0.5)
	assert.Equal(t, "#007f00", result, "half-darken green")
}

// --- normalizeColor ---

func TestNormalizeColor(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "already normalized", input: "#aabbcc", want: "#aabbcc"},
		{name: "missing hash", input: "aabbcc", want: "#aabbcc"},
		{name: "uppercase", input: "#AABBCC", want: "#aabbcc"},
		{name: "uppercase no hash", input: "AABBCC", want: "#aabbcc"},
		{name: "with whitespace", input: "  #AABBCC  ", want: "#aabbcc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeColor(tt.input))
		})
	}
}

// --- normalizeName ---

func TestNormalizeName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "simple name", input: "Dracula", want: "dracula"},
		{name: "with extension", input: "Dracula.conf", want: "dracula"},
		{name: "underscores to dashes", input: "Solarized_Dark", want: "solarized-dark"},
		{name: "spaces to dashes", input: "Solarized Dark", want: "solarized-dark"},
		{name: "mixed separators", input: "My_Cool Theme.txt", want: "my-cool-theme"},
		{name: "already normalized", input: "gruvbox", want: "gruvbox"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeName(tt.input))
		})
	}
}

// --- parseConfigLine ---

func TestParseConfigLine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantKey string
		wantVal string
		wantOk  bool
	}{
		{name: "standard line", line: "background = #1a1b26", wantKey: "background", wantVal: "#1a1b26", wantOk: true},
		{name: "no spaces", line: "foreground=#c0caf5", wantKey: "foreground", wantVal: "#c0caf5", wantOk: true},
		{name: "palette entry", line: "palette = 0=#15161e", wantKey: "palette", wantVal: "0=#15161e", wantOk: true},
		{name: "no equals sign", line: "just-a-key", wantKey: "", wantVal: "", wantOk: false},
		{name: "extra spaces", line: "  key  =  value  ", wantKey: "key", wantVal: "value", wantOk: true},
		{name: "empty value", line: "key = ", wantKey: "key", wantVal: "", wantOk: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, val, ok := parseConfigLine(tt.line)
			assert.Equal(t, tt.wantOk, ok)
			if ok {
				assert.Equal(t, tt.wantKey, key)
				assert.Equal(t, tt.wantVal, val)
			}
		})
	}
}

// --- parseThemeFile ---

func TestParseThemeFile(t *testing.T) {
	t.Run("valid theme file", func(t *testing.T) {
		dir := t.TempDir()
		content := buildThemeFile("#1a1b26", "#c0caf5", fullPalette())
		path := filepath.Join(dir, "tokyo-night")
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		theme, err := parseThemeFile(path)
		require.NoError(t, err)
		assert.Equal(t, "#1a1b26", theme.Background)
		assert.Equal(t, "#c0caf5", theme.Foreground)
		assert.Equal(t, "#15161e", theme.Palette[0])
		assert.Equal(t, "#f7768e", theme.Palette[1])
	})

	t.Run("missing background", func(t *testing.T) {
		dir := t.TempDir()
		content := "foreground = #c0caf5\n"
		for i := range 16 {
			content += paletteEntry(i, fullPalette()[i])
		}
		path := filepath.Join(dir, "bad-theme")
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		_, err := parseThemeFile(path)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing background or foreground")
	})

	t.Run("missing foreground", func(t *testing.T) {
		dir := t.TempDir()
		content := "background = #1a1b26\n"
		for i := range 16 {
			content += paletteEntry(i, fullPalette()[i])
		}
		path := filepath.Join(dir, "no-fg")
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		_, err := parseThemeFile(path)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing background or foreground")
	})

	t.Run("incomplete palette", func(t *testing.T) {
		dir := t.TempDir()
		content := "background = #1a1b26\nforeground = #c0caf5\n"
		// Only 4 palette entries (need at least 8).
		for i := range 4 {
			content += paletteEntry(i, fullPalette()[i])
		}
		path := filepath.Join(dir, "short-palette")
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		_, err := parseThemeFile(path)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "incomplete palette")
	})

	t.Run("fills missing palette entries", func(t *testing.T) {
		dir := t.TempDir()
		// Provide only palette 0-7 (8 entries), missing 8-15.
		content := "background = #1a1b26\nforeground = #c0caf5\n"
		pal := fullPalette()
		for i := range 8 {
			content += paletteEntry(i, pal[i])
		}
		path := filepath.Join(dir, "half-palette")
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		theme, err := parseThemeFile(path)
		require.NoError(t, err)
		// Bright colors (8-15) should mirror normal (0-7).
		for i := 8; i < 16; i++ {
			assert.Equal(t, theme.Palette[i-8], theme.Palette[i],
				"palette[%d] should mirror palette[%d]", i, i-8)
		}
	})

	t.Run("skips comments and blank lines", func(t *testing.T) {
		dir := t.TempDir()
		content := "# This is a comment\n\n"
		content += buildThemeFile("#000000", "#ffffff", fullPalette())
		path := filepath.Join(dir, "with-comments")
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		theme, err := parseThemeFile(path)
		require.NoError(t, err)
		assert.Equal(t, "#000000", theme.Background)
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := parseThemeFile("/nonexistent/path/theme")
		assert.Error(t, err)
	})

	t.Run("invalid palette index is ignored", func(t *testing.T) {
		dir := t.TempDir()
		content := "background = #1a1b26\nforeground = #c0caf5\n"
		pal := fullPalette()
		for i := range 16 {
			content += paletteEntry(i, pal[i])
		}
		content += "palette = 99=#ffffff\n"  // out of range
		content += "palette = abc=#ffffff\n" // non-numeric
		content += "palette = badformat\n"   // missing =
		path := filepath.Join(dir, "bad-palette-idx")
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		theme, err := parseThemeFile(path)
		require.NoError(t, err)
		// Valid entries should still parse correctly.
		assert.Equal(t, "#15161e", theme.Palette[0])
	})
}

// --- mapToTheme ---

func TestMapToTheme(t *testing.T) {
	t.Run("dark theme", func(t *testing.T) {
		raw := rawTheme{
			Background: "#1a1b26",
			Foreground: "#c0caf5",
			Palette:    buildPaletteArray(fullPalette()),
		}

		result := mapToTheme(raw)
		assert.Equal(t, raw.Background, result.Background)
		assert.Equal(t, raw.Foreground, result.Foreground)
		assert.Equal(t, raw.Palette, result.Palette)
	})

	t.Run("light theme", func(t *testing.T) {
		pal := buildPaletteArray(fullPalette())
		raw := rawTheme{
			Background: "#f5f5f5",
			Foreground: "#333333",
			Palette:    pal,
		}

		result := mapToTheme(raw)
		assert.Equal(t, raw.Background, result.Background)
		assert.Equal(t, raw.Foreground, result.Foreground)
	})

	t.Run("border fallback when palette8 equals bg", func(t *testing.T) {
		pal := buildPaletteArray(fullPalette())
		pal[8] = "#1a1b26" // same as background
		raw := rawTheme{
			Background: "#1a1b26",
			Foreground: "#c0caf5",
			Palette:    pal,
		}

		result := mapToTheme(raw)
		// mapToTheme still returns the raw theme unchanged, but exercises the
		// border fallback code path internally.
		assert.Equal(t, raw.Background, result.Background)
	})

	t.Run("border fallback when palette8 equals fg", func(t *testing.T) {
		pal := buildPaletteArray(fullPalette())
		pal[8] = "#c0caf5" // same as foreground
		raw := rawTheme{
			Background: "#1a1b26",
			Foreground: "#c0caf5",
			Palette:    pal,
		}

		result := mapToTheme(raw)
		assert.Equal(t, raw.Foreground, result.Foreground)
	})
}

// --- themeFields ---

func TestThemeFields(t *testing.T) {
	t.Run("dark theme field mapping", func(t *testing.T) {
		pal := buildPaletteArray(fullPalette())
		raw := rawTheme{
			Background: "#1a1b26", // dark
			Foreground: "#c0caf5",
			Palette:    pal,
		}

		fields := themeFields(raw)

		assert.Equal(t, pal[4], fields[0], "Primary = palette[4]")
		assert.Equal(t, pal[2], fields[1], "Secondary = palette[2]")
		assert.Equal(t, raw.Foreground, fields[2], "Text = foreground")
		assert.Equal(t, raw.Background, fields[3], "SelectedFg = background")
		assert.Equal(t, pal[4], fields[4], "SelectedBg = palette[4]")
		assert.Equal(t, pal[8], fields[6], "Dimmed = palette[8]")
		assert.Equal(t, pal[1], fields[7], "Error = palette[1]")
		assert.Equal(t, pal[3], fields[8], "Warning = palette[3]")
		assert.Equal(t, pal[5], fields[9], "Purple = palette[5]")
		assert.Equal(t, raw.Background, fields[10], "Base = background")
	})

	t.Run("light theme derives lighter bar/surface", func(t *testing.T) {
		pal := buildPaletteArray(fullPalette())
		raw := rawTheme{
			Background: "#f5f5f5", // light
			Foreground: "#333333",
			Palette:    pal,
		}

		fields := themeFields(raw)
		// BarBg should be darker than background for light themes.
		assert.NotEqual(t, raw.Background, fields[11], "BarBg should differ from background")
		// Surface should differ from background.
		assert.NotEqual(t, raw.Background, fields[12], "Surface should differ from background")
	})

	t.Run("border falls back when matching bg", func(t *testing.T) {
		pal := buildPaletteArray(fullPalette())
		// Set palette[8] equal to background to trigger fallback.
		pal[8] = "#1a1b26"
		raw := rawTheme{
			Background: "#1a1b26",
			Foreground: "#c0caf5",
			Palette:    pal,
		}

		fields := themeFields(raw)
		// Border should not equal the background -- it falls back to blended.
		expected := blendColor(raw.Background, raw.Foreground, 0.25)
		assert.Equal(t, expected, fields[5], "border should use blended fallback")
	})

	t.Run("border falls back when matching fg", func(t *testing.T) {
		pal := buildPaletteArray(fullPalette())
		pal[8] = "#c0caf5" // same as foreground
		raw := rawTheme{
			Background: "#1a1b26",
			Foreground: "#c0caf5",
			Palette:    pal,
		}

		fields := themeFields(raw)
		expected := blendColor(raw.Background, raw.Foreground, 0.25)
		assert.Equal(t, expected, fields[5], "border should use blended fallback when matching fg")
	})
}

// --- run (integration) ---

func TestRun(t *testing.T) {
	t.Run("processes valid theme directory", func(t *testing.T) {
		inputDir := t.TempDir()
		outputDir := t.TempDir()
		outPath := filepath.Join(outputDir, "schemes.go")

		// Create two valid theme files.
		writeThemeFixture(t, inputDir, "dracula", "#282a36", "#f8f8f2")
		writeThemeFixture(t, inputDir, "gruvbox", "#282828", "#ebdbb2")

		err := run(inputDir, outPath, "")
		require.NoError(t, err)

		data, err := os.ReadFile(outPath)
		require.NoError(t, err)
		content := string(data)
		assert.Contains(t, content, `"dracula"`)
		assert.Contains(t, content, `"gruvbox"`)
	})

	t.Run("skips themes in skip list", func(t *testing.T) {
		inputDir := t.TempDir()
		outputDir := t.TempDir()
		outPath := filepath.Join(outputDir, "schemes.go")

		writeThemeFixture(t, inputDir, "dracula", "#282a36", "#f8f8f2")
		writeThemeFixture(t, inputDir, "gruvbox", "#282828", "#ebdbb2")

		err := run(inputDir, outPath, "dracula")
		require.NoError(t, err)

		data, err := os.ReadFile(outPath)
		require.NoError(t, err)
		content := string(data)
		assert.NotContains(t, content, `"dracula"`)
		assert.Contains(t, content, `"gruvbox"`)
	})

	t.Run("skips subdirectories", func(t *testing.T) {
		inputDir := t.TempDir()
		outputDir := t.TempDir()
		outPath := filepath.Join(outputDir, "schemes.go")

		writeThemeFixture(t, inputDir, "valid-theme", "#000000", "#ffffff")
		require.NoError(t, os.Mkdir(filepath.Join(inputDir, "subdir"), 0o755))

		err := run(inputDir, outPath, "")
		require.NoError(t, err)

		data, err := os.ReadFile(outPath)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"valid-theme"`)
	})

	t.Run("handles invalid theme files gracefully", func(t *testing.T) {
		inputDir := t.TempDir()
		outputDir := t.TempDir()
		outPath := filepath.Join(outputDir, "schemes.go")

		// One valid, one invalid (missing palette).
		writeThemeFixture(t, inputDir, "good-theme", "#000000", "#ffffff")
		require.NoError(t, os.WriteFile(
			filepath.Join(inputDir, "bad-theme"),
			[]byte("background = #000000\nforeground = #ffffff\n"),
			0o644,
		))

		err := run(inputDir, outPath, "")
		require.NoError(t, err)

		data, err := os.ReadFile(outPath)
		require.NoError(t, err)
		content := string(data)
		assert.Contains(t, content, `"good-theme"`)
		assert.NotContains(t, content, `"bad-theme"`)
	})

	t.Run("deduplicates names", func(t *testing.T) {
		inputDir := t.TempDir()
		outputDir := t.TempDir()
		outPath := filepath.Join(outputDir, "schemes.go")

		// Two files that normalize to the same name.
		writeThemeFixture(t, inputDir, "My_Theme", "#000000", "#ffffff")
		writeThemeFixture(t, inputDir, "my-theme.txt", "#111111", "#eeeeee")

		err := run(inputDir, outPath, "")
		require.NoError(t, err)

		data, err := os.ReadFile(outPath)
		require.NoError(t, err)
		// Should only appear once.
		assert.Equal(t, 1, strings.Count(string(data), `"my-theme"`))
	})

	t.Run("error on nonexistent input dir", func(t *testing.T) {
		err := run("/nonexistent/dir", "/dev/null", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "reading input directory")
	})

	t.Run("light theme detection", func(t *testing.T) {
		inputDir := t.TempDir()
		outputDir := t.TempDir()
		outPath := filepath.Join(outputDir, "schemes.go")

		writeThemeFixture(t, inputDir, "light-theme", "#f5f5f5", "#333333")

		err := run(inputDir, outPath, "")
		require.NoError(t, err)

		data, err := os.ReadFile(outPath)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"light-theme": true`)
	})

	t.Run("multiple skip entries with spaces", func(t *testing.T) {
		inputDir := t.TempDir()
		outputDir := t.TempDir()
		outPath := filepath.Join(outputDir, "schemes.go")

		writeThemeFixture(t, inputDir, "alpha", "#000000", "#ffffff")
		writeThemeFixture(t, inputDir, "beta", "#111111", "#eeeeee")
		writeThemeFixture(t, inputDir, "gamma", "#222222", "#dddddd")

		err := run(inputDir, outPath, "alpha, beta")
		require.NoError(t, err)

		data, err := os.ReadFile(outPath)
		require.NoError(t, err)
		content := string(data)
		assert.NotContains(t, content, `"alpha"`)
		assert.NotContains(t, content, `"beta"`)
		assert.Contains(t, content, `"gamma"`)
	})
}

// writeThemeFixture creates a valid ghostty theme file in the given directory.
func writeThemeFixture(t *testing.T, dir, name, bg, fg string) {
	t.Helper()
	content := buildThemeFile(bg, fg, fullPalette())
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
}

// --- writeOutput ---

func TestWriteOutput(t *testing.T) {
	t.Run("generates valid go file", func(t *testing.T) {
		dir := t.TempDir()
		outPath := filepath.Join(dir, "out.go")

		pal := buildPaletteArray(fullPalette())
		themes := []themeEntry{
			{
				Name:    "test-dark",
				Theme:   rawTheme{Background: "#1a1b26", Foreground: "#c0caf5", Palette: pal},
				IsLight: false,
			},
			{
				Name:    "test-light",
				Theme:   rawTheme{Background: "#f5f5f5", Foreground: "#333333", Palette: pal},
				IsLight: true,
			},
		}

		err := writeOutput(outPath, themes, 2, 1)
		require.NoError(t, err)

		data, err := os.ReadFile(outPath)
		require.NoError(t, err)
		content := string(data)

		assert.Contains(t, content, "DO NOT EDIT")
		assert.Contains(t, content, "package ui")
		assert.Contains(t, content, "generatedSchemes")
		assert.Contains(t, content, `"test-dark"`)
		assert.Contains(t, content, `"test-light"`)
		assert.Contains(t, content, "generatedLightSchemes")
		// Only the light theme should appear in light schemes.
		assert.Contains(t, content, `"test-light": true`)
	})

	t.Run("empty themes list", func(t *testing.T) {
		dir := t.TempDir()
		outPath := filepath.Join(dir, "empty.go")

		err := writeOutput(outPath, nil, 0, 0)
		require.NoError(t, err)

		data, err := os.ReadFile(outPath)
		require.NoError(t, err)
		assert.Contains(t, string(data), "package ui")
	})

	t.Run("error on invalid path", func(t *testing.T) {
		err := writeOutput("/nonexistent/dir/file.go", nil, 0, 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "creating output file")
	})
}

// --- Roundtrip: parseHex -> toHex ---

func TestParseHexToHexRoundtrip(t *testing.T) {
	colors := []string{"#000000", "#ffffff", "#1a2b3c", "#ff8800", "#abcdef"}
	for _, c := range colors {
		r, g, b, err := parseHex(c)
		require.NoError(t, err)
		assert.Equal(t, c, toHex(r, g, b), "roundtrip should preserve color %s", c)
	}
}

// --- Test helpers ---

func fullPalette() [16]string {
	return [16]string{
		"#15161e", // 0 black
		"#f7768e", // 1 red
		"#9ece6a", // 2 green
		"#e0af68", // 3 yellow
		"#7aa2f7", // 4 blue
		"#bb9af7", // 5 magenta
		"#7dcfff", // 6 cyan
		"#a9b1d6", // 7 white
		"#414868", // 8 bright black
		"#f7768e", // 9 bright red
		"#9ece6a", // 10 bright green
		"#e0af68", // 11 bright yellow
		"#7aa2f7", // 12 bright blue
		"#bb9af7", // 13 bright magenta
		"#7dcfff", // 14 bright cyan
		"#c0caf5", // 15 bright white
	}
}

func buildPaletteArray(pal [16]string) [16]string {
	return pal
}

func paletteEntry(idx int, color string) string {
	// Remove the '#' prefix for the palette value since normalizeColor adds it back.
	c := strings.TrimPrefix(color, "#")
	return "palette = " + string(rune('0'+idx/10)) + string(rune('0'+idx%10)) + "=" + c + "\n"
}

func buildThemeFile(bg, fg string, palette [16]string) string {
	var b strings.Builder
	b.WriteString("background = " + strings.TrimPrefix(bg, "#") + "\n")
	b.WriteString("foreground = " + strings.TrimPrefix(fg, "#") + "\n")
	for i, c := range palette {
		b.WriteString(paletteEntry(i, c))
	}
	return b.String()
}
