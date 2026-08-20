package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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

	// 'staging' has no match. Among prefix matches, the length bonus puts the
	// shorter candidate first: prodigy (7) > production (10). The substring
	// match (my-prod-cluster) ranks below both prefix matches.
	texts := suggestionTexts(got)
	assert.Equal(t, []string{"prodigy", "production", "my-prod-cluster"}, texts)
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
	assert.Equal(t, []string{"alpha", "beta", "gamma"}, suggestionTexts(got),
		"empty query preserves input order")
}

func TestFilterSuggestionsFuzzy_EmptyQueryPreservesNonAlphabeticalOrder(t *testing.T) {
	// Simulate a kubeconfig where contexts arrive in usage order, not
	// alphabetical. With an empty query the upstream order must survive.
	cands := []string{"prod-east", "default", "prod-west", "staging"}
	got := filterSuggestionsFuzzy(cands, "", "context")
	assert.Equal(t, cands, suggestionTexts(got))
}
