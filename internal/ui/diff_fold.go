package ui

import (
	"fmt"

	"charm.land/lipgloss/v2"
)

// DiffFoldRegion represents a contiguous run of unchanged lines that can be
// collapsed. ContextBefore and ContextAfter are the number of lines kept
// visible at each end of the region (typically 3).
type DiffFoldRegion struct {
	Start         int // index in the diffLine slice
	End           int // inclusive end index
	ContextBefore int // lines to keep visible at the start
	ContextAfter  int // lines to keep visible at the end
}

// HiddenCount returns the number of lines hidden when the region is collapsed.
func (r DiffFoldRegion) HiddenCount() int {
	return (r.End - r.Start + 1) - r.ContextBefore - r.ContextAfter
}

// VisibleDiffLine is a line produced after applying fold state. It either
// contains real diff content or is a fold placeholder.
type VisibleDiffLine struct {
	Original          int // index into the raw diffLine slice (-1 for fold placeholders)
	IsFoldPlaceholder bool
	HiddenCount       int // number of hidden lines (only set for placeholders)
	RegionIdx         int // fold region index (for toggle), -1 if none
}

// ComputeDiffFoldRegions scans diff lines and finds runs of consecutive
// unchanged ('=') lines longer than 6 lines. Each such run is a foldable
// region with 3 lines of context kept at each end.
func ComputeDiffFoldRegions(left, right string) []DiffFoldRegion {
	diffLines := computeDiff(left, right)
	return ComputeDiffFoldRegionsFromLines(diffLines)
}

// ComputeDiffFoldRegionsFromLines is ComputeDiffFoldRegions for a caller that
// already holds the diff. computeDiff builds an O(nxm) LCS table, so a caller
// needing both the regions and the raw lines must not pay for it twice.
func ComputeDiffFoldRegionsFromLines(diffLines []diffLine) []DiffFoldRegion {
	var regions []DiffFoldRegion
	const minRun = 4 // minimum unchanged run to make foldable
	const ctx = 1    // context lines at each end

	i := 0
	for i < len(diffLines) {
		if diffLines[i].status != '=' {
			i++
			continue
		}
		start := i
		for i < len(diffLines) && diffLines[i].status == '=' {
			i++
		}
		end := i - 1 // inclusive
		runLen := end - start + 1
		if runLen >= minRun {
			cb := ctx
			ca := ctx
			if cb+ca >= runLen {
				cb = runLen / 2
				ca = runLen - cb
			}
			regions = append(regions, DiffFoldRegion{
				Start:         start,
				End:           end,
				ContextBefore: cb,
				ContextAfter:  ca,
			})
		}
	}
	return regions
}

// BuildVisibleDiffLines takes raw diff lines and fold state and produces the
// list of visible lines with fold placeholders for collapsed regions.
func BuildVisibleDiffLines(diffLines []diffLine, regions []DiffFoldRegion, foldState []bool) []VisibleDiffLine {
	hidden := make(map[int]bool)
	type placeholder struct {
		afterIdx    int // insert placeholder after this original index
		hiddenCount int
		regionIdx   int
	}
	var placeholders []placeholder

	for ri, region := range regions {
		if ri >= len(foldState) || !foldState[ri] {
			continue
		}
		foldStart := region.Start + region.ContextBefore
		foldEnd := region.End - region.ContextAfter
		if foldStart > foldEnd {
			continue
		}
		for j := foldStart; j <= foldEnd; j++ {
			hidden[j] = true
		}
		placeholders = append(placeholders, placeholder{
			afterIdx:    foldStart - 1,
			hiddenCount: foldEnd - foldStart + 1,
			regionIdx:   ri,
		})
	}

	phMap := make(map[int]placeholder)
	for _, p := range placeholders {
		phMap[p.afterIdx] = p
	}

	var result []VisibleDiffLine
	for i := range diffLines {
		if hidden[i] {
			continue
		}
		regionIdx := -1
		for ri, region := range regions {
			if i >= region.Start && i <= region.End {
				regionIdx = ri
				break
			}
		}
		result = append(result, VisibleDiffLine{
			Original:  i,
			RegionIdx: regionIdx,
		})
		if p, ok := phMap[i]; ok {
			result = append(result, VisibleDiffLine{
				Original:          -1,
				IsFoldPlaceholder: true,
				HiddenCount:       p.hiddenCount,
				RegionIdx:         p.regionIdx,
			})
		}
	}
	return result
}

// DiffFoldPlaceholderText returns the styled text for a fold placeholder.
func DiffFoldPlaceholderText(hiddenCount int) string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorWarning)).
		Background(SurfaceBg).
		Bold(true)
	return style.Render(fmt.Sprintf("--- %d unchanged lines ---", hiddenCount))
}

// FindDiffFoldRegionAt returns the fold region index containing the given
// original diff line index, or -1 if none.
func FindDiffFoldRegionAt(regions []DiffFoldRegion, origIdx int) int {
	for i, r := range regions {
		if origIdx >= r.Start && origIdx <= r.End {
			return i
		}
	}
	return -1
}

// ExpandDiffFoldForLine ensures the fold region containing the given original
// diff line index is expanded (uncollapsed). Returns true if a change was made.
func ExpandDiffFoldForLine(regions []DiffFoldRegion, foldState []bool, origIdx int) bool {
	ri := FindDiffFoldRegionAt(regions, origIdx)
	if ri < 0 || ri >= len(foldState) {
		return false
	}
	if !foldState[ri] {
		return false
	}
	foldState[ri] = false
	return true
}

// highlightDiffSearchInLine applies search highlighting to a plain text line.
// Supports substring, regex, and fuzzy search modes via HighlightMatch.
// When isCurrent is true, the entire line uses SelectedSearchHighlightStyle
// (the current-match color) rather than the default search highlight.
func highlightDiffSearchInLine(line, query string, isCurrent bool) string {
	if query == "" {
		return line
	}
	if isCurrent {
		return HighlightMatchStyled(line, query, SelectedSearchHighlightStyle)
	}
	return HighlightMatch(line, query)
}
