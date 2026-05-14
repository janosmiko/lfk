package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateYamlClipboardStatusByFormat(t *testing.T) {
	tests := []struct {
		name     string
		msg      yamlClipboardMsg
		expected string
	}{
		{"single yaml (legacy empty format)", yamlClipboardMsg{content: "x", count: 1}, "YAML copied to clipboard"},
		{"single yaml (explicit)", yamlClipboardMsg{content: "x", count: 1, format: "yaml"}, "YAML copied to clipboard"},
		{"single json", yamlClipboardMsg{content: "x", count: 1, format: "json"}, "JSON copied to clipboard"},
		{"single table", yamlClipboardMsg{content: "x", count: 1, format: "table"}, "Table copied to clipboard"},
		{"bulk yaml", yamlClipboardMsg{content: "x", count: 4, format: "yaml"}, "Copied 4 manifests as YAML"},
		{"bulk json", yamlClipboardMsg{content: "x", count: 4, format: "json"}, "Copied 4 manifests as JSON"},
		{"bulk table", yamlClipboardMsg{content: "x", count: 4, format: "table"}, "Copied 4 rows as Table"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{}
			ret, _ := m.updateYamlClipboard(tt.msg)
			rm := ret.(Model)
			assert.Equal(t, tt.expected, rm.statusMessage)
		})
	}
}
