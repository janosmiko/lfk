package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// evalJSONPath compiles and evaluates expr against obj in one step,
// returning "" for a compile error, matching the removed EvalJSONPath helper.
func evalJSONPath(expr string, obj map[string]any) string {
	jp, err := CompileJSONPath(expr)
	if err != nil {
		return ""
	}
	return EvalCompiled(jp, obj)
}

func TestEvalJSONPath(t *testing.T) {
	obj := map[string]any{
		"metadata": map[string]any{
			"name":              "nginx",
			"resourceVersion":   "12345",
			"creationTimestamp": "2026-05-23T10:00:00Z",
			"labels": map[string]any{
				"app": "web",
			},
		},
		"spec": map[string]any{
			"replicas": float64(3),
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{"name": "main", "image": "nginx:1.27"},
					},
				},
			},
		},
	}
	cases := []struct {
		path string
		want string
	}{
		{path: ".metadata.name", want: "nginx"},
		{path: ".metadata.resourceVersion", want: "12345"},
		{path: ".spec.replicas", want: "3"},
		{path: ".spec.template.spec.containers[0].image", want: "nginx:1.27"},
		{path: ".metadata.labels.app", want: "web"},
		{path: ".does.not.exist", want: ""},
		{path: ".spec.replicas.bogus", want: ""},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			got := evalJSONPath(c.path, obj)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestEvalJSONPath_InvalidExpression(t *testing.T) {
	got := evalJSONPath("not a valid path", map[string]any{})
	assert.Equal(t, "", got)
}

func TestEvalJSONPath_EmptyExpr(t *testing.T) {
	assert.Equal(t, "", evalJSONPath("", map[string]any{"x": 1}))
}

func TestEvalJSONPath_NilObj(t *testing.T) {
	assert.Equal(t, "", evalJSONPath(".x", nil))
}

func TestCompileAndEval(t *testing.T) {
	jp, err := CompileJSONPath(".metadata.name")
	assert.NoError(t, err)
	got := EvalCompiled(jp, map[string]any{"metadata": map[string]any{"name": "abc"}})
	assert.Equal(t, "abc", got)
}

func TestCompileJSONPath_InvalidReturnsError(t *testing.T) {
	_, err := CompileJSONPath("not a valid path")
	assert.Error(t, err)
}
