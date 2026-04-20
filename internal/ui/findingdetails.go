// Pure-function renderer for the right preview pane when a security
// finding is selected. Reads everything from item.Columns via
// Item.ColumnValue, so the renderer has no dependency on internal/security.
package ui

import (
	"fmt"
	"strings"

	"github.com/janosmiko/lfk/internal/model"
)

// RenderFindingDetails produces the right preview pane body for a selected
// security-finding Item.
func RenderFindingDetails(item model.Item, width, height int) string {
	var b strings.Builder

	severity := item.ColumnValue("Severity")
	title := item.ColumnValue("Title")
	fmt.Fprintf(&b, "%s  %s\n", styleSeverityBadge(severity), title)
	sepWidth := width
	if sepWidth < 1 {
		sepWidth = 1
	}
	if sepWidth > 120 {
		sepWidth = 120
	}
	b.WriteString(strings.Repeat("─", sepWidth))
	b.WriteString("\n\n")

	kv := func(k, v string) {
		if v == "" {
			return
		}
		label := k + ":"
		if len(label) < 12 {
			label = label + strings.Repeat(" ", 12-len(label))
		}
		fmt.Fprintf(&b, "  %s  %s\n", label, v)
	}
	kv("Resource", item.ColumnValue("Resource"))
	if item.Namespace != "" {
		kv("Namespace", item.Namespace)
	}
	kv("Source", item.ColumnValue("Source"))
	kv("Category", item.ColumnValue("Category"))

	reserved := map[string]bool{
		"Severity":     true,
		"Resource":     true,
		"Title":        true,
		"Category":     true,
		"Source":       true,
		"ResourceKind": true,
		"Description":  true,
		"References":   true,
	}
	var extras []model.KeyValue
	for _, c := range item.Columns {
		if !reserved[c.Key] {
			extras = append(extras, c)
		}
	}
	if len(extras) > 0 {
		b.WriteString("\n")
		for _, c := range extras {
			kv(c.Key, c.Value)
		}
	}

	if desc := item.ColumnValue("Description"); desc != "" {
		b.WriteString("\n  Description:\n")
		wrapWidth := width - 4
		if wrapWidth < 20 {
			wrapWidth = 20
		}
		for _, line := range wrapLines(desc, wrapWidth) {
			fmt.Fprintf(&b, "    %s\n", line)
		}
	}

	if refs := item.ColumnValue("References"); refs != "" {
		b.WriteString("\n  References:\n")
		for _, ref := range strings.Split(refs, "\n") {
			fmt.Fprintf(&b, "    %s\n", ref)
		}
	}

	b.WriteString("\n")
	b.WriteString(DimStyle.Render("  [Enter] jump to resource   [o] owner   [y] copy id"))
	return b.String()
}

