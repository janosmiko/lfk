package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/ui"
)

// --- ensureYAMLCursorVisible ---

func TestEnsureYAMLCursorVisible(t *testing.T) {
	so := ui.ConfigScrollOff

	t.Run("cursor above viewport scrolls up with scrolloff", func(t *testing.T) {
		m := Model{
			height:     30,
			yamlScroll: 20,
			yamlCursor: 5,
		}
		m.ensureYAMLCursorVisible()
		assert.Equal(t, max(5-so, 0), m.yamlScroll)
	})

	t.Run("cursor below viewport scrolls down with scrolloff", func(t *testing.T) {
		m := Model{
			height:     30,
			yamlScroll: 0,
			yamlCursor: 50,
		}
		m.ensureYAMLCursorVisible()
		maxLines := m.yamlViewportLines()
		assert.Equal(t, 50-maxLines+so+1, m.yamlScroll)
	})

	t.Run("cursor within viewport no scroll change", func(t *testing.T) {
		m := Model{
			height:     30,
			yamlScroll: 0,
			yamlCursor: 10,
		}
		m.ensureYAMLCursorVisible()
		assert.Equal(t, 0, m.yamlScroll)
	})

	t.Run("small height clamps scrolloff", func(t *testing.T) {
		m := Model{
			height:     5,
			yamlScroll: 0,
			yamlCursor: 10,
		}
		m.ensureYAMLCursorVisible()
		maxLines := m.yamlViewportLines()
		clampedSo := min(so, maxLines/2)
		assert.Equal(t, 10-maxLines+clampedSo+1, m.yamlScroll)
	})
}

// --- clampYAMLScroll ---

func TestClampYAMLScroll(t *testing.T) {
	t.Run("clamps scroll past content", func(t *testing.T) {
		m := Model{
			height:        30,
			yamlContent:   "line1\nline2\nline3",
			yamlScroll:    100,
			yamlCollapsed: make(map[string]bool),
		}
		m.clampYAMLScroll()
		assert.GreaterOrEqual(t, m.yamlScroll, 0)
		assert.LessOrEqual(t, m.yamlScroll, 3)
	})

	t.Run("negative scroll clamped to zero", func(t *testing.T) {
		m := Model{
			height:        30,
			yamlContent:   "line1\nline2",
			yamlScroll:    -5,
			yamlCollapsed: make(map[string]bool),
		}
		m.clampYAMLScroll()
		assert.Equal(t, 0, m.yamlScroll)
	})

	t.Run("content shorter than viewport keeps scroll at zero", func(t *testing.T) {
		m := Model{
			height:        100,
			yamlContent:   "line1\nline2",
			yamlScroll:    5,
			yamlCollapsed: make(map[string]bool),
		}
		m.clampYAMLScroll()
		assert.Equal(t, 0, m.yamlScroll)
	})
}

// --- updateYAMLSearchMatches ---

func TestUpdateYAMLSearchMatches(t *testing.T) {
	t.Run("finds matching lines", func(t *testing.T) {
		m := Model{
			yamlContent:    "apiVersion: v1\nkind: Pod\nmetadata:\n  name: nginx-pod\nspec:\n  containers:\n  - name: nginx",
			yamlSearchText: TextInput{Value: "nginx"},
		}
		m.updateYAMLSearchMatches()
		assert.Len(t, m.yamlMatchLines, 2) // line 3 (name: nginx-pod) and line 6 (name: nginx)
	})

	t.Run("case insensitive matching", func(t *testing.T) {
		m := Model{
			yamlContent:    "Kind: Pod\nkind: Service",
			yamlSearchText: TextInput{Value: "KIND"},
		}
		m.updateYAMLSearchMatches()
		assert.Len(t, m.yamlMatchLines, 2)
	})

	t.Run("empty search clears matches", func(t *testing.T) {
		m := Model{
			yamlContent:    "apiVersion: v1",
			yamlSearchText: TextInput{},
			yamlMatchLines: []int{0}, // pre-existing matches
		}
		m.updateYAMLSearchMatches()
		assert.Nil(t, m.yamlMatchLines)
	})

	t.Run("no matches returns empty", func(t *testing.T) {
		m := Model{
			yamlContent:    "apiVersion: v1\nkind: Pod",
			yamlSearchText: TextInput{Value: "nonexistent"},
		}
		m.updateYAMLSearchMatches()
		assert.Empty(t, m.yamlMatchLines)
	})
}

