package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// TestStyledNameCellDimsIgnoredRows verifies that a row tagged __ignored__
// (a revealed-but-ignored security finding) renders with distinct, dimmed
// styling versus an otherwise identical active row, so the show-ignored view
// visually distinguishes ignored entries.
func TestStyledNameCellDimsIgnoredRows(t *testing.T) {
	active := model.Item{Kind: "__security_finding_group__", Name: "CVE-2024-1"}
	ignored := model.Item{
		Kind:    "__security_finding_group__",
		Name:    "CVE-2024-1",
		Columns: []model.KeyValue{{Key: "__ignored__", Value: "true"}},
	}

	got := styledNameCell(ignored, 30, nil)
	want := styledNameCell(active, 30, nil)
	assert.NotEqual(t, want, got, "an __ignored__ row must render with distinct (dim) styling")
}
