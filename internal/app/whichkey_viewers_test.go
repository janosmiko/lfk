package app

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/ui"
)

// whichKeyModeNames maps each catalogued mode to the registry's own name for
// it, so a guard failure names the viewer rather than an integer.
var whichKeyModeNames = func() map[viewMode]string {
	out := make(map[viewMode]string, len(whichKeyCatalogList))
	for _, mc := range whichKeyCatalogList {
		out[mc.mode] = mc.name
	}
	return out
}()

// TestWhichKeyCatalogs_RegistryIsTheOnlySeam pins the "add a viewer by adding a
// catalog" contract: the lookup map the render and dispatch paths use is
// derived from whichKeyCatalogList and nothing else, so a viewer added to one
// and not the other cannot half-exist.
func TestWhichKeyCatalogs_RegistryIsTheOnlySeam(t *testing.T) {
	if len(whichKeyCatalogs) != len(whichKeyCatalogList) {
		t.Fatalf("whichKeyCatalogs has %d modes, whichKeyCatalogList %d", len(whichKeyCatalogs), len(whichKeyCatalogList))
	}
	for _, mc := range whichKeyCatalogList {
		if mc.name == "" {
			t.Errorf("catalog for mode %v has no name; guards report failures under it", mc.mode)
		}
		if _, ok := whichKeyCatalogs[mc.mode]; !ok {
			t.Errorf("mode %q is in the list but missing from the lookup map", mc.name)
		}
	}
}

// whichKeyViewerModel builds a model sitting in the given viewer with enough
// content for the cursor guards to pass, so a test only has to set the one
// piece of state it is about.
func whichKeyViewerModel(mode viewMode) Model {
	m := whichKeyTestModel()
	m.mode = mode
	m.width, m.height = 120, 40
	m.yamlView.content = "apiVersion: v1\nkind: Pod\nmetadata:\n  name: p1\n"
	m.yamlView.sections = []yamlSection{{key: "metadata", startLine: 2, endLine: 3}}
	m.logView.lines = []string{"line one", "line two"}
	m.logView.cursor = 0
	m.describeView.content = "Name:\tp1\nNamespace:\tdefault\n"
	return m
}

// whichKeyOffered reports the labels the panel would draw right now.
func whichKeyOffered(m Model) []string {
	out := make([]string, 0, 16)
	for _, e := range m.availableWhichKeyActions() {
		out = append(out, e.Label)
	}
	return out
}

// whichKeyOfferedKeys reports the keys the panel would draw right now.
func whichKeyOfferedKeys(m Model) []string {
	kb := ui.ActiveKeybindings
	out := make([]string, 0, 16)
	for _, e := range m.availableWhichKeyActions() {
		out = append(out, e.Key(kb))
	}
	return out
}

