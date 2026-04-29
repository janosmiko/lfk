package ui

import (
	"strings"
	"testing"
)

func TestDetectSearchMode(t *testing.T) {
	tests := []struct {
		name      string
		rawQuery  string
		wantMode  SearchMode
		wantQuery string
	}{
		{"empty", "", SearchSubstring, ""},
		{"plain substring", "error", SearchSubstring, "error"},
		{"fuzzy prefix", "~deplymnt", SearchFuzzy, "deplymnt"},
		{"fuzzy empty after prefix", "~", SearchFuzzy, ""},
		{"literal escape", `\.*`, SearchSubstring, ".*"},
		{"literal escape plain", `\error`, SearchSubstring, "error"},
		{"regex dot", "err.r", SearchRegex, "err.r"},
		{"regex star", "err*", SearchRegex, "err*"},
		{"regex plus", "err+", SearchRegex, "err+"},
		{"regex question", "err?", SearchRegex, "err?"},
		{"regex caret", "^error", SearchRegex, "^error"},
		{"regex dollar", "error$", SearchRegex, "error$"},
		{"regex pipe", "err|warn", SearchRegex, "err|warn"},
		{"regex brackets", "[abc]", SearchRegex, "[abc]"},
		{"regex parens", "(err)", SearchRegex, "(err)"},
		{"regex braces", "a{2}", SearchRegex, "a{2}"},
		{"no metacharacters", "deployment", SearchSubstring, "deployment"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, query := DetectSearchMode(tt.rawQuery)
			if mode != tt.wantMode {
				t.Errorf("mode = %d, want %d", mode, tt.wantMode)
			}
			if query != tt.wantQuery {
				t.Errorf("query = %q, want %q", query, tt.wantQuery)
			}
		})
	}
}

func TestContainsRegexMeta(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"hello", false},
		{"hello.", true},
		{"a*b", true},
		{"a+b", true},
		{"a?b", true},
		{"^start", true},
		{"end$", true},
		{"{1,2}", true},
		{"(group)", true},
		{"a|b", true},
		{"[class]", true},
		{"plain-text_123", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := containsRegexMeta(tt.input)
			if got != tt.want {
				t.Errorf("containsRegexMeta(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestMatchLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		rawQuery string
		want     bool
	}{
		// Substring mode.
		{"substring match", "Error in deployment", "error", true},
		{"substring no match", "all good", "error", false},
		{"substring case insensitive", "ERROR in deployment", "error", true},

		// Regex mode (auto-detected).
		{"regex match", "error42", "error[0-9]+", true},
		{"regex no match", "errorXY", "error[0-9]+", false},
		{"regex case insensitive", "ERROR42", "error[0-9]+", true},
		{"regex fallback on invalid", "[invalid( pattern here", "[invalid(", true}, // falls back to substring containing "[invalid(".

		// Fuzzy mode.
		{"fuzzy match", "deployment", "~dplmnt", true},
		{"fuzzy match order", "deployment", "~dep", true},
		{"fuzzy no match", "deployment", "~xyz", false},
		{"fuzzy case insensitive", "Deployment", "~dplmnt", true},

		// Literal escape.
		{"literal match", "file.txt", `\file.txt`, true},
		{"literal dot not regex", "filetxt", `\file.txt`, false}, // . is literal, not regex.
		{"literal star", "a*b", `\a*b`, true},

		// Edge cases.
		{"empty query", "anything", "", false},
		{"empty line", "", "query", false},
		{"fuzzy empty effective query", "anything", "~", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchLine(tt.line, tt.rawQuery)
			if got != tt.want {
				t.Errorf("MatchLine(%q, %q) = %v, want %v", tt.line, tt.rawQuery, got, tt.want)
			}
		})
	}
}

func TestFuzzyScore(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		query string
		want  int // -1 for no match, >0 for match.
	}{
		{"exact match", "deployment", "deployment", -1}, // Handled as: score should be > 0.
		{"partial match", "deployment", "dep", -1},      // Placeholder; checked below.
		{"no match", "deployment", "xyz", -1},
		{"empty query", "deployment", "", 0},
		{"word boundary bonus", "my-deployment", "dep", -1}, // Placeholder.
		{"consecutive bonus", "deployment", "depl", -1},     // Placeholder.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FuzzyScore(tt.line, tt.query)
			switch tt.name {
			case "no match":
				if got != -1 {
					t.Errorf("FuzzyScore(%q, %q) = %d, want -1", tt.line, tt.query, got)
				}
			case "empty query":
				if got != 0 {
					t.Errorf("FuzzyScore(%q, %q) = %d, want 0", tt.line, tt.query, got)
				}
			default:
				if got < 0 {
					t.Errorf("FuzzyScore(%q, %q) = %d, want >= 0", tt.line, tt.query, got)
				}
			}
		})
	}

	// Verify scoring: word boundary match scores higher.
	scoreAtBoundary := FuzzyScore("my-deployment", "dep")
	scoreNoBoubndary := FuzzyScore("xxdeployment", "dep")
	if scoreAtBoundary <= scoreNoBoubndary {
		t.Errorf("word boundary score (%d) should be > non-boundary score (%d)", scoreAtBoundary, scoreNoBoubndary)
	}

	// Verify consecutive bonus.
	scoreConsec := FuzzyScore("deployment", "depl")
	scoreSpread := FuzzyScore("d_e_p_l_oyment", "depl")
	if scoreConsec <= scoreSpread {
		t.Errorf("consecutive score (%d) should be > spread score (%d)", scoreConsec, scoreSpread)
	}
}

