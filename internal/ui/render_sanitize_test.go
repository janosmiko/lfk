package ui

import (
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/model"
)

// End-to-end guard for TASK-880. A cluster-sourced field carrying terminal
// control sequences must not reach the screen: OSC-52 writes the operator's
// clipboard, CSI rewrites the screen, and a bidi override makes one value read
// as another. The unit tests next to each sink pin the individual call sites;
// this one renders the panes a normal session shows and asserts nothing hostile
// comes out the far end, so a sink added later is caught even if nobody
// remembers to guard it.

// e2eHostilePayloads are the sequences every render path is driven with, written
// as escapes because two of them are invisible in a source file.
var e2eHostilePayloads = map[string]string{
	"OSC-52 clipboard write": "\x1b]52;c;aGF4\x07",
	"CSI erase display":      "\x1b[2J",
	"bidi override":          "\u202e",
	"C1 control":             "\u009b",
}

// A payload is neutralized once the bytes that give it power are gone. The
// printable tail of an OSC sequence renders as the literal text "]52;c;aGF4",
// which is harmless, and the sanitizers deliberately keep printable characters.
// The markers are therefore the specific introducers rather than ESC on its
// own: styled output carries ESC for every SGR colour, so testing for that
// alone would fail on correct output. "\x1b]" opens an OS command and has no
// place in lipgloss output; "\x1b[2J" erases the display. The bidi overrides
// reorder text with no introducer at all.
var e2eHostileMarkers = map[string]string{
	"OSC-52 clipboard write": "\x1b]",
	"CSI erase display":      "\x1b[2J",
	"bidi override":          "\u202e",
	"C1 control":             "\u009b",
}

// BEL terminates an OSC sequence and is a control byte in its own right. No
// render path has a reason to emit one.
const e2eBellMarker = "\a"

func assertE2ENoHostileSequence(t *testing.T, sink, payloadKey, rendered string) {
	t.Helper()
	if strings.Contains(rendered, e2eHostileMarkers[payloadKey]) {
		t.Errorf("%s: %s survived into the rendered output", sink, payloadKey)
	}
	if strings.Contains(rendered, e2eBellMarker) {
		t.Errorf("%s: BEL survived into the rendered output (%s)", sink, payloadKey)
	}
}

// e2eTaintedItem builds a list row whose every cluster-sourced field carries the
// payload.
func e2eTaintedItem(payload string) model.Item {
	return model.Item{
		Name:      "pod" + payload,
		Kind:      "Pod",
		Namespace: "ns" + payload,
		Status:    "Running" + payload,
		Ready:     "1/1" + payload,
		Restarts:  "0" + payload,
		Extra:     "image:tag" + payload,
		Columns: []model.KeyValue{
			{Key: "Reason", Value: "CrashLoopBackOff" + payload},
			{Key: "Message", Value: "back-off restarting" + payload},
		},
	}
}

func TestRenderTableDropsHostileSequences(t *testing.T) {
	for name, payload := range e2eHostilePayloads {
		t.Run(name, func(t *testing.T) {
			items := []model.Item{e2eTaintedItem(payload)}
			// The cursor row and the plain rows take different paths.
			for _, cursor := range []int{0, -1} {
				out := RenderTable("PODS", items, cursor, 120, 20, false, "", "")
				assertE2ENoHostileSequence(t, "RenderTable", name, out)
			}
		})
	}
}

func TestRenderColumnDropsHostileSequences(t *testing.T) {
	for name, payload := range e2eHostilePayloads {
		t.Run(name, func(t *testing.T) {
			items := []model.Item{e2eTaintedItem(payload)}
			out := RenderColumn("TYPES", items, 0, 40, 20, true, false, "", "")
			assertE2ENoHostileSequence(t, "RenderColumn", name, out)
		})
	}
}

func TestRenderContainerDetailDropsHostileSequences(t *testing.T) {
	for name, payload := range e2eHostilePayloads {
		t.Run(name, func(t *testing.T) {
			item := e2eTaintedItem(payload)
			out := RenderContainerDetail(&item, 120, 20)
			assertE2ENoHostileSequence(t, "RenderContainerDetail", name, out)
		})
	}
}

// SGR colour has to survive the body paths, or trivy output and helm values
// render as plain text. Guards against closing a leak with the blunt sanitizer
// where the ANSI-aware one belongs.
func TestBodySanitizerKeepsColour(t *testing.T) {
	const red = "\x1b[31mFAILED\x1b[0m"
	if got := SanitizeLogBody(red, true); !strings.Contains(got, "[31m") {
		t.Errorf("SGR colour must survive the body sanitizer, got %q", got)
	}
}
