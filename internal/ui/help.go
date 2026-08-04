package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// innerPanelStyle is used for the content panel inside the help overlay.
var innerPanelStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color(ColorBorder)).
	Padding(0, 1)

// helpEntry holds a single keybinding entry.
type helpEntry struct {
	key  string
	desc string
}

// helpSection groups keybindings under a section header.
// context identifies which view this section belongs to.
// Empty context means the explorer (main) view.
type helpSection struct {
	title    string
	context  string // e.g. "YAML View", "Log Viewer", "" for explorer
	bindings []helpEntry
}

// helpSections lives in help_sections.go; key formatting in help_keys.go.

// BuildHelpLines builds the searchable help lines, optionally filtering
// by a query string. contextMode limits sections to those matching the
// current view (empty = explorer). Exported so the app layer can run
// the same line-building pipeline to compute search match indices for
// n/N navigation.
//
// Returns plain (un-styled) text, one entry per row RenderHelpScreen
// will display, in the same order. Plain text is what app-layer search
// routines need: running MatchLine / strings.Contains over a styled line
// lets a digit query match bytes that live inside an SGR escape (e.g.
// the "1" in "\x1b[33;1m"), inflating match counts and pointing n/N at
// rows with no visible match.
//
// The key column carries the TEXTUAL chord ("Ctrl+D") even when the
// screen draws the symbol ("⌃D"), so a search for "ctrl" still finds the
// row. RenderHelpScreen highlights the whole key cell for those rows.
func BuildHelpLines(filter, contextMode string, screenWidth int) []string {
	specs := buildHelpSpecs(filter, contextMode, helpInnerWidth(screenWidth))
	out := make([]string, len(specs))
	for i, s := range specs {
		out[i] = helpSpecSearchText(s)
	}
	return out
}

// helpInnerWidth returns the content width (in cells) available for a
// single help row at the given screen width. Mirrors the boxW / contentW
// arithmetic in RenderHelpScreen so buildHelpSpecs wraps descriptions to
// the exact width the renderer will display — keeping the wrapped row set
// (and therefore the search-match indices) identical on both paths.
func helpInnerWidth(screenWidth int) int {
	boxW := max(screenWidth*70/100, 50)
	contentW := boxW - 6 // border + padding
	return max(contentW-2, 10)
}

// HelpVisibleLines returns the number of help-content rows that fit
// inside the overlay box for a given screen height. Mirrors the same
// boxH / maxLines / visibleLines arithmetic RenderHelpScreen uses, so
// callers (clamp helpers, scroll-to-match positioning) compute the
// same maxScroll the renderer enforces.
func HelpVisibleLines(screenHeight int) int {
	boxH := max(screenHeight*80/100, 20)
	maxLines := max(boxH-6, 5)
	visibleLines := max(maxLines-2, 1)
	return visibleLines
}

// helpLineKind labels each logical row in the help screen so the
// renderer can pick the correct outer style for that row's plain text.
type helpLineKind int

const (
	helpLineBlank helpLineKind = iota
	helpLineSectionHeader
	helpLineEntry
	helpLineMessage
)

// helpLineSpec is the structural form of a help row, kept un-styled
// so the renderer can splice the search highlight into plain text
// before applying the outer styles. The pre-style highlight path
// avoids the bug where a "/" search query containing digits matched
// bytes inside an SGR escape sequence on the already-styled line —
// terminals rendered the leftover sequence fragments as literal
// "[33;" / ";1m" text on screen.
type helpLineSpec struct {
	kind helpLineKind
	// text is the plain content for header and message rows.
	text string
	// key is the right-aligned key column as drawn (symbols when the
	// icon mode allows them).
	key string
	// keyText is the unpadded textual form of the same binding
	// ("Ctrl+D" for a key column drawing "⌃D"). Search indexes this so a
	// "ctrl" query still matches a symbol-rendered chord.
	keyText string
	// desc is the plain description column for entry rows.
	desc string
}

