package app

import (
	"strconv"
	"strings"
)

// yamlSection represents a YAML section (a key with nested content below it).
// Sections can be at any indentation level, forming a hierarchy.
// The key field uses dot-path notation (e.g., "metadata.annotations").
type yamlSection struct {
	key       string // dot-path key (e.g., "metadata.annotations")
	indent    int    // indentation level of the header line
	startLine int    // index of the header line (the key: line itself)
	endLine   int    // index of the last line belonging to this section (inclusive)
	listItem  bool   // true for foldable list items (e.g., "- name: nginx" with children)
}

// parseYAMLSections identifies hierarchical YAML sections and their line ranges.
// A section is any line containing "key:" with no inline value (or a block scalar
// indicator like | or >) that has indented content on subsequent lines.
// Each section gets a dot-path key reflecting its position in the hierarchy
// (e.g., "metadata", "metadata.labels", "spec.containers.ports").
func parseYAMLSections(content string) []yamlSection {
	lines := strings.Split(content, "\n")
	var sections []yamlSection

	for i, line := range lines {
		if len(line) == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || trimmed == "---" {
			continue
		}

		// Calculate indent level.
		indent := 0
		for _, ch := range line {
			if ch == ' ' {
				indent++
			} else {
				break
			}
		}

		// Check if this line is a section header (key: with no inline value).
		// Strip list item prefix "- " for detection purposes.
		keyLine := trimmed
		isList := strings.HasPrefix(keyLine, "- ")
		if isList {
			keyLine = keyLine[2:]
		}

		colonIdx := strings.Index(keyLine, ":")
		if colonIdx <= 0 {
			continue
		}

		afterColon := strings.TrimSpace(keyLine[colonIdx+1:])
		// It's a section header if nothing meaningful comes after the colon
		// (empty, or block scalar indicator like | or >).
		isBlockHeader := afterColon == "" || afterColon == "|" || afterColon == ">" ||
			afterColon == "|-" || afterColon == ">-" ||
			afterColon == "|+" || afterColon == ">+"

		if !isBlockHeader {
			// Not a section header, but could be a foldable list item.
			// A list item like "- name: nginx" with continuation lines at deeper indent is foldable.
			if isList {
				contentIndent := indent + 2 // content after "- " starts 2 chars deeper
				hasListChildren := false
				for j := i + 1; j < len(lines); j++ {
					nextTrimmed := strings.TrimSpace(lines[j])
					if nextTrimmed == "" || strings.HasPrefix(nextTrimmed, "#") {
						continue
					}
					nextIndent := 0
					for _, ch := range lines[j] {
						if ch == ' ' {
							nextIndent++
						} else {
							break
						}
					}
					if nextIndent >= contentIndent {
						hasListChildren = true
					}
					break
				}
				if hasListChildren {
					listKey := "#" + strconv.Itoa(i)
					dotPath := buildDotPath(lines, i, indent+1, listKey)
					sections = append(sections, yamlSection{
						key:       dotPath,
						indent:    indent,
						startLine: i,
						listItem:  true,
					})
				}
			}
			continue
		}

		key := keyLine[:colonIdx]

		// Check that the next non-empty line is MORE indented (confirming children).
		// Also treat a YAML list item at the same indent as a child
		// (e.g., "containers:" followed by "- name:" at the same indent).
		hasChildren := false
		for j := i + 1; j < len(lines); j++ {
			nextTrimmed := strings.TrimSpace(lines[j])
			if nextTrimmed == "" || strings.HasPrefix(nextTrimmed, "#") {
				continue
			}
			nextIndent := 0
			for _, ch := range lines[j] {
				if ch == ' ' {
					nextIndent++
				} else {
					break
				}
			}
			if nextIndent > indent {
				hasChildren = true
			} else if nextIndent == indent && strings.HasPrefix(nextTrimmed, "- ") {
				hasChildren = true
			}
			break
		}

		if !hasChildren {
			continue
		}

		// If this line is BOTH a list item and a block header (e.g., "- args:"),
		// create a listItem section first (for folding the entire list element),
		// then the regular section (for folding just the block header's content).
		if isList {
			listKey := "#" + strconv.Itoa(i)
			listDotPath := buildDotPath(lines, i, indent+1, listKey)
			sections = append(sections, yamlSection{
				key:       listDotPath,
				indent:    indent,
				startLine: i,
				listItem:  true,
			})
		}

		// Build the dot-path key by finding ancestors.
		dotPath := buildDotPath(lines, i, indent, key)

		sections = append(sections, yamlSection{
			key:       dotPath,
			indent:    indent,
			startLine: i,
		})
	}

	// Calculate endLine for each section. A section ends when we encounter
	// a non-blank, non-comment line at the same or lesser indent level.
	// Exception: list items ("- ...") at the same indent are children of the
	// section (e.g., "containers:" followed by "- name: nginx" at the same indent).
	for idx := range sections {
		sec := &sections[idx]
		sec.endLine = len(lines) - 1

		if sec.listItem {
			// List item: ends at any line with indent < contentIndent.
			// This correctly handles sibling list items (at same indent < contentIndent).
			contentIndent := sec.indent + 2
			for j := sec.startLine + 1; j < len(lines); j++ {
				jTrimmed := strings.TrimSpace(lines[j])
				if jTrimmed == "" || strings.HasPrefix(jTrimmed, "#") {
					continue
				}
				jIndent := 0
				for _, ch := range lines[j] {
					if ch == ' ' {
						jIndent++
					} else {
						break
					}
				}
				if jIndent < contentIndent {
					sec.endLine = j - 1
					break
				}
			}
		} else {
			// Regular section.
			headerTrimmed := strings.TrimSpace(lines[sec.startLine])
			headerIsList := strings.HasPrefix(headerTrimmed, "- ")

			if headerIsList {
				// Section header is a list item (e.g., "- args:", "- backend:").
				// The key's content lives at indent+2 (after "- ").
				// Sibling list items at the same indent end the section.
				// Non-list lines at contentIndent are sibling keys within
				// the list item (e.g., "name:" next to "args:"), not children.
				contentIndent := sec.indent + 2
				for j := sec.startLine + 1; j < len(lines); j++ {
					jTrimmed := strings.TrimSpace(lines[j])
					if jTrimmed == "" || strings.HasPrefix(jTrimmed, "#") {
						continue
					}
					jIndent := 0
					for _, ch := range lines[j] {
						if ch == ' ' {
							jIndent++
						} else {
							break
						}
					}
					if jIndent < contentIndent {
						sec.endLine = j - 1
						break
					}
					// Line at contentIndent that isn't a list child is a
					// sibling key in the same list item.
					if jIndent == contentIndent && !strings.HasPrefix(jTrimmed, "- ") {
						sec.endLine = j - 1
						break
					}
				}
			} else {
				// Plain section (e.g., "containers:", "metadata:").
				// "- " lines at the same indent ARE children (kubectl raw format).
				seenListChild := false
				for j := sec.startLine + 1; j < len(lines); j++ {
					jTrimmed := strings.TrimSpace(lines[j])
					if jTrimmed == "" || strings.HasPrefix(jTrimmed, "#") {
						continue
					}
					jIndent := 0
					for _, ch := range lines[j] {
						if ch == ' ' {
							jIndent++
						} else {
							break
						}
					}
					if jIndent < sec.indent {
						sec.endLine = j - 1
						break
					}
					if jIndent == sec.indent {
						if strings.HasPrefix(jTrimmed, "- ") {
							seenListChild = true
							continue
						}
						if seenListChild {
							sec.endLine = j - 1
							break
						}
						sec.endLine = j - 1
						break
					}
				}
			}
		}
		// Trim trailing blank lines from the section.
		for sec.endLine > sec.startLine && strings.TrimSpace(lines[sec.endLine]) == "" {
			sec.endLine--
		}
	}

	return sections
}

