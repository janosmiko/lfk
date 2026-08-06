package app

import (
	"slices"
	"testing"

	"github.com/janosmiko/lfk/internal/logagg"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// Per-viewer behaviour for the second batch of catalogs (diff, API Explorer,
// Object Explorer, Log Top, event viewer). The registry-wide sweeps live in
// whichkey_viewers_test.go and pick these viewers up automatically; what is
// here is the one assertion per predicate that its handler's gate was read
// correctly.

// --- Exec terminal (deliberately uncatalogued) ---

// TestWhichKeyExec_LeaderNeverArmsInThePTY is the executable half of the
// exec-mode decision. handleExecKey forwards every unclaimed keystroke into the
// PTY (ptyexec.go:186-202), so the leader must stay unarmed there and leave "?"
// to the program running inside — a shell prompt, a pager's help, vim's
// register. Giving exec a catalog would consume "?" one dispatch step earlier
// (update_keys.go:71-77) and the PTY would never see it.
func TestWhichKeyExec_LeaderNeverArmsInThePTY(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 0

	m := whichKeyTestModel()
	m.mode = modeExec
	out, _ := m.handleKey(leaderKey())
	got := out.(Model)
	if got.whichKey.armed || got.whichKey.shown {
		t.Errorf("the leader must not arm in the exec terminal; armed=%v shown=%v", got.whichKey.armed, got.whichKey.shown)
	}
	if got.mode != modeExec {
		t.Errorf("the key must fall through to handleExecKey, not change mode; got %v", got.mode)
	}
}

// --- Diff view ---

func TestWhichKeyDiff_VisualModeSwapsTheCatalog(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	normal := whichKeyOffered(whichKeyViewerModel(modeDiff))
	for _, want := range []string{"Copy line", "Visual select", "Search in content", "Toggle line wrapping", "Back", "Unified diff"} {
		if !slices.Contains(normal, want) {
			t.Errorf("normal mode must offer %q; got %v", want, normal)
		}
	}

	m := whichKeyViewerModel(modeDiff)
	m.diffView.visualMode = true
	visual := whichKeyOffered(m)
	if !slices.Contains(visual, "Copy selection") {
		t.Errorf("visual mode must offer %q; got %v", "Copy selection", visual)
	}
	// handleDiffVisualKey has no case for any of these — not even q, which the
	// log viewer does answer in visual mode.
	for _, banned := range []string{"Copy line", "Search in content", "Toggle line wrapping", "Back", "Full help", "Unified diff", "Switch active side", "Fold / unfold all"} {
		if slices.Contains(visual, banned) {
			t.Errorf("visual mode must not offer %q; the visual handler has no case for it", banned)
		}
	}
}

// handleDiffNormalCopy skips lines whose ACTIVE side is empty, so on a
// side-by-side diff the yank is a no-op over a line the other side owns.
func TestWhichKeyDiff_YankFollowsTheActiveSide(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyViewerModel(modeDiff)
	m.diffView.left = "only: left\n"
	m.diffView.right = "only: right\n"
	m.diffView.cursor = 0
	m.diffView.cursorSide = 0
	if !slices.Contains(whichKeyOffered(m), "Copy line") {
		t.Fatalf("the left side has text at the cursor; got %v", whichKeyOffered(m))
	}

	// The removed line renders on the left only, so with the cursor on the
	// right pane the same row yields nothing to copy.
	m.diffView.cursorSide = 1
	if slices.Contains(whichKeyOffered(m), "Copy line") {
		t.Errorf("the right side is empty at this row; the yank copies nothing there")
	}

	off := whichKeyViewerModel(modeDiff)
	off.diffView.cursor = 999
	if labels := whichKeyOffered(off); slices.Contains(labels, "Copy line") || slices.Contains(labels, "Copy N lines") {
		t.Errorf("the yank must be hidden past the last visible line; got %v", labels)
	}
}

func TestWhichKeyDiff_CountPrefixRelabelsTheYank(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyViewerModel(modeDiff)
	m.diffView.lineInput = "4"
	labels := whichKeyOffered(m)
	if !slices.Contains(labels, "Copy N lines") || slices.Contains(labels, "Copy line") {
		t.Errorf("with a count armed, handleDiffNormalCopy consumes it; got %v", labels)
	}
}

// The counted yank has TWO silent no-op paths, not one: running off the end of
// the visible list, and a range whose every line is empty on the active side —
// handleDiffNormalCopy skips those and returns unchanged on an empty parts
// slice (update_diff.go:510-512). A run of consecutive added lines read from
// the left pane is exactly that shape, so the predicate must scan the range,
// not just bound it.
func TestWhichKeyDiff_CountedYankNeedsTextOnTheActiveSide(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	base := func() Model {
		m := whichKeyViewerModel(modeDiff)
		m.diffView.left = ""
		m.diffView.right = "a: 1\nb: 2\nc: 3\n"
		m.diffView.foldState = nil
		m.diffView.cursor = 0
		m.diffView.lineInput = "3"
		return m
	}

	empty := base()
	empty.diffView.cursorSide = 0
	if labels := whichKeyOffered(empty); slices.Contains(labels, "Copy N lines") {
		t.Errorf("every line in the counted range is empty on the left; the yank copies nothing. got %v", labels)
	}

	full := base()
	full.diffView.cursorSide = 1
	if labels := whichKeyOffered(full); !slices.Contains(labels, "Copy N lines") {
		t.Errorf("the right side carries all three lines; the yank copies them. got %v", labels)
	}

	// A range that STRADDLES an empty line still copies the rest, so one
	// non-empty line anywhere in it is enough to keep the entry offered.
	straddle := whichKeyViewerModel(modeDiff)
	straddle.diffView.left = "kind: Pod\n"
	straddle.diffView.right = "kind: Pod\nadded: 1\nadded: 2\n"
	straddle.diffView.foldState = nil
	straddle.diffView.cursor = 0
	straddle.diffView.cursorSide = 0
	straddle.diffView.lineInput = "3"
	if labels := whichKeyOffered(straddle); !slices.Contains(labels, "Copy N lines") {
		t.Errorf("the first line has text on the left; the yank copies it. got %v", labels)
	}
}

// toggleDiffFoldAtCursor needs the cursor inside a fold region;
// toggleAllDiffFolds needs at least one region to exist.
func TestWhichKeyDiff_FoldEntriesFollowTheDocument(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyViewerModel(modeDiff) // cursor 0, inside the unchanged run
	if labels := whichKeyOffered(m); !slices.Contains(labels, "Fold unchanged block at cursor") {
		t.Errorf("a cursor inside an unchanged run must offer the fold; got %v", labels)
	}

	changed := whichKeyViewerModel(modeDiff)
	changed.diffView.cursor = 6 // the changed "name:" line, outside every region
	if labels := whichKeyOffered(changed); slices.Contains(labels, "Fold unchanged block at cursor") {
		t.Errorf("a changed line belongs to no fold region; got %v", labels)
	}

	flat := whichKeyViewerModel(modeDiff)
	flat.diffView.left, flat.diffView.right = "a: 1\n", "a: 2\n"
	labels := whichKeyOffered(flat)
	if slices.Contains(labels, "Fold / unfold all") {
		t.Errorf("a diff with no unchanged run has nothing to fold; got %v", labels)
	}
	if slices.Contains(labels, "Fold unchanged block at cursor") {
		t.Errorf("a diff with no unchanged run has no region at the cursor; got %v", labels)
	}
}

// tab switches the pane the cursor acts on, which unified mode does not have.
func TestWhichKeyDiff_SideSwitchAndUnifiedLabelFollowTheLayout(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	side := whichKeyOffered(whichKeyViewerModel(modeDiff))
	if !slices.Contains(side, "Switch active side") || !slices.Contains(side, "Unified diff") {
		t.Errorf("side-by-side offers the side switch and the unified toggle; got %v", side)
	}

	m := whichKeyViewerModel(modeDiff)
	m.diffView.unified = true
	unified := whichKeyOffered(m)
	if slices.Contains(unified, "Switch active side") {
		t.Errorf("unified mode has one column; tab is a no-op there; got %v", unified)
	}
	if !slices.Contains(unified, "Side-by-side diff") || slices.Contains(unified, "Unified diff") {
		t.Errorf("the toggle names the layout it switches to; got %v", unified)
	}
}

// --- API Explorer ---

// toggleExplainTree has three outcomes, and which one runs is decided by the
// tree state plus whether a fetch is already in flight.
func TestWhichKeyExplain_TreeToggleReadsItsState(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	flat := whichKeyOffered(whichKeyViewerModel(modeExplain))
	if !slices.Contains(flat, "Field tree view") {
		t.Errorf("a flat level offers the tree; got %v", flat)
	}

	pending := whichKeyViewerModel(modeExplain)
	pending.explainTreeWanted = true
	if labels := whichKeyOffered(pending); !slices.Contains(labels, "Cancel field tree load") {
		t.Errorf("a second press cancels the in-flight fetch; got %v", labels)
	}

	tree := whichKeyViewerModel(modeExplain)
	tree.explainTree = true
	if labels := whichKeyOffered(tree); !slices.Contains(labels, "Flat field list") || slices.Contains(labels, "Field tree view") {
		t.Errorf("in the tree the key leaves it; got %v", labels)
	}
}

// toggleExplainTreeFold is a no-op outside tree mode and on a row with no
// children in the loaded subtree.
func TestWhichKeyExplain_FoldNeedsATreeRowWithChildren(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	if labels := whichKeyOffered(whichKeyViewerModel(modeExplain)); slices.Contains(labels, "Fold subtree at cursor") {
		t.Errorf("the flat list has no folds; got %v", labels)
	}

	leaf := whichKeyViewerModel(modeExplain)
	leaf.explainTree = true
	leaf.explainTreeAll = []model.ExplainField{{Name: "spec", Path: "spec"}}
	if labels := whichKeyOffered(leaf); slices.Contains(labels, "Fold subtree at cursor") {
		t.Errorf("a leaf row has nothing to fold; got %v", labels)
	}

	parent := whichKeyViewerModel(modeExplain)
	parent.explainTree = true
	parent.explainTreeAll = []model.ExplainField{
		{Name: "spec", Path: "spec"},
		{Name: "containers", Path: "spec.containers"},
	}
	if labels := whichKeyOffered(parent); !slices.Contains(labels, "Fold subtree at cursor") {
		t.Errorf("a row with children folds; got %v", labels)
	}
}

// The API Explorer matches a literal "?" rather than kb.Help, so a rebind of
// the help key must not move the row the panel draws.
func TestWhichKeyExplain_HelpKeyIgnoresTheHelpRebind(t *testing.T) {
	restoreWhichKeyGlobals(t)
	kb := ui.DefaultKeybindings()
	kb.Help = "H"
	ui.ActiveKeybindings = kb

	keys := whichKeyOfferedKeys(whichKeyViewerModel(modeExplain))
	if !slices.Contains(keys, "f1") {
		t.Errorf("with the leader on \"?\", f1 is the only way into help here; got %v", keys)
	}
	if slices.Contains(keys, "H") {
		t.Errorf("the rebound help key does nothing in the API Explorer; got %v", keys)
	}

	// Move the leader off "?" and the literal case becomes reachable again.
	kb.WhichKeyLeader = "\\"
	ui.ActiveKeybindings = kb
	if keys := whichKeyOfferedKeys(whichKeyViewerModel(modeExplain)); !slices.Contains(keys, "?") {
		t.Errorf("with the leader elsewhere, \"?\" reaches help again; got %v", keys)
	}
}

// TestWhichKeyLevelledViewers_QuitLabelsSayCloseNotBack pins the label against
// its handler in the two viewers that have levels. handleExplainKeyQ and
// exitObjectExplorer both leave outright at any depth; the step back one level
// is esc's, and esc is never advertised (whichKeyLeaderIntercept eats it while
// the panel is shown). "Back" there named the wrong key's behaviour.
func TestWhichKeyLevelledViewers_QuitLabelsSayCloseNotBack(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	for mode, want := range map[viewMode]string{
		modeExplain:        "Close API Explorer",
		modeObjectExplorer: "Close Object Explorer",
	} {
		labels := whichKeyOffered(whichKeyViewerModel(mode))
		if !slices.Contains(labels, want) {
			t.Errorf("%s must label q %q; got %v", whichKeyModeNames[mode], want, labels)
		}
		if slices.Contains(labels, "Back") {
			t.Errorf("%s must not label q %q — esc is what walks back; got %v", whichKeyModeNames[mode], "Back", labels)
		}
	}
}

// --- Object Explorer ---

func TestWhichKeyObjectExplorer_YanksNeedACursorNode(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyViewerModel(modeObjectExplorer)
	labels := whichKeyOffered(m)
	if !slices.Contains(labels, "Copy node path") || !slices.Contains(labels, "Copy node YAML") {
		t.Fatalf("a selected node offers both yanks; got %v", labels)
	}

	off := whichKeyViewerModel(modeObjectExplorer)
	off.objectExplorerView.cursor = 999
	labels = whichKeyOffered(off)
	if slices.Contains(labels, "Copy node path") || slices.Contains(labels, "Copy node YAML") {
		t.Errorf("selected() fails past the last row, so both yanks no-op; got %v", labels)
	}
}

func TestWhichKeyObjectExplorer_FoldNeedsAnUnfilteredTreeRowWithChildren(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	if labels := whichKeyOffered(whichKeyViewerModel(modeObjectExplorer)); slices.Contains(labels, "Fold subtree at cursor") {
		t.Errorf("the flat level has no folds; got %v", labels)
	}

	tree := whichKeyViewerModel(modeObjectExplorer)
	tree.objectExplorerView.tree = true
	tree.objectExplorerView.rebuildTreeRows()
	tree.objectExplorerView.cursorOnTreeSegs([]string{"metadata"})
	if labels := whichKeyOffered(tree); !slices.Contains(labels, "Fold subtree at cursor") {
		t.Errorf("a tree row with children folds; got %v", labels)
	}

	// toggleObjectExplorerTreeFold refuses outright while a filter is applied.
	filtered := tree
	filtered.objectExplorerView.filter = "name"
	if labels := whichKeyOffered(filtered); slices.Contains(labels, "Fold subtree at cursor") {
		t.Errorf("folding is refused while filtering; got %v", labels)
	}
}

func TestWhichKeyObjectExplorer_LiveRefreshLabelFollowsTheState(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	// The test model carries the production default (live on, seeded from
	// ui.ConfigObjectExplorerLive), so this direction comes first.
	m := whichKeyViewerModel(modeObjectExplorer)
	if labels := whichKeyOffered(m); !slices.Contains(labels, "Live refresh off") || slices.Contains(labels, "Live refresh on") {
		t.Errorf("on: got %v", labels)
	}
	m.objectExplorerLive = false
	if labels := whichKeyOffered(m); !slices.Contains(labels, "Live refresh on") || slices.Contains(labels, "Live refresh off") {
		t.Errorf("off: got %v", labels)
	}
}

// openExplainAtObjectPath toasts "Cannot determine resource type" when the
// navigated type has neither a plural resource nor a Kind to pluralise.
func TestWhichKeyObjectExplorer_APIExplorerNeedsAResourceType(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyViewerModel(modeObjectExplorer)
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
	if !slices.Contains(whichKeyOffered(m), "API Explorer") {
		t.Errorf("a Kind alone is enough; got %v", whichKeyOffered(m))
	}
	m.nav.ResourceType = model.ResourceTypeEntry{}
	if slices.Contains(whichKeyOffered(m), "API Explorer") {
		t.Errorf("with no resource type the key only toasts; got %v", whichKeyOffered(m))
	}
}

// --- Log Top ---

// handleLogTopKey pops a drill frame before it returns to the log viewer.
func TestWhichKeyLogTop_QuitLabelFollowsTheDrillStack(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyViewerModel(modeLogTop)
	if labels := whichKeyOffered(m); !slices.Contains(labels, "Back to log viewer") || slices.Contains(labels, "Pop drill level") {
		t.Errorf("with no drill frame q returns to the viewer; got %v", labels)
	}
	m.logTop.drillStack = []logTopDrillFrame{{groupBy: []string{logagg.FieldPath}}}
	if labels := whichKeyOffered(m); !slices.Contains(labels, "Pop drill level") || slices.Contains(labels, "Back to log viewer") {
		t.Errorf("with a drill frame q pops it; got %v", labels)
	}
}

// logTopCycleDrillTarget returns unchanged when every dimension is pinned.
func TestWhichKeyLogTop_DrillCycleNeedsACandidate(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	if labels := whichKeyOffered(whichKeyViewerModel(modeLogTop)); !slices.Contains(labels, "Cycle drill dimension") {
		t.Errorf("an unpinned dimension is left; got %v", labels)
	}

	pinned := whichKeyViewerModel(modeLogTop)
	pinned.logTop.displayDims = pinned.logTop.groupBy
	if labels := whichKeyOffered(pinned); slices.Contains(labels, "Cycle drill dimension") {
		t.Errorf("every dimension is pinned; the key is a no-op; got %v", labels)
	}
}

// logTopCycleSort returns unchanged with no sortable column, but the flip and
// reset keys write the sort state either way.
func TestWhichKeyLogTop_SortCycleNeedsAColumn(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyViewerModel(modeLogTop)
	m.logTop.colHidden = map[string]bool{}
	for _, c := range m.logTopColumnList() {
		m.logTop.colHidden[c] = true
	}
	labels := whichKeyOffered(m)
	for _, banned := range []string{"Sort next column", "Sort previous column"} {
		if slices.Contains(labels, banned) {
			t.Errorf("with every column hidden %q cycles nothing; got %v", banned, labels)
		}
	}
	for _, want := range []string{"Flip sort direction", "Reset sort"} {
		if !slices.Contains(labels, want) {
			t.Errorf("%q writes the sort state regardless; got %v", want, labels)
		}
	}
}

// Log Top has no help case at all, so the panel must not claim it has one.
func TestWhichKeyLogTop_OffersNoHelpRow(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	if labels := whichKeyOffered(whichKeyViewerModel(modeLogTop)); slices.Contains(labels, "Full help") {
		t.Errorf("handleLogTopKey has no help case; got %v", labels)
	}
}

// --- Event viewer ---

func TestWhichKeyEventViewer_VisualModeSwapsTheCatalog(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyViewerModel(modeEventViewer)
	m.eventTimelineVisualMode = 'v'
	visual := whichKeyOffered(m)
	if !slices.Contains(visual, "Copy selection") || !slices.Contains(visual, "Cancel selection") {
		t.Errorf("visual mode must offer the yank and the q-cancel; got %v", visual)
	}
	for _, banned := range []string{"Copy line", "Search in content", "Toggle line wrapping", "Back to explorer", "Full help"} {
		if slices.Contains(visual, banned) {
			t.Errorf("visual mode must not offer %q; the visual handler has no case for it", banned)
		}
	}
	// handleEventViewerModeKey claims the fullscreen toggle BEFORE the visual
	// routing, so this one key survives where every other normal-mode key dies.
	if !slices.Contains(visual, "Minimize to overlay") {
		t.Errorf("the fullscreen toggle is claimed ahead of the visual handler; got %v", visual)
	}
}

// The clear-search branch is keyed to esc only, so q leaves the viewer whether
// or not a search is applied — unlike the YAML and describe viewers.
func TestWhichKeyEventViewer_QuitDoesNotSplitOnTheSearch(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyViewerModel(modeEventViewer)
	m.eventTimelineSearchQuery = "Warning"
	labels := whichKeyOffered(m)
	if !slices.Contains(labels, "Back to explorer") {
		t.Errorf("q still leaves with a search applied; got %v", labels)
	}
	if slices.Contains(labels, "Clear search") {
		t.Errorf("only esc clears the search here, and esc is never advertised; got %v", labels)
	}
}

func TestWhichKeyEventViewer_YankNeedsACursorLine(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyViewerModel(modeEventViewer)
	m.eventTimelineLines = nil
	if labels := whichKeyOffered(m); slices.Contains(labels, "Copy line") {
		t.Errorf("an empty timeline must hide the yank; got %v", labels)
	}

	counted := whichKeyViewerModel(modeEventViewer)
	counted.eventTimelineLineInput = "3"
	if labels := whichKeyOffered(counted); !slices.Contains(labels, "Copy N lines") || slices.Contains(labels, "Copy line") {
		t.Errorf("with a count armed the yank copies a range; got %v", labels)
	}
}
