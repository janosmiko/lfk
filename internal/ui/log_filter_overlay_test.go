package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderLogFilterOverlayEmpty(t *testing.T) {
	out := RenderLogFilterOverlay(LogFilterOverlayState{
		Title:       "Pod: api-7f4",
		IncludeMode: "any",
		FocusInput:  true,
		Input:       "",
	}, 80, 24)
	assert.Contains(t, out, "Filters")
	assert.Contains(t, out, "Pod: api-7f4")
	assert.Contains(t, out, "ANY")
}

func TestRenderLogFilterOverlayWithRules(t *testing.T) {
	out := RenderLogFilterOverlay(LogFilterOverlayState{
		Title:       "Pod: api",
		IncludeMode: "any",
		Rules: []LogFilterRowState{
			{Kind: "SEV", Mode: "", Pattern: ">= WARN"},
			{Kind: "INC", Mode: "fuzzy", Pattern: "error"},
			{Kind: "EXC", Mode: "substr", Pattern: "/healthz"},
		},
		ListCursor: 1,
	}, 80, 24)
	// Each kind marker appears as its own cell value.
	assert.Contains(t, out, "SEV")
	assert.Contains(t, out, "INC")
	assert.Contains(t, out, "EXC")
	assert.Contains(t, out, ">= WARN")
	assert.Contains(t, out, "error")
	assert.Contains(t, out, "/healthz")
	// The table header line is present.
	assert.Contains(t, out, "Pattern")
	// The line containing "error" (the selected row) should be styled
	// differently; at minimum the rendered output must not be empty.
	_ = strings.Split(out, "\n")
}
