package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// WhoCanRow is the renderer-facing representation of a single result
// row in the Who-Can table. Mirrors k8s.WhoCanSubject but kept here so
// the ui package doesn't import the k8s package.
type WhoCanRow struct {
	Kind      string // "User" / "Group" / "ServiceAccount"
	Name      string
	Namespace string // ServiceAccount namespace; empty for User/Group
	Via       string // "ClusterRoleBinding/foo → ClusterRole/bar"
}

// WhoCanVerbs is the canonical verb chip list rendered above the
// subjects table. The trailing "*" matches any verb (rule.Verbs == ["*"]
// in RBAC). Order is `kubectl who-can`'s order so muscle memory carries.
var WhoCanVerbs = []string{"get", "list", "watch", "create", "update", "patch", "delete", "*"}

// WhoCanViewParams bundles every field renderWhoCanOverlay needs to
// pass through. Wrapping them in a struct keeps RenderWhoCanView's
// signature legible as the picker grows (filter, scroll, dual-pane
// state) and lets the caller's argument list stay readable.
type WhoCanViewParams struct {
	VerbCursor     int
	Resources      []string // visible (post-filter) resource list
	ResourceCursor int      // index into Resources (the visible slice)
	ResourceScroll int      // first visible row of Resources (stateful — handlers maintain this so scroll-up doesn't pin the cursor to the last visible row)
	NamespaceLabel string   // "ns: foo" or "all-namespaces"
	Subjects       []WhoCanRow
	SubjectsScroll int
	Loading        bool
	// FooterBar is rendered as the bottom row inside the overlay,
	// below the columns. The caller builds it (the filter input lives
	// here so its position matches Can-I's). Empty string = blank row.
	FooterBar     string
	Width, Height int
}

// RenderWhoCanView paints the reverse-RBAC overlay's inner content:
// a single header row (title + verb chips at left, namespace label
// flushed to the far right), then the 2-column body with the resource
// picker on the left and the subjects table on the right.
//
// One header row keeps the table tall (every line above the columns
// is a line the user can't see subjects in). Both columns sit inside
// ActiveColumnStyle/InactiveColumnStyle boxes (same baseBg as the
// Can-I view), so flipping between modes with Tab keeps the same
// visible "shape" — only the right pane swaps content.
func RenderWhoCanView(p WhoCanViewParams) string {
	headerRow := renderWhoCanHeaderRow(p.VerbCursor, p.NamespaceLabel, p.Width)

	// Two columns: resources picker (left, 20%), subjects (right, rest).
	// 20% matches caniview's leftW — resource names rarely exceed 20 cols
	// so a wider picker just steals space from the subjects table where
	// long Via paths actually need it.
	usable := max(p.Width-4, 20)
	leftW := max(10, usable*20/100)
	middleW := max(10, usable-leftW)
	// contentHeight matches caniview's: -1 header, -2 column borders,
	// -1 reserved bottom row. The reserved row keeps Tab between modes
	// from making the table jump up by one (caniview uses that row for
	// its hint bar).
	contentHeight := max(p.Height-4, 3)
	colPad := 2
	leftInner := max(8, leftW-colPad)
	middleInner := max(10, middleW-colPad)

	leftContent := renderWhoCanResourcePicker(
		p.Resources, p.ResourceCursor, p.ResourceScroll,
		leftInner, contentHeight,
	)
	leftContent = PadToHeight(leftContent, contentHeight)
	left := ActiveColumnStyle.Width(leftW).Height(contentHeight).MaxHeight(contentHeight + 2).Render(leftContent)

	rightContent := renderWhoCanSubjects(
		p.Subjects, p.SubjectsScroll, p.Loading,
		currentResource(p.Resources, p.ResourceCursor),
		middleInner, contentHeight,
	)
	rightContent = PadToHeight(rightContent, contentHeight)
	right := InactiveColumnStyle.Width(middleW).Height(contentHeight).MaxHeight(contentHeight + 2).Render(rightContent)

	columns := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	// Footer row matches caniview: same vertical position, same role
	// (filter input lives there). Empty string when no footer content.
	return lipgloss.JoinVertical(lipgloss.Left, headerRow, columns, p.FooterBar)
}

// renderWhoCanHeaderRow assembles the single header row that sits
// above the column table. Layout:
//
//	[title] [verb chips] ............................. [namespace]
//
// joinTitleAndRightLabel handles the right-edge alignment so this row
// reads consistently with the Can-I title row when the user pivots
// with Tab.
func renderWhoCanHeaderRow(verbCursor int, namespaceLabel string, width int) string {
	title := TitleStyle.Render("RBAC Explorer: Who-Can?")
	verbs := renderWhoCanVerbs(verbCursor)
	leftBlock := title + BarNormalStyle.Render(" ") + verbs
	if lipgloss.Width(leftBlock)+1+lipgloss.Width(namespaceLabel) > width {
		// Title doesn't fit alongside the chips + ns — shorten it.
		title = TitleStyle.Render("Who-Can?")
		leftBlock = title + BarNormalStyle.Render(" ") + verbs
	}
	return joinTitleAndRightLabel(leftBlock, BarDimStyle.Render(namespaceLabel), width)
}

