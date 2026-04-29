package ui

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
)

// RenderYAMLContent renders arbitrary YAML content with syntax highlighting, truncated to fit.
func RenderYAMLContent(content string, width, height int) string {
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	var b strings.Builder
	for i, line := range lines {
		b.WriteString(HighlightYAMLLine(Truncate(line, width)))
		if i < len(lines)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// RenderContainerDetail renders detailed information about a container.
func RenderContainerDetail(item *model.Item, width, height int) string {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Bold(true).Background(BaseBg)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFile)).Background(BaseBg)

	// Collect all rows as (key, value, style) tuples.
	type row struct {
		key   string
		value string
		style lipgloss.Style // value style override
	}
	rows := make([]row, 0, 10)

	rows = append(rows, row{"Name", item.Name, valueStyle})
	// Show container type if not a regular container.
	switch item.Category {
	case "Init Containers":
		rows = append(rows, row{"Type", "Init Container", DimStyle})
	case "Sidecar Containers":
		rows = append(rows, row{"Type", "Sidecar Container", DimStyle})
	}
	rows = append(rows, row{"Status", item.Status, StatusStyle(item.Status)})
	if item.Extra != "" {
		rows = append(rows, row{"Image", item.Extra, DimStyle})
	}
	if item.Ready != "" {
		rows = append(rows, row{"Ready", item.Ready, valueStyle})
	}
	if item.Restarts != "" {
		rows = append(rows, row{"Restarts", item.Restarts, valueStyle})
	}
	if age := LiveAge(*item); age != "" {
		rows = append(rows, row{"Age", age, AgeStyle(age)})
	}

	// Additional columns (reason, message, resources, ports, etc.).
	for _, kv := range item.Columns {
		if strings.HasPrefix(kv.Key, "__") || strings.HasPrefix(kv.Key, "owner:") || strings.HasPrefix(kv.Key, "secret:") || strings.HasPrefix(kv.Key, "data:") {
			continue
		}
		rows = append(rows, row{kv.Key, kv.Value, valueStyle})
	}

	// Find max key length for alignment.
	maxKeyLen := 0
	for _, r := range rows {
		if len(r.key) > maxKeyLen {
			maxKeyLen = len(r.key)
		}
	}

	// Render all rows with aligned labels.
	lines := make([]string, 0, len(rows)+3)
	lines = append(lines, DimStyle.Bold(true).Render("CONTAINER DETAILS"))
	lines = append(lines, "")
	for _, r := range rows {
		if len(lines) >= height-1 {
			break
		}
		padded := r.key + ": " + strings.Repeat(" ", maxKeyLen-len(r.key))
		lines = append(lines, labelStyle.Render(padded)+r.style.Render(r.value))
	}

	return strings.Join(lines, "\n")
}

// yamlNumberRe matches YAML numeric values: integers, floats, hex, octal,
// infinity, and NaN.
var yamlNumberRe = regexp.MustCompile(
	`^[+-]?(\d[\d_]*(\.\d[\d_]*)?(e[+-]?\d+)?` + // decimal / float / sci
		`|0x[\da-fA-F_]+` + // hex
		`|0o[0-7_]+` + // octal
		`|\.inf|\.Inf|\.INF` + // infinity
		`|\.nan|\.NaN|\.NAN)$`, // NaN
)

// yamlBoolValues contains all YAML boolean literals (1.1 + 1.2).
var yamlBoolValues = map[string]bool{
	"true": true, "false": true,
	"True": true, "False": true,
	"TRUE": true, "FALSE": true,
	"yes": true, "no": true,
	"Yes": true, "No": true,
	"YES": true, "NO": true,
	"on": true, "off": true,
	"On": true, "Off": true,
	"ON": true, "OFF": true,
}

// yamlNullValues contains all YAML null literals.
var yamlNullValues = map[string]bool{
	"null": true, "~": true, "Null": true, "NULL": true,
}

// yamlBlockScalarIndicators contains YAML block scalar indicators.
var yamlBlockScalarIndicators = map[string]bool{
	"|": true, ">": true, "|-": true, ">-": true, "|+": true, ">+": true,
}

// styleYAMLValue applies type-aware styling to a YAML value string.
func styleYAMLValue(val string) string {
	v := strings.TrimSpace(val)
	if v == "" {
		return YamlValueStyle.Render(val)
	}

	// Preserve leading whitespace from the original val.
	lead := val[:len(val)-len(strings.TrimLeft(val, " "))]

	switch {
	case yamlNullValues[v]:
		return lead + YamlNullStyle.Render(v)
	case yamlBoolValues[v]:
		return lead + YamlBoolStyle.Render(v)
	case isYAMLQuotedString(v):
		return lead + YamlStringStyle.Render(v)
	case strings.HasPrefix(v, "&") || strings.HasPrefix(v, "*"):
		return lead + YamlAnchorStyle.Render(v)
	case strings.HasPrefix(v, "!!") || strings.HasPrefix(v, "!"):
		return lead + YamlTagStyle.Render(v)
	case yamlBlockScalarIndicators[v]:
		return lead + YamlBlockScalarStyle.Render(v)
	case yamlNumberRe.MatchString(v):
		return lead + YamlNumberStyle.Render(v)
	}

	return lead + YamlStringStyle.Render(v)
}

