package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- RenderUnifiedDiffView ---

func TestRenderUnifiedDiffView(t *testing.T) {
	tests := []struct {
		name       string
		left       string
		right      string
		leftName   string
		rightName  string
		scroll     int
		width      int
		height     int
		lineNums   bool
		wantSubstr []string
	}{
		{
			name:       "identical content shows unified header",
			left:       "name: test\nreplicas: 1",
			right:      "name: test\nreplicas: 1",
			leftName:   "before",
			rightName:  "after",
			scroll:     0,
			width:      80,
			height:     30,
			lineNums:   false,
			wantSubstr: []string{"Resource Diff (unified)", "--- before", "+++ after", "name: test", "replicas: 1"},
		},
		{
			name:       "added line shows plus prefix",
			left:       "name: test",
			right:      "name: test\nreplicas: 3",
			leftName:   "old",
			rightName:  "new",
			scroll:     0,
			width:      80,
			height:     30,
			lineNums:   false,
			wantSubstr: []string{"+replicas: 3"},
		},
		{
			name:       "removed line shows minus prefix",
			left:       "name: test\nreplicas: 3",
			right:      "name: test",
			leftName:   "old",
			rightName:  "new",
			scroll:     0,
			width:      80,
			height:     30,
			lineNums:   false,
			wantSubstr: []string{"-replicas: 3"},
		},
		{
			name:       "line numbers enabled",
			left:       "line1\nline2",
			right:      "line1\nline2\nline3",
			leftName:   "a",
			rightName:  "b",
			scroll:     0,
			width:      80,
			height:     30,
			lineNums:   true,
			wantSubstr: []string{"1", "2"},
		},
		{
			name:       "scroll info shown",
			left:       "a",
			right:      "b",
			leftName:   "x",
			rightName:  "y",
			scroll:     0,
			width:      140,
			height:     30,
			lineNums:   false,
			wantSubstr: []string{"[1/"},
		},
		{
			name:       "hint bar shows key bindings",
			left:       "a",
			right:      "b",
			leftName:   "x",
			rightName:  "y",
			scroll:     0,
			width:      140,
			height:     30,
			lineNums:   false,
			wantSubstr: []string{"j/k", "scroll", "q/esc", "back", "side-by-side", "y"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderUnifiedDiffView(tt.left, tt.right, tt.leftName, tt.rightName, tt.scroll, tt.width, tt.height, tt.lineNums, false, "", nil, nil, false, "", 0, DiffVisualParams{}, "")
			for _, sub := range tt.wantSubstr {
				assert.Contains(t, result, sub, "result should contain %q", sub)
			}
		})
	}
}

// --- footerOverride ---

// When the caller passes a non-empty footerOverride, the renderer must use
// it verbatim in place of the default key-binding hint bar. This lets the
// caller paint copy feedback / status messages in the footer slot without
// resorting to splitting the rendered output and overwriting its last
// line — that trick was fragile against multi-line hints and trailing
// newlines.
func TestRenderDiffViewHonorsFooterOverride(t *testing.T) {
	override := "[STATUS] Copied 1 line"
	result := RenderDiffView(
		"a: 1", "a: 2", "before", "after",
		0, 140, 30, false, false, "", nil, nil, false, "", 0,
		DiffVisualParams{}, override,
	)
	assert.Contains(t, result, override, "override must appear in the rendered output")
	assert.NotContains(t, result, "side", "default 'side' hint must not also be drawn")
	assert.NotContains(t, result, "q/esc", "default 'q/esc' hint must not also be drawn")
}

func TestRenderUnifiedDiffViewHonorsFooterOverride(t *testing.T) {
	override := "[STATUS] Copied 1 line"
	result := RenderUnifiedDiffView(
		"a: 1", "a: 2", "before", "after",
		0, 140, 30, false, false, "", nil, nil, false, "", 0,
		DiffVisualParams{}, override,
	)
	assert.Contains(t, result, override, "override must appear in the rendered output")
	assert.NotContains(t, result, "side-by-side", "default 'side-by-side' hint must not also be drawn")
	assert.NotContains(t, result, "q/esc", "default 'q/esc' hint must not also be drawn")
}

// Empty override means "use the default hint bar" — important for the
// majority of callers that don't paint a status message.
func TestRenderDiffViewEmptyOverrideUsesDefault(t *testing.T) {
	result := RenderDiffView(
		"a: 1", "a: 2", "before", "after",
		0, 140, 30, false, false, "", nil, nil, false, "", 0,
		DiffVisualParams{}, "",
	)
	assert.Contains(t, result, "side", "default hint must be drawn when override is empty")
}

// --- UnifiedDiffViewTotalLines ---

func TestUnifiedDiffViewTotalLines(t *testing.T) {
	tests := []struct {
		name     string
		left     string
		right    string
		expected int
	}{
		{
			name:     "identical single lines",
			left:     "line1",
			right:    "line1",
			expected: 1, // 1 diff line (headers not counted)
		},
		{
			name:     "one addition",
			left:     "line1",
			right:    "line1\nline2",
			expected: 2, // 2 diff lines (headers not counted)
		},
		{
			name:     "one removal",
			left:     "line1\nline2",
			right:    "line1",
			expected: 2, // 2 diff lines (headers not counted)
		},
		{
			name:     "completely different",
			left:     "aaa",
			right:    "bbb",
			expected: 2, // 1 removed + 1 added (headers not counted)
		},
		{
			name:     "empty inputs",
			left:     "",
			right:    "",
			expected: 0, // headers are always visible, not counted
		},
		{
			name:     "multi-line diff",
			left:     "a\nb\nc",
			right:    "a\nx\nc",
			expected: 4, // a(=) + b(<) + x(>) + c(=); headers not counted
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, UnifiedDiffViewTotalLines(tt.left, tt.right, nil, nil))
		})
	}
}
