package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// choreYAML is in the viewer's displayed form (list items indented under their
// parent key, as produced by the YAML view's indentYAMLListItems transform).
const choreYAML = `apiVersion: chore.example.com/v1
kind: Chore
metadata:
  name: chore-1
status:
  phase: Running
  steps:
    - name: build
      phase: Succeeded
      steps:
        - name: compile
          phase: Succeeded
    - name: deploy
      phase: Pending
`

func TestYAMLLineForPath(t *testing.T) {
	cases := []struct {
		name string
		segs []string
		want int // 0-based physical line
	}{
		{"top-level key", []string{"kind"}, 1},
		{"nested map key", []string{"metadata", "name"}, 3},
		{"array element", []string{"status", "steps", "[0]"}, 7},
		{"array element leaf", []string{"status", "steps", "[0]", "phase"}, 8},
		{"deep nested array", []string{"status", "steps", "[0]", "steps", "[0]", "name"}, 10},
		{"second array element", []string{"status", "steps", "[1]"}, 12},
		{"second element leaf", []string{"status", "steps", "[1]", "phase"}, 13},
		{"missing path", []string{"status", "nope"}, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, YAMLLineForPath(choreYAML, tc.segs))
		})
	}
}

func TestPathForYAMLLine(t *testing.T) {
	cases := []struct {
		line int
		want []string
	}{
		{1, []string{"kind"}},
		{3, []string{"metadata", "name"}},
		{5, []string{"status", "phase"}},
		{7, []string{"status", "steps", "[0]", "name"}},
		{8, []string{"status", "steps", "[0]", "phase"}},
		{10, []string{"status", "steps", "[0]", "steps", "[0]", "name"}},
		{12, []string{"status", "steps", "[1]", "name"}},
	}
	for _, tc := range cases {
		t.Run(tc.want[len(tc.want)-1], func(t *testing.T) {
			assert.Equal(t, tc.want, PathForYAMLLine(choreYAML, tc.line))
		})
	}
}

func TestYAMLPath_RoundTrip(t *testing.T) {
	// A path resolved to a line, then back, yields a path with the original as
	// a prefix (the line may carry an inline key beyond the container path).
	segs := []string{"status", "steps", "[1]"}
	line := YAMLLineForPath(choreYAML, segs)
	require.GreaterOrEqual(t, line, 0)
	got := PathForYAMLLine(choreYAML, line)
	assert.True(t, hasPathPrefix(got, segs), "got %v should start with %v", got, segs)
}