// helpKeyColumnMinWidth is the minimum width of the key column, so the
// description column keeps a comfortable left margin even when every
// visible key is a single character.
const helpKeyColumnMinWidth = 14

// helpKeyRow is one catalog entry resolved to its display and search forms.
type helpKeyRow struct {
	key     string // as drawn (symbols when available)
	keyText string // textual chord, for the search index
	desc    string
}

// helpGroup is a section that survived context and filter matching.
type helpGroup struct {
	title string
	rows  []helpKeyRow
}

// buildHelpSpecs walks the help sections and produces structural
// specs (un-styled) in the exact display order. Used by both
// BuildHelpLines and RenderHelpScreen so the plain match indices
// computed by the app layer line up 1:1 with the styled rows on
// screen.
func buildHelpSpecs(filter, contextMode string, innerW int) []helpLineSpec {
	groups := collectHelpGroups(filter, contextMode)
	if len(groups) == 0 {
		if filter != "" {
			return []helpLineSpec{{kind: helpLineMessage, text: "No matching keybindings"}}
		}
		return nil
	}

	keyWidth := helpKeyColumnWidth(groups, innerW)
	// rowOverhead is the fixed prefix helpSpecPlain puts before a
	// description: 4 leading spaces + keyWidth + 2 spaces between the
	// columns. descBudget is the exact room left; flooring at 1 keeps the
	// wrapped row inside innerW so the renderer's Truncate never lops a
	// character with a "~". The only residual overflow is a key wider than
	// the capped column, which no description width could fix.
	rowOverhead := 4 + keyWidth + 2
	descBudget := max(innerW-rowOverhead, 1)
	blankKey := strings.Repeat(" ", keyWidth)

	specs := make([]helpLineSpec, 0, 64)
	for _, g := range groups {
		if len(specs) > 0 {
			specs = append(specs, helpLineSpec{kind: helpLineBlank})
		}
		specs = append(specs, helpLineSpec{kind: helpLineSectionHeader, text: g.title})
		specs = append(specs, helpEntrySpecs(g.rows, keyWidth, blankKey, descBudget)...)
	}
	return specs
}

// helpEntrySpecs turns one section's rows into rendered-row specs.
//
// Word-wrap keeps long descriptions readable instead of truncating them
// with a "~". Each wrapped chunk becomes its own spec, preserving the
// one-spec-per-rendered-row invariant the search/scroll machinery relies
// on: continuation rows carry a blank (but same-width) key so the wrapped
// text stays aligned under the original description.
func helpEntrySpecs(rows []helpKeyRow, keyWidth int, blankKey string, descBudget int) []helpLineSpec {
	out := make([]helpLineSpec, 0, len(rows))
	for _, r := range rows {
		chunks := wrapHelpText(r.desc, descBudget)
		if len(chunks) == 0 {
			chunks = []string{""}
		}
		for ci, chunk := range chunks {
			spec := helpLineSpec{kind: helpLineEntry, key: blankKey, desc: chunk}
			if ci == 0 {
				spec.key = padKeyLeft(r.key, keyWidth)
				spec.keyText = r.keyText
			}
			out = append(out, spec)
		}
	}
	return out
}

// collectHelpGroups applies context and filter matching and resolves each
// surviving entry's key to its display and search forms. Split from
// buildHelpSpecs so the key column can be sized across every visible
// section before any row is laid out.
func collectHelpGroups(filter, contextMode string) []helpGroup {
	groups := make([]helpGroup, 0, 16)
	for _, section := range helpSections() {
		if !helpSectionInContext(section, contextMode) {
			continue
		}
		rows := make([]helpKeyRow, 0, len(section.bindings))
		for _, b := range section.bindings {
			row := helpKeyRow{
				key:     helpKeySymbols(b.key),
				keyText: helpKeyDisplay(b.key),
				desc:    b.desc,
			}
			// Match the textual chord as well as the drawn one so an "f"
			// filter for "ctrl" narrows to the rows drawn as "⌃D".
			if filter != "" &&
				!MatchLine(row.keyText, filter) &&
				!MatchLine(row.key, filter) &&
				!MatchLine(row.desc, filter) {
				continue
			}
			rows = append(rows, row)
		}
		if len(rows) == 0 {
			continue
		}
		groups = append(groups, helpGroup{title: section.title, rows: rows})
	}
	return groups
}

