package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// TestRenderSecretEditor_CursorRowNoSelectedBgWhileEditing pins the
// fix for the user's "background is different before and after the
// cursor" report. The cursor renders as reverse-video on the char at
// the cursor byte offset (overlayCursor); its embedded \x1b[7m
// ... \x1b[0m pair issues an SGR reset, and the table's outer
// SelectedBg wrapper does NOT survive that reset — so head text
// before the cursor showed the row's SelectedBg, while tail text
// after the cursor reverted to BaseBg, producing the two-tone band.
//
// The fix: during edit mode, the cursor row drops its SelectedBg
// styling (kept Bold + Primary fg so the row stays visually
// distinct). The reverse-video cursor remains the active-row
// indicator, and head/tail share a single background.
//
// Asserts: the SelectedBg color (defaultColorSelectedBg = "#7aa2f7"
// in TrueColor SGR = 48;2;122;162;247) does NOT appear in the
// rendered editor when editing == true. Pinned to TrueColor profile
// so the assertion is deterministic regardless of the host
// terminal's detected profile.
func TestRenderSecretEditor_CursorRowNoSelectedBgWhileEditing(t *testing.T) {
	original := lipgloss.DefaultRenderer().ColorProfile()
	t.Cleanup(func() { lipgloss.DefaultRenderer().SetColorProfile(original) })

	origTheme := ActiveTheme
	origContrast := ConfigMinContrastRatio
	origNoColor := ConfigNoColor
	t.Cleanup(func() {
		ConfigNoColor = origNoColor
		ConfigMinContrastRatio = origContrast
		ApplyTheme(origTheme)
	})
	ConfigNoColor = false
	ConfigMinContrastRatio = 0
	ApplyTheme(DefaultTheme())
	// ApplyTheme's color branch resets the color profile to whatever
	// termenv originally detected when a previous test triggered
	// no-color mode. Force TrueColor AFTER ApplyTheme so the SGR
	// triples this test inspects are emitted verbatim.
	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)

	secret := &model.SecretData{
		Keys: []string{"DB_PASSWORD"},
		Data: map[string]string{"DB_PASSWORD": "<redacted>"},
	}

	// Render with editing=true, cursor parked in the middle of the
	// value so head+cursor+tail are all non-empty.
	editing := RenderSecretEditorOverlay(
		secret, 0, nil, true,
		true,
		"DB_PASSWORD", 11,
		"<redacted>", 3,
		1, "", false,
		nil, false, 0, 0,
		120, 30,
	)

	// TrueColor SGR for #7aa2f7 (defaultColorSelectedBg). lipgloss
	// quantises the hex through termenv before emitting, which yields
	// "121;162;247" rather than the literal "122;162;247" — so match
	// the actually-emitted triple, not the raw hex math.
	selectedBgSGR := "48;2;121;162;247"
	assert.NotContains(t, editing, selectedBgSGR,
		"editing-mode render must not embed the SelectedBg color anywhere — "+
			"its embedded SGR reset would create a head/tail bg mismatch around the cursor")
}

// TestRenderSecretEditor_CursorRowKeepsSelectedBgWhenIdle is the
// negative companion: when NOT editing, the cursor row SHOULD carry
// the SelectedBg highlight — that's the normal "you're on this row"
// indicator. Without this guard the previous test would pass even if
// we accidentally stripped SelectedBg in the idle case too.
func TestRenderSecretEditor_CursorRowKeepsSelectedBgWhenIdle(t *testing.T) {
	original := lipgloss.DefaultRenderer().ColorProfile()
	t.Cleanup(func() { lipgloss.DefaultRenderer().SetColorProfile(original) })

	origTheme := ActiveTheme
	origContrast := ConfigMinContrastRatio
	origNoColor := ConfigNoColor
	t.Cleanup(func() {
		ConfigNoColor = origNoColor
		ConfigMinContrastRatio = origContrast
		ApplyTheme(origTheme)
	})
	ConfigNoColor = false
	ConfigMinContrastRatio = 0
	ApplyTheme(DefaultTheme())
	// ApplyTheme's color branch resets the color profile to whatever
	// termenv originally detected when a previous test triggered
	// no-color mode. Force TrueColor AFTER ApplyTheme so the SGR
	// triples this test inspects are emitted verbatim.
	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)

	secret := &model.SecretData{
		Keys: []string{"DB_PASSWORD"},
		Data: map[string]string{"DB_PASSWORD": "<redacted>"},
	}

	idle := RenderSecretEditorOverlay(
		secret, 0, nil, true,
		false,
		"", 0,
		"", 0,
		0, "", false,
		nil, false, 0, 0,
		120, 30,
	)

	selectedBgSGR := "48;2;121;162;247"
	assert.True(t, strings.Contains(idle, selectedBgSGR),
		"idle-mode render must keep SelectedBg on the cursor row — "+
			"that's the only visual cue without an active text cursor")
}