// buildDotPath constructs the dot-separated path for a section key
// by looking at ancestor section headers above it.
// Only lines that are themselves section headers (key: with no inline value)
// are considered as ancestors, preventing key-value pairs like "name: nginx"
// from appearing in the path.
func buildDotPath(lines []string, lineIdx, indent int, key string) string {
	if indent == 0 {
		return key
	}

	// Walk upward to find parent sections at decreasing indent levels.
	var ancestors []string
	currentIndent := indent
	for i := lineIdx - 1; i >= 0; i-- {
		line := lines[i]
		if len(line) == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		lineIndent := 0
		for _, ch := range line {
			if ch == ' ' {
				lineIndent++
			} else {
				break
			}
		}

		if lineIndent < currentIndent {
			// This could be an ancestor -- but only if it's a section header.
			keyLine := trimmed
			if strings.HasPrefix(keyLine, "- ") {
				keyLine = keyLine[2:]
			}
			colonIdx := strings.Index(keyLine, ":")
			if colonIdx > 0 {
				afterColon := strings.TrimSpace(keyLine[colonIdx+1:])
				// Only treat it as an ancestor if it's a section header
				// (no inline value, or block scalar indicator).
				isSectionHeader := afterColon == "" || afterColon == "|" || afterColon == ">" ||
					afterColon == "|-" || afterColon == ">-" ||
					afterColon == "|+" || afterColon == ">+"
				if isSectionHeader {
					ancestors = append([]string{keyLine[:colonIdx]}, ancestors...)
				}
				currentIndent = lineIndent
				if lineIndent == 0 {
					break
				}
			}
		}
	}

	ancestors = append(ancestors, key)
	return strings.Join(ancestors, ".")
}

