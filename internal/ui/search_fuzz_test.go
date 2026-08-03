package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// FuzzDetectSearchMode verifies the search-mode prefix protocol holds for
// arbitrary user input. The function must never panic and must respect the
// "~" / "\" prefix contract: when those prefixes are present, the returned
// query is exactly the rest of the input.
func FuzzDetectSearchMode(f *testing.F) {
	for _, s := range []string{"", "foo", "~foo", `\foo`, "foo.*bar", "a|b", "[abc]", "~", `\`, "."} {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		mode, query := DetectSearchMode(raw)

		if mode < SearchSubstring || mode > SearchFuzzy {
			t.Fatalf("DetectSearchMode(%q) returned out-of-range mode %d", raw, mode)
		}

		switch {
		case raw == "":
			if mode != SearchSubstring || query != "" {
				t.Fatalf("DetectSearchMode(\"\") = (%d, %q); want (Substring, \"\")", mode, query)
			}
		case strings.HasPrefix(raw, "~"):
			if mode != SearchFuzzy {
				t.Fatalf("DetectSearchMode(%q): tilde prefix must yield SearchFuzzy, got %d", raw, mode)
			}
			if query != raw[1:] {
				t.Fatalf("DetectSearchMode(%q): query %q; want %q", raw, query, raw[1:])
			}
		case strings.HasPrefix(raw, `\`):
			if mode != SearchSubstring {
				t.Fatalf("DetectSearchMode(%q): backslash prefix must yield SearchSubstring, got %d", raw, mode)
			}
			if query != raw[1:] {
				t.Fatalf("DetectSearchMode(%q): query %q; want %q", raw, query, raw[1:])
			}
		default:
			if query != raw {
				t.Fatalf("DetectSearchMode(%q): un-prefixed input must return the raw query, got %q", raw, query)
			}
		}
	})
}

// FuzzMatchLine drives the substring / regex / fuzzy code paths with
// arbitrary line / query pairs. Invalid regex must fall back gracefully
// (the documented behaviour) rather than panic.
func FuzzMatchLine(f *testing.F) {
	for _, s := range [...][2]string{
		{"hello world", "hello"},
		{"hello world", "~hlw"},
		{"hello world", `\.`},
		{"hello world", "(invalid"},
		{"hello world", ""},
		{"", "anything"},
		{"foo.bar.baz", "f.*z"},
	} {
		f.Add(s[0], s[1])
	}

	f.Fuzz(func(t *testing.T, line, query string) {
		got := MatchLine(line, query)

		_, effective := DetectSearchMode(query)
		if effective == "" && got {
			t.Fatalf("MatchLine(%q, %q): empty effective query must return false", line, query)
		}
	})
}

// FuzzFuzzyScore checks the rune-walking scorer for panics and basic
// identity properties: empty query scores 0; a line always matches itself.
func FuzzFuzzyScore(f *testing.F) {
	for _, s := range [...][2]string{
		{"hello world", "hlw"},
		{"hello", ""},
		{"", "x"},
		{"", ""},
		{"Hello", "hello"},
	} {
		f.Add(s[0], s[1])
	}

	f.Fuzz(func(t *testing.T, line, query string) {
		got := FuzzyScore(line, query)

		if query == "" && got != 0 {
			t.Fatalf("FuzzyScore(%q, \"\") = %d; want 0", line, got)
		}
		if line == query && got < 0 {
			t.Fatalf("FuzzyScore(%q, %q): a string must fuzzy-match itself, got %d", line, query, got)
		}
	})
}

// FuzzHighlightMatchStyled exercises the substring / regex / fuzzy highlight
// paths (and their ansi.Cut / ansi.TruncateLeft bridges) for panics on
// arbitrary input, including invalid UTF-8.
//
// Note: we don't assert visible-text invariance. highlightSpans mixes byte
// offsets with visual columns, so stray control bytes (`\x19`) or lone
// non-UTF-8 bytes (`\x80`) can get duplicated or dropped in the output —
// those are pre-existing bugs in the column/byte index bridge worth their
// own fix. Panic-class bugs (e.g. the strings.ToLower-grows-past-plain
// crash) are the in-scope target here.
func FuzzHighlightMatchStyled(f *testing.F) {
	for _, s := range [...][2]string{
		{"hello world", "hello"},
		{"hello world", "~hlw"},
		{"hello world", "(invalid"},
		{"foo.bar.baz", "f.*z"},
		{"already \x1b[31mstyled\x1b[0m line", "styled"},
		{"000\xe7", "\xdc"}, // regression: strings.ToLower length-grow panic
		{"", ""},
	} {
		f.Add(s[0], s[1])
	}

	style := lipgloss.NewStyle().Background(lipgloss.Color("3"))

	f.Fuzz(func(t *testing.T, line, query string) {
		_ = HighlightMatchStyled(line, query, style)
	})
}

// FuzzHighlightMatchInline drives the inline-ANSI highlighter — a separate
// code path from highlightSpans that walks input bytes and tracks the
// currently-open SGR sequence so the post-match segment keeps its color.
// The state machine has its own pitfalls (escape-sequence boundary scans,
// SGR re-assertion). Panic discovery is the in-scope target.
func FuzzHighlightMatchInline(f *testing.F) {
	for _, s := range [...][2]string{
		{"hello world", "hello"},
		{"plain", ""},
		{"\x1b[31mred\x1b[0m blue", "red"},
		{"\x1b[33;1mtoken\x1b[0m", "[33m"}, // SGR-introducer query must not latch
		{"a\x1b[1mb\x1b[0mc", "abc"},
		{"", "anything"},
		{"hello world", "~hlw"}, // fuzzy mode falls back to Styled
	} {
		f.Add(s[0], s[1])
	}

	style := lipgloss.NewStyle().Background(lipgloss.Color("3"))

	f.Fuzz(func(t *testing.T, line, query string) {
		_ = HighlightMatchInline(line, query, style)
	})
}