// helpSectionInContext reports whether a section belongs to the view the
// help screen was opened from. An empty (or explorer-level) context shows
// only the explorer sections; any other context shows only its own.
func helpSectionInContext(section helpSection, contextMode string) bool {
	if contextMode == "" || contextMode == "Navigation" || contextMode == "Bookmarks" {
		return section.context == ""
	}
	return section.context == contextMode
}

// helpKeyColumnWidth sizes the single key column shared by every visible
// section, so all descriptions start at one vertical line rather than
// stepping in and out per section.
//
// The cap stops one unusually long key from pushing every description off
// a narrow screen; a key past the cap simply overflows its own cell.
func helpKeyColumnWidth(groups []helpGroup, innerW int) int {
	maxWidth := max(innerW/3, helpKeyColumnMinWidth)
	width := helpKeyColumnMinWidth
	for _, g := range groups {
		for _, r := range g.rows {
			if w := lipgloss.Width(r.key); w > width {
				width = w
			}
		}
	}
	return min(width, maxWidth)
}

// wrapHelpText word-wraps desc to width on word boundaries. A space-free
// token wider than width (e.g. a slash-joined "owner/port-forward/orphan/
// finding/mark" enumeration) is broken after its "/" separators so it
// wraps at readable boundaries rather than mid-word; a segment between
// slashes that is itself wider than width falls back to a hard character
// break. The output never exceeds width, so RenderHelpScreen's final
// Truncate never adds a "~".
func wrapHelpText(desc string, width int) []string {
	if width < 1 {
		width = 1
	}
	if strings.TrimSpace(desc) == "" {
		return nil
	}

	// Tokenize into pieces no wider than width. spaceBefore marks pieces
	// that follow a real space (a word boundary); slash continuations and
	// hard-break fragments glue to the previous piece with no space.
	type piece struct {
		text        string
		spaceBefore bool
	}
	var pieces []piece
	for word := range strings.FieldsSeq(desc) {
		first := true
		for _, seg := range splitAfterSlash(word) {
			for _, chunk := range hardChunks(seg, width) {
				pieces = append(pieces, piece{text: chunk, spaceBefore: first})
				first = false
			}
		}
	}

	// Greedy pack: keep adding pieces to the current line while they fit.
	var lines []string
	cur := ""
	for _, p := range pieces {
		if cur == "" {
			cur = p.text
			continue
		}
		sep := ""
		if p.spaceBefore {
			sep = " "
		}
		if lipgloss.Width(cur)+lipgloss.Width(sep)+lipgloss.Width(p.text) <= width {
			cur += sep + p.text
			continue
		}
		lines = append(lines, cur)
		cur = p.text
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

// splitAfterSlash splits s into segments that each end just after a "/"
// (the slash stays on the left segment), so "owner/port-forward/orphan"
// becomes ["owner/", "port-forward/", "orphan"]. Used to give long
// slash-joined tokens readable break points.
func splitAfterSlash(s string) []string {
	if !strings.Contains(s, "/") {
		return []string{s}
	}
	runes := []rune(s)
	var out []string
	start := 0
	for i, r := range runes {
		if r == '/' {
			out = append(out, string(runes[start:i+1]))
			start = i + 1
		}
	}
	if start < len(runes) {
		out = append(out, string(runes[start:]))
	}
	return out
}

// hardChunks splits s into pieces no wider than width by raw rune count,
// used only as the last resort for a segment with no break opportunity.
func hardChunks(s string, width int) []string {
	if lipgloss.Width(s) <= width {
		return []string{s}
	}
	runes := []rune(s)
	var out []string
	for len(runes) > 0 {
		n := min(width, len(runes))
		out = append(out, string(runes[:n]))
		runes = runes[n:]
	}
	return out
}

// helpSpecPlain returns the un-styled visible form of a help-line
// spec. The plain form is what the app-layer match counter sees, so
// substring/regex/fuzzy queries match the same characters the user
// reads on screen — never bytes hidden inside an ANSI SGR sequence.
func helpSpecPlain(s helpLineSpec) string {
	switch s.kind {
	case helpLineBlank:
		return ""
	case helpLineSectionHeader:
		return "  " + s.text
	case helpLineEntry:
		return "    " + s.key + "  " + s.desc
	case helpLineMessage:
		return "  " + s.text
	}
	return ""
}

// helpSpecSearchText returns the row's searchable form. Identical to
// helpSpecPlain except that the key column carries the textual chord, so a
// "ctrl" query matches a row whose key column draws "⌃D". Row order and
// count are the same on both paths, keeping the app layer's n/N match
// indices aligned with the rendered rows.
func helpSpecSearchText(s helpLineSpec) string {
	if s.kind != helpLineEntry || s.keyText == "" {
		return helpSpecPlain(s)
	}
	return "    " + padKeyLeft(s.keyText, lipgloss.Width(s.key)) + "  " + s.desc
}

// helpKeyMatchesSearch reports whether the search query hits a row's key
// via its textual form only. Those rows get their whole key cell
// highlighted, because the drawn symbol shares no characters with the
// query and an inline splice would show nothing.
func helpKeyMatchesSearch(s helpLineSpec, search string) bool {
	return search != "" && s.keyText != "" &&
		!MatchLine(s.key, search) && MatchLine(s.keyText, search)
}

// helpSpecStyled renders a help-line spec to its final styled form.
// When search is non-empty the inline highlight is applied to plain
// key/desc/text first via HighlightMatchStyledOver, then wrapped with
// the segment's outer style via RenderOverPrestyled. Highlighting on
// plain segments keeps the match-finder away from any ANSI bytes —
// fixing the "/ search of a digit prints raw [33;1m" report.
//
// isCurrent flips the row's highlight to SelectedSearchHighlightStyle
// so n/N navigation can mark the active match distinctly.
func helpSpecStyled(s helpLineSpec, search string, isCurrent bool) string {
	hl := SearchHighlightStyle
	if isCurrent {
		hl = SelectedSearchHighlightStyle
	}
	switch s.kind {
	case helpLineBlank:
		return ""
	case helpLineSectionHeader:
		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorPrimary)).Underline(true).Background(SurfaceBg)
		inner := HighlightMatchStyledOver(s.text, search, hl, headerStyle)
		return "  " + RenderOverPrestyled(inner, headerStyle)
	case helpLineEntry:
		keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Bold(true).Background(SurfaceBg)
		descStyle := OverlayDimStyle
		keyInner := HighlightMatchStyledOver(s.key, search, hl, keyStyle)
		if helpKeyMatchesSearch(s, search) {
			// Query matched "Ctrl+D" but the cell draws "⌃D" — highlight
			// the whole chord so the hit is visible where the user looks.
			keyInner = padKeyLeft(hl.Render(strings.TrimLeft(s.key, " ")), lipgloss.Width(s.key))
		}
		descInner := HighlightMatchStyledOver(s.desc, search, hl, descStyle)
		return "    " + RenderOverPrestyled(keyInner, keyStyle) + "  " + RenderOverPrestyled(descInner, descStyle)
	case helpLineMessage:
		inner := HighlightMatchStyledOver(s.text, search, hl, OverlayDimStyle)
		return "  " + RenderOverPrestyled(inner, OverlayDimStyle)
	}
	return ""
}

