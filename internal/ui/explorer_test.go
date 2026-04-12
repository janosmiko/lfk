package ui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- resolveIcon ---

func TestResolveIcon(t *testing.T) {
	t.Run("empty icon always returns empty", func(t *testing.T) {
		for _, mode := range []string{"unicode", "simple", "emoji", "none"} {
			origMode := IconMode
			IconMode = mode
			assert.Equal(t, "", resolveIcon(model.Icon{}))
			IconMode = origMode
		}
	})
}

// --- isItemSelected ---

func TestIsItemSelected(t *testing.T) {
	t.Run("nil map returns false", func(t *testing.T) {
		origSel := ActiveSelectedItems
		ActiveSelectedItems = nil
		defer func() { ActiveSelectedItems = origSel }()
		assert.False(t, isItemSelected(model.Item{Name: "test"}))
	})

	t.Run("item without namespace", func(t *testing.T) {
		origSel := ActiveSelectedItems
		ActiveSelectedItems = map[string]bool{"my-pod": true}
		defer func() { ActiveSelectedItems = origSel }()
		assert.True(t, isItemSelected(model.Item{Name: "my-pod"}))
		assert.False(t, isItemSelected(model.Item{Name: "other-pod"}))
	})

	t.Run("item with namespace", func(t *testing.T) {
		origSel := ActiveSelectedItems
		ActiveSelectedItems = map[string]bool{"default/my-pod": true}
		defer func() { ActiveSelectedItems = origSel }()
		assert.True(t, isItemSelected(model.Item{Name: "my-pod", Namespace: "default"}))
		assert.False(t, isItemSelected(model.Item{Name: "my-pod", Namespace: "kube-system"}))
		assert.False(t, isItemSelected(model.Item{Name: "my-pod"}))
	})
}

// --- highlightName ---

func TestHighlightName(t *testing.T) {
	t.Run("empty query returns original", func(t *testing.T) {
		assert.Equal(t, "hello", highlightName("hello", ""))
	})

	t.Run("no match returns original", func(t *testing.T) {
		assert.Equal(t, "hello", highlightName("hello", "xyz"))
	})

	t.Run("case insensitive match", func(t *testing.T) {
		result := highlightName("Hello World", "hello")
		// In test mode, lipgloss renders as no-op; just verify it doesn't panic
		// and contains the original text.
		assert.Contains(t, result, "Hello")
	})

	t.Run("partial match", func(t *testing.T) {
		result := highlightName("deployment-nginx", "nginx")
		assert.Contains(t, result, "nginx")
		assert.Contains(t, result, "deployment-")
	})
}

// --- highlightNameSelected ---

func TestHighlightNameSelected(t *testing.T) {
	t.Run("empty query returns original", func(t *testing.T) {
		assert.Equal(t, "hello", highlightNameSelected("hello", ""))
	})

	t.Run("no match returns original", func(t *testing.T) {
		assert.Equal(t, "hello", highlightNameSelected("hello", "xyz"))
	})

	t.Run("match is processed", func(t *testing.T) {
		result := highlightNameSelected("Hello World", "world")
		assert.Contains(t, result, "World")
		assert.Contains(t, result, "Hello ")
	})
}

// --- ParseResourceValue ---

func TestParseResourceValue(t *testing.T) {
	t.Run("CPU values", func(t *testing.T) {
		tests := []struct {
			val      string
			expected int64
		}{
			{"100m", 100},
			{"250m", 250},
			{"1000m", 1000},
			{"1.5", 1500},
			{"0.5", 500},
			{"2", 2000},
			{"", 0},
		}
		for _, tt := range tests {
			assert.Equal(t, tt.expected, ParseResourceValue(tt.val, true), "CPU: %s", tt.val)
		}
	})

	t.Run("memory values", func(t *testing.T) {
		tests := []struct {
			val      string
			expected int64
		}{
			{"128Mi", 128 * 1024 * 1024},
			{"1Gi", 1024 * 1024 * 1024},
			{"1024Ki", 1024 * 1024},
			{"512B", 512},
			{"", 0},
			{"1024", 1024},
		}
		for _, tt := range tests {
			assert.Equal(t, tt.expected, ParseResourceValue(tt.val, false), "Memory: %s", tt.val)
		}
	})

	t.Run("fractional memory", func(t *testing.T) {
		val := ParseResourceValue("1.5Gi", false)
		expected := int64(1.5 * 1024 * 1024 * 1024)
		assert.Equal(t, expected, val)
	})
}

