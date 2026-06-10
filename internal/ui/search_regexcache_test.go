package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The search/filter hot paths (MatchLine, FindColumnInLine, highlightRegex)
// call into regex compilation per item per frame. Recompiling the same pattern
// thousands of times per keypress is a visible freeze on large clusters / log
// buffers, so the same query must reuse a cached compiled regex.
func TestCompileSearchRegexMemoizesSameQuery(t *testing.T) {
	re1, err := compileSearchRegex("foo.*")
	require.NoError(t, err)
	require.NotNil(t, re1)

	re2, err := compileSearchRegex("foo.*")
	require.NoError(t, err)
	assert.Same(t, re1, re2, "same query must return the cached *regexp.Regexp")
}

func TestCompileSearchRegexRecompilesOnQueryChange(t *testing.T) {
	re1, err := compileSearchRegex("foo.*")
	require.NoError(t, err)
	re2, err := compileSearchRegex("bar.*")
	require.NoError(t, err)
	assert.NotSame(t, re1, re2, "different query must recompile")
}

func TestCompileSearchRegexIsCaseInsensitive(t *testing.T) {
	re, err := compileSearchRegex("Foo")
	require.NoError(t, err)
	assert.True(t, re.MatchString("xxfooxx"), "compiled regex must carry the (?i) flag")
}

func TestCompileSearchRegexInvalidReturnsError(t *testing.T) {
	_, err := compileSearchRegex("[unterminated")
	assert.Error(t, err)
}

// Regression guard: MatchLine in regex mode must not pay a fresh compile per
// call. Run: go test -bench=BenchmarkMatchLineRegex -benchmem ./internal/ui/
func BenchmarkMatchLineRegex(b *testing.B) {
	const line = "default   nginx-deployment-7c5ddbdf54-abcde   1/1   Running   0   5m"
	const query = "nginx.*Running"
	for b.Loop() {
		_ = MatchLine(line, query)
	}
}
