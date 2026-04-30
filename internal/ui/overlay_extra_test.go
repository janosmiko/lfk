package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- RenderPasteConfirmOverlay ---

// Paste confirm overlay must NOT include inline `[y] yes [n] no` text.
// Keymap hints belong in the bottom hint bar (overlayHintBarDialog) so all
// confirm overlays share one convention. PRs #80 and #97 both proposed the
// same inline-hint addition and were closed as inconsistent with the design.
func TestRenderPasteConfirmOverlayHasNoInlineKeyHints(t *testing.T) {
	result := stripANSI(RenderPasteConfirmOverlay(7))
	assert.Contains(t, result, "Paste", "should show the title")
	assert.Contains(t, result, "Paste contains 7 lines.")
	assert.Contains(t, result, "Flatten and paste?")
	assert.NotContains(t, result, "[y]", "inline y/n hints belong in the hint bar")
	assert.NotContains(t, result, "[n]")
	assert.NotContains(t, result, "yes")
	assert.NotContains(t, result, "no")
}

// --- RenderQuitConfirmOverlay ---

func TestRenderQuitConfirmOverlay(t *testing.T) {
	// Single question, centered both horizontally and vertically within
	// (innerWidth, innerHeight). The previous two-line layout (title +
	// question) was being misread as a menu, so it was dropped.

	t.Run("multi-row inner centers on middle row", func(t *testing.T) {
		result := RenderQuitConfirmOverlay(26, 3)
		assert.Contains(t, result, "Quit lfk?", "should show the question")

		// Vertical centering: 3 rows total, 1 row of question → 1 blank row
		// above and below.
		lines := strings.Split(result, "\n")
		assert.Equal(t, 3, len(lines), "should fill innerHeight rows")
		questionRow := -1
		for i, line := range lines {
			if strings.Contains(stripANSI(line), "Quit lfk?") {
				questionRow = i
			}
		}
		assert.Equal(t, 1, questionRow, "question must sit on the middle row")

		// Horizontal centering: roughly equal leading/trailing whitespace.
		plain := stripANSI(lines[questionRow])
		leading := len(plain) - len(strings.TrimLeft(plain, " "))
		trailing := len(plain) - len(strings.TrimRight(plain, " "))
		diff := leading - trailing
		if diff < 0 {
			diff = -diff
		}
		assert.LessOrEqual(t, diff, 1, "text should be horizontally centered (leading=%d trailing=%d)", leading, trailing)
	})

	// Production sizing — view_status.go passes (overlayW-6, overlayH-4)
	// with overlayW=32 and overlayH=5, giving 26x1. Pin this so a future
	// "let's add more padding" change doesn't silently revive the
	// floating-text-in-empty-box look that triggered PRs #80 and #97.
	t.Run("production 26x1 inner produces single centered row", func(t *testing.T) {
		result := RenderQuitConfirmOverlay(26, 1)
		lines := strings.Split(result, "\n")
		assert.Equal(t, 1, len(lines), "single inner row")
		assert.Contains(t, stripANSI(lines[0]), "Quit lfk?")
	})
}

// --- RenderConfirmTypeOverlay ---

func TestRenderConfirmTypeOverlay(t *testing.T) {
	tests := []struct {
		name       string
		title      string
		question   string
		input      string
		wantSubstr []string
		wantAbsent []string
	}{
		{
			name:       "force finalize empty input shows placeholder",
			title:      "Confirm Force Finalize",
			question:   "Remove all finalizers from my-pod?",
			input:      "",
			wantSubstr: []string{"Confirm Force Finalize", "my-pod", "DELETE", "_"},
		},
		{
			name:       "force delete shows custom title and question",
			title:      "Confirm Force Delete",
			question:   "Force delete my-pod?",
			input:      "DEL",
			wantSubstr: []string{"Confirm Force Delete", "Force delete my-pod?", "DELETE", "DEL"},
			wantAbsent: []string{"_"},
		},
		{
			name:       "full DELETE input",
			title:      "Confirm Force Delete",
			question:   "Force delete resource?",
			input:      "DELETE",
			wantSubstr: []string{"Confirm Force Delete", "resource", "DELETE"},
			wantAbsent: []string{"_"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderConfirmTypeOverlay(tt.title, tt.question, tt.input)
			for _, sub := range tt.wantSubstr {
				assert.Contains(t, result, sub, "result should contain %q", sub)
			}
			for _, absent := range tt.wantAbsent {
				assert.NotContains(t, result, absent, "result should not contain %q", absent)
			}
		})
	}
}
