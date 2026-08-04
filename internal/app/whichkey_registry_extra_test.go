package app

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// TestWhichKeyRegistry_CoversEveryBinding is a forcing function: every
// ui.Keybindings field must either appear in the explorer registry or be listed
// in whichKeyExcludedBindings with a reason. A new hotkey added later therefore
// fails CI until someone decides whether it belongs in the panel.
func TestWhichKeyRegistry_CoversEveryBinding(t *testing.T) {
	restoreWhichKeyGlobals(t)

	// Give every field a unique sentinel value so registry entries can be
	// matched back to the field they read.
	kb := ui.Keybindings{}
	v := reflect.ValueOf(&kb).Elem()
	typ := v.Type()
	fieldForValue := map[string]string{}
	for f, fv := range v.Fields() {
		if f.Type.Kind() != reflect.String {
			continue
		}
		sentinel := "__wk_" + f.Name + "__"
		fv.SetString(sentinel)
		fieldForValue[sentinel] = f.Name
	}

	registered := map[string]bool{}
	for _, a := range whichKeyExplorerActions() {
		if name, ok := fieldForValue[a.Key(kb)]; ok {
			registered[name] = true
		}
	}

	excluded := whichKeyExcludedBindings()
	for f := range typ.Fields() {
		if f.Type.Kind() != reflect.String {
			continue
		}
		if registered[f.Name] {
			if _, dup := excluded[f.Name]; dup {
				t.Errorf("binding %q is both registered and excluded; remove the exclusion", f.Name)
			}
			continue
		}
		if _, ok := excluded[f.Name]; !ok {
			t.Errorf("binding %q is neither in the which-key registry nor in whichKeyExcludedBindings; add it to one", f.Name)
		}
	}

	for name := range excluded {
		if _, ok := typ.FieldByName(name); !ok {
			t.Errorf("whichKeyExcludedBindings names %q, which is not a ui.Keybindings field anymore; remove the stale entry", name)
		}
	}
}

// TestWhichKeyRegistry_EveryEntryHasADeclaredGroup is the guard that kept the
// Group field honest once the panel stopped rendering section headers. Group no
// longer reaches the screen, so nothing else would notice an entry carrying a
// typo'd or zero-value group; whichKeyGroupOrder is the declared set it must
// belong to.
func TestWhichKeyRegistry_EveryEntryHasADeclaredGroup(t *testing.T) {
	declared := map[whichKeyGroup]bool{}
	for _, g := range whichKeyGroupOrder() {
		if g == "" {
			t.Fatal("whichKeyGroupOrder must not contain the zero group")
		}
		declared[g] = true
	}
	seen := map[whichKeyGroup]bool{}
	for _, a := range whichKeyExplorerActions() {
		if !declared[a.Group] {
			t.Errorf("catalog entry %q carries group %q, which whichKeyGroupOrder does not declare", a.Label, a.Group)
		}
		seen[a.Group] = true
	}
	for g := range declared {
		if !seen[g] {
			t.Errorf("group %q is declared but no catalog entry uses it; drop it or use it", g)
		}
	}
}

// TestAvailableWhichKeyActions_LevelScopingTableIsComplete is the forcing
// function the level-scoping table's "add a row here" comment used to only
// request: every catalog entry must have a table row or a documented
// exclusion. Without it, "Bookmarks" was added with neither and went
// unnoticed.
func TestAvailableWhichKeyActions_LevelScopingTableIsComplete(t *testing.T) {
	restoreWhichKeyGlobals(t)

	inTable := map[string]bool{}
	for _, tc := range wkLevelScopingCases() {
		inTable[tc.label] = true
	}
	excluded := wkLevelScopingExclusions()

	inCatalog := map[string]bool{}
	for _, a := range whichKeyExplorerActions() {
		inCatalog[a.Label] = true
		if inTable[a.Label] {
			if _, dup := excluded[a.Label]; dup {
				t.Errorf("catalog entry %q is both in the level-scoping table and excluded; remove the exclusion", a.Label)
			}
			continue
		}
		if _, ok := excluded[a.Label]; !ok {
			t.Errorf("catalog entry %q has no level-scoping table row and no documented exclusion; add it to one", a.Label)
		}
	}

	for label := range excluded {
		if !inCatalog[label] {
			t.Errorf("wkLevelScopingExclusions names %q, which is not a catalog entry anymore; remove the stale entry", label)
		}
	}
	for label := range inTable {
		if !inCatalog[label] {
			t.Errorf("the level-scoping table has a row for %q, which is not a catalog entry anymore; remove the stale row", label)
		}
	}
}

