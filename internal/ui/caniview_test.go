package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- canIVerbColWidth ---

func TestCanIVerbColWidth(t *testing.T) {
	tests := []struct {
		label    string
		expected int
	}{
		{"GET", 4},
		{"LIST", 5},
		{"DELETE", 7},
		{"", 1},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			assert.Equal(t, tt.expected, canIVerbColWidth(tt.label))
		})
	}
}

// --- canITotalVerbWidth ---

func TestCanITotalVerbWidth(t *testing.T) {
	t.Run("returns positive sum of all verb columns", func(t *testing.T) {
		total := canITotalVerbWidth()
		assert.Greater(t, total, 0)

		// Verify it matches manual sum.
		expected := 0
		for _, v := range canIVerbs {
			expected += canIVerbColWidth(v.label)
		}
		assert.Equal(t, expected, total)
	})
}

// --- renderCanIMiddleHeader ---

func TestRenderCanIMiddleHeader(t *testing.T) {
	t.Run("contains RESOURCE label", func(t *testing.T) {
		header := renderCanIMiddleHeader(120)
		assert.Contains(t, header, "RESOURCE")
	})

	t.Run("contains verb labels", func(t *testing.T) {
		header := renderCanIMiddleHeader(120)
		for _, v := range canIVerbs {
			assert.Contains(t, header, v.label, "header should contain verb %q", v.label)
		}
	})

	t.Run("narrow width still contains RESOURCE", func(t *testing.T) {
		header := renderCanIMiddleHeader(40)
		assert.Contains(t, header, "RESOURCE")
	})
}

// --- renderCanIGroups ---

func TestRenderCanIGroups(t *testing.T) {
	t.Run("empty groups shows no groups message", func(t *testing.T) {
		lines := renderCanIGroups(nil, 0, 0, 30, 10)
		assert.Len(t, lines, 10)
		assert.Contains(t, lines[0], "No groups found")
	})

	t.Run("cursor on first group shows selection prefix", func(t *testing.T) {
		groups := []string{"apps", "batch", "core"}
		lines := renderCanIGroups(groups, 0, 0, 30, 10)
		assert.Len(t, lines, 10)
		assert.Contains(t, lines[0], ">")
		assert.Contains(t, lines[0], "apps")
	})

	t.Run("cursor on second group", func(t *testing.T) {
		groups := []string{"apps", "batch", "core"}
		lines := renderCanIGroups(groups, 1, 0, 30, 10)
		assert.Contains(t, lines[1], ">")
		assert.Contains(t, lines[1], "batch")
	})

	t.Run("pads to maxLines", func(t *testing.T) {
		groups := []string{"apps"}
		lines := renderCanIGroups(groups, 0, 0, 30, 5)
		assert.Len(t, lines, 5)
	})

	t.Run("scrolling shows later items", func(t *testing.T) {
		groups := make([]string, 20)
		for i := range groups {
			groups[i] = strings.Repeat("g", i+1)
		}
		lines := renderCanIGroups(groups, 15, 15, 30, 5)
		// Should show items starting near cursor 15.
		found := false
		for _, line := range lines {
			if strings.Contains(line, ">") {
				found = true
			}
		}
		assert.True(t, found, "should show cursor indicator")
	})
}

// --- renderCanIResources ---

func TestRenderCanIResources(t *testing.T) {
	t.Run("empty resources shows message", func(t *testing.T) {
		lines := renderCanIResources(nil, 80, 10, 0)
		assert.Len(t, lines, 10)
		assert.Contains(t, lines[0], "No resources in this group")
	})

	t.Run("resources with verbs show check marks", func(t *testing.T) {
		resources := []model.CanIResource{
			{
				Resource: "deployments",
				Verbs:    map[string]bool{"get": true, "list": true, "delete": false},
			},
		}
		lines := renderCanIResources(resources, 80, 10, 0)
		assert.Len(t, lines, 10)
		// Should contain the resource name.
		assert.Contains(t, lines[0], "deployments")
		// Check mark for allowed verbs.
		assert.Contains(t, lines[0], "\u2713")
	})

	t.Run("pads to maxLines", func(t *testing.T) {
		resources := []model.CanIResource{
			{Resource: "pods", Verbs: map[string]bool{}},
		}
		lines := renderCanIResources(resources, 80, 5, 0)
		assert.Len(t, lines, 5)
	})

	t.Run("multiple resources rendered", func(t *testing.T) {
		resources := []model.CanIResource{
			{Resource: "pods", Verbs: map[string]bool{"get": true}},
			{Resource: "services", Verbs: map[string]bool{"create": true}},
		}
		lines := renderCanIResources(resources, 80, 10, 0)
		foundPods := false
		foundServices := false
		for _, line := range lines {
			if strings.Contains(line, "pods") {
				foundPods = true
			}
			if strings.Contains(line, "services") {
				foundServices = true
			}
		}
		assert.True(t, foundPods, "should contain pods")
		assert.True(t, foundServices, "should contain services")
	})
}