// isMultiLineSection returns true if the section has content lines beyond its header.
func isMultiLineSection(sec yamlSection) bool {
	return sec.endLine > sec.startLine
}

// buildVisibleLines produces the lines to display in the YAML view, applying
// fold indicators and hiding collapsed section content.
// It returns:
//   - the visible lines (with fold indicators prepended to section headers)
//   - a mapping from visible line index to original line index (for search coordination)
func buildVisibleLines(content string, sections []yamlSection, collapsed map[string]bool) ([]string, []int) {
	lines := strings.Split(content, "\n")
	if len(sections) == 0 {
		// No sections parsed: return all lines as-is.
		mapping := make([]int, len(lines))
		for i := range lines {
			mapping[i] = i
		}
		return lines, mapping
	}

	// Build lookup maps: listItem sections and non-listItem sections by start line.
	listItemByStart := make(map[int]yamlSection, len(sections))
	sectionByStart := make(map[int]yamlSection, len(sections))
	for _, sec := range sections {
		if sec.listItem {
			listItemByStart[sec.startLine] = sec
		} else {
			sectionByStart[sec.startLine] = sec
		}
	}

	// Build the set of lines that are hidden (inside collapsed sections).
	hidden := make(map[int]bool)
	for _, sec := range sections {
		if collapsed[sec.key] && isMultiLineSection(sec) {
			for ln := sec.startLine + 1; ln <= sec.endLine; ln++ {
				hidden[ln] = true
			}
		}
	}

	var visible []string
	var mapping []int
	for i, line := range lines {
		if hidden[i] {
			continue
		}

		// Check if this line starts BOTH a foldable listItem AND a section (split case).
		listSec, isListStart := listItemByStart[i]
		secSec, isSecStart := sectionByStart[i]
		if isListStart && isSecStart && isMultiLineSection(listSec) {
			// === Split: emit dash line + content line ===

			// Dash line: shows just the "-" with the listItem fold indicator.
			indicator := '▾'
			if collapsed[listSec.key] {
				indicator = '▸'
			}
			dashRaw := strings.Repeat(" ", listSec.indent) + "-"
			dashPrefixed := "  " + dashRaw
			runes := []rune(dashPrefixed)
			if listSec.indent < len(runes) {
				runes[listSec.indent] = indicator
			}
			visible = append(visible, string(runes))
			mapping = append(mapping, i)

			// Content line (only if listItem is not collapsed).
			if !collapsed[listSec.key] {
				trimmed := strings.TrimSpace(line)
				content := trimmed
				if strings.HasPrefix(content, "- ") {
					content = content[2:]
				}
				contentRaw := strings.Repeat(" ", listSec.indent+2) + content
				contentPrefixed := "  " + contentRaw

				// Add fold indicator for the section.
				if isMultiLineSection(secSec) {
					contentIndicator := '▾'
					if collapsed[secSec.key] {
						contentIndicator = '▸'
					}
					contentRunes := []rune(contentPrefixed)
					contentMarkerPos := listSec.indent + 2
					if contentMarkerPos < len(contentRunes) {
						contentRunes[contentMarkerPos] = contentIndicator
					}
					contentPrefixed = string(contentRunes)
				}

				visible = append(visible, contentPrefixed)
				mapping = append(mapping, i)
			}
			continue
		}

		// Non-split lines: use existing logic with sectionByStart OR listItemByStart.
		prefixed := "  " + line
		if isListStart && isMultiLineSection(listSec) {
			// Non-split listItem (like "- name: nginx" with children).
			indicator := '▾'
			if collapsed[listSec.key] {
				indicator = '▸'
			}
			runes := []rune(prefixed)
			markerPos := listSec.indent
			if markerPos < len(runes) {
				runes[markerPos] = indicator
			}
			visible = append(visible, string(runes))
		} else if isSecStart && isMultiLineSection(secSec) {
			// Regular section (not a list item).
			indicator := '▾'
			if collapsed[secSec.key] {
				indicator = '▸'
			}
			runes := []rune(prefixed)
			markerPos := secSec.indent
			if markerPos < len(runes) {
				runes[markerPos] = indicator
			}
			visible = append(visible, string(runes))
		} else {
			visible = append(visible, prefixed)
		}
		mapping = append(mapping, i)
	}

	return visible, mapping
}

