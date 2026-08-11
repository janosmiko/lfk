package app

import (
	"fmt"
	"slices"
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// whichKeyKeys returns just the resolved keys of the available actions, in order.
func whichKeyKeys(m Model) []string {
	kb := ui.ActiveKeybindings
	acts := m.availableWhichKeyActions()
	out := make([]string, 0, len(acts))
	for _, a := range acts {
		out = append(out, a.Key(kb))
	}
	return out
}

func containsKey(keys []string, want string) bool {
	return slices.Contains(keys, want)
}

// whichKeyLabels returns the labels of the available actions, in order. Two
// entries can share a Key (e.g. the two OpenBrowser entries), so label is
// the only unambiguous way to assert which specific entry is present.
func whichKeyLabels(m Model) []string {
	acts := m.availableWhichKeyActions()
	out := make([]string, 0, len(acts))
	for _, a := range acts {
		out = append(out, a.Label)
	}
	return out
}

// whichKeyTestModel is a writable explorer model sitting on a Pod row.
func whichKeyTestModel() Model {
	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.setMiddleItems([]model.Item{{Name: "p1", Kind: "Pod", Namespace: "default"}})
	m.setCursor(0)
	return m
}

func TestAvailableWhichKeyActions_HidesMutatingWhenReadOnly(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel()
	m.readOnly = true
	keys := whichKeyKeys(m)
	if containsKey(keys, ui.ActiveKeybindings.Delete) {
		t.Fatalf("delete must be hidden in read-only mode; got %v", keys)
	}
	if containsKey(keys, ui.ActiveKeybindings.Edit) {
		t.Fatalf("edit must be hidden in read-only mode; got %v", keys)
	}
}

func TestAvailableWhichKeyActions_ShowsMutatingWhenWritable(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel()
	keys := whichKeyKeys(m)
	if !containsKey(keys, ui.ActiveKeybindings.Delete) {
		t.Fatalf("delete must be available on a writable resource row; got %v", keys)
	}
}

func TestAvailableWhichKeyActions_SecretEditorOnlyForSecretAndConfigMap(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel() // Pod
	if containsKey(whichKeyKeys(m), ui.ActiveKeybindings.SecretEditor) {
		t.Fatal("secret editor must not be offered for a Pod")
	}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Secret", APIVersion: "v1", Resource: "secrets", Namespaced: true}
	m.setMiddleItems([]model.Item{{Name: "s1", Kind: "Secret", Namespace: "default"}})
	m.setCursor(0)
	if !containsKey(whichKeyKeys(m), ui.ActiveKeybindings.SecretEditor) {
		t.Fatal("secret editor must be offered for a Secret")
	}
}

func TestAvailableWhichKeyActions_RespectsRebind(t *testing.T) {
	restoreWhichKeyGlobals(t)
	kb := ui.DefaultKeybindings()
	// "f9" is unbound by default. The old value here was "X", which is
	// kb.ForceDelete's own default — both rows are offered on a Pod row, so the
	// rebind collided and wkDropAmbiguousKeys correctly hid both. That made this
	// test assert the rebind was ignored, which is not what it is about.
	kb.Delete = "f9"
	ui.ActiveKeybindings = kb
	if !containsKey(whichKeyKeys(whichKeyTestModel()), "f9") {
		t.Fatal("registry must resolve keys from ActiveKeybindings at call time")
	}
}

func TestAvailableWhichKeyActions_DropsClearedBindings(t *testing.T) {
	restoreWhichKeyGlobals(t)
	kb := ui.DefaultKeybindings()
	kb.Delete = ""
	ui.ActiveKeybindings = kb
	if containsKey(whichKeyKeys(whichKeyTestModel()), "") {
		t.Fatal("entries with a cleared binding must be dropped")
	}
}

// Regression for review finding 1: handleExplorerActionKeyOpenBrowser
// (update_keys_actions.go) matches "Ingress", "__port_forwards__", and
// "__port_forward_entry__" — not "Service", which routes to the separate
// "Port Forward & Open" action. selectedResourceKind() at LevelResources
// returns the list-level kind, so on the port-forwards list that is always
// "__port_forwards__".
func TestAvailableWhichKeyActions_OpenBrowserOnPortForwardsList(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__port_forwards__", APIGroup: "_portforward", APIVersion: "v1", Resource: "portforwards", Namespaced: false}
	m.setMiddleItems([]model.Item{{Name: "p1", Kind: "__port_forward_entry__"}})
	m.setCursor(0)
	if !containsKey(whichKeyKeys(m), ui.ActiveKeybindings.OpenBrowser) {
		t.Fatal("open in browser must be offered on the port-forwards list, matching handleExplorerActionKeyOpenBrowser")
	}
}

// Regression for review finding B (round 2): a Service row must offer the
// "Port forward & open" catalog entry, not "Open in browser" — the two share
// a key (kb.OpenBrowser) but are mutually exclusive by predicate, matching
// handleExplorerActionKeyOpenBrowser's kind switch (update_keys_actions.go).
func TestAvailableWhichKeyActions_OpenBrowserServiceUsesPortForwardAndOpen(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Service", APIVersion: "v1", Resource: "services", Namespaced: true}
	m.setMiddleItems([]model.Item{{Name: "svc1", Kind: "Service", Namespace: "default"}})
	m.setCursor(0)
	labels := whichKeyLabels(m)
	if slices.Contains(labels, "Open in browser") {
		t.Fatalf("open in browser must not be offered for a Service; that key routes to Port Forward & Open, a different label; got %v", labels)
	}
	if !slices.Contains(labels, "Port forward & open") {
		t.Fatalf("port forward & open must be offered for a Service; got %v", labels)
	}
}

// Regression for review finding B (round 2): the two OpenBrowser-key entries
// ("Open in browser", "Port forward & open") must never both be available
// for the same row, since only one of them matches what the key actually
// does for any given kind.
func TestAvailableWhichKeyActions_OpenBrowserEntriesMutuallyExclusive(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	kinds := []string{"Ingress", "Service", "__port_forwards__", "__port_forward_entry__", "Pod", "Deployment"}
	for _, kind := range kinds {
		t.Run(kind, func(t *testing.T) {
			m := whichKeyTestModel()
			m.nav.ResourceType = model.ResourceTypeEntry{Kind: kind, APIVersion: "v1", Resource: "x", Namespaced: true}
			m.setMiddleItems([]model.Item{{Name: "r1", Kind: kind, Namespace: "default"}})
			m.setCursor(0)
			n := 0
			for _, key := range whichKeyKeys(m) {
				if key == ui.ActiveKeybindings.OpenBrowser {
					n++
				}
			}
			if n > 1 {
				t.Fatalf("kind %q offered %d OpenBrowser-key entries at once, want at most 1", kind, n)
			}
		})
	}
}

// Regression for review finding 2: handleExplorerActionKeySecretEditor,
// handleExplorerActionKeyLabelEditor, and the inline PasteApply case
// (update_keys_actions.go) each block with a "requires a single cluster"
// toast when isUnionSentinel() is true, regardless of kind.
func TestAvailableWhichKeyActions_HidesSingleClusterActionsInUnionMode(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Secret", APIVersion: "v1", Resource: "secrets", Namespaced: true}
	m.setMiddleItems([]model.Item{{Name: "s1", Kind: "Secret", Namespace: "default", ClusterName: "ctx-a"}})
	m.setCursor(0)
	m.unionMode = true
	m.nav.Context = UnionContextSentinel
	keys := whichKeyKeys(m)
	if containsKey(keys, ui.ActiveKeybindings.SecretEditor) {
		t.Fatalf("secret editor must be hidden in union mode; got %v", keys)
	}
	if containsKey(keys, ui.ActiveKeybindings.LabelEditor) {
		t.Fatalf("label editor must be hidden in union mode; got %v", keys)
	}
	if containsKey(keys, ui.ActiveKeybindings.PasteApply) {
		t.Fatalf("paste & apply must be hidden in union mode; got %v", keys)
	}
}

// Regression for review finding 3: directActionDescribe, directActionEdit,
// and directActionDelete (update_actions.go) each call isVirtualResourceKind
// and silently no-op — no status message — for synthetic rows such as
// security findings.
func TestAvailableWhichKeyActions_HidesDescribeEditDeleteOnVirtualKind(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__security_findings__"}
	m.setMiddleItems([]model.Item{{Name: "f1", Kind: "__security_findings__"}})
	m.setCursor(0)
	keys := whichKeyKeys(m)
	if containsKey(keys, ui.ActiveKeybindings.Describe) {
		t.Fatalf("describe must be hidden for a virtual/security kind; got %v", keys)
	}
	if containsKey(keys, ui.ActiveKeybindings.Edit) {
		t.Fatalf("edit must be hidden for a virtual/security kind; got %v", keys)
	}
	if containsKey(keys, ui.ActiveKeybindings.Delete) {
		t.Fatalf("delete must be hidden for a virtual/security kind; got %v", keys)
	}
}

// Regression for review finding A (round 2): isUnionAllowedActionForKind
// (readonly.go) has no "Edit" case, so it falls to its default of false for
// every kind — Edit is unconditionally blocked in union mode via
// executeAction's backstop, exactly like SecretEditor/LabelEditor/PasteApply.
func TestAvailableWhichKeyActions_HidesEditInUnionMode(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel() // Pod, writable
	m.unionMode = true
	m.nav.Context = UnionContextSentinel
	keys := whichKeyKeys(m)
	if containsKey(keys, ui.ActiveKeybindings.Edit) {
		t.Fatalf("edit must be hidden in union mode for every kind; got %v", keys)
	}
}

// Regression for review finding C.1 (round 2): handleExplorerActionKeyLabelEditor
// (update_keys_actions.go) silently no-ops at LevelResources when
// nav.ResourceType.Kind is "__port_forwards__" or "__captures__" — a
// hand-rolled check distinct from isVirtualResourceKind.
func TestAvailableWhichKeyActions_HidesLabelEditorOnPortForwardsAndCaptures(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	for _, kind := range []string{"__port_forwards__", "__captures__"} {
		t.Run(kind, func(t *testing.T) {
			m := whichKeyTestModel()
			m.nav.ResourceType = model.ResourceTypeEntry{Kind: kind}
			m.setMiddleItems([]model.Item{{Name: "r1", Kind: kind}})
			m.setCursor(0)
			keys := whichKeyKeys(m)
			if containsKey(keys, ui.ActiveKeybindings.LabelEditor) {
				t.Fatalf("label editor must be hidden for kind %q; got %v", kind, keys)
			}
		})
	}
}

// Regression for review finding C.2 (round 2): isUnionAllowedActionForKind
// restricts "Force Delete" to kind == "Pod" in union mode (readonly.go), so
// the predicate must hide it for Job while still showing it for Pod.
func TestAvailableWhichKeyActions_ForceDeleteUnionModeRestrictedToPod(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyTestModel() // Pod, writable
	m.unionMode = true
	m.nav.Context = UnionContextSentinel
	if !containsKey(whichKeyKeys(m), ui.ActiveKeybindings.ForceDelete) {
		t.Fatal("force delete must still be offered for a Pod in union mode")
	}

	job := whichKeyTestModel()
	job.nav.ResourceType = model.ResourceTypeEntry{Kind: "Job", APIGroup: "batch", APIVersion: "v1", Resource: "jobs", Namespaced: true}
	job.setMiddleItems([]model.Item{{Name: "j1", Kind: "Job", Namespace: "default"}})
	job.setCursor(0)
	job.unionMode = true
	job.nav.Context = UnionContextSentinel
	keys := whichKeyKeys(job)
	if containsKey(keys, ui.ActiveKeybindings.ForceDelete) {
		t.Fatalf("force delete must be hidden for a Job in union mode; got %v", keys)
	}
}

// Regression found while sweeping for review finding C (round 2), same class
// as C.2: the Delete predicate had no union gate at all, but
// isUnionAllowedActionForKind restricts "Delete" to kind == "Pod" in union
// mode too, so it was over-permissive for every other kind.
func TestAvailableWhichKeyActions_DeleteUnionModeRestrictedToPod(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyTestModel() // Pod, writable
	m.unionMode = true
	m.nav.Context = UnionContextSentinel
	if !containsKey(whichKeyKeys(m), ui.ActiveKeybindings.Delete) {
		t.Fatal("delete must still be offered for a Pod in union mode")
	}

	dep := whichKeyTestModel()
	dep.nav.ResourceType = model.ResourceTypeEntry{Kind: "Deployment", APIGroup: "apps", APIVersion: "v1", Resource: "deployments", Namespaced: true}
	dep.setMiddleItems([]model.Item{{Name: "d1", Kind: "Deployment", Namespace: "default"}})
	dep.setCursor(0)
	dep.unionMode = true
	dep.nav.Context = UnionContextSentinel
	keys := whichKeyKeys(dep)
	if containsKey(keys, ui.ActiveKeybindings.Delete) {
		t.Fatalf("delete must be hidden for a Deployment in union mode; got %v", keys)
	}
}

// Regression for review finding E (round 3): handleExplorerActionKeySecretEditor
// (update_keys_actions.go) only has an "m.nav.Level == model.LevelResources"
// branch — no LevelOwned branch, unlike LabelEditor — and silently no-ops
// otherwise. Reachable via Helm/ArgoCD/Flux-managed Secret/ConfigMap rows at
// LevelOwned (commands_load.go -> k8s/resources.go; Item.Kind copied
// straight from the manifest in k8s/gitops_argo.go, k8s/helm.go,
// k8s/gitops_flux.go).
func TestAvailableWhichKeyActions_HidesSecretEditorAtLevelOwned(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel()
	m.nav.Level = model.LevelOwned
	m.setMiddleItems([]model.Item{{Name: "s1", Kind: "Secret", Namespace: "default"}})
	m.setCursor(0)
	keys := whichKeyKeys(m)
	if containsKey(keys, ui.ActiveKeybindings.SecretEditor) {
		t.Fatalf("secret editor must be hidden at LevelOwned; got %v", keys)
	}
}

// Regression for review finding F (round 3): handleExplorerActionKeyLabelEditor
// (update_keys_actions.go) only has LevelResources and LevelOwned branches —
// no LevelContainers branch — and silently no-ops otherwise.
func TestAvailableWhichKeyActions_HidesLabelEditorAtLevelContainers(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel()
	m.nav.Level = model.LevelContainers
	m.setMiddleItems([]model.Item{{Name: "c1", Kind: "Container"}})
	m.setCursor(0)
	keys := whichKeyKeys(m)
	if containsKey(keys, ui.ActiveKeybindings.LabelEditor) {
		t.Fatalf("label editor must be hidden at LevelContainers; got %v", keys)
	}
}

// Guards against over-correcting finding F: LabelEditor's LevelOwned branch
// is real (update_keys_actions.go:672-679, resolveOwnedResourceType), so
// LevelOwned must stay offered.
func TestAvailableWhichKeyActions_ShowsLabelEditorAtLevelOwned(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel()
	m.nav.Level = model.LevelOwned
	m.setMiddleItems([]model.Item{{Name: "p1", Kind: "Pod"}})
	m.setCursor(0)
	if !containsKey(whichKeyKeys(m), ui.ActiveKeybindings.LabelEditor) {
		t.Fatal("label editor must still be offered at LevelOwned")
	}
}

// Regression found during the round-3 level audit (same class, not one of
// the two named findings): CopyName never checked level at all, and
// availableCopyFormats (copy_format.go) explicitly documents that
// CopyYAML/CopyField also work at every level — "Clusters and ResourceTypes
// only support Table ... All other levels offer the full YAML / JSON / Table
// set" — yet all three predicates inherited wkOnRow's ">= LevelResources"
// and were hidden at LevelClusters/LevelResourceTypes where the key still
// works.
func TestAvailableWhichKeyActions_CopyActionsAvailableAtEveryLevel(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	keysToCheck := []struct {
		name string
		key  func(ui.Keybindings) string
	}{
		{"CopyName", func(kb ui.Keybindings) string { return kb.CopyName }},
		{"CopyYAML", func(kb ui.Keybindings) string { return kb.CopyYAML }},
		{"CopyField", func(kb ui.Keybindings) string { return kb.CopyField }},
	}
	for lvl, name := range levelName {
		t.Run(name, func(t *testing.T) {
			m := whichKeyTestModel()
			m.nav.Level = lvl
			m.setMiddleItems([]model.Item{{Name: "row1", Kind: "Pod"}})
			m.setCursor(0)
			keys := whichKeyKeys(m)
			for _, kc := range keysToCheck {
				if !containsKey(keys, kc.key(ui.ActiveKeybindings)) {
					t.Errorf("%s must be offered at %s; got %v", kc.name, name, keys)
				}
			}
		})
	}
}

// Regression found during the round-3 level/handler audit while verifying
// Delete (same entry named in finding C.2/D last round): the dispatcher for
// kb.Delete special-cases the port-forwards list before ever reaching
// directActionDelete —
//
//	case kb.Delete:
//	    if m.nav.Level == model.LevelResources && m.nav.ResourceType.Kind == "__port_forwards__" {
//	        ret, cmd := m.removeSelectedPortForward()
//	        ...
//
// removeSelectedPortForward (update_actions_helm_misc.go) is a local,
// ungated operation (no read-only or union check — it only touches
// m.portForwardMgr). The Delete entry's wkRealKind gate correctly hides the
// k8s-mutation "Delete" label there (isVirtualResourceKind("__port_forwards__")
// is true), but nothing offered the key's real behavior — a "hidden
// available action" of the same severity as finding 3 in round 1.
func TestAvailableWhichKeyActions_DeleteOffersRemovePortForwardOnPortForwardsList(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__port_forwards__", APIGroup: "_portforward", APIVersion: "v1", Resource: "portforwards", Namespaced: false}
	m.setMiddleItems([]model.Item{{Name: "pf1", Kind: "__port_forward_entry__"}})
	m.setCursor(0)
	// removeSelectedPortForward has no read-only or union gate — assert the
	// entry survives even with both set, matching the handler exactly.
	m.readOnly = true
	m.unionMode = true
	m.nav.Context = UnionContextSentinel

	labels := whichKeyLabels(m)
	if !slices.Contains(labels, "Remove port forward") {
		t.Fatalf("remove port forward must be offered on the port-forwards list even when read-only/union; got %v", labels)
	}
	if slices.Contains(labels, "Delete") {
		t.Fatalf("the k8s-mutation Delete entry must not be offered on the port-forwards list; got %v", labels)
	}
}

// Regression for review finding G (round 4): directActionDelete
// (update_actions.go:256-259) opens with an unconditional
// "m.nav.Level == model.LevelContainers" check that toasts
// "Delete not available for containers" and returns — before the kind check
// and before hasSelection(). The key can never delete at that level, so the
// panel must not advertise it.
func TestAvailableWhichKeyActions_HidesDeleteAtLevelContainers(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel()
	m.nav.Level = model.LevelContainers
	m.setMiddleItems([]model.Item{{Name: "c1", Kind: "Container"}})
	m.setCursor(0)
	if slices.Contains(whichKeyLabels(m), "Delete") {
		t.Fatalf("delete must be hidden at LevelContainers; got %v", whichKeyLabels(m))
	}
}

// Regression for review finding H (round 4): openActionMenu
// (update_actions.go:25-30) routes LevelClusters to openClusterPickerActionMenu
// and LevelResourceTypes to openResourceTypeActionMenu, neither of which has a
// level floor — the key opens a real menu there.
func TestAvailableWhichKeyActions_ActionMenuAtClusterAndResourceTypeLevels(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	clusters := whichKeyTestModel()
	clusters.nav.Level = model.LevelClusters
	clusters.setMiddleItems([]model.Item{{Name: "ctx-a"}})
	clusters.setCursor(0)
	if !slices.Contains(whichKeyLabels(clusters), "Action menu") {
		t.Fatalf("action menu must be offered at LevelClusters; got %v", whichKeyLabels(clusters))
	}

	types := whichKeyTestModel()
	types.nav.Level = model.LevelResourceTypes
	// Expand the row's category: an unexpanded one collapses to a
	// __collapsed_group__ header, which the real handler refuses (asserted in
	// TestAvailableWhichKeyActions_ActionMenuHiddenOnNonActionableRows).
	types.expandedGroup = "Workloads"
	types.setMiddleItems([]model.Item{{Name: "Pods", Extra: "/v1/pods", Category: "Workloads"}})
	types.setCursor(0)
	if !slices.Contains(whichKeyLabels(types), "Action menu") {
		t.Fatalf("action menu must be offered at LevelResourceTypes; got %v", whichKeyLabels(types))
	}
}

// Guards against over-correcting finding H with a bare wkRowSelected: both
// newly-allowed levels have per-row conditions of their own.
// openClusterPickerActionMenu (update_actions_cluster_picker.go:19) returns
// unchanged for a union-set row; openResourceTypeActionMenu
// (update_actions_hidden.go:51-60) returns unchanged for collapsed-group
// headers, the Dashboards category, and any ref PinKeyFromRef can't parse.
func TestAvailableWhichKeyActions_ActionMenuHiddenOnNonActionableRows(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	cases := []struct {
		name  string
		level model.Level
		item  model.Item
	}{
		{"union set row at LevelClusters", model.LevelClusters, model.Item{Name: "prod-set", Kind: unionSetItemKind}},
		{"collapsed group header", model.LevelResourceTypes, model.Item{Name: "Workloads", Kind: "__collapsed_group__", Extra: "/v1/pods"}},
		{"dashboard pseudo-item", model.LevelResourceTypes, model.Item{Name: "Overview", Category: "Dashboards", Extra: "/v1/pods"}},
		{"unparseable pin ref", model.LevelResourceTypes, model.Item{Name: "Overview", Extra: "__overview__"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := whichKeyTestModel()
			m.nav.Level = tc.level
			m.setMiddleItems([]model.Item{tc.item})
			m.setCursor(0)
			if slices.Contains(whichKeyLabels(m), "Action menu") {
				t.Fatalf("action menu must be hidden for %s; got %v", tc.name, whichKeyLabels(m))
			}
		})
	}
}

// levelName is a small helper for subtest names in the level-scoping table
// below; model.Level has no String() method.
var levelName = map[model.Level]string{
	model.LevelClusters:      "LevelClusters",
	model.LevelResourceTypes: "LevelResourceTypes",
	model.LevelResources:     "LevelResources",
	model.LevelOwned:         "LevelOwned",
	model.LevelContainers:    "LevelContainers",
}

// TestAvailableWhichKeyActions_LevelScopingTable pins the navigation levels
// at which every current catalog entry is offered, verified against each
// entry's real handler (see the round-3 per-entry audit table in the Task 2
// report for file:line citations). kind is a resource kind compatible with
// the entry's own kind gate (ignored by entries with none); it is only
// honored at LevelResources (nav.ResourceType.Kind) and LevelOwned
// (the row's Kind) — LevelContainers always reads back "Container" via
// selectedResourceKind(), and LevelClusters/LevelResourceTypes always read
// back "", regardless of what's set here, which is itself part of what this
// table pins. extra is the row's resource ref, needed only by entries whose
// handler parses it (the resource-type action menu's PinKeyFromRef).
//
// New catalog entries must add a row here rather than relying on the blanket
// wkOnRow default — TestAvailableWhichKeyActions_LevelScopingTableIsComplete
// enforces that, so the requirement is checked rather than merely requested.
func TestAvailableWhichKeyActions_LevelScopingTable(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	allLevels := []model.Level{model.LevelClusters, model.LevelResourceTypes, model.LevelResources, model.LevelOwned, model.LevelContainers}

	for _, tc := range wkLevelScopingCases() {
		t.Run(tc.label, func(t *testing.T) {
			want := make(map[model.Level]bool, len(tc.levels))
			for _, lvl := range tc.levels {
				want[lvl] = true
			}
			for _, lvl := range allLevels {
				t.Run(levelName[lvl], func(t *testing.T) {
					m := whichKeyTestModel()
					m.nav.Level = lvl
					// Raw is set unconditionally: real explorer rows always
					// carry the source object once loaded, and Object
					// Explorer's gate (sel.Raw != nil) is otherwise
					// untestable through this generic per-level harness.
					item := model.Item{Name: "row1", Extra: tc.extra, Raw: map[string]any{}}
					switch lvl {
					case model.LevelResources:
						m.nav.ResourceType = model.ResourceTypeEntry{Kind: tc.kind}
						item.Kind = tc.kind
					case model.LevelOwned:
						item.Kind = tc.kind
					}
					m.setMiddleItems([]model.Item{item})
					m.setCursor(0)

					got := slices.Contains(whichKeyLabels(m), tc.label)
					if got != want[lvl] {
						t.Errorf("%q at %s: got offered=%v, want %v", tc.label, levelName[lvl], got, want[lvl])
					}
				})
			}
		})
	}
}

// wkLevelScopingCase is one row of the level-scoping table.
type wkLevelScopingCase struct {
	label  string
	kind   string
	extra  string
	levels []model.Level // levels where the entry must be offered
}

// wkLevelScopingExclusions names the catalog entries deliberately absent from
// the table, each with the reason a level alone cannot decide whether they are
// offered. Every one has a dedicated test instead. Read by the completeness
// guard, so an undocumented omission fails CI rather than sitting in a comment.
func wkLevelScopingExclusions() map[string]string {
	return map[string]string{
		"Diff two selected":     "needs exactly 2 selected rows, not level-dependent alone",
		"Mouse capture":         "needs m.mouseAvailable, false by default here",
		"Show ignored findings": "needs a security-prefixed kind",
	}
}

// wkLevelScopingCases is the table itself, at package scope so the
// completeness guard can compare it against the catalog.
func wkLevelScopingCases() []wkLevelScopingCase {
	allLevels := []model.Level{model.LevelClusters, model.LevelResourceTypes, model.LevelResources, model.LevelOwned, model.LevelContainers}

	return []wkLevelScopingCase{
		{"Action menu", "Pod", "apps/v1/deployments", allLevels},
		{"Logs (fullscreen)", "Pod", "", []model.Level{model.LevelResources, model.LevelOwned}},
		{"Describe", "Pod", "", []model.Level{model.LevelResources, model.LevelOwned, model.LevelContainers}},
		{"Edit in $EDITOR", "Pod", "", []model.Level{model.LevelResources, model.LevelOwned, model.LevelContainers}},
		// LevelContainers is excluded: directActionDelete refuses it outright
		// with a toast before any kind or selection check (finding G).
		{"Delete", "Pod", "", []model.Level{model.LevelResources, model.LevelOwned}},
		{"Remove port forward", "__port_forwards__", "", []model.Level{model.LevelResources}},
		{"Force delete", "Pod", "", []model.Level{model.LevelResources, model.LevelOwned}},
		{"Secret/ConfigMap editor", "Secret", "", []model.Level{model.LevelResources}},
		{"Label/annotation editor", "Pod", "", []model.Level{model.LevelResources, model.LevelOwned}},
		{"Copy name", "", "", allLevels},
		{"Copy as...", "", "", allLevels},
		{"Copy a field", "", "", allLevels},
		{"Paste & apply", "", "", allLevels},
		{"Open in browser", "Ingress", "", []model.Level{model.LevelResources, model.LevelOwned}},
		{"Port forward & open", "Service", "", []model.Level{model.LevelResources, model.LevelOwned}},
		{"Save to file", "", "", []model.Level{model.LevelResources, model.LevelOwned, model.LevelContainers}},
		{"Refresh view", "", "", allLevels},
		{"Create from template", "", "", []model.Level{model.LevelResourceTypes, model.LevelResources, model.LevelOwned, model.LevelContainers}},
		{"Scale", "Deployment", "", []model.Level{model.LevelResources, model.LevelOwned}},

		{"Toggle selection", "", "", []model.Level{model.LevelResources, model.LevelOwned, model.LevelContainers}},
		{"Select/deselect all", "", "", []model.Level{model.LevelResources, model.LevelOwned, model.LevelContainers}},
		{"Select range", "", "", []model.Level{model.LevelResources, model.LevelOwned, model.LevelContainers}},

		{"Details / YAML preview", "", "", allLevels},
		{"Live log preview", "Pod", "", []model.Level{model.LevelResources, model.LevelOwned}},
		{"Resource map", "", "", []model.Level{model.LevelResources, model.LevelOwned, model.LevelContainers}},
		{"Object Explorer", "", "", []model.Level{model.LevelResources, model.LevelOwned, model.LevelContainers}},
		{"API Explorer", "Pod", "", []model.Level{model.LevelResourceTypes, model.LevelResources, model.LevelOwned, model.LevelContainers}},
		{"RBAC browser", "", "", allLevels},
		{"Orphan overview", "", "", allLevels},
		{"Session manager", "", "", allLevels},
		{"Column visibility", "", "", []model.Level{model.LevelResources, model.LevelOwned, model.LevelContainers}},
		{"Monitoring dashboard", "", "", []model.Level{model.LevelResourceTypes, model.LevelResources, model.LevelOwned, model.LevelContainers}},
		{"Quota dashboard", "", "", allLevels},
		{"Task queue", "", "", allLevels},
		{"Error log", "", "", allLevels},
		{"Finalizer search", "", "", allLevels},
		{"Cycle layout", "", "", allLevels},
		{"Pin/unpin type", "", "apps/v1/deployments", []model.Level{model.LevelResourceTypes}},
		{"Show rare/hidden types", "", "", allLevels},
		{"Local cluster manager", "", "", []model.Level{model.LevelClusters}},
		// handleKeyOpenMarks opens the overlay unconditionally (update_keys.go).
		{"Bookmarks", "", "", allLevels},

		{"Filter list", "", "", allLevels},
		{"Search and jump", "", "", allLevels},
		{"Filter presets", "", "", []model.Level{model.LevelResources, model.LevelOwned, model.LevelContainers}},
		{"Namespace selector", "", "", []model.Level{model.LevelResourceTypes, model.LevelResources, model.LevelOwned, model.LevelContainers}},
		{"All namespaces", "", "", []model.Level{model.LevelResourceTypes, model.LevelResources, model.LevelOwned, model.LevelContainers}},
		{"Command bar", "", "", allLevels},

		{"Sort next column", "", "", []model.Level{model.LevelResources, model.LevelOwned, model.LevelContainers}},
		{"Sort previous column", "", "", []model.Level{model.LevelResources, model.LevelOwned, model.LevelContainers}},
		{"Flip sort direction", "", "", []model.Level{model.LevelResources, model.LevelOwned, model.LevelContainers}},
		{"Reset sort", "", "", []model.Level{model.LevelResources, model.LevelOwned, model.LevelContainers}},

		{"Watch mode", "", "", allLevels},
		{"Read-only mode", "", "", allLevels},
		{"Color scheme", "", "", allLevels},
		{"Terminal mode", "", "", allLevels},
		{"Reveal secret values", "", "", allLevels},
		{"Security badge", "", "", allLevels},
		{"Cluster color", "", "", []model.Level{model.LevelClusters}},
		{"Full help", "", "", allLevels},
	}
}

// BenchmarkAvailableWhichKeyActions measures the per-render cost of the
// filter, which Task 4 calls from View() on every frame. LevelResources with
// no filter is the common case; the ResourceTypes and Filtered variants
// isolate the cost of selectedMiddleItem -> visibleMiddleItems, which
// allocates a fresh slice on those paths and is called once per predicate.
func BenchmarkAvailableWhichKeyActions(b *testing.B) {
	prevKb := ui.ActiveKeybindings
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	b.Cleanup(func() { ui.ActiveKeybindings = prevKb })

	rows := make([]model.Item, 200)
	for i := range rows {
		rows[i] = model.Item{Name: fmt.Sprintf("pod-%03d", i), Kind: "Pod", Namespace: "default", Category: "Workloads", Extra: "/v1/pods"}
	}

	variants := []struct {
		name  string
		build func() Model
	}{
		{"LevelResources", func() Model {
			m := whichKeyTestModel()
			m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
			m.setMiddleItems(rows)
			m.setCursor(0)
			return m
		}},
		{"LevelResourceTypes", func() Model {
			m := whichKeyTestModel()
			m.nav.Level = model.LevelResourceTypes
			m.setMiddleItems(rows)
			m.setCursor(0)
			return m
		}},
		{"LevelResourcesFiltered", func() Model {
			m := whichKeyTestModel()
			m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
			m.setMiddleItems(rows)
			m.filterText = "pod"
			m.setCursor(0)
			return m
		}},
	}

	for _, v := range variants {
		m := v.build()
		b.Run(v.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = m.availableWhichKeyActions()
			}
		})
	}
}