// RenderFindingGroupDetails produces the right preview pane body when a
// __security_finding_group__ item is selected. It shows the group summary
// plus a mini table of affected resources (from rightItems).
func RenderFindingGroupDetails(group model.Item, affected []model.Item, width, height int) string {
	var b strings.Builder

	severity := group.ColumnValue("Severity")
	title := group.Name
	fmt.Fprintf(&b, "%s  %s\n", styleSeverityBadge(severity), title)
	sepWidth := width
	if sepWidth < 1 {
		sepWidth = 1
	}
	if sepWidth > 120 {
		sepWidth = 120
	}
	b.WriteString(strings.Repeat("─", sepWidth))
	b.WriteString("\n\n")

	kv := func(k, v string) {
		if v == "" {
			return
		}
		label := k + ":"
		if len(label) < 14 {
			label = label + strings.Repeat(" ", 14-len(label))
		}
		fmt.Fprintf(&b, "  %s  %s\n", label, v)
	}
	kv("Affected", group.ColumnValue("Affected")+" resources")
	kv("Source", group.ColumnValue("Source"))
	kv("Category", group.ColumnValue("Category"))

	if desc := group.ColumnValue("Description"); desc != "" {
		b.WriteString("\n  Description:\n")
		wrapWidth := width - 4
		if wrapWidth < 20 {
			wrapWidth = 20
		}
		for _, line := range wrapLines(desc, wrapWidth) {
			fmt.Fprintf(&b, "    %s\n", line)
		}
	}

	if refs := group.ColumnValue("References"); refs != "" {
		b.WriteString("\n  References:\n")
		for _, ref := range strings.Split(refs, "\n") {
			fmt.Fprintf(&b, "    %s\n", ref)
		}
	}

	if len(affected) > 0 {
		b.WriteString("\n  Affected resources:\n")
		maxShow := height / 2
		if maxShow < 5 {
			maxShow = 5
		}
		if maxShow > len(affected) {
			maxShow = len(affected)
		}
		for i := range maxShow {
			it := affected[i]
			sev := it.ColumnValue("Severity")
			fmt.Fprintf(&b, "    %s  %s",
				styleSeverityBadge(sev), it.Name)
			if it.Namespace != "" {
				fmt.Fprintf(&b, "  (%s)", it.Namespace)
			}
			b.WriteString("\n")
		}
		if len(affected) > maxShow {
			fmt.Fprintf(&b, "    ... and %d more\n", len(affected)-maxShow)
		}
	}

	b.WriteString("\n")
	b.WriteString(DimStyle.Render("  [Enter/l] affected resources   [#] security menu"))
	return b.String()
}

// RenderAffectedResourceDetails produces the right preview pane body for
// a __security_affected_resource__ item at LevelOwned.
func RenderAffectedResourceDetails(item model.Item, width, height int) string {
	var b strings.Builder

	severity := item.ColumnValue("Severity")
	resource := item.ColumnValue("Resource")
	fmt.Fprintf(&b, "%s  %s\n", styleSeverityBadge(severity), resource)
	sepWidth := width
	if sepWidth < 1 {
		sepWidth = 1
	}
	if sepWidth > 120 {
		sepWidth = 120
	}
	b.WriteString(strings.Repeat("─", sepWidth))
	b.WriteString("\n\n")

	kv := func(k, v string) {
		if v == "" {
			return
		}
		label := k + ":"
		if len(label) < 14 {
			label = label + strings.Repeat(" ", 14-len(label))
		}
		fmt.Fprintf(&b, "  %s  %s\n", label, v)
	}
	kv("Kind", item.ColumnValue("ResourceKind"))
	kv("Namespace", item.ColumnValue("Namespace"))
	kv("Findings", item.ColumnValue("FindingCount"))

	if desc := item.ColumnValue("Description"); desc != "" {
		b.WriteString("\n  Details:\n")
		wrapWidth := width - 4
		if wrapWidth < 20 {
			wrapWidth = 20
		}
		for _, line := range wrapLines(desc, wrapWidth) {
			fmt.Fprintf(&b, "    %s\n", line)
		}
	}

	b.WriteString("\n")
	b.WriteString(DimStyle.Render("  [Enter] jump to resource   [h] back to group   [#] security menu"))
	return b.String()
}

// styleSeverityBadge returns a colored inline badge for a severity string.
func styleSeverityBadge(sev string) string {
	switch sev {
	case "CRIT":
		return StatusFailed.Render(" CRIT ")
	case "HIGH":
		return DeprecationStyle.Render(" HIGH ")
	case "MED":
		return StatusProgressing.Render(" MED  ")
	case "LOW":
		return StatusRunning.Render(" LOW  ")
	}
	return " ?    "
}

// wrapLines splits a string into lines no longer than width, preserving
// any pre-existing newlines as paragraph breaks. Long words that exceed
// width are not broken. Unlike wrapText, wrapLines handles multi-paragraph
// input by splitting on "\n" first and wrapping each paragraph independently.
func wrapLines(s string, width int) []string {
	var out []string
	for _, para := range strings.Split(s, "\n") {
		if para == "" {
			out = append(out, "")
			continue
		}
		wrapped := wrapText(para, width)
		if len(wrapped) == 0 {
			out = append(out, "")
			continue
		}
		out = append(out, wrapped...)
	}
	return out
}
