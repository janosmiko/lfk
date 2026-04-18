package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPrettyPrintJSONObject covers the single-key object case: the
// rendered output must be multi-line and use exactly 2-space indent.
func TestPrettyPrintJSONObject(t *testing.T) {
	j := DetectJSONLine(`{"foo":1}`)
	require.True(t, j.IsJSON)

	out := PrettyPrintJSON(j.Value, 0)
	assert.Contains(t, out, "{\n")
	assert.Contains(t, out, "\n}")
	assert.Contains(t, out, "\n  \"foo\": 1")
	// No trailing newline — the caller splits on "\n" and a trailing
	// newline would produce an empty tail line.
	assert.False(t, strings.HasSuffix(out, "\n"), "trailing newline should be trimmed")
}

// TestPrettyPrintJSONNested covers 2+ levels of nesting. Each level
// should increase the indent by 2 spaces.
func TestPrettyPrintJSONNested(t *testing.T) {
	j := DetectJSONLine(`{"a":{"b":{"c":1}}}`)
	require.True(t, j.IsJSON)

	out := PrettyPrintJSON(j.Value, 0)
	// Level 1: 2-space indent for "a".
	assert.Contains(t, out, "\n  \"a\":")
	// Level 2: 4-space indent for "b".
	assert.Contains(t, out, "\n    \"b\":")
	// Level 3: 6-space indent for "c".
	assert.Contains(t, out, "\n      \"c\": 1")
}

// TestPrettyPrintJSONPreservesNumber verifies that large integers (that
// would lose precision if decoded as float64) survive the pretty-print
// round trip when the detector uses json.Number. This is the key reason
// the detector sets UseNumber() — filters and pretty-printing both
// expect exact numeric forms.
func TestPrettyPrintJSONPreservesNumber(t *testing.T) {
	const big = "9007199254740999" // max safe int + 7 (loses precision as float64)
	j := DetectJSONLine(`{"n":` + big + `}`)
	require.True(t, j.IsJSON)

	out := PrettyPrintJSON(j.Value, 0)
	assert.Contains(t, out, big, "large integer should round-trip verbatim")
	assert.NotContains(t, out, "9.00719", "no scientific notation allowed")
}

// TestPrettyPrintJSONArray covers the array variant — same rules as
// objects but the outer brackets are [ / ].
func TestPrettyPrintJSONArray(t *testing.T) {
	j := DetectJSONLine(`[1,2,[3,4]]`)
	require.True(t, j.IsJSON)

	out := PrettyPrintJSON(j.Value, 0)
	assert.True(t, strings.HasPrefix(out, "["))
	assert.True(t, strings.HasSuffix(out, "]"))
	// Top-level elements: 2-space indent.
	assert.Contains(t, out, "\n  1")
	assert.Contains(t, out, "\n  2")
	// Nested array: starts at 2-space indent, inner elements at 4.
	assert.Contains(t, out, "\n  [")
	assert.Contains(t, out, "\n    3")
	assert.Contains(t, out, "\n    4")
}

// TestPrettyPrintJSONNilReturnsEmpty guards against NPE when the caller
// passes a zero JSONLine (IsJSON=false path through BuildPrettyVisibleLines).
func TestPrettyPrintJSONNilReturnsEmpty(t *testing.T) {
	out := PrettyPrintJSON(nil, 0)
	assert.Empty(t, out)
}

// TestPrettyPrintJSONDoesNotEscapeHTML verifies HTML escaping is off —
// log JSON often contains URLs with `&`, `<`, `>` and the default
// json.Encoder escape pass would garble those for no gain inside a TUI.
func TestPrettyPrintJSONDoesNotEscapeHTML(t *testing.T) {
	j := DetectJSONLine(`{"url":"https://example.com?a=1&b=2"}`)
	require.True(t, j.IsJSON)

	out := PrettyPrintJSON(j.Value, 0)
	assert.Contains(t, out, "a=1&b=2")
	assert.NotContains(t, out, `\u0026`)
}

// TestBuildPrettyVisibleLinesNoVisibleIndices covers the non-filtered
// path: one entry per source line, JSON lines expanded, plain lines
// left verbatim.
func TestBuildPrettyVisibleLinesNoVisibleIndices(t *testing.T) {
	lines := []string{
		`{"a":1}`,
		`plain text line`,
		`{"b":2}`,
	}
	lookup := func(idx int) JSONLine { return DetectJSONLine(lines[idx]) }

	out := BuildPrettyVisibleLines(lines, nil, lookup)
	require.Len(t, out, 3)

	// JSON entries expanded (have newlines).
	assert.Contains(t, out[0], "\n")
	assert.Contains(t, out[0], `"a": 1`)
	assert.Contains(t, out[2], `"b": 2`)

	// Plain entry left verbatim, no newline injected.
	assert.Equal(t, lines[1], out[1])
}

// TestBuildPrettyVisibleLinesWithVisibleIndices covers the filter path:
// only the visible subset is returned.
func TestBuildPrettyVisibleLinesWithVisibleIndices(t *testing.T) {
	lines := []string{
		`{"a":1}`,
		`plain text`,
		`{"b":2}`,
	}
	visible := []int{0, 2} // exclude the plain line
	lookup := func(idx int) JSONLine { return DetectJSONLine(lines[idx]) }

	out := BuildPrettyVisibleLines(lines, visible, lookup)
	require.Len(t, out, 2)
	assert.Contains(t, out[0], `"a": 1`)
	assert.Contains(t, out[1], `"b": 2`)
}

// TestBuildPrettyVisibleLinesPreservesPodPrefix: a line with a
// pod/container prefix should have the prefix reattached to the
// first visual sub-line of the pretty output.
func TestBuildPrettyVisibleLinesPreservesPodPrefix(t *testing.T) {
	line := `[api/web] {"level":"info"}`
	lookup := func(idx int) JSONLine { return DetectJSONLine(line) }

	out := BuildPrettyVisibleLines([]string{line}, nil, lookup)
	require.Len(t, out, 1)
	assert.True(t, strings.HasPrefix(out[0], "[api/web] {"),
		"pretty output must keep the pod prefix before the JSON opening brace")
	assert.Contains(t, out[0], `"level": "info"`)
}
