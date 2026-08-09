package app

import (
	"slices"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// wkYAMLCtx is the YAML viewer's resolved which-key context. A field earns a
// place here on the same rule wkCtx follows: it is read by more than one
// predicate, or deriving it costs a walk of the whole document.
//
// visual is read by nearly every entry because handleYAMLKey routes to
// handleYAMLVisualKey BEFORE handleYAMLNormalKey (update_yaml.go:33-37), and
// the visual handler has no case for the normal-mode keys — pressing them
// there is a silent no-op, not a fallthrough.
type wkYAMLCtx struct {
	m      *Model
	visual bool
	// counted reports an armed digit prefix. handleYAMLNormalCopy consumes it
	// (update_yaml.go:108), so `y` copies N lines rather than one.
	counted bool
	// visibleLines is len(mapping) from buildVisibleLines — the bound
	// handleYAMLNormalCopy checks the cursor against (update_yaml.go:110-112).
	visibleLines int
	// foldSection is what handleYAMLKeyFoldToggle would toggle at the cursor;
	// empty means the key is a no-op there (update_yaml.go:305-307).
	foldSection string
	// foldable reports at least one multi-line section, the only thing
	// handleYAMLKeyZ acts on (update_yaml.go:685-699).
	foldable bool
	// sel and kind are the explorer row the viewer was opened over; ctrl+e and
	// the refresh/Object Explorer gates all re-derive them, and
	// selectedMiddleItem re-filters the whole row list on every call.
	sel  *model.Item
	kind string
}

// newWKYAMLCtx resolves the YAML viewer's row and document state once per
// availability pass. Safe on a zero-value Model: buildVisibleLines on empty
// content yields a single-line mapping, and selectedMiddleItem /
// selectedResourceKind both tolerate a zero Model.
func newWKYAMLCtx(m *Model) *wkYAMLCtx {
	c := &wkYAMLCtx{
		m:       m,
		visual:  m.yamlView.visualMode,
		counted: m.yamlView.lineInput != "",
	}
	_, mapping := buildVisibleLines(m.yamlView.content, m.yamlView.sections, m.yamlView.collapsed)
	c.visibleLines = len(mapping)
	c.foldSection = sectionAtScrollPos(m.yamlView.cursor, mapping, m.yamlView.sections)
	c.foldable = slices.ContainsFunc(m.yamlView.sections, isMultiLineSection)
	c.sel = m.selectedMiddleItem()
	c.kind = m.selectedResourceKind()
	return c
}

// wkYAMLNormal / wkYAMLVisual split the catalog along the one branch
// handleYAMLKey takes before anything else. Nearly every entry carries one of
// them, so they are named rather than inlined.
func wkYAMLNormal(c *wkYAMLCtx) bool { return !c.visual }
func wkYAMLVisual(c *wkYAMLCtx) bool { return c.visual }

// wkYAMLOnLine reports that the cursor sits on a real visible line, the guard
// handleYAMLNormalCopy applies before yanking anything (update_yaml.go:110-112).
func wkYAMLOnLine(c *wkYAMLCtx) bool {
	return c.m.yamlView.cursor >= 0 && c.m.yamlView.cursor < c.visibleLines
}

// wkYAMLObjectExplorerAvailable mirrors handleYAMLKeyObjectExplorer
// (update_yaml.go:593-612): it either returns to a tree the viewer was opened
// from, or calls openObjectExplorer (objectexplorer.go:98-107), which toasts
// and refuses above LevelResources or without a row carrying its Raw payload.
func wkYAMLObjectExplorerAvailable(c *wkYAMLCtx) bool {
	if c.visual {
		return false
	}
	if c.m.yamlReturnMode == modeObjectExplorer && c.m.objectExplorerView.root != nil {
		return true
	}
	return c.m.nav.Level >= model.LevelResources && c.sel != nil && c.sel.Raw != nil
}

// wkYAMLAPIExplorerAvailable mirrors openExplainAtObjectPath
// (update_explain.go:82-90): it toasts "Cannot determine resource type" when
// buildExplainResourceFromType returns nothing AND the navigated type has no
// Kind to pluralise either.
func wkYAMLAPIExplorerAvailable(c *wkYAMLCtx) bool {
	if c.visual {
		return false
	}
	rt := c.m.nav.ResourceType
	return rt.Resource != "" || rt.Kind != ""
}

// wkYAMLEditAvailable mirrors handleYAMLKeyCtrlE (update_yaml.go:289-301):
// it needs a resolvable kind and a highlighted row, and the read-only branch
// only toasts. m.readOnly is the field the handler itself reads — not
// readOnlyForContext, which resolves against a different context.
func wkYAMLEditAvailable(c *wkYAMLCtx) bool {
	return !c.visual && c.kind != "" && c.sel != nil && !c.m.readOnly
}

// wkYAMLRefreshAvailable mirrors handleYAMLRefresh (update_yaml.go:133-140),
// which does nothing when loadYAML returns nil — that is, on a security view,
// above LevelResources, or with no row at LevelResources/LevelOwned
// (commands_load.go:457-548). LevelContainers reads m.nav.OwnedName and needs
// no row.
func wkYAMLRefreshAvailable(c *wkYAMLCtx) bool {
	if c.visual || onSecurityView(c.m) {
		return false
	}
	switch c.m.nav.Level {
	case model.LevelResources, model.LevelOwned:
		return c.sel != nil
	case model.LevelContainers:
		return true
	default:
		return false
	}
}

// whichKeyYAMLActionList is the YAML viewer's catalog. Motions (j/k/h/l/w/b/e,
// gg/G, ctrl+d/u/f/b, 0/$/^, the digit prefix) and n/N are absent for the same
// reason they are absent from the explorer's: the panel advertises actions, not
// the keymap.
//
// The keys are read from the handler, not from the binding of the same name:
// "O" and "I" are HARDCODED in handleYAMLNormalKey (update_yaml.go:173-176)
// rather than read from kb.ObjectExplorer / kb.APIExplorer, so a user who
// rebinds those still presses "O"/"I" here and the panel must say so.
//
// esc is absent even though the visual handler cancels on it
// (update_yaml_visual.go:19-21): whichKeyLeaderIntercept CONSUMES esc while
// the panel is shown (whichkey_leader.go:169-181), so the first esc closes the
// panel rather than reaching the viewer. ctrl+c is absent for the same reason
// the explorer catalog omits it — it closes the tab or quits.
var whichKeyYAMLActionList = []wkAction[*wkYAMLCtx]{
	// update_yaml.go:171-172 — kb.Help and the f1 alias both reach help.
	{Key: whichKeyHelpKey, Label: "Full help", Group: wkViews, Avail: wkYAMLNormal},
	{Key: wkLiteralKey("O"), Label: "Object Explorer", Group: wkViews, Avail: wkYAMLObjectExplorerAvailable},
	{Key: wkLiteralKey("I"), Label: "API Explorer", Group: wkViews, Avail: wkYAMLAPIExplorerAvailable},
	{Key: func(kb ui.Keybindings) string { return kb.Refresh }, Label: "Re-fetch YAML", Group: wkActions, Avail: wkYAMLRefreshAvailable},
	{Key: wkLiteralKey("ctrl+e"), Label: "Edit in $EDITOR", Group: wkActions, Avail: wkYAMLEditAvailable},

	// update_yaml.go:202-205 — folds.
	{Key: func(kb ui.Keybindings) string { return kb.ToggleFold }, Label: "Fold section at cursor", Group: wkViews, Avail: func(c *wkYAMLCtx) bool { return !c.visual && c.foldSection != "" }},
	{Key: func(kb ui.Keybindings) string { return kb.ToggleFoldAll }, Label: "Fold / unfold all", Group: wkViews, Avail: func(c *wkYAMLCtx) bool { return !c.visual && c.foldable }},
	{Key: func(kb ui.Keybindings) string { return kb.ToggleWrap }, Label: "Toggle line wrapping", Group: wkSettings, Avail: wkYAMLNormal},
	{Key: wkLiteralKey("m"), Label: "Toggle field-manager blame", Group: wkSettings, Avail: wkYAMLNormal},

	// update_yaml.go:187-188 / 183-184 — search, then the two things q does.
	{Key: func(kb ui.Keybindings) string { return kb.Search }, Label: "Search in content", Group: wkFilter, Avail: wkYAMLNormal},
	{Key: wkLiteralKey("q"), Label: "Clear search", Group: wkFilter, Avail: func(c *wkYAMLCtx) bool {
		return !c.visual && c.m.yamlView.searchText.Value != ""
	}},
	{Key: wkLiteralKey("q"), Label: "Back", Group: wkViews, Avail: func(c *wkYAMLCtx) bool {
		return !c.visual && c.m.yamlView.searchText.Value == ""
	}},

	// Yank. The same key does three different things, so it is three entries
	// with mutually exclusive predicates rather than one vague label:
	// handleYAMLNormalCopy consumes a count prefix (update_yaml.go:107-128) and
	// handleYAMLVisualCopy yanks the selection (update_yaml_visual.go:126-151).
	{Key: wkLiteralKey("y"), Label: "Copy line", Group: wkActions, Avail: func(c *wkYAMLCtx) bool {
		return !c.visual && !c.counted && wkYAMLOnLine(c)
	}},
	{Key: wkLiteralKey("y"), Label: "Copy N lines", Group: wkActions, Avail: func(c *wkYAMLCtx) bool {
		return !c.visual && c.counted && wkYAMLOnLine(c)
	}},
	{Key: wkLiteralKey("y"), Label: "Copy selection", Group: wkActions, Avail: wkYAMLVisual},

	// Visual selection. update_yaml.go:177-182 enters it; the same three keys
	// switch or cancel the selection type once inside
	// (update_yaml_visual.go:28-33, handleYAMLVisualToggleMode).
	{Key: wkLiteralKey("v"), Label: "Visual select", Group: wkSelection, Avail: wkYAMLNormal},
	{Key: wkLiteralKey("V"), Label: "Visual line select", Group: wkSelection, Avail: wkYAMLNormal},
	{Key: wkLiteralKey("ctrl+v"), Label: "Visual block select", Group: wkSelection, Avail: wkYAMLNormal},
	{Key: wkLiteralKey("v"), Label: "Char selection", Group: wkSelection, Avail: wkYAMLVisual},
	{Key: wkLiteralKey("V"), Label: "Line selection", Group: wkSelection, Avail: wkYAMLVisual},
	{Key: wkLiteralKey("ctrl+v"), Label: "Block selection", Group: wkSelection, Avail: wkYAMLVisual},
	// update_yaml_visual.go:22-27 — i/a arm a text object that w/W completes
	// (consumeTextObjectPrelude, update_vim.go:272-283).
	{Key: wkLiteralKey("i"), Label: "Inner word (iw/iW)", Group: wkSelection, Avail: wkYAMLVisual},
	{Key: wkLiteralKey("a"), Label: "Around word (aw/aW)", Group: wkSelection, Avail: wkYAMLVisual},
}

// whichKeyYAMLCatalog is the YAML viewer's registry entry. The search prompt
// (handleYAMLSearchInput, update_yaml.go:28-30) swallows every printable key,
// so "?" typed there is part of the query.
var whichKeyYAMLCatalog = wkCatalog[*wkYAMLCtx]{
	resolve: newWKYAMLCtx,
	input:   func(m *Model) bool { return m.yamlView.searchMode },
	actions: whichKeyYAMLActionList,
}
