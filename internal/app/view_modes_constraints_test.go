package app

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/janosmiko/lfk/internal/k8s"
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

func TestFormatConstraintRow_FitsTheAvailableWidth(t *testing.T) {
	row := k8s.ConstraintRow{
		Source:    "Quota",
		Kind:      "ResourceQuota",
		Namespace: "a-rather-long-namespace-name",
		Name:      "compute-quota-with-a-long-name",
		Headroom:  "1500m",
		Detail:    "requests 500m cpu (hard 4, used 3) and then some more text to overflow",
	}

	for _, width := range []int{20, 40, 60, 80, 120, 200} {
		t.Run(fmt.Sprintf("width-%d", width), func(t *testing.T) {
			line := stripANSI(formatConstraintRow(row, true, constraintsColumns(width)))
			if got := lipgloss.Width(line); got > width {
				t.Errorf("row width = %d, want at most %d: %q", got, width, line)
			}
		})
	}
}

func TestConstraintsBanner_EmptyWhenNothingSkippedOrFailed(t *testing.T) {
	if out := constraintsBanner(nil, nil); out != "" {
		t.Errorf("expected an empty banner, got %q", out)
	}
}
