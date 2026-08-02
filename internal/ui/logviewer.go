package ui

import (
	"fmt"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// LogSearchHighlightStyle highlights matched substrings in log lines.
var LogSearchHighlightStyle = lipgloss.NewStyle().
	Background(lipgloss.Color(ColorWarning)).
	Foreground(lipgloss.Color(ColorBase)).
	Bold(true)

// RenderLogViewer renders the full-screen log viewer.
//
// omitFooter, when true, suppresses the internal hotkey hint bar so the caller
// can render a full-terminal-width footer below a JoinHorizontal'd composition
// (e.g. with the side preview pane). The output is one row shorter than the
// default rendering. Callers that pass omitFooter=true must render their own
// footer via RenderLogFooter.
func RenderLogViewer(lines []string, scroll, width, height int, follow, wrap, lineNumbers, timestamps, previous, hidePrefixes bool, title, searchQuery, searchInput string, searchActive, canSwitchPod, canFilterContainers, hasMoreHistory, loadingHistory bool, statusMsg string, statusIsErr bool, cursor int, visualMode bool, visualStart int, visualType rune, visualCol, visualCurCol, wrapTopSkip int, omitFooter bool, filterActive bool, filterInput, filterQuery string, sevThreshold int) string {
	titleBar := renderLogTitleBar(title, lines, width, follow, wrap, lineNumbers, timestamps, previous, hidePrefixes, visualMode, visualType, loadingHistory, searchQuery, filterQuery, sevThreshold)
	var footer string
	if !omitFooter {
		footer = RenderLogFooter(width, statusMsg, statusIsErr, searchActive, searchInput, searchQuery, visualMode, canSwitchPod, canFilterContainers, filterActive, filterInput)
	}

	// Content area: subtract border top + bottom (2 lines).
	contentHeight := max(height-2, 1)

	// Content width accounting for border (2) + padding (2).
	contentWidth := max(width-4, 10)

	// Calculate line number width if needed.
	lineNumWidth := 0
	if lineNumbers && len(lines) > 0 {
		lineNumWidth = len(fmt.Sprintf("%d", len(lines))) + 1 // +1 for separator space
	}

	// Clamp scroll.
	if scroll < 0 {
		scroll = 0
	}

	// Strip timestamps and/or pod prefixes from visible lines.
	displayLines := lines
	if (!timestamps || hidePrefixes) && len(lines) > 0 {
		end := min(scroll+contentHeight, len(lines))
		displayLines = make([]string, len(lines))
		copy(displayLines, lines)
		for i := scroll; i < end; i++ {
			if !timestamps {
				displayLines[i] = StripTimestamp(lines[i])
			}
			if hidePrefixes {
				displayLines[i] = StripPodPrefix(displayLines[i])
			}
		}
	}

	// Sanitize visible lines: replace non-printable characters (except common
	// whitespace) that can break terminal width calculations and corrupt the
	// layout. ANSI SGR sequences are preserved when ConfigLogRenderAnsi is on
	// so colour output from log producers renders correctly.
	{
		end := min(scroll+contentHeight, len(displayLines))
		for i := scroll; i < end; i++ {
			displayLines[i] = sanitizeLogLine(displayLines[i], ConfigLogRenderAnsi)
		}
	}

	// Compute visual selection range.
	selStart, selEnd := -1, -1
	if visualMode {
		selStart = min(visualStart, cursor)
		selEnd = max(visualStart, cursor)
	}

	// Build visible lines, handling wrapping.
	var rendered []string
	var cursorRow, cursorCol int
	if wrap {
		rendered, cursorRow, cursorCol = renderWrappedLines(displayLines, scroll, contentHeight, contentWidth, lineNumbers, lineNumWidth, cursor, selStart, selEnd, visualStart, visualType, visualCol, visualCurCol, wrapTopSkip)
	} else {
		rendered, cursorRow, cursorCol = renderPlainLines(displayLines, scroll, contentHeight, contentWidth, lineNumbers, lineNumWidth, cursor, selStart, selEnd, visualStart, visualType, visualCol, visualCurCol)
	}

	// Highlight search matches in rendered lines.
	if searchQuery != "" {
		rendered = highlightSearchMatches(rendered, searchQuery, cursorRow, cursorCol)
	}

	// Pad to fill content height. Cap at contentHeight too: if the line
	// renderers ever return more rows than the body can hold (a wrap
	// underestimate, a stray newline embedded in a producer log line, etc.)
	// the surplus rows would push the bottom border off the visible area.
	// User-reported on dragonfly-operator logs in particular.
	if len(rendered) > contentHeight {
		rendered = rendered[:contentHeight]
	}
	for len(rendered) < contentHeight {
		rendered = append(rendered, "")
	}

	// Ensure no rendered line exceeds contentWidth visual cells. Wrapped lines
	// or FillLinesBg padding can occasionally produce lines wider than the border
	// container expects, causing lipgloss to re-wrap them internally and push the
	// bottom border out of view. Same defense extends to lines that contain an
	// embedded newline (a sanitize gap or a producer with unusual control bytes)
	// — strip those so each row stays a single visible row before lipgloss
	// counts heights.
	for i, line := range rendered {
		if strings.ContainsRune(line, '\n') {
			line = strings.ReplaceAll(line, "\n", " ")
			rendered[i] = line
		}
		if lipgloss.Width(line) > contentWidth {
			rendered[i] = ansi.Truncate(line, contentWidth, "")
		}
	}

	bodyContent := strings.Join(rendered, "\n")
	// Fill each line's background so ANSI resets from styled segments
	// (line numbers, search highlights) don't leave gaps in the theme bg.
	bodyContent = FillLinesBg(bodyContent, contentWidth, BaseBg)
	borderStyle := FullscreenBorderStyle(width, contentHeight)
	body := borderStyle.Render(bodyContent)

	if omitFooter {
		return lipgloss.JoinVertical(lipgloss.Left, titleBar, body)
	}
	return lipgloss.JoinVertical(lipgloss.Left, titleBar, body, footer)
}

// renderLogTitleBar builds the title bar with status indicators for the log viewer.
func renderLogTitleBar(title string, lines []string, width int, follow, wrap, lineNumbers, timestamps, previous, hidePrefixes, visualMode bool, visualType rune, loadingHistory bool, searchQuery string, filterQuery string, sevThreshold int) string {
	type indicatorFlag struct {
		enabled bool
		label   string
	}
	flags := []indicatorFlag{
		{follow, "FOLLOW"},
		{wrap, "WRAP"},
		{lineNumbers, "LINE#"},
		{timestamps, "TIMESTAMPS"},
		{hidePrefixes, "NO PREFIX"},
		{previous, "PREVIOUS"},
		{loadingHistory, "LOADING HISTORY..."},
	}
	var indicators []string
	for _, f := range flags {
		if f.enabled {
			indicators = append(indicators, HelpKeyStyle.Render("["+f.label+"]"))
		}
	}
	if visualMode {
		switch visualType {
		case 'v':
			indicators = append(indicators, HelpKeyStyle.Render("[VISUAL]"))
		case 'B':
			indicators = append(indicators, HelpKeyStyle.Render("[VISUAL BLOCK]"))
		default:
			indicators = append(indicators, HelpKeyStyle.Render("[VISUAL LINE]"))
		}
	}
	if searchQuery != "" {
		indicators = append(indicators, HelpKeyStyle.Render("[/"+searchQuery+"]"))
	}
	if filterQuery != "" {
		indicators = append(indicators, HelpKeyStyle.Render("[F:"+filterQuery+"]"))
	}
	if sevThreshold > 0 {
		indicators = append(indicators, HelpKeyStyle.Render("[≥"+LogLevelName(sevThreshold)+"]"))
	}

	titleText := " " + title + " "
	if len(indicators) > 0 {
		titleText += " " + strings.Join(indicators, " ")
	}
	titleText += BarDimStyle.Render(fmt.Sprintf(" [%d lines]", len(lines)))

	maxTitleWidth := max(width-2, 10)
	if ansi.StringWidth(titleText) > maxTitleWidth {
		titleText = ansi.Truncate(titleText, maxTitleWidth, "...")
	}
	return FillLinesBg(TitleStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(titleText), width, BarBg)
}

// RenderLogFooter builds the footer bar for the log viewer.
//
// searchQuery is the *committed* search query (empty until the user presses
// enter on the search prompt). The n/N next/prev hint is only surfaced once
// a query has been committed \u2014 advertising it on a fresh viewer with no
// search would be a no-op chord and a confusing claim.
//
// Exported so callers can render the footer at a wider terminal width than the
// log column itself (e.g. when the side preview pane is on and the hint bar
// must span the full screen below the JoinHorizontal'd panes \u2014 issue #71).
func RenderLogFooter(width int, statusMsg string, statusIsErr, searchActive bool, searchInput, searchQuery string, visualMode, canSwitchPod, canFilterContainers bool, filterActive bool, filterInput string) string {
	if statusMsg != "" {
		style := HelpKeyStyle
		if statusIsErr {
			style = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Bold(true).Background(BarBg)
		}
		return StatusBarBgStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(style.Render(statusMsg))
	}
	if searchActive {
		modeInd := SearchModeIndicator(searchInput)
		prompt := HelpKeyStyle.Render(ActiveKeybindings.Search) + BarDimStyle.Render(": ") + BarDimStyle.Render(modeInd) + searchInput + BarDimStyle.Render("\u2588") + BarDimStyle.Render("  (enter:apply  esc:cancel)")
		return StatusBarBgStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(prompt)
	}
	if filterActive {
		modeInd := SearchModeIndicator(filterInput)
		prompt := HelpKeyStyle.Render(ActiveKeybindings.Filter) + BarDimStyle.Render(": ") + BarDimStyle.Render(modeInd) + filterInput + BarDimStyle.Render("\u2588") + BarDimStyle.Render("  (esc:clear  enter:keep)")
		return StatusBarBgStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(prompt)
	}
	if visualMode {
		return RenderHintBar([]HintEntry{
			{Key: "j/k", Desc: "extend"},
			{Key: "h/l", Desc: "column"},
			{Key: "y", Desc: "copy"},
			{Key: "v/V/ctrl+v", Desc: "switch mode"},
			{Key: "esc", Desc: "cancel"},
		}, width)
	}
	hints := []HintEntry{
		{Key: "q", Desc: "close"},
		{Key: "j/k", Desc: "move"},
		{Key: "ctrl+d/u", Desc: "half page"},
		{Key: "ctrl+f/b", Desc: "page"},
		{Key: ActiveKeybindings.ToggleFollow, Desc: "follow"},
		{Key: ActiveKeybindings.ToggleWrap, Desc: "wrap"},
		{Key: ActiveKeybindings.ToggleLineNumbers, Desc: "line#"},
		{Key: ActiveKeybindings.ToggleTimestamps, Desc: "timestamps"},
		{Key: ActiveKeybindings.TogglePrefixes, Desc: "prefixes"},
		{Key: ActiveKeybindings.TogglePreview, Desc: "preview"},
		{Key: "J/K", Desc: "preview scroll"},
		{Key: "c", Desc: "previous"},
		{Key: "v/V/ctrl+v", Desc: "select"},
		{Key: "y", Desc: "copy"},
		{Key: ActiveKeybindings.Search, Desc: "search"},
		{Key: ActiveKeybindings.Filter, Desc: "filter"},
		{Key: ActiveKeybindings.SeverityDown + "/" + ActiveKeybindings.SeverityUp, Desc: "severity"},
	}
	if searchQuery != "" {
		hints = append(hints, HintEntry{Key: ActiveKeybindings.NextMatch + "/" + ActiveKeybindings.PrevMatch, Desc: "next/prev"})
	}
	hints = append(hints,
		HintEntry{Key: "123G", Desc: "goto"},
		HintEntry{Key: "S", Desc: "save"},
		HintEntry{Key: "ctrl+s", Desc: "save all"},
	)
	if canSwitchPod {
		hints = append(hints, HintEntry{"\\", "switch pod"})
	} else if canFilterContainers {
		hints = append(hints, HintEntry{"\\", "containers"})
	}
	return RenderHintBar(hints, width)
}

// highlightSearchMatches highlights occurrences of query in each line.
// Supports substring, regex, and fuzzy search modes via HighlightMatch.
// curRow is the rendered-slice index of the cursor line (-1 if none).
// curCol is the absolute visual column of the current match start on that row
// (-1 when curRow is -1). The cursor row's match at curCol is styled with
// SelectedSearchHighlightStyle; all other matches use LogSearchHighlightStyle.
func highlightSearchMatches(lines []string, query string, curRow, curCol int) []string {
	result := make([]string, len(lines))
	for i, line := range lines {
		if i == curRow && curRow >= 0 {
			result[i] = HighlightMatchCurrentAtCol(line, query, LogSearchHighlightStyle, SelectedSearchHighlightStyle, curCol)
		} else {
			result[i] = HighlightMatch(line, query)
		}
	}
	return result
}

// renderPlainLines renders lines without wrapping. It also returns cursorRow
// (the index in result of the cursor line, or -1 if not visible) and cursorCol
// (the absolute visual column of the search-match start on the cursor row, or
// -1 when cursorRow is -1 or the cursor is in visual-selection mode).
func renderPlainLines(lines []string, scroll, height, width int, lineNumbers bool, lineNumWidth int, cursor int, selStart, selEnd, visualStart int, visualType rune, visualCol, visualCurCol int) ([]string, int, int) {
	var result []string
	cursorRow := -1
	cursorCol := -1

	end := min(scroll+height, len(lines))

	// Reserve 1 column for cursor gutter.
	effectiveWidth := max(width-1, 10)

	for i := scroll; i < end; i++ {
		line := lines[i]

		// Check selection state before applying styles.
		isSelected := selStart >= 0 && i >= selStart && i <= selEnd

		if isSelected {
			// Build plain text for selection highlight on raw content (no line numbers).
			// Line numbers are prepended after highlighting to avoid column offset mismatch.
			//
			// Use ansi.Truncate (visual-width aware) for the pre-trim instead
			// of rune-slicing. On a kyverno-style log row each embedded SGR
			// sequence costs 4-5 rune slots while contributing zero visual
			// width; the rune-based cap chopped lines off mid-content (and
			// often mid-CSI), which downstream let "0m" / "[NNm" leak as
			// literal text and the visible payload was replaced by trailing
			// spaces from FillLinesBg's pad-to-width pass.
			plainLine := lines[i]
			selEffWidth := effectiveWidth
			if lineNumbers {
				selEffWidth -= lineNumWidth
			}
			if ansi.StringWidth(plainLine) > selEffWidth {
				plainLine = ansi.Truncate(plainLine, selEffWidth, "")
			}
			line = RenderVisualSelection(plainLine, visualType, i, selStart, selEnd, visualStart, visualCol, visualCurCol, min(visualCol, visualCurCol), max(visualCol, visualCurCol))
			if lineNumbers {
				numStr := fmt.Sprintf("%*d ", lineNumWidth-1, i+1)
				line = DimStyle.Render(numStr) + line
			}
		} else {
			// Colorize pod prefix for non-selected, non-cursor lines.
			if i != cursor {
				line = colorizePodPrefix(line)
			}
			if lineNumbers && i != cursor {
				// Non-cursor lines get line numbers here.
				// Cursor line's number is added after RenderCursorAtCol to avoid it being stripped.
				numStr := fmt.Sprintf("%*d ", lineNumWidth-1, i+1)
				line = DimStyle.Render(numStr) + line
			}
			// Truncate to width if needed (ANSI-aware to handle styled line numbers).
			if lipgloss.Width(line) > effectiveWidth {
				line = ansi.Truncate(line, effectiveWidth, "")
			}
		}

		// Cursor gutter indicator and block cursor at column.
		if i == cursor {
			if isSelected {
				// In visual mode, don't overlay block cursor on visual selection styling.
				line = YamlCursorIndicatorStyle.Render("\u258e") + line
			} else {
				cursorLine := RenderCursorAtCol(line, lines[i], visualCurCol)
				if lineNumbers {
					numStr := fmt.Sprintf("%*d ", lineNumWidth-1, i+1)
					cursorLine = YamlCursorIndicatorStyle.Render(numStr) + cursorLine
				}
				line = YamlCursorIndicatorStyle.Render("\u258e") + colorizePodPrefix(cursorLine)
				// Record where in the result slice this cursor row lands and its
				// search-match column. The gutter indicator is 1 visual cell; the
				// optional line-number gutter follows; then content at visualCurCol.
				cursorRow = len(result)
				cursorCol = 1 + visualCurCol
				if lineNumbers {
					cursorCol = 1 + lineNumWidth + visualCurCol
				}
			}
		} else {
			line = " " + line
		}

		result = append(result, line)
	}
	return result, cursorRow, cursorCol
}

// renderWrappedLines renders lines with wrapping, accounting for scroll position.
// topSkip drops that many wrapped sub-lines from the top of lines[scroll], so
// the renderer can pin a too-tall final source line's tail to the bottom row
// when following — without it, long log lines wrapping past viewH lose their
// most recent sub-lines off the bottom.
//
// It also returns cursorRow (index in result of the first wrapped sub-line of
// the cursor line, or -1 if not visible/in visual mode) and cursorCol (absolute
// visual column of the search-match start on that row, or -1 when cursorRow=-1).
func renderWrappedLines(lines []string, scroll, height, width int, lineNumbers bool, lineNumWidth int, cursor int, selStart, selEnd, visualStart int, visualType rune, visualCol, visualCurCol, topSkip int) ([]string, int, int) {
	// Reserve 1 column for cursor gutter.
	gutterWidth := 1
	availWidth := width - gutterWidth
	if lineNumbers {
		availWidth = width - gutterWidth - lineNumWidth
	}
	if availWidth < 10 {
		availWidth = 10
	}

	// We need to figure out which source lines and which wrapped sub-lines
	// correspond to the scroll offset. For wrapping, scroll refers to source
	// lines; topSkip drops sub-lines from the very top of lines[scroll].
	var result []string
	cursorRow := -1
	cursorCol := -1

	skipped := 0
	end := len(lines)
	for i := scroll; i < end && len(result) < height; i++ {
		line := lines[i]
		isSelected := selStart >= 0 && i >= selStart && i <= selEnd
		wrapped := wrapLine(line, availWidth)
		for j, wl := range wrapped {
			if skipped < topSkip {
				skipped++
				continue
			}
			if len(result) >= height {
				break
			}

			if isSelected {
				// Highlight raw content first, then prepend line numbers
				// to avoid column offset mismatch.
				wl = RenderVisualSelection(wl, visualType, i, selStart, selEnd, visualStart, visualCol, visualCurCol, min(visualCol, visualCurCol), max(visualCol, visualCurCol))
				if lineNumbers {
					if j == 0 {
						numStr := fmt.Sprintf("%*d ", lineNumWidth-1, i+1)
						wl = DimStyle.Render(numStr) + wl
					} else {
						wl = strings.Repeat(" ", lineNumWidth) + wl
					}
				}
			} else {
				// Colorize pod prefix for non-selected, non-cursor first sub-lines.
				if j == 0 && i != cursor {
					wl = colorizePodPrefix(wl)
				}
				if lineNumbers && (i != cursor || j != 0) {
					// Non-cursor lines get line numbers here.
					// Cursor line's number is added after RenderCursorAtCol.
					if j == 0 {
						numStr := fmt.Sprintf("%*d ", lineNumWidth-1, i+1)
						wl = DimStyle.Render(numStr) + wl
					} else {
						wl = strings.Repeat(" ", lineNumWidth) + wl
					}
				}
			}

			// Cursor gutter indicator and block cursor at column (only on first wrapped sub-line).
			if i == cursor && j == 0 {
				if isSelected {
					wl = YamlCursorIndicatorStyle.Render("\u258e") + wl
				} else {
					cursorLine := RenderCursorAtCol(wl, lines[i], visualCurCol)
					if lineNumbers {
						numStr := fmt.Sprintf("%*d ", lineNumWidth-1, i+1)
						cursorLine = YamlCursorIndicatorStyle.Render(numStr) + cursorLine
					}
					wl = YamlCursorIndicatorStyle.Render("\u258e") + colorizePodPrefix(cursorLine)
					// Record cursor position for search-match current highlighting.
					cursorRow = len(result)
					cursorCol = 1 + visualCurCol
					if lineNumbers {
						cursorCol = 1 + lineNumWidth + visualCurCol
					}
				}
			} else {
				wl = " " + wl
			}

			result = append(result, wl)
		}
	}
	return result, cursorRow, cursorCol
}

// WrapLine splits a line into chunks of at most width runes.
// Exported for reuse in YAML, describe, and diff view wrapping.
func WrapLine(line string, width int) []string {
	return wrapLine(line, width)
}

// wrapLine splits a line into chunks of at most width runes.
// wrapLine hard-wraps a log line to width visual columns. Uses ansi.Hardwrap
// so embedded SGR sequences (kyverno timestamps, klog level colors, etc.)
// stay intact across the split — rune-slicing instead would split mid-CSI
// and leak "0m"/"[NNm" as literal text or chop real content because escape
// bytes are zero-width but consume rune budget.
func wrapLine(line string, width int) []string {
	if width <= 0 {
		return []string{line}
	}
	if line == "" {
		return []string{""}
	}
	// preserveSpace=true keeps a leading space at the start of a wrapped
	// sub-line. Log content like "level message" must wrap to ["level", " ",
	// "message"] not ["level", "message"], otherwise field separation goes
	// missing visually -- and the existing rune-slicer kept the space too.
	return strings.Split(ansi.Hardwrap(line, width, true), "\n")
}

// StripTimestamp removes a leading kubectl timestamp (RFC3339Nano + space) from a log line.
// kubectl --timestamps format: "2024-01-15T10:30:00.000000000Z <log content>"
// With --prefix: "[pod/name container] 2024-01-15T10:30:00.000000000Z <log content>"
func StripTimestamp(line string) string {
	// Handle prefixed lines: "[pod/name container] timestamp rest"
	if len(line) > 0 && line[0] == '[' {
		closeBracket := strings.Index(line, "] ")
		if closeBracket > 0 {
			prefix := line[:closeBracket+2]
			rest := stripTimestampRaw(line[closeBracket+2:])
			return prefix + rest
		}
	}
	return stripTimestampRaw(line)
}

// stripTimestampRaw removes a leading RFC3339Nano timestamp from a string.
// Delegates to splitLeadingTimestamp so detection rules stay in one place.
func stripTimestampRaw(s string) string {
	if _, rest, ok := splitLeadingTimestamp(s); ok {
		return rest
	}
	return s
}

// podPrefixColors is a palette of distinct colors for pod/container log prefixes.
// Values are routed through ThemeColor so ConfigNoColor strips them to NoColor{}.
var podPrefixColors = []string{
	"#7aa2f7", // blue
	"#9ece6a", // green
	"#bb9af7", // purple
	"#e0af68", // yellow
	"#f7768e", // red/pink
	"#73daca", // cyan
	"#ff9e64", // orange
	"#2ac3de", // light blue
	"#b4f9f8", // teal
	"#c0caf5", // light lavender
	"#ff007c", // magenta
	"#41a6b5", // dark cyan
}

// podPrefixColorIdx remembers the palette slot assigned to each
// "pod/name/container" prefix so a prefix keeps its color for the session.
// Only the first len(podPrefixColors) distinct prefixes are recorded (each
// entry claims a free slot), so the map is bounded at the palette size;
// later prefixes fall back to their stable hash slot without caching.
var (
	podPrefixColorMu    sync.Mutex
	podPrefixColorIdx   = map[string]int{}
	podPrefixColorTaken = make([]bool, len(podPrefixColors))
)

// podPrefixColor picks the color for a "pod/name/container" prefix. The hash
// of the full prefix (pod AND container) seeds the slot, then probing skips
// slots already taken by other prefixes — so different containers of the
// same pod never share a color until the palette is exhausted. Past
// exhaustion the raw hash slot is used, which is still deterministic.
func podPrefixColor(prefix string) string {
	podPrefixColorMu.Lock()
	defer podPrefixColorMu.Unlock()
	if idx, ok := podPrefixColorIdx[prefix]; ok {
		return podPrefixColors[idx]
	}

	var hash uint32
	for _, c := range prefix {
		hash = hash*31 + uint32(c)
	}
	start := int(hash % uint32(len(podPrefixColors)))
	for i := range podPrefixColors {
		cand := (start + i) % len(podPrefixColors)
		if !podPrefixColorTaken[cand] {
			podPrefixColorTaken[cand] = true
			podPrefixColorIdx[prefix] = cand
			return podPrefixColors[cand]
		}
	}
	return podPrefixColors[start]
}

// colorizePodPrefix replaces the "[pod/name/container] " prefix with a
// colorized version. Each distinct pod/container pair gets its own color
// (see podPrefixColor), stable for the session.
func colorizePodPrefix(line string) string {
	if len(line) == 0 || line[0] != '[' {
		return line
	}
	closeBracket := strings.Index(line, "] ")
	if closeBracket < 0 {
		return line
	}
	prefix := line[1:closeBracket] // "pod/name/container"
	rest := line[closeBracket+2:]

	style := lipgloss.NewStyle().Foreground(ThemeColor(podPrefixColor(prefix)))
	return style.Render("["+prefix+"]") + " " + rest
}

// StripPodPrefix removes the leading "[pod/name/container] " prefix from a log line.
func StripPodPrefix(line string) string {
	if len(line) == 0 || line[0] != '[' {
		return line
	}
	_, after, ok := strings.Cut(line, "] ")
	if !ok {
		return line
	}
	return after
}

// logTabWidth is the column step used when expanding tab characters in
// log lines. 8 matches the default tab stop on virtually every terminal,
// so the post-expansion text aligns the way a user pasting the same line
// into their shell would expect.
const logTabWidth = 8

// sanitizeLogLine replaces non-printable control bytes (NUL, DEL, the C0
// control range minus tab) with the Unicode replacement character and
// expands tab characters to spaces using a logTabWidth-column tab stop.
// Binary data from processes like MySQL handshakes contains bytes that
// break terminal width calculations and corrupt the viewer layout.
//
// Tab expansion is required because lipgloss.Width treats '\t' as
// zero-width while the terminal renders it as a jump to the next tab
// stop. The viewer's contentWidth-overflow guard (in RenderLogViewer)
// uses lipgloss.Width to decide when to truncate; without expansion,
// tab-bearing lines slip through with an undercounted width, get
// re-wrapped internally by lipgloss, and push the bottom border off the
// visible area. Reported on dragonfly-operator (controller-runtime/zap)
// logs in particular - those use tabs to separate timestamp / level /
// logger / message.
//
// When renderAnsi is true, valid CSI SGR sequences (ESC [ params m — the
// ones that set colour, bold, underline, etc.) are preserved verbatim so
// log producers that emit ANSI colours render as intended. Non-SGR CSI
// sequences (cursor movement, screen erase) remain unsafe for an inline
// viewer and are still replaced. A bare ESC with no valid CSI introducer
// is replaced too; leaving it would cause terminals to wait for a
// follow-up byte and mis-interpret subsequent output.
func sanitizeLogLine(s string, renderAnsi bool) string {
	// Fast path: no control bytes (incl. tabs that need expansion) means
	// no work to do.
	needsSanitize := false
	for i := range len(s) {
		c := s[i]
		if c < 32 || c == 127 {
			needsSanitize = true
			break
		}
	}
	if !needsSanitize {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))
	col := 0
	i := 0
	for i < len(s) {
		c := s[i]
		if renderAnsi && c == 0x1b {
			if end := parseSGRSequence(s, i); end > i {
				// SGR sequences are zero-width; do not advance col.
				b.WriteString(s[i:end])
				i = end
				continue
			}
		}
		if c == '\t' {
			n := logTabWidth - col%logTabWidth
			for range n {
				b.WriteByte(' ')
			}
			col += n
			i++
			continue
		}
		if c >= 32 && c != 127 {
			// Printable ASCII or UTF-8 leading/continuation byte.
			// UTF-8 continuation bytes are all >= 0x80 so they land
			// here on subsequent iterations and copy through intact.
			// Column tracking treats every UTF-8 leading byte and
			// every ASCII printable as one cell; an approximation
			// that is fine for tab-stop alignment (CJK in log lines
			// is rare and the worst case is a 1-cell off-stop nudge,
			// not a width undercount).
			b.WriteByte(c)
			if c < 0x80 || c >= 0xC0 {
				col++
			}
			i++
			continue
		}
		// Control byte (< 32 and not tab, or DEL). Emit the
		// replacement character and advance one byte.
		b.WriteRune('\ufffd')
		i++
	}
	return b.String()
}

// parseSGRSequence returns the index after a valid ESC [ ... m sequence
// starting at s[i], or i if no valid sequence is present. Only SGR
// (Select Graphic Rendition) finals are accepted because they set
// colour and text attributes without moving the cursor or clearing the
// screen — preserving them is safe in an inline viewer, whereas other
// CSI finals would corrupt the layout.
func parseSGRSequence(s string, i int) int {
	if i+1 >= len(s) || s[i] != 0x1b || s[i+1] != '[' {
		return i
	}
	j := i + 2
	// Parameter bytes: 0x30-0x3F (digits and ; : < = > ?).
	for j < len(s) && s[j] >= 0x30 && s[j] <= 0x3F {
		j++
	}
	// Intermediate bytes: 0x20-0x2F (space and ! " # $ % & ' ( ) * + , - . /).
	for j < len(s) && s[j] >= 0x20 && s[j] <= 0x2F {
		j++
	}
	// SGR final byte is lowercase 'm'. Anything else is a CSI we can't
	// safely forward to an inline viewer.
	if j < len(s) && s[j] == 'm' {
		return j + 1
	}
	return i
}
