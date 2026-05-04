package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
)

// labelInnerPanelStyle is the bordered panel containing the label/annotation table.
//
// The bg fields are NOT set here — they're chained at render time so
// the value comes from the active theme (this var is initialised
// before ApplyTheme runs, so theme bgs would be NoColor).
var labelInnerPanelStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color(ColorBorder)).
	Padding(0, 1)

// RenderLabelEditorOverlay renders the label/annotation editor popup.
//
// searchQuery / searchActive drive the / filter; selected /
// formatActive / formatCursor drive the multi-row Shift+Y copy
// (see RenderSecretEditorOverlay for the contract). The filter
// narrows the keys for the ACTIVE tab (labels or annotations);
// switching tabs preserves the query but clears the selection (the
// next slice will scope selection per-tab if needed).
func RenderLabelEditorOverlay(
	data *model.LabelAnnotationData,
	cursor int,
	tab int, // 0=labels, 1=annotations
	editing bool,
	editKey string,
	editKeyCursor int,
	editValue string,
	editValueCursor int,
	editColumn int,
	searchQuery string,
	searchActive bool,
	selected map[string]bool,
	formatActive bool,
	formatCursor int,
	editValueScroll int,
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
	title := titleStyle.Render("Label / Annotation Editor")

	// Tab bar — inactive tab and the separator both use bg-bound styles
	// so the row's bg matches the surrounding overlay (DimStyle is fg-
	// only and would let terminal default bg leak between the two tabs).
	labelsTab := fmt.Sprintf(" Labels (%d) ", len(data.LabelKeys))
	annotsTab := fmt.Sprintf(" Annotations (%d) ", len(data.AnnotKeys))
	if tab == 0 {
		labelsTab = OverlaySelectedStyle.Render(labelsTab)
		annotsTab = BarDimStyle.Render(annotsTab)
	} else {
		labelsTab = BarDimStyle.Render(labelsTab)
		annotsTab = OverlaySelectedStyle.Render(annotsTab)
	}
	tabBar := labelsTab + BarNormalStyle.Render("  ") + annotsTab

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

	// Mode selection while editing: pane for multi-line values,
	// inline table edit for single-line. Same contract as Secret /
	// ConfigMap — see secretview.go for the full rationale.
	var dataContent string
	switch {
	case editing && strings.Contains(editValue, "\n"):
		dataContent = RenderKVEditorEditPane(
			editKey, editKeyCursor,
			editValue, editValueCursor,
			editColumn, editValueScroll, panelContentW, panelContentH,
		)
	default:
		visibleKeys := FilterKVKeys(keys, searchQuery)
		dataContent = renderLabelEditorTable(visibleKeys, dataMap, cursor, editing, editKey, editKeyCursor, editValue, editValueCursor, editColumn, selected, panelContentW, panelContentH)
	}

	// Inner bordered panel — bg + border-bg pulled from the active
	// theme at render time. See secretview for the full rationale.
	innerPanel := labelInnerPanelStyle.
		Background(BaseBg).
		BorderBackground(BaseBg).
		Width(panelW).
		Height(panelContentH).
		Render(dataContent)

	body := title + "\n" + tabBar
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

func renderLabelEditorTable(keys []string, data map[string]string, selectedIdx int, editing bool, editKey string, editKeyCursor int, editValue string, editValueCursor int, editColumn int, selectedKeys map[string]bool, width, height int) string {
	// +2 budgets for the "✓ " / "  " selection-indicator prefix.
	keyColW := computeKeyColumnWidth(keys, width, 2) + 2
	valColW := max(width-keyColW-5, 8)

	bodyHeight := max(height-2, 1)
	start := scrollWindowStart(selectedIdx, bodyHeight, len(keys))
	end := min(start+bodyHeight, len(keys))

	t := newKVEditorTable(keyColW, valColW, selectedIdx-start)
	for i := start; i < end; i++ {
		k := keys[i]
		v := data[k]

		// Consistent 2-char prefix across all rows (including editing)
		// so column alignment stays stable. See secretview.
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
			valText = SingleLineCell(v, valColW)
		}
		t.Row(keyText, valText)
	}
	rendered := t.Render()
	if len(keys) == 0 {
		return rendered + "\n" + BarDimStyle.Render("  (empty - press 'a' to add)")
	}
	return rendered
}
