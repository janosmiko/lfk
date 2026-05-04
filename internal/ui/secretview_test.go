package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// TestRenderSecretEditorOverlay_CursorDoesNotShiftCharacters pins
// the fix for the user's "characters are shifted after the cursor"
// report. The previous insert-style "█" cursor pushed every char to
// the right of the cursor by one column, which made the text appear
// to "jump" as the cursor moved. The fix renders the cursor as
// inverse-video on the char AT the cursor, so chars stay put.
//
// Asserts that all six chars of "abcdef" appear in the rendered
// output as a contiguous run with no inserted "█" between them.
func TestRenderSecretEditorOverlay_CursorDoesNotShiftCharacters(t *testing.T) {
	secret := &model.SecretData{
		Keys: []string{"k"},
		Data: map[string]string{"k": "ignored-while-editing"},
	}
	out := RenderSecretEditorOverlay(
		secret, 0, nil, true,
		true,
		"k", 1,
		"abcdef", 3,
		1, // editing the value column
		"", false,
		nil, false, 0,
		120, 30,
	)
	assert.NotContains(t, out, "abc█def",
		"the inserted-block cursor was the bug — must not appear in rendered output")
	assert.NotContains(t, out, "ab█cdef", "or any other inserted-block variant")
	// All six chars still present (no character lost), even though
	// 'd' is reverse-styled — Strip ANSI and check.
	stripped := stripANSI(out)
	assert.Contains(t, stripped, "abcdef",
		"reverse-video cursor must overlay 'd' without removing or shifting other chars")
}

// TestRenderSecretEditorOverlay_EditingShowsValueAsMultiline asserts
// that opening an existing multi-line value for editing renders the
// value across actual lines (not collapsed via SingleLineCell's "↵"
// glyph). The user reported they couldn't see/edit multi-line
// secrets because the table cell collapsed them; the fix swaps the
// table for a focused edit pane while editing is true.
func TestRenderSecretEditorOverlay_EditingShowsValueAsMultiline(t *testing.T) {
	secret := &model.SecretData{
		Keys: []string{"DB_PASSWORD"},
		Data: map[string]string{"DB_PASSWORD": "ignored-while-editing"},
	}
	out := RenderSecretEditorOverlay(
		secret, 0, nil, true,
		true,              // editing
		"DB_PASSWORD", 11, // editKey + cursor at end
		"line-one\nline-two\nline-three", // editValue (multi-line)
		31,                               // editValue cursor at end
		1,                                // editing the value column
		"", false,
		nil, false, 0,
		120, 30,
	)
	assert.Contains(t, out, "line-one", "first line of the multi-line value must be visible")
	assert.Contains(t, out, "line-two", "second line must be visible — not collapsed to a ↵ glyph")
	assert.Contains(t, out, "line-three", "third line must be visible")
	assert.NotContains(t, out, "line-one ↵", "newlines stay as actual line breaks in edit mode, not collapsed")
}

// TestRenderSecretEditorOverlay_LongMultilineValueKeepsHeight pins
// the regression for the layout bug the user reported: a really long
// or multi-line value used to make lipgloss/table wrap the cell
// vertically, expanding the row, the table, and the entire editor
// box past its target height. SingleLineCell collapses newlines and
// truncates so the rendered overlay's line count stays the same as
// for a short single-line value.
func TestRenderSecretEditorOverlay_LongMultilineValueKeepsHeight(t *testing.T) {
	short := &model.SecretData{
		Keys: []string{"k1"}, Data: map[string]string{"k1": "short"},
	}
	long := &model.SecretData{
		Keys: []string{"k1"}, Data: map[string]string{"k1": strings.Repeat("AAAAAAAAAA", 200) + "\nline2\nline3"},
	}

	revealed := map[string]bool{"k1": true}
	a := RenderSecretEditorOverlay(short, 0, revealed, false, false, "", 0, "", 0, 0, "", false, nil, false, 0, 100, 25)
	b := RenderSecretEditorOverlay(long, 0, revealed, false, false, "", 0, "", 0, 0, "", false, nil, false, 0, 100, 25)

	aLines := strings.Count(a, "\n")
	bLines := strings.Count(b, "\n")
	assert.Equal(t, aLines, bLines,
		"long/multi-line value must not change the editor's rendered line count — got short=%d, long=%d",
		aLines, bLines)
}