// isYAMLQuotedString returns true if v is a single- or double-quoted string.
func isYAMLQuotedString(v string) bool {
	return (strings.HasPrefix(v, `"`) && strings.HasSuffix(v, `"`)) ||
		(strings.HasPrefix(v, "'") && strings.HasSuffix(v, "'"))
}

// isYAMLKey reports whether s looks like a valid YAML mapping key.
// It accepts unquoted identifiers (may contain alphanumerics, dashes, dots,
// slashes, underscores) and quoted keys ("key" or 'key').
func isYAMLKey(s string) bool {
	if s == "" {
		return false
	}
	// Quoted keys are always valid.
	if (s[0] == '"' && s[len(s)-1] == '"') ||
		(s[0] == '\'' && s[len(s)-1] == '\'') {
		return true
	}
	// Unquoted key: must not contain spaces.
	return !strings.Contains(s, " ")
}

// renderKeyValue renders a YAML key: value pair with syntax highlighting.
func renderKeyValue(indent, key, rest string) string {
	styledKey := YamlKeyStyle.Render(key)
	if len(rest) <= 1 {
		// Key with colon only (no value), e.g. "metadata:"
		return indent + styledKey + YamlPunctuationStyle.Render(":")
	}

	colon := YamlPunctuationStyle.Render(":")
	valPart := rest[1:] // skip the ":"

	// Handle inline comment after value.
	if ci := findInlineComment(valPart); ci >= 0 {
		return indent + styledKey + colon +
			styleYAMLValue(valPart[:ci]) +
			YamlCommentStyle.Render(valPart[ci:])
	}

	return indent + styledKey + colon + styleYAMLValue(valPart)
}

// HighlightYAMLLine applies syntax highlighting to a single YAML line.
func HighlightYAMLLine(line string) string {
	// Strip fold indicator characters (▾/▸) injected by the YAML fold system.
	// They appear inside the content when sections are at indent >= 2.
	// We strip them, highlight the YAML content, then prepend them back.
	var foldPrefix string
	cleaned := line
	for _, r := range line {
		if r == '▾' || r == '▸' {
			runes := []rune(line)
			for i, cr := range runes {
				if cr == '▾' || cr == '▸' {
					foldPrefix = string(runes[:i+1])
					cleaned = string(runes[i+1:])
					break
				}
			}
			break
		}
		if r != ' ' {
			break
		}
	}
	if foldPrefix != "" {
		return foldPrefix + highlightYAMLContent(cleaned)
	}
	return highlightYAMLContent(line)
}

// highlightYAMLContent applies syntax highlighting to YAML content (without
// fold indicators).
func highlightYAMLContent(line string) string {
	trimmed := strings.TrimLeft(line, " ")
	indent := line[:len(line)-len(trimmed)]

	// Comment lines.
	if strings.HasPrefix(trimmed, "#") {
		return YamlCommentStyle.Render(line)
	}

	// List items: "- ..." — handle dash prefix first so the dash is always
	// rendered as punctuation, then detect key: value inside the item.
	if strings.HasPrefix(trimmed, "- ") {
		marker := YamlPunctuationStyle.Render("- ")
		content := trimmed[2:]

		// List item with key: value, e.g. "- name: my-pod".
		if colonIdx := findYAMLColon(content); colonIdx > 0 {
			key := content[:colonIdx]
			rest := content[colonIdx:]
			if isYAMLKey(key) {
				return indent + marker + renderKeyValue("", key, rest)
			}
		}

		return indent + marker + styleYAMLValue(content)
	}

	// Lines with key: value.
	if colonIdx := findYAMLColon(trimmed); colonIdx > 0 {
		key := trimmed[:colonIdx]
		rest := trimmed[colonIdx:]
		if isYAMLKey(key) {
			return renderKeyValue(indent, key, rest)
		}
	}

	// Continuation / plain value lines (e.g. block scalar content, truncated
	// keys). Use neutral text color — actual string values are already handled
	// via styleYAMLValue in the key: value path above.
	return YamlValueStyle.Render(line)
}

// findYAMLColon finds the index of the first colon that looks like a YAML
// key-value separator. It must be followed by a space, end-of-string, or
// nothing (bare key). Colons inside quoted segments are skipped.
func findYAMLColon(s string) int {
	inSingle := false
	inDouble := false
	for i := range len(s) {
		switch s[i] {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case ':':
			if !inSingle && !inDouble {
				// Colon must be at end or followed by space to be a key separator.
				if i == len(s)-1 || s[i+1] == ' ' {
					return i
				}
			}
		}
	}
	return -1
}

// findInlineComment returns the index of an inline comment (# preceded by
// whitespace) in a YAML value, or -1 if none is found. It skips # inside
// quoted strings.
func findInlineComment(s string) int {
	inSingle := false
	inDouble := false
	for i := range len(s) {
		switch s[i] {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble && i > 0 && s[i-1] == ' ' {
				return i - 1
			}
		}
	}
	return -1
}

