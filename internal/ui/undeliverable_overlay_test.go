package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestUndeliverableBodyHeight(t *testing.T) {
	// Chrome is title + subtitle + filter row + blank = 4 lines inside the
	// overlay's 1+1 vertical padding.
	if got := UndeliverableBodyHeight(20); got != 14 {
		t.Errorf("UndeliverableBodyHeight(20) = %d, want 14", got)
	}
	if got := UndeliverableBodyHeight(3); got != 1 {
		t.Errorf("UndeliverableBodyHeight(3) = %d, want 1 (floor)", got)
	}
}

// TestRenderUndeliverableOverlayHeightIsFixed guards the invariant the move
// handler depends on: the box does not change size as rows come and go.
func TestRenderUndeliverableOverlayHeightIsFixed(t *testing.T) {
	rows := make([]UndeliverableRow, 40)
	for i := range rows {
		rows[i] = UndeliverableRow{Kind: "Pod", Namespace: "web", Name: "p", Reason: "FailedScheduling"}
	}
	cases := map[string][]UndeliverableRow{"empty": nil, "full": rows}
	for name, in := range cases {
		out := RenderUndeliverableOverlay(in, 0, 0, 100, 20, "", false, false, "")
		if got := strings.Count(out, "\n") + 1; got != 18 {
			t.Errorf("%s: rendered %d lines, want 18 (height-2)", name, got)
		}
	}
}