// TestRenderSecretEditorOverlay_SearchFiltersKeys confirms the
// active / search query narrows the visible key list. Acts as the
// integration check for FilterKVKeys + the renderer's search-bar slot.
func TestRenderSecretEditorOverlay_SearchFiltersKeys(t *testing.T) {
	secret := &model.SecretData{
		Keys: []string{"DB_PASSWORD", "API_TOKEN", "AWS_KEY"},
		Data: map[string]string{
			"DB_PASSWORD": "p1",
			"API_TOKEN":   "p2",
			"AWS_KEY":     "p3",
		},
	}
	out := RenderSecretEditorOverlay(secret, 0, nil, true, false, "", 0, "", 0, 0, "API", true, nil, false, 0, 120, 30)
	assert.Contains(t, out, "API_TOKEN", "filter API matches API_TOKEN")
	assert.NotContains(t, out, "DB_PASSWORD", "DB_PASSWORD doesn't contain 'API' — must be filtered out")
	assert.NotContains(t, out, "AWS_KEY", "AWS_KEY doesn't contain 'API' — must be filtered out")
	assert.Contains(t, out, "/ API", "search bar must show the active query so the user sees what's filtering")
}

// TestRenderSecretEditorOverlay_InnerPanelMatchesOuterBg pins the
// fix for the bug the user reported: the bordered inner panel used
// to render with no Background, so the panel's content area showed
// terminal default bg while the surrounding OverlayStyle had a
// themed bg — visible as a "darker frame around lighter inner box".
//
// After the fix both the outer overlay and the inner panel bind
// BaseBg, so the rendered output emits at least one bg-setting SGR
// per styled span and the BaseBg sequence appears many times across
// the rendered overlay (one per row, plus borders). This is a
// structural assertion that catches a regression to fg-only styling.
func TestRenderSecretEditorOverlay_InnerPanelMatchesOuterBg(t *testing.T) {
	originalProfile := lipgloss.DefaultRenderer().ColorProfile()
	originalNoColor := ConfigNoColor
	originalTransparent := ConfigTransparentBg
	t.Cleanup(func() {
		lipgloss.DefaultRenderer().SetColorProfile(originalProfile)
		ConfigNoColor = originalNoColor
		ConfigTransparentBg = originalTransparent
		ApplyTheme(DefaultTheme())
	})
	ConfigNoColor = false
	ConfigTransparentBg = false
	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)
	ApplyTheme(DefaultTheme())
	// ApplyTheme restores originalColorProfile (theme.go:109-110), so
	// re-force TrueColor for the SGR-counting check to be observable.
	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)

	secret := &model.SecretData{
		Keys: []string{"DB_PASSWORD"},
		Data: map[string]string{"DB_PASSWORD": "hunter2"},
	}
	out := RenderSecretEditorOverlay(secret, 0, nil, false, false, "", 0, "", 0, 0, "", false, nil, false, 0, 120, 30)

	// 256-color bg = "48;5;", truecolor bg = "48;2;". Both forms count
	// as a bg-setting SGR.
	bgMarkers := strings.Count(out, "48;5;") + strings.Count(out, "48;2;")
	assert.GreaterOrEqualf(t, bgMarkers, 4,
		"editor overlay must emit bg-setting SGRs for the outer overlay AND the inner panel; got %d", bgMarkers)
}

// TestRenderSecretEditorOverlay_EditingDoesNotLeakANSITail pins the
// regression for the user's "Value  ;162;247;48;2;36;40;59m╭──────"
// report. kvFieldBox used to splice the label into the styled top
// border via []rune(top) slicing, but the styled border carries ANSI
// escape sequences ("\x1b[38;2;R;G;B;48;2;r;g;bm…") whose bytes are
// counted as runes — so the slice index landed inside an SGR code,
// leaving the tail (digits + ';' + 'm') visible as raw text. The fix
// uses ANSI-aware splicing so the escape sequence stays intact.
//
// Asserts that, after stripping ANSI from the rendered editor in
// editing mode, no SGR-tail signature (";\d+;\d+;\d+m" or similar)
// remains in the visible text.
func TestRenderSecretEditorOverlay_EditingDoesNotLeakANSITail(t *testing.T) {
	originalProfile := lipgloss.DefaultRenderer().ColorProfile()
	originalNoColor := ConfigNoColor
	originalTransparent := ConfigTransparentBg
	t.Cleanup(func() {
		lipgloss.DefaultRenderer().SetColorProfile(originalProfile)
		ConfigNoColor = originalNoColor
		ConfigTransparentBg = originalTransparent
		ApplyTheme(DefaultTheme())
	})
	ConfigNoColor = false
	ConfigTransparentBg = false
	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)
	ApplyTheme(DefaultTheme())
	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)

	secret := &model.SecretData{
		Keys: []string{"DB_PASSWORD"},
		Data: map[string]string{"DB_PASSWORD": "ignored-while-editing"},
	}
	// Editing mode (true) routes through RenderKVEditorEditPane which
	// uses kvFieldBox for both the Key and Value labels.
	out := RenderSecretEditorOverlay(
		secret, 0, nil, true,
		true,
		"DB_PASSWORD", 11,
		"hunter2", 7,
		1,
		"", false,
		nil, false, 0,
		120, 30,
	)
	plain := stripANSI(out)
	// SGR tail signature: a sequence of digits and semicolons followed
	// by 'm'. Two or more semicolon-separated numeric groups before an
	// 'm' is a near-certain SGR tail when seen in stripped (ANSI-free)
	// text — visible content shouldn't have that shape anywhere.
	for _, marker := range []string{"48;2;", "38;2;", "48;5;", "38;5;"} {
		assert.NotContains(t, plain, marker,
			"stripped output must not contain SGR-tail signature %q — kvFieldBox is leaking ANSI bytes via []rune slicing", marker)
	}
	// The Key/Value labels should still be visible in the output.
	assert.Contains(t, plain, "Key", "Key field-box label must render")
	assert.Contains(t, plain, "Value", "Value field-box label must render")
}