// assertNoDuplicateWhichKeyBindings fails if two entries in the panel resolve
// to the same key at once. A duplicate is worse than a wrong Avail on a
// single entry: whichever handler actually owns the key at runtime silently
// makes the other entry's advertised keystroke a lie, and dispatch order
// (nav keys -> UI keys -> action keys, see handleExplorerKey) decides the
// winner, not the registry.
func assertNoDuplicateWhichKeyBindings(t *testing.T, scenario string, m Model) {
	t.Helper()
	kb := ui.ActiveKeybindings
	byKey := map[string][]string{}
	for _, a := range m.availableWhichKeyActions() {
		k := a.Key(kb)
		byKey[k] = append(byKey[k], a.Label)
	}
	for k, labels := range byKey {
		if len(labels) > 1 {
			t.Errorf("%s: key %q offered by multiple entries at once: %v", scenario, k, labels)
		}
	}
}

// TestAvailableWhichKeyActions_NoDuplicateKeysOffered is the structural guard
// against the class of defect wkTogglePreviewLogsAvailable and LabelEditor's
// predicate both had: an Avail that is individually plausible but, combined
// with dispatch order, offers a key the row doesn't actually own. Sweeps
// every (kind, extra, level) combination the LevelScopingTable already
// exercises, plus two scenarios that combination alone does not reach: a
// security-view row (only reachable by setting nav.ResourceType.Kind to a
// "__security_" kind, not from the generic kind list at every level) and a
// LevelClusters row with the live log preview already on (only reachable by
// setting m.fullLogPreview directly).
func TestAvailableWhichKeyActions_NoDuplicateKeysOffered(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	allLevels := []model.Level{model.LevelClusters, model.LevelResourceTypes, model.LevelResources, model.LevelOwned, model.LevelContainers}
	kinds := []string{
		"", "Pod", "Container", "Deployment", "StatefulSet", "ReplicaSet", "HorizontalPodAutoscaler",
		"Secret", "ConfigMap", "Ingress", "Service", "__port_forwards__", "__port_forward_entry__",
		"__captures__", "__security_findings__", "__collapsed_group__",
	}
	extras := []string{"", "apps/v1/deployments"}

	for _, lvl := range allLevels {
		for _, kind := range kinds {
			for _, extra := range extras {
				m := whichKeyTestModel()
				m.nav.Level = lvl
				if lvl == model.LevelResources {
					m.nav.ResourceType = model.ResourceTypeEntry{Kind: kind}
				}
				m.setMiddleItems([]model.Item{{Name: "row1", Kind: kind, Extra: extra, Raw: map[string]any{}}})
				m.setCursor(0)
				assertNoDuplicateWhichKeyBindings(t, fmt.Sprintf("level=%s kind=%q extra=%q", levelName[lvl], kind, extra), m)
			}
		}
	}

	// Security view: nav.ResourceType.Kind carries the "__security_" prefix
	// (model/sidebar.go builds Kind: "__security_"+src.SourceName+"__" for the
	// sidebar entry), which the kind-only sweep above never sets on nav —
	// only on the row.
	sec := whichKeyTestModel()
	sec.nav.ResourceType = model.ResourceTypeEntry{Kind: "__security_findings__"}
	sec.setMiddleItems([]model.Item{{Name: "f1", Kind: "__security_findings__"}})
	sec.setCursor(0)
	assertNoDuplicateWhichKeyBindings(t, "security view via nav kind", sec)

	// LevelClusters with the live log preview already on: ClusterColorPicker
	// consumes "L" unconditionally at this level (update_keys_explorer.go),
	// regardless of fullLogPreview — the sweep above never sets fullLogPreview.
	cl := whichKeyTestModel()
	cl.nav.Level = model.LevelClusters
	cl.fullLogPreview = true
	cl.setMiddleItems([]model.Item{{Name: "ctx-a"}})
	cl.setCursor(0)
	assertNoDuplicateWhichKeyBindings(t, "LevelClusters with fullLogPreview", cl)
}