// sectionForVisibleLine returns the most specific section key that the given
// visible line index falls within, or "" if it's outside any section.
// If the line is exactly a section header, that section is returned.
// Otherwise the deepest (highest indent) enclosing section is returned,
// so toggling a fold targets the innermost section, not a parent.
func sectionForVisibleLine(visibleIdx int, mapping []int, sections []yamlSection) string {
	if visibleIdx < 0 || visibleIdx >= len(mapping) {
		return ""
	}
	origLine := mapping[visibleIdx]

	// Detect split content line: previous visible line maps to the same original line.
	isSplitContent := visibleIdx > 0 && mapping[visibleIdx-1] == origLine

	// Prefer an exact header match first.
	var listItemFallback string
	for _, sec := range sections {
		if origLine == sec.startLine {
			if isSplitContent {
				// Content line: prefer non-listItem section.
				if !sec.listItem {
					return sec.key
				}
				listItemFallback = sec.key
			} else {
				// Dash line or normal line: return first match.
				return sec.key
			}
		}
	}
	if listItemFallback != "" {
		return listItemFallback
	}

	// Otherwise return the deepest enclosing section.
	// When two sections have the same indent (e.g., a list parent and a list item),
	// prefer the one with the later startLine (the more specific one).
	bestKey := ""
	bestIndent := -1
	bestStart := -1
	for _, sec := range sections {
		if origLine >= sec.startLine && origLine <= sec.endLine {
			if sec.indent > bestIndent || (sec.indent == bestIndent && sec.startLine > bestStart) {
				bestIndent = sec.indent
				bestStart = sec.startLine
				bestKey = sec.key
			}
		}
	}
	return bestKey
}

// sectionAtScrollPos returns the section key at the current scroll position
// (the top visible line in the viewport).
func sectionAtScrollPos(scrollPos int, mapping []int, sections []yamlSection) string {
	return sectionForVisibleLine(scrollPos, mapping, sections)
}