func TestWhichKeyPredicates_ZeroValueModelDoesNotPanic(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	var m Model
	c := newWKCtx(&m)
	for _, a := range whichKeyExplorerActions() {
		if a.Avail == nil {
			continue
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("predicate for %q panicked on zero-value Model: %v", a.Label, r)
				}
			}()
			_ = a.Avail(c)
		}()
	}
}

// The panel re-renders on every keypress while it is open. Resolving the row
// once per render is the difference between one filter pass and one per entry.
// TestAvailableWhichKeyActions_ResolvesRowOncePerCall is deliberately
// adversarial: 200 rows plus an active filter, matching
// BenchmarkAvailableWhichKeyActions/LevelResourcesFiltered. A 3-row model (the
// prior version of this test, threshold 12) hid a real regression — a
// predicate that calls selectedMiddleItem() -> visibleMiddleItems() a second
// time only costs ~2 extra allocs against a handful of rows, comfortably
// under a threshold of 12, but costs a full extra re-filter pass at list
// sizes the panel actually renders against (measured: 211 allocs/op for one
// pass at 200 rows vs 420 allocs/op with the stray second pass wkOnSecurityView
// now avoids — see its doc comment). The threshold below sits with headroom
// above the single-pass cost and well below what a second pass would add, so
// it stays stable across minor allocation-count drift from unrelated changes
// while still catching a reintroduced re-filter.
const wkResolvesRowOncePerCallThreshold = 300

func TestAvailableWhichKeyActions_ResolvesRowOncePerCall(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	rows := make([]model.Item, 200)
	for i := range rows {
		rows[i] = model.Item{Name: fmt.Sprintf("pod-%03d", i), Kind: "Pod", Namespace: "default", Category: "Workloads", Extra: "/v1/pods"}
	}
	m := whichKeyTestModel()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
	m.setMiddleItems(rows)
	m.filterText = "pod" // force visibleMiddleItems to do real filtering work
	m.setCursor(0)
	before := testing.AllocsPerRun(100, func() { _ = m.availableWhichKeyActions() })
	if before > wkResolvesRowOncePerCallThreshold {
		t.Fatalf("availableWhichKeyActions allocates %.0f times per call (threshold %d); the row must be resolved once per entry, not re-filtered by an individual predicate", before, wkResolvesRowOncePerCallThreshold)
	}
}
