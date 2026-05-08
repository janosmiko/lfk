package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestRenderTrafficCaptureOverlay_PhaseConfig_ShowsBackendChips(t *testing.T) {
	e := CaptureOverlayEntry{
		Title: "Traffic Capture — ns/pod-xyz",
		Phase: CapturePhaseConfig,
		Backends: []CaptureBackendChip{
			{Label: "kubectl debug", Selected: true, Available: true},
			{Label: "kubeshark", Available: false, Reason: "not detected"},
		},
		Interface:          "any",
		SnapLen:            65535,
		FilterPresets:      []string{"all", "DNS", "HTTP/S", "no kube internals"},
		FilterPresetCursor: 0,
	}
	out := RenderTrafficCaptureOverlay(e, 90, 30)
	for _, want := range []string{"kubectl debug", "kubeshark", "65535", "any"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRenderTrafficCaptureOverlay_PhaseEndpointPick_RendersList(t *testing.T) {
	e := CaptureOverlayEntry{
		Title: "Traffic Capture — ns/svc-foo · pick backing pod",
		Phase: CapturePhaseEndpointPick,
		EndpointRows: []CaptureEndpointRow{
			{PodName: "pod-foo-7d8c-abc12", IP: "10.0.1.5", Ready: true},
			{PodName: "pod-foo-7d8c-def34", IP: "10.0.1.6", Ready: false},
		},
		EndpointCursor: 0,
	}
	out := RenderTrafficCaptureOverlay(e, 90, 30)
	for _, want := range []string{"pod-foo-7d8c-abc12", "10.0.1.5", "pod-foo-7d8c-def34", "not-ready"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRenderTrafficCaptureOverlay_PhaseLive_ShowsPackets(t *testing.T) {
	e := CaptureOverlayEntry{
		Title:       "Traffic Capture — ns/pod-xyz",
		HeaderHints: "iface=any · backend=kubectl-debug",
		FilterText:  "filter: port 443",
		StatusBadge: "● capturing 12s",
		Phase:       CapturePhaseLive,
		PacketCount: 1234,
		ByteCount:   2400000,
		OutputPath:  "/tmp/test.pcap",
		Packets: []PacketRow{
			{Time: "12:34:56.123", Protocol: "TCP", Src: "10.0.0.4:51234", Dst: "10.0.1.5:443", Length: 120, Flags: "PSH ACK"},
			{Time: "12:34:56.124", Protocol: "TCP", Src: "10.0.1.5:443", Dst: "10.0.0.4:51234", Length: 1480, Flags: "ACK"},
		},
	}
	out := RenderTrafficCaptureOverlay(e, 90, 30)
	for _, want := range []string{"1234", "10.0.0.4:51234", "10.0.1.5:443", "PSH ACK", "● capturing"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRenderTrafficCaptureOverlay_PhaseLive_StatusOnlyHidesTable(t *testing.T) {
	e := CaptureOverlayEntry{
		Title:          "Traffic Capture — ns/pod-xyz",
		Phase:          CapturePhaseLive,
		ShowStatusOnly: true,
		PacketCount:    1234,
		Packets:        []PacketRow{{Time: "x", Protocol: "TCP", Src: "a", Dst: "b", Length: 1}},
	}
	out := RenderTrafficCaptureOverlay(e, 90, 30)
	// status-only mode hides the entire table header; either token leaking
	// would mean the renderer painted the header row when it shouldn't.
	if strings.Contains(out, "TIME") || strings.Contains(out, "PROTO") {
		t.Errorf("status-only mode should hide the table header; got:\n%s", out)
	}
}

func TestRenderTrafficCaptureOverlay_PhaseStopped_ShowsBadge(t *testing.T) {
	// Hint bar moved to the bottom-of-screen hint bar (see
	// overlayHintBarOverlayTrafficCapture); this renderer no longer paints
	// inline keymap hints. We assert only the status badge here — the keymap
	// hint coverage lives in app/overlay_hintbar_test.go.
	e := CaptureOverlayEntry{
		Title:       "Traffic Capture — ns/pod-xyz",
		Phase:       CapturePhaseStopped,
		StatusBadge: "■ stopped",
	}
	out := RenderTrafficCaptureOverlay(e, 90, 30)
	if !strings.Contains(out, "■ stopped") {
		t.Errorf("missing ■ stopped:\n%s", out)
	}
}

// TestRenderTrafficCaptureOverlay_LivePhase_BoundedByContentH guards that
// the rendered output never exceeds contentH lines (otherwise the overlay
// box visibly grows on every new packet — the bug the user reported).
// Also exercises the new tail-anchored ScrollOffset semantics:
//
//   - ScrollOffset=0 → latest packets visible (auto-follow).
//   - Large ScrollOffset → older packets visible; renderer clamps to
//     "oldest fits at top".
func TestRenderTrafficCaptureOverlay_LivePhase_BoundedByContentH(t *testing.T) {
	packets := make([]PacketRow, 200)
	for i := range packets {
		packets[i] = PacketRow{
			Time: "12:00:00.000", Protocol: "TCP",
			Src: "1.2.3.4:1", Dst: "5.6.7.8:443", Length: 64, Flags: "ACK",
		}
	}
	// Distinct markers at both ends so we can assert which window is visible.
	packets[0] = PacketRow{
		Time: "12:00:00.000", Protocol: "OLDEST",
		Src: "1.1.1.1:1", Dst: "1.1.1.1:1", Length: 11, Flags: "",
	}
	packets[len(packets)-1] = PacketRow{
		Time: "12:00:99.999", Protocol: "LATEST",
		Src: "9.9.9.9:9", Dst: "9.9.9.9:9", Length: 99, Flags: "",
	}
	contentH := 12
	e := CaptureOverlayEntry{
		Title:        "T",
		Phase:        CapturePhaseLive,
		PacketCount:  int64(len(packets)),
		Packets:      packets,
		ScrollOffset: 0, // tail -f
	}
	out := RenderTrafficCaptureOverlay(e, 90, contentH)
	if got := countRenderedLines(out); got > contentH {
		t.Errorf("rendered %d lines, want at most contentH=%d so the overlay box doesn't grow", got, contentH)
	}
	if !strings.Contains(out, "LATEST") {
		t.Errorf("ScrollOffset=0 should be tail -f mode (latest visible); output:\n%s", out)
	}
	if strings.Contains(out, "OLDEST") {
		t.Errorf("ScrollOffset=0 should not show oldest packet; output:\n%s", out)
	}

	// Scroll all the way back: an oversized ScrollOffset must clamp to "oldest
	// packets visible". This is the g (jump-to-top) behaviour.
	e.ScrollOffset = 1 << 30
	scrolled := RenderTrafficCaptureOverlay(e, 90, contentH)
	if !strings.Contains(scrolled, "OLDEST") {
		t.Errorf("oversized ScrollOffset should clamp to oldest-visible; output:\n%s", scrolled)
	}
	if strings.Contains(scrolled, "LATEST") {
		t.Errorf("oversized ScrollOffset should hide the latest packet; output:\n%s", scrolled)
	}
}

// countRenderedLines returns the number of visible lines in s. Plain
// strings.Count(s, "\n") undercounts by 1 when the last line lacks a
// trailing newline (which the renderer doesn't always emit), letting an
// off-by-one regression slip past the contentH bound check.
func countRenderedLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

// TestRenderTrafficCaptureOverlay_NoInlineHintKeys guards the project-wide
// convention: keymap hints belong in the bottom hint bar, never inline. A
// regression that re-adds inline hints would surface as one of the well-known
// hint substrings appearing in this renderer's output.
func TestRenderTrafficCaptureOverlay_NoInlineHintKeys(t *testing.T) {
	for _, phase := range []CaptureOverlayPhase{
		CapturePhaseConfig, CapturePhaseEndpointPick,
		CapturePhaseLive, CapturePhaseStopped,
	} {
		e := CaptureOverlayEntry{
			Title:         "T",
			Phase:         phase,
			Backends:      []CaptureBackendChip{{Label: "kubectl debug", Selected: true, Available: true}},
			FilterPresets: []string{"all"},
		}
		out := RenderTrafficCaptureOverlay(e, 90, 30)
		for _, banned := range []string{
			"Enter start", "Enter restart", "Esc close",
			"s stop", "t status-only", "j/k scroll",
		} {
			if strings.Contains(out, banned) {
				t.Errorf("phase=%v: inline hint %q should live in bottom hint bar; output:\n%s", phase, banned, out)
			}
		}
	}
}

func TestRenderTrafficCaptureOverlay_NoInnerBorder(t *testing.T) {
	// The renderer must NOT add its own border or padding — the caller
	// (view_overlays.go) wraps the result with OverlayStyle. Adding an inner
	// border on top of OverlayStyle's outer border doubles the chrome and
	// causes lines to soft-wrap on narrow terminals (the bug the user reported).
	originalProfile := lipgloss.DefaultRenderer().ColorProfile()
	t.Cleanup(func() {
		lipgloss.DefaultRenderer().SetColorProfile(originalProfile)
		ApplyTheme(DefaultTheme())
	})
	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)
	ApplyTheme(DefaultTheme())
	lipgloss.DefaultRenderer().SetColorProfile(termenv.TrueColor)

	e := CaptureOverlayEntry{
		Title: "Traffic Capture — ns/pod-xyz",
		Phase: CapturePhaseConfig,
		Backends: []CaptureBackendChip{
			{Label: "kubectl debug", Selected: true, Available: true},
		},
		FilterPresets: []string{"all"},
	}
	rendered := RenderTrafficCaptureOverlay(e, 90, 30)
	// Verify the renderer does NOT emit rounded-border corner glyphs anywhere
	// in its output — those corners would mean a redundant border was painted.
	for _, glyph := range []string{"╭", "╮", "╰", "╯"} {
		if strings.Contains(rendered, glyph) {
			t.Errorf("renderer output contains border glyph %q; OverlayStyle owns the border, the inner renderer must not paint one", glyph)
		}
	}
}

func TestRenderTrafficCaptureOverlay_LinesFitContentWidth(t *testing.T) {
	// All horizontal-divider lines must be exactly contentW visible runes wide
	// so the line doesn't soft-wrap inside OverlayStyle's padding budget.
	contentW := 50
	e := CaptureOverlayEntry{
		Title:              "T",
		Phase:              CapturePhaseConfig,
		Backends:           []CaptureBackendChip{{Label: "kubectl debug", Selected: true, Available: true}},
		Interface:          "any",
		SnapLen:            65535,
		FilterPresets:      []string{"all"},
		FilterPresetCursor: 0,
	}
	out := RenderTrafficCaptureOverlay(e, contentW, 30)
	// Count the longest run of `─` in the output — should match contentW.
	runs := 0
	for line := range strings.SplitSeq(out, "\n") {
		count := strings.Count(line, "─")
		if count > runs {
			runs = count
		}
	}
	if runs != contentW {
		t.Errorf("longest divider = %d runes, want %d (contentW)", runs, contentW)
	}
}