// --- padCanIToHeight ---

func TestPadCanIToHeight(t *testing.T) {
	t.Run("already at height unchanged", func(t *testing.T) {
		input := "line1\nline2\nline3"
		result := padCanIToHeight(input, 3)
		assert.Equal(t, 3, len(strings.Split(result, "\n")))
	})

	t.Run("shorter string padded", func(t *testing.T) {
		input := "line1"
		result := padCanIToHeight(input, 5)
		lines := strings.Split(result, "\n")
		assert.Equal(t, 5, len(lines))
		assert.Equal(t, "line1", lines[0])
		assert.Equal(t, "", lines[1])
	})

	t.Run("taller string truncated", func(t *testing.T) {
		input := "a\nb\nc\nd\ne"
		result := padCanIToHeight(input, 3)
		lines := strings.Split(result, "\n")
		assert.Equal(t, 3, len(lines))
		assert.Equal(t, "a", lines[0])
	})
}

// --- RenderCanIView ---

func TestRenderCanIView(t *testing.T) {
	t.Run("basic rendering contains expected elements", func(t *testing.T) {
		groups := []string{"apps", "batch", "core"}
		resources := []model.CanIResource{
			{Resource: "deployments", Verbs: map[string]bool{"get": true, "list": true}},
			{Resource: "statefulsets", Verbs: map[string]bool{"get": false}},
		}
		result := RenderCanIView(groups, resources, 0, 0, "admin", []string{"default"}, 120, 30, "hint bar text", 0)
		assert.Contains(t, result, "RBAC Permissions")
		assert.Contains(t, result, "admin")
		assert.Contains(t, result, "API Groups")
		assert.Contains(t, result, "apps")
		assert.Contains(t, result, "deployments")
		assert.Contains(t, result, "hint bar text")
	})

	t.Run("narrow width does not panic", func(t *testing.T) {
		result := RenderCanIView([]string{"apps"}, nil, 0, 0, "user", []string{"default"}, 40, 10, "", 0)
		assert.Contains(t, result, "RBAC")
	})

	t.Run("empty groups and resources", func(t *testing.T) {
		result := RenderCanIView(nil, nil, 0, 0, "test-user", []string{"ns1"}, 80, 20, "", 0)
		assert.Contains(t, result, "No groups found")
		assert.Contains(t, result, "No resources in this group")
	})
}

// --- RenderCanISubjectOverlay ---

func TestRenderCanISubjectOverlay(t *testing.T) {
	t.Run("nil items shows loading", func(t *testing.T) {
		result := RenderCanISubjectOverlay(nil, "", 0, false)
		assert.Contains(t, result, "Select Subject")
		assert.Contains(t, result, "Loading subjects")
	})

	t.Run("empty items shows no matching", func(t *testing.T) {
		result := RenderCanISubjectOverlay([]model.Item{}, "", 0, false)
		assert.Contains(t, result, "No matching subjects")
	})

	t.Run("items rendered with cursor", func(t *testing.T) {
		items := []model.Item{
			{Name: "admin"},
			{Name: "viewer"},
			{Name: "editor"},
		}
		ResetOverlayCanISubjectScroll()
		result := RenderCanISubjectOverlay(items, "", 1, false)
		assert.Contains(t, result, "Select Subject")
		assert.Contains(t, result, "admin")
		assert.Contains(t, result, "viewer")
		assert.Contains(t, result, "editor")
		assert.Contains(t, result, ">")
	})

	t.Run("filter mode active shows cursor block", func(t *testing.T) {
		items := []model.Item{{Name: "admin"}}
		ResetOverlayCanISubjectScroll()
		result := RenderCanISubjectOverlay(items, "adm", 0, true)
		assert.Contains(t, result, "adm")
		assert.Contains(t, result, "\u2588")
	})

	t.Run("filter text without filter mode", func(t *testing.T) {
		items := []model.Item{{Name: "admin"}}
		ResetOverlayCanISubjectScroll()
		result := RenderCanISubjectOverlay(items, "adm", 0, false)
		assert.Contains(t, result, "/ adm")
		assert.NotContains(t, result, "\u2588")
	})

	t.Run("no filter shows placeholder", func(t *testing.T) {
		items := []model.Item{{Name: "admin"}}
		ResetOverlayCanISubjectScroll()
		result := RenderCanISubjectOverlay(items, "", 0, false)
		assert.Contains(t, result, "/ to filter")
	})

	t.Run("hint bar removed from overlay body", func(t *testing.T) {
		// Hints now live in the main status bar, not inline.
		items := []model.Item{{Name: "admin"}}
		ResetOverlayCanISubjectScroll()
		result := RenderCanISubjectOverlay(items, "", 0, false)
		assert.NotContains(t, result, "Enter: select")
		assert.NotContains(t, result, "Esc: cancel")
	})
}