func TestRenderUndeliverableOverlayShowsReasonAndScopes(t *testing.T) {
	rows := []UndeliverableRow{
		{Kind: "Pod", Namespace: "web", Name: "api-0", Reason: "FailedScheduling: no nodes"},
		{Kind: "Ingress", Namespace: "web", Name: "site", Reason: "no address in status.loadBalancer"},
	}
	out := stripANSI(RenderUndeliverableOverlay(rows, 0, 0, 120, 20, "", false, false, ""))
	for _, want := range []string{
		"Undeliverable", "api-0", "FailedScheduling: no nodes",
		"site", "no address in status.loadBalancer",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
}

func TestRenderUndeliverableOverlayLoadingAndEmpty(t *testing.T) {
	loading := stripANSI(RenderUndeliverableOverlay(nil, 0, 0, 100, 20, "", false, true, ""))
	if !strings.Contains(loading, "Scanning cluster") {
		t.Errorf("loading state missing scan notice\n%s", loading)
	}
	empty := stripANSI(RenderUndeliverableOverlay(nil, 0, 0, 100, 20, "", false, false, ""))
	if !strings.Contains(empty, "Nothing is stuck waiting") {
		t.Errorf("empty state missing message\n%s", empty)
	}
}

func TestRenderUndeliverableOverlayShowsPartialError(t *testing.T) {
	out := stripANSI(RenderUndeliverableOverlay(nil, 0, 0, 100, 20, "", false, false, "listing pvcs: forbidden"))
	if !strings.Contains(out, "partial result") || !strings.Contains(out, "listing pvcs: forbidden") {
		t.Errorf("partial banner missing\n%s", out)
	}
}

// TestRenderUndeliverableOverlaySanitizesClusterText covers the four
// cluster-sourced fields on a row plus the partial-error banner. Every one
// of them can carry terminal escapes: reason and finalizer text come
// straight out of an Event or an object's metadata.
func TestRenderUndeliverableOverlaySanitizesClusterText(t *testing.T) {
	hostile := []UndeliverableRow{{
		Kind:      "Pod\x1b]52;c;YWJj\a",
		Namespace: "we\u202eb",
		Name:      "api\x1b[2J",
		Reason:    "FailedScheduling:\x07 nope\u2066",
	}}
	out := RenderUndeliverableOverlay(hostile, 0, 0, 120, 20, "", false, false,
		"listing pvcs:\x1b]0;pwned\a forbidden")

	for _, bad := range []string{"\x1b]", "\x1b[2J", "\a", "\u202e", "\u2066"} {
		if strings.Contains(out, bad) {
			t.Errorf("output leaked %q", bad)
		}
	}
	// The printable remainder survives - the sanitizers drop control bytes,
	// not text, so a stripped row is still identifiable.
	if !strings.Contains(stripANSI(out), "FailedScheduling") {
		t.Errorf("sanitizing removed the printable reason\n%s", out)
	}
}

// TestRenderUndeliverableOverlaySanitizesFilterEcho covers the one field the
// user controls but that still round-trips through the renderer: a pasted
// filter query can carry the same escapes a cluster value can.
func TestRenderUndeliverableOverlaySanitizesFilterEcho(t *testing.T) {
	out := RenderUndeliverableOverlay(nil, 0, 0, 100, 20, "web\x1b]52;c;YWJj\a", true, false, "")
	if strings.Contains(out, "\x1b]") || strings.Contains(out, "\a") {
		t.Errorf("filter echo leaked an escape introducer")
	}
}

func TestRenderUndeliverableOverlayScrollsToCursor(t *testing.T) {
	rows := make([]UndeliverableRow, 60)
	for i := range rows {
		rows[i] = UndeliverableRow{Kind: "Pod", Namespace: "web", Name: "pod-" + string(rune('a'+i%26))}
	}
	// Scroll past the first window; the top row must be the scrolled-to one.
	out := stripANSI(RenderUndeliverableOverlay(rows, 30, 30, 100, 20, "", false, false, ""))
	if strings.Contains(out, "pod-a  ") {
		t.Errorf("scrolled render still shows the first row\n%s", out)
	}
}

// TestUndeliverableScrollForCursorUsesVimScrollOff guards the repo
// convention (CLAUDE.md: "any movable cursor must scroll to stay visible -
// use ui.VimScrollOff") - a hand-rolled edge clamp gives the cursor no
// scrolloff margin, unlike every other overlay's list.
// TestUndeliverableItemsPadsByDisplayWidthNotRuneCount guards the KIND /
// NAMESPACE / NAME column alignment against wide Unicode. "ab" and "文" both
// occupy 2 display columns but differ in rune count (2 vs 1); padding by
// rune count would give the "文" row one extra column and drift every
// column after it.
func TestUndeliverableItemsPadsByDisplayWidthNotRuneCount(t *testing.T) {
	rows := []UndeliverableRow{
		{Kind: "Pod", Namespace: "ab", Name: "x"},
		{Kind: "Pod", Namespace: "文", Name: "x"},
	}
	items := undeliverableItems(rows, 120)

	w0, w1 := lipgloss.Width(items[0].Name), lipgloss.Width(items[1].Name)
	if w0 != w1 {
		t.Errorf("row display widths differ: ascii=%d wide=%d - wide namespace desynced the columns", w0, w1)
	}
}

func TestUndeliverableScrollForCursorUsesVimScrollOff(t *testing.T) {
	identity := func(from, to int) int { return to - from }
	scroll, cursor, bodyHeight, total := 5, 8, 10, 40

	want := VimScrollOff(scroll, cursor, total, bodyHeight, ConfigScrollOff, identity)
	got := UndeliverableScrollForCursor(scroll, cursor, bodyHeight, total)

	if got != want {
		t.Errorf("UndeliverableScrollForCursor = %d, want %d (ui.VimScrollOff result)", got, want)
	}
}

// TestUndeliverableScrollForCursorClampsToWindow pins the wrapper's
// wiring (scroll/cursor/bodyHeight/total pass straight through to
// ui.VimScrollOff) against the same inputs ui.VimScrollOff's own tests
// cover with a scrolloff margin, plus the always-true edge case where the
// whole list fits in the viewport.
func TestUndeliverableScrollForCursorClampsToWindow(t *testing.T) {
	identity := func(from, to int) int { return to - from }

	// Cursor near the bottom edge: VimScrollOff's default margin (5) pulls
	// the window forward rather than pinning the cursor to the last row.
	want := VimScrollOff(5, 7, 40, 10, ConfigScrollOff, identity)
	if got := UndeliverableScrollForCursor(5, 7, 10, 40); got != want {
		t.Errorf("scroll = %d, want %d", got, want)
	}
	want = VimScrollOff(5, 20, 40, 10, ConfigScrollOff, identity)
	if got := UndeliverableScrollForCursor(5, 20, 10, 40); got != want {
		t.Errorf("scroll = %d, want %d", got, want)
	}
	// Whole list fits: pinned at the top.
	if got := UndeliverableScrollForCursor(4, 2, 10, 8); got != 0 {
		t.Errorf("scroll = %d, want 0", got)
	}
}
