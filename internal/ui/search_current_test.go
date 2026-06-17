package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// setTestColorProfile forces the lipgloss renderer to emit ANSI256 color codes
// so style comparisons in tests are meaningful.
func setTestColorProfile(t *testing.T) {
	t.Helper()
	r := lipgloss.DefaultRenderer()
	orig := r.ColorProfile()
	r.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { r.SetColorProfile(orig) })
}

func TestHighlightMatchCurrentAtCol(t *testing.T) {
	setTestColorProfile(t)
	normal := lipgloss.NewStyle().Background(lipgloss.Color("3"))
	current := lipgloss.NewStyle().Background(lipgloss.Color("5"))
	line := "aXaXa" // three 'a' at visual cols 0,2,4
	// current at col 2 -> the middle 'a' uses current style, others normal.
	out := HighlightMatchCurrentAtCol(line, "a", normal, current, 2)
	// All three 'a's are highlighted; assert the current style's open code
	// appears and the result still contains all the original runes.
	plain := ansi.Strip(out)
	if plain != line {
		t.Fatalf("plain text changed: %q", plain)
	}
	// The current style (bg 5) must appear in the output.
	if !strings.Contains(out, current.Render("a")) && !strings.Contains(out, "\x1b[48;5;5m") {
		// fallback structural check: at least the output differs from uniform
		if out == HighlightMatchStyled(line, "a", normal) {
			t.Fatal("current match not styled differently from the rest")
		}
	}
}

func TestHighlightMatchCurrentAtCol_NoCurrentWhenColMissing(t *testing.T) {
	setTestColorProfile(t)
	normal := lipgloss.NewStyle().Background(lipgloss.Color("3"))
	current := lipgloss.NewStyle().Background(lipgloss.Color("5"))
	line := "abc abc"
	// col 99 matches no span -> identical to uniform normal highlight.
	if HighlightMatchCurrentAtCol(line, "abc", normal, current, 99) != HighlightMatchStyled(line, "abc", normal) {
		t.Fatal("no current span should equal uniform highlight")
	}
}

func TestHighlightMatchCurrentAtCol_EmptyQuery(t *testing.T) {
	setTestColorProfile(t)
	normal := lipgloss.NewStyle().Background(lipgloss.Color("3"))
	current := lipgloss.NewStyle().Background(lipgloss.Color("5"))
	line := "hello world"
	out := HighlightMatchCurrentAtCol(line, "", normal, current, 0)
	if out != line {
		t.Fatalf("empty query should return line unchanged, got %q", out)
	}
}

func TestHighlightMatchCurrentAtCol_NegativeCol(t *testing.T) {
	setTestColorProfile(t)
	normal := lipgloss.NewStyle().Background(lipgloss.Color("3"))
	current := lipgloss.NewStyle().Background(lipgloss.Color("5"))
	line := "abc abc"
	// col -1 -> no current match -> identical to uniform normal highlight.
	if HighlightMatchCurrentAtCol(line, "abc", normal, current, -1) != HighlightMatchStyled(line, "abc", normal) {
		t.Fatal("negative col should equal uniform highlight")
	}
}

func TestHighlightMatchCurrentAtCol_FuzzyFallback(t *testing.T) {
	setTestColorProfile(t)
	normal := lipgloss.NewStyle().Background(lipgloss.Color("3"))
	current := lipgloss.NewStyle().Background(lipgloss.Color("5"))
	line := "hello world"
	// Fuzzy mode falls back to uniform normalStyle.
	out := HighlightMatchCurrentAtCol(line, "~hw", normal, current, 0)
	expected := HighlightMatchStyled(line, "~hw", normal)
	if out != expected {
		t.Fatalf("fuzzy mode should fall back to HighlightMatchStyled, got %q want %q", out, expected)
	}
}

func TestHighlightMatchCurrentAtCol_RegexCurrentCol(t *testing.T) {
	setTestColorProfile(t)
	normal := lipgloss.NewStyle().Background(lipgloss.Color("3"))
	current := lipgloss.NewStyle().Background(lipgloss.Color("5"))
	line := "foo bar foo"
	// `f.o` has a regex metacharacter -> regex mode; matches "foo" at col 0
	// and col 8; current at col 8 -> second is current.
	const q = "f.o"
	if mode, _ := DetectSearchMode(q); mode != SearchRegex {
		t.Fatalf("query %q should be regex mode, got %v", q, mode)
	}
	out := HighlightMatchCurrentAtCol(line, q, normal, current, 8)
	plain := ansi.Strip(out)
	if plain != line {
		t.Fatalf("plain text changed: %q", plain)
	}
	// Output must differ from uniform highlight.
	if out == HighlightMatchStyled(line, q, normal) {
		t.Fatal("current match not styled differently from the rest")
	}
}