// currentResource returns the resource the cursor is on, or "" when
// the visible list is empty (filter matches nothing). Callers use the
// empty string as the "no resource selected" sentinel — the subjects
// panel renders an instructional placeholder in that case.
func currentResource(resources []string, cursor int) string {
	if cursor < 0 || cursor >= len(resources) {
		return ""
	}
	return resources[cursor]
}

// renderWhoCanVerbs builds the verb-chip row. Each chip is a label
// padded with a space; the cursor chip uses OverlaySelectedStyle so
// the highlight covers it cleanly across themes.
func renderWhoCanVerbs(cursor int) string {
	var b strings.Builder
	b.WriteString(BarDimStyle.Render("Verb: "))
	for i, v := range WhoCanVerbs {
		chip := " " + v + " "
		if i == cursor {
			b.WriteString(OverlaySelectedStyle.Render(chip))
		} else {
			b.WriteString(BarNormalStyle.Render(chip))
		}
		if i < len(WhoCanVerbs)-1 {
			b.WriteString(BarDimStyle.Render(" "))
		}
	}
	return b.String()
}

// renderWhoCanResourcePicker paints the left-column list. Header line
// shows either the active filter input or the resource count + "/" hint
// so the user knows how to narrow the list. Body is a vertical list of
// resource names with the cursor row highlighted.
//
// Every styled span uses a Bar*Style (which has baseBg) — the column
// box wraps in baseBg and any fg-only style here would punch through to
// the terminal default bg between styled spans, producing a "swap" band
// the eye picks up immediately.
//
// Scroll is taken as input (not derived from cursor) so vim-like
// behavior holds: the viewport stays put when the cursor moves inside
// it, and only scrolls when the cursor leaves an edge. The handlers
// maintain the scroll offset; this function only clamps to a valid
// range and renders.
func renderWhoCanResourcePicker(resources []string, cursor, scroll, width, height int) string {
	header := renderWhoCanResourceHeader(len(resources), width)

	if len(resources) == 0 {
		return header + "\n" + BarDimStyle.Render("  No matches")
	}

	bodyHeight := max(height-1, 1) // -1 for header
	scroll = whoCanClampScroll(scroll, len(resources), bodyHeight)
	end := min(scroll+bodyHeight, len(resources))

	lines := make([]string, 0, end-scroll)
	for i := scroll; i < end; i++ {
		name := resources[i]
		if len(name) > width-2 {
			name = name[:max(width-3, 1)] + "…"
		}
		if i == cursor {
			line := fmt.Sprintf("> %-*s", width-2, name)
			lines = append(lines, OverlaySelectedStyle.Render(line))
		} else {
			line := fmt.Sprintf("  %s", name)
			lines = append(lines, BarNormalStyle.Render(line))
		}
	}
	return header + "\n" + strings.Join(lines, "\n")
}

// renderWhoCanResourceHeader paints the picker's column header. Just
// "Resources (N)" — the filter input lives in the overlay's footer
// row (matching Can-I's filter location) so this stays clean.
func renderWhoCanResourceHeader(count, width int) string {
	return whoCanFitPlaceholder(
		BarDimStyle.Bold(true).Render(fmt.Sprintf("  Resources (%d)", count)),
		width,
	)
}

// whoCanClampScroll snaps the requested scroll offset to a valid range
// for the given list size and viewport. Doesn't try to keep the cursor
// in view — handlers do that — only protects against stale offsets
// that would otherwise show blank space past the end of the list.
func whoCanClampScroll(scroll, total, bodyHeight int) int {
	if total <= bodyHeight {
		return 0
	}
	maxScroll := total - bodyHeight
	scroll = max(scroll, 0)
	scroll = min(scroll, maxScroll)
	return scroll
}

// WhoCanScrollForCursor returns the new scroll offset that keeps
// `cursor` visible inside a viewport of `bodyHeight` rows starting at
// `scroll`. Vim semantics: do nothing if the cursor is already in
// view; otherwise scroll just enough to put the cursor on the nearest
// visible edge. Used by handlers that move the cursor.
func WhoCanScrollForCursor(scroll, cursor, bodyHeight, total int) int {
	if total <= bodyHeight {
		return 0
	}
	if cursor < scroll {
		return cursor // cursor moved above the viewport — pull it down
	}
	if cursor >= scroll+bodyHeight {
		return cursor - bodyHeight + 1 // cursor moved below — pull viewport down
	}
	return scroll // cursor still in view — viewport stays put
}