// wkViewerScenarios enumerates the state combinations each viewer catalog's
// predicates branch on, so the duplicate-key guard below sweeps the real
// product rather than one happy path. Every conditional named in a catalog
// comment (visual mode, an armed count prefix, an applied search, follow,
// previous, the severity ends, the pod/container branch) has a scenario here.
func wkViewerScenarios(mode viewMode) []struct {
	name  string
	build func() Model
} {
	type scenario = struct {
		name  string
		build func() Model
	}
	base := func() Model { return whichKeyViewerModel(mode) }

	switch mode {
	case modeYAML:
		return []scenario{
			{"normal", base},
			{"visual", func() Model { m := base(); m.yamlView.visualMode = true; return m }},
			{"counted", func() Model { m := base(); m.yamlView.lineInput = "12"; return m }},
			{"search applied", func() Model { m := base(); m.yamlView.searchText.Set("kind"); return m }},
			{"cursor off content", func() Model { m := base(); m.yamlView.cursor = 999; return m }},
			{"no content", func() Model {
				m := base()
				m.yamlView.content, m.yamlView.sections = "", nil
				return m
			}},
			{"read-only", func() Model { m := base(); m.readOnly = true; return m }},
			{"returned from object explorer", func() Model {
				m := base()
				m.yamlReturnMode = modeObjectExplorer
				m.objectExplorerView.root = map[string]any{"kind": "Pod"}
				return m
			}},
		}
	case modeLogs:
		return []scenario{
			{"normal", base},
			{"visual", func() Model { m := base(); m.logView.visualMode = true; return m }},
			{"counted", func() Model { m := base(); m.logView.lineInput = "5"; return m }},
			{"following", func() Model { m := base(); m.logView.follow = true; return m }},
			{"previous", func() Model { m := base(); m.logView.previous = true; return m }},
			{"severity floor", func() Model { m := base(); m.logView.sevThreshold = 0; return m }},
			{"severity ceiling", func() Model { m := base(); m.logView.sevThreshold = ui.LogError; return m }},
			{"group resource", func() Model { m := base(); m.logView.parentKind = "Deployment"; return m }},
			{"single pod", func() Model { m := base(); m.actionCtx.kind = "Pod"; return m }},
			{"group resource over a pod", func() Model {
				m := base()
				m.logView.parentKind = "Deployment"
				m.actionCtx.kind = "Pod"
				return m
			}},
			{"no cursor", func() Model { m := base(); m.logView.cursor = -1; return m }},
			{"preview open", func() Model { m := base(); m.logView.previewVisible = true; return m }},
		}
	case modeDescribe:
		return []scenario{
			{"normal", base},
			{"visual char", func() Model { m := base(); m.describeView.visualMode = 'v'; return m }},
			{"visual line", func() Model { m := base(); m.describeView.visualMode = 'V'; return m }},
			{"counted", func() Model { m := base(); m.describeView.lineInput = "3"; return m }},
			{"search applied", func() Model { m := base(); m.describeView.searchQuery = "Name"; return m }},
			{"cursor off content", func() Model { m := base(); m.describeView.cursor = 999; return m }},
		}
	}
	return nil
}

// TestWhichKeyCatalogs_NoDuplicateKeysOffered is the structural guard the
// explorer catalog needed twice: two entries resolving to the same key at once
// means whichever handler owns the key silently makes the other row's
// advertised keystroke a lie. The viewers lean on one-key-many-meanings far
// harder than the explorer does (y, q, v/V/ctrl+v and \\ all carry two or
// three entries), so the sweep is mandatory rather than nice to have.
func TestWhichKeyCatalogs_NoDuplicateKeysOffered(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	for _, mode := range []viewMode{modeYAML, modeLogs, modeDescribe} {
		t.Run(whichKeyModeNames[mode], func(t *testing.T) {
			for _, sc := range wkViewerScenarios(mode) {
				t.Run(sc.name, func(t *testing.T) {
					m := sc.build()
					kb := ui.ActiveKeybindings
					byKey := map[string][]string{}
					for _, e := range m.availableWhichKeyActions() {
						byKey[e.Key(kb)] = append(byKey[e.Key(kb)], e.Label)
					}
					for k, labels := range byKey {
						if len(labels) > 1 {
							t.Errorf("key %q offered by multiple entries at once: %v", k, labels)
						}
					}
				})
			}
		})
	}
}

// TestWhichKeyCatalogs_NeverAdvertiseEscOrMotions guards the two exclusion
// rules every viewer catalog is written against. esc is the sharp one: it is
// the ONLY key whichKeyLeaderIntercept consumes outright while the panel is
// shown, so a viewer entry keyed to esc would advertise a keystroke the panel
// itself eats. The motion keys are excluded on the explorer's rule — the panel
// lists actions, not the keymap.
func TestWhichKeyCatalogs_NeverAdvertiseEscOrMotions(t *testing.T) {
	restoreWhichKeyGlobals(t)
	kb := ui.DefaultKeybindings()
	ui.ActiveKeybindings = kb

	// The motion set is VIEWER-scoped: w/e/W/B/E are word motions inside a text
	// viewer but ordinary action bindings in the explorer (kb.WatchMode,
	// kb.SecretEditor, kb.SaveResource, kb.SecurityBadgeToggle, kb.Edit), so
	// banning them everywhere would be wrong. esc is banned everywhere.
	viewerMotions := []string{
		"j", "k", "h", "l", "0", "$", "^", "w", "b", "e", "W", "B", "E",
		"g", "G", "ctrl+d", "ctrl+u", "ctrl+f", "ctrl+b",
		"home", "end", "pgup", "pgdown",
		kb.NextMatch, kb.PrevMatch,
	}
	for _, mc := range whichKeyCatalogList {
		t.Run(mc.name, func(t *testing.T) {
			banned := []string{"esc"}
			if mc.mode != modeExplorer {
				banned = append(banned, viewerMotions...)
			}
			for _, e := range mc.catalog.entries() {
				key := e.Key(kb)
				if slices.Contains(banned, key) {
					t.Errorf("entry %q is keyed to %q, which the panel must not advertise", e.Label, key)
				}
			}
		})
	}
}

