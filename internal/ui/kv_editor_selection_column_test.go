package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// visualCol returns the visual column at which `needle` starts inside
// `line`. Counts grapheme width via lipgloss.Width so multi-byte UTF-8
// characters (box-drawing glyphs, the ✓ mark) count as one column —
// strings.Index returns the byte offset, which mis-aligns rows that
// contain different multi-byte runs above the column under test.
// Returns -1 when needle isn't found.
func visualCol(line, needle string) int {
	prefix, _, found := strings.Cut(line, needle)
	if !found {
		return -1
	}
	return lipgloss.Width(prefix)
}

// TestKVEditor_SelectionMarkInDedicatedColumn pins the layout change
// for the user's "the checkmark shifts the value by a space" report.
// Before the fix every row carried a 2-char prefix on the KEY cell:
// "  " when unselected, "✓ " when selected. The result was that the
// KEY column header sat flush at column 0 while every key value
// started 2 columns to the right — and unselected rows wasted those
// 2 columns rendering whitespace whether or not anything was marked.
//
// The fix promotes the marker to its own leading column. The header
// of the marker column is empty; selected rows show "✓" in that cell,
// unselected rows render an empty cell. Either way the KEY column's
// content aligns with the KEY header, and the value column doesn't
// shift when the user toggles selection.
func TestKVEditor_SelectionMarkInDedicatedColumn(t *testing.T) {
	secret := &model.SecretData{
		Keys: []string{"alpha", "bravo"},
		Data: map[string]string{"alpha": "a-val", "bravo": "b-val"},
	}

	// Selecting only the first row exercises both branches inside a
	// single render.
	selected := map[string]bool{"alpha": true}

	out := RenderSecretEditorOverlay(
		secret, 0, nil, true,
		false, "", 0, "", 0, 0, "", false,
		selected, false, 0, 0,
		120, 30,
	)
	plain := stripANSI(out)
	lines := strings.Split(plain, "\n")

	// Locate the KEY header line and the two data lines by content.
	headerIdx, alphaIdx, bravoIdx := -1, -1, -1
	for i, line := range lines {
		if strings.Contains(line, "KEY") && strings.Contains(line, "VALUE") {
			headerIdx = i
		}
		if strings.Contains(line, "alpha") && alphaIdx == -1 {
			alphaIdx = i
		}
		if strings.Contains(line, "bravo") && bravoIdx == -1 {
			bravoIdx = i
		}
	}
	if headerIdx < 0 || alphaIdx < 0 || bravoIdx < 0 {
		t.Fatalf("could not locate header/alpha/bravo rows in render:\n%s", plain)
	}

	keyHeaderCol := visualCol(lines[headerIdx], "KEY")
	alphaCol := visualCol(lines[alphaIdx], "alpha")
	bravoCol := visualCol(lines[bravoIdx], "bravo")

	// Both data rows must align with the header. Without the fix the
	// data rows landed 2 columns to the right (the "✓ " / "  " prefix
	// budget inside the KEY cell).
	assert.Equal(t, keyHeaderCol, alphaCol,
		"alpha key must align with KEY header; got header=%d alpha=%d", keyHeaderCol, alphaCol)
	assert.Equal(t, keyHeaderCol, bravoCol,
		"bravo key must align with KEY header; got header=%d bravo=%d", keyHeaderCol, bravoCol)

	// The checkmark must appear strictly to the LEFT of the KEY column
	// header — i.e. in the dedicated selection column, not inside the
	// KEY cell.
	checkCol := visualCol(lines[alphaIdx], "✓")
	if checkCol < 0 {
		t.Fatalf("alpha row must render the ✓ glyph:\n%s", lines[alphaIdx])
	}
	assert.Less(t, checkCol, keyHeaderCol,
		"selection mark must sit in its own column, before the KEY column; got mark=%d key=%d",
		checkCol, keyHeaderCol)

	// The bravo row (unselected) must NOT carry the mark.
	assert.NotContains(t, lines[bravoIdx], "✓",
		"unselected row must render an empty selection cell, not a checkmark")
}
