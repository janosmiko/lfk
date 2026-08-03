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

func TestAvailableWhichKeyActions_OpenBrowserExcludesService(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Service", APIVersion: "v1", Resource: "services", Namespaced: true}
	m.setMiddleItems([]model.Item{{Name: "svc1", Kind: "Service", Namespace: "default"}})
	m.setCursor(0)
	if containsKey(whichKeyKeys(m), ui.ActiveKeybindings.OpenBrowser) {
		t.Fatal("open in browser must not be offered for a Service; that key routes to Port Forward & Open, a different label")
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
