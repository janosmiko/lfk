package ui

import (
	"strings"

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
// searchQuery / searchActive drive the / filter; selected /
// formatActive / formatCursor drive the multi-row Shift+Y copy
// flow (see RenderSecretEditorOverlay for the full contract). The
// cursor is an index into the FILTERED key list.
func RenderConfigMapEditorOverlay(
	cm *model.ConfigMapData,
	cursor int,
	editing bool,
	editKey string,
	editKeyCursor int,
	editValue string,
	editValueCursor int,
	editColumn int, // 0=key, 1=value
	searchQuery string,
	searchActive bool,
	selected map[string]bool,
	formatActive bool,
	formatCursor int,
	editValueScroll int,
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
	var formatBar string
	if formatActive {
		formatBar = RenderKVFormatPicker(formatCursor)
	}
	hasBar := searchBar != "" || formatBar != ""

	// Bars replace the title's bottom padding so the panel doesn't
	// shrink when search/format opens. See secretview for rationale.
	panelContentH := max(boxH-outerPadH-innerPadH-titleH-gapH, 3)
	panelContentW := max(boxW-outerPadW-innerPadW, 20)
	panelW := boxW - outerPadW

	titleStyle := OverlayTitleStyle.Background(BaseBg)
	if hasBar {
		titleStyle = titleStyle.Padding(0, 0, 0, 0)
	}
	title := titleStyle.Render("ConfigMap Editor")

	// Mode selection while editing: pane for multi-line values,
	// inline table edit for single-line. Same contract as the secret
	// editor — see secretview.go for the full rationale.
	var dataContent string
	switch {
	case editing && strings.Contains(editValue, "\n"):
		dataContent = RenderKVEditorEditPane(
			editKey, editKeyCursor,
			editValue, editValueCursor,
			editColumn, editValueScroll, panelContentW, panelContentH,
		)
	default:
		visibleKeys := FilterKVKeys(cm.Keys, searchQuery)
		filteredCM := &model.ConfigMapData{Keys: visibleKeys, Data: cm.Data}
		dataContent = renderConfigMapEditorTable(
			filteredCM, cursor,
			editing, editKey, editKeyCursor, editValue, editValueCursor, editColumn,
			selected,
			panelContentW, panelContentH,
		)
	}

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
	if formatBar != "" {
		body += "\n" + formatBar
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
	editKeyCursor int,
	editValue string,
	editValueCursor int,
	editColumn int,
	selectedKeys map[string]bool, // keys marked with `s` for batch copy; nil = none
	width, height int,
) string {
	// +2 budgets for the "✓ " / "  " selection-indicator prefix every
	// key row carries — without this the key text gets truncated even
	// when the underlying name fits.
	keyColW := computeKeyColumnWidth(cm.Keys, width, 3) + 2
	valColW := max(width-keyColW-5, 8)

	bodyHeight := max(height-2, 1)
	start := scrollWindowStart(selectedIdx, bodyHeight, len(cm.Keys))
	end := min(start+bodyHeight, len(cm.Keys))

	t := newKVEditorTable(keyColW, valColW, selectedIdx-start)
	for i := start; i < end; i++ {
		k := cm.Keys[i]
		v := cm.Data[k]
		displayV := configMapValueDisplay(v, valColW)

		// Consistent 2-char prefix across all rows (including editing)
		// so column alignment stays stable. See secretview for the
		// rationale.
		prefix := "  "
		if selectedKeys[k] {
			prefix = "\u2713 "
		}
		var keyText, valText string
		switch {
		case i == selectedIdx && editing && editColumn == 0:
			keyText = prefix + overlayCursor(editKey, editKeyCursor, true, keyColW-2)
			valText = SingleLineCell(editValue, valColW)
		case i == selectedIdx && editing && editColumn == 1:
			keyText = prefix + SingleLineCell(editKey, keyColW-2)
			valText = overlayCursor(editValue, editValueCursor, true, valColW)
		default:
			keyText = prefix + SingleLineCell(k, keyColW-2)
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
