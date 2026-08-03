package app

import (
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
	kb.Delete = "X"
	ui.ActiveKeybindings = kb
	if !containsKey(whichKeyKeys(whichKeyTestModel()), "X") {
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

func TestWhichKeyPredicates_ZeroValueModelDoesNotPanic(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	var m Model
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
			_ = a.Avail(&m)
		}()
	}
}
