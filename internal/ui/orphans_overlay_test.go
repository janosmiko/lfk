package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOrphanChips_AlwaysShowsAllKinds verifies every kind chip renders
// regardless of count — the strip is a stable map of cluster orphan
// state so users can see "no PVCs are orphaned" at a glance instead of
// having to remember whether an absent chip means "no orphans" or
// "compact mode hid it".
func TestOrphanChips_AlwaysShowsAllKinds(t *testing.T) {
	counts := OrphanCounts{Pods: 3, Secrets: 2}
	out := renderOrphanChips(counts, 0, 200) // wide enough for one row

	assert.Contains(t, out, "All 5", "All chip shown with total")
	assert.Contains(t, out, "Pods 3")
	assert.Contains(t, out, "Secrets 2")
	assert.Contains(t, out, "CMs 0", "zero-count kinds still rendered")
	assert.Contains(t, out, "Svcs 0")
	assert.Contains(t, out, "PVCs 0")
	assert.Contains(t, out, "HPAs 0")
	assert.Contains(t, out, "PDBs 0")
	assert.Contains(t, out, "NetPols 0")
	assert.Contains(t, out, "Roles 0")
	assert.Contains(t, out, "RBs 0")
}

// TestOrphanChips_ActiveZeroCount covers the edge case where the user
// Tab-cycled onto an empty kind. Since every chip is always rendered,
// the active highlight still has a target — Tab cycling never lands
// on nothing.
func TestOrphanChips_ActiveZeroCount(t *testing.T) {
	counts := OrphanCounts{Pods: 3}          // only Pods has orphans
	out := renderOrphanChips(counts, 6, 200) // active = HPAs (idx=6), count=0

	assert.Contains(t, out, "HPAs 0", "active zero-count chip is shown")
	assert.Contains(t, out, "Pods 3", "non-zero still shown")
}

// TestOrphanChips_GridAlignmentOnWrap verifies the wrapped layout is a
// true grid: every chip cell is the same width, so column N of row 1
// lines up vertically with column N of row 2. Without that alignment
// the user reported the chips looked "random" mid-row.
func TestOrphanChips_GridAlignmentOnWrap(t *testing.T) {
	counts := OrphanCounts{
		Pods: 1, Secrets: 1, ConfigMaps: 1, Services: 1, PVCs: 1,
		HPAs: 1, PDBs: 1, NetworkPolicies: 1, Roles: 1, Bindings: 1,
	}
	out := renderOrphanChips(counts, 0, 50) // narrow → multi-row

	rows := strings.Split(out, "\n")
	require.Greater(t, len(rows), 1, "must wrap to multiple rows")

	// Each row must be the same visual width — short final rows pad
	// out to the grid width so column alignment is exact end-to-end.
	w0 := lipgloss.Width(rows[0])
	for i, r := range rows[1:] {
		assert.Equal(t, w0, lipgloss.Width(r),
			"row %d width should match row 0 for grid alignment", i+1)
	}
}

// TestOrphanChips_WrapsOnTypicalOverlay asserts that with all 11 kinds
// always rendered, the strip wraps to multiple rows on the typical
// 100-col overlay (rather than overflowing the inner content area).
func TestOrphanChips_WrapsOnTypicalOverlay(t *testing.T) {
	counts := OrphanCounts{Pods: 5, Secrets: 3}
	out := renderOrphanChips(counts, 0, 96) // overlay 100 - padding 4

	rows := strings.Split(out, "\n")
	require.Greater(t, len(rows), 1,
		"all 11 chips must wrap to multiple rows on a 96-col strip")
	for i, r := range rows {
		assert.LessOrEqual(t, lipgloss.Width(r), 96,
			"row %d must fit inside the requested width", i)
	}
}

// TestOrphanChipLines_MatchesRenderer pins the move-handler helper to
// the renderer's actual wrap row count for a representative width.
// When these disagree the cursor lands past the renderer's viewport.
func TestOrphanChipLines_MatchesRenderer(t *testing.T) {
	counts := OrphanCounts{Pods: 5, Secrets: 3}
	for _, width := range []int{40, 60, 96, 120, 200} {
		out := renderOrphanChips(counts, 0, width)
		got := OrphanChipLines(counts, width)
		want := strings.Count(out, "\n") + 1
		assert.Equalf(t, want, got,
			"width=%d: OrphanChipLines must equal renderer row count", width)
	}
}