func TestAvailableWhichKeyActions_LevelScoping(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	clusters := whichKeyTestModel()
	clusters.nav.Level = model.LevelClusters
	if !containsKey(whichKeyKeys(clusters), ui.ActiveKeybindings.ClusterColorPicker) {
		t.Error("cluster color picker must be offered at the cluster level")
	}
	if containsKey(whichKeyKeys(clusters), ui.ActiveKeybindings.Delete) {
		t.Error("delete must not be offered at the cluster level")
	}

	types := whichKeyTestModel()
	types.nav.Level = model.LevelResourceTypes
	// PinGroup additionally needs a pinnable row (model.PinKeyFromRef parses
	// Extra as "group/version/resource"); a bare {Kind: "Pod"} row without a
	// resource ref does not qualify, matching handleKeyPinGroup's own
	// "This item cannot be pinned" refusal.
	types.setMiddleItems([]model.Item{{Name: "Deployments", Extra: "apps/v1/deployments"}})
	types.setCursor(0)
	if !containsKey(whichKeyKeys(types), ui.ActiveKeybindings.PinGroup) {
		t.Error("pin group must be offered at the resource-types level")
	}
	if containsKey(whichKeyKeys(types), ui.ActiveKeybindings.ClusterColorPicker) {
		t.Error("cluster color picker must not leak past the cluster level")
	}
}

func TestAvailableWhichKeyActions_NoNavigationKeys(t *testing.T) {
	restoreWhichKeyGlobals(t)
	kb := ui.DefaultKeybindings()
	ui.ActiveKeybindings = kb
	keys := whichKeyKeys(whichKeyTestModel())
	for _, nav := range []string{kb.Left, kb.Right, kb.Down, kb.Up, kb.Enter, kb.JumpTop, kb.JumpBottom} {
		if nav != "" && containsKey(keys, nav) {
			t.Errorf("navigation key %q must never appear in the panel", nav)
		}
	}
}

// Regression: handleExplorerActionKeyDiff (update_keys_actions.go) only does
// the diff — rather than showing a "select exactly 2" toast — when exactly
// two rows are selected. 0, 1, and 3 selections must all hide the entry even
// though the level is otherwise eligible.
func TestAvailableWhichKeyActions_DiffRequiresExactlyTwoSelected(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	counts := []int{0, 1, 3}
	for _, n := range counts {
		m := whichKeyTestModel()
		for i := range n {
			m.selectedItems[selectionKey(model.Item{Name: "r", Extra: string(rune('a' + i))})] = true
		}
		if containsKey(whichKeyKeys(m), ui.ActiveKeybindings.Diff) {
			t.Errorf("diff must be hidden with %d selected rows; got %v", n, whichKeyKeys(m))
		}
	}

	m := whichKeyTestModel()
	m.selectedItems[selectionKey(model.Item{Name: "a"})] = true
	m.selectedItems[selectionKey(model.Item{Name: "b"})] = true
	if !containsKey(whichKeyKeys(m), ui.ActiveKeybindings.Diff) {
		t.Fatal("diff must be offered with exactly 2 selected rows")
	}
	m.nav.Level = model.LevelResourceTypes
	if containsKey(whichKeyKeys(m), ui.ActiveKeybindings.Diff) {
		t.Fatal("diff must be hidden above LevelResources even with 2 selected")
	}
}

