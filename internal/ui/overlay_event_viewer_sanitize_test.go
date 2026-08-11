package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRenderEventViewer_SanitizesResourceNameInTitle guards the title line:
// p.Lines (the event body) is already sanitized upstream by the caller, but
// the title is built here directly from the raw ResourceName - a cluster
// object name - and used to render "Event Timeline - <name>".
func TestRenderEventViewer_SanitizesResourceNameInTitle(t *testing.T) {
	hostile := map[string]string{
		"bidi override":         "\u202e",
		"raw csi":               "\x1b[31m",
		"csi screen erase":      "\x1b[2J",
		"osc52 clipboard write": "\x1b]52;c;aGF4\x07",
	}
	for name, payload := range hostile {
		t.Run(name, func(t *testing.T) {
			p := EventViewerParams{
				Lines:        []string{"line"},
				ResourceName: "pod" + payload,
				Width:        80,
				Height:       20,
			}
			out := RenderEventViewer(p)
			assert.NotContains(t, out, payload)
		})
	}
}
