package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
)

// RenderResourceSummary renders a table-style detail summary of resource fields,
// followed by truncated YAML if space permits. It shows metadata that is NOT
// already visible in the middle column table (Name, Ready, Restarts, Status, Age).
func RenderResourceSummary(item *model.Item, yaml string, width, height int) string {
	if item == nil || len(item.Columns) == 0 {
		// No summary data, fall back to YAML.
		if yaml != "" {
			return RenderYAMLContent(yaml, width, height)
		}
		return DimStyle.Render("No preview")
	}

	var lines []string

	// Skip metrics keys and keys already shown as standard table columns.
	metricsKeys := map[string]bool{
		"CPU": true, "CPU/R": true, "CPU/L": true,
		"MEM": true, "MEM/R": true, "MEM/L": true,
		"CPU%": true, "MEM%": true,
		"CPU Req": true, "CPU Lim": true, "Mem Req": true, "Mem Lim": true,
		"CPU Alloc": true, "Mem Alloc": true,
	}
	tableKeys := map[string]bool{
		"Status": true, "Reason": true,
	}

	// Collect fields into categorized buckets for ordered rendering.
	type detailRow struct {
		key   string
		value string
	}

	// Category buckets.
	var statusRows []detailRow  // Status/Health/Sync/Phase/Ready/AutoSync/Suspended/Condition
	var syncRows []detailRow    // Last Sync, Synced At, Sync Message, Sync Errors, Revision
	var specRows []detailRow    // Dest NS, Dest Server, Repo, Path, Type, etc.
	var messageRows []detailRow // Reason, Message, Health Message
	var multiLineFields []model.KeyValue
	var dataLines []string

	// Key sets for categorization.
	statusKeys := map[string]bool{
		"Ready": true, "Health": true, "Sync": true, "Phase": true,
		"AutoSync": true, "Suspended": true, "Condition": true,
	}
	syncKeys := map[string]bool{
		"Last Sync": true, "Synced At": true, "Sync Message": true,
		"Sync Errors": true, "Revision": true,
	}
	messageKeys := map[string]bool{
		"Reason": true, "Message": true, "Health Message": true,
	}

	for _, kv := range item.Columns {
		if metricsKeys[kv.Key] || tableKeys[kv.Key] {
			continue
		}
		if strings.HasPrefix(kv.Key, "secret:") {
			label := kv.Key[len("secret:"):]
			if ActiveShowSecretValues {
				dataLines = append(dataLines, renderDataKV(label, kv.Value, width)...)
			} else {
				dataLines = append(dataLines, renderDataKV(label, "********", width)...)
			}
			continue
		}
		if strings.HasPrefix(kv.Key, "data:") {
			label := kv.Key[len("data:"):]
			dataLines = append(dataLines, renderDataKV(label, kv.Value, width)...)
			continue
		}
		if strings.HasPrefix(kv.Key, "condition:") {
			label := kv.Key[len("condition:"):]
			statusRows = append(statusRows, detailRow{strings.ToUpper(label), kv.Value})
			continue
		}
		if kv.Key == "Labels" || kv.Key == "Finalizers" || kv.Key == "Annotations" || kv.Key == "Used By" || kv.Key == "Selector" || kv.Key == "Taints" {
			multiLineFields = append(multiLineFields, kv)
			continue
		}
		row := detailRow{strings.ToUpper(kv.Key), kv.Value}
		switch {
		case statusKeys[kv.Key]:
			statusRows = append(statusRows, row)
		case syncKeys[kv.Key]:
			syncRows = append(syncRows, row)
		case messageKeys[kv.Key]:
			messageRows = append(messageRows, row)
		default:
			specRows = append(specRows, row)
		}
	}

	// Build ordered rows:
	// 1. Identity (Name, Namespace, Deletion)
	// 2. Multi-line fields (Labels, Annotations, Finalizers, Selector, Used By)
	//    — rendered separately below
	// 3. Status/Health
	// 4. Sync/Operation details
	// 5. Spec fields
	// 6. Messages/Reasons
	var identityRows []detailRow
	if item.Name != "" {
		identityRows = append(identityRows, detailRow{"NAME", item.Name})
	}
	if item.Namespace != "" {
		identityRows = append(identityRows, detailRow{"NAMESPACE", item.Namespace})
	}
	if item.Deleting {
		// Find deletion timestamp from columns.
		for _, kv := range item.Columns {
			if kv.Key == "Deletion" || kv.Key == "DELETION" {
				identityRows = append(identityRows, detailRow{"DELETION", kv.Value})
				break
			}
		}
	}

	allRows := make([]detailRow, 0, len(identityRows)+len(statusRows)+len(syncRows)+len(specRows)+len(messageRows))
	allRows = append(allRows, identityRows...)
	allRows = append(allRows, statusRows...)
	allRows = append(allRows, syncRows...)
	allRows = append(allRows, specRows...)
	allRows = append(allRows, messageRows...)

	// Calculate key column width for table alignment.
	keyW := 0
	for _, r := range allRows {
		if len(r.key) > keyW {
			keyW = len(r.key)
		}
	}
	keyW += 2

	// Style for detail keys: bold + primary, no underline (HeaderStyle has underline).
	detailKeyStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorPrimary)).Background(BaseBg)

	// Render identity rows.
	for _, r := range identityRows {
		if len(lines) >= height-2 {
			break
		}
		valW := max(width-keyW-2, 4)
		val := r.value
		if len(val) > valW {
			val = val[:valW-3] + "..."
		}
		keyStr := detailKeyStyle.Render(fmt.Sprintf("%-*s", keyW, r.key))
		if r.key == "DELETION" {
			lines = append(lines, keyStr+ErrorStyle.Render(val))
		} else {
			lines = append(lines, keyStr+DimStyle.Render(val))
		}
	}

	// Render status and sync rows first.
	for _, r := range statusRows {
		if len(lines) >= height-2 {
			break
		}
		valW := max(width-keyW-2, 4)
		val := r.value
		if len(val) > valW {
			val = val[:valW-3] + "..."
		}
		keyStr := detailKeyStyle.Render(fmt.Sprintf("%-*s", keyW, r.key))
		lines = append(lines, keyStr+DimStyle.Render(val))
	}
	for _, r := range syncRows {
		if len(lines) >= height-2 {
			break
		}
		valW := max(width-keyW-2, 4)
		val := r.value
		if len(val) > valW {
			val = val[:valW-3] + "..."
		}
		keyStr := detailKeyStyle.Render(fmt.Sprintf("%-*s", keyW, r.key))
		lines = append(lines, keyStr+DimStyle.Render(val))
	}

	// Render multi-line fields in order: Labels, Annotations, Finalizers,
	// then Selector, Used By.
	multiOrder := []string{"Labels", "Annotations", "Finalizers", "Taints", "Selector", "Used By"}
	multiMap := make(map[string]model.KeyValue, len(multiLineFields))
	for _, kv := range multiLineFields {
		multiMap[kv.Key] = kv
	}
	for _, name := range multiOrder {
		kv, ok := multiMap[name]
		if !ok {
			continue
		}
		if len(lines) >= height-2 {
			break
		}
		lines = append(lines, "")
		lines = append(lines, detailKeyStyle.Render(strings.ToUpper(kv.Key)))
		for entry := range strings.SplitSeq(kv.Value, ", ") {
			if len(lines) >= height-2 {
				break
			}
			maxW := max(width-4, 10)
			entryRunes := []rune(entry)
			if len(entryRunes) <= maxW {
				lines = append(lines, "  "+DimStyle.Render(entry))
			} else {
				lines = append(lines, "  "+DimStyle.Render(string(entryRunes[:maxW])))
				for start := maxW; start < len(entryRunes); start += maxW {
					if len(lines) >= height-2 {
						break
					}
					end := min(start+maxW, len(entryRunes))
					lines = append(lines, "    "+DimStyle.Render(string(entryRunes[start:end])))
				}
			}
		}
	}

	// Add separator after multi-line fields before spec rows.
	if len(multiLineFields) > 0 && len(lines) < height-2 {
		lines = append(lines, "")
	}

	// Render spec and message rows.
	orderedRows := make([]detailRow, 0, len(specRows)+len(messageRows))
	orderedRows = append(orderedRows, specRows...)
	orderedRows = append(orderedRows, messageRows...)
	for _, r := range orderedRows {
		if len(lines) >= height-2 {
			break
		}
		valW := max(width-keyW-2, 4)
		val := r.value
		if len(val) > valW {
			val = val[:valW-3] + "..."
		}
		keyStr := detailKeyStyle.Render(fmt.Sprintf("%-*s", keyW, r.key))
		lines = append(lines, keyStr+DimStyle.Render(val))
	}

	// Render data/secret fields in a separate section with a header.
	if len(dataLines) > 0 && len(lines) < height-2 {
		lines = append(lines, "")
		lines = append(lines, detailKeyStyle.Render(fmt.Sprintf("DATA (%d)", len(dataLines))))
		for _, dl := range dataLines {
			if len(lines) >= height-2 {
				break
			}
			lines = append(lines, dl)
		}
	}

	// Render CONDITIONS section from item.Conditions with color coding.
	if len(item.Conditions) > 0 && len(lines) < height-2 {
		lines = append(lines, "")
		lines = append(lines, detailKeyStyle.Render("CONDITIONS"))
		for _, cond := range item.Conditions {
			if len(lines) >= height-2 {
				break
			}

			// Color the condition type based on status and type name.
			typeStyle := DimStyle // False = greyed out
			if cond.Status == "True" {
				if isNegativeCondType(cond.Type) {
					typeStyle = ErrorStyle // True + negative type = red
				} else {
					typeStyle = StatusRunning // True + positive type = green
				}
			}

			line := "  " + typeStyle.Render(cond.Type)
			if cond.Reason != "" {
				line += DimStyle.Render(": " + cond.Reason)
			}
			lines = append(lines, line)

			if cond.Message != "" && cond.Status != "True" {
				maxW := max(width-6, 10)
				msg := cond.Message
				if len(msg) > maxW {
					msg = msg[:maxW-3] + "..."
				}
				lines = append(lines, "    "+DimStyle.Render(msg))
			}
		}
	}

	// If we have space left and YAML available, show separator + YAML.
	usedLines := len(lines)
	remaining := height - usedLines - 1 // -1 for separator
	if remaining > 3 && yaml != "" {
		lines = append(lines, "")
		yamlContent := RenderYAMLContent(yaml, width, remaining)
		lines = append(lines, yamlContent)
	}

	return strings.Join(lines, "\n")
}