// --- findYAMLMatchFromCursor ---

func TestFindYAMLMatchFromCursor(t *testing.T) {
	t.Run("finds match at cursor", func(t *testing.T) {
		m := Model{
			yamlContent:    "line0\nline1\nline2\nline3",
			yamlMatchLines: []int{1, 3},
			yamlCursor:     1,
			yamlCollapsed:  make(map[string]bool),
		}
		idx := m.findYAMLMatchFromCursor()
		assert.Equal(t, 0, idx) // match at line 1 is first match at/after cursor
	})

	t.Run("finds next match after cursor", func(t *testing.T) {
		m := Model{
			yamlContent:    "line0\nline1\nline2\nline3",
			yamlMatchLines: []int{1, 3},
			yamlCursor:     2,
			yamlCollapsed:  make(map[string]bool),
		}
		idx := m.findYAMLMatchFromCursor()
		assert.Equal(t, 1, idx) // match at line 3
	})

	t.Run("wraps to first match when past all matches", func(t *testing.T) {
		m := Model{
			yamlContent:    "line0\nline1\nline2\nline3\nline4",
			yamlMatchLines: []int{1},
			yamlCursor:     4,
			yamlCollapsed:  make(map[string]bool),
		}
		idx := m.findYAMLMatchFromCursor()
		assert.Equal(t, 0, idx) // wraps to first match
	})
}

// --- yamlScrollToMatchFolded ---

func TestYAMLScrollToMatchFolded(t *testing.T) {
	t.Run("scrolls to match position", func(t *testing.T) {
		m := Model{
			height:         30,
			yamlContent:    "line0\nline1\nline2\nline3\nline4",
			yamlMatchLines: []int{3},
			yamlMatchIdx:   0,
			yamlCollapsed:  make(map[string]bool),
		}
		viewportLines := m.height - 4
		m.yamlScrollToMatchFolded(viewportLines)
		assert.Equal(t, 3, m.yamlCursor) // cursor at visible line for original line 3
	})

	t.Run("no-op when matchIdx out of range", func(t *testing.T) {
		m := Model{
			height:         30,
			yamlContent:    "line0\nline1",
			yamlMatchLines: []int{1},
			yamlMatchIdx:   5, // out of range
			yamlCursor:     0,
			yamlCollapsed:  make(map[string]bool),
		}
		m.yamlScrollToMatchFolded(26)
		assert.Equal(t, 0, m.yamlCursor) // unchanged
	})

	t.Run("no-op when negative matchIdx", func(t *testing.T) {
		m := Model{
			height:         30,
			yamlContent:    "line0",
			yamlMatchLines: []int{0},
			yamlMatchIdx:   -1,
			yamlCursor:     0,
			yamlCollapsed:  make(map[string]bool),
		}
		m.yamlScrollToMatchFolded(26)
		assert.Equal(t, 0, m.yamlCursor) // unchanged
	})
}

// --- yamlNextIntraLineMatch ---

func TestYamlNextIntraLineMatch(t *testing.T) {
	t.Run("forward does not panic when cursor col exceeds line length", func(t *testing.T) {
		// Regression: yamlVisualCurCol carries over from a previously
		// focused long line. When `n` triggers an intra-line search and
		// the current line is shorter than yamlVisualCurCol+1, the
		// rune-slice indexing must not panic.
		m := Model{
			yamlContent:      "short\nfoo: target here",
			yamlCursor:       0,
			yamlVisualCurCol: 900, // far beyond "short"
			yamlSearchText:   TextInput{Value: "target"},
			yamlCollapsed:    map[string]bool{},
		}
		var found bool
		assert.NotPanics(t, func() { found = m.yamlNextIntraLineMatch(true) })
		assert.False(t, found, "no match expected on the short current line")
	})

	t.Run("backward does not panic when cursor col exceeds line length", func(t *testing.T) {
		// Same regression for the backward path (N / shift-n).
		m := Model{
			yamlContent:      "foo: target here\nshort",
			yamlCursor:       1,
			yamlVisualCurCol: 900, // far beyond "short"
			yamlSearchText:   TextInput{Value: "target"},
			yamlCollapsed:    map[string]bool{},
		}
		var found bool
		assert.NotPanics(t, func() { found = m.yamlNextIntraLineMatch(false) })
		assert.False(t, found, "no match expected on the short current line")
	})
}