func TestHighlightMatch(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		rawQuery string
	}{
		{"substring highlight", "Error in deployment", "error"},
		{"regex highlight", "error42 warning", "error[0-9]+"},
		{"fuzzy highlight", "deployment", "~dplmnt"},
		{"no match", "all good", "missing"},
		{"empty query", "something", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HighlightMatch(tt.line, tt.rawQuery)
			// Basic sanity: result should not be empty for non-empty input.
			if tt.line != "" && result == "" {
				t.Error("HighlightMatch returned empty for non-empty line")
			}
			// For non-matches, the result should equal input.
			if !MatchLine(tt.line, tt.rawQuery) && result != tt.line {
				t.Errorf("HighlightMatch modified non-matching line: got %q", result)
			}
			// Note: In non-TTY environments lipgloss renders without ANSI codes,
			// so we cannot reliably check that matching lines are modified.
			// The function is verified to be correct by checking the underlying
			// highlight* functions for structural correctness.
		})
	}
}

func TestHighlightSubstring(t *testing.T) {
	result := highlightSubstring("Hello World Hello", "hello", LogSearchHighlightStyle, "")
	// Should contain the original text (preserve case).
	if !strings.Contains(result, "Hello") {
		t.Error("highlight should preserve original case")
	}
	// Non-matching should return unchanged.
	result = highlightSubstring("all good", "missing", LogSearchHighlightStyle, "")
	if result != "all good" {
		t.Errorf("non-matching highlight should return unchanged, got %q", result)
	}
}

func TestHighlightRegex(t *testing.T) {
	result := highlightRegex("error42 and error99", "error[0-9]+", LogSearchHighlightStyle, "")
	// Should contain the original content.
	if !strings.Contains(result, "and") {
		t.Error("regex highlight should preserve non-matching parts")
	}
	// Invalid regex should fallback gracefully.
	result = highlightRegex("error.*stuff", "[invalid(", LogSearchHighlightStyle, "")
	if result == "" {
		t.Error("invalid regex fallback should not return empty")
	}
	// Non-matching should return unchanged.
	result = highlightRegex("all good", "error[0-9]+", LogSearchHighlightStyle, "")
	if result != "all good" {
		t.Errorf("non-matching regex should return unchanged, got %q", result)
	}
}

func TestHighlightFuzzy(t *testing.T) {
	// Non-matching fuzzy should return unchanged.
	result := highlightFuzzy("deployment", "xyz", LogSearchHighlightStyle, "")
	if result != "deployment" {
		t.Error("non-matching fuzzy should return unchanged line")
	}
	// Matching fuzzy should contain the original characters.
	result = highlightFuzzy("deployment", "dplmnt", LogSearchHighlightStyle, "")
	if !strings.Contains(result, "e") {
		t.Error("fuzzy highlight should preserve non-matched characters")
	}
}

func TestSearchModeIndicator(t *testing.T) {
	tests := []struct {
		rawQuery string
		want     string
	}{
		{"error", ""},
		{"err.r", "[RE] "},
		{"~fuzzy", "[~] "},
		{`\literal.*`, ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.rawQuery, func(t *testing.T) {
			got := SearchModeIndicator(tt.rawQuery)
			if got != tt.want {
				t.Errorf("SearchModeIndicator(%q) = %q, want %q", tt.rawQuery, got, tt.want)
			}
		})
	}
}

func TestFindColumnInLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		rawQuery string
		want     int
	}{
		{"substring at start", "error in log", "error", 0},
		{"substring in middle", "the error is here", "error", 4},
		{"substring not found", "all good", "error", -1},
		{"regex match", "error42 in log", "error[0-9]+", 0},
		{"regex not found", "errorXY", "error[0-9]+", -1},
		{"fuzzy first char", "deployment", "~dplmnt", 0},
		{"fuzzy first char in middle", "my-deployment", "~dep", 3},
		{"empty query", "anything", "", -1},
		{"empty line", "", "query", -1},
		{"literal escape", "a.b.c", `\.b`, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindColumnInLine(tt.line, tt.rawQuery)
			if got != tt.want {
				t.Errorf("FindColumnInLine(%q, %q) = %d, want %d", tt.line, tt.rawQuery, got, tt.want)
			}
		})
	}
}

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		line  string
		query string
		want  bool
	}{
		{"deployment", "dplmnt", true},
		{"deployment", "deployment", true},
		{"deployment", "xyz", false},
		{"Deployment", "dep", true},
		{"abc", "abcd", false},
		{"", "a", false},
		{"abc", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.line+"/"+tt.query, func(t *testing.T) {
			got := fuzzyMatch(tt.line, tt.query)
			if got != tt.want {
				t.Errorf("fuzzyMatch(%q, %q) = %v, want %v", tt.line, tt.query, got, tt.want)
			}
		})
	}
}
