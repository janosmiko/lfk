package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

func TestLoadConfig_NoColor(t *testing.T) {
	origNoColor := ConfigNoColor
	t.Cleanup(func() {
		ConfigNoColor = origNoColor
		ApplyTheme(DefaultTheme())
	})

	tests := []struct {
		name string
		yaml string
		want bool
	}{
		{"no_color: true", "no_color: true\n", true},
		{"no_color: false", "no_color: false\n", false},
		{"no_color unset uses default false", "colorscheme: dracula\n", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ConfigNoColor = false

			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(path, []byte(tc.yaml), 0o600); err != nil {
				t.Fatal(err)
			}
			LoadConfig(path)
			assert.Equal(t, tc.want, ConfigNoColor)
		})
	}
}

func TestSetNoColor_TogglesAndRebuildsStyles(t *testing.T) {
	origNoColor := ConfigNoColor
	t.Cleanup(func() {
		SetNoColor(origNoColor)
	})

	// Switch to no-color mode.
	SetNoColor(true)
	assert.True(t, ConfigNoColor)

	// In no-color mode, every Color* theme slot is blanked so inline
	// lipgloss.Color(ColorX) calls throughout the codebase produce
	// NoColor{} and emit no color SGR.
	assert.Empty(t, ColorPrimary, "Color* slots must be blanked in no-color mode")
	assert.Empty(t, ColorSecondary)
	assert.Empty(t, ColorError)
	assert.Empty(t, ColorWarning)
	assert.Empty(t, ColorCyan)

	// Inline styles that reference the theme color slots must emit no
	// color SGR because lipgloss.Color("") returns NoColor{}.
	inline := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorError)).
		Background(lipgloss.Color(ColorWarning)).
		Render("age")
	assert.NotContains(t, inline, "38;", "no-color must not emit fg color codes; got: %q", inline)
	assert.NotContains(t, inline, "48;", "no-color must not emit bg color codes; got: %q", inline)

	// SelectedStyle preserves visibility via Bold+Reverse SGR attributes.
	// These are emitted under the ANSI profile that no-color mode forces.
	selected := SelectedStyle.Render("row")
	hasReverse := strings.Contains(selected, "\x1b[7m") ||
		strings.Contains(selected, "\x1b[7;") ||
		strings.Contains(selected, ";7m") ||
		strings.Contains(selected, ";7;")
	assert.True(t, hasReverse,
		"SelectedStyle must emit Reverse SGR in no-color mode; got: %q", selected)

	// Toggle back to verify state is restored.
	SetNoColor(false)
	assert.False(t, ConfigNoColor)
	assert.Equal(t, defaultColorPrimary, ColorPrimary,
		"Color* slots must be restored when leaving no-color mode")
	assert.Equal(t, defaultColorError, ColorError)
}

func TestLoadConfig_NoColor_Default(t *testing.T) {
	origNoColor := ConfigNoColor
	t.Cleanup(func() {
		ConfigNoColor = origNoColor
		ApplyTheme(DefaultTheme())
	})
	ConfigNoColor = false

	// Default: no config, no CLI, no env.
	t.Setenv("NO_COLOR", "")
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.yaml")
	LoadConfig(path)
	assert.False(t, ConfigNoColor, "no NO_COLOR env, no config → color enabled")
}

func TestNoColor_StripsAgeStyleColors(t *testing.T) {
	origNoColor := ConfigNoColor
	t.Cleanup(func() { SetNoColor(origNoColor) })

	// Build an age-style inline style (mimics styles.go:372 — "very new"
	// resources use Cyan foreground). Verify no color SGR is emitted when
	// no-color mode is active, and that the same style DOES emit color
	// when we leave no-color mode.
	ageStyle := func() lipgloss.Style {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorCyan)).
			Background(BaseBg)
	}

	SetNoColor(true)
	rendered := ageStyle().Render("42s")
	assert.NotContains(t, rendered, "38;",
		"no-color must strip fg color from age cell; got: %q", rendered)
	assert.NotContains(t, rendered, "48;",
		"no-color must strip bg color from age cell; got: %q", rendered)

	// Also a percentage-style cell (uses ColorError for >=90%).
	pctStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorError)).
		Bold(true).
		Background(BaseBg).
		Render("95%")
	assert.NotContains(t, pctStyle, "38;", "no-color must strip fg from pct cell; got: %q", pctStyle)

	// And a monitoring-severity inline style (raw hex via ThemeColor helper).
	errStyle := lipgloss.NewStyle().
		Foreground(ThemeColor("#ff5555")).
		Bold(true).
		Render("ERROR")
	assert.NotContains(t, errStyle, "38;",
		"no-color must strip fg color from ThemeColor hex style; got: %q", errStyle)

	// ANSI numeric specs must also be stripped (spinner uses "62", check
	// marks use "2"). These bypass the Color* slots, so ThemeColor is the
	// only thing standing between them and leaked SGR.
	ansiNumStyle := lipgloss.NewStyle().
		Foreground(ThemeColor("2")).
		Render("\u2713")
	assert.NotContains(t, ansiNumStyle, "\x1b[",
		"no-color must strip ANSI numeric ThemeColor style; got: %q", ansiNumStyle)
	spinnerStyle := lipgloss.NewStyle().
		Foreground(ThemeColor("62")).
		Render("spinner")
	assert.NotContains(t, spinnerStyle, "\x1b[",
		"no-color must strip ANSI256 numeric ThemeColor style; got: %q", spinnerStyle)

	SetNoColor(false)
	// After leaving no-color mode the same style should emit color (proof
	// that the defaults were restored, not permanently blanked).
	afterRestore := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorError)).
		Render("err")
	// Under the test runner's Ascii profile (no TTY) the renderer still
	// strips SGR, so we only check the Color* variable itself was restored.
	assert.Equal(t, defaultColorError, ColorError)
	_ = afterRestore
}

func TestLoadConfig_NoColor_EnvVarOverride(t *testing.T) {
	origNoColor := ConfigNoColor
	t.Cleanup(func() {
		ConfigNoColor = origNoColor
		ApplyTheme(DefaultTheme())
	})

	tests := []struct {
		name   string
		envVal string
		yaml   string
		want   bool
	}{
		{"NO_COLOR=1 forces true", "1", "", true},
		{"NO_COLOR=yes forces true", "yes", "", true},
		{"NO_COLOR=anything forces true", "anything", "no_color: false\n", true},
		{"NO_COLOR empty respects config false", "", "no_color: false\n", false},
		{"NO_COLOR empty respects config true", "", "no_color: true\n", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ConfigNoColor = false
			t.Setenv("NO_COLOR", tc.envVal)

			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(path, []byte(tc.yaml), 0o600); err != nil {
				t.Fatal(err)
			}
			LoadConfig(path)
			assert.Equal(t, tc.want, ConfigNoColor)
		})
	}
}
