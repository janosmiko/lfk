package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputeDiffFoldRegions(t *testing.T) {
	t.Run("no foldable regions for short unchanged runs", func(t *testing.T) {
		left := "a\nb\nc"
		right := "a\nb\nc"
		regions := ComputeDiffFoldRegions(left, right)
		assert.Empty(t, regions)
	})

	t.Run("foldable region for long unchanged run", func(t *testing.T) {
		lines := make([]string, 10)
		for i := range lines {
			lines[i] = "unchanged-line"
		}
		content := strings.Join(lines, "\n")
		regions := ComputeDiffFoldRegions(content, content)
		assert.Len(t, regions, 1)
		assert.Equal(t, 0, regions[0].Start)
		assert.Equal(t, 9, regions[0].End)
		assert.Equal(t, 1, regions[0].ContextBefore)
		assert.Equal(t, 1, regions[0].ContextAfter)
		assert.Equal(t, 8, regions[0].HiddenCount())
	})

	t.Run("no region when all lines differ", func(t *testing.T) {
		left := "a\nb\nc"
		right := "x\ny\nz"
		regions := ComputeDiffFoldRegions(left, right)
		assert.Empty(t, regions)
	})

	t.Run("multiple foldable regions", func(t *testing.T) {
		leftLines := make([]string, 0, 21)
		rightLines := make([]string, 0, 21)
		for i := range 10 {
			leftLines = append(leftLines, "same")
			rightLines = append(rightLines, "same")
			_ = i
		}
		leftLines = append(leftLines, "only-left")
		rightLines = append(rightLines, "only-right")
		for range 10 {
			leftLines = append(leftLines, "same")
			rightLines = append(rightLines, "same")
		}
		left := strings.Join(leftLines, "\n")
		right := strings.Join(rightLines, "\n")
		regions := ComputeDiffFoldRegions(left, right)
		assert.Len(t, regions, 2)
	})
}

func TestBuildVisibleDiffLines(t *testing.T) {
	lines := make([]string, 10)
	for i := range lines {
		lines[i] = "line"
	}
	content := strings.Join(lines, "\n")
	diffLines := computeDiff(content, content)
	regions := computeDiffFoldRegionsFromLines(diffLines)

	t.Run("no folds applied shows all lines", func(t *testing.T) {
		visLines := BuildVisibleDiffLines(diffLines, regions, nil)
		assert.Len(t, visLines, len(diffLines))
		for _, vl := range visLines {
			assert.False(t, vl.IsFoldPlaceholder)
		}
	})

	t.Run("collapsed region hides lines and adds placeholder", func(t *testing.T) {
		foldState := []bool{true}
		visLines := BuildVisibleDiffLines(diffLines, regions, foldState)
		// Should be less than original due to hidden lines.
		assert.Less(t, len(visLines), len(diffLines))

		// Should have exactly one placeholder.
		placeholderCount := 0
		for _, vl := range visLines {
			if vl.IsFoldPlaceholder {
				placeholderCount++
				assert.Greater(t, vl.HiddenCount, 0)
			}
		}
		assert.Equal(t, 1, placeholderCount)
	})
}

func TestDiffFoldPlaceholderText(t *testing.T) {
	result := DiffFoldPlaceholderText(5)
	assert.Contains(t, result, "5 unchanged lines")
}

func TestFindDiffFoldRegionAt(t *testing.T) {
	regions := []DiffFoldRegion{
		{Start: 0, End: 9, ContextBefore: 3, ContextAfter: 3},
		{Start: 15, End: 25, ContextBefore: 3, ContextAfter: 3},
	}

	t.Run("finds region containing index", func(t *testing.T) {
		assert.Equal(t, 0, FindDiffFoldRegionAt(regions, 5))
		assert.Equal(t, 1, FindDiffFoldRegionAt(regions, 20))
	})

	t.Run("returns -1 for index outside regions", func(t *testing.T) {
		assert.Equal(t, -1, FindDiffFoldRegionAt(regions, 12))
	})
}

func TestExpandDiffFoldForLine(t *testing.T) {
	regions := []DiffFoldRegion{
		{Start: 0, End: 9, ContextBefore: 3, ContextAfter: 3},
	}

	t.Run("expands collapsed region", func(t *testing.T) {
		foldState := []bool{true}
		changed := ExpandDiffFoldForLine(regions, foldState, 5)
		assert.True(t, changed)
		assert.False(t, foldState[0])
	})

	t.Run("no change for already expanded region", func(t *testing.T) {
		foldState := []bool{false}
		changed := ExpandDiffFoldForLine(regions, foldState, 5)
		assert.False(t, changed)
	})

	t.Run("no change for index outside regions", func(t *testing.T) {
		foldState := []bool{true}
		changed := ExpandDiffFoldForLine(regions, foldState, 15)
		assert.False(t, changed)
	})
}

