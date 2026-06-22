package ui

import (
	"strings"
	"testing"
)

func TestPlaceOverlayBottom(t *testing.T) {
	bg := strings.Repeat("background\n", 10)
	overlay := "AAAA\nBBBB"
	out := PlaceOverlayBottom(20, 10, 0, overlay, bg)
	lines := strings.Split(out, "\n")
	if len(lines) != 10 {
		t.Fatalf("expected 10 lines, got %d", len(lines))
	}
	// Overlay occupies the last two rows; earlier rows stay as background.
	if !strings.Contains(lines[8], "AAAA") || !strings.Contains(lines[9], "BBBB") {
		t.Fatalf("overlay not anchored to bottom:\n%s", out)
	}
	if !strings.Contains(lines[0], "background") {
		t.Fatalf("top rows must remain background:\n%s", out)
	}
}

func TestPlaceOverlayBottom_Margin(t *testing.T) {
	bg := strings.Repeat("background\n", 10)
	overlay := "AAAA\nBBBB"
	out := PlaceOverlayBottom(20, 10, 2, overlay, bg)
	lines := strings.Split(out, "\n")
	// With a 2-row bottom margin the overlay sits on rows 6 and 7, and the
	// last two rows stay as background.
	if !strings.Contains(lines[6], "AAAA") || !strings.Contains(lines[7], "BBBB") {
		t.Fatalf("overlay not lifted by margin:\n%s", out)
	}
	if !strings.Contains(lines[9], "background") {
		t.Fatalf("bottom margin rows must remain background:\n%s", out)
	}
}
