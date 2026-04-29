package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unknownCSILike simulates the type bubbletea uses for unrecognized CSI
// sequences: a named []byte type. We replicate the shape here so tests don't
// depend on importing bubbletea's unexported type directly.
type unknownCSILike []byte

func TestParseColorModeMsg_Dark(t *testing.T) {
	msg := unknownCSILike("\x1b[?997;1n")
	dark, ok := ParseColorModeMsg(msg)
	assert.True(t, ok, "CSI ?997;1n should be recognized as a color-mode report")
	assert.True(t, dark, "CSI ?997;1n should report dark mode")
}

func TestParseColorModeMsg_Light(t *testing.T) {
	msg := unknownCSILike("\x1b[?997;2n")
	dark, ok := ParseColorModeMsg(msg)
	assert.True(t, ok, "CSI ?997;2n should be recognized as a color-mode report")
	assert.False(t, dark, "CSI ?997;2n should report light mode")
}

func TestParseColorModeMsg_Unrelated(t *testing.T) {
	cases := []struct {
		name string
		msg  any
	}{
		{"string", "hello"},
		{"int", 42},
		{"nil", nil},
		{"unknown CSI", unknownCSILike("\x1b[?997;3n")},
		{"empty", unknownCSILike("")},
		{"plain []byte", []byte("\x1b[?997;1n")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := ParseColorModeMsg(tc.msg)
			assert.False(t, ok, "msg %q should not be recognized as a color-mode report", tc.name)
		})
	}
}

func TestSetColorMode_Dark(t *testing.T) {
	orig := ActiveSchemeName
	origDark := ConfigDarkColorscheme
	origLight := ConfigLightColorscheme
	t.Cleanup(func() {
		ActiveSchemeName = orig
		ConfigDarkColorscheme = origDark
		ConfigLightColorscheme = origLight
		ApplyTheme(DefaultTheme())
	})

	ConfigDarkColorscheme = "tokyonight-storm"
	ConfigLightColorscheme = "tokyonight-day"

	SetColorMode(true)
	assert.Equal(t, "tokyonight-storm", ActiveSchemeName)
}

func TestSetColorMode_Light(t *testing.T) {
	orig := ActiveSchemeName
	origDark := ConfigDarkColorscheme
	origLight := ConfigLightColorscheme
	t.Cleanup(func() {
		ActiveSchemeName = orig
		ConfigDarkColorscheme = origDark
		ConfigLightColorscheme = origLight
		ApplyTheme(DefaultTheme())
	})

	ConfigDarkColorscheme = "tokyonight-storm"
	ConfigLightColorscheme = "tokyonight-day"

	SetColorMode(false)
	assert.Equal(t, "tokyonight-day", ActiveSchemeName)
}

func TestSetColorMode_NoConfig(t *testing.T) {
	orig := ActiveSchemeName
	origDark := ConfigDarkColorscheme
	origLight := ConfigLightColorscheme
	t.Cleanup(func() {
		ActiveSchemeName = orig
		ConfigDarkColorscheme = origDark
		ConfigLightColorscheme = origLight
	})

	ConfigDarkColorscheme = ""
	ConfigLightColorscheme = ""

	SetColorMode(true)
	assert.Equal(t, orig, ActiveSchemeName, "scheme should not change when neither dark nor light is configured")
}

func TestSetColorMode_UnknownScheme(t *testing.T) {
	orig := ActiveSchemeName
	origDark := ConfigDarkColorscheme
	t.Cleanup(func() {
		ActiveSchemeName = orig
		ConfigDarkColorscheme = origDark
	})

	ConfigDarkColorscheme = "nonexistent-scheme-xyz"
	SetColorMode(true)
	assert.Equal(t, orig, ActiveSchemeName, "scheme should not change for an unknown scheme name")
}

func TestColorModeEnabled(t *testing.T) {
	origDark := ConfigDarkColorscheme
	origLight := ConfigLightColorscheme
	t.Cleanup(func() {
		ConfigDarkColorscheme = origDark
		ConfigLightColorscheme = origLight
	})

	ConfigDarkColorscheme = ""
	ConfigLightColorscheme = ""
	assert.False(t, ColorModeEnabled())

	ConfigDarkColorscheme = "dracula"
	assert.True(t, ColorModeEnabled())

	ConfigDarkColorscheme = ""
	ConfigLightColorscheme = "tokyonight-day"
	assert.True(t, ColorModeEnabled())
}

