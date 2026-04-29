package ui

import (
	"fmt"
	"strings"
)

// FinalizerMatchEntry is the UI-facing representation of a finalizer match.
type FinalizerMatchEntry struct {
	Name      string
	Namespace string
	Kind      string
	Matched   string // the specific finalizer that matched
	Age       string
}

// RenderFinalizerSearchOverlay renders the finalizer search results overlay.
func RenderFinalizerSearchOverlay(
	results []FinalizerMatchEntry,
	cursor int,
	selected map[string]bool,
	pattern, filter string,
	filterActive, loading bool,
	width, height int,
) string {
	innerW := max(
		// account for overlay padding and borders
		width-6, 20)

	// Initial search prompt: no pattern entered yet.
	if pattern == "" {
		title := OverlayTitleStyle.Render("Finalizer Search")
		prompt := HelpKeyStyle.Render("search") + BarDimStyle.Render(": ") +
			OverlayNormalStyle.Render(filter) +
			OverlayDimStyle.Render("\u2588")
		body := title + "\n\n" +
			OverlayDimStyle.Render("  Enter a finalizer name or pattern to search for.") + "\n\n" +
			prompt
		return body
	}

	// Title with match count.
	matchCount := len(results)
	selectedCount := len(selected)
	titleText := fmt.Sprintf("Finalizer Search: %s (%d matches", pattern, matchCount)
	if selectedCount > 0 {
		titleText += fmt.Sprintf(", %d selected", selectedCount)
	}
	titleText += ")"
	title := OverlayTitleStyle.Render(titleText)

	if loading {
		body := title + "\n\n" + OverlayDimStyle.Render("  Scanning resources...")
		return body
	}

	if matchCount == 0 {
		body := title + "\n\n" + OverlayDimStyle.Render("  No resources found with matching finalizer.")
		return body
	}

	// Calculate visible area.
	headerLines := 2 // title + blank line
	footerLines := 2 // blank + filter/hints
	maxVisible := max(height-headerLines-footerLines-4, 1)

	// Determine scroll window with scrolloff margin.
	scrollOff := ConfigScrollOff
	if maxVisible < 8 {
		scrollOff = 0
	}
	scrollOffset := min(cursor-scrollOff, 0)
	if cursor+scrollOff >= scrollOffset+maxVisible {
		scrollOffset = cursor + scrollOff - maxVisible + 1
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}
	// Don't leave empty space at the bottom.
	if scrollOffset+maxVisible > len(results) {
		scrollOffset = max(len(results)-maxVisible, 0)
	}
	endIdx := min(scrollOffset+maxVisible, len(results))

	// Column widths for alignment.
	nsWidth := 0
	kindWidth := 0
	nameWidth := 0
	for _, r := range results {
		if len(r.Namespace) > nsWidth {
			nsWidth = len(r.Namespace)
		}
		if len(r.Kind) > kindWidth {
			kindWidth = len(r.Kind)
		}
		if len(r.Name) > nameWidth {
			nameWidth = len(r.Name)
		}
	}
	// Cap namespace and kind columns.
	if nsWidth > 20 {
		nsWidth = 20
	}
	if kindWidth > 15 {
		kindWidth = 15
	}
	// Split remaining space between resource name and finalizer name.
	fixedCols := 2 + nsWidth + 1 + kindWidth + 1 + 2 + 5 // checkmark+space + ns + kind + gaps + age
	remaining := max(innerW-fixedCols, 20)
	// Give 40% to name, 60% to finalizer.
	nameWidth = max(min(nameWidth, remaining*40/100), 10)
	finalizerW := max(remaining-nameWidth, 10)

	var lines []string
	for i := scrollOffset; i < endIdx; i++ {
		r := results[i]
		key := r.Namespace + "/" + r.Kind + "/" + r.Name
		prefix := "  "
		if selected[key] {
			prefix = "\u2713 "
		}

		ns := truncateStr(r.Namespace, nsWidth)
		kind := truncateStr(r.Kind, kindWidth)
		name := truncateStr(r.Name, nameWidth)
		fin := truncateStr(r.Matched, finalizerW)

		lineContent := fmt.Sprintf(
			"%s%-*s %-*s %-*s  %-*s %s",
			prefix, nsWidth, ns, kindWidth, kind, nameWidth, name,
			finalizerW, fin, r.Age,
		)

		if i == cursor {
			lines = append(lines, OverlaySelectedStyle.Render(lineContent))
		} else if selected[key] {
			lines = append(lines, OverlayFilterStyle.Render(lineContent))
		} else {
			lines = append(lines, OverlayNormalStyle.Render(lineContent))
		}
	}

	content := strings.Join(lines, "\n")

	// Show filter bar only when filter is active or has text.
	var footer string
	if filterActive {
		footer = "\n" + HelpKeyStyle.Render("/") + BarDimStyle.Render(": ") +
			OverlayNormalStyle.Render(filter) +
			OverlayDimStyle.Render("\u2588")
	} else if filter != "" {
		footer = "\n" + OverlayDimStyle.Render("filter: ") + OverlayFilterStyle.Render(filter)
	}

	return title + "\n" + content + footer
}
