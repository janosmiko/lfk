package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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

// helpKeyDisplay formats a keybinding value for display in the help screen.
// It capitalizes "ctrl+" and "alt+" modifier prefixes for readability (e.g.
// "alt+ctrl+y" -> "Alt+Ctrl+Y"); other bindings display verbatim.
func helpKeyDisplay(key string) string {
	if !strings.HasPrefix(key, "ctrl+") && !strings.HasPrefix(key, "alt+") {
		return key
	}
	parts := strings.Split(key, "+")
	for i, p := range parts {
		switch p {
		case "ctrl":
			parts[i] = "Ctrl"
		case "alt":
			parts[i] = "Alt"
		default:
			parts[i] = strings.ToUpper(p)
		}
	}
	return strings.Join(parts, "+")
}

// helpSections lives in help_sections.go.

// BuildHelpLines builds the formatted help lines, optionally filtering
// by a query string. contextMode limits sections to those matching the
// current view (empty = explorer). Exported so the app layer can run
// the same line-building pipeline to compute search match indices for
// n/N navigation.
//
// Returns plain (un-styled) text in the same row order RenderHelpScreen
// will display. Plain text is what app-layer search routines need:
// running MatchLine / strings.Contains over a styled line lets a
// digit query match bytes that live inside an SGR escape (e.g. the
// "1" in "\x1b[33;1m"), inflating match counts and pointing n/N at
// rows with no visible match.
func BuildHelpLines(filter, contextMode string, screenWidth int) []string {
	specs := buildHelpSpecs(filter, contextMode, helpInnerWidth(screenWidth))
	out := make([]string, len(specs))
	for i, s := range specs {
		out[i] = helpSpecPlain(s)
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
	// key is the padded plain key column for entry rows.
	key string
	// desc is the plain description column for entry rows.
	desc string
}

// helpKeyColumnMinWidth is the minimum width of the key column. Sections
// whose widest key is shorter still pad to this so the description column
// has a comfortable left margin. Sections with longer keys widen the
// column to fit, keeping descriptions vertically aligned within the
// section.
const helpKeyColumnMinWidth = 14

// buildHelpSpecs walks the help sections and produces structural
// specs (un-styled) in the exact display order. Used by both
// BuildHelpLines and RenderHelpScreen so the plain match indices
// computed by the app layer line up 1:1 with the styled rows on
// screen.
func buildHelpSpecs(filter, contextMode string, innerW int) []helpLineSpec {
	sections := helpSections()
	specs := make([]helpLineSpec, 0, 64)
	for _, section := range sections {
		// Context filtering: when a context is active, show only sections
		// that match that context. When no context (explorer), show only
		// sections with empty context (explorer sections).
		if contextMode == "" || contextMode == "Navigation" || contextMode == "Bookmarks" {
			if section.context != "" {
				continue
			}
		} else {
			if section.context != contextMode {
				continue
			}
		}

		// First pass: collect bindings that pass the filter. Sizing the
		// key column to filtered content (rather than the full section)
		// keeps the column tight when a filter narrows the visible rows.
		matched := make([]helpEntry, 0, len(section.bindings))
		for _, b := range section.bindings {
			if filter != "" {
				if !MatchLine(b.key, filter) && !MatchLine(b.desc, filter) {
					continue
				}
			}
			matched = append(matched, b)
		}

		// Only include sections that have matching bindings.
		if len(matched) == 0 {
			continue
		}

		// Per-section column width: pad keys to the widest key in this
		// section so descriptions align vertically. The fixed-14 column
		// used previously broke alignment for sections containing long
		// keys like "Ctrl+F / Ctrl+B / PgDn / PgUp" — those overflowed
		// the column and shifted their descriptions right of the rest.
		keyWidth := helpKeyColumnMinWidth
		for _, b := range matched {
			if w := lipgloss.Width(b.key); w > keyWidth {
				keyWidth = w
			}
		}

		// Word-wrap each description to the width left after the key column
		// so long entries read in full instead of being truncated with a
		// "~". Each wrapped chunk becomes its own entry spec, preserving the
		// one-spec-per-rendered-row invariant the search/scroll machinery
		// relies on: continuation rows carry a blank (but same-width) key so
		// the wrapped text stays aligned under the original description.
		// rowOverhead is the fixed prefix helpSpecPlain prepends to an entry
		// row: 4 leading spaces + keyWidth + 2 spaces between key and desc.
		// descBudget is the exact room left for the description; flooring at
		// 1 (rather than a larger value) keeps the wrapped row within innerW
		// so the renderer's Truncate never lops a character with a "~" — the
		// only residual overflow is when a section's key column alone is
		// wider than the panel, which no description width could fix.
		rowOverhead := 4 + keyWidth + 2
		descBudget := max(innerW-rowOverhead, 1)
		blankKey := strings.Repeat(" ", keyWidth)
		entries := make([]helpLineSpec, 0, len(matched))
		for _, b := range matched {
			chunks := wrapHelpText(b.desc, descBudget)
			if len(chunks) == 0 {
				chunks = []string{""}
			}
			for ci, chunk := range chunks {
				key := blankKey
				if ci == 0 {
					key = fmt.Sprintf("%-*s", keyWidth, b.key)
				}
				entries = append(entries, helpLineSpec{
					kind: helpLineEntry,
					key:  key,
					desc: chunk,
				})
			}
		}

		if len(specs) > 0 {
			specs = append(specs, helpLineSpec{kind: helpLineBlank})
		}
		specs = append(specs, helpLineSpec{kind: helpLineSectionHeader, text: section.title})
		specs = append(specs, entries...)
	}

	if filter != "" && len(specs) == 0 {
		specs = append(specs, helpLineSpec{kind: helpLineMessage, text: "No matching keybindings"})
	}

	return specs
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
	innerPanel := innerPanelStyle.
		Width(contentW).
		Render(content)

	body := title + "\n" + innerPanel
	body = FillLinesBg(body, boxW-4, SurfaceBg) // -4 for OverlayStyle padding(1,2)

	return OverlayStyle.
		Width(boxW).
		Height(boxH).
		Render(body)
}