// Regression: toggleMouseCapture (update_mouse.go) shows a permanent
// "disabled" toast and never actually toggles when the session started
// without mouse support (--no-mouse or config). The panel must not advertise
// a key that can never succeed.
func TestAvailableWhichKeyActions_MouseToggleRequiresMouseAvailable(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyTestModel()
	m.mouseAvailable = false
	if containsKey(whichKeyKeys(m), ui.ActiveKeybindings.MouseToggle) {
		t.Fatal("mouse capture toggle must be hidden when mouse capture was never available")
	}
	m.mouseAvailable = true
	if !containsKey(whichKeyKeys(m), ui.ActiveKeybindings.MouseToggle) {
		t.Fatal("mouse capture toggle must be offered once mouse capture is available")
	}
}

// Regression: handleExplorerSecurityViewKeys (update_keys_actions_security.go)
// only dispatches SecurityIgnoreToggle when onSecurityView reports the
// navigation is inside a security source; elsewhere the key is unbound (it
// shadows LabelEditor's default binding "i", meaningless on ordinary resource
// rows — hence checking by label, not by key, since both entries resolve to
// the same key string on a writable Pod row).
func TestAvailableWhichKeyActions_SecurityIgnoreToggleOnlyOnSecurityView(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyTestModel() // Pod, not a security view
	if slices.Contains(whichKeyLabels(m), "Show ignored findings") {
		t.Fatal("show-ignored toggle must be hidden off a security view")
	}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__security_findings__"}
	if !slices.Contains(whichKeyLabels(m), "Show ignored findings") {
		t.Fatal("show-ignored toggle must be offered on a security view")
	}
}

// Regression: handleKeyPinGroup (update_keys_explorer.go) refuses a
// collapsed-group header and the Dashboards pseudo-category with a toast
// ("Select a resource type to pin"/"This item cannot be pinned"), and blocks
// entirely in union mode without a named union set.
func TestAvailableWhichKeyActions_PinGroupExclusions(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	cases := []struct {
		name string
		item model.Item
	}{
		{"collapsed group header", model.Item{Name: "Workloads", Kind: "__collapsed_group__", Extra: "/v1/pods"}},
		{"dashboard pseudo-item", model.Item{Name: "Overview", Category: "Dashboards", Extra: "/v1/pods"}},
		{"unparseable ref", model.Item{Name: "Overview", Extra: "__overview__"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := whichKeyTestModel()
			m.nav.Level = model.LevelResourceTypes
			m.setMiddleItems([]model.Item{tc.item})
			m.setCursor(0)
			if containsKey(whichKeyKeys(m), ui.ActiveKeybindings.PinGroup) {
				t.Fatalf("pin group must be hidden for %s", tc.name)
			}
		})
	}

	m := whichKeyTestModel()
	m.nav.Level = model.LevelResourceTypes
	m.setMiddleItems([]model.Item{{Name: "Deployments", Extra: "apps/v1/deployments"}})
	m.setCursor(0)
	m.unionMode = true
	m.nav.Context = UnionContextSentinel
	if containsKey(whichKeyKeys(m), ui.ActiveKeybindings.PinGroup) {
		t.Fatal("pin group must be hidden in union mode without a named union set")
	}
	m.unionSetName = "prod-set"
	if !containsKey(whichKeyKeys(m), ui.ActiveKeybindings.PinGroup) {
		t.Fatal("pin group must be offered in union mode with a named union set")
	}
}

// Regression: handleKeyClusterColorPicker and handleKeyReadOnlyToggle
// (cluster_color_overlay.go, readonly.go) both refuse a union-set row at
// LevelClusters ("Colors apply to individual contexts" / "Read-only toggle
// applies to contexts").
func TestAvailableWhichKeyActions_HidesClusterRowActionsOnUnionSetRow(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyTestModel()
	m.nav.Level = model.LevelClusters
	m.setMiddleItems([]model.Item{{Name: "prod-set", Kind: unionSetItemKind}})
	m.setCursor(0)
	keys := whichKeyKeys(m)
	if containsKey(keys, ui.ActiveKeybindings.ClusterColorPicker) {
		t.Fatalf("cluster color picker must be hidden on a union-set row; got %v", keys)
	}
	if containsKey(keys, ui.ActiveKeybindings.ReadOnlyToggle) {
		t.Fatalf("read-only toggle must be hidden on a union-set row; got %v", keys)
	}
}

