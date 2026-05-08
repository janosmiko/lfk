package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/k8s"
)

func TestCaptureRing_PushAndSnapshot(t *testing.T) {
	r := newCaptureRing(3)
	r.Push(k8s.PacketSummary{Protocol: "TCP"})
	r.Push(k8s.PacketSummary{Protocol: "UDP"})
	got := r.Snapshot()
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Protocol != "TCP" || got[1].Protocol != "UDP" {
		t.Errorf("order wrong: %v", got)
	}
}

func TestCaptureRing_OverflowEvictsOldest(t *testing.T) {
	r := newCaptureRing(2)
	r.Push(k8s.PacketSummary{Protocol: "TCP"})
	r.Push(k8s.PacketSummary{Protocol: "UDP"})
	r.Push(k8s.PacketSummary{Protocol: "DNS"})
	got := r.Snapshot()
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Protocol != "UDP" || got[1].Protocol != "DNS" {
		t.Errorf("eviction wrong: %v", got)
	}
}

func TestCaptureRing_FullRingExactlyCap(t *testing.T) {
	r := newCaptureRing(2)
	r.Push(k8s.PacketSummary{Protocol: "A"})
	r.Push(k8s.PacketSummary{Protocol: "B"})
	got := r.Snapshot()
	if len(got) != 2 || got[0].Protocol != "A" || got[1].Protocol != "B" {
		t.Errorf("got %v, want [A B]", got)
	}
}
