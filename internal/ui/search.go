package ui

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
)

// SearchMode represents the type of search being performed.
type SearchMode int

const (
	// SearchSubstring performs a case-insensitive substring match.
	SearchSubstring SearchMode = iota
	// SearchRegex compiles the query as a case-insensitive regex.
	SearchRegex
	// SearchFuzzy matches characters in order (fuzzy matching).
	SearchFuzzy
)

// DetectSearchMode determines the search mode from the raw query string.
// Returns the mode and the effective query (with prefix stripped if applicable).
//
// Modes:
//   - "~" prefix: fuzzy match
//   - "\" prefix: literal/escaped substring match
//   - auto-detected regex metacharacters: regex mode
//   - otherwise: plain substring
func DetectSearchMode(rawQuery string) (SearchMode, string) {
	if rawQuery == "" {
		return SearchSubstring, ""
	}
	// Fuzzy prefix: ~
	if strings.HasPrefix(rawQuery, "~") {
		return SearchFuzzy, rawQuery[1:]
	}
	// Literal escape prefix: backslash
	if strings.HasPrefix(rawQuery, `\`) {
		return SearchSubstring, rawQuery[1:]
	}
	// Auto-detect regex: check for regex metacharacters.
	if containsRegexMeta(rawQuery) {
		return SearchRegex, rawQuery
	}
	return SearchSubstring, rawQuery
}

// containsRegexMeta returns true if the string contains regex metacharacters.
func containsRegexMeta(s string) bool {
	for _, c := range s {
		switch c {
		case '.', '*', '+', '?', '^', '$', '{', '}', '(', ')', '|', '[', ']':
			return true
		}
	}
	return false
}

// MatchLine returns true if the line matches the query using the appropriate
// search mode. The rawQuery is the original user input (not lowercased).
func MatchLine(line, rawQuery string) bool {
	if rawQuery == "" {
		return false
	}
	mode, query := DetectSearchMode(rawQuery)
	if query == "" {
		return false
	}
	switch mode {
	case SearchRegex:
		re, err := regexp.Compile("(?i)" + query)
		if err != nil {
			// Fall back to substring on invalid regex.
			return strings.Contains(strings.ToLower(line), strings.ToLower(query))
		}
		return re.MatchString(line)
	case SearchFuzzy:
		return fuzzyMatch(line, query)
	default:
		return strings.Contains(strings.ToLower(line), strings.ToLower(query))
	}
}

// FuzzyScore returns a score for how well the line matches the fuzzy query.
// Higher is better. Returns -1 if no match. Used for ranking in filter mode.
func FuzzyScore(line, query string) int {
	lineLower := strings.ToLower(line)
	queryLower := strings.ToLower(query)

	lineRunes := []rune(lineLower)
	queryRunes := []rune(queryLower)

	if len(queryRunes) == 0 {
		return 0
	}

	qi := 0
	score := 0
	consecutive := 0
	prevMatch := false

	for li := 0; li < len(lineRunes) && qi < len(queryRunes); li++ {
		if lineRunes[li] == queryRunes[qi] {
			qi++
			if prevMatch {
				consecutive++
				score += consecutive * 2 // Bonus for consecutive matches.
			}
			score++
			// Bonus for matching at word boundaries.
			if li == 0 || !unicode.IsLetter(lineRunes[li-1]) {
				score += 3
			}
			prevMatch = true
		} else {
			prevMatch = false
			consecutive = 0
		}
	}

	if qi < len(queryRunes) {
		return -1 // Not all query characters matched.
	}
	return score
}

// fuzzyMatch returns true if all characters of query appear in line in order.
func fuzzyMatch(line, query string) bool {
	return FuzzyScore(line, query) >= 0
}

// HighlightMatch returns the line with the matched portion highlighted using
// LogSearchHighlightStyle. The rawQuery is the original user input.
func HighlightMatch(line, rawQuery string) string {
	return HighlightMatchStyled(line, rawQuery, LogSearchHighlightStyle)
}

// HighlightMatchStyled returns the line with the matched portion highlighted
// using the given style. The rawQuery is the original user input.
func HighlightMatchStyled(line, rawQuery string, style lipgloss.Style) string {
	return HighlightMatchStyledOver(line, rawQuery, style, lipgloss.Style{})
}

// HighlightMatchStyledOver behaves like HighlightMatchStyled but
// re-asserts outerStyle's open codes after each inner highlight's
// reset. Use this when the returned string will be wrapped by
// outerStyle.Render(...) and the inner highlights' resets would
// otherwise wipe the outer background for the rest of the line —
// e.g. a search hit inside a cursor row's selection bg, or a match
// inside a category bar.
//
// Pass an empty (zero-value) outerStyle to opt out of the
// re-assertion and get the legacy single-pass behaviour.
func HighlightMatchStyledOver(line, rawQuery string, style, outerStyle lipgloss.Style) string {
	if rawQuery == "" {
		return line
	}
	mode, query := DetectSearchMode(rawQuery)
	if query == "" {
		return line
	}
	restore := styleOpenCodes(outerStyle)
	switch mode {
	case SearchRegex:
		return highlightRegex(line, query, style, restore)
	case SearchFuzzy:
		return highlightFuzzy(line, query, style, restore)
	default:
		return highlightSubstring(line, query, style, restore)
	}
}

// styleOpenCodes extracts just the SGR open sequence a style emits
// when rendering. Returns "" when the style produces no codes (zero
// value or color-less). Used by HighlightMatchStyledOver to splice
// the outer style back in after each inner highlight reset, and by
// RenderOverPrestyled to re-open the outer style around an already-
// highlighted line.
//
// Implementation note: a single ASCII character is used as the marker
// because lipgloss in some profile/attribute combinations (notably
// Underline(true) under termenv.ANSI — the NO_COLOR profile) renders
// each input character through a fresh open/close SGR pair. A
// multi-character marker would be split across those per-char wraps
// and never appear contiguously in the output, so the search would
// fall through and return "" — yielding a missing outer style (e.g.
// the Bold+Underline category bar losing its underline whenever
// search highlighting was active under NO_COLOR).
func styleOpenCodes(style lipgloss.Style) string {
	const marker = "x"
	open, _, found := strings.Cut(style.Render(marker), marker)
	if !found {
		return ""
	}
	return open
}

// ansiReset is the SGR reset sequence emitted at the end of a rendered
// run. Matches what lipgloss appends to its own Render output.
const ansiReset = "\x1b[0m"

// RenderOverPrestyled wraps a line that may already contain inner ANSI
// codes (from a prior HighlightMatchStyledOver pass) with outerStyle's
// open/close SGR codes, bypassing lipgloss.Render.
//
// This exists because lipgloss.Render fragments any embedded ANSI in
// its input — every byte of the embedded escape sequences gets wrapped
// with outerStyle individually, doubling the ESC introducer and
// producing malformed output of the form "\x1b\x1b[0m...". Most
// terminals tolerate that, but in NO_COLOR / ANSI profile mode some
// terminals render the second sequence as literal text ("[1;7mNetw[0m"),
// and the visible-width calculation goes off so the line wraps.
//
// The manual SGR open + content + reset path keeps the inner
// highlights intact and produces a stream the terminal can parse
// uniformly.
//
// Returns line unchanged when outerStyle has no open codes.
func RenderOverPrestyled(line string, outerStyle lipgloss.Style) string {
	open := styleOpenCodes(outerStyle)
	if open == "" {
		return line
	}
	return open + line + ansiReset
}

// HighlightMatchInline overlays hlStyle on each occurrence of query in a
// line that already contains inline SGR codes (e.g. YAML syntax styling).
// After each match's reset it re-emits the SGR sequence that was active
// just before the match so the rest of the surrounding token keeps its
// styling — without this, hlStyle.Render's trailing "\x1b[0m" cancels the
// outer token's open codes and the post-match segment renders in the
// terminal default.
//
// Substring mode only. Skips ANSI escape sequences while scanning for
// matches and refuses matches whose byte range crosses an ESC byte, so a
// query like "[33m" can't latch onto an SGR introducer.
func HighlightMatchInline(line, rawQuery string, hlStyle lipgloss.Style) string {
	if rawQuery == "" {
		return line
	}
	mode, query := DetectSearchMode(rawQuery)
	if query == "" {
		return line
	}
	switch mode {
	case SearchRegex, SearchFuzzy:
		// Regex / fuzzy don't get the inline-aware treatment yet; fall
		// back to the plain substring path so they keep the existing
		// behavior. The YAML preview's main caller uses substring.
		return HighlightMatchStyled(line, rawQuery, hlStyle)
	}

	queryLen := len(query)
	var b strings.Builder
	b.Grow(len(line))
	activeOpen := ""

	i := 0
	for i < len(line) {
		// Pass through ANSI CSI sequences and update the active SGR state.
		if i+1 < len(line) && line[i] == 0x1b && line[i+1] == '[' {
			j := i + 2
			for j < len(line) && line[j] >= 0x30 && line[j] <= 0x3F {
				j++
			}
			for j < len(line) && line[j] >= 0x20 && line[j] <= 0x2F {
				j++
			}
			if j < len(line) && line[j] >= 0x40 && line[j] <= 0x7E {
				seq := line[i : j+1]
				b.WriteString(seq)
				if line[j] == 'm' {
					if seq == ansiReset || seq == "\x1b[m" {
						activeOpen = ""
					} else {
						activeOpen = seq
					}
				}
				i = j + 1
				continue
			}
		}
		// Try matching the query at this byte position.
		if i+queryLen <= len(line) {
			tail := line[i : i+queryLen]
			hasEsc := false
			for k := range len(tail) {
				if tail[k] == 0x1b {
					hasEsc = true
					break
				}
			}
			if !hasEsc && strings.EqualFold(tail, query) {
				b.WriteString(hlStyle.Render(tail))
				// Re-assert the active outer SGR so the rest of the
				// token reads in its original color instead of dropping
				// to terminal default after hlStyle's reset.
				if activeOpen != "" {
					b.WriteString(activeOpen)
				}
				i += queryLen
				continue
			}
		}
		b.WriteByte(line[i])
		i++
	}
	return b.String()
}

// highlightSubstring highlights all occurrences of query in line
// (case-insensitive). When restoreCodes is non-empty it's emitted
// after each inner highlight reset so a wrapping outer style's
// background stays visible for the post-match segment.
func highlightSubstring(line, query string, style lipgloss.Style, restoreCodes string) string {
	queryLower := strings.ToLower(query)
	lineLower := strings.ToLower(line)
	if !strings.Contains(lineLower, queryLower) {
		return line
	}
	var b strings.Builder
	pos := 0
	for pos < len(line) {
		idx := strings.Index(strings.ToLower(line[pos:]), queryLower)
		if idx < 0 {
			b.WriteString(line[pos:])
			break
		}
		b.WriteString(line[pos : pos+idx])
		b.WriteString(style.Render(line[pos+idx : pos+idx+len(query)]))
		b.WriteString(restoreCodes)
		pos = pos + idx + len(query)
	}
	return b.String()
}

// highlightRegex highlights all regex matches in the line. See
// highlightSubstring for the restoreCodes contract.
func highlightRegex(line, query string, style lipgloss.Style, restoreCodes string) string {
	re, err := regexp.Compile("(?i)" + query)
	if err != nil {
		return highlightSubstring(line, query, style, restoreCodes) // fallback
	}
	matches := re.FindAllStringIndex(line, -1)
	if len(matches) == 0 {
		return line
	}
	var b strings.Builder
	pos := 0
	for _, m := range matches {
		if m[0] > pos {
			b.WriteString(line[pos:m[0]])
		}
		b.WriteString(style.Render(line[m[0]:m[1]]))
		b.WriteString(restoreCodes)
		pos = m[1]
	}
	if pos < len(line) {
		b.WriteString(line[pos:])
	}
	return b.String()
}

// highlightFuzzy highlights the matched characters in a fuzzy match.
// See highlightSubstring for the restoreCodes contract.
func highlightFuzzy(line, query string, style lipgloss.Style, restoreCodes string) string {
	lineLower := strings.ToLower(line)
	queryLower := strings.ToLower(query)
	lineRunes := []rune(line)
	lineLowerRunes := []rune(lineLower)
	queryRunes := []rune(queryLower)

	if len(queryRunes) == 0 {
		return line
	}

	// Find matching positions.
	matchPositions := make([]bool, len(lineRunes))
	qi := 0
	for li := 0; li < len(lineLowerRunes) && qi < len(queryRunes); li++ {
		if lineLowerRunes[li] == queryRunes[qi] {
			matchPositions[li] = true
			qi++
		}
	}
	if qi < len(queryRunes) {
		return line // No full match.
	}

	// Build highlighted string, grouping consecutive matched characters.
	var b strings.Builder
	inHighlight := false
	highlightStart := 0
	for i, r := range lineRunes {
		if matchPositions[i] {
			if !inHighlight {
				inHighlight = true
				highlightStart = i
			}
		} else {
			if inHighlight {
				b.WriteString(style.Render(string(lineRunes[highlightStart:i])))
				b.WriteString(restoreCodes)
				inHighlight = false
			}
			b.WriteRune(r)
		}
	}
	if inHighlight {
		b.WriteString(style.Render(string(lineRunes[highlightStart:])))
		b.WriteString(restoreCodes)
	}
	return b.String()
}

// SearchModeIndicator returns a short string to show in the search bar
// indicating the active search mode: "" for substring, "[RE] " for regex,
// "[~] " for fuzzy.
func SearchModeIndicator(rawQuery string) string {
	mode, _ := DetectSearchMode(rawQuery)
	switch mode {
	case SearchRegex:
		return "[RE] "
	case SearchFuzzy:
		return "[~] "
	default:
		return ""
	}
}

// FindColumnInLine returns the rune column of the first match of rawQuery in
// line, or -1 if not found. Used for cursor positioning after search.
func FindColumnInLine(line, rawQuery string) int {
	if rawQuery == "" || line == "" {
		return -1
	}
	mode, query := DetectSearchMode(rawQuery)
	if query == "" {
		return -1
	}
	switch mode {
	case SearchRegex:
		re, err := regexp.Compile("(?i)" + query)
		if err != nil {
			// Fallback to substring.
			col := strings.Index(strings.ToLower(line), strings.ToLower(query))
			if col < 0 {
				return -1
			}
			return len([]rune(line[:col]))
		}
		loc := re.FindStringIndex(line)
		if loc == nil {
			return -1
		}
		return len([]rune(line[:loc[0]]))
	case SearchFuzzy:
		// For fuzzy, find the position of the first matching character.
		queryLower := strings.ToLower(query)
		lineLower := strings.ToLower(line)
		lineRunes := []rune(lineLower)
		queryRunes := []rune(queryLower)
		if len(queryRunes) == 0 {
			return -1
		}
		for i, r := range lineRunes {
			if r == queryRunes[0] {
				return i
			}
		}
		return -1
	default:
		col := strings.Index(strings.ToLower(line), strings.ToLower(query))
		if col < 0 {
			return -1
		}
		return len([]rune(line[:col]))
	}
}
