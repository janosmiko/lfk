package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- renderableBuiltin ---

func TestRenderableBuiltin(t *testing.T) {
	allWidths := builtinColWidths{context: 8, ns: 10, ready: 6, restarts: 4, status: 12, age: 5}

	tests := []struct {
		name        string
		key         string
		widths      builtinColWidths
		wantNonNil  bool
		wantKeyBack string
	}{
		{name: "Namespace with positive width", key: "Namespace", widths: allWidths, wantNonNil: true, wantKeyBack: "Namespace"},
		{name: "Ready with positive width", key: "Ready", widths: allWidths, wantNonNil: true, wantKeyBack: "Ready"},
		{name: "Restarts with positive width", key: "Restarts", widths: allWidths, wantNonNil: true, wantKeyBack: "Restarts"},
		{name: "Status with positive width", key: "Status", widths: allWidths, wantNonNil: true, wantKeyBack: "Status"},
		{name: "Age with positive width", key: "Age", widths: allWidths, wantNonNil: true, wantKeyBack: "Age"},
		{name: "Context with positive width", key: "Context", widths: allWidths, wantNonNil: true, wantKeyBack: "Context"},

		// Non-Context builtins remain renderable even at width zero — callers
		// may still emit a zero-width cell.
		{name: "Namespace with zero width still renderable", key: "Namespace", widths: builtinColWidths{}, wantNonNil: true, wantKeyBack: "Namespace"},
		{name: "Status with zero width still renderable", key: "Status", widths: builtinColWidths{}, wantNonNil: true, wantKeyBack: "Status"},

		// Context is gated — zero or negative width falls through to extras.
		{name: "Context with zero width falls through", key: "Context", widths: builtinColWidths{}, wantNonNil: false},
		{name: "Context with negative width falls through", key: "Context", widths: builtinColWidths{context: -1}, wantNonNil: false},

		// Unknown keys are never registry entries.
		{name: "unknown key", key: "CPU", widths: allWidths, wantNonNil: false},
		{name: "empty key", key: "", widths: allWidths, wantNonNil: false},
		{name: "Name is not a builtin column", key: "Name", widths: allWidths, wantNonNil: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderableBuiltin(tt.key, tt.widths)
			if !tt.wantNonNil {
				assert.Nil(t, got, "expected nil for key=%q", tt.key)
				return
			}
			if assert.NotNil(t, got, "expected non-nil for key=%q", tt.key) {
				assert.Equal(t, tt.wantKeyBack, got.key)
			}
		})
	}
}

// --- isBuiltinColumnKey ---

// Pinned so that drifts between the registry and the predicate (Context
// inclusion in particular) surface as test failures rather than runtime
// renderer bugs.
func TestIsBuiltinColumnKey(t *testing.T) {
	for _, key := range []string{"Namespace", "Ready", "Restarts", "Status", "Age"} {
		assert.Truef(t, isBuiltinColumnKey(key), "%q should be a strict builtin", key)
	}
	assert.False(t, isBuiltinColumnKey("Context"), "Context is intentionally excluded")
	assert.False(t, isBuiltinColumnKey("Name"), "Name is the leading column, not a strict builtin")
	assert.False(t, isBuiltinColumnKey("CPU"), "extras like CPU are not strict builtins")
	assert.False(t, isBuiltinColumnKey(""), "empty key is not a builtin")
}

// --- builtinColumns registry shape ---

func TestBuiltinColumnsRegistryHasExpectedKeys(t *testing.T) {
	expected := map[string]bool{
		"Context":   true,
		"Namespace": true,
		"Ready":     true,
		"Restarts":  true,
		"Status":    true,
		"Age":       true,
	}
	got := make(map[string]bool, len(builtinColumns))
	for _, col := range builtinColumns {
		// A duplicate key would otherwise overwrite the prior entry in
		// builtinColumnsByKey (the first would be dead) and silently
		// leave the set-equality check below intact, since map writes
		// of the same key don't grow the set.
		assert.Falsef(t, got[col.key], "duplicate registry key %q", col.key)
		got[col.key] = true
		assert.NotNil(t, col.width, "%q missing width fn", col.key)
		assert.NotNil(t, col.header, "%q missing header fn", col.key)
		assert.NotNil(t, col.plain, "%q missing plain fn", col.key)
		assert.NotNil(t, col.styled, "%q missing styled fn", col.key)
	}
	assert.Len(t, builtinColumns, len(expected), "registry entry count drifted from expected set")
	assert.Equal(t, expected, got, "registry keys drifted from expected set")
}
