package ui

import (
	"regexp"
	"strings"
)

// dimSGR is the SGR "faint" attribute (decreased intensity). It is applied
// as a non-destructive wrap on top of each line's existing styling so the
// theme's foreground and background colours survive — terminals render the
// original colour at reduced brightness rather than the line going gray.
//
// dimReset terminates the run with SGR 0 (full reset). Each dimmed line is
// closed with this so faint does not bleed into whatever a downstream
// renderer (e.g. PlaceOverlay's lipgloss styling) emits next.
const (
	dimSGR   = "\x1b[2m"
	dimReset = "\x1b[0m"
)

// sgrResetRe matches every common SGR reset form. After each match we
// re-emit dimSGR so the faint attribute persists through mid-line resets
// that the original explorer rendering inserted (lipgloss closes every
// styled run with `\x1b[0m`, which would otherwise clear faint and leave
// the rest of the line at full brightness).
var sgrResetRe = regexp.MustCompile(`\x1b\[0?m`)

// DimBackground wraps each non-kept line with the SGR 2 (faint) attribute,
// re-applying it after every internal reset so faint persists through the
// resets that lipgloss inserts mid-line. The line's existing foreground,
// background, and bold styling all pass through unchanged — terminals
// render the original theme styling at reduced brightness rather than
// replacing it with a flat gray. Selection highlights keep both their
// theme colour and their bold weight so the user can still see which row
// is the cursor target while the overlay is up.
//
// keepLast bottom lines pass through verbatim — the hint bar carries the
// overlay's keymap and must stay at full intensity. A trailing newline on
// the input is normalised before split-and-process and restored on the
// way out, so `keepLast` always counts visible rows rather than the
// empty trailer that strings.Split would otherwise produce. Line count
// (and trailing newline, if any) is preserved so callers can feed the
// result back into PlaceOverlay (which is height-sensitive) without
// realigning. keepLast is clamped to [0, len(lines)]; passing a value
// >= line count returns the input unchanged, and negative values are
// treated as zero (dim everything).
//
// When ConfigNoColor is on the function is a no-op: NoColor mode promises
// a colour-escape-free render and even SGR 2 would be a colour escape.
// Callers that need an unconditional dim should bypass this function.
func DimBackground(s string, keepLast int) string {
	if s == "" {
		return s
	}
	if ConfigNoColor {
		return s
	}
	hasTrailingNewline := strings.HasSuffix(s, "\n")
	trimmed := strings.TrimSuffix(s, "\n")
	lines := strings.Split(trimmed, "\n")
	if keepLast >= len(lines) {
		return s
	}
	if keepLast < 0 {
		keepLast = 0
	}
	cutoff := len(lines) - keepLast

	for i := range cutoff {
		if lines[i] == "" {
			continue
		}
		// Re-apply faint after every full reset so it survives the
		// `\x1b[0m` runs that lipgloss inserts between styled segments.
		relined := sgrResetRe.ReplaceAllStringFunc(lines[i], func(m string) string {
			return m + dimSGR
		})
		lines[i] = dimSGR + relined + dimReset
	}
	out := strings.Join(lines, "\n")
	if hasTrailingNewline {
		out += "\n"
	}
	return out
}