// --- padRight ---

func TestPadRight(t *testing.T) {
	t.Run("shorter string gets padded", func(t *testing.T) {
		result := padRight("hi", 5)
		assert.Equal(t, "hi   ", result)
	})

	t.Run("exact width unchanged", func(t *testing.T) {
		result := padRight("hello", 5)
		assert.Equal(t, "hello", result)
	})

	t.Run("longer string unchanged", func(t *testing.T) {
		result := padRight("hello world", 5)
		assert.Equal(t, "hello world", result)
	})

	t.Run("empty string", func(t *testing.T) {
		result := padRight("", 3)
		assert.Equal(t, "   ", result)
	})
}

// --- Truncate ---

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		maxW     int
		expected string
	}{
		{"zero maxW", "hello", 0, ""},
		{"negative maxW", "hello", -1, ""},
		{"fits exactly", "hello", 5, "hello"},
		{"fits with room", "hi", 5, "hi"},
		{"needs truncation", "hello world", 6, "hello~"},
		{"maxW 1", "hello", 1, "~"},
		{"empty string", "", 5, ""},
		{"unicode", "héllo", 4, "hél~"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, Truncate(tt.s, tt.maxW))
		})
	}
}

// --- FormatCPU ---

func TestFormatCPU(t *testing.T) {
	tests := []struct {
		millis   int64
		expected string
	}{
		{0, "0m"},
		{100, "100m"},
		{999, "999m"},
		{1000, "1.0"},
		{1500, "1.5"},
		{2000, "2.0"},
		{10000, "10.0"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, FormatCPU(tt.millis))
		})
	}
}

// --- FormatMemory ---

func TestFormatMemory(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0B"},
		{512, "512B"},
		{1023, "1023B"},
		{1024, "1Ki"},
		{1024 * 1024, "1Mi"},
		{1024 * 1024 * 1024, "1.0Gi"},
		{1536 * 1024 * 1024, "1.5Gi"},
		{500 * 1024, "500Ki"},
		{256 * 1024 * 1024, "256Mi"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, FormatMemory(tt.bytes))
		})
	}
}

// --- ComputePctStr ---

func TestComputePctStr(t *testing.T) {
	tests := []struct {
		name     string
		used     int64
		refStr   string
		isCPU    bool
		expected string
	}{
		{"empty ref", 100, "", true, "n/a"},
		{"zero ref CPU", 100, "0m", true, "n/a"},
		{"50% CPU", 500, "1000m", true, "50%"},
		{"100% CPU", 1000, "1000m", true, "100%"},
		{"50% memory", 512 * 1024 * 1024, "1Gi", false, "50%"},
		{"over 100%", 2000, "1000m", true, "200%"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ComputePctStr(tt.used, tt.refStr, tt.isCPU))
		})
	}
}

// --- StatusStyle ---

func TestStatusStyle(t *testing.T) {
	// Just verify these don't panic and return non-zero styles.
	statuses := []string{
		"Running", "Active", "Bound", "Available", "Ready",
		"Pending", "ContainerCreating", "Terminating",
		"Failed", "CrashLoopBackOff", "Error", "OOMKilled",
		"Succeeded", "Completed",
		"Warning", "Normal",
		"default", "", "UnknownStatus",
	}
	for _, s := range statuses {
		t.Run(s, func(t *testing.T) {
			style := StatusStyle(s)
			// Just verify it returns a style (no panic).
			_ = style.Render("test")
		})
	}
}

// --- RenderResourceTree ---

