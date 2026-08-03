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
