package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFuzzyScore_EmptyQueryAllMatch(t *testing.T) {
	assert.Positive(t, fuzzyScore("anything", ""), "empty query matches all")
	assert.Positive(t, fuzzyScore("", ""), "empty against empty still matches")
}

func TestFuzzyScore_ExactBeatsPrefix(t *testing.T) {
	exact := fuzzyScore("pod", "pod")
	prefix := fuzzyScore("pods", "pod")
	assert.Greater(t, exact, prefix, "exact should outrank prefix")
}

func TestFuzzyScore_PrefixBeatsSubstring(t *testing.T) {
	prefix := fuzzyScore("production", "prod")
	substring := fuzzyScore("my-prod-ns", "prod")
	assert.Greater(t, prefix, substring, "prefix should outrank substring")
}

func TestFuzzyScore_SubstringBeatsSubsequence(t *testing.T) {
	substring := fuzzyScore("kube-system", "system")
	subseq := fuzzyScore("kube-system", "ksys")
	assert.Greater(t, substring, subseq, "substring should outrank subsequence")
}

func TestFuzzyScore_Subsequence(t *testing.T) {
	assert.Positive(t, fuzzyScore("kube-system", "ks"),
		"subsequence 'ks' should match 'kube-system'")
	assert.Positive(t, fuzzyScore("my-prod-cluster", "prod"),
		"substring 'prod' should match 'my-prod-cluster'")
}

func TestFuzzyScore_NoMatch(t *testing.T) {
	assert.Zero(t, fuzzyScore("default", "kube"),
		"'kube' should not match 'default'")
	assert.Zero(t, fuzzyScore("pod", "xyz"),
		"'xyz' should not match 'pod'")
}

func TestFuzzyScore_TighterSpanWins(t *testing.T) {
	tight := fuzzyScore("abcxy", "axy")
	loose := fuzzyScore("axbcxxxxxy", "axy")
	assert.Greater(t, tight, loose, "tighter subsequence span should rank higher")
}

func TestFuzzyScore_ShorterPrefixWins(t *testing.T) {
	short := fuzzyScore("pod", "po")
	long := fuzzyScore("podsecuritypolicy", "po")
	assert.Greater(t, short, long, "shorter candidate should rank higher on prefix tie")
}

func TestFuzzyScore_CaseInsensitive(t *testing.T) {
	assert.Equal(t, fuzzyScore("Pods", "pod"), fuzzyScore("pods", "POD"),
		"matching should be case-insensitive")
}

func TestSubsequenceSpan(t *testing.T) {
	span, ok := subsequenceSpan("kube-system", "ks")
	assert.True(t, ok)
	assert.Equal(t, 6, span, "'ks' spans from index 0 ('k') to 5 ('s'), length 6")

	span, ok = subsequenceSpan("abc", "ac")
	assert.True(t, ok)
	assert.Equal(t, 3, span)

	_, ok = subsequenceSpan("abc", "xyz")
	assert.False(t, ok)

	span, ok = subsequenceSpan("anything", "")
	assert.True(t, ok)
	assert.Zero(t, span)
}

func TestFilterSuggestionsFuzzy_Ranking(t *testing.T) {
	cands := []string{"my-prod-cluster", "production", "prodigy", "staging"}
	got := filterSuggestionsFuzzy(cands, "prod", "namespace")

	assert.Len(t, got, 3, "'staging' should be excluded")
	// production (prefix, shorter) should come before prodigy (prefix, shorter than production).
	// But both are prefix matches; the tiebreak on length should put prodigy (7) > production (10).
	// Then my-prod-cluster is substring.
	texts := suggestionTexts(got)
	assert.Equal(t, "my-prod-cluster", texts[len(texts)-1],
		"substring match should rank below prefix matches")
	assert.Contains(t, texts[:2], "production")
	assert.Contains(t, texts[:2], "prodigy")
}

func TestFilterSuggestionsFuzzy_SubsequenceMatch(t *testing.T) {
	cands := []string{"kube-system", "kube-public", "default", "production"}
	got := filterSuggestionsFuzzy(cands, "ksys", "namespace")

	texts := suggestionTexts(got)
	assert.Contains(t, texts, "kube-system", "'ksys' should match 'kube-system' as subsequence")
	assert.NotContains(t, texts, "default")
}

func TestFilterSuggestionsFuzzy_PreservesCategory(t *testing.T) {
	got := filterSuggestionsFuzzy([]string{"yaml", "json"}, "", "format")
	assert.Len(t, got, 2)
	for _, s := range got {
		assert.Equal(t, "format", s.Category)
	}
}

func TestFilterSuggestionsFuzzy_AlphabeticalTiebreak(t *testing.T) {
	// All three are exact-substring "foo" matches at the same position; with
	// the same length they'll tie on score and should fall back to alphabetical.
	cands := []string{"z-foo-1", "a-foo-1", "m-foo-1"}
	got := filterSuggestionsFuzzy(cands, "foo", "namespace")

	texts := suggestionTexts(got)
	assert.Equal(t, []string{"a-foo-1", "m-foo-1", "z-foo-1"}, texts)
}

func TestFilterSuggestionsFuzzy_EmptyQueryKeepsAll(t *testing.T) {
	cands := []string{"alpha", "beta", "gamma"}
	got := filterSuggestionsFuzzy(cands, "", "option")
	assert.Len(t, got, 3)
}
