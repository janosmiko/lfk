package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
)

// configMapInnerPanelStyle is the bordered panel containing the configmap table.
var configMapInnerPanelStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color(ColorBorder)).
	Padding(0, 1)

// RenderConfigMapEditorOverlay renders a centered popup overlay for editing configmaps.
func RenderConfigMapEditorOverlay(
	cm *model.ConfigMapData,
	cursor int,
	editing bool,
	editKey string,
	editValue string,
	editColumn int, // 0=key, 1=value
	screenWidth, screenHeight int,
) string {
	if cm == nil {
		return OverlayStyle.Render(ErrorStyle.Render("No configmap loaded"))
	}

	// Popup dimensions: 75% of screen.
	boxW := screenWidth * 75 / 100
	boxH := screenHeight * 75 / 100
	if boxW < 50 {
		boxW = 50
	}
	if boxH < 10 {
		boxH = 10
	}

	outerPadH := 4 // outer border (2) + outer padding (2)
	outerPadW := 6 // outer border (2) + outer padding (2*2)
	innerPadH := 2 // inner border (2)
	innerPadW := 4 // inner border (2) + inner padding (1*2)
	titleH := 1
	gapH := 1

	panelContentH := max(boxH-outerPadH-innerPadH-titleH-gapH, 3)
	panelContentW := max(boxW-outerPadW-innerPadW, 20)
	panelW := boxW - outerPadW

	// Title.
	title := OverlayTitleStyle.Render("ConfigMap Editor")

	// Data table content.
	dataContent := renderConfigMapEditorTable(
		cm, cursor,
		editing, editKey, editValue, editColumn,
		panelContentW, panelContentH,
	)

	// Inner bordered panel.
	innerPanel := configMapInnerPanelStyle.
		Width(panelW).
		Height(panelContentH).
		Render(dataContent)

	body := title + "\n" + innerPanel

	return OverlayStyle.
		Width(boxW).
		Render(body)
}

// renderConfigMapEditorTable renders the key-value table inside the configmap editor.
func renderConfigMapEditorTable(
	cm *model.ConfigMapData,
	selectedIdx int,
	editing bool,
	editKey string,
	editValue string,
	editColumn int,
	width, height int,
) string {
	keyColW := 0
	for _, k := range cm.Keys {
		if len(k) > keyColW {
			keyColW = len(k)
		}
	}
	if keyColW < 10 {
		keyColW = 10
	}
	if keyColW > width/3 {
		keyColW = width / 3
	}

	valColW := max(width-keyColW-10, 8)

	var lines []string

	// Header.
	keyPadded := fmt.Sprintf("%-*s", keyColW, "Key")
	headerLine := "  " + HeaderStyle.Render(keyPadded) + "  |  " + HeaderStyle.Render("Value")
	separator := "  " + strings.Repeat("-", keyColW) + "--+--" + strings.Repeat("-", valColW)
	lines = append(lines, headerLine)
	lines = append(lines, DimStyle.Render(separator))

	tableHeight := max(height-2, 1)
	start := 0
	if selectedIdx >= tableHeight {
		start = selectedIdx - tableHeight + 1
	}
	end := min(start+tableHeight, len(cm.Keys))

	for i := start; i < end; i++ {
		k := cm.Keys[i]
		v := cm.Data[k]
		// For multiline values, show first line + indicator.
		displayV := configMapValueDisplay(v, valColW)

		var line string
		switch {
		case i == selectedIdx && editing:
			// Inline edit mode. Always show the in-progress edit values in
			// both columns so they survive tabbing between columns. Only
			// the cursor block follows editColumn.
			if editColumn == 0 {
				// Editing key, value column shows the in-progress value.
				editDisplay := editKey + DimStyle.Render("\u2588")
				editW := lipgloss.Width(editDisplay)
				pad := max(keyColW-editW, 0)
				valDisplay := Truncate(editValue, valColW)
				line = HelpKeyStyle.Render("> ") + editDisplay + strings.Repeat(" ", pad) + "  |  " + valDisplay
			} else {
				// Editing value, key column shows the in-progress key.
				keyDisplay := Truncate(editKey, keyColW)
				editDisplay := editValue + DimStyle.Render("\u2588")
				line = HelpKeyStyle.Render("> ") + fmt.Sprintf("%-*s", keyColW, keyDisplay) + "  |  " + editDisplay
			}
		case i == selectedIdx:
			// Selected row.
			rawLine := fmt.Sprintf("> %-*s  |  %-*s", keyColW, Truncate(k, keyColW), valColW, displayV)
			line = OverlaySelectedStyle.Render(rawLine)
		default:
			// Normal row.
			kPadded := fmt.Sprintf("%-*s", keyColW, Truncate(k, keyColW))
			keyStr := HelpKeyStyle.Render(kPadded)
			valStr := DimStyle.Render(displayV)
			line = "  " + keyStr + "  |  " + valStr
		}
		lines = append(lines, line)
	}

	if len(cm.Keys) == 0 {
		lines = append(lines, DimStyle.Render("  (empty - press 'a' to add a key)"))
	}

	return strings.Join(lines, "\n")
}

// configMapValueDisplay returns the display string for a configmap value.
// For multiline values, shows the first line with a continuation indicator.
func configMapValueDisplay(val string, maxW int) string {
	if before, _, ok := strings.Cut(val, "\n"); ok {
		firstLine := before
		return Truncate(firstLine, maxW-4) + " ..."
	}
	return Truncate(val, maxW)
}