// TestWhichKeyCatalogs_ZeroValueModelDoesNotPanic: predicates run on every
// render, including the first frame after a mode switch, before any content
// has loaded. Each catalog resolves its own context, so each has to survive a
// zero Model independently.
func TestWhichKeyCatalogs_ZeroValueModelDoesNotPanic(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	for _, mc := range whichKeyCatalogList {
		t.Run(mc.name, func(t *testing.T) {
			var m Model
			m.mode = mc.mode
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%s catalog panicked on a zero-value Model: %v", mc.name, r)
				}
			}()
			_ = mc.catalog.available(&m)
			_ = mc.catalog.inputFocused(&m)
		})
	}
}

// TestWhichKeyCatalogs_AllEntriesReachableViaScrolling is the viewer half of
// TestWhichKeyLeader_AllEntriesReachableViaScrolling: no entry may be
// unreachable at any terminal size, and the scroll keys must actually get
// there. Driven through the real key path and asserted on rendered text.
func TestWhichKeyCatalogs_AllEntriesReachableViaScrolling(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 0

	for _, mode := range []viewMode{modeYAML, modeLogs, modeDescribe} {
		for _, size := range [][2]int{{80, 14}, {80, 24}, {120, 40}} {
			name := fmt.Sprintf("%s/%dx%d", whichKeyModeNames[mode], size[0], size[1])
			t.Run(name, func(t *testing.T) {
				m := whichKeyViewerModel(mode)
				m.width, m.height = size[0], size[1]

				out, _ := m.handleKey(leaderKey())
				m = out.(Model)
				if !m.whichKey.shown {
					t.Fatal("precondition: the leader must show the panel")
				}
				cells := m.whichKeyLeaderCells()
				if len(cells) == 0 {
					t.Fatal("the viewer catalog offered nothing at all")
				}
				lay, ok := m.whichKeyLayoutFor(cells)
				if !ok {
					t.Fatal("the panel must lay out")
				}

				bg := strings.Repeat("\n", m.height)
				var rendered strings.Builder
				rendered.WriteString(stripANSI(m.renderWhichKeyLeader(bg)))
				for step := 0; m.whichKey.scroll < lay.maxScroll; step++ {
					if step > lay.bodyRows {
						t.Fatalf("ctrl+d never reached the end (%d of %d)", m.whichKey.scroll, lay.maxScroll)
					}
					out, _ = m.handleKey(keyMsg(ui.ActiveKeybindings.PageDown))
					m = out.(Model)
					rendered.WriteString("\n")
					rendered.WriteString(stripANSI(m.renderWhichKeyLeader(bg)))
				}
				for i, c := range cells {
					col := i / lay.grid.rowN
					drawn := c.keyText() + " " + ui.Truncate(c.desc, lay.grid.descW[col])
					if !strings.Contains(rendered.String(), drawn) {
						t.Errorf("%q never appears at any scroll offset — unreachable", drawn)
					}
				}
			})
		}
	}
}

// TestWhichKeyCatalogs_PanelIsScopedToItsOwnMode: the catalog is keyed on
// m.mode, so a viewer can never show the explorer's Delete/Scale/Namespace
// rows, and the explorer can never show the log viewer's follow toggle.
func TestWhichKeyCatalogs_PanelIsScopedToItsOwnMode(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	explorerOnly := []string{"Delete", "Scale", "Namespace selector", "Action menu"}
	for _, mode := range []viewMode{modeYAML, modeLogs, modeDescribe} {
		t.Run(whichKeyModeNames[mode], func(t *testing.T) {
			labels := whichKeyOffered(whichKeyViewerModel(mode))
			for _, label := range explorerOnly {
				if slices.Contains(labels, label) {
					t.Errorf("explorer entry %q leaked into %s", label, whichKeyModeNames[mode])
				}
			}
		})
	}

	explorer := whichKeyTestModel()
	for _, label := range []string{"Follow new lines", "Fold section at cursor", "Copy selection"} {
		if slices.Contains(whichKeyOffered(explorer), label) {
			t.Errorf("viewer entry %q leaked into the explorer", label)
		}
	}
}

