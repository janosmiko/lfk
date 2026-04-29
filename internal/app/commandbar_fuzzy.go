package app

import (
	"sort"
	"strings"

	"github.com/janosmiko/lfk/internal/ui"
)

// fuzzyScore returns a match score for query against text. Higher is better;
// 0 means no match. The scoring tiers are ordered so that a prefix match of
// any candidate always outranks a substring match, which always outranks a
// subsequence match, regardless of length bonuses within each tier.
//
// Tiers (case-insensitive):
//   - Exact match:      10000
//   - Prefix match:      5000 + (100 - len(text))                 // shorter prefers
//   - Substring match:   2000 + (100 - idx) + (100 - len(text))   // earlier & shorter
//   - Subsequence match:  500 + (100 - span)                      // tighter prefers
//
// An empty query returns a small baseline score so direct callers that score
// individual candidates still match. filterSuggestionsFuzzy short-circuits
// empty queries to preserve the candidate input order.
func fuzzyScore(text, query string) int {
	return fuzzyScoreLower(text, strings.ToLower(query))
}

// fuzzyScoreLower is fuzzyScore with the query already lowercased. Hot-loop
// callers should lowercase the query once and reuse this helper to avoid
// repeated allocations per candidate.
func fuzzyScoreLower(text, qLower string) int {
	if qLower == "" {
		return 1
	}

	t := strings.ToLower(text)

	if t == qLower {
		return 10000
	}

	if strings.HasPrefix(t, qLower) {
		return 5000 + lengthBonus(len(t))
	}

	if idx := strings.Index(t, qLower); idx >= 0 {
		return 2000 + positionBonus(idx) + lengthBonus(len(t))
	}

	if span, ok := subsequenceSpan(t, qLower); ok {
		return 500 + positionBonus(span)
	}

	return 0
}

// lengthBonus prefers shorter candidates. Capped so it never crosses tiers.
func lengthBonus(n int) int {
	if n >= 100 {
		return 0
	}

	return 100 - n
}

// positionBonus prefers smaller values (earlier position, tighter span).
func positionBonus(n int) int {
	if n >= 100 {
		return 0
	}

	return 100 - n
}

// subsequenceSpan reports whether all query runes appear in text in order
// (not necessarily contiguous) and returns the rune span from the first
// matched rune to the last (inclusive). Both inputs must already be
// lowercased.
func subsequenceSpan(text, query string) (int, bool) {
	if query == "" {
		return 0, true
	}

	qr := []rune(query)
	qi := 0
	first := -1
	last := -1
	ti := 0

	for _, r := range text {
		if qi >= len(qr) {
			break
		}

		if r == qr[qi] {
			if first == -1 {
				first = ti
			}

			last = ti
			qi++
		}

		ti++
	}

	if qi < len(qr) {
		return 0, false
	}

	return last - first + 1, true
}

// filterSuggestionsFuzzy filters candidates by a fuzzy match against query
// and sorts the results by score (descending) with alphabetical tiebreak.
// Empty queries short-circuit to preserve the candidate input order so
// upstream ordering (e.g. kubeconfig context order, default-namespace first)
// survives until the user starts typing.
//
// Use this at value positions (namespace, context, resource name, option,
// column, format). First-token command names stay on prefix via
// filterSuggestionsTyped — a small curated list doesn't benefit from fuzzy
// matching and would produce noisy results for short inputs.
func filterSuggestionsFuzzy(candidates []string, query, category string) []ui.Suggestion {
	if query == "" {
		result := make([]ui.Suggestion, len(candidates))
		for i, c := range candidates {
			result[i] = ui.Suggestion{Text: c, Category: category}
		}

		return result
	}

	type scored struct {
		s     ui.Suggestion
		score int
	}

	qLower := strings.ToLower(query)
	rs := make([]scored, 0, len(candidates))

	for _, c := range candidates {
		sc := fuzzyScoreLower(c, qLower)
		if sc <= 0 {
			continue
		}

		rs = append(rs, scored{ui.Suggestion{Text: c, Category: category}, sc})
	}

	sort.SliceStable(rs, func(i, j int) bool {
		if rs[i].score != rs[j].score {
			return rs[i].score > rs[j].score
		}

		return rs[i].s.Text < rs[j].s.Text
	})

	result := make([]ui.Suggestion, len(rs))
	for i, r := range rs {
		result[i] = r.s
	}

	return result
}