func TestRenderResourceTree(t *testing.T) {
	t.Run("nil root shows message", func(t *testing.T) {
		result := RenderResourceTree(nil, 80, 20)
		assert.Contains(t, result, "No resource tree available")
	})

	t.Run("root with no children", func(t *testing.T) {
		root := &model.ResourceNode{
			Name: "my-deploy",
			Kind: "Deployment",
		}
		result := RenderResourceTree(root, 80, 20)
		assert.Contains(t, result, "Resource Map")
		assert.Contains(t, result, "Deployment")
		assert.Contains(t, result, "my-deploy")
		assert.Contains(t, result, "no owned resources")
	})

	t.Run("deployment tree with replicasets and pods", func(t *testing.T) {
		root := &model.ResourceNode{
			Name:      "nginx",
			Kind:      "Deployment",
			Namespace: "default",
			Children: []*model.ResourceNode{
				{
					Name:      "nginx-abc123",
					Kind:      "ReplicaSet",
					Namespace: "default",
					Status:    "Running",
					Children: []*model.ResourceNode{
						{Name: "nginx-abc123-pod1", Kind: "Pod", Namespace: "default", Status: "Running"},
						{Name: "nginx-abc123-pod2", Kind: "Pod", Namespace: "default", Status: "Pending"},
					},
				},
			},
		}
		result := RenderResourceTree(root, 120, 20)
		assert.Contains(t, result, "Resource Map")
		assert.Contains(t, result, "nginx")
		assert.Contains(t, result, "ReplicaSet")
		assert.Contains(t, result, "nginx-abc123")
		assert.Contains(t, result, "Pod")
		assert.Contains(t, result, "nginx-abc123-pod1")
		assert.Contains(t, result, "nginx-abc123-pod2")
		assert.Contains(t, result, "Running")
		assert.Contains(t, result, "Pending")
		// Check child count annotation on the ReplicaSet node.
		assert.Contains(t, result, "2 Pod")
	})

	t.Run("multiple children use correct connectors", func(t *testing.T) {
		root := &model.ResourceNode{
			Name:      "my-svc",
			Kind:      "Service",
			Namespace: "default",
			Children: []*model.ResourceNode{
				{Name: "pod-1", Kind: "Pod", Namespace: "default"},
				{Name: "pod-2", Kind: "Pod", Namespace: "default"},
				{Name: "pod-3", Kind: "Pod", Namespace: "default"},
			},
		}
		result := RenderResourceTree(root, 120, 20)
		assert.Contains(t, result, "├──")
		assert.Contains(t, result, "└──")
		// Check child count annotation on the root node.
		assert.Contains(t, result, "3 Pod")
	})

	t.Run("cross-namespace children show namespace", func(t *testing.T) {
		root := &model.ResourceNode{
			Name:      "my-node",
			Kind:      "Node",
			Namespace: "",
			Children: []*model.ResourceNode{
				{Name: "pod-a", Kind: "Pod", Namespace: "kube-system"},
				{Name: "pod-b", Kind: "Pod", Namespace: "default"},
			},
		}
		result := RenderResourceTree(root, 120, 20)
		// Both pods have different namespaces than root (empty), so both should show ns.
		assert.Contains(t, result, "ns: kube-system")
		assert.Contains(t, result, "ns: default")
	})

	t.Run("same-namespace children hide namespace", func(t *testing.T) {
		root := &model.ResourceNode{
			Name:      "my-deploy",
			Kind:      "Deployment",
			Namespace: "default",
			Children: []*model.ResourceNode{
				{Name: "rs-1", Kind: "ReplicaSet", Namespace: "default"},
			},
		}
		result := RenderResourceTree(root, 120, 20)
		assert.NotContains(t, result, "ns: default")
	})

	t.Run("pod tree with containers", func(t *testing.T) {
		root := &model.ResourceNode{
			Name:      "nginx-pod",
			Kind:      "Pod",
			Namespace: "default",
			Status:    "Running",
			Children: []*model.ResourceNode{
				{Name: "nginx", Kind: "Container", Namespace: "default", Status: "Running"},
				{Name: "sidecar", Kind: "Container", Namespace: "default", Status: "Running"},
			},
		}
		result := RenderResourceTree(root, 120, 20)
		assert.Contains(t, result, "Pod")
		assert.Contains(t, result, "nginx-pod")
		assert.Contains(t, result, "Container")
		assert.Contains(t, result, "nginx")
		assert.Contains(t, result, "sidecar")
		assert.Contains(t, result, "2 Container")
	})
}

