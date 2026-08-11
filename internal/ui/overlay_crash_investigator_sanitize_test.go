package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// hostileCrashPayloads mirrors the format used by the other TASK-880 sink
// tests: literal escape sequences (not the invisible runes they represent),
// keyed so each case's assertion can check for the exact ESC+body sequence
// rather than a bare "\x1b" - the overlay's own styling legitimately emits
// ESC elsewhere (colors, reverse-video, borders).
var hostileCrashPayloads = map[string]string{
	"bidi override":         "\u202e",
	"raw csi":               "\x1b[31m",
	"csi screen erase":      "\x1b[2J",
	"osc52 clipboard write": "\x1b]52;c;aGF4\x07",
}

// TestRenderCrashInvestigatorOverlay_SanitizesSummaryFields guards the
// Summary tab's container State/StateReason and the active container's
// LastReason/LastMessage - all sourced from the cluster's pod status.
func TestRenderCrashInvestigatorOverlay_SanitizesSummaryFields(t *testing.T) {
	for name, payload := range hostileCrashPayloads {
		t.Run(name, func(t *testing.T) {
			entry := CrashInvestigatorEntry{
				PodName: "p", Namespace: "default", ActiveContainer: "app",
				Tab: CrashTabSummary,
				AppContainers: []CrashContainerEntry{
					{
						Name:         "app",
						State:        "Waiting",
						StateReason:  "Reason" + payload,
						RestartCount: 1,
						HasLastTerm:  true,
						LastReason:   "Err" + payload,
						LastMessage:  "boom" + payload,
					},
				},
			}
			out := RenderCrashInvestigatorOverlay(entry, 0, 120, 30)
			assert.NotContains(t, out, payload)
		})
	}
}

// TestRenderCrashInvestigatorOverlay_SanitizesEventFields guards the
// Events tab's Type/Reason/Message columns.
func TestRenderCrashInvestigatorOverlay_SanitizesEventFields(t *testing.T) {
	for name, payload := range hostileCrashPayloads {
		t.Run(name, func(t *testing.T) {
			entry := CrashInvestigatorEntry{
				PodName: "p", Namespace: "default",
				Tab:           CrashTabEvents,
				AppContainers: []CrashContainerEntry{{Name: "app"}},
				Events: []CrashEventEntry{
					{Type: "Warning" + payload, Reason: "BackOff" + payload, Age: "5s", Message: "back-off" + payload},
				},
			}
			out := RenderCrashInvestigatorOverlay(entry, 0, 120, 30)
			assert.NotContains(t, out, payload)
		})
	}
}
