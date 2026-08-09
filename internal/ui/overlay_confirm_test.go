package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
)

// --- RenderOverlayConfirm ---

func TestRenderOverlayConfirm(t *testing.T) {
	t.Run("title + warning rendered", func(t *testing.T) {
		out := RenderOverlayConfirm(OverlayConfirmConfig{
			Title:   "Confirm Delete",
			Warning: "Delete my-pod?",
		})
		assert.Contains(t, out, "Confirm Delete")
		assert.Contains(t, out, "Delete my-pod?")
	})

	t.Run("body lines rendered in order", func(t *testing.T) {
		out := RenderOverlayConfirm(OverlayConfirmConfig{
			Title: "Paste",
			Body:  []string{"Paste contains 12 lines.", "Flatten and paste?"},
		})
		assert.Contains(t, out, "Paste contains 12 lines.")
		assert.Contains(t, out, "Flatten and paste?")
	})

	t.Run("choice row renders label and value", func(t *testing.T) {
		out := RenderOverlayConfirm(OverlayConfirmConfig{
			Title:       "Confirm Delete",
			Warning:     "Delete job/backup-nightly?",
			ChoiceLabel: "Cascade",
			ChoiceValue: "Background",
		})
		assert.Contains(t, out, "Cascade")
		assert.Contains(t, out, "Background")
		// The choice belongs below the question, not spliced into it.
		assert.Less(t, strings.Index(stripANSI(out), "Delete job"), strings.Index(stripANSI(out), "Cascade"))
	})

	t.Run("ChoiceWarn styles the value differently from the safe default", func(t *testing.T) {
		// Styles collapse to bare text without a color profile, which would
		// make this assertion vacuous.

		plain := RenderOverlayConfirm(OverlayConfirmConfig{
			Title:       "Confirm Delete",
			ChoiceLabel: "Cascade",
			ChoiceValue: "Orphan",
		})
		warned := RenderOverlayConfirm(OverlayConfirmConfig{
			Title:       "Confirm Delete",
			ChoiceLabel: "Cascade",
			ChoiceValue: "Orphan",
			ChoiceWarn:  true,
		})

		assert.Equal(t, stripANSI(plain), stripANSI(warned), "only styling may differ")
		assert.NotEqual(t, plain, warned, "a warned choice must not render identically to a safe one")
	})

	t.Run("choice row omitted when value empty", func(t *testing.T) {
		out := RenderOverlayConfirm(OverlayConfirmConfig{
			Title:       "Confirm Delete",
			Warning:     "Delete my-pod?",
			ChoiceLabel: "Cascade",
		})
		assert.NotContains(t, out, "Cascade")
	})

	t.Run("type-to-confirm renders DELETE token + input", func(t *testing.T) {
		out := RenderOverlayConfirm(OverlayConfirmConfig{
			Title:     "Confirm Delete",
			Warning:   "Delete my-pod?",
			TypeToken: "DELETE",
			Input:     "DEL",
		})
		assert.Contains(t, out, "DELETE")
		assert.Contains(t, out, "Type ")
		assert.Contains(t, out, "to confirm")
		assert.Contains(t, out, "DEL")
	})

	t.Run("type-to-confirm with empty input shows placeholder", func(t *testing.T) {
		out := RenderOverlayConfirm(OverlayConfirmConfig{
			Title:     "Confirm Delete",
			Warning:   "Delete my-pod?",
			TypeToken: "DELETE",
			Input:     "",
		})
		assert.Contains(t, out, "DELETE")
		assert.Contains(t, out, "_")
	})

	t.Run("empty TypeToken skips type-to-confirm row", func(t *testing.T) {
		out := RenderOverlayConfirm(OverlayConfirmConfig{
			Title:   "Confirm Delete",
			Warning: "Delete my-pod?",
		})
		assert.NotContains(t, out, "to confirm")
	})

	t.Run("centered mode renders single line centered in a fixed box", func(t *testing.T) {
		out := RenderOverlayConfirm(OverlayConfirmConfig{
			Title:       "Quit lfk?",
			Centered:    true,
			InnerWidth:  26,
			InnerHeight: 1,
		})
		assert.Contains(t, out, "Quit lfk?")
		// Visible width of the rendered box must be at least the InnerWidth.
		w := 0
		for l := range strings.SplitSeq(out, "\n") {
			if lw := lipgloss.Width(l); lw > w {
				w = lw
			}
		}
		assert.GreaterOrEqual(t, w, 26)
	})

	t.Run("centered mode fills InnerHeight rows with the title on the middle row", func(t *testing.T) {
		// 3-row inner area: question must sit on row 1 (middle), with one
		// blank row above and below. Pins the layout that PRs #80, #97 tried
		// to break with inline hint additions.
		out := RenderOverlayConfirm(OverlayConfirmConfig{
			Title: "Quit lfk?", Centered: true, InnerWidth: 26, InnerHeight: 3,
		})
		lines := strings.Split(out, "\n")
		assert.Equal(t, 3, len(lines), "must fill InnerHeight rows")
		row := -1
		for i, l := range lines {
			if strings.Contains(stripANSI(l), "Quit lfk?") {
				row = i
				break
			}
		}
		assert.Equal(t, 1, row, "title must sit on the middle row")
	})

	t.Run("paste-style body has no inline y/n hints", func(t *testing.T) {
		// Hint bar is the single source of truth for confirm dialogs.
		// PRs #80, #97 proposed inline `[y] yes [n] no` text inside the
		// overlay body and were closed as inconsistent — pin that here.
		out := RenderOverlayConfirm(OverlayConfirmConfig{
			Title: "Paste",
			Body:  []string{"Paste contains 7 lines.", "Flatten and paste?"},
		})
		out = stripANSI(out)
		assert.Contains(t, out, "Paste")
		assert.Contains(t, out, "Paste contains 7 lines.")
		assert.Contains(t, out, "Flatten and paste?")
		assert.NotContains(t, out, "[y]")
		assert.NotContains(t, out, "[n]")
		assert.NotContains(t, out, " yes")
		assert.NotContains(t, out, " no")
	})

	t.Run("centered mode ignores Warning/Body/TypeToken", func(t *testing.T) {
		out := RenderOverlayConfirm(OverlayConfirmConfig{
			Title:       "Quit lfk?",
			Warning:     "ignored",
			Body:        []string{"ignored body"},
			TypeToken:   "DELETE",
			Centered:    true,
			InnerWidth:  20,
			InnerHeight: 1,
		})
		assert.Contains(t, out, "Quit lfk?")
		assert.NotContains(t, out, "ignored")
		assert.NotContains(t, out, "ignored body")
		assert.NotContains(t, out, "to confirm")
	})
}

