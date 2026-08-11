package ui

import (
	"fmt"
	"strings"
)

// UndeliverableRow is the renderer's input - the minimum data to render one
// row. Field names match k8s.UndeliverableItem so the caller is a trivial copy.
type UndeliverableRow struct {
	Kind, Namespace, Name, Reason string
}

// undeliverableChrome is the number of lines RenderOverlayList emits above
// the rows for this overlay's config: the title, the subtitle, the filter
// prompt, and the blank line after it. The move handler sizes its viewport
// off the same constant, so the cursor can never land on a row the renderer
// did not emit.
const undeliverableChrome = 4

// undeliverableKindW fits "PersistentVolumeClaim" (21), the longest kind any
// detector reports.
const undeliverableKindW = 22

// UndeliverableBodyHeight returns how many rows fit inside an overlay of
// outer height `overlayH`. The -2 is OverlayStyle's 1+1 vertical padding.
func UndeliverableBodyHeight(overlayH int) int {
	return max(overlayH-2-undeliverableChrome, 1)
}

// UndeliverableScrollForCursor returns the scroll offset that keeps `cursor`
// visible in a viewport of `bodyHeight` rows, honouring the user's
// scrolloff config. Every row here is exactly one display line, so the
// displayLines callback is just entry count.
func UndeliverableScrollForCursor(scroll, cursor, bodyHeight, total int) int {
	identity := func(from, to int) int { return to - from }
	return VimScrollOff(scroll, cursor, total, bodyHeight, ConfigScrollOff, identity)
}

// UndeliverableClampScroll snaps a scroll offset back into range so the
// rendered window never runs past the end of a list that just shrank
// (filter typed, background refresh returned fewer rows).
func UndeliverableClampScroll(scroll, total, bodyHeight int) int {
	if total <= bodyHeight {
		return 0
	}
	return min(max(scroll, 0), total-bodyHeight)
}

// RenderUndeliverableOverlay produces the cluster-wide "stuck waiting"
// overlay, padded to exactly height-2 lines so the box stays a fixed size
// whatever the report holds. Caller wraps the result in
// BoxHeight(BoxWidth(OverlayStyle, width), height).Render(...).
//
// Every value on a row originates in the cluster - a reason quotes an Event
// message verbatim, and finalizer names are attacker-settable on a hostile
// CRD - so all four fields pass through SanitizeTerminalText before they are
// measured or truncated. Sanitizing after a Truncate would leave the layout
// off by the bytes the sanitizer removed.
func RenderUndeliverableOverlay(
	rows []UndeliverableRow,
	cursor, scroll int,
	width, height int,
	filter string,
	filterActive bool,
	loading bool,
	partialError string,
) string {
	innerW := max(width-4, 40) // OverlayStyle.Padding(1, 2) -> -4 horizontal
	innerH := max(height-2, 1)
	bodyHeight := UndeliverableBodyHeight(height)

	cfg := OverlayListConfig{
		Title:        "Undeliverable (cluster-wide)",
		Subtitle:     undeliverableSubtitle(len(rows), loading, partialError),
		Cursor:       cursor,
		Filter:       SanitizeTerminalText(filter),
		FilterActive: filterActive,
		Filterable:   true,
		Scroll:       UndeliverableClampScroll(scroll, len(rows), bodyHeight),
		MaxVisible:   bodyHeight,
		Height:       innerH,
		EmptyMessage: "Nothing is stuck waiting",
	}
	if loading {
		cfg.EmptyMessage = "Scanning cluster…"
		return RenderOverlayList(nil, cfg, innerW)
	}

	cfg.ShowDescription = true
	return RenderOverlayList(undeliverableItems(rows, innerW), cfg, innerW)
}

// undeliverableSubtitle carries the row count and, when the scan was only
// partly authorised, the reason. It rides on the list's subtitle line rather
// than a hand-drawn banner so the chrome height stays constant - a banner
// that appears and disappears would move every row under it.
func undeliverableSubtitle(n int, loading bool, partialError string) string {
	if loading {
		return "scanning…"
	}
	s := fmt.Sprintf("%d stuck", n)
	if partialError != "" {
		s += " - partial result: " + SanitizeTerminalText(partialError)
	}
	return s
}

// undeliverableItems lays the rows out as fixed-width KIND / NAMESPACE /
// NAME columns with the reason in the description column, so the reason
// stays readable at any terminal width while the identity columns line up.
func undeliverableItems(rows []UndeliverableRow, innerW int) []OverlayListItem {
	nsW := min(20, max(8, innerW/6))
	reasonW := max(20, innerW/3)
	// -2 for the gap RenderOverlayList inserts before the description.
	nameW := max(10, innerW-(undeliverableKindW+1+nsW+1+reasonW+2))

	out := make([]OverlayListItem, len(rows))
	for i, r := range rows {
		kind := padRight(Truncate(SanitizeTerminalText(r.Kind), undeliverableKindW), undeliverableKindW)
		ns := padRight(Truncate(SanitizeTerminalText(r.Namespace), nsW), nsW)
		name := padRight(Truncate(SanitizeTerminalText(r.Name), nameW), nameW)
		out[i] = OverlayListItem{
			Name:        kind + " " + ns + " " + name,
			Description: Truncate(SanitizeTerminalText(r.Reason), reasonW),
		}
	}
	return out
}

// UndeliverableOverlayWidth returns the outer overlay width for a terminal
// `termW` wide. Wide enough for the reason column to be worth reading,
// capped so the box never touches the terminal edge.
func UndeliverableOverlayWidth(termW int) int {
	return max(min(120, termW-10), OverlayListFloor)
}

// MatchesUndeliverableRow reports whether a row matches the overlay's filter
// query. Kind and reason are searchable alongside namespace and name so
// "FailedScheduling" or "Ingress" narrows the list the way users expect.
func MatchesUndeliverableRow(r UndeliverableRow, query string) bool {
	if query == "" {
		return true
	}
	q := strings.ToLower(query)
	return strings.Contains(strings.ToLower(r.Kind), q) ||
		strings.Contains(strings.ToLower(r.Namespace), q) ||
		strings.Contains(strings.ToLower(r.Name), q) ||
		strings.Contains(strings.ToLower(r.Reason), q)
}
