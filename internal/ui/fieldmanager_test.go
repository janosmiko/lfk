package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFieldManagerColorIndex_IsStableAndInRange(t *testing.T) {
	for _, name := range []string{"kubectl", "argocd-controller", "helm", ""} {
		idx := FieldManagerColorIndex(name)
		assert.GreaterOrEqual(t, idx, 0)
		assert.Less(t, idx, len(fieldManagerPalette()))
		assert.Equal(t, idx, FieldManagerColorIndex(name), "same name, same slot")
	}
}

func TestFieldManagerStyle_DimIsFaint(t *testing.T) {
	assert.False(t, FieldManagerStyle("kubectl", false).GetFaint())
	assert.True(t, FieldManagerStyle("kubectl", true).GetFaint())
}

func TestFieldManagerStyle_FollowsTheActiveTheme(t *testing.T) {
	original := ColorPrimary
	t.Cleanup(func() { ColorPrimary = original })

	before := FieldManagerStyle("kubectl", false).GetForeground()
	ColorPrimary = "#ff0000"
	after := FieldManagerStyle("kubectl", false).GetForeground()

	if FieldManagerColorIndex("kubectl") == 0 {
		assert.NotEqual(t, before, after, "the gutter must follow a theme change")
	}
}

func TestSanitizeTerminalText_DropsControlsAndBidiOverrides(t *testing.T) {
	assert.Equal(t, "abc", SanitizeTerminalText("a\x1bb\x7fc"))
	assert.Equal(t, "abc", SanitizeTerminalText("ab\u202ec"))
	assert.Equal(t, "abc", SanitizeTerminalText("a\u2066b\u2069c"))
	assert.Equal(t, "\u200eab", SanitizeTerminalText("\u200eab"),
		"a plain direction mark is not an override")
	assert.Equal(t, "kube-controller-manager", SanitizeTerminalText("kube-controller-manager"))
}

func TestStripBidiOverrides_DropsOnlyBidiChars(t *testing.T) {
	// Unlike SanitizeTerminalText, ESC/TAB/SGR must survive - this is for
	// sinks that already run the ANSI-aware body sanitizer and only need
	// the bidi-reordering guard on top.
	assert.Equal(t, "a\tb\x1b[31mc\x1b[0m", StripBidiOverrides("a\tb\x1b[31mc\x1b[0m"))
	assert.Equal(t, "abc", StripBidiOverrides("ab\u202ec"))
	assert.Equal(t, "abc", StripBidiOverrides("a\u2066b\u2069c"))
	assert.Equal(t, "\u200eab", StripBidiOverrides("\u200eab"),
		"a plain direction mark is not an override")
}