// RenderResourceUsage renders CPU and memory usage bars for the preview.
// If request/limit values are zero, usage is shown without a percentage bar.
func RenderResourceUsage(cpuUsed, cpuReq, cpuLim, memUsed, memReq, memLim int64, width int) string {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Bold(true).Background(BaseBg)

	lines := make([]string, 0, 4)
	lines = append(lines, DimStyle.Bold(true).Render("RESOURCE USAGE"))
	lines = append(lines, "")

	// CPU bar.
	cpuLabel := labelStyle.Render("CPU: ")
	cpuBar := renderUsageBar(cpuUsed, cpuReq, cpuLim, width-6, FormatCPU)
	lines = append(lines, cpuLabel+cpuBar)

	// Memory bar.
	memLabel := labelStyle.Render("Mem: ")
	memBar := renderUsageBar(memUsed, memReq, memLim, width-6, FormatMemory)
	lines = append(lines, memLabel+memBar)

	return strings.Join(lines, "\n")
}

// renderUsageBar renders a single usage bar line.
func renderUsageBar(used, req, lim int64, barWidth int, formatFn func(int64) string) string {
	usedStr := formatFn(used)

	// Determine the reference value for percentage (prefer limit, fallback to request).
	ref := lim
	if ref == 0 {
		ref = req
	}

	if ref == 0 {
		// No reference: just show the used value without a bar.
		return NormalStyle.Render(usedStr)
	}

	pct := float64(used) / float64(ref) * 100
	if pct > 100 {
		pct = 100
	}

	refStr := formatFn(ref)
	suffix := fmt.Sprintf(" %s / %s (%.0f%%)", usedStr, refStr, pct)
	suffixW := lipgloss.Width(suffix)

	// Calculate bar width.
	bw := max(barWidth-suffixW-2, 5) // 2 for brackets

	filled := min(int(float64(bw)*pct/100), bw)
	empty := bw - filled

	// Color based on percentage.
	var barColor string
	switch {
	case pct >= 90:
		barColor = ColorError
	case pct >= 75:
		barColor = ColorOrange
	default:
		barColor = ColorDimmed
	}

	barStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(barColor)).Background(BaseBg)
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBorder)).Background(BaseBg)

	bar := NormalStyle.Render("[") + barStyle.Render(strings.Repeat("\u2588", filled)) + emptyStyle.Render(strings.Repeat("\u2591", empty)) + NormalStyle.Render("]")
	return bar + NormalStyle.Render(suffix)
}

