package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
)

// labelInnerPanelStyle is the bordered panel containing the label/annotation table.
var labelInnerPanelStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color(ColorBorder)).
	Padding(0, 1)

// RenderLabelEditorOverlay renders the label/annotation editor popup.
func RenderLabelEditorOverlay(
	data *model.LabelAnnotationData,
	cursor int,
	tab int, // 0=labels, 1=annotations
	editing bool,
	editKey string,
	editValue string,
	editColumn int,
	screenWidth, screenHeight int,
) string {
	if data == nil {
		return OverlayStyle.Render(ErrorStyle.Render("No data loaded"))
	}

	boxW := screenWidth * 75 / 100
	boxH := screenHeight * 75 / 100
	if boxW < 50 {
		boxW = 50
	}
	if boxH < 10 {
		boxH = 10
	}

	outerPadH := 4
	outerPadW := 6
	innerPadH := 2
	innerPadW := 4
	titleH := 2 // title + tab bar
	gapH := 1

	panelContentH := boxH - outerPadH - innerPadH - titleH - gapH
	if panelContentH < 3 {
		panelContentH = 3
	}
	panelContentW := boxW - outerPadW - innerPadW
	if panelContentW < 20 {
		panelContentW = 20
	}
	panelW := boxW - outerPadW

	title := OverlayTitleStyle.Render("Label / Annotation Editor")

	// Tab bar.
	labelsTab := fmt.Sprintf(" Labels (%d) ", len(data.LabelKeys))
	annotsTab := fmt.Sprintf(" Annotations (%d) ", len(data.AnnotKeys))
	if tab == 0 {
		labelsTab = OverlaySelectedStyle.Render(labelsTab)
		annotsTab = DimStyle.Render(annotsTab)
	} else {
		labelsTab = DimStyle.Render(labelsTab)
		annotsTab = OverlaySelectedStyle.Render(annotsTab)
	}
	tabBar := labelsTab + "  " + annotsTab

	// Content.
	var keys []string
	var dataMap map[string]string
	if tab == 0 {
		keys = data.LabelKeys
		dataMap = data.Labels
	} else {
		keys = data.AnnotKeys
		dataMap = data.Annotations
	}

	dataContent := renderLabelEditorTable(keys, dataMap, cursor, editing, editKey, editValue, editColumn, panelContentW, panelContentH)

	innerPanel := labelInnerPanelStyle.
		Width(panelW).
		Height(panelContentH).
		Render(dataContent)

	body := title + "\n" + tabBar + "\n" + innerPanel

	return OverlayStyle.
		Width(boxW).
		Render(body)
}

func renderLabelEditorTable(keys []string, data map[string]string, selectedIdx int, editing bool, editKey, editValue string, editColumn int, width, height int) string {
	keyColW := 0
	for _, k := range keys {
		if len(k) > keyColW {
			keyColW = len(k)
		}
	}
	if keyColW < 10 {
		keyColW = 10
	}
	if keyColW > width/2 {
		keyColW = width / 2
	}

	valColW := width - keyColW - 10
	if valColW < 8 {
		valColW = 8
	}

	var lines []string
	keyPadded := fmt.Sprintf("%-*s", keyColW, "Key")
	headerLine := "  " + HeaderStyle.Render(keyPadded) + "  |  " + HeaderStyle.Render("Value")
	separator := "  " + strings.Repeat("-", keyColW) + "--+--" + strings.Repeat("-", valColW)
	lines = append(lines, headerLine)
	lines = append(lines, DimStyle.Render(separator))

	tableHeight := height - 2
	if tableHeight < 1 {
		tableHeight = 1
	}
	start := 0
	if selectedIdx >= tableHeight {
		start = selectedIdx - tableHeight + 1
	}
	end := start + tableHeight
	if end > len(keys) {
		end = len(keys)
	}

	for i := start; i < end; i++ {
		k := keys[i]
		v := data[k]
		displayV := Truncate(v, valColW)

		var line string
		switch {
		case i == selectedIdx && editing:
			// While editing, always show the in-progress edit values in
			// both columns, not the committed slot value. Only the cursor
			// block follows editColumn. This keeps the user's typed key
			// visible after tabbing to the value column (and vice versa).
			if editColumn == 0 {
				editDisplay := editKey + DimStyle.Render("\u2588")
				editW := lipgloss.Width(editDisplay)
				pad := keyColW - editW
				if pad < 0 {
					pad = 0
				}
				valDisplay := Truncate(editValue, valColW)
				line = HelpKeyStyle.Render("> ") + editDisplay + strings.Repeat(" ", pad) + "  |  " + valDisplay
			} else {
				keyDisplay := Truncate(editKey, keyColW)
				editDisplay := editValue + DimStyle.Render("\u2588")
				line = HelpKeyStyle.Render("> ") + fmt.Sprintf("%-*s", keyColW, keyDisplay) + "  |  " + editDisplay
			}
		case i == selectedIdx:
			rawLine := fmt.Sprintf("> %-*s  |  %-*s", keyColW, Truncate(k, keyColW), valColW, displayV)
			line = OverlaySelectedStyle.Render(rawLine)
		default:
			kPadded := fmt.Sprintf("%-*s", keyColW, Truncate(k, keyColW))
			keyStr := HelpKeyStyle.Render(kPadded)
			valStr := DimStyle.Render(displayV)
			line = "  " + keyStr + "  |  " + valStr
		}
		lines = append(lines, line)
	}

	if len(keys) == 0 {
		lines = append(lines, DimStyle.Render("  (empty - press 'a' to add)"))
	}

	return strings.Join(lines, "\n")
}