// TestWhichKeyCatalogs_UncataloguedModeOffersNothing: the diff viewer and the
// exec terminal have no catalog in phase 1, so the leader key must fall through
// rather than open an empty box.
func TestWhichKeyCatalogs_UncataloguedModeOffersNothing(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true

	for _, mode := range []viewMode{modeDiff, modeExec, modeExplain, modeObjectExplorer, modeLogTop, modeEventViewer} {
		m := whichKeyTestModel()
		m.mode = mode
		if got := m.availableWhichKeyActions(); len(got) != 0 {
			t.Errorf("mode %v has no catalog but offered %d entries", mode, len(got))
		}
		if m.whichKeyLeaderArmable() {
			t.Errorf("mode %v has no catalog but reports the leader armable", mode)
		}
	}
}

// wkViewerHelpContexts maps each catalogued viewer to the help screen's own
// context name for it — the same string the viewer's handler writes into
// m.helpContextMode.
func wkViewerHelpContexts() map[viewMode]string {
	return map[viewMode]string{
		modeYAML:     "YAML View",
		modeLogs:     "Log Viewer",
		modeDescribe: "Describe View",
	}
}

// wkHelpUndocumentedKeys names the catalog keys the view's help section
// deliberately does not carry as a standalone row, with the reason. Read by
// the cross-check below, so an undocumented omission fails CI rather than
// sitting in a comment.
func wkHelpUndocumentedKeys() map[string]string {
	return map[string]string{
		"i":  "documented as the combined viw/viW form in textViewHelpEntries",
		"a":  "documented as the combined vaw/vaW form in textViewHelpEntries",
		"f1": "the help screen's own key; the per-view sections do not list the way into them",
	}
}

// TestWhichKeyCatalogs_EveryEntryIsDocumentedInItsViewHelp is the cross-check
// between the two surfaces that describe the same viewer. The help section is
// the closest thing to a spec for what a view supports, so a catalog entry with
// no help row is either a key the help forgot or a key the catalog invented —
// both worth failing on. Help rows write combined keys ("q/esc", "v/V/ctrl+v"),
// so each is split before matching.
func TestWhichKeyCatalogs_EveryEntryIsDocumentedInItsViewHelp(t *testing.T) {
	restoreWhichKeyGlobals(t)
	kb := ui.DefaultKeybindings()
	ui.ActiveKeybindings = kb
	undocumented := wkHelpUndocumentedKeys()

	for mode, helpCtx := range wkViewerHelpContexts() {
		t.Run(whichKeyModeNames[mode], func(t *testing.T) {
			documented := map[string]bool{}
			for _, row := range ui.ViewerHelpRows(helpCtx) {
				// The whole string first: "/" is itself a binding
				// (kb.Search), and splitting it yields two empty halves.
				documented[row.Key] = true
				for k := range strings.SplitSeq(row.Key, "/") {
					if k = strings.TrimSpace(k); k != "" {
						documented[k] = true
					}
				}
			}
			if len(documented) == 0 {
				t.Fatalf("no help rows carry context %q", helpCtx)
			}
			for _, e := range whichKeyCatalogs[mode].entries() {
				key := e.Key(kb)
				if documented[key] {
					continue
				}
				if _, ok := undocumented[key]; ok {
					continue
				}
				t.Errorf("entry %q advertises %q, which the %q help section does not document; add a help row or a reasoned exclusion",
					e.Label, key, helpCtx)
			}
		})
	}
}

// --- YAML view ---