// --- truncateNoMarker ---

func TestTruncateNoMarker(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		maxW     int
		expected string
	}{
		{"zero maxW", "hello", 0, ""},
		{"negative maxW", "hello", -1, ""},
		{"fits exactly", "hello", 5, "hello"},
		{"fits with room", "hi", 5, "hi"},
		{"needs truncation", "hello world", 6, "hello "},
		{"maxW 1", "hello", 1, "h"},
		{"empty string", "", 5, ""},
		{"unicode fits", "héllo", 5, "héllo"},
		{"unicode truncated", "héllo world", 4, "héll"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, truncateNoMarker(tt.s, tt.maxW))
		})
	}
}

// --- truncateStr ---

func TestTruncateStr(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		maxLen   int
		expected string
	}{
		{"fits exactly", "hello", 5, "hello"},
		{"fits with room", "hi", 5, "hi"},
		{"needs truncation with ellipsis", "hello world", 8, "hello..."},
		{"maxLen 3 no ellipsis", "hello", 3, "hel"},
		{"maxLen 2 no ellipsis", "hello", 2, "he"},
		{"maxLen 1 no ellipsis", "hello", 1, "h"},
		{"empty string", "", 5, ""},
		{"maxLen 4 with ellipsis", "abcdef", 4, "a..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, truncateStr(tt.s, tt.maxLen))
		})
	}
}

// --- VimScrollOff ---

func TestVimScrollOff(t *testing.T) {
	// Simple displayLines: each entry = 1 line.
	oneLine := func(from, to int) int { return to - from }

	t.Run("negative cursor returns 0", func(t *testing.T) {
		assert.Equal(t, 0, VimScrollOff(0, -1, 10, 5, 2, oneLine))
	})

	t.Run("zero entries returns 0", func(t *testing.T) {
		assert.Equal(t, 0, VimScrollOff(0, 0, 0, 5, 2, oneLine))
	})

	t.Run("all entries fit in viewport returns 0", func(t *testing.T) {
		assert.Equal(t, 0, VimScrollOff(0, 3, 5, 10, 2, oneLine))
	})

	t.Run("cursor at top no scroll needed", func(t *testing.T) {
		result := VimScrollOff(0, 0, 20, 10, 2, oneLine)
		assert.Equal(t, 0, result)
	})

	t.Run("cursor near bottom scrolls down", func(t *testing.T) {
		// cursor=15, 20 entries, height=10, scrolloff=2
		// Cursor should be visible with scrolloff margin.
		result := VimScrollOff(0, 15, 20, 10, 2, oneLine)
		assert.Greater(t, result, 0)
		// Cursor should be within the viewport.
		assert.LessOrEqual(t, result, 15)
	})

	t.Run("cursor above viewport scrolls up", func(t *testing.T) {
		// scroll=10, cursor=5 -> should scroll up to show cursor.
		result := VimScrollOff(10, 5, 20, 10, 2, oneLine)
		assert.LessOrEqual(t, result, 5)
	})

	t.Run("scrolloff clamped to half viewport", func(t *testing.T) {
		// height=10, scrolloff=20 -> clamped to 4.
		result := VimScrollOff(0, 5, 20, 10, 20, oneLine)
		assert.GreaterOrEqual(t, result, 0)
		assert.LessOrEqual(t, result, 5)
	})

	t.Run("no empty space at bottom", func(t *testing.T) {
		// Cursor at last entry: scroll should not leave empty space below.
		result := VimScrollOff(15, 19, 20, 10, 2, oneLine)
		// displayLines(result, 20) should be >= height.
		assert.Equal(t, 10, oneLine(result, 20))
	})

	t.Run("scroll past end clamped", func(t *testing.T) {
		result := VimScrollOff(100, 5, 20, 10, 2, oneLine)
		assert.LessOrEqual(t, result, 5)
	})

	t.Run("negative scroll normalized to 0", func(t *testing.T) {
		// Negative scroll should be treated as 0.
		result := VimScrollOff(-5, 0, 20, 10, 2, oneLine)
		assert.Equal(t, 0, result)
	})

	t.Run("negative scroll with cursor in middle", func(t *testing.T) {
		result := VimScrollOff(-10, 10, 20, 10, 2, oneLine)
		assert.GreaterOrEqual(t, result, 0)
	})
}

