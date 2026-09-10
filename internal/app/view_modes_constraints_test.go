package app

import (
	"strings"
	"testing"
)

func TestConstraintsBanner_DistinguishesDeniedFromFailed(t *testing.T) {
	out := stripANSI(constraintsBanner([]string{"poddisruptionbudgets"}, []string{"nodes"}))

	if !strings.Contains(out, "denied") || !strings.Contains(out, "poddisruptionbudgets") {
		t.Errorf("expected the denied group to name poddisruptionbudgets, got %q", out)
	}
	if !strings.Contains(out, "failed") || !strings.Contains(out, "nodes") {
		t.Errorf("expected the failed group to name nodes, got %q", out)
	}
}

func TestConstraintsBanner_EmptyWhenNothingSkippedOrFailed(t *testing.T) {
	if out := constraintsBanner(nil, nil); out != "" {
		t.Errorf("expected an empty banner, got %q", out)
	}
}
