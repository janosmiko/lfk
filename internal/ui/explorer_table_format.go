package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
)

// RenderContainerDetail renders detailed information about a container.
func RenderContainerDetail(item *model.Item, width, height int) string {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Bold(true).Background(BaseBg)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFile)).Background(BaseBg)

	type row struct {
		key   string
		value string
		style lipgloss.Style
	}
	rows := make([]row, 0, 10)

	rows = append(rows, row{"Name", item.Name, valueStyle})
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

	for _, kv := range item.Columns {
		if strings.HasPrefix(kv.Key, "__") || strings.HasPrefix(kv.Key, "owner:") || strings.HasPrefix(kv.Key, "secret:") || strings.HasPrefix(kv.Key, "data:") {
			continue
		}
		rows = append(rows, row{kv.Key, kv.Value, valueStyle})
	}

	maxKeyLen := 0
	for _, r := range rows {
		if len(r.key) > maxKeyLen {
			maxKeyLen = len(r.key)
		}
	}

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

// HighlightSearchInLine highlights search matches in a YAML line, applying
// the YAML syntax styling first and then overlaying the search highlight on
// the styled output.
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
func FormatItemNameOnly(item model.Item, width int) string {
	displayName := item.Name
	if item.Namespace != "" {
		displayName = item.Namespace + "/" + displayName
	}

	deprecationSuffix := ""
	deprecationW := 0
	if item.Deprecated {
		deprecationSuffix = DeprecationStyle.Render(" ⚠")
		deprecationW = lipgloss.Width(deprecationSuffix)
	}

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
func FormatItemNameOnlyPlain(item model.Item, width int) string {
	displayName := item.Name
	if item.Namespace != "" {
		displayName = item.Namespace + "/" + displayName
	}

	deprecationSuffix := ""
	deprecationW := 0
	if item.Deprecated {
		deprecationSuffix = " ⚠"
		deprecationW = lipgloss.Width(deprecationSuffix)
	}

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