func TestWhichKeyYAML_VisualModeSwapsTheCatalog(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	normal := whichKeyOffered(whichKeyViewerModel(modeYAML))
	for _, want := range []string{"Copy line", "Visual select", "Search in content", "Toggle line wrapping", "Back"} {
		if !slices.Contains(normal, want) {
			t.Errorf("normal mode must offer %q; got %v", want, normal)
		}
	}

	m := whichKeyViewerModel(modeYAML)
	m.yamlView.visualMode = true
	visual := whichKeyOffered(m)
	if !slices.Contains(visual, "Copy selection") {
		t.Errorf("visual mode must offer %q; got %v", "Copy selection", visual)
	}
	// handleYAMLVisualKey has no case for any of these — they are silent
	// no-ops there, which is exactly what the panel must not advertise.
	for _, banned := range []string{"Copy line", "Search in content", "Toggle line wrapping", "Back", "Full help", "Object Explorer", "API Explorer", "Edit in $EDITOR", "Re-fetch YAML", "Fold section at cursor"} {
		if slices.Contains(visual, banned) {
			t.Errorf("visual mode must not offer %q; the visual handler has no case for it", banned)
		}
	}
	for _, want := range []string{"Inner word (iw/iW)", "Around word (aw/aW)", "Char selection"} {
		if !slices.Contains(visual, want) {
			t.Errorf("visual mode must offer %q; got %v", want, visual)
		}
	}
}

func TestWhichKeyYAML_CountPrefixRelabelsTheYank(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	plain := whichKeyOffered(whichKeyViewerModel(modeYAML))
	if !slices.Contains(plain, "Copy line") || slices.Contains(plain, "Copy N lines") {
		t.Errorf("with no count armed, y copies one line; got %v", plain)
	}

	m := whichKeyViewerModel(modeYAML)
	m.yamlView.lineInput = "12"
	counted := whichKeyOffered(m)
	if !slices.Contains(counted, "Copy N lines") || slices.Contains(counted, "Copy line") {
		t.Errorf("with a count armed, handleYAMLNormalCopy consumes it; got %v", counted)
	}
}

// handleYAMLNormalCopy refuses outright when the cursor is off the visible-line
// mapping (update_yaml.go:110-112).
func TestWhichKeyYAML_YankHiddenWithTheCursorOffContent(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyViewerModel(modeYAML)
	m.yamlView.cursor = 999
	labels := whichKeyOffered(m)
	if slices.Contains(labels, "Copy line") || slices.Contains(labels, "Copy N lines") {
		t.Errorf("the yank must be hidden with the cursor past the last visible line; got %v", labels)
	}
}

// handleYAMLKeyQ clears an applied search before it closes the viewer
// (update_yaml.go:645-658), so the two outcomes are two entries.
func TestWhichKeyYAML_QuitLabelFollowsTheSearchState(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyViewerModel(modeYAML)
	if labels := whichKeyOffered(m); !slices.Contains(labels, "Back") || slices.Contains(labels, "Clear search") {
		t.Errorf("with no search applied q goes back; got %v", labels)
	}
	m.yamlView.searchText.Set("kind")
	if labels := whichKeyOffered(m); !slices.Contains(labels, "Clear search") || slices.Contains(labels, "Back") {
		t.Errorf("with a search applied q clears it first; got %v", labels)
	}
}

// handleYAMLKeyFoldToggle no-ops when the cursor is not inside a section, and
// handleYAMLKeyZ has nothing to act on without a multi-line section.
func TestWhichKeyYAML_FoldEntriesFollowTheDocument(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyViewerModel(modeYAML)
	m.yamlView.cursor = 2 // the metadata header line
	if labels := whichKeyOffered(m); !slices.Contains(labels, "Fold section at cursor") {
		t.Errorf("a cursor inside a section must offer the fold; got %v", labels)
	}

	flat := whichKeyViewerModel(modeYAML)
	flat.yamlView.sections = nil
	labels := whichKeyOffered(flat)
	if slices.Contains(labels, "Fold section at cursor") {
		t.Errorf("a document with no sections must not offer the fold; got %v", labels)
	}
	if slices.Contains(labels, "Fold / unfold all") {
		t.Errorf("a document with no multi-line section must not offer fold-all; got %v", labels)
	}

	single := whichKeyViewerModel(modeYAML)
	single.yamlView.sections = []yamlSection{{key: "kind", startLine: 1, endLine: 1}}
	if slices.Contains(whichKeyOffered(single), "Fold / unfold all") {
		t.Error("a single-line section is not foldable (isMultiLineSection)")
	}
}

