package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

func TestRenderHintBar_SingleHint(t *testing.T) {
	hints := []HintEntry{
		{Key: "q", Desc: "quit"},
	}
	got := RenderHintBar(hints, 80)

	// Should contain the key styled with HelpKeyStyle and description with BarDimStyle.
	if !strings.Contains(got, "q") {
		t.Errorf("expected hint bar to contain key 'q', got: %s", got)
	}
	if !strings.Contains(got, "quit") {
		t.Errorf("expected hint bar to contain description 'quit', got: %s", got)
	}
	// Should NOT contain separator when there is only one hint.
	if strings.Contains(got, "|") {
		t.Errorf("expected no separator for single hint, got: %s", got)
	}
}

func TestRenderHintBar_MultipleHints(t *testing.T) {
	hints := []HintEntry{
		{Key: "q", Desc: "quit"},
		{Key: "j/k", Desc: "scroll"},
		{Key: "g/G", Desc: "top/bottom"},
	}
	got := RenderHintBar(hints, 120)

	// Should contain all keys and descriptions.
	for _, h := range hints {
		if !strings.Contains(got, h.Key) {
			t.Errorf("expected hint bar to contain key %q, got: %s", h.Key, got)
		}
		if !strings.Contains(got, h.Desc) {
			t.Errorf("expected hint bar to contain desc %q, got: %s", h.Desc, got)
		}
	}
	// Should contain the separator between entries.
	if !strings.Contains(got, "|") {
		t.Errorf("expected separator '|' in hint bar with multiple hints, got: %s", got)
	}
}

func TestRenderHintBar_EmptyHints(t *testing.T) {
	got := RenderHintBar(nil, 80)

	// Empty hints should still produce a styled bar (just empty content).
	// It should not panic or produce garbage.
	if got == "" {
		t.Error("expected non-empty string for empty hints (status bar wrapper should still render)")
	}
}

func TestRenderHintBar_ZeroWidth(t *testing.T) {
	hints := []HintEntry{
		{Key: "q", Desc: "quit"},
	}
	// Should not panic with zero width.
	got := RenderHintBar(hints, 0)
	if got == "" {
		t.Error("expected non-empty string even with zero width")
	}
}

func TestFormatHintParts_SingleHint(t *testing.T) {
	hints := []HintEntry{
		{Key: "q", Desc: "quit"},
	}
	got := FormatHintParts(hints)

	if !strings.Contains(got, "q") {
		t.Errorf("expected formatted hints to contain key 'q', got: %s", got)
	}
	if !strings.Contains(got, "quit") {
		t.Errorf("expected formatted hints to contain desc 'quit', got: %s", got)
	}
}

func TestFormatHintParts_MultipleHints(t *testing.T) {
	hints := []HintEntry{
		{Key: "a", Desc: "first"},
		{Key: "b", Desc: "second"},
	}
	got := FormatHintParts(hints)

	if !strings.Contains(got, "a") || !strings.Contains(got, "first") {
		t.Errorf("expected first hint in output, got: %s", got)
	}
	if !strings.Contains(got, "b") || !strings.Contains(got, "second") {
		t.Errorf("expected second hint in output, got: %s", got)
	}
	if !strings.Contains(got, "|") {
		t.Errorf("expected separator in output, got: %s", got)
	}
}

func TestFormatHintParts_Empty(t *testing.T) {
	got := FormatHintParts(nil)
	if got != "" {
		t.Errorf("expected empty string for nil hints, got: %q", got)
	}
}

func TestFormatHintPartsFit(t *testing.T) {
	hints := []HintEntry{
		{Key: "j/k", Desc: "move"},
		{Key: "enter", Desc: "view"},
		{Key: "ctrl+r", Desc: "toggle RO"},
		{Key: "a", Desc: "create"},
		{Key: "?", Desc: "help"},
	}

	t.Run("empty hints returns empty", func(t *testing.T) {
		assert.Equal(t, "", FormatHintPartsFit(nil, 80))
	})

	t.Run("non-positive width returns empty", func(t *testing.T) {
		assert.Equal(t, "", FormatHintPartsFit(hints, 0))
		assert.Equal(t, "", FormatHintPartsFit(hints, -1))
	})

	t.Run("all entries fit returns full join", func(t *testing.T) {
		got := FormatHintPartsFit(hints, 200)
		full := FormatHintParts(hints)
		assert.Equal(t, full, got, "with ample width all entries appear in full")
	})

	t.Run("first entry doesn't fit returns empty", func(t *testing.T) {
		assert.Equal(t, "", FormatHintPartsFit(hints, 5))
	})

	t.Run("greedy fit lands exactly on entry boundary", func(t *testing.T) {
		two := FormatHintParts(hints[:2])
		got := FormatHintPartsFit(hints, lipgloss.Width(two))
		assert.Equal(t, two, got, "must fit exactly the first two entries")
	})

	t.Run("never cuts mid-description", func(t *testing.T) {
		two := FormatHintParts(hints[:2])
		budget := lipgloss.Width(two) + 5 // mid-third entry territory
		got := FormatHintPartsFit(hints, budget)
		stripped := stripANSI(got)
		assert.NotContains(t, stripped, "ctrl+r:", "third entry must not appear partially")
		assert.NotContains(t, stripped, "toggle", "no fragment of the third entry's desc")
		assert.Contains(t, stripped, "j/k: move")
		assert.Contains(t, stripped, "enter: view")
	})

	t.Run("skips a too-large entry but keeps later shorter ones", func(t *testing.T) {
		// Width 35 fits j/k + enter + (skip ctrl+r, too wide) + a.
		got := FormatHintPartsFit(hints, 35)
		stripped := stripANSI(got)
		assert.Contains(t, stripped, "j/k: move")
		assert.Contains(t, stripped, "enter: view")
		assert.NotContains(t, stripped, "ctrl+r: toggle RO",
			"the wide entry that doesn't fit must be skipped")
		assert.Contains(t, stripped, "a: create",
			"a shorter later entry must be picked up after a skip")
	})
}