func TestHighlightDiffSearchInLine(t *testing.T) {
	t.Run("empty query returns original", func(t *testing.T) {
		result := highlightDiffSearchInLine("hello world", "")
		assert.Equal(t, "hello world", result)
	})

	t.Run("no match returns original", func(t *testing.T) {
		result := highlightDiffSearchInLine("hello world", "xyz")
		assert.Equal(t, "hello world", result)
	})

	t.Run("match is highlighted", func(t *testing.T) {
		result := highlightDiffSearchInLine("hello world", "world")
		assert.Contains(t, result, "world")
		// Length should be at least as long as original (may have ANSI codes).
		assert.GreaterOrEqual(t, len(result), len("hello world"))
	})

	t.Run("case insensitive match", func(t *testing.T) {
		result := highlightDiffSearchInLine("Hello World", "hello")
		assert.Contains(t, result, "Hello")
		assert.GreaterOrEqual(t, len(result), len("Hello World"))
	})
}

func TestUpdateDiffSearchMatches(t *testing.T) {
	t.Run("empty query returns nil", func(t *testing.T) {
		matches := UpdateDiffSearchMatches("a\nb", "a\nb", "")
		assert.Nil(t, matches)
	})

	t.Run("finds matches in unchanged lines", func(t *testing.T) {
		matches := UpdateDiffSearchMatches("name: test\nvalue: hello", "name: test\nvalue: hello", "test")
		assert.Len(t, matches, 1)
		assert.Equal(t, 0, matches[0])
	})

	t.Run("finds matches in both sides", func(t *testing.T) {
		matches := UpdateDiffSearchMatches("name: test", "name: other", "name")
		assert.Greater(t, len(matches), 0)
	})

	t.Run("case insensitive", func(t *testing.T) {
		matches := UpdateDiffSearchMatches("Name: Test", "Name: Test", "name")
		assert.Len(t, matches, 1)
	})
}

func TestDiffVisibleIndexForOriginal(t *testing.T) {
	lines := make([]string, 10)
	for i := range lines {
		lines[i] = "line"
	}
	content := strings.Join(lines, "\n")

	t.Run("no folds returns same index", func(t *testing.T) {
		idx := DiffVisibleIndexForOriginal(content, content, nil, nil, 5)
		assert.Equal(t, 5, idx)
	})

	t.Run("returns -1 for hidden line", func(t *testing.T) {
		regions := ComputeDiffFoldRegions(content, content)
		foldState := make([]bool, len(regions))
		for i := range foldState {
			foldState[i] = true
		}
		// Line 5 is in the middle of a fold region, should be hidden.
		idx := DiffVisibleIndexForOriginal(content, content, regions, foldState, 5)
		assert.Equal(t, -1, idx)
	})
}

func TestRenderDiffViewWithSearch(t *testing.T) {
	left := "apiVersion: v1\nkind: Pod\nmetadata:\n  name: test"
	right := "apiVersion: v2\nkind: Pod\nmetadata:\n  name: test"

	t.Run("search query appears in title", func(t *testing.T) {
		result := RenderDiffView(left, right, "a", "b", 0, 120, 30, false, "Pod", nil, nil, false, "", 0, DiffVisualParams{})
		assert.Contains(t, result, "[/Pod]")
	})

	t.Run("search mode shows search bar", func(t *testing.T) {
		result := RenderDiffView(left, right, "a", "b", 0, 120, 30, false, "", nil, nil, true, "test", 0, DiffVisualParams{})
		assert.Contains(t, result, "type: search")
	})
}

func TestRenderUnifiedDiffViewWithSearch(t *testing.T) {
	left := "apiVersion: v1\nkind: Pod"
	right := "apiVersion: v2\nkind: Pod"

	t.Run("search query appears in title", func(t *testing.T) {
		result := RenderUnifiedDiffView(left, right, "a", "b", 0, 120, 30, false, "Pod", nil, nil, false, "", 0, DiffVisualParams{})
		assert.Contains(t, result, "[/Pod]")
	})
}

func TestRenderDiffViewWithFolding(t *testing.T) {
	lines := make([]string, 0, 20)
	for i := range 20 {
		lines = append(lines, "same-line")
		_ = i
	}
	content := strings.Join(lines, "\n")
	regions := ComputeDiffFoldRegions(content, content)
	foldState := make([]bool, len(regions))
	for i := range foldState {
		foldState[i] = true
	}

	t.Run("collapsed region shows placeholder", func(t *testing.T) {
		result := RenderDiffView(content, content, "a", "b", 0, 120, 40, false, "", regions, foldState, false, "", 0, DiffVisualParams{})
		assert.Contains(t, result, "unchanged lines")
	})

	t.Run("unified view collapsed region shows placeholder", func(t *testing.T) {
		result := RenderUnifiedDiffView(content, content, "a", "b", 0, 120, 40, false, "", regions, foldState, false, "", 0, DiffVisualParams{})
		assert.Contains(t, result, "unchanged lines")
	})
}
