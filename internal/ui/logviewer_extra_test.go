package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- highlightSearchMatches ---

func TestHighlightSearchMatches(t *testing.T) {
	tests := []struct {
		name       string
		lines      []string
		query      string
		wantSubstr [][]string // per-line expected substrings
		wantMatch  []bool     // whether each line should be modified
	}{
		{
			name:      "no match leaves lines unchanged",
			lines:     []string{"hello world", "foo bar"},
			query:     "xyz",
			wantMatch: []bool{false, false},
		},
		{
			name:       "case-insensitive match highlights query",
			lines:      []string{"Hello World", "nothing here"},
			query:      "hello",
			wantSubstr: [][]string{{"Hello"}, nil},
			wantMatch:  []bool{true, false},
		},
		{
			name:       "multiple matches in one line",
			lines:      []string{"foo bar foo baz"},
			query:      "foo",
			wantSubstr: [][]string{{"foo"}},
			wantMatch:  []bool{true},
		},
		{
			name:       "match in all lines",
			lines:      []string{"error here", "error there"},
			query:      "error",
			wantSubstr: [][]string{{"error"}, {"error"}},
			wantMatch:  []bool{true, true},
		},
		{
			name:      "empty lines are unchanged",
			lines:     []string{"", "test"},
			query:     "test",
			wantMatch: []bool{false, true},
		},
		{
			name:       "preserves text around match",
			lines:      []string{"prefix ERROR suffix"},
			query:      "error",
			wantSubstr: [][]string{{"prefix", "ERROR", "suffix"}},
			wantMatch:  []bool{true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := highlightSearchMatches(tt.lines, tt.query)
			assert.Equal(t, len(tt.lines), len(result), "result length should match input length")
			for i, matched := range tt.wantMatch {
				if !matched {
					assert.Equal(t, tt.lines[i], result[i], "unmatched line %d should be unchanged", i)
				} else {
					// The result should contain the original text plus highlighting.
					if tt.wantSubstr != nil && tt.wantSubstr[i] != nil {
						for _, sub := range tt.wantSubstr[i] {
							assert.Contains(t, result[i], sub, "line %d should contain %q", i, sub)
						}
					}
				}
			}
		})
	}
}

// --- renderPlainLines ---