// HighlightSearchInLine highlights search matches in a YAML line, applying
// the YAML syntax styling first and then overlaying the search highlight on
// the styled output. Two passes are needed:
//
//  1. YAML syntax: keys/values/punctuation/comments each get their own SGR
//     pair. Without this, matched lines used to render as plain text — the
//     old code returned HighlightMatchStyled(line, ...) directly when a
//     match existed, bypassing HighlightYAMLLine entirely.
//
//  2. Search overlay via HighlightMatchInline: re-asserts the YAML token's
//     active open SGR after each highlight's reset so the rest of the token
//     after the match keeps its color. Without re-assertion the post-match
//     tail of the matched word dropped to terminal default — the user saw
//     "ngi" highlighted in yellow but "nx" rendered in plain white.
//
// When isCurrent is true, uses a more prominent style for the current match.
// Supports substring (the YAML preview's main path) plus regex/fuzzy via
// fallback to HighlightMatchStyled.
func HighlightSearchInLine(line, query string, isCurrent bool) string {
	styled := HighlightYAMLLine(line)
	if query == "" || !MatchLine(line, query) {
		return styled
	}
	highlight := SearchHighlightStyle
	if isCurrent {
		highlight = SelectedSearchHighlightStyle
	}
	return HighlightMatchInline(styled, query, highlight)
}

// FormatItemNameOnly formats an item showing only its name and icon (no status, age, etc.).
// Used for parent and child columns where details are not needed.
func FormatItemNameOnly(item model.Item, width int) string {
	displayName := item.Name
	if item.Namespace != "" {
		displayName = item.Namespace + "/" + displayName
	}

	// Build deprecation suffix (styled).
	deprecationSuffix := ""
	deprecationW := 0
	if item.Deprecated {
		deprecationSuffix = DeprecationStyle.Render(" ⚠")
		deprecationW = lipgloss.Width(deprecationSuffix)
	}

	// Read-only marker is rendered as a styled "[RO] " prefix (after the
	// "* " star, before the icon) so it sits like a tag next to the name
	// and won't wrap to a new line at narrow column widths.
	roPrefix := readOnlyPrefix(item)
	roPrefixW := lipgloss.Width(roPrefix)

	resolvedIcon := resolveIcon(item.Icon)

	if item.Status == "current" {
		prefix := CurrentMarkerStyle.Render("* ")
		prefixW := lipgloss.Width(prefix)
		if resolvedIcon != "" {
			icon := IconStyle.Render(resolvedIcon + " ")
			iconW := lipgloss.Width(icon)
			remaining := max(width-prefixW-roPrefixW-iconW-deprecationW, 1)
			return prefix + roPrefix + icon + NormalStyle.Render(Truncate(displayName, remaining)) + deprecationSuffix
		}
		remaining := max(width-prefixW-roPrefixW-deprecationW, 1)
		return prefix + roPrefix + NormalStyle.Render(Truncate(displayName, remaining)) + deprecationSuffix
	}

	if resolvedIcon != "" {
		icon := IconStyle.Render(resolvedIcon + " ")
		iconW := lipgloss.Width(icon)
		remaining := max(width-roPrefixW-iconW-deprecationW, 1)
		return roPrefix + icon + NormalStyle.Render(Truncate(displayName, remaining)) + deprecationSuffix
	}

	remaining := max(width-roPrefixW-deprecationW, 1)
	return roPrefix + NormalStyle.Render(Truncate(displayName, remaining)) + deprecationSuffix
}

// FormatItemNameOnlyPlain formats an item showing only name and icon, without ANSI styling.
// Used for highlighted items in parent/child columns.
func FormatItemNameOnlyPlain(item model.Item, width int) string {
	displayName := item.Name
	if item.Namespace != "" {
		displayName = item.Namespace + "/" + displayName
	}

	// Build deprecation suffix (plain text).
	deprecationSuffix := ""
	deprecationW := 0
	if item.Deprecated {
		deprecationSuffix = " ⚠"
		deprecationW = lipgloss.Width(deprecationSuffix)
	}

	// Plain "[RO] " prefix — no ANSI styling so the outer selection
	// background renders cleanly.
	roPrefix := readOnlyPrefixPlain(item)
	roPrefixW := lipgloss.Width(roPrefix)

	resolvedIcon := resolveIcon(item.Icon)

	if item.Status == "current" {
		prefix := "* "
		prefixW := len(prefix)
		if resolvedIcon != "" {
			icon := resolvedIcon + " "
			iconW := lipgloss.Width(icon)
			remaining := max(width-prefixW-roPrefixW-iconW-deprecationW, 1)
			return prefix + roPrefix + icon + Truncate(displayName, remaining) + deprecationSuffix
		}
		remaining := max(width-prefixW-roPrefixW-deprecationW, 1)
		return prefix + roPrefix + Truncate(displayName, remaining) + deprecationSuffix
	}

	if resolvedIcon != "" {
		icon := resolvedIcon + " "
		iconW := lipgloss.Width(icon)
		remaining := max(width-roPrefixW-iconW-deprecationW, 1)
		return roPrefix + icon + Truncate(displayName, remaining) + deprecationSuffix
	}

	remaining := max(width-roPrefixW-deprecationW, 1)
	return roPrefix + Truncate(displayName, remaining) + deprecationSuffix
}

