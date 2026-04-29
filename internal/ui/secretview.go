package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
)

// secretInnerPanelStyle is the bordered panel containing the secret table.
var secretInnerPanelStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color(ColorBorder)).
	Padding(0, 1)

// RenderSecretEditorOverlay renders a centered popup overlay for editing secrets.
func RenderSecretEditorOverlay(
	secret *model.SecretData,
	cursor int,
	revealedKeys map[string]bool,
	allRevealed bool,
	editing bool,
	editKey string,
	editValue string,
	editColumn int, // 0=key, 1=value
	screenWidth, screenHeight int,
) string {
	if secret == nil {
		return OverlayStyle.Render(ErrorStyle.Render("No secret loaded"))
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
	title := OverlayTitleStyle.Render("Secret Editor")

	// Data table content.
	dataContent := renderSecretEditorTable(
		secret, cursor, revealedKeys, allRevealed,
		editing, editKey, editValue, editColumn,
		panelContentW, panelContentH,
	)

	// Inner bordered panel.
	innerPanel := secretInnerPanelStyle.
		Width(panelW).
		Height(panelContentH).
		Render(dataContent)

	body := title + "\n" + innerPanel

	return OverlayStyle.
		Width(boxW).
		Render(body)
}

// renderSecretEditorTable renders the key-value table inside the secret editor.
func renderSecretEditorTable(
	secret *model.SecretData,
	selectedIdx int,
	revealedKeys map[string]bool,
	allRevealed bool,
	editing bool,
	editKey string,
	editValue string,
	editColumn int,
	width, height int,
) string {
	keyColW := 0
	for _, k := range secret.Keys {
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
	end := min(start+tableHeight, len(secret.Keys))

	for i := start; i < end; i++ {
		k := secret.Keys[i]
		v := secret.Data[k]

		var line string
		switch {
		case i == selectedIdx && editing:
			// Inline edit mode. Always show the in-progress edit values
			// in both columns so they survive tabbing between columns.
			// Only the cursor block follows editColumn.
			if editColumn == 0 {
				// Editing key, value column shows the in-progress edit value.
				valDisplay := Truncate(editValue, valColW)
				editDisplay := editKey + DimStyle.Render("\u2588")
				editW := lipgloss.Width(editDisplay)
				pad := max(keyColW-editW, 0)
				line = HelpKeyStyle.Render("> ") + editDisplay + strings.Repeat(" ", pad) + "  |  " + valDisplay
			} else {
				// Editing value, key column shows the in-progress edit key.
				keyDisplay := Truncate(editKey, keyColW)
				editDisplay := editValue + DimStyle.Render("\u2588")
				line = HelpKeyStyle.Render("> ") + fmt.Sprintf("%-*s", keyColW, keyDisplay) + "  |  " + editDisplay
			}
		case i == selectedIdx:
			// Selected row.
			valDisplay := secretValueDisplay(v, revealedKeys[k] || allRevealed, valColW)
			rawLine := fmt.Sprintf("> %-*s  |  %-*s", keyColW, Truncate(k, keyColW), valColW, valDisplay)
			line = OverlaySelectedStyle.Render(rawLine)
		default:
			// Normal row.
			kPadded := fmt.Sprintf("%-*s", keyColW, Truncate(k, keyColW))
			keyStr := HelpKeyStyle.Render(kPadded)

			var valStr string
			if revealedKeys[k] || allRevealed {
				valStr = DimStyle.Render(Truncate(v, valColW))
			} else {
				valStr = DimStyle.Render("********")
			}
			line = "  " + keyStr + "  |  " + valStr
		}
		lines = append(lines, line)
	}

	if len(secret.Keys) == 0 {
		lines = append(lines, DimStyle.Render("  (empty - press 'a' to add a key)"))
	}

	return strings.Join(lines, "\n")
}

// secretValueDisplay returns the display string for a secret value.
func secretValueDisplay(val string, revealed bool, maxW int) string {
	if revealed {
		return Truncate(val, maxW)
	}
	return "********"
}
