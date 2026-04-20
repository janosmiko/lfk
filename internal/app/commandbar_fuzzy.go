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
//   - Prefix match:      5000 + (100 - len(text))       // shorter prefers
//   - Substring match:   2000 + (100 - idx) - len(text) // earlier & shorter
//   - Subsequence match:  500 + (100 - span)            // tighter prefers
//
// An empty query returns a small baseline score so every candidate is kept
// while preserving alphabetical order.
func fuzzyScore(text, query string) int {
	if query == "" {
		return 1
	}

	t := strings.ToLower(text)
	q := strings.ToLower(query)

	if t == q {
		return 10000
	}

	if strings.HasPrefix(t, q) {
		return 5000 + lengthBonus(len(t))
	}

	if idx := strings.Index(t, q); idx >= 0 {
		return 2000 + positionBonus(idx) + lengthBonus(len(t))
	}

	if span, ok := subsequenceSpan(t, q); ok {
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
// (not necessarily contiguous) and returns the span from the first matched
// rune to the last. Both inputs must already be lowercased.
func subsequenceSpan(text, query string) (int, bool) {
	if query == "" {
		return 0, true
	}

	qi := 0
	first := -1
	last := -1

	qr := []rune(query)
	for i, r := range text {
		if qi >= len(qr) {
			break
		}

		if r == qr[qi] {
			if first == -1 {
				first = i
			}

			last = i
			qi++
		}
	}

	if qi < len(qr) {
		return 0, false
	}

	return last - first + 1, true
}

// filterSuggestionsFuzzy filters candidates by a fuzzy match against query
// and sorts the results by score (descending) with alphabetical tiebreak.
// Use this at value positions (namespace, context, resource name, option,
// column, format). First-token command names stay on prefix via
// filterSuggestionsTyped — a small curated list doesn't benefit from fuzzy
// matching and would produce noisy results for short inputs.
func filterSuggestionsFuzzy(candidates []string, query, category string) []ui.Suggestion {
	type scored struct {
		s     ui.Suggestion
		score int
	}

	rs := make([]scored, 0, len(candidates))

	for _, c := range candidates {
		sc := fuzzyScore(c, query)
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
