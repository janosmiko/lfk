package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseColumnSpec(t *testing.T) {
	cases := []struct {
		input     string
		wantName  string
		wantPath  string
		wantFlags ColumnFlags
		wantOK    bool
	}{
		{input: "NAME", wantName: "NAME", wantOK: true},
		{input: "REV", wantName: "REV", wantOK: true},
		{input: "IMAGE:.spec.template.spec.containers[0].image", wantName: "IMAGE", wantPath: ".spec.template.spec.containers[0].image", wantOK: true},
		{input: "REV:.metadata.resourceVersion", wantName: "REV", wantPath: ".metadata.resourceVersion", wantOK: true},
		{input: "AGE:.metadata.creationTimestamp|T", wantName: "AGE", wantPath: ".metadata.creationTimestamp", wantFlags: FlagTimestamp, wantOK: true},
		{input: "CPU:.usage|R", wantName: "CPU", wantPath: ".usage", wantFlags: FlagRightAlign, wantOK: true},
		{input: "NODE:.spec.nodeName|W", wantName: "NODE", wantPath: ".spec.nodeName", wantFlags: FlagWideOnly, wantOK: true},
		{input: "USAGE:.x|R|W", wantName: "USAGE", wantPath: ".x", wantFlags: FlagRightAlign | FlagWideOnly, wantOK: true},
		{input: "MIXEDCASE:.x|r|w", wantName: "MIXEDCASE", wantPath: ".x", wantFlags: FlagRightAlign | FlagWideOnly, wantOK: true},
		{input: "  PADDED  ", wantName: "PADDED", wantOK: true},
		{input: "", wantOK: false},
		{input: ":.path", wantOK: false},       // empty name
		{input: "NAME:|R", wantOK: false},      // empty path after colon
		{input: "NAME:.path|Q", wantOK: false}, // unknown flag
		{input: "NAME:.path|", wantOK: false},  // empty flag
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			spec, ok := ParseColumnSpec(c.input)
			assert.Equal(t, c.wantOK, ok)
			if !ok {
				return
			}
			assert.Equal(t, c.wantName, spec.Name)
			assert.Equal(t, c.wantPath, spec.JSONPath)
			assert.Equal(t, c.wantFlags, spec.Flags)
		})
	}
}

func TestColumnSpec_IsCustom(t *testing.T) {
	custom, _ := ParseColumnSpec("X:.foo")
	builtin, _ := ParseColumnSpec("Name")
	assert.True(t, custom.IsCustom())
	assert.False(t, builtin.IsCustom())
}
