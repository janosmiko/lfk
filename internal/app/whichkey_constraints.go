package app

import "github.com/janosmiko/lfk/internal/ui"

// wkConstraintsCtx is the fullscreen constraints view's which-key context.
// The view has no gated modes (no visual/search/filter), so it carries
// nothing beyond the Model pointer every resolver needs.
type wkConstraintsCtx struct{ m *Model }

func newWKConstraintsCtx(m *Model) *wkConstraintsCtx { return &wkConstraintsCtx{m: m} }

// whichKeyConstraintsActionList excludes Enter: drilling into the row's
// object is list-viewer navigation, the same exclusion Log Top and the
// Object/API Explorers apply to their own Enter.
var whichKeyConstraintsActionList = []wkAction[*wkConstraintsCtx]{
	{Key: func(kb ui.Keybindings) string { return kb.Refresh }, Label: "Re-run the scan", Group: wkActions},
	{Key: wkLiteralKey("q"), Label: "Back to explorer", Group: wkViews},
}

// whichKeyConstraintsCatalog is the constraints view's registry entry.
// Registered for modeConstraints only.
var whichKeyConstraintsCatalog = wkCatalog[*wkConstraintsCtx]{
	resolve: newWKConstraintsCtx,
	actions: whichKeyConstraintsActionList,
}