// renderWhoCanSubjects paints the right-column subjects table. The
// queried resource is already echoed in the header row's verb chip
// + picker cursor, so we go straight to the column header — no
// "Subjects for X" preamble eating a row.
//
// Every styled span uses BarDimStyle (baseBg) so spans don't punch
// through to terminal default bg between the inactive column's baseBg
// padding — same fix as the resource picker.
//
// Placeholders are width-truncated so they never wrap inside the
// column box. Wrapped placeholder lines would push the column box's
// content past contentHeight, which lipgloss handles by dropping the
// bottom border — visible to the user as "the last line is out of
// the viewport".
func renderWhoCanSubjects(rows []WhoCanRow, scroll int, loading bool, resource string, width, height int) string {
	switch {
	case resource == "":
		return BarDimStyle.Render(whoCanFitPlaceholder("  Pick a resource on the left", width))
	case loading:
		return BarDimStyle.Render(whoCanFitPlaceholder("  Loading…", width))
	case len(rows) == 0:
		return BarDimStyle.Render(whoCanFitPlaceholder("  No subject has this permission", width))
	}

	// Column widths shrink with `width` so the row fits inside narrow
	// overlays. Kind/Namespace are short; SUBJECT and VIA both can be
	// long (full SA paths, full RBAC chains) so the leftover width is
	// split evenly between them — no upper cap on SUBJECT, otherwise
	// long ServiceAccount paths get truncated unnecessarily even when
	// there's room to show them.
	kindW := min(14, max(6, width/6))
	nsW := min(16, max(8, width/5))
	remaining := max(width-(kindW+nsW+8), 0)
	nameW := max(10, remaining/2)
	viaW := max(8, remaining-nameW)

	// Truncate header labels too — at narrow widths "NAMESPACE" alone
	// overflows nsW and pushes the row past the column's inner area.
	colHeader := fmt.Sprintf("  %-*s  %-*s  %-*s  %-*s",
		nameW, whoCanTruncate("SUBJECT", nameW),
		kindW, whoCanTruncate("KIND", kindW),
		nsW, whoCanTruncate("NAMESPACE", nsW),
		viaW, whoCanTruncate("VIA", viaW))
	colHeaderLine := BarDimStyle.Bold(true).Render(colHeader)

	bodyHeight := max(height-1, 1) // -1 for column header
	scroll = max(scroll, 0)
	if scroll > len(rows)-bodyHeight {
		scroll = max(len(rows)-bodyHeight, 0)
	}
	end := min(scroll+bodyHeight, len(rows))

	body := make([]string, 0, end-scroll)
	for _, r := range rows[scroll:end] {
		body = append(body, renderWhoCanRow(r, nameW, kindW, nsW, viaW))
	}
	return colHeaderLine + "\n" + strings.Join(body, "\n")
}

// renderWhoCanRow formats a single subject line. Every cell AND every
// separator/padding span is rendered through a baseBg-bound style;
// bare spaces between styled spans would punch through to terminal
// default bg and produce visible "swap" stripes between cells.
//
// Kind/Namespace/Via use BarNormalStyle (not BarDimStyle) so secondary
// columns stay readable instead of fading into the background.
func renderWhoCanRow(r WhoCanRow, nameW, kindW, nsW, viaW int) string {
	ns := r.Namespace
	if ns == "" {
		ns = "—"
	}
	sep := BarNormalStyle.Render("  ")
	nameStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorPrimary)).
		Background(BaseBg).
		Bold(true)
	nameCell := whoCanPadCellStyled(whoCanTruncate(r.Name, nameW), nameW, nameStyle)
	kindCell := whoCanPadCellStyled(whoCanTruncate(r.Kind, kindW), kindW, BarNormalStyle)
	nsCell := whoCanPadCellStyled(whoCanTruncate(ns, nsW), nsW, BarNormalStyle)
	viaCell := whoCanPadCellStyled(whoCanTruncate(r.Via, viaW), viaW, BarNormalStyle)
	return sep + nameCell + sep + kindCell + sep + nsCell + sep + viaCell
}

// whoCanPadCellStyled renders text in `style` and right-pads to width
// using the SAME style for the gap spaces. Unlike whoCanPadRight (bare
// padding), this guarantees bg coverage all the way to the column's
// right edge — important for subject rows where kindW/nsW shrink at
// narrow widths and the gap could otherwise show terminal default bg.
func whoCanPadCellStyled(text string, width int, style lipgloss.Style) string {
	rendered := style.Render(text)
	gap := width - lipgloss.Width(rendered)
	if gap <= 0 {
		return rendered
	}
	return rendered + style.Render(strings.Repeat(" ", gap))
}

// whoCanFitPlaceholder shortens an ANSI-styled string so it stays on a
// single visual line inside the column. Wrapping would push the
// column past contentHeight and lipgloss would drop the bottom border
// — the "last line out of viewport" the user reported. Uses
// ansi.Truncate so styled spans remain valid after truncation.
func whoCanFitPlaceholder(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	return ansi.Truncate(s, width, "")
}

// whoCanTruncate trims a plain (non-ANSI) string to maxW columns,
// appending "…" when cut. Kept private to this file under a unique
// name so it doesn't collide with the existing padRight in
// explorer_format.go (which is ANSI-aware in different ways).
func whoCanTruncate(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxW {
		return s
	}
	if maxW <= 1 {
		return "…"
	}
	return string(runes[:maxW-1]) + "…"
}