// wrapExtraValue splits a value into continuation-line chunks of the given width.
// Retained for test compatibility. No longer called by production code.
func wrapExtraValue(val string, width int) []string {
	if width <= 0 {
		return nil
	}
	runes := []rune(val)
	if len(runes) <= width {
		return nil
	}
	var lines []string
	for i := width; i < len(runes); i += width {
		end := min(i+width, len(runes))
		lines = append(lines, string(runes[i:end]))
	}
	return lines
}

// itemExtraLines returns how many continuation lines an item needs.
// Line wrapping has been removed; every row is exactly one line.
func itemExtraLines(_ *model.Item, _ []extraColumn) int {
	return 0
}

// isBuiltinColumnKey reports whether key is one of the five mandatory
// built-in column keys. Extras sharing a built-in name are never surfaced
// (the built-in always wins — matches the existing ActiveSortableColumns
// de-dup precedent).
func isBuiltinColumnKey(key string) bool {
	switch key {
	case "Namespace", "Ready", "Restarts", "Status", "Age":
		return true
	}
	return false
}

// orderedColumnKeys returns the ordered list of column keys (excluding "Name")
// that RenderTable should emit for a middle-column render. It honors
// ActiveColumnOrder when set (gated on ActiveMiddleScroll >= 0), falling back
// to the default layout [Namespace, Ready, Restarts, Status, extras..., Age]
// otherwise. Keys whose columns are not currently visible (hasX=false or not
// in extraCols) are dropped. Keys not referenced by the saved order are
// appended at their default position so newly-discovered extras still show up.
func orderedColumnKeys(hasNs, hasReady, hasRestarts, hasStatus, hasAge bool, extraCols []extraColumn) []string {
	defaults := make([]string, 0, 5+len(extraCols))
	if hasNs {
		defaults = append(defaults, "Namespace")
	}
	if hasReady {
		defaults = append(defaults, "Ready")
	}
	if hasRestarts {
		defaults = append(defaults, "Restarts")
	}
	if hasStatus {
		defaults = append(defaults, "Status")
	}
	for _, ec := range extraCols {
		defaults = append(defaults, ec.key)
	}
	if hasAge {
		defaults = append(defaults, "Age")
	}

	if ActiveMiddleScroll < 0 || ActiveColumnOrder == nil {
		return defaults
	}

	// Build a "visible" set so stale entries in the saved order are dropped.
	visible := make(map[string]bool, len(defaults))
	for _, k := range defaults {
		visible[k] = true
	}

	seen := make(map[string]bool, len(defaults))
	ordered := make([]string, 0, len(defaults))

	// Apply the user's saved order first, dropping stale or hidden keys.
	for _, k := range ActiveColumnOrder {
		if !visible[k] || seen[k] {
			continue
		}
		ordered = append(ordered, k)
		seen[k] = true
	}
	// Append any visible keys the saved order didn't mention, in default slot.
	for _, k := range defaults {
		if !seen[k] {
			ordered = append(ordered, k)
			seen[k] = true
		}
	}
	return ordered
}

// widthForColumnKey returns the precomputed width for a given column key.
// Built-in keys use their dedicated width variables; extra keys are looked
// up in extraCols. Returns 0 for unknown keys (should not happen in practice).
func widthForColumnKey(key string, nsW, readyW, restartsW, statusW, ageW int, extraCols []extraColumn) int {
	switch key {
	case "Namespace":
		return nsW
	case "Ready":
		return readyW
	case "Restarts":
		return restartsW
	case "Status":
		return statusW
	case "Age":
		return ageW
	}
	for _, ec := range extraCols {
		if ec.key == key {
			return ec.width
		}
	}
	return 0
}

// headerCellForKey returns the pre-styled header cell string for a single
// column key, reusing the already-built headers for built-in columns and
// calling headerWithIndicator for extras.
func headerCellForKey(key string, extraCols []extraColumn,
	nsHeader, readyHeader, rsHeader, statusHeader, ageHeader string,
) string {
	switch key {
	case "Namespace":
		return nsHeader
	case "Ready":
		return readyHeader
	case "Restarts":
		return rsHeader
	case "Status":
		return statusHeader
	case "Age":
		return ageHeader
	}
	for _, ec := range extraCols {
		if ec.key == key {
			return headerWithIndicator(columnHeaderLabel(ec.key), ec.key, ec.width)
		}
	}
	return ""
}

