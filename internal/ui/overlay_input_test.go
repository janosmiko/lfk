package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
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

	t.Run("ShowCursor renders a reverse-video cursor cell at the cursor offset", func(t *testing.T) {
		// With styling stripped in tests, a cursor at the end of the input
		// surfaces as a trailing cursor cell (space) after the value.
		out := RenderOverlayInput(OverlayInputConfig{
			Title: "Add Labels",
			Rows:  []OverlayInputRow{{Label: "key=value: ", Input: "app=foo", ShowCursor: true, Cursor: 7}},
		})
		assert.Contains(t, stripANSI(out), "app=foo ")
	})

	t.Run("ShowCursor off renders no cursor cell", func(t *testing.T) {
		out := RenderOverlayInput(OverlayInputConfig{
			Title: "Scale",
			Rows:  []OverlayInputRow{{Label: "Replicas: ", Input: "3", ShowCursor: false}},
		})
		assert.NotContains(t, out, "█")
		assert.NotContains(t, out, "3 ")
	})

	t.Run("Width highlights the active row full-width", func(t *testing.T) {
		out := RenderOverlayInput(OverlayInputConfig{
			Title: "Scale",
			Width: 30,
			Rows:  []OverlayInputRow{{Label: "Replicas: ", Input: "3", ShowCursor: true}},
		})
		// The active row carries the label + value and is padded to Width.
		// Match on the stripped line: the rendered row interleaves SGR with
		// the text, so a raw substring search would not find it.
		var rowLine string
		for line := range strings.SplitSeq(out, "\n") {
			if strings.Contains(stripANSI(line), "Replicas: 3") {
				rowLine = line
			}
		}
		assert.NotEmpty(t, rowLine)
		assert.GreaterOrEqual(t, lipgloss.Width(rowLine), 30)
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
