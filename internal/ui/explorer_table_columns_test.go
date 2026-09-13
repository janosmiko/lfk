package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- widthForColumnKey / headerCellForKey ---

func TestBuiltinColumnWidthAndHeader(t *testing.T) {
	allWidths := builtinColWidths{context: 8, ns: 10, ready: 6, restarts: 4, status: 12, age: 5}
	headers := builtinColHeaders{context: "CTX", ns: "NS", ready: "RDY", restarts: "RS", status: "ST", age: "AGE"}
	extras := []extraColumn{{key: "Context", width: 11}}

	tests := []struct {
		name       string
		key        string
		widths     builtinColWidths
		wantWidth  int
		wantHeader string
	}{
		{name: "Context with positive width", key: "Context", widths: allWidths, wantWidth: 8, wantHeader: "CTX"},
		{name: "Namespace", key: "Namespace", widths: allWidths, wantWidth: 10, wantHeader: "NS"},
		{name: "Ready", key: "Ready", widths: allWidths, wantWidth: 6, wantHeader: "RDY"},
		{name: "Restarts", key: "Restarts", widths: allWidths, wantWidth: 4, wantHeader: "RS"},
		{name: "Status", key: "Status", widths: allWidths, wantWidth: 12, wantHeader: "ST"},
		{name: "Age", key: "Age", widths: allWidths, wantWidth: 5, wantHeader: "AGE"},

		// Non-Context builtins stay builtin even at width zero.
		{name: "Namespace with zero width", key: "Namespace", widths: builtinColWidths{}, wantWidth: 0, wantHeader: "NS"},

		// Unknown keys match neither a builtin nor an extra.
		{name: "unknown key", key: "CPU", widths: allWidths},
		{name: "empty key", key: "", widths: allWidths},
		{name: "Name is not a builtin column", key: "Name", widths: allWidths},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantWidth, widthForColumnKey(tt.key, tt.widths, extras))
			assert.Equal(t, tt.wantHeader, headerCellForKey(tt.key, tt.widths, headers, extras))
		})
	}

	// Context is gated: zero or negative width falls through to the extra.
	for _, w := range []int{0, -1} {
		widths := builtinColWidths{context: w}
		assert.Equal(t, 11, widthForColumnKey("Context", widths, extras), "context width %d", w)
		assert.NotEqual(t, "CTX", headerCellForKey("Context", widths, headers, extras), "context width %d", w)
		assert.Contains(t, headerCellForKey("Context", widths, headers, extras), "CONTEXT", "context width %d", w)
	}
}

// --- isBuiltinColumnKey ---

func TestIsBuiltinColumnKey(t *testing.T) {
	for _, key := range []string{"Namespace", "Ready", "Restarts", "Status", "Age"} {
		assert.Truef(t, isBuiltinColumnKey(key), "%q should be a strict builtin", key)
	}
	assert.False(t, isBuiltinColumnKey("Context"), "Context is intentionally excluded")
	assert.False(t, isBuiltinColumnKey("Name"), "Name is the leading column, not a strict builtin")
	assert.False(t, isBuiltinColumnKey("CPU"), "extras like CPU are not strict builtins")
	assert.False(t, isBuiltinColumnKey(""), "empty key is not a builtin")
}

// --- sanitization: builtin cells ---

// builtinCellHostilePayloads mirrors explorer_format_test.go's hostilePayloads:
// a bidi override, a raw CSI sequence, and an OSC-52 clipboard write.
var builtinCellHostilePayloads = map[string]string{
	"bidi override": "ab\u202ecd",
	"raw CSI":       "ab\x1b[2Jcd",
	"OSC-52":        "ab\x1b]52;c;aGF4\x07cd",
}

func assertBuiltinCellClean(t *testing.T, out string) {
	t.Helper()
	assert.NotContains(t, out, "\u202e")
	assert.NotContains(t, out, "\x1b[2J")
	assert.NotContains(t, out, "\x1b]52")
}

func plainBuiltinCell(key, ns, ready, status string) string {
	return formatTableRowOrdered("", ns, ready, "", status, "",
		0, 0, 20, 10, 0, 20, 0, []string{key}, nil, nil)
}

func styledBuiltinCell(key string, item model.Item) string {
	return formatTableRowStyledOrdered(item,
		0, 0, 20, 10, 0, 20, 0, []string{key}, nil, false, nil)
}

func TestBuiltinColumns_NamespaceCellSanitizes(t *testing.T) {
	for name, payload := range builtinCellHostilePayloads {
		t.Run(name, func(t *testing.T) {
			assertBuiltinCellClean(t, plainBuiltinCell("Namespace", payload, "", ""))
			assertBuiltinCellClean(t, styledBuiltinCell("Namespace", model.Item{Namespace: payload}))
		})
	}
	assert.Contains(t, plainBuiltinCell("Namespace", "default", "", ""), "default")
}

func TestBuiltinColumns_ReadyCellSanitizes(t *testing.T) {
	for name, payload := range builtinCellHostilePayloads {
		t.Run(name, func(t *testing.T) {
			assertBuiltinCellClean(t, plainBuiltinCell("Ready", "", payload, ""))
			assertBuiltinCellClean(t, styledBuiltinCell("Ready", model.Item{Ready: payload}))
		})
	}
	assert.Contains(t, plainBuiltinCell("Ready", "", "1/1", ""), "1/1")
}

func TestBuiltinColumns_StatusCellSanitizes(t *testing.T) {
	for name, payload := range builtinCellHostilePayloads {
		t.Run(name, func(t *testing.T) {
			assertBuiltinCellClean(t, plainBuiltinCell("Status", "", "", "Running"+payload))
			assertBuiltinCellClean(t, styledBuiltinCell("Status", model.Item{Status: "Running" + payload}))
		})
	}
	assert.Contains(t, plainBuiltinCell("Status", "", "", "Running"), "Running")
}