func TestRenderPlainLines(t *testing.T) {
	tests := []struct {
		name         string
		lines        []string
		scroll       int
		height       int
		width        int
		lineNumbers  bool
		lineNumWidth int
		cursor       int
		wantCount    int
		wantSubstr   []string
	}{
		{
			name:        "basic rendering",
			lines:       []string{"line 1", "line 2", "line 3"},
			scroll:      0,
			height:      3,
			width:       80,
			lineNumbers: false,
			cursor:      -1,
			wantCount:   3,
			wantSubstr:  []string{"line 1", "line 2", "line 3"},
		},
		{
			name:        "scroll skips initial lines",
			lines:       []string{"line 1", "line 2", "line 3", "line 4"},
			scroll:      2,
			height:      2,
			width:       80,
			lineNumbers: false,
			cursor:      -1,
			wantCount:   2,
			wantSubstr:  []string{"line 3", "line 4"},
		},
		{
			name:        "height limits output",
			lines:       []string{"a", "b", "c", "d", "e"},
			scroll:      0,
			height:      3,
			width:       80,
			lineNumbers: false,
			cursor:      -1,
			wantCount:   3,
		},
		{
			name:         "line numbers shown",
			lines:        []string{"line 1", "line 2"},
			scroll:       0,
			height:       2,
			width:        80,
			lineNumbers:  true,
			lineNumWidth: 3,
			cursor:       -1,
			wantCount:    2,
			wantSubstr:   []string{"1", "2"},
		},
		{
			name:        "cursor line gets indicator",
			lines:       []string{"line 1", "line 2", "line 3"},
			scroll:      0,
			height:      3,
			width:       80,
			lineNumbers: false,
			cursor:      1,
			wantCount:   3,
		},
		{
			name:        "empty lines list",
			lines:       []string{},
			scroll:      0,
			height:      5,
			width:       80,
			lineNumbers: false,
			cursor:      -1,
			wantCount:   0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderPlainLines(tt.lines, tt.scroll, tt.height, tt.width,
				tt.lineNumbers, tt.lineNumWidth, tt.cursor, -1, -1, -1, 0, 0, 0)
			assert.Equal(t, tt.wantCount, len(result), "rendered line count")
			for _, sub := range tt.wantSubstr {
				found := false
				for _, line := range result {
					if strings.Contains(line, sub) {
						found = true
						break
					}
				}
				assert.True(t, found, "rendered output should contain %q", sub)
			}
		})
	}

	t.Run("cursor line has cursor indicator glyph", func(t *testing.T) {
		lines := []string{"hello", "world"}
		result := renderPlainLines(lines, 0, 2, 80, false, 0, 0, -1, -1, -1, 0, 0, 0)
		// Cursor line should contain the bar cursor indicator.
		assert.Contains(t, result[0], "\u258e", "cursor line should have indicator glyph")
		// Non-cursor line should start with a space.
		assert.True(t, strings.HasPrefix(result[1], " "), "non-cursor line should start with space")
	})

	t.Run("scroll beyond end produces empty result", func(t *testing.T) {
		lines := []string{"a", "b"}
		result := renderPlainLines(lines, 5, 3, 80, false, 0, -1, -1, -1, -1, 0, 0, 0)
		assert.Empty(t, result)
	})

	// Regression: visual selection over a kyverno-style log row used to
	// rune-truncate the line before handing it to RenderVisualSelection.
	// Each embedded "\x1b[NNm" costs 4-5 rune slots while contributing zero
	// visual width, so the rune cap chopped lines off way before the
	// configured width budget — visible content was replaced by trailing
	// spaces from FillLinesBg's pad-to-width pass and "0m" / "[NNm"
	// fragments leaked from the cut-mid-CSI tail.
	t.Run("selection over ANSI-heavy line keeps full visible width", func(t *testing.T) {
		// Real kyverno log row: every field is wrapped in its own SGR pair,
		// so the rune count balloons past the visual width budget. With
		// rune-based truncation at width=120 the line was cut around
		// character 80 of visible content (the first ~40 rune slots get
		// eaten by ANSI bytes), and FillLinesBg padded the rest with
		// spaces. The user reported seeing the row chop off mid-word.
		line := "\x1b[90m2026-04-27T16:07:08Z \x1b[0m \x1b[34mTRC \x1b[0m " +
			"\x1b[1mgithub.com/kyverno/kyverno/pkg/controllers/admissionpolicygenerator/cpol.go:12 " +
			"\x1b[0m \x1b[36m > \x1b[0m policy created \x1b[36mkind=\x1b[0mClusterPolicy"

		// Width tight enough that we'd lose the ClusterPolicy tail to a
		// rune-based truncate (rune count exceeds the budget) but loose
		// enough that the visible content still fits within the visual
		// budget.
		const width = 170
		require.Greater(t, len([]rune(line)), width,
			"test premise: rune count must exceed width so the OLD rune-truncate would have chopped the line")
		require.LessOrEqual(t, ansi.StringWidth(line), width-1,
			"test premise: visible width must fit so the NEW visual-truncate keeps every char")

		result := renderPlainLines(
			[]string{line},
			0, 1, width,
			false, 0,
			0,    // cursor
			0, 0, // selStart, selEnd (single-line selection)
			0,    // visualStart
			'V',  // visualType: line mode
			0, 0, // visualCol, visualCurCol
		)
		require.Len(t, result, 1)

		stripped := strings.TrimSpace(ansi.Strip(result[0]))
		// The full visible payload — including the trailing ClusterPolicy
		// — must reach the output even though the line carries lots of
		// embedded ANSI.
		assert.Contains(t, stripped, "ClusterPolicy",
			"trailing visible content must survive the visual-width-aware pre-trim (got %q)", stripped)
	})
}
