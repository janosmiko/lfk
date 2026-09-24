package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The initial search prompt (no pattern entered yet) shows the active
// search-mode label instead of the plain "/ " placeholder.
func TestRenderFinalizerSearchOverlay_InitialPromptShowsModeLabel(t *testing.T) {
	out := stripANSI(RenderFinalizerSearchOverlay(
		nil, 0, map[string]bool{}, "", "~pvc", false, false, 80, 20,
	))
	assert.Contains(t, out, "[fuzzy]", "initial prompt must show the fuzzy mode label for a ~ query")
}

// The results footer's filter row is mode-aware for both the active
// (cursor block) and inactive (committed value) filter states.
func TestRenderFinalizerSearchOverlay_ResultsFooterShowsModeLabel(t *testing.T) {
	entries := []FinalizerMatchEntry{{Name: "svc", Namespace: "default", Kind: "Pod", Matched: "kubernetes.io/pvc-protection", Age: "1d"}}

	active := stripANSI(RenderFinalizerSearchOverlay(
		entries, 0, map[string]bool{}, "pvc", "~svc", true, false, 80, 20,
	))
	assert.Contains(t, active, "[fuzzy]", "active filter footer must show the fuzzy mode label for a ~ query")

	committed := stripANSI(RenderFinalizerSearchOverlay(
		entries, 0, map[string]bool{}, "pvc", "err.*", false, false, 80, 20,
	))
	assert.Contains(t, committed, "[regex]", "committed filter footer must show the regex mode label for a regex-meta query")
}
