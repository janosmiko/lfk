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
// searchQuery / searchActive drive the / filter: when searchQuery is
// non-empty, only keys containing it (case-insensitive substring) are
// shown. searchActive draws the input cursor block in the search bar
// while the user is typing. Cursor is interpreted as an index into the
// FILTERED key list — the caller (Model) keeps that invariant.
func RenderSecretEditorOverlay(
	secret *model.SecretData,
	cursor int,
	revealedKeys map[string]bool,
	allRevealed bool,
	editing bool,
	editKey string,
	editValue string,
	editColumn int, // 0=key, 1=value
	searchQuery string,
	searchActive bool,
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

	// Reserve a row for the search bar when the / filter is in use.
	searchBar := RenderKVEditorSearchBar(searchQuery, searchActive)
	searchH := 0
	if searchBar != "" {
		searchH = 1
	}

	panelContentH := max(boxH-outerPadH-innerPadH-titleH-gapH-searchH, 3)
	panelContentW := max(boxW-outerPadW-innerPadW, 20)
	panelW := boxW - outerPadW

	// Title — bg overridden to baseBg so the title row (and its 1-row
	// bottom padding from OverlayTitleStyle) match the rest of the
	// editor's baseBg surface; the stock OverlayTitleStyle uses
	// surfaceBg, which would re-introduce the very mismatch the panel
	// fix above eliminates.
	title := OverlayTitleStyle.Background(BaseBg).Render("Secret Editor")

	// Filter keys before passing to the table renderer so the cursor +
	// row iteration land on the visible subset.
	visibleKeys := FilterKVKeys(secret.Keys, searchQuery)
	filteredSecret := &model.SecretData{Keys: visibleKeys, Data: secret.Data}

	// Data table content.
	dataContent := renderSecretEditorTable(
		filteredSecret, cursor, revealedKeys, allRevealed,
		editing, editKey, editValue, editColumn,
		panelContentW, panelContentH,
	)

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
	width, height int,
) string {
	keyColW := computeKeyColumnWidth(secret.Keys, width, 3)
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
			keyText = Truncate(editKey, keyColW) + "█"
			valText = Truncate(editValue, valColW)
		case i == selectedIdx && editing && editColumn == 1:
			keyText = Truncate(editKey, keyColW)
			valText = Truncate(editValue, valColW) + "█"
		default:
			keyText = Truncate(k, keyColW)
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
func secretValueDisplay(val string, revealed bool, maxW int) string {
	if revealed {
		return Truncate(val, maxW)
	}
	return "********"
}
