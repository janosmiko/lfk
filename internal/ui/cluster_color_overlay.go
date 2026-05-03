package ui

import (
	"fmt"
	"strings"
)

// clusterColorPickerNoneRow is the user-visible label for the picker row
// that clears any previously assigned color.
const clusterColorPickerNoneRow = "None  (clear)"

// RenderClusterColorOverlay renders the color picker for the cluster
// highlighted in the cluster picker. Rows are the named colours in
// ClusterColorNames declaration order, followed by a "None" row that
// clears the assignment. cursor selects which row is highlighted.
func RenderClusterColorOverlay(contextName string, cursor int) string {
	titleText := "Set color for " + contextName
	if contextName == "" {
		titleText = "Set cluster color"
	}
	title := OverlayTitleStyle.Render(titleText)

	rows := make([]string, 0, len(ClusterColorNames)+1)
	for i, name := range ClusterColorNames {
		swatch := ClusterColorSwatch(name)
		// Pad the label so the cursor highlight (which renders the whole
		// line) makes the rows align visually instead of jaggedly hugging
		// the shortest name.
		label := fmt.Sprintf("%-9s", name)
		raw := fmt.Sprintf("  %s  %s", swatch, label)
		if i == cursor {
			rows = append(rows, OverlaySelectedStyle.Render(raw))
		} else {
			rows = append(rows, OverlayNormalStyle.Render(raw))
		}
	}
	noneRaw := fmt.Sprintf("  %s  %s", ClusterColorSwatch(""), clusterColorPickerNoneRow)
	if cursor == len(ClusterColorNames) {
		rows = append(rows, OverlaySelectedStyle.Render(noneRaw))
	} else {
		rows = append(rows, OverlayNormalStyle.Render(noneRaw))
	}

	hints := OverlayDimStyle.Render("↑↓ navigate | enter: apply | esc: cancel")
	body := title + "\n\n" + strings.Join(rows, "\n") + "\n\n" + hints
	return OverlayStyle.Render(body)
}
