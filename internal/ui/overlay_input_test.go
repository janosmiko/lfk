package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- RenderOverlayInput ---

func TestRenderOverlayInput(t *testing.T) {
	t.Run("title + single input row", func(t *testing.T) {
		out := RenderOverlayInput(OverlayInputConfig{
			Title: "Scale Deployment",
			Rows:  []OverlayInputRow{{Label: "Replicas: ", Input: "3"}},
		})
		assert.Contains(t, out, "Scale Deployment")
		assert.Contains(t, out, "Replicas: ")
		assert.Contains(t, out, "3")
	})

	t.Run("empty input shows placeholder", func(t *testing.T) {
		out := RenderOverlayInput(OverlayInputConfig{
			Title: "Resize PVC",
			Rows:  []OverlayInputRow{{Label: "New size: ", Input: "", Placeholder: "e.g. 10Gi"}},
		})
		assert.Contains(t, out, "e.g. 10Gi")
	})

	t.Run("non-empty input hides placeholder", func(t *testing.T) {
		out := RenderOverlayInput(OverlayInputConfig{
			Title: "Resize PVC",
			Rows:  []OverlayInputRow{{Label: "New size: ", Input: "20Gi", Placeholder: "e.g. 10Gi"}},
		})
		assert.Contains(t, out, "20Gi")
		assert.NotContains(t, out, "e.g. 10Gi")
	})

	t.Run("subtitle renders below title", func(t *testing.T) {
		out := RenderOverlayInput(OverlayInputConfig{
			Title:    "Port Forward",
			Subtitle: "my-service",
			Rows:     []OverlayInputRow{{Label: "Local port: ", Input: "8080"}},
		})
		assert.Contains(t, out, "my-service")
	})

	t.Run("Hint renders dim", func(t *testing.T) {
		out := RenderOverlayInput(OverlayInputConfig{
			Title: "Resize PVC",
			Hint:  "Current: 10Gi",
			Rows:  []OverlayInputRow{{Label: "New size: ", Input: ""}},
		})
		assert.Contains(t, out, "Current: 10Gi")
	})

	t.Run("multiple rows rendered in order", func(t *testing.T) {
		out := RenderOverlayInput(OverlayInputConfig{
			Title: "Port Forward",
			Rows: []OverlayInputRow{
				{Label: "Remote port: ", Input: "80", ReadOnly: true},
				{Label: "Local port:  ", Input: "8080"},
			},
		})
		assert.Contains(t, out, "Remote port: ")
		assert.Contains(t, out, "Local port:  ")
		assert.Contains(t, out, "80")
		assert.Contains(t, out, "8080")
	})

	t.Run("ShowCursor appends a cursor block to the row", func(t *testing.T) {
		out := RenderOverlayInput(OverlayInputConfig{
			Title: "Add Labels",
			Rows:  []OverlayInputRow{{Label: "key=value: ", Input: "app=foo", ShowCursor: true}},
		})
		assert.Contains(t, out, "█")
	})

	t.Run("ShowCursor off omits cursor block", func(t *testing.T) {
		out := RenderOverlayInput(OverlayInputConfig{
			Title: "Scale",
			Rows:  []OverlayInputRow{{Label: "Replicas: ", Input: "3", ShowCursor: false}},
		})
		assert.NotContains(t, out, "█")
	})

	t.Run("candidate list rendered with header and cursor", func(t *testing.T) {
		out := RenderOverlayInput(OverlayInputConfig{
			Title:           "Port Forward",
			CandidateTitle:  "Available ports:",
			Candidates:      []OverlayListItem{{Name: "80"}, {Name: "443"}},
			CandidateCursor: 1,
			Rows:            []OverlayInputRow{{Label: "Local port: ", Input: ""}},
		})
		assert.Contains(t, out, "Available ports:")
		assert.Contains(t, out, "80")
		assert.Contains(t, out, "443")
	})

	t.Run("empty Candidates skips the list", func(t *testing.T) {
		out := RenderOverlayInput(OverlayInputConfig{
			Title:          "Port Forward",
			CandidateTitle: "Available ports:",
			Rows:           []OverlayInputRow{{Label: "Local port: ", Input: "8080"}},
		})
		assert.NotContains(t, out, "Available ports:")
	})
}