// Regression: handleKeyReadOnlyToggle refuses unconditionally, at every
// level, when --read-only was passed at startup (cliReadOnly is sticky for
// the process and cannot be defeated by the in-session toggle).
func TestAvailableWhichKeyActions_HidesReadOnlyToggleWhenCLIForced(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyTestModel()
	m.cliReadOnly = true
	if containsKey(whichKeyKeys(m), ui.ActiveKeybindings.ReadOnlyToggle) {
		t.Fatal("read-only toggle must be hidden when --read-only was forced at startup")
	}
}

// Regression: openObjectExplorer (objectexplorer.go) toasts "No resource data
// available" and refuses when the selected row's Raw payload hasn't loaded.
func TestAvailableWhichKeyActions_HidesObjectExplorerWithoutRawData(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyTestModel()
	m.setMiddleItems([]model.Item{{Name: "p1", Kind: "Pod", Namespace: "default"}}) // Raw is nil
	m.setCursor(0)
	if containsKey(whichKeyKeys(m), ui.ActiveKeybindings.ObjectExplorer) {
		t.Fatal("object explorer must be hidden until the row's Raw payload is loaded")
	}
}

// Regression: openExplainBrowser (update_explain.go) refuses a collapsed-group
// header and the dashboard pseudo-items at LevelResourceTypes ("Cannot
// explain this item").
func TestAvailableWhichKeyActions_HidesAPIExplorerOnVirtualResourceTypeRow(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	cases := []struct {
		name string
		item model.Item
	}{
		{"collapsed group header", model.Item{Name: "Workloads", Kind: "__collapsed_group__"}},
		{"overview dashboard", model.Item{Name: "Overview", Kind: "__overview__"}},
		{"monitoring dashboard", model.Item{Name: "Monitoring", Extra: "__monitoring__"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := whichKeyTestModel()
			m.nav.Level = model.LevelResourceTypes
			m.setMiddleItems([]model.Item{tc.item})
			m.setCursor(0)
			if containsKey(whichKeyKeys(m), ui.ActiveKeybindings.APIExplorer) {
				t.Fatalf("API Explorer must be hidden for %s", tc.name)
			}
		})
	}
}

// Regression: handleExplorerActionKeyCreateTemplate (update_keys_actions.go)
// blocks when read-only or at the union sentinel, matching the "requires a
// single cluster" shape of Edit/SecretEditor/PasteApply.
func TestAvailableWhichKeyActions_CreateTemplateBlockedWhenReadOnlyOrUnion(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	ro := whichKeyTestModel()
	ro.readOnly = true
	if containsKey(whichKeyKeys(ro), ui.ActiveKeybindings.CreateTemplate) {
		t.Fatal("create from template must be hidden in read-only mode")
	}

	union := whichKeyTestModel()
	union.unionMode = true
	union.nav.Context = UnionContextSentinel
	if containsKey(whichKeyKeys(union), ui.ActiveKeybindings.CreateTemplate) {
		t.Fatal("create from template must be hidden at the union sentinel")
	}
}