func TestLoadConfig_DualColorscheme(t *testing.T) {
	origDark := ConfigDarkColorscheme
	origLight := ConfigLightColorscheme
	t.Cleanup(func() {
		ConfigDarkColorscheme = origDark
		ConfigLightColorscheme = origLight
	})

	cfg := configFile{Colorscheme: "dark:Dracula,light:tokyonight-day"}
	theme := DefaultTheme()
	applyColorscheme(&theme, cfg)

	require.Equal(t, "dracula", ConfigDarkColorscheme)
	require.Equal(t, "tokyonight-day", ConfigLightColorscheme)
}

func TestLoadConfig_DualColorscheme_ReverseOrder(t *testing.T) {
	origDark := ConfigDarkColorscheme
	origLight := ConfigLightColorscheme
	t.Cleanup(func() {
		ConfigDarkColorscheme = origDark
		ConfigLightColorscheme = origLight
	})

	cfg := configFile{Colorscheme: "light:Rose Pine Dawn,dark:Rose Pine"}
	theme := DefaultTheme()
	applyColorscheme(&theme, cfg)

	require.Equal(t, "rose-pine", ConfigDarkColorscheme)
	require.Equal(t, "rose-pine-dawn", ConfigLightColorscheme)
}

func TestLoadConfig_DualColorscheme_DarkOnly(t *testing.T) {
	origDark := ConfigDarkColorscheme
	origLight := ConfigLightColorscheme
	t.Cleanup(func() {
		ConfigDarkColorscheme = origDark
		ConfigLightColorscheme = origLight
	})

	ConfigLightColorscheme = "should-be-cleared"
	cfg := configFile{Colorscheme: "dark:dracula"}
	theme := DefaultTheme()
	applyColorscheme(&theme, cfg)

	require.Equal(t, "dracula", ConfigDarkColorscheme)
	require.Equal(t, "", ConfigLightColorscheme, "light scheme should be empty when not specified")
}

func TestApplyColorscheme_PlainSingleScheme(t *testing.T) {
	orig := ActiveSchemeName
	origDark := ConfigDarkColorscheme
	origLight := ConfigLightColorscheme
	t.Cleanup(func() {
		ActiveSchemeName = orig
		ConfigDarkColorscheme = origDark
		ConfigLightColorscheme = origLight
		ApplyTheme(DefaultTheme())
	})
	ConfigDarkColorscheme = ""
	ConfigLightColorscheme = ""

	theme := DefaultTheme()
	applyColorscheme(&theme, configFile{Colorscheme: "dracula"})

	assert.Equal(t, "dracula", ActiveSchemeName)
	assert.Empty(t, ConfigDarkColorscheme, "dark scheme must stay unset for plain syntax")
	assert.Empty(t, ConfigLightColorscheme, "light scheme must stay unset for plain syntax")
}

func TestApplyColorscheme_PlainSingleScheme_WithSpaces(t *testing.T) {
	orig := ActiveSchemeName
	t.Cleanup(func() {
		ActiveSchemeName = orig
		ApplyTheme(DefaultTheme())
	})

	theme := DefaultTheme()
	applyColorscheme(&theme, configFile{Colorscheme: "Rose Pine"})

	assert.Equal(t, "rose-pine", ActiveSchemeName, "spaces should normalise to hyphens for plain syntax")
}

func TestParseDualColorscheme(t *testing.T) {
	cases := []struct {
		input string
		dark  string
		light string
		dual  bool
	}{
		{"dark:Rose Pine,light:Rose Pine Dawn", "rose-pine", "rose-pine-dawn", true},
		{"light:Rose Pine Dawn,dark:Rose Pine", "rose-pine", "rose-pine-dawn", true},
		{"dark:dracula", "dracula", "", true},
		{"light:tokyonight-day", "", "tokyonight-day", true},
		{"dracula", "", "", false},
		{"", "", "", false},
		{"dark: Dracula , light: tokyonight-day", "dracula", "tokyonight-day", true},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			dark, light, dual := parseDualColorscheme(tc.input)
			assert.Equal(t, tc.dual, dual, "isDual")
			assert.Equal(t, tc.dark, dark, "dark")
			assert.Equal(t, tc.light, light, "light")
		})
	}
}
