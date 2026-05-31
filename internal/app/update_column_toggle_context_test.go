package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// TestColumnMemoryIsPerContext verifies column visibility chosen in one
// cluster context does not leak into another, and is restored when the user
// returns to the original context — mirroring sort memory.
func TestColumnMemoryIsPerContext(t *testing.T) {
	defer func(s []string) { ui.ActiveSessionColumns = s }(ui.ActiveSessionColumns)
	defer func(h map[string]bool) { ui.ActiveHiddenBuiltinColumns = h }(ui.ActiveHiddenBuiltinColumns)

	m := baseModelCov()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods"}

	// In "prod": add extra column IP and hide the Ready built-in.
	m.nav.Context = "prod"
	m.columnToggleItems = []columnToggleEntry{
		{key: "Namespace", visible: true, builtin: true},
		{key: "Ready", visible: false, builtin: true},
		{key: "IP", visible: true, builtin: false},
	}
	m.applyColumnToggleState()

	if _, ok := m.sessionColumns[m.columnMemoryKey("pod")]; !ok {
		t.Fatalf("prod column config must be stored under its context-scoped key")
	}

	// Switch to "dev": the render path must see no override for the same kind.
	m.nav.Context = "dev"
	m.applySessionColumnsForKind("pod")
	if ui.ActiveSessionColumns != nil {
		t.Fatalf("dev must not inherit prod's extra columns, got %v", ui.ActiveSessionColumns)
	}
	if ui.ActiveHiddenBuiltinColumns != nil {
		t.Fatalf("dev must not inherit prod's hidden built-ins, got %v", ui.ActiveHiddenBuiltinColumns)
	}

	// Back to "prod": the choice applies again.
	m.nav.Context = "prod"
	m.applySessionColumnsForKind("pod")
	if len(ui.ActiveSessionColumns) != 1 || ui.ActiveSessionColumns[0] != "IP" {
		t.Fatalf("prod extras must persist on return, got %v", ui.ActiveSessionColumns)
	}
	if !ui.ActiveHiddenBuiltinColumns["Ready"] {
		t.Fatalf("prod hidden built-in Ready must persist on return, got %v", ui.ActiveHiddenBuiltinColumns)
	}
}

// TestColumnMemoryResetIsPerContext verifies the R reset clears only the
// current context's column config, leaving other contexts untouched.
func TestColumnMemoryResetIsPerContext(t *testing.T) {
	m := baseModelCov()
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", Resource: "pods"}

	m.nav.Context = "prod"
	prodKey := m.columnMemoryKey("pod")
	m.nav.Context = "dev"
	devKey := m.columnMemoryKey("pod")
	m.sessionColumns = map[string][]string{prodKey: {"IP"}, devKey: {"Node"}}

	m.nav.Context = "prod"
	m.overlay = overlayColumnToggle
	m.handleColumnToggleKeyR()

	if _, ok := m.sessionColumns[prodKey]; ok {
		t.Fatalf("reset must clear prod's column config")
	}
	if got := m.sessionColumns[devKey]; len(got) != 1 || got[0] != "Node" {
		t.Fatalf("reset must not touch dev's column config, got %v", got)
	}
}