// handleYAMLKeyCtrlE toasts and refuses in read-only mode, and needs both a
// resolvable kind and a highlighted row (update_yaml.go:289-301).
func TestWhichKeyYAML_EditGates(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyViewerModel(modeYAML)
	if !slices.Contains(whichKeyOffered(m), "Edit in $EDITOR") {
		t.Fatalf("edit must be offered over a writable Pod row; got %v", whichKeyOffered(m))
	}
	m.readOnly = true
	if slices.Contains(whichKeyOffered(m), "Edit in $EDITOR") {
		t.Error("edit must be hidden in read-only mode; the handler only toasts")
	}

	noRow := whichKeyViewerModel(modeYAML)
	noRow.setMiddleItems(nil)
	if slices.Contains(whichKeyOffered(noRow), "Edit in $EDITOR") {
		t.Error("edit must be hidden with no highlighted row")
	}
}

// handleYAMLKeyObjectExplorer either returns to a preserved tree or falls
// through to openObjectExplorer, which needs the row's Raw payload.
func TestWhichKeyYAML_ObjectExplorerGates(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	noRaw := whichKeyViewerModel(modeYAML) // whichKeyTestModel's row carries no Raw
	if slices.Contains(whichKeyOffered(noRaw), "Object Explorer") {
		t.Error("object explorer must be hidden until the row's Raw payload is loaded")
	}

	withRaw := whichKeyViewerModel(modeYAML)
	withRaw.middleItems[0].Raw = map[string]any{"kind": "Pod"}
	if !slices.Contains(whichKeyOffered(withRaw), "Object Explorer") {
		t.Error("object explorer must be offered once the row carries Raw")
	}

	// Opened FROM the tree: the handler returns to it without touching the row.
	returned := whichKeyViewerModel(modeYAML)
	returned.yamlReturnMode = modeObjectExplorer
	returned.objectExplorerView.root = map[string]any{"kind": "Pod"}
	if !slices.Contains(whichKeyOffered(returned), "Object Explorer") {
		t.Error("O must be offered when it returns to the preserved tree, regardless of the row")
	}
}

// handleYAMLRefresh does nothing when loadYAML returns nil.
func TestWhichKeyYAML_RefreshFollowsLoadYAML(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyViewerModel(modeYAML)
	if !slices.Contains(whichKeyOffered(m), "Re-fetch YAML") {
		t.Fatalf("refresh must be offered over a resource row; got %v", whichKeyOffered(m))
	}

	noRow := whichKeyViewerModel(modeYAML)
	noRow.setMiddleItems(nil)
	if slices.Contains(whichKeyOffered(noRow), "Re-fetch YAML") {
		t.Error("refresh must be hidden with no row: loadYAML returns nil")
	}

	sec := whichKeyViewerModel(modeYAML)
	sec.nav.ResourceType.Kind = "__security_findings__"
	if slices.Contains(whichKeyOffered(sec), "Re-fetch YAML") {
		t.Error("refresh must be hidden on a security view: loadYAML short-circuits to nil")
	}
}

// The YAML viewer HARDCODES "O" and "I" rather than reading kb.ObjectExplorer /
// kb.APIExplorer (update_yaml.go:173-176), so the panel must keep saying O/I
// after a rebind — advertising the rebound key would name a dead keystroke.
func TestWhichKeyYAML_CrossViewKeysIgnoreTheRebind(t *testing.T) {
	restoreWhichKeyGlobals(t)
	kb := ui.DefaultKeybindings()
	kb.ObjectExplorer = "X"
	kb.APIExplorer = "Y"
	ui.ActiveKeybindings = kb

	m := whichKeyViewerModel(modeYAML)
	m.middleItems[0].Raw = map[string]any{"kind": "Pod"}
	keys := whichKeyOfferedKeys(m)
	if !slices.Contains(keys, "O") || !slices.Contains(keys, "I") {
		t.Errorf("the YAML viewer dispatches literal O/I; got %v", keys)
	}
	if slices.Contains(keys, "X") || slices.Contains(keys, "Y") {
		t.Errorf("the rebound explorer keys do nothing in the YAML viewer; got %v", keys)
	}
}

// --- Log viewer ---