// TestRenderSecretEditorOverlay_LongValueScrollsToCursor pins the
// fix for the user's "secret value is really long … the end of the
// secret is not shown on the screen" report. overlayCursorMultiline
// used to hard-clip the visual lines to maxH from the top, so when
// editing a value taller than the visible field box the trailing
// lines (and any cursor placed inside them) were silently dropped.
// The fix scrolls the visible window so the cursor's line stays in
// view; when the cursor sits at the end of a long value, the END of
// the value renders, not the beginning.
func TestRenderSecretEditorOverlay_LongValueScrollsToCursor(t *testing.T) {
	// Build a value with many short lines — comfortably more than
	// the field box's ~maxH so the top would clip the bottom under
	// the old behaviour.
	var b strings.Builder
	for i := range 60 {
		b.WriteString("line-")
		// Pad the index so we can spot specific lines in the output.
		switch {
		case i < 10:
			b.WriteString("0")
		}
		b.WriteString(itoaTiny(i))
		b.WriteString("\n")
	}
	b.WriteString("LAST-LINE-MARKER")
	value := b.String()

	secret := &model.SecretData{
		Keys: []string{"big"},
		Data: map[string]string{"big": "ignored-while-editing"},
	}
	out := RenderSecretEditorOverlay(
		secret, 0, nil, true,
		true,
		"big", 3,
		value, len(value), // cursor at end of value
		1, // editing the value column
		"", false,
		nil, false, 0,
		120, 30,
	)
	plain := stripANSI(out)
	assert.Contains(t, plain, "LAST-LINE-MARKER",
		"end of long multi-line value must be visible — the renderer must scroll to follow the cursor")
}

// itoaTiny converts a non-negative int < 1000 to a string without
// pulling in strconv at the top of the test file.
func itoaTiny(n int) string {
	if n == 0 {
		return "0"
	}
	var digits [4]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
}

// --- secretValueDisplay ---

func TestSecretValueDisplay(t *testing.T) {
	tests := []struct {
		name     string
		val      string
		revealed bool
		maxW     int
		expected string
	}{
		{
			name:     "hidden value shows mask",
			val:      "super-secret",
			revealed: false,
			maxW:     20,
			expected: "********",
		},
		{
			name:     "revealed value shows actual",
			val:      "mypassword",
			revealed: true,
			maxW:     20,
			expected: "mypassword",
		},
		{
			name:     "revealed long value truncated",
			val:      "a-very-long-secret-value-that-exceeds-width",
			revealed: true,
			maxW:     15,
		},
		{
			name:     "empty revealed value",
			val:      "",
			revealed: true,
			maxW:     20,
			expected: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := secretValueDisplay(tt.val, tt.revealed, tt.maxW)
			if tt.expected != "" {
				assert.Equal(t, tt.expected, result)
			}
			if tt.revealed && tt.maxW > 0 && len(tt.val) > tt.maxW {
				// Truncated value should be shorter than the original.
				assert.LessOrEqual(t, len(result), tt.maxW)
			}
		})
	}
}

// --- renderSecretEditorTable ---