// visibleLineCount returns the number of visible lines after applying folds.
func visibleLineCount(content string, sections []yamlSection, collapsed map[string]bool) int {
	lines := strings.Split(content, "\n")
	if len(sections) == 0 {
		return len(lines)
	}

	// Use the hidden-set approach to correctly handle overlapping section ranges
	// (parent + child collapsed).
	hidden := make(map[int]bool)
	for _, sec := range sections {
		if collapsed[sec.key] && isMultiLineSection(sec) {
			for ln := sec.startLine + 1; ln <= sec.endLine; ln++ {
				hidden[ln] = true
			}
		}
	}

	// Count extra lines from split list items (isList && isBlockHeader).
	// A split adds one extra visible line when the listItem is visible and not collapsed.
	extra := 0
	for _, sec := range sections {
		if !sec.listItem {
			continue
		}
		// Check if this listItem has a companion section (making it a split case).
		hasSibling := false
		for _, other := range sections {
			if other.startLine == sec.startLine && !other.listItem {
				hasSibling = true
				break
			}
		}
		if hasSibling && isMultiLineSection(sec) && !hidden[sec.startLine] && !collapsed[sec.key] {
			extra++
		}
	}
	return len(lines) - len(hidden) + extra
}

// originalToVisible converts an original line index to the corresponding visible
// line index, accounting for collapsed sections. Returns -1 if the line is hidden.
func originalToVisible(origLine int, mapping []int) int {
	for i, m := range mapping {
		if m == origLine {
			return i
		}
	}
	return -1
}

// indentYAMLListItems re-indents YAML list items so they appear nested under
// their parent key. kubectl outputs lists at the same indent as the parent:
//
//	containers:
//	- name: nginx
//
// This function transforms it to:
//
//	containers:
//	  - name: nginx
//
// The transformation is applied by detecting section headers (key: with no
// inline value) followed by list items at the same indent, and adding 2 spaces
// to all lines in that list zone.
func indentYAMLListItems(content string) string {
	lines := strings.Split(content, "\n")
	extra := make([]int, len(lines))

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || trimmed == "---" {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			continue
		}
		indent := countIndent(line)

		colonIdx := strings.Index(trimmed, ":")
		if colonIdx <= 0 {
			continue
		}
		afterColon := strings.TrimSpace(trimmed[colonIdx+1:])
		if afterColon != "" {
			continue
		}

		// Section header with no value. Check if next content is a list item at same indent.
		listStart := -1
		for j := i + 1; j < len(lines); j++ {
			nextTrimmed := strings.TrimSpace(lines[j])
			if nextTrimmed == "" || strings.HasPrefix(nextTrimmed, "#") {
				continue
			}
			nextIndent := countIndent(lines[j])
			if nextIndent == indent && strings.HasPrefix(nextTrimmed, "- ") {
				listStart = j
			}
			break
		}
		if listStart < 0 {
			continue
		}

		// Mark all lines in this list zone with +2 indentation.
		for j := listStart; j < len(lines); j++ {
			jTrimmed := strings.TrimSpace(lines[j])
			if jTrimmed == "" || strings.HasPrefix(jTrimmed, "#") {
				extra[j] += 2
				continue
			}
			jIndent := countIndent(lines[j])
			if jIndent < indent {
				break
			}
			if jIndent == indent && !strings.HasPrefix(jTrimmed, "- ") {
				break
			}
			extra[j] += 2
		}
	}

	var result strings.Builder
	for i, line := range lines {
		if i > 0 {
			result.WriteByte('\n')
		}
		if extra[i] > 0 && strings.TrimSpace(line) != "" {
			result.WriteString(strings.Repeat(" ", extra[i]))
		}
		result.WriteString(line)
	}
	return result.String()
}

// countIndent returns the number of leading spaces in a line.
func countIndent(line string) int {
	n := 0
	for _, ch := range line {
		if ch == ' ' {
			n++
		} else {
			break
		}
	}
	return n
}