// RenderTable renders items in a table format with column headers for resource views.
// headerLabel is used as the first column header; defaults to "NAME" if empty.
func RenderTable(headerLabel string, items []model.Item, cursor int, width, height int, loading bool, spinnerView string, errMsg string, showMarker ...bool) string { //nolint:gocyclo // rendering function with inherent layout complexity
	var b strings.Builder

	if len(items) == 0 {
		switch {
		case loading:
			b.WriteString(DimStyle.Render(spinnerView+" ") + DimStyle.Render("Loading..."))
		case errMsg != "":
			b.WriteString(ErrorStyle.Render(Truncate(errMsg, width)))
		default:
			b.WriteString(DimStyle.Render("No resources found"))
		}
		return b.String()
	}

	var hasNs, hasReady, hasRestarts, hasAge, hasStatus bool
	var nsW, readyW, restartsW, ageW, statusW int
	var anyRecentRestart bool

	if ActiveTableLayout != nil && ActiveTableLayout.Computed {
		hasNs = ActiveTableLayout.HasNs
		hasReady = ActiveTableLayout.HasReady
		hasRestarts = ActiveTableLayout.HasRestarts
		hasAge = ActiveTableLayout.HasAge
		hasStatus = ActiveTableLayout.HasStatus
		nsW = ActiveTableLayout.NsW
		readyW = ActiveTableLayout.ReadyW
		restartsW = ActiveTableLayout.RestartsW
		ageW = ActiveTableLayout.AgeW
		statusW = ActiveTableLayout.StatusW
		anyRecentRestart = ActiveTableLayout.AnyRecentRestart
	} else {
		// Detect which detail columns have data.
		for _, item := range items {
			if item.Namespace != "" {
				hasNs = true
			}
			if item.Ready != "" {
				hasReady = true
			}
			if item.Restarts != "" {
				hasRestarts = true
			}
			if item.Age != "" {
				hasAge = true
			}
			if item.Status != "" {
				hasStatus = true
			}
		}

		// Apply user-chosen built-in column suppression from the column toggle
		// overlay. Only the middle column honors this — child/right previews do
		// not use ActiveHiddenBuiltinColumns so their layout stays stable.
		if ActiveMiddleScroll >= 0 && ActiveHiddenBuiltinColumns != nil {
			if ActiveHiddenBuiltinColumns["Namespace"] {
				hasNs = false
			}
			if ActiveHiddenBuiltinColumns["Ready"] {
				hasReady = false
			}
			if ActiveHiddenBuiltinColumns["Restarts"] {
				hasRestarts = false
			}
			if ActiveHiddenBuiltinColumns["Age"] {
				hasAge = false
			}
			if ActiveHiddenBuiltinColumns["Status"] {
				hasStatus = false
			}
		}

		// Content-aware column widths: size each column based on actual data,
		// then give all remaining space to the name column so the table fills
		// the full available width.
		if hasNs {
			nsW = len("NAMESPACE") // minimum: header width
			for _, item := range items {
				if w := len(item.Namespace); w > nsW {
					nsW = w
				}
			}
			nsW++ // spacing
			if nsW > 30 {
				nsW = 30
			}
		}
		if hasReady {
			readyW = len("READY") // 5
			for _, item := range items {
				if w := len(item.Ready); w > readyW {
					readyW = w
				}
			}
			readyW++ // spacing
		}
		// Check if any item has a recent restart — if so, reserve arrow space for all.
		if hasRestarts {
			restartsW = len("RS") + 1 // 3
			for _, item := range items {
				if rc, _ := strconv.Atoi(item.Restarts); rc > 0 {
					if !item.LastRestartAt.IsZero() && time.Since(item.LastRestartAt) < time.Hour {
						anyRecentRestart = true
						break
					}
				}
			}
			for _, item := range items {
				w := len(item.Restarts)
				if anyRecentRestart {
					w++ // reserve space for "↑" indicator (or placeholder)
				}
				if w >= restartsW {
					restartsW = w + 1
				}
			}
		}
		if hasAge {
			ageW = len("AGE") + 1 // 4
			for _, item := range items {
				if w := len(LiveAge(item)); w >= ageW {
					ageW = w + 1
				}
			}
			if ageW > 10 {
				ageW = 10
			}
		}
		if hasStatus {
			statusW = len("STATUS") // minimum for header
			for _, item := range items {
				if w := len(item.Status); w > statusW {
					statusW = w
				}
			}
			statusW++ // spacing
			if statusW > 20 {
				statusW = 20
			}
		}
	}

	// Prefer keeping the full Name visible over a content-sized Namespace.
	// Pods can have long names (5-suffix generated, plus deployment template
	// hashes) and a single 28-char namespace would otherwise burn ~29 cols on
	// NAMESPACE while the name column truncates. Shrink nsW down toward its
	// header width when doing so keeps the longest name from being cut.
	if hasNs && (ActiveTableLayout == nil || !ActiveTableLayout.Computed) {
		longestName := 0
		for _, item := range items {
			if w := len(item.Name); w > longestName {
				longestName = w
			}
		}
		// Marker col is reserved later but also counts toward the layout
		// budget — include it so the floor matches what nameW will see.
		markerW := 0
		if len(showMarker) == 0 || showMarker[0] {
			markerW = 2
		}
		fixedOther := readyW + restartsW + ageW + statusW + markerW
		nsHeaderW := len("NAMESPACE") + 1
		// Largest nsW that still leaves room for the longest name (+1
		// spacing) without truncation, floored at the NAMESPACE header.
		targetNs := max(width-fixedOther-(longestName+1), nsHeaderW)
		if targetNs < nsW {
			nsW = targetNs
		}
	}
	// Same idea for STATUS: when the width budget is tight enough that
	// names would still truncate even after the namespace shrink above,
	// fall back to abbreviated status labels (PodInitializing → Init,
	// Succeeded → Done) so the saved cells flow into NAME instead of
	// being burned on long status strings the layout can't drop. Skipped
	// when no status value would actually shrink, since recomputing here
	// would otherwise undo a wider cap users had set explicitly.
	if hasStatus && (ActiveTableLayout == nil || !ActiveTableLayout.Computed) {
		longestName := 0
		for _, item := range items {
			if w := len(item.Name); w > longestName {
				longestName = w
			}
		}
		markerW := 0
		if len(showMarker) == 0 || showMarker[0] {
			markerW = 2
		}
		// What's the smallest STATUS width if every value used its
		// abbreviation? The header floor still applies.
		abbrevMaxW := len("STATUS")
		willShrinkAny := false
		for _, item := range items {
			abbr := AbbreviateStatusForWidth(item.Status, 0)
			if abbr != item.Status {
				willShrinkAny = true
			}
			if w := len(abbr); w > abbrevMaxW {
				abbrevMaxW = w
			}
		}
		abbrevStatusW := abbrevMaxW + 1 // matches the +1 spacing applied above
		if willShrinkAny && abbrevStatusW < statusW {
			fixedOther := readyW + restartsW + ageW + markerW
			minNsW := 0
			if hasNs {
				minNsW = min(len("NAMESPACE")+1, nsW)
			}
			if width-fixedOther-statusW-minNsW-(longestName+1) < 0 {
				statusW = abbrevStatusW
			}
		}
	}
	// Reserve space for the selection marker column in the focus pane
	// so the table doesn't shift when selections are made.
	wantMarker := len(showMarker) == 0 || showMarker[0]
	markerColW := 0
	if wantMarker {
		markerColW = 2
	}

	var extraCols []extraColumn
	if ActiveTableLayout != nil && ActiveTableLayout.Computed {
		extraCols = ActiveTableLayout.ExtraCols
	} else {
		// Collect extra columns from item data (additionalPrinterColumns).
		tableKind := ""
		if len(items) > 0 {
			tableKind = items[0].Kind
		}
		extraCols = collectExtraColumns(items, width, nsW+readyW+restartsW+ageW+statusW+markerColW, tableKind)

		// Drop any extras that collide with a built-in key so the built-in always
		// wins (matches the existing ActiveSortableColumns de-dup precedent) and
		// keeps column-order serialization simple (bare keys, no disambiguation).
		filtered := extraCols[:0]
		for _, ec := range extraCols {
			if !isBuiltinColumnKey(ec.key) {
				filtered = append(filtered, ec)
			}
		}
		extraCols = filtered

		// Populate the layout cache so subsequent renders skip the O(N) work.
		if ActiveTableLayout != nil {
			ActiveTableLayout.HasNs = hasNs
			ActiveTableLayout.HasReady = hasReady
			ActiveTableLayout.HasRestarts = hasRestarts
			ActiveTableLayout.HasAge = hasAge
			ActiveTableLayout.HasStatus = hasStatus
			ActiveTableLayout.NsW = nsW
			ActiveTableLayout.ReadyW = readyW
			ActiveTableLayout.RestartsW = restartsW
			ActiveTableLayout.AgeW = ageW
			ActiveTableLayout.StatusW = statusW
			ActiveTableLayout.AnyRecentRestart = anyRecentRestart
			ActiveTableLayout.ExtraCols = extraCols
			ActiveTableLayout.Computed = true
		}
	}

	// Populate ActiveExtraColumnKeys for the column toggle overlay.
	// Only the active middle column is authoritative — child/right table
	// renders must not overwrite this or the overlay would show stale data.
	if ActiveMiddleScroll >= 0 {
		ActiveExtraColumnKeys = ActiveExtraColumnKeys[:0]
		for _, ec := range extraCols {
			ActiveExtraColumnKeys = append(ActiveExtraColumnKeys, ec.key)
		}
	}

	// Build the ordered list of non-Name column keys. This honors any saved
	// ActiveColumnOrder for the current kind when rendering the middle column,
	// falling back to the default layout for child/right renders.
	order := orderedColumnKeys(hasNs, hasReady, hasRestarts, hasStatus, hasAge, extraCols)

	// Build sortable column list only for the middle column (not children/right column).
	// ActiveMiddleScroll >= 0 indicates this is the active middle column render.
	if ActiveMiddleScroll >= 0 {
		ActiveSortableColumns = ActiveSortableColumns[:0]
		ActiveSortableColumns = append(ActiveSortableColumns, "Name")
		ActiveSortableColumns = append(ActiveSortableColumns, order...)
		ActiveSortableColumnCount = len(ActiveSortableColumns)
		// Derive ActiveSortColumn index from the name now that columns are built.
		ActiveSortColumn = 0
		for i, col := range ActiveSortableColumns {
			if col == ActiveSortColumnName {
				ActiveSortColumn = i
				break
			}
		}
	}

	// Calculate total extra column width.
	extraTotalW := 0
	for _, ec := range extraCols {
		extraTotalW += ec.width
	}

	// Name column gets all remaining space so the table fills the full width.
	nameW := max(width-nsW-readyW-restartsW-ageW-statusW-markerColW-extraTotalW, 10)
	// Cap name column when names are short (e.g., CVE IDs ~15 chars).
	// Redistribute the surplus to the last extra column (typically Title
	// or the most descriptive column) so it can show more content instead
	// of padding the name with whitespace.
	if len(items) > 0 && len(extraCols) > 0 {
		maxName := len(headerLabel)
		for i := range items {
			n := len(items[i].Name)
			if badge := securityBadgePlainForItem(&items[i]); badge != "" {
				n += len(badge) + 1
			}
			if n > maxName {
				maxName = n
			}
		}
		maxName += 3 // padding
		if nameW > maxName && maxName > 10 {
			surplus := nameW - maxName
			nameW = maxName
			// Give all surplus to the last extra column (the widest /
			// most descriptive one, e.g., Title).
			extraCols[len(extraCols)-1].width += surplus
		}
	}

	// Render header with sort indicators fitted within column widths.
	if headerLabel == "" {
		headerLabel = "NAME"
	}
	nameHeader := headerWithIndicator(headerLabel, "Name", nameW)
	nsHeader := headerWithIndicator("NAMESPACE", "Namespace", nsW)
	readyHeader := headerWithIndicator("READY", "Ready", readyW)
	rsHeader := headerWithIndicator("RS", "Restarts", restartsW)
	statusHeader := headerWithIndicator("STATUS", "Status", statusW)
	ageHeader := headerWithIndicator("AGE", "Age", ageW)

	// Build header row using the ordered walk. Name is always first (after
	// any marker column). Built-ins use their pre-built headers; extras are
	// built on demand via headerCellForKey.
	var hdrParts []string
	if wantMarker {
		hdrParts = append(hdrParts, "  ")
	}
	hdrParts = append(hdrParts, nameHeader)
	for _, key := range order {
		hdrParts = append(hdrParts, headerCellForKey(key, extraCols, nsHeader, readyHeader, rsHeader, statusHeader, ageHeader))
	}
	hdr := strings.Join(hdrParts, "")
	b.WriteString(DimStyle.Bold(true).Render(Truncate(hdr, width)))
	height-- // header takes one line

	// Record the byte ranges of each rendered column so mouse handling can
	// map a click X coordinate to a column key. Only populated for the
	// active middle render.
	if ActiveMiddleScroll >= 0 {
		ActiveMiddleColumnLayout = ActiveMiddleColumnLayout[:0]
		x := 0
		if wantMarker {
			x += markerColW
		}
		ActiveMiddleColumnLayout = append(ActiveMiddleColumnLayout, MiddleColumnRegion{Key: "Name", StartX: x, EndX: x + nameW})
		x += nameW
		for _, key := range order {
			w := widthForColumnKey(key, nsW, readyW, restartsW, statusW, ageW, extraCols)
			ActiveMiddleColumnLayout = append(ActiveMiddleColumnLayout, MiddleColumnRegion{Key: key, StartX: x, EndX: x + w})
			x += w
		}
	}

	// Detect category transitions for category headers.
	hasCategories := false
	categoryForItem := make([]string, len(items)) // category header to show before item i, or ""
	hasSepForItem := make([]bool, len(items))     // true if a blank separator row precedes item i's category header
	{
		lastCat := ""
		for i, item := range items {
			if item.Category != "" && item.Category != lastCat {
				categoryForItem[i] = item.Category
				if lastCat != "" {
					hasCategories = true // at least 2 different categories
					hasSepForItem[i] = true
				}
				lastCat = item.Category
			}
		}
		// Only show category headers if there are multiple distinct categories.
		if !hasCategories {
			for i := range categoryForItem {
				categoryForItem[i] = ""
				hasSepForItem[i] = false
			}
		}
	}

	// Count extra lines consumed by category headers and blank
	// separator rows. The separator is elided for the first visible
	// entry so the viewport doesn't start with a leading empty row
	// when the user scrolls the list tail into view.
	categoryLines := func(start, end int) int {
		n := 0
		for i := start; i < end && i < len(items); i++ {
			if categoryForItem[i] != "" {
				n++ // category header line
			}
			if hasSepForItem[i] && i > start {
				n++ // blank separator row above header
			}
		}
		return n
	}

	// Display lines for a range of items (each item = 1 line + category headers).
	tableDisplayLines := func(from, to int) int {
		return (to - from) + categoryLines(from, to)
	}

	// Scrolling: use vim-style scrolloff for stable viewport.
	scrollOff := ConfigScrollOff
	startIdx := 0
	if ActiveMiddleScroll >= 0 {
		startIdx = VimScrollOff(ActiveMiddleScroll, cursor, len(items), height, scrollOff, tableDisplayLines)
		ActiveMiddleScroll = startIdx
	} else {
		// Fallback: calculate from scratch (old behavior).
		totalDisplayLines := tableDisplayLines(0, len(items))
		if totalDisplayLines <= height {
			scrollOff = 0
		} else if maxSO := (height - 1) / 2; scrollOff > maxSO {
			scrollOff = maxSO
		}
		if cursor >= 0 {
			displayLinesUpTo := func(start, idx int) int {
				return tableDisplayLines(start, idx+1)
			}
			for startIdx < len(items) && displayLinesUpTo(startIdx, cursor) > height {
				startIdx++
			}
			if cursor+scrollOff < len(items) {
				for startIdx < len(items) && displayLinesUpTo(startIdx, cursor+scrollOff) > height {
					startIdx++
				}
			}
			if cursor-scrollOff >= 0 && startIdx > cursor-scrollOff {
				startIdx = max(cursor-scrollOff, 0)
			}
			// Check the new position BEFORE decrementing so we
			// don't overshoot when an earlier row has 2 display
			// lines (category header + item).
			for startIdx > 0 {
				if tableDisplayLines(startIdx-1, len(items)) > height {
					break
				}
				startIdx--
			}
		}
	}

	// Determine endIdx that fits within height.
	usedLines := 0
	endIdx := startIdx
	for endIdx < len(items) {
		extraLines := 0
		if categoryForItem[endIdx] != "" {
			extraLines++ // category header line
		}
		if hasSepForItem[endIdx] && endIdx > startIdx {
			extraLines++ // blank separator row (elided for first visible entry)
		}
		if usedLines+1+extraLines > height {
			break
		}
		usedLines += 1 + extraLines
		endIdx++
	}

	// Build display-line-to-item map for mouse click handling.
	// Only build when rendering the active middle column (ActiveMiddleScroll >= 0).
	if ActiveMiddleScroll >= 0 {
		ActiveMiddleLineMap = ActiveMiddleLineMap[:0]
		for i := startIdx; i < endIdx; i++ {
			if hasSepForItem[i] && i > startIdx {
				ActiveMiddleLineMap = append(ActiveMiddleLineMap, -1) // separator
			}
			if hasCategories && categoryForItem[i] != "" {
				ActiveMiddleLineMap = append(ActiveMiddleLineMap, -1) // category header
			}
			ActiveMiddleLineMap = append(ActiveMiddleLineMap, i) // item line
		}
	}

	for i := startIdx; i < endIdx; i++ {
		item := items[i]

		// Blank separator row above the category header, elided for
		// the first visible entry so the viewport never starts with
		// a leading empty row.
		if hasSepForItem[i] && i > startIdx {
			b.WriteString("\n")
		}

		// Render category header if this item starts a new category.
		// Full-width bar with strong background, plus the blank row
		// above it, makes groups visually obvious.
		if hasCategories && categoryForItem[i] != "" {
			headerLine := Truncate(categoryForItem[i], width)
			if w := lipgloss.Width(headerLine); w < width {
				headerLine += strings.Repeat(" ", width-w)
			}
			b.WriteString("\n" + CategoryBarStyle.Render(headerLine))
		}

		b.WriteString("\n")

		ns := item.Namespace
		if ns == "" && hasNs {
			ns = "-"
		}

		displayName := item.Name
		if icon := resolveIcon(item.Icon); icon != "" {
			displayName = icon + " " + item.Name
		}

		selected := isItemSelected(item)

		if i == cursor {
			// Selection marker (plain text for cursor row).
			markerPrefix := ""
			if wantMarker {
				markerPrefix = "  "
				if selected {
					markerPrefix = selectionMarker
				}
			}
			// Preprocess restarts value to match styled rendering:
			// add "↑" prefix for recent restarts, or " " placeholder when
			// other items have recent restarts (so the column aligns).
			cursorRestarts := item.Restarts
			if hasRestarts {
				restartCount, _ := strconv.Atoi(item.Restarts)
				recentRestart := !item.LastRestartAt.IsZero() && time.Since(item.LastRestartAt) < time.Hour
				if restartCount > 0 && recentRestart {
					cursorRestarts = "↑" + item.Restarts
				} else if anyRecentRestart {
					cursorRestarts = " " + item.Restarts
				}
			}
			// Selected row: plain text, no inner styles.
			row := markerPrefix + formatTableRowOrdered(displayName, ns, item.Ready, cursorRestarts, item.Status, LiveAge(item),
				nameW, nsW, readyW, restartsW, statusW, ageW, order, extraCols, &item)
			highlighted := false
			// Apply search/filter highlight on the selected row with
			// contrasting style. Pass the cursor-row outer style so
			// the inner highlight's reset doesn't kill the cursor bg
			// for the post-match part of the row.
			if ActiveHighlightQuery != "" {
				row = highlightNameSelectedOver(row, ActiveHighlightQuery, ActiveSelectedStyle(i))
				highlighted = true
			}
			// Pad to full width for clean highlight.
			lineW := lipgloss.Width(row)
			if lineW < width {
				row += strings.Repeat(" ", width-lineW)
			}
			if highlighted {
				// Avoid lipgloss.Render fragmenting embedded inner
				// highlight ANSI per-character — see RenderOverPrestyled.
				b.WriteString(RenderOverPrestyled(row, ActiveSelectedStyle(i)))
			} else {
				b.WriteString(ActiveSelectedStyle(i).MaxWidth(width).Render(row))
			}
		} else {
			var rendered string
			if ActiveRowCache != nil {
				rendered = ActiveRowCache[i]
			}
			if rendered == "" {
				markerPrefix := ""
				if wantMarker {
					markerPrefix = "  "
					if selected {
						markerPrefix = SelectionMarkerStyle.Render(selectionMarker)
					}
				}
				rendered = markerPrefix + formatTableRowStyledOrdered(item, nameW, nsW, readyW, restartsW, statusW, ageW,
					order, extraCols, anyRecentRestart)
				if ActiveRowCache != nil {
					ActiveRowCache[i] = rendered
				}
			}
			b.WriteString(rendered)
		}

	}
	return b.String()
}
