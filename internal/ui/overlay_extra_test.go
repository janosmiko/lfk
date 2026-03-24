package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- RenderQuitConfirmOverlay ---

func TestRenderQuitConfirmOverlay(t *testing.T) {
	tests := []struct {
		name       string
		wantSubstr []string
	}{
		{
			name:       "contains quit title and confirmation prompts",
			wantSubstr: []string{"Quit", "Quit lfk?"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderQuitConfirmOverlay()
			for _, sub := range tt.wantSubstr {
				assert.Contains(t, result, sub, "result should contain %q", sub)
			}
		})
	}
}

// --- RenderConfirmTypeOverlay ---

func TestRenderConfirmTypeOverlay(t *testing.T) {
	tests := []struct {
		name       string
		action     string
		input      string
		wantSubstr []string
		wantAbsent []string
	}{
		{
			name:       "empty input shows placeholder",
			action:     "my-pod",
			input:      "",
			wantSubstr: []string{"Confirm Force Finalize", "my-pod", "DELETE", "_"},
		},
		{
			name:       "partial input shown",
			action:     "my-namespace/my-pod",
			input:      "DEL",
			wantSubstr: []string{"Confirm Force Finalize", "my-namespace/my-pod", "DELETE", "DEL"},
			wantAbsent: []string{"_"},
		},
		{
			name:       "full DELETE input",
			action:     "resource",
			input:      "DELETE",
			wantSubstr: []string{"Confirm Force Finalize", "resource", "DELETE"},
			wantAbsent: []string{"_"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderConfirmTypeOverlay(tt.action, tt.input)
			for _, sub := range tt.wantSubstr {
				assert.Contains(t, result, sub, "result should contain %q", sub)
			}
			for _, absent := range tt.wantAbsent {
				assert.NotContains(t, result, absent, "result should not contain %q", absent)
			}
		})
	}
}