// RenderHelpScreen renders a full help overlay with all keybindings.
// filter narrows the visible lines (f key). search highlights matches
// in the visible lines without removing them (/ key). currentMatchLine
// is the index (in the post-filter line list) of the line under the
// n/N navigation cursor — that line gets a distinct "selected match"
// style so the user can see which match is current. Pass -1 when
// there's no active navigation. contextMode limits sections to the
// current view (empty = explorer).
func RenderHelpScreen(screenWidth, screenHeight, scroll int, filter, search, contextMode string, currentMatchLine int) string {
	boxW := max(screenWidth*70/100, 50)
	// Mirror HelpVisibleLines so outer height stays in sync with the
	// inner row budget — lipgloss pads short content to this height,
	// stopping the box from shrinking when filter narrows results or
	// from growing when long lines wrap.
	boxH := max(screenHeight*80/100, 20)

	contentW := boxW - 6 // account for border + padding

	title := OverlayTitleStyle.Render("Keybindings")

	// Build structural specs once, then render each row with the
	// search highlight pre-spliced into the plain segments before the
	// outer style is applied. Highlighting on plain text keeps the
	// match-finder away from ANSI escape bytes — the previous
	// "highlight on already-styled, already-truncated line" path could
	// match a digit query inside an SGR like \x1b[33;1m, which broke
	// the sequence and printed "[33;" / ";1m" as visible text.
	innerW := helpInnerWidth(screenWidth)
	specs := buildHelpSpecs(filter, contextMode, innerW)
	// buildHelpSpecs already word-wrapped descriptions to innerW, so each
	// spec is one rendered row. Truncate stays as a safety net for the
	// rare row whose key column alone overruns innerW (extremely narrow
	// terminals); it never trims wrapped descriptions in normal layouts.
	lines := make([]string, len(specs))
	for i, s := range specs {
		lines[i] = Truncate(helpSpecStyled(s, search, i == currentMatchLine), innerW)
	}
	totalLines := len(lines)

	// Calculate visible area via shared helper so app-layer clamps see
	// the same maxScroll the renderer enforces.
	visibleLines := HelpVisibleLines(screenHeight)

	// Clamp scroll.
	maxScroll := max(totalLines-visibleLines, 0)
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}

	// Determine scroll indicators.
	hasAbove := scroll > 0
	hasBelow := scroll+visibleLines < totalLines

	// Slice visible portion.
	end := min(scroll+visibleLines, totalLines)
	visible := lines[scroll:end]

	// Pad the visible window to exactly visibleLines rows so a filter
	// that narrows results doesn't shrink the box. Without this the
	// outer overlay box collapses to fit the short content and the user
	// sees the window resize on every keystroke.
	for len(visible) < visibleLines {
		visible = append(visible, "")
	}

	// Build final lines with indicators.
	var displayLines []string
	// Always include indicator lines (empty when not scrollable) to keep height stable.
	if hasAbove {
		displayLines = append(displayLines, OverlayDimStyle.Render("  \u2191 more above"))
	} else {
		displayLines = append(displayLines, "")
	}
	displayLines = append(displayLines, visible...)
	if hasBelow {
		displayLines = append(displayLines, OverlayDimStyle.Render("  \u2193 more below"))
	} else {
		displayLines = append(displayLines, "")
	}

	content := strings.Join(displayLines, "\n")
	content = FillLinesBg(content, contentW-2, SurfaceBg) // -2 for innerPanelStyle padding
	innerPanel := BoxWidth(innerPanelStyle, contentW).
		Render(content)

	body := title + "\n" + innerPanel
	body = FillLinesBg(body, boxW-4, SurfaceBg) // -4 for OverlayStyle padding(1,2)

	return BoxHeight(BoxWidth(OverlayStyle, boxW), boxH).
		Render(body)
}