func TestRenderOverlayConfirm_NotesAppearBelowTheChoiceRow(t *testing.T) {
	out := stripANSI(RenderOverlayConfirm(OverlayConfirmConfig{
		Title:       "Confirm Delete",
		Warning:     "Delete web?",
		ChoiceLabel: "Cascade",
		ChoiceValue: "Background",
		Notes: []ConfirmNote{
			{Text: "PDB web-pdb 2 -> 0 allowed, 3 of 5 ready after"},
		},
	}))

	assert.Contains(t, out, "PDB web-pdb 2 -> 0 allowed")
	assert.Less(t, strings.Index(out, "Background"), strings.Index(out, "PDB web-pdb"),
		"the note sits below the cascade row, not above it")
}

func TestRenderOverlayConfirm_AWarningNoteIsStyledApart(t *testing.T) {
	plain := RenderOverlayConfirm(OverlayConfirmConfig{
		Title: "t", Notes: []ConfirmNote{{Text: "same text"}},
	})
	warned := RenderOverlayConfirm(OverlayConfirmConfig{
		Title: "t", Notes: []ConfirmNote{{Text: "same text", Warn: true}},
	})

	assert.Equal(t, stripANSI(plain), stripANSI(warned), "same words")
	assert.NotEqual(t, plain, warned, "a PDB violation must not look like an ordinary note")
}

func TestRenderOverlayConfirm_NoNotesChangesNothing(t *testing.T) {
	without := RenderOverlayConfirm(OverlayConfirmConfig{Title: "t", Warning: "w"})
	empty := RenderOverlayConfirm(OverlayConfirmConfig{Title: "t", Warning: "w", Notes: nil})

	assert.Equal(t, without, empty)
}