func TestWhichKeyLog_VisualModeSwapsTheCatalog(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyViewerModel(modeLogs)
	m.logView.visualMode = true
	visual := whichKeyOffered(m)
	if !slices.Contains(visual, "Copy selection") || !slices.Contains(visual, "Cancel selection") {
		t.Errorf("visual mode must offer the yank and q-cancel; got %v", visual)
	}
	// handleLogVisualKey has no case for any of these.
	for _, banned := range []string{"Filter log lines", "Search in content", "Raise min severity", "Lower min severity", "Follow new lines", "Toggle line wrapping", "Save loaded logs", "Close log viewer", "Log Top aggregation"} {
		if slices.Contains(visual, banned) {
			t.Errorf("visual mode must not offer %q; the visual handler has no case for it", banned)
		}
	}
}

// severityStep clamps to [0, ui.LogError] (logfilter.go:108-112): at either end
// the key redraws the same view and changes nothing.
func TestWhichKeyLog_SeverityStepsHideAtTheClamps(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	floor := whichKeyViewerModel(modeLogs)
	floor.logView.sevThreshold = 0
	labels := whichKeyOffered(floor)
	if slices.Contains(labels, "Lower min severity") {
		t.Errorf("at threshold 0 the down-step is clamped; got %v", labels)
	}
	if !slices.Contains(labels, "Raise min severity") {
		t.Errorf("at threshold 0 the up-step still moves; got %v", labels)
	}

	ceiling := whichKeyViewerModel(modeLogs)
	ceiling.logView.sevThreshold = ui.LogError
	labels = whichKeyOffered(ceiling)
	if slices.Contains(labels, "Raise min severity") {
		t.Errorf("at LogError the up-step is clamped; got %v", labels)
	}
	if !slices.Contains(labels, "Lower min severity") {
		t.Errorf("at LogError the down-step still moves; got %v", labels)
	}
}

// handleLogKeyF jumps to the bottom when it turns following ON, so the two
// directions read differently.
func TestWhichKeyLog_FollowLabelFollowsTheState(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyViewerModel(modeLogs)
	if labels := whichKeyOffered(m); !slices.Contains(labels, "Follow new lines") || slices.Contains(labels, "Stop following") {
		t.Errorf("not following: got %v", labels)
	}
	m.logView.follow = true
	if labels := whichKeyOffered(m); !slices.Contains(labels, "Stop following") || slices.Contains(labels, "Follow new lines") {
		t.Errorf("following: got %v", labels)
	}
}

// handleLogKeyC restarts the stream with or without --previous.
func TestWhichKeyLog_PreviousLabelFollowsTheState(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyViewerModel(modeLogs)
	if labels := whichKeyOffered(m); !slices.Contains(labels, "Previous container logs") {
		t.Errorf("showing current logs, c switches to previous; got %v", labels)
	}
	m.logView.previous = true
	if labels := whichKeyOffered(m); !slices.Contains(labels, "Current container logs") || slices.Contains(labels, "Previous container logs") {
		t.Errorf("showing previous logs, c switches back; got %v", labels)
	}
}

// handleLogKeyOther tests parentKind FIRST (update_logs_normal.go:504), so a
// group resource opens the pod selector even when actionCtx.kind is "Pod" —
// which it is, because the viewer is streaming one pod of that group.
func TestWhichKeyLog_BackslashBranchesOnParentKind(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	none := whichKeyViewerModel(modeLogs)
	labels := whichKeyOffered(none)
	if slices.Contains(labels, "Switch pod") || slices.Contains(labels, "Filter containers") {
		t.Errorf("with neither a parent kind nor a Pod target, \\ is a no-op; got %v", labels)
	}

	pod := whichKeyViewerModel(modeLogs)
	pod.actionCtx.kind = "Pod"
	labels = whichKeyOffered(pod)
	if !slices.Contains(labels, "Filter containers") || slices.Contains(labels, "Switch pod") {
		t.Errorf("a single pod opens the container filter; got %v", labels)
	}

	group := whichKeyViewerModel(modeLogs)
	group.logView.parentKind = "Deployment"
	group.actionCtx.kind = "Pod"
	labels = whichKeyOffered(group)
	if !slices.Contains(labels, "Switch pod") || slices.Contains(labels, "Filter containers") {
		t.Errorf("a group resource opens the pod selector even with a Pod target; got %v", labels)
	}
}

