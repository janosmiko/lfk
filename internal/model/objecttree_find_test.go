package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindObjectPaths_FindsNestedKeys(t *testing.T) {
	matches := FindObjectPaths(choreObj(), "name", 0)
	// metadata.name, status.steps[0].name, status.steps[0].steps[0].name,
	// status.steps[1].name
	paths := make([][]string, 0, len(matches))
	for _, m := range matches {
		paths = append(paths, m.Segs)
	}
	assert.Contains(t, paths, []string{"metadata", "name"})
	assert.Contains(t, paths, []string{"status", "steps", "[0]", "name"})
	assert.Contains(t, paths, []string{"status", "steps", "[0]", "steps", "[0]", "name"})
	assert.Contains(t, paths, []string{"status", "steps", "[1]", "name"})
}

func TestFindObjectPaths_CaseInsensitiveSubstring(t *testing.T) {
	matches := FindObjectPaths(choreObj(), "PHA", 0)
	require.NotEmpty(t, matches)
	for _, m := range matches {
		last := m.Segs[len(m.Segs)-1]
		assert.Contains(t, last, "pha") // "phase"
	}
}

func TestFindObjectPaths_EmptyQuery(t *testing.T) {
	assert.Empty(t, FindObjectPaths(choreObj(), "  ", 0))
}

func TestFindObjectPaths_PreviewPopulated(t *testing.T) {
	matches := FindObjectPaths(choreObj(), "phase", 0)
	require.NotEmpty(t, matches)
	// status.phase preview is the scalar value.
	found := false
	for _, m := range matches {
		if len(m.Segs) == 2 && m.Segs[0] == "status" && m.Segs[1] == "phase" {
			assert.Equal(t, "Running", m.Preview)
			found = true
		}
	}
	assert.True(t, found)
}

func TestFindObjectPaths_Limit(t *testing.T) {
	matches := FindObjectPaths(choreObj(), "name", 2)
	assert.Len(t, matches, 2)
}
