package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The cluster-color picker's filter row is mode-aware: a ~ query shows
// the fuzzy label instead of the plain "/" prompt.
func TestRenderClusterColorOverlay_FilterShowsModeLabel(t *testing.T) {
	m := Model{
		clusterColorOverlayContext: "prod-eu",
		clusterColorFilterMode:     true,
	}
	m.clusterColorFilter.Insert("~re")

	out := stripANSI(renderClusterColorOverlay(m, 60, 20))
	assert.Contains(t, out, "[fuzzy]", "cluster color filter must show the fuzzy mode label for a ~ query")
}