func TestJoinStatusBar(t *testing.T) {
	t.Run("returns empty for non-positive width", func(t *testing.T) {
		assert.Equal(t, "", JoinStatusBar("left", "right", 0))
		assert.Equal(t, "", JoinStatusBar("left", "right", -5))
	})

	t.Run("returns empty when both sides empty", func(t *testing.T) {
		assert.Equal(t, "", JoinStatusBar("", "", 80))
	})

	t.Run("right-aligns hints with elastic spacer when both fit", func(t *testing.T) {
		out := JoinStatusBar("LEFT", "RIGHT", 20)
		assert.Equal(t, 20, lipgloss.Width(out), "must fill exactly width columns")
		assert.True(t, strings.HasPrefix(out, "LEFT"), "left content sits at the left edge")
		assert.True(t, strings.HasSuffix(out, "RIGHT"), "right content sits at the right edge")
		// Middle is whitespace-only.
		middle := out[len("LEFT") : len(out)-len("RIGHT")]
		assert.Equal(t, strings.Repeat(" ", len(middle)), middle, "spacer must be pure whitespace")
	})

	t.Run("only-left content padded to fill width", func(t *testing.T) {
		out := JoinStatusBar("LEFT", "", 10)
		assert.Equal(t, 10, lipgloss.Width(out))
		assert.True(t, strings.HasPrefix(out, "LEFT"))
		assert.Equal(t, "      ", out[len("LEFT"):], "trailing spaces fill the bar")
	})

	t.Run("only-right content padded to fill width with leading spaces", func(t *testing.T) {
		out := JoinStatusBar("", "HINTS", 10)
		assert.Equal(t, 10, lipgloss.Width(out))
		assert.True(t, strings.HasSuffix(out, "HINTS"))
		assert.Equal(t, "     ", out[:len(out)-len("HINTS")], "leading spaces right-align the hints")
	})

	t.Run("left hard-cut without marker when combined exceeds width", func(t *testing.T) {
		// Left is 30 cols, right is 15 cols, width is 30. Right must stay
		// intact; left must shrink. The cut is unmarked — the gap between
		// the truncated left and the right is whitespace-only, never a `~`,
		// because the explorer status bar relies on a clean visual gutter
		// between the keymap and the chip group.
		left := strings.Repeat("L", 30)
		right := strings.Repeat("R", 15)
		out := JoinStatusBar(left, right, 30)
		assert.Equal(t, 30, lipgloss.Width(out), "must fill exactly width columns")
		assert.True(t, strings.HasSuffix(out, right), "info chips (right) must survive intact")
		assert.NotContains(t, out, "~", "no truncate marker may bleed into the separator")
	})

	t.Run("right truncated when alone wider than width", func(t *testing.T) {
		left := "DROPPED"
		right := strings.Repeat("R", 30)
		out := JoinStatusBar(left, right, 10)
		assert.Equal(t, 10, lipgloss.Width(out), "must not exceed width")
		assert.NotContains(t, out, "DROPPED", "left chunk dropped when right alone overflows")
	})

	t.Run("info chips survive a long keymap — explorer bar contract", func(t *testing.T) {
		// The explorer's full keymap is ~165 cols on its own. JoinStatusBar
		// pins the right-anchored info chips (sort, counter / selected, etc.)
		// to the right edge and truncates the keymap on overflow so the
		// user keeps the at-a-glance state indicators intact even on a
		// terminal narrow enough to clip the hint list.
		bigLeft := strings.Repeat("hint | ", 25) // ~175 cols
		chips := "sort:name  [42/100]"
		out := JoinStatusBar(bigLeft, chips, 60)
		assert.Equal(t, 60, lipgloss.Width(out))
		assert.True(t, strings.HasSuffix(out, chips),
			"chips must remain at the right edge in full; got %q", out)
	})
}