// --- styledRestartsCell restart arrow ---

// These tests exercise the restart-arrow rendering logic that used to live
// inline in formatTableRowStyled. The helper was extracted so the ordered
// rendering path can reuse it; the tests still cover the same behavior.
func TestStyledRestartsCell_RestartArrow(t *testing.T) {
	t.Run("recent restart with count > 0 shows arrow", func(t *testing.T) {
		item := model.Item{
			Name:          "my-pod",
			Restarts:      "3",
			LastRestartAt: time.Now().Add(-10 * time.Minute), // 10 minutes ago
		}
		result := styledRestartsCell(item, 10, true)
		assert.Contains(t, result, "↑")
		assert.Contains(t, result, "3")
	})

	t.Run("old restart with count > 0 does not show arrow", func(t *testing.T) {
		item := model.Item{
			Name:          "my-pod",
			Restarts:      "3",
			LastRestartAt: time.Now().Add(-2 * time.Hour), // 2 hours ago
		}
		result := styledRestartsCell(item, 10, true)
		assert.NotContains(t, result, "↑")
		assert.Contains(t, result, "3")
	})

	t.Run("zero restarts shows dim with no arrow", func(t *testing.T) {
		item := model.Item{
			Name:     "my-pod",
			Restarts: "0",
		}
		result := styledRestartsCell(item, 10, true)
		assert.NotContains(t, result, "↑")
		assert.Contains(t, result, "0")
	})

	t.Run("high restart count with recent restart uses ErrorStyle", func(t *testing.T) {
		item := model.Item{
			Name:          "crash-pod",
			Restarts:      "5",
			LastRestartAt: time.Now().Add(-5 * time.Minute), // 5 minutes ago
		}
		result := styledRestartsCell(item, 10, true)
		assert.Contains(t, result, "↑")
		assert.Contains(t, result, "5")
	})

	t.Run("zero LastRestartAt with restarts does not show arrow", func(t *testing.T) {
		item := model.Item{
			Name:     "my-pod",
			Restarts: "2",
			// LastRestartAt is zero value
		}
		result := styledRestartsCell(item, 10, true)
		assert.NotContains(t, result, "↑")
	})
}

// --- resolveIcon struct-field-direct ---

func TestResolveIconAllModes(t *testing.T) {
	icon := model.Icon{
		Unicode:  "X",
		Simple:   "[Xx]",
		Emoji:    "🅾️",
		NerdFont: "\U000f01a7", // nf-md-cube-outline, single-cell
	}
	tests := []struct {
		mode string
		want string
	}{
		{"unicode", "X"},
		{"simple", "[Xx]"},
		{"emoji", "🅾️"},
		{"nerdfont", "\U000f01a7"},
		{"none", ""},
		{"bogus", "X"}, // unknown mode falls back to Unicode
	}
	for _, tc := range tests {
		t.Run(tc.mode, func(t *testing.T) {
			prev := IconMode
			defer func() { IconMode = prev }()
			IconMode = tc.mode
			if got := resolveIcon(icon); got != tc.want {
				t.Errorf("resolveIcon in %q = %q, want %q", tc.mode, got, tc.want)
			}
		})
	}
}