// Regression: directActionScale (update_actions.go) only proceeds for
// IsScaleableKind kinds or HorizontalPodAutoscaler, and is unconditionally
// blocked in union mode (isUnionAllowedActionForKind has no "Scale" case).
func TestAvailableWhichKeyActions_ScaleKindAndUnionGates(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	pod := whichKeyTestModel() // Pod is not scalable
	if containsKey(whichKeyKeys(pod), ui.ActiveKeybindings.Scale) {
		t.Fatal("scale must be hidden for a Pod")
	}

	hpa := whichKeyTestModel()
	hpa.nav.ResourceType = model.ResourceTypeEntry{Kind: "HorizontalPodAutoscaler"}
	hpa.setMiddleItems([]model.Item{{Name: "h1", Kind: "HorizontalPodAutoscaler"}})
	hpa.setCursor(0)
	if !containsKey(whichKeyKeys(hpa), ui.ActiveKeybindings.Scale) {
		t.Fatal("scale must be offered for a HorizontalPodAutoscaler")
	}
	hpa.unionMode = true
	hpa.nav.Context = UnionContextSentinel
	if containsKey(whichKeyKeys(hpa), ui.ActiveKeybindings.Scale) {
		t.Fatal("scale must be hidden at the union sentinel for every kind")
	}
}

// Regression: selectedPodForLogPreview (previewlog.go) supports a Container
// row only when m.nav.OwnedName resolves the owning pod; toggling the
// preview back OFF must stay available regardless of kind once it is on.
func TestAvailableWhichKeyActions_TogglePreviewLogsContainerAndOffPaths(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	container := whichKeyTestModel()
	container.nav.Level = model.LevelContainers
	container.setMiddleItems([]model.Item{{Name: "c1", Kind: "Container"}})
	container.setCursor(0)
	if containsKey(whichKeyKeys(container), ui.ActiveKeybindings.TogglePreviewLogs) {
		t.Fatal("live log preview must be hidden for a container with no resolved owning pod")
	}
	container.nav.OwnedName = "pod-1"
	if !containsKey(whichKeyKeys(container), ui.ActiveKeybindings.TogglePreviewLogs) {
		t.Fatal("live log preview must be offered for a container once the owning pod name resolves")
	}

	dep := whichKeyTestModel()
	dep.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment"}
	dep.setMiddleItems([]model.Item{{Name: "d1", Kind: "Deployment"}})
	dep.setCursor(0)
	if containsKey(whichKeyKeys(dep), ui.ActiveKeybindings.TogglePreviewLogs) {
		t.Fatal("live log preview must be hidden for a Deployment when the preview is off")
	}
	dep.fullLogPreview = true
	if !containsKey(whichKeyKeys(dep), ui.ActiveKeybindings.TogglePreviewLogs) {
		t.Fatal("live log preview toggle-off must stay available for any kind once the preview is on")
	}
}

// Regression: handleExplorerActionKeyAllNamespaces (update_keys_actions.go)
// checks the raw m.unionMode flag ("Union mode supports exactly one
// namespace"), not the union sentinel context — it must stay hidden even
// when navigated to a real (non-sentinel) context while a union is active.
func TestAvailableWhichKeyActions_HidesAllNamespacesInUnionMode(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyTestModel()
	m.unionMode = true
	if containsKey(whichKeyKeys(m), ui.ActiveKeybindings.AllNamespaces) {
		t.Fatal("toggle all namespaces must be hidden whenever union mode is active, not only at the sentinel")
	}
}

// Regression for the drift-guard sweep: Restart and Exec are defined in
// DefaultKeybindings (theme_test.go asserts they are non-empty) but no
// explorer dispatcher compares a keypress against either field — the
// "Restart"/"Exec" actions are reachable only through the action menu, whose
// items carry their own hardcoded quick-key hints. Confirms they are excluded
// rather than silently missing from both the registry and the exclusion map.
func TestWhichKeyExcludedBindings_RestartAndExecAreDeadBindings(t *testing.T) {
	excluded := whichKeyExcludedBindings()
	for _, name := range []string{"Restart", "Exec"} {
		if _, ok := excluded[name]; !ok {
			t.Errorf("%q must be listed in whichKeyExcludedBindings", name)
		}
	}
	kb := ui.DefaultKeybindings()
	for _, a := range whichKeyExplorerActions() {
		if a.Key(kb) == kb.Restart || a.Key(kb) == kb.Exec {
			t.Errorf("registry entry %q resolves to Restart/Exec's key, but neither is dispatched by the explorer", a.Label)
		}
	}
}

