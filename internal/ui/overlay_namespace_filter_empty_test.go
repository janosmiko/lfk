package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// Contract reminder: the namespace overlay distinguishes nil (pre-fetch
// loader) from empty slice (after-fetch / after-filter with no
// matches). Callers must pass an empty slice for the no-match case, not
// nil, otherwise the user sees the loading spinner forever.
func TestRenderNamespaceOverlay_EmptySliceShowsNoMatch(t *testing.T) {
	out := RenderNamespaceOverlay([]model.Item{}, "weird-text", 0, "default", false, nil, false)
	plain := stripANSI(out)
	assert.NotContains(t, plain, "Loading",
		"empty slice with active filter must not look like the pre-fetch loader")
	assert.Contains(t, plain, "No matching")
}
