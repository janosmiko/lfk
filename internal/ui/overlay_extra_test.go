package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- RenderQuitConfirmOverlay ---

func TestRenderQuitConfirmOverlay(t *testing.T) {
	// Question + key hint, centered both horizontally and vertically within
	// (innerWidth, innerHeight).
	result := RenderQuitConfirmOverlay(26, 5)
	assert.Contains(t, result, "Quit lfk?", "should show the question")
	assert.Contains(t, result, "[y] yes", "should show yes hint")
	assert.Contains(t, result, "[n] no", "should show no hint")

	// Should fill innerHeight rows.
	lines := strings.Split(result, "\n")
	assert.Equal(t, 5, len(lines), "should fill innerHeight rows")

	// Question should appear in the output.
	questionRow := -1
	for i, line := range lines {
		if strings.Contains(stripANSI(line), "Quit lfk?") {
			questionRow = i
		}
	}
	assert.GreaterOrEqual(t, questionRow, 0, "question must be present")
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
