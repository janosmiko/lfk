package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- RenderQuitConfirmOverlay ---

func TestRenderQuitConfirmOverlay(t *testing.T) {
	// Two-line layout: question + key hint, separated by a blank
	// spacer row, centered both horizontally and vertically within
	// (innerWidth, innerHeight).
	result := RenderQuitConfirmOverlay(26, 5)
	assert.Contains(t, result, "Quit lfk?", "should show the question")
	assert.Contains(t, result, "[y] yes", "should show the yes hint")
	assert.Contains(t, result, "[n] no", "should show the no hint")

	// Vertical centering: 5 rows total. Content is 3 rows (question,
	// blank spacer, hint), leaving 1 blank row above and 1 below.
	lines := strings.Split(result, "\n")
	assert.Equal(t, 5, len(lines), "should fill innerHeight rows")

	questionRow, hintRow := -1, -1
	for i, line := range lines {
		plain := stripANSI(line)
		if strings.Contains(plain, "Quit lfk?") {
			questionRow = i
		}
		if strings.Contains(plain, "[y] yes") {
			hintRow = i
		}
	}
	assert.Equal(t, 1, questionRow, "question must sit on the second row")
	assert.Equal(t, 3, hintRow, "hint must sit two rows below the question")

	// Horizontal centering of the question line: roughly equal
	// leading/trailing whitespace.
	plain := stripANSI(lines[questionRow])
	leading := len(plain) - len(strings.TrimLeft(plain, " "))
	trailing := len(plain) - len(strings.TrimRight(plain, " "))
	diff := leading - trailing
	if diff < 0 {
		diff = -diff
	}
	assert.LessOrEqual(t, diff, 1, "text should be horizontally centered (leading=%d trailing=%d)", leading, trailing)
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
