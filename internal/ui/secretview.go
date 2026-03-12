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
	helpH := 1
	gapH := 2

	panelContentH := boxH - outerPadH - innerPadH - titleH - helpH - gapH
	if panelContentH < 3 {
		panelContentH = 3
	}
	panelContentW := boxW - outerPadW - innerPadW
	if panelContentW < 20 {
		panelContentW = 20
	}
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

	// Help line.
	var helpLine string
	if editing {
		helpLine = HelpKeyStyle.Render("ctrl+s") + DimStyle.Render(" save") + "  " +
			HelpKeyStyle.Render("enter") + DimStyle.Render(" newline") + "  " +
			HelpKeyStyle.Render("tab") + DimStyle.Render(" switch") + "  " +
			HelpKeyStyle.Render("esc") + DimStyle.Render(" cancel")
	} else {
		helpLine = HelpKeyStyle.Render("jk") + DimStyle.Render(" nav") + "  " +
			HelpKeyStyle.Render("v") + DimStyle.Render(" toggle") + "  " +
			HelpKeyStyle.Render("V") + DimStyle.Render(" all") + "  " +
			HelpKeyStyle.Render("e") + DimStyle.Render(" edit") + "  " +
			HelpKeyStyle.Render("a") + DimStyle.Render(" add") + "  " +
			HelpKeyStyle.Render("y") + DimStyle.Render(" copy") + "  " +
			HelpKeyStyle.Render("D") + DimStyle.Render(" del") + "  " +
			HelpKeyStyle.Render("s") + DimStyle.Render(" save") + "  " +
			HelpKeyStyle.Render("esc") + DimStyle.Render(" close")
	}

	body := title + "\n" + innerPanel + "\n" + helpLine

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

	valColW := width - keyColW - 10
	if valColW < 8 {
		valColW = 8
	}

	var lines []string

	// Header.
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
	if end > len(secret.Keys) {
		end = len(secret.Keys)
	}

	for i := start; i < end; i++ {
		k := secret.Keys[i]
		v := secret.Data[k]

		var line string
		if i == selectedIdx && editing {
			// Inline edit mode.
			if editColumn == 0 {
				// Editing key.
				valDisplay := secretValueDisplay(v, revealedKeys[k] || allRevealed, valColW)
				editDisplay := editKey + DimStyle.Render("\u2588")
				editW := lipgloss.Width(editDisplay)
				pad := keyColW - editW
				if pad < 0 {
					pad = 0
				}
				line = HelpKeyStyle.Render("> ") + editDisplay + strings.Repeat(" ", pad) + "  |  " + valDisplay
			} else {
				// Editing value.
				keyDisplay := Truncate(k, keyColW)
				editDisplay := editValue + DimStyle.Render("\u2588")
				line = HelpKeyStyle.Render("> ") + fmt.Sprintf("%-*s", keyColW, keyDisplay) + "  |  " + editDisplay
			}
		} else if i == selectedIdx {
			// Selected row.
			valDisplay := secretValueDisplay(v, revealedKeys[k] || allRevealed, valColW)
			rawLine := fmt.Sprintf("> %-*s  |  %-*s", keyColW, Truncate(k, keyColW), valColW, valDisplay)
			line = OverlaySelectedStyle.Render(rawLine)
		} else {
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
