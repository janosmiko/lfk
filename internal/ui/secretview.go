package ui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
)

// secretInnerPanelStyle is the bordered panel containing the secret table.
//
// The bg fields are NOT set here — they're chained at render time
// (see RenderSecretEditorOverlay) so the value comes from the active
// theme. This var is initialised at package-load, before ApplyTheme
// runs, so SurfaceBg / BaseBg are NoColor at this moment.
var secretInnerPanelStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color(ColorBorder)).
	Padding(0, 1)

// RenderSecretEditorOverlay renders a centered popup overlay for editing secrets.
//
//   - searchQuery / searchActive: the / filter.
//   - editKeyCursor / editValueCursor: byte offsets inside editKey /
//     editValue where the "█" cursor block lands.
//   - selected: keys marked with `s` for batch copy; rendered with a
//     "✓" prefix in the key column. May be nil.
//   - formatActive / formatCursor: drive the Shift+Y format-picker
//     chip row, rendered above the inner panel when formatActive.
//
// Cursor is interpreted as an index into the FILTERED key list — the
// caller (Model) keeps that invariant.
func RenderSecretEditorOverlay(
	secret *model.SecretData,
	cursor int,
	revealedKeys map[string]bool,
	allRevealed bool,
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

	// Reserve a row for the search bar when the / filter is in use,
	// and a row for the Shift+Y format-picker chip strip when the
	// user is choosing a copy format.
	searchBar := RenderKVEditorSearchBar(searchQuery, searchActive)
	searchH := 0
	if searchBar != "" {
		searchH = 1
	}
	var formatBar string
	formatH := 0
	if formatActive {
		formatBar = RenderKVFormatPicker(formatCursor)
		formatH = 1
	}

	panelContentH := max(boxH-outerPadH-innerPadH-titleH-gapH-searchH-formatH, 3)
	panelContentW := max(boxW-outerPadW-innerPadW, 20)
	panelW := boxW - outerPadW

	// Title — bg overridden to baseBg so the title row (and its 1-row
	// bottom padding from OverlayTitleStyle) match the rest of the
	// editor's baseBg surface; the stock OverlayTitleStyle uses
	// surfaceBg, which would re-introduce the very mismatch the panel
	// fix above eliminates.
	title := OverlayTitleStyle.Background(BaseBg).Render("Secret Editor")

	// Editing swaps the compact key/value table for a focused edit
	// pane that renders the value with embedded newlines preserved.
	// Without this swap, multi-line values would either collapse
	// (SingleLineCell hides the line breaks) or expand the table
	// vertically and break the editor's outer dimensions.
	var dataContent string
	if editing {
		dataContent = RenderKVEditorEditPane(
			editKey, editKeyCursor,
			editValue, editValueCursor,
			editColumn, panelContentW, panelContentH,
		)
	} else {
		// Filter keys before passing to the table renderer so the
		// cursor + row iteration land on the visible subset.
		visibleKeys := FilterKVKeys(secret.Keys, searchQuery)
		filteredSecret := &model.SecretData{Keys: visibleKeys, Data: secret.Data}
		dataContent = renderSecretEditorTable(
			filteredSecret, cursor, revealedKeys, allRevealed,
			false, "", "", 0,
			selected,
			panelContentW, panelContentH,
		)
	}

	// Inner bordered panel — bg + border-bg pulled from the active theme
	// at render time (the package-level secretInnerPanelStyle is bare so
	// it stays valid before ApplyTheme runs).
	innerPanel := secretInnerPanelStyle.
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

	// Match the inner panel's baseBg end-to-end so the editor reads as
	// one uniform surface — the package-default OverlayStyle uses
	// surfaceBg, which produces a visible "darker frame around lighter
	// inner box" mismatch.
	return OverlayStyle.
		Background(BaseBg).
		BorderBackground(BaseBg).
		Width(boxW).
		Render(body)
}

// renderSecretEditorTable renders the key-value table inside the secret editor.
//
// Uses lipgloss/table instead of hand-rolled ASCII separators so the
// borders, header underline, and per-cell styling come from a real
// table primitive (cleaner alignment, theme-aware borders, no manual
// `-` / `|` glue).
//
// The table package renders all rows it's given, so we pre-truncate to
// the visible window based on cursor position. Cursor-row + editing
// cell styling are applied via StyleFunc; the cursor block ('█') for
// inline editing is injected into the row text itself because it must
// land at a specific character position the style system can't reach.
func renderSecretEditorTable(
	secret *model.SecretData,
	selectedIdx int,
	revealedKeys map[string]bool,
	allRevealed bool,
	editing bool,
	editKey string,
	editValue string,
	editColumn int,
	selectedKeys map[string]bool, // keys marked with `s` for batch copy; nil = none
	width, height int,
) string {
	// +2 for the "✓ " / "  " selection-indicator prefix every key
	// row carries — without this the key text gets truncated even
	// when the underlying name fits, because the prefix steals two
	// of the column's visible chars.
	keyColW := computeKeyColumnWidth(secret.Keys, width, 3) + 2
	// Width budget left after the key column: subtract column divider
	// + 2*2 padding cells per column = ~5 chars of table chrome.
	valColW := max(width-keyColW-5, 8)

	bodyHeight := max(height-2, 1) // -1 header, -1 header underline
	start := scrollWindowStart(selectedIdx, bodyHeight, len(secret.Keys))
	end := min(start+bodyHeight, len(secret.Keys))

	t := newKVEditorTable(keyColW, valColW, selectedIdx-start)
	for i := start; i < end; i++ {
		k := secret.Keys[i]
		v := secret.Data[k]
		var keyText, valText string
		switch {
		case i == selectedIdx && editing && editColumn == 0:
			// "█" cursor block has visual width 1 — reserve a column for
			// it inside maxW so the cell stays at exactly keyColW chars.
			keyText = SingleLineCell(editKey, keyColW-1) + "█"
			valText = SingleLineCell(editValue, valColW)
		case i == selectedIdx && editing && editColumn == 1:
			keyText = SingleLineCell(editKey, keyColW)
			valText = SingleLineCell(editValue, valColW-1) + "█"
		default:
			// Reserve 2 cols for the "✓ " / "  " selection prefix so
			// adding a checkmark on selected rows doesn't push the key
			// text past the cell width.
			prefix := "  "
			if selectedKeys[k] {
				prefix = "✓ "
			}
			keyText = prefix + SingleLineCell(k, keyColW-2)
			valText = secretValueDisplay(v, revealedKeys[k] || allRevealed, valColW)
		}
		t.Row(keyText, valText)
	}
	rendered := t.Render()
	if len(secret.Keys) == 0 {
		// Headers always rendered (above) so the user sees the column
		// labels; placeholder hint sits below them on its own line.
		return rendered + "\n" + BarDimStyle.Render("  (empty - press 'a' to add a key)")
	}
	return rendered
}

// secretValueDisplay returns the display string for a secret value.
// Hidden values render as the fixed mask; revealed values flow through
// SingleLineCell so embedded newlines (multi-line secret payloads like
// kubeconfig values) don't expand the table cell vertically.
func secretValueDisplay(val string, revealed bool, maxW int) string {
	if revealed {
		return SingleLineCell(val, maxW)
	}
	return "********"
}