func TestRenderSecretEditorTable(t *testing.T) {
	t.Run("empty secret shows add hint", func(t *testing.T) {
		secret := &model.SecretData{
			Keys: []string{},
			Data: map[string]string{},
		}
		result := renderSecretEditorTable(secret, 0, nil, false, false, "", "", 0, nil, 60, 20)
		// Headers stay visible above the placeholder; lipgloss/table
		// renders them uppercase.
		assert.Contains(t, result, "KEY")
		assert.Contains(t, result, "VALUE")
		assert.Contains(t, result, "(empty - press 'a' to add a key)")
	})

	t.Run("hidden values show mask", func(t *testing.T) {
		secret := &model.SecretData{
			Keys: []string{"password", "token"},
			Data: map[string]string{"password": "secret123", "token": "abc"},
		}
		result := renderSecretEditorTable(secret, 0, nil, false, false, "", "", 0, nil, 80, 20)
		assert.Contains(t, result, "password")
		assert.Contains(t, result, "********")
		// The actual value should not appear when not revealed.
		assert.NotContains(t, result, "secret123")
	})

	t.Run("revealed keys show actual values", func(t *testing.T) {
		secret := &model.SecretData{
			Keys: []string{"password"},
			Data: map[string]string{"password": "secret123"},
		}
		revealed := map[string]bool{"password": true}
		result := renderSecretEditorTable(secret, 0, revealed, false, false, "", "", 0, nil, 80, 20)
		assert.Contains(t, result, "secret123")
	})

	t.Run("allRevealed shows all values", func(t *testing.T) {
		secret := &model.SecretData{
			Keys: []string{"password", "token"},
			Data: map[string]string{"password": "pass1", "token": "tok1"},
		}
		result := renderSecretEditorTable(secret, 0, nil, true, false, "", "", 0, nil, 80, 20)
		assert.Contains(t, result, "pass1")
		assert.Contains(t, result, "tok1")
	})

	t.Run("selected row keys are present", func(t *testing.T) {
		// Cursor row is highlighted via StyleFunc bg/bold (lipgloss/table
		// handles the visual cue); just assert the data lands in the
		// rendered output.
		secret := &model.SecretData{
			Keys: []string{"key1", "key2"},
			Data: map[string]string{"key1": "v1", "key2": "v2"},
		}
		result := renderSecretEditorTable(secret, 1, nil, false, false, "", "", 0, nil, 60, 20)
		assert.Contains(t, result, "key1")
		assert.Contains(t, result, "key2")
	})

	t.Run("editing key column shows edit cursor", func(t *testing.T) {
		secret := &model.SecretData{
			Keys: []string{"mykey"},
			Data: map[string]string{"mykey": "myval"},
		}
		result := renderSecretEditorTable(secret, 0, nil, false, true, "newkey", "", 0, nil, 60, 20)
		assert.Contains(t, result, "newkey")
		assert.Contains(t, result, "\u2588")
	})

	t.Run("editing value column shows edit cursor", func(t *testing.T) {
		secret := &model.SecretData{
			Keys: []string{"mykey"},
			Data: map[string]string{"mykey": "myval"},
		}
		result := renderSecretEditorTable(secret, 0, nil, false, true, "", "newval", 1, nil, 60, 20)
		assert.Contains(t, result, "newval")
		assert.Contains(t, result, "\u2588")
	})
}

// --- RenderSecretEditorOverlay ---

func TestRenderSecretEditorOverlay(t *testing.T) {
	t.Run("nil secret shows error", func(t *testing.T) {
		result := RenderSecretEditorOverlay(nil, 0, nil, false, false, "", 0, "", 0, 0, "", false, nil, false, 0, 100, 40)
		assert.Contains(t, result, "No secret loaded")
	})

	t.Run("normal mode hints removed from overlay body", func(t *testing.T) {
		// Hints now live in the main status bar, not inline.
		secret := &model.SecretData{
			Keys: []string{"key1"},
			Data: map[string]string{"key1": "val1"},
		}
		result := RenderSecretEditorOverlay(secret, 0, nil, false, false, "", 0, "", 0, 0, "", false, nil, false, 0, 100, 40)
		assert.Contains(t, result, "Secret Editor")
		assert.Contains(t, result, "key1")
	})

	t.Run("editing mode hints removed from overlay body", func(t *testing.T) {
		// Hints now live in the main status bar, not inline.
		secret := &model.SecretData{
			Keys: []string{"key1"},
			Data: map[string]string{"key1": "val1"},
		}
		result := RenderSecretEditorOverlay(secret, 0, nil, false, true, "key1", 4, "val1", 4, 1, "", false, nil, false, 0, 100, 40)
		assert.Contains(t, result, "Secret Editor")
	})
}