// Regression: OpenMarks ("'") was previously excluded with a false, circular
// reason ("reachable from the bookmarks overlay") — it is in fact the key
// that OPENS that overlay. handleKeyOpenMarks (update_keys.go) has no gate at
// all; the case in handleExplorerNavKey (update_keys_explorer.go:327-329)
// dispatches it unconditionally. It belongs in the registry, not the
// exclusion map.
func TestAvailableWhichKeyActions_OpenMarksIsRegisteredAndUnconditional(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	if _, excluded := whichKeyExcludedBindings()["OpenMarks"]; excluded {
		t.Fatal("OpenMarks must not be in whichKeyExcludedBindings; it is a real, unconditionally-available explorer action")
	}

	for _, lvl := range []model.Level{model.LevelClusters, model.LevelResourceTypes, model.LevelResources, model.LevelOwned, model.LevelContainers} {
		m := whichKeyTestModel()
		m.nav.Level = lvl
		if !containsKey(whichKeyKeys(m), ui.ActiveKeybindings.OpenMarks) {
			t.Errorf("OpenMarks must be offered at %s; got %v", levelName[lvl], whichKeyKeys(m))
		}
	}
}

// Regression (M2): handleKeySelectAll (update_keys.go:325) gates only on
// level, not on the cursor row — it operates on m.selectedItems (which
// survives filtering) and m.visibleMiddleItems() directly, not
// selectedMiddleItem(). With existing selections but the filter narrowed to
// zero visible rows, hasSelection() is still true and ctrl+a still clears
// them; wkOnRow's sel != nil requirement hid that working keystroke.
func TestAvailableWhichKeyActions_SelectAllAvailableWithNoVisibleRows(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyTestModel()
	m.setMiddleItems([]model.Item{{Name: "p1", Kind: "Pod", Namespace: "default"}})
	m.selectedItems[selectionKey(model.Item{Name: "p1", Kind: "Pod", Namespace: "default"})] = true
	m.filterText = "does-not-match-anything" // visibleMiddleItems() becomes empty
	m.setCursor(0)
	if !containsKey(whichKeyKeys(m), ui.ActiveKeybindings.SelectAll) {
		t.Fatal("select/deselect all must stay offered when a selection exists but the filter hides every row")
	}
}

// Regression (M1): handleExplorerFullscreen (update_keys_explorer.go) refuses
// with only a toast ("Open dashboard members with right-arrow") — neither
// toggling the dashboard fullscreen nor cycling the three-pane layout — when
// hovering the Overview/Monitoring dashboard pseudo-item at LevelResourceTypes
// while in union mode. Every other entry in this file treats a toast-only
// branch as unavailable (Diff, CreateTemplate, Scale, ...); Fullscreen had no
// Avail at all and so was offered even here.
func TestAvailableWhichKeyActions_HidesFullscreenOnUnionDashboardRow(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyTestModel()
	m.nav.Level = model.LevelResourceTypes
	m.setMiddleItems([]model.Item{{Name: "Overview", Extra: "__overview__"}})
	m.setCursor(0)
	m.unionMode = true
	m.nav.Context = UnionContextSentinel
	if containsKey(whichKeyKeys(m), ui.ActiveKeybindings.Fullscreen) {
		t.Fatal("cycle layout must be hidden on the dashboard row in union mode; the key only shows a toast there")
	}

	// Guard against over-correcting: the layout-cycle branch is unconditional
	// on an ordinary row, and the dashboard-toggle branch works fine outside
	// union mode.
	m.unionMode = false
	m.nav.Context = "ctx-a"
	if !containsKey(whichKeyKeys(m), ui.ActiveKeybindings.Fullscreen) {
		t.Fatal("cycle layout must stay offered on the dashboard row outside union mode")
	}
	plain := whichKeyTestModel()
	if !containsKey(whichKeyKeys(plain), ui.ActiveKeybindings.Fullscreen) {
		t.Fatal("cycle layout must stay offered on an ordinary resource row")
	}
}