// handleLogNormalCopy refuses when the cursor is inactive (-1) or past the end.
func TestWhichKeyLog_YankHiddenWithoutACursorLine(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyViewerModel(modeLogs)
	m.logView.cursor = -1
	if labels := whichKeyOffered(m); slices.Contains(labels, "Copy line") {
		t.Errorf("an inactive cursor must hide the yank; got %v", labels)
	}

	empty := whichKeyViewerModel(modeLogs)
	empty.logView.lines = nil
	empty.logView.cursor = 0
	if labels := whichKeyOffered(empty); slices.Contains(labels, "Copy line") {
		t.Errorf("an empty buffer must hide the yank; got %v", labels)
	}
}

func TestWhichKeyLog_CountPrefixRelabelsTheYank(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyViewerModel(modeLogs)
	m.logView.lineInput = "7"
	if labels := whichKeyOffered(m); !slices.Contains(labels, "Copy N lines") || slices.Contains(labels, "Copy line") {
		t.Errorf("with a count armed, handleLogNormalCopy consumes it; got %v", labels)
	}
}

// --- Describe view ---

func TestWhichKeyDescribe_VisualModeSwapsTheCatalog(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	normal := whichKeyOffered(whichKeyViewerModel(modeDescribe))
	for _, want := range []string{"Copy line", "Visual select", "Search in content", "Toggle line wrapping", "Back", "Full help"} {
		if !slices.Contains(normal, want) {
			t.Errorf("normal mode must offer %q; got %v", want, normal)
		}
	}

	m := whichKeyViewerModel(modeDescribe)
	m.describeView.visualMode = 'v'
	visual := whichKeyOffered(m)
	if !slices.Contains(visual, "Copy selection") {
		t.Errorf("visual mode must offer the yank; got %v", visual)
	}
	// handleDescribeVisualKey has no case for any of these, and notably no
	// case for q at all — unlike the log viewer, where q cancels.
	for _, banned := range []string{"Copy line", "Search in content", "Toggle line wrapping", "Back", "Clear search", "Full help"} {
		if slices.Contains(visual, banned) {
			t.Errorf("visual mode must not offer %q; the visual handler has no case for it", banned)
		}
	}
}

// handleDescribeQuit clears an applied search before it closes the viewer.
func TestWhichKeyDescribe_QuitLabelFollowsTheSearchState(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyViewerModel(modeDescribe)
	if labels := whichKeyOffered(m); !slices.Contains(labels, "Back") || slices.Contains(labels, "Clear search") {
		t.Errorf("with no search applied q goes back; got %v", labels)
	}
	m.describeView.searchQuery = "Name"
	if labels := whichKeyOffered(m); !slices.Contains(labels, "Clear search") || slices.Contains(labels, "Back") {
		t.Errorf("with a search applied q clears it first; got %v", labels)
	}
}

func TestWhichKeyDescribe_YankHiddenWithTheCursorOffContent(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyViewerModel(modeDescribe)
	m.describeView.cursor = 999
	if labels := whichKeyOffered(m); slices.Contains(labels, "Copy line") {
		t.Errorf("the yank must be hidden with the cursor past the last line; got %v", labels)
	}
}

// --- rendering ---

// TestWhichKeyViewer_PanelRendersOverTheViewer proves the whole path end to
// end: the leader key inside the YAML viewer draws the bordered panel over the
// viewer's content and swaps the viewer's hint bar for the panel's own.
func TestWhichKeyViewer_PanelRendersOverTheViewer(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 0

	m := whichKeyViewerModel(modeYAML)
	m.width, m.height = 100, 30
	out, _ := m.handleKey(leaderKey())
	m = out.(Model)

	view := stripANSI(m.renderView())
	for _, want := range []string{"Copy line", "Search in content", "Visual select"} {
		if !strings.Contains(view, want) {
			t.Errorf("the panel must render %q over the YAML viewer:\n%s", want, view)
		}
	}
	if !strings.Contains(view, "esc: close") {
		t.Errorf("the viewer's hint bar must be swapped for the panel's:\n%s", view)
	}
	if strings.Contains(view, "object explorer") {
		t.Errorf("the viewer's own hint bar must be gone while the panel is up:\n%s", view)
	}
}
