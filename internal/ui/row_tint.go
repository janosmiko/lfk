// Package ui - row_tint.go
// Row-status-tint style selection (issue #540): failed/progressing rows are
// emphasized beyond the Status cell. The RowTint* style globals live in
// styles.go / theme.go / theme_nocolor.go; the selection logic lives here.
package ui

import "github.com/charmbracelet/lipgloss"

// RowTintForStatus returns the whole-row emphasis style for a status per
// ConfigRowStatusTint, and whether one applies (issue #540). Only failed and
// progressing severities tint; everything else keeps the default row look.
func RowTintForStatus(status string) (lipgloss.Style, bool) {
	if ConfigRowStatusTint == RowStatusTintOff {
		return lipgloss.Style{}, false
	}
	bg := ConfigRowStatusTint == RowStatusTintBackground
	switch statusSeverity(status) {
	case sevFailed:
		if bg {
			return RowTintFailedBg, true
		}
		return RowTintFailedFg, true
	case sevProgressing:
		if bg {
			return RowTintProgressingBg, true
		}
		return RowTintProgressingFg, true
	default:
		return lipgloss.Style{}, false
	}
}

// rowTintForeground returns the foreground-variant tint style for a status and
// whether one applies (failed/progressing, mode != off). Used both for the
// name-only tint (non-cursor rows whose Status column is hidden) and for the
// cursor row, where the selection background owns the background channel so the
// status must ride on the foreground color even in "background" config mode —
// keeping a selected+failed row reading as "selected AND failed" instead of
// losing its signal under the highlight (issue #540).
func rowTintForeground(status string) (lipgloss.Style, bool) {
	if ConfigRowStatusTint == RowStatusTintOff {
		return lipgloss.Style{}, false
	}
	switch statusSeverity(status) {
	case sevFailed:
		return RowTintFailedFg, true
	case sevProgressing:
		return RowTintProgressingFg, true
	default:
		return lipgloss.Style{}, false
	}
}

// cursorKeptBackgroundTint returns the cursor row's background style in
// background mode, and whether it applies. Instead of swapping in the selection
// background (which would erase the status color) the cursor row uses the status
// background blended toward the selection color, so it reads as both "failed"
// and "cursor" (issue #540 UAT). No-color mode has no background to blend, so
// the cursor falls back to the normal reverse-video selection.
func cursorKeptBackgroundTint(status string) (lipgloss.Style, bool) {
	if ConfigRowStatusTint != RowStatusTintBackground || ConfigNoColor {
		return lipgloss.Style{}, false
	}
	switch statusSeverity(status) {
	case sevFailed:
		return RowTintFailedCursorBg, true
	case sevProgressing:
		return RowTintProgressingCursorBg, true
	default:
		return lipgloss.Style{}, false
	}
}

// tintedSelectionMarker renders the multi-select checkmark carrying the row's
// status background, so on a whole-row-tinted line the marker cell blends into
// the row instead of showing a default-background gap. It then re-asserts the
// tint so the cells after the marker keep the status background past the
// marker's own SGR reset (issue #540 UAT).
func tintedSelectionMarker(tint lipgloss.Style) string {
	return SelectionMarkerStyle.Background(tint.GetBackground()).Render(selectionMarker) + styleOpenCodes(tint)
}

// mergeRowTintIntoSelected layers a foreground-variant tint onto the selection
// style. In color mode the severity foreground color carries the signal. In
// no-color mode the tint uses bold (failed) / italic (progressing) attributes;
// italic survives on the selection as-is, but the selection style is already
// bold, so a failed row's bold cue would vanish — fall back to underline so a
// selected failed row stays distinct from a selected healthy one.
func mergeRowTintIntoSelected(sel, tint lipgloss.Style) lipgloss.Style {
	if fg := tint.GetForeground(); fg != (lipgloss.NoColor{}) {
		return sel.Foreground(fg)
	}
	if tint.GetItalic() {
		sel = sel.Italic(true)
	}
	if tint.GetBold() {
		if sel.GetBold() {
			sel = sel.Underline(true)
		} else {
			sel = sel.Bold(true)
		}
	}
	return sel
}
