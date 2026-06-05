package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func choreObj() map[string]any {
	return map[string]any{
		"apiVersion": "chore.example.com/v1",
		"kind":       "Chore",
		"metadata":   map[string]any{"name": "chore-1", "generation": int64(3)},
		"status": map[string]any{
			"phase": "Running",
			"steps": []any{
				map[string]any{
					"name":  "build",
					"phase": "Succeeded",
					"steps": []any{
						map[string]any{"name": "compile", "ok": true},
					},
				},
				map[string]any{"name": "deploy", "phase": "Pending"},
			},
		},
	}
}

func TestObjectFieldsAt_RootKeysSorted(t *testing.T) {
	fields := ObjectFieldsAt(choreObj(), nil)
	keys := make([]string, len(fields))
	for i, f := range fields {
		keys[i] = f.Key
	}
	assert.Equal(t, []string{"apiVersion", "kind", "metadata", "status"}, keys)
}

func TestObjectFieldsAt_Types(t *testing.T) {
	fields := ObjectFieldsAt(choreObj(), nil)
	byKey := map[string]ObjectField{}
	for _, f := range fields {
		byKey[f.Key] = f
	}
	assert.Equal(t, "<string>", byKey["apiVersion"].Type)
	assert.False(t, byKey["apiVersion"].HasChildren)
	assert.Equal(t, "<Object>", byKey["metadata"].Type)
	assert.True(t, byKey["metadata"].HasChildren)
	assert.Equal(t, "<Object>", byKey["status"].Type)
}

func TestObjectFieldsAt_DrillIntoArray(t *testing.T) {
	// status.steps is an array -> indexed elements.
	fields := ObjectFieldsAt(choreObj(), []string{"status", "steps"})
	require.Len(t, fields, 2)
	assert.Equal(t, "[0]", fields[0].Key)
	assert.Equal(t, "[1]", fields[1].Key)
	assert.Equal(t, "<Object>", fields[0].Type)
	assert.True(t, fields[0].HasChildren)
	// Preview labels the array element by a friendly name when available.
	assert.Contains(t, fields[0].Preview, "build")
}

func TestObjectFieldsAt_DrillIntoArrayElement(t *testing.T) {
	fields := ObjectFieldsAt(choreObj(), []string{"status", "steps", "[0]"})
	byKey := map[string]ObjectField{}
	for _, f := range fields {
		byKey[f.Key] = f
	}
	assert.Equal(t, "build", byKey["name"].Preview)
	assert.Equal(t, "<string>", byKey["name"].Type)
	assert.Equal(t, "<[]Object>", byKey["steps"].Type)
	assert.True(t, byKey["steps"].HasChildren)
}

func TestObjectFieldsAt_Leaf(t *testing.T) {
	// A scalar path has no navigable fields.
	fields := ObjectFieldsAt(choreObj(), []string{"status", "phase"})
	assert.Empty(t, fields)
}

func TestObjectFieldsAt_ScalarPreviewAndTypes(t *testing.T) {
	fields := ObjectFieldsAt(choreObj(), []string{"status", "steps", "[0]", "steps", "[0]"})
	byKey := map[string]ObjectField{}
	for _, f := range fields {
		byKey[f.Key] = f
	}
	assert.Equal(t, "compile", byKey["name"].Preview)
	assert.Equal(t, "<boolean>", byKey["ok"].Type)
	assert.Equal(t, "true", byKey["ok"].Preview)
}

func TestResolveObjectPath(t *testing.T) {
	v, ok := ResolveObjectPath(choreObj(), []string{"status", "steps", "[1]", "name"})
	require.True(t, ok)
	assert.Equal(t, "deploy", v)

	_, ok = ResolveObjectPath(choreObj(), []string{"status", "steps", "[9]"})
	assert.False(t, ok)

	_, ok = ResolveObjectPath(choreObj(), []string{"nope"})
	assert.False(t, ok)
}

func TestObjectFieldsAt_EmptyContainers(t *testing.T) {
	obj := map[string]any{"a": map[string]any{}, "b": []any{}}
	fields := ObjectFieldsAt(obj, nil)
	byKey := map[string]ObjectField{}
	for _, f := range fields {
		byKey[f.Key] = f
	}
	// Empty map/array are leaves (nothing to drill into).
	assert.False(t, byKey["a"].HasChildren)
	assert.False(t, byKey["b"].HasChildren)
}
