package ui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
)

// configMapInnerPanelStyle is the bordered panel containing the configmap table.
//
// The bg fields are NOT set here — they're chained at render time so
// the value comes from the active theme (this var is initialised
// before ApplyTheme runs, so theme bgs would be NoColor).
var configMapInnerPanelStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color(ColorBorder)).
	Padding(0, 1)

// RenderConfigMapEditorOverlay renders a centered popup overlay for editing configmaps.
//
// searchQuery / searchActive drive the / filter (see RenderSecretEditorOverlay
// for the full contract). Cursor is interpreted as an index into the
// FILTERED key list.
func RenderConfigMapEditorOverlay(
	cm *model.ConfigMapData,
	cursor int,
	editing bool,
	editKey string,
	editValue string,
	editColumn int, // 0=key, 1=value
	searchQuery string,
	searchActive bool,
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

	searchBar := RenderKVEditorSearchBar(searchQuery, searchActive)
	searchH := 0
	if searchBar != "" {
		searchH = 1
	}

	panelContentH := max(boxH-outerPadH-innerPadH-titleH-gapH-searchH, 3)
	panelContentW := max(boxW-outerPadW-innerPadW, 20)
	panelW := boxW - outerPadW

	// Title — bg overridden to baseBg so the title row (and its 1-row
	// bottom padding) match the rest of the editor's baseBg surface;
	// the stock OverlayTitleStyle uses surfaceBg.
	title := OverlayTitleStyle.Background(BaseBg).Render("ConfigMap Editor")

	visibleKeys := FilterKVKeys(cm.Keys, searchQuery)
	filteredCM := &model.ConfigMapData{Keys: visibleKeys, Data: cm.Data}

	// Data table content.
	dataContent := renderConfigMapEditorTable(
		filteredCM, cursor,
		editing, editKey, editValue, editColumn,
		panelContentW, panelContentH,
	)

	// Inner bordered panel — bg + border-bg pulled from the active
	// theme at render time. See secretview for the full rationale.
	innerPanel := configMapInnerPanelStyle.
		Background(BaseBg).
		BorderBackground(BaseBg).
		Width(panelW).
		Height(panelContentH).
		Render(dataContent)

	body := title
	if searchBar != "" {
		body += "\n" + searchBar
	}
	body += "\n" + innerPanel

	// baseBg end-to-end so the outer frame matches the inner panel.
	return OverlayStyle.
		Background(BaseBg).
		BorderBackground(BaseBg).
		Width(boxW).
		Render(body)
}

// renderConfigMapEditorTable renders the key-value table inside the
// configmap editor. Uses the shared lipgloss/table-based renderer
// (newKVEditorTable) so the three K/V editors stay visually identical.
func renderConfigMapEditorTable(
	cm *model.ConfigMapData,
	selectedIdx int,
	editing bool,
	editKey string,
	editValue string,
	editColumn int,
	width, height int,
) string {
	keyColW := computeKeyColumnWidth(cm.Keys, width, 3)
	valColW := max(width-keyColW-5, 8)

	bodyHeight := max(height-2, 1)
	start := scrollWindowStart(selectedIdx, bodyHeight, len(cm.Keys))
	end := min(start+bodyHeight, len(cm.Keys))

	t := newKVEditorTable(keyColW, valColW, selectedIdx-start)
	for i := start; i < end; i++ {
		k := cm.Keys[i]
		v := cm.Data[k]
		displayV := configMapValueDisplay(v, valColW)

		var keyText, valText string
		switch {
		case i == selectedIdx && editing && editColumn == 0:
			keyText = SingleLineCell(editKey, keyColW-1) + "\u2588"
			valText = SingleLineCell(editValue, valColW)
		case i == selectedIdx && editing && editColumn == 1:
			keyText = SingleLineCell(editKey, keyColW)
			valText = SingleLineCell(editValue, valColW-1) + "\u2588"
		default:
			keyText = SingleLineCell(k, keyColW)
			valText = displayV
		}
		t.Row(keyText, valText)
	}
	rendered := t.Render()
	if len(cm.Keys) == 0 {
		return rendered + "\n" + BarDimStyle.Render("  (empty - press 'a' to add a key)")
	}
	return rendered
}

// configMapValueDisplay returns the display string for a configmap value.
// Routes through SingleLineCell so multi-line YAML/JSON payloads
// (common in configmaps) collapse to one row with a "↵" glyph instead
// of expanding the table cell vertically.
func configMapValueDisplay(val string, maxW int) string {
	return SingleLineCell(val, maxW)
}