// isNegativeCondType returns true if a condition type name represents a
// negative/failure state (e.g., "Failed", "Error", "Degraded").
func isNegativeCondType(condType string) bool {
	lower := strings.ToLower(condType)
	for _, neg := range []string{"fail", "error", "degrad"} {
		if strings.Contains(lower, neg) {
			return true
		}
	}
	return false
}

// RenderPreviewEvents renders an event timeline section for the preview pane.
// Events are shown in a compact table with color-coded types.
func RenderPreviewEvents(events []EventTimelineEntry, width int) string {
	if len(events) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(DimStyle.Bold(true).Render("EVENTS"))
	b.WriteString("\n")

	warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorWarning)).Background(BaseBg)
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Background(BaseBg)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Background(BaseBg)
	reasonStyle := lipgloss.NewStyle().Bold(true).Background(BaseBg)
	dimStyle := DimStyle

	// Show at most 20 events to avoid making the preview pane too long.
	maxEvents := 20
	if len(events) > maxEvents {
		events = events[:maxEvents]
	}

	// Calculate column widths for alignment.
	// Format: AGE  TYPE  REASON  MESSAGE
	maxAgeW := 0
	maxReasonW := 0
	for _, e := range events {
		ageStr := relativeTime(e.Timestamp)
		if len(ageStr) > maxAgeW {
			maxAgeW = len(ageStr)
		}
		if len(e.Reason) > maxReasonW {
			maxReasonW = len(e.Reason)
		}
	}
	// Cap reason column width.
	if maxReasonW > 25 {
		maxReasonW = 25
	}

	// Message width: total width minus age, type indicator, reason, spacing.
	msgWidth := max(width-maxAgeW-3-maxReasonW-4, 20) // 3 for " ● ", 4 for spacing

	for _, e := range events {
		ageStr := relativeTime(e.Timestamp)

		// Type indicator and styling.
		var dot, reasonStr string
		switch e.Type {
		case "Warning":
			dot = errorStyle.Render("\u25cf")
			reasonStr = errorStyle.Bold(true).Render(fmt.Sprintf("%-*s", maxReasonW, truncateStr(e.Reason, maxReasonW)))
		default:
			dot = normalStyle.Render("\u25cf")
			reasonStr = reasonStyle.Render(fmt.Sprintf("%-*s", maxReasonW, truncateStr(e.Reason, maxReasonW)))
		}

		// Age.
		ageFormatted := dimStyle.Render(fmt.Sprintf("%-*s", maxAgeW, ageStr))

		// Count suffix.
		countStr := ""
		if e.Count > 1 {
			countStr = warningStyle.Render(fmt.Sprintf(" (x%d)", e.Count))
		}

		// Message - wrap long messages rather than truncate.
		msg := e.Message
		if len(msg) > msgWidth {
			msg = msg[:msgWidth-3] + "..."
		}

		fmt.Fprintf(&b, "%s%s%s%s%s%s%s",
			DimStyle.Render(" "), ageFormatted, DimStyle.Render(" "), dot, DimStyle.Render(" "), reasonStr, DimStyle.Render(" "+msg))
		if countStr != "" {
			b.WriteString(countStr)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// truncateStr truncates a string to maxLen characters.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// FormatCPU formats millicores into a human-readable string.
func FormatCPU(millis int64) string {
	if millis >= 1000 {
		return fmt.Sprintf("%.1f", float64(millis)/1000)
	}
	return fmt.Sprintf("%dm", millis)
}

// FormatMemory formats bytes into a human-readable string (Ki/Mi/Gi).
func FormatMemory(bytes int64) string {
	switch {
	case bytes >= 1024*1024*1024:
		return fmt.Sprintf("%.1fGi", float64(bytes)/(1024*1024*1024))
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.0fMi", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.0fKi", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

// ComputePctStr computes usage as a percentage of a reference value string.
// Returns "n/a" if reference is empty or zero, otherwise returns e.g. "42%".
func ComputePctStr(used int64, refStr string, isCPU bool) string {
	if refStr == "" {
		return "n/a"
	}
	ref := ParseResourceValue(refStr, isCPU)
	if ref == 0 {
		return "n/a"
	}
	pct := float64(used) / float64(ref) * 100
	return fmt.Sprintf("%.0f%%", pct)
}

// renderKV renders a single key-value pair for the resource summary.
func renderKV(key, value string, width int) string {
	keyStr := HeaderStyle.Render(key + ":")
	keyW := lipgloss.Width(keyStr)
	maxVal := max(width-keyW-2, 4)
	if len(value) > maxVal {
		value = value[:maxVal-3] + "..."
	}
	return keyStr + " " + DimStyle.Render(value)
}

// renderDataKV renders a key-value pair for ConfigMap/Secret data fields.
// If the value contains newlines, the key is rendered on its own line and
// each subsequent line of the value is indented with two spaces.
// Returns a slice of lines to allow the caller to account for height limits.
func renderDataKV(key, value string, width int) []string {
	// Normalize escaped newline sequences to actual newlines.
	value = strings.ReplaceAll(value, `\n`, "\n")
	value = strings.ReplaceAll(value, `\t`, "\t")

	if !strings.Contains(value, "\n") {
		return []string{renderKV(key, value, width)}
	}
	// Multiline: render key on its own line, then indented value lines.
	header := HeaderStyle.Render(key + ":")
	lines := []string{header}
	indent := "  "
	maxVal := max(width-len(indent)-1, 4)
	for vline := range strings.SplitSeq(value, "\n") {
		rendered := vline
		if len(rendered) > maxVal {
			rendered = rendered[:maxVal-3] + "..."
		}
		lines = append(lines, DimStyle.Render(indent+rendered))
	}
	return lines
}

// RenderResourceTree renders a resource relationship tree as an ASCII tree diagram.
func RenderResourceTree(root *model.ResourceNode, width, height int) string {
	if root == nil {
		return DimStyle.Render("No resource tree available")
	}

	var b strings.Builder

	// Title.
	title := SelectedStyle.Bold(true).Render(" Resource Map ")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Render root node. Pass root's own namespace to suppress the namespace annotation
	// on the root itself (the user already knows which resource they selected).
	rootLabel := renderTreeNodeLabel(root, root.Namespace)
	b.WriteString(Truncate(rootLabel, width))
	b.WriteString("\n")

	// Empty line between root and children for better visual separation.
	if len(root.Children) > 0 {
		b.WriteString(DimStyle.Render("│"))
		b.WriteString("\n")
	}

	// Render children recursively.
	lines := renderTreeChildren(root.Children, "", root.Namespace, width)
	for _, line := range lines {
		b.WriteString(line)
		b.WriteString("\n")
	}

	if len(root.Children) == 0 {
		b.WriteString(DimStyle.Render("  (no owned resources)"))
		b.WriteString("\n")
	}

	result := b.String()
	return FillLinesBg(result, width, BaseBg)
}

// renderTreeNodeLabel builds the display label for a single tree node.
// parentNamespace is used to conditionally show the namespace when it differs
// from the parent node.
func renderTreeNodeLabel(node *model.ResourceNode, parentNamespace string) string {
	// Use YamlKeyStyle (bold + primary + themed background) for the kind prefix.
	// OverlayTitleStyle has bottom padding which breaks tree indentation.
	label := YamlKeyStyle.Render(node.Kind+"/") + NormalStyle.Render(node.Name)

	// Show namespace when it differs from the parent.
	if node.Namespace != "" && node.Namespace != parentNamespace {
		label += " " + DimStyle.Render("(ns: "+node.Namespace+")")
	}

	// Show child count for nodes that have children.
	if len(node.Children) > 0 {
		childKind := node.Children[0].Kind
		label += " " + DimStyle.Render(fmt.Sprintf("(%d %s)", len(node.Children), childKind))
	}

	if node.Status != "" {
		label += " " + StatusStyle(node.Status).Render("["+node.Status+"]")
	}
	return label
}

func renderTreeChildren(children []*model.ResourceNode, prefix, parentNamespace string, width int) []string {
	lines := make([]string, 0, len(children))
	for i, child := range children {
		isLast := i == len(children)-1

		connector := "├── "
		childPrefix := "│   "
		if isLast {
			connector = "└── "
			childPrefix = "    "
		}

		line := DimStyle.Render(prefix+connector) + renderTreeNodeLabel(child, parentNamespace)
		lines = append(lines, Truncate(line, width))

		// Recurse into children.
		if len(child.Children) > 0 {
			subLines := renderTreeChildren(child.Children, prefix+childPrefix, child.Namespace, width)
			lines = append(lines, subLines...)
		}
	}
	return lines
}
