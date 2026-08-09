package app

import (
	"fmt"

	"charm.land/lipgloss/v2"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// renderOverlayConfirm renders the simple y/n confirm box.
func (m Model) renderOverlayConfirm() (string, int, int, bool) {
	// Default to the delete wording; non-delete confirm actions (e.g.
	// Longhorn replica eviction) set confirmTitle/confirmQuestion to
	// override it.
	title := m.confirmTitle
	if title == "" {
		title = "Confirm Delete"
	}
	warning := m.confirmQuestion
	if warning == "" {
		warning = fmt.Sprintf("Delete %s?", m.confirmAction)
	}
	w := min(50, m.width-10)
	// The cascade row only shows for deletes that go through DeleteResource;
	// it costs two extra rows when present.
	choiceLabel, choiceValue, h := "", "", min(8, m.height-6)
	choiceWarn := false
	if m.deleteConfirmShowsPolicy() {
		choiceLabel, choiceValue, choiceWarn = cascadeChoiceRow(m.deletePropagation(), w-4)
		h = min(10, m.height-6)
	}
	// A drain has no single workload, so its line drops the replica count.
	notes := blastRadiusNotes(m.blast.radius, m.blast.loading, m.pendingAction != "Drain")
	h = min(h+blastRadiusRows(notes, w-4), m.height-6)
	return ui.RenderOverlayConfirm(ui.OverlayConfirmConfig{
		Title:       title,
		Warning:     warning,
		ChoiceLabel: choiceLabel,
		ChoiceValue: choiceValue,
		ChoiceWarn:  choiceWarn,
		Notes:       notes,
		WrapWidth:   w - 4,
	}), w, h, true
}

// cascadeChoiceRow builds the Cascade row for a confirm box of inner width
// columns, returning the label, value, and whether the value needs warning
// styling. Orphan and None can both leave workloads running, so they say why.
func cascadeChoiceRow(policy model.DeletePropagation, inner int) (string, string, bool) {
	note, warn := "", false
	if policy.NeedsWarning() {
		warn = true
		note = " (dependents kept)"
		if policy.DefersToServer() {
			note = " (server default)"
		}
	}
	label, value := fitChoiceRow("Cascade", policy.Label(), note, inner)
	return label, value, warn
}

// fitChoiceRow picks the most informative form of the choice row that fits
// inner display columns. The row is rendered on one unwrapped line, so it sheds
// the explanatory note first, then the label, and only truncates the value as a
// last resort — the value is the part the user is choosing.
func fitChoiceRow(label, value, note string, inner int) (string, string) {
	for _, candidate := range [][2]string{
		{label, value + note},
		{label, value},
		{"", value + note},
		{"", value},
	} {
		width := lipgloss.Width(candidate[1])
		if candidate[0] != "" {
			width += lipgloss.Width(candidate[0] + ": ")
		}
		if width <= inner {
			return candidate[0], candidate[1]
		}
	}
	return "", ui.Truncate(value, max(inner, 0))
}

// renderOverlayConfirmType renders the type-to-confirm box (force delete,
// force finalize).
func (m Model) renderOverlayConfirmType() (string, int, int, bool) {
	w := min(55, m.width-10)
	choiceLabel, choiceValue, h := "", "", min(10, m.height-6)
	choiceWarn := false
	if m.forceDeleteConfirmShowsPolicy() {
		choiceLabel, choiceValue, choiceWarn = cascadeChoiceRow(m.deletePropagation().Cascading(), w-4)
		h = min(12, m.height-6)
	}
	return ui.RenderOverlayConfirm(ui.OverlayConfirmConfig{
		Title:       m.confirmTitle,
		Warning:     m.confirmQuestion,
		ChoiceLabel: choiceLabel,
		ChoiceValue: choiceValue,
		ChoiceWarn:  choiceWarn,
		TypeToken:   "DELETE",
		Input:       m.confirmTypeInput.Value,
		WrapWidth:   w - 4,
	}), w, h, true
}
