package ui

import (
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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

	t.Run("union row matches its cluster-scoped key", func(t *testing.T) {
		origSel := ActiveSelectedItems
		// The app stores union selections under "<cluster>:<ns>/<name>"
		// (model.Item.SelectionKey); the renderer must look them up the
		// same way or the multi-select checkmark never appears in union view.
		ActiveSelectedItems = map[string]bool{"prod:default/my-pod": true}
		defer func() { ActiveSelectedItems = origSel }()
		assert.True(t, isItemSelected(model.Item{Name: "my-pod", Namespace: "default", ClusterName: "prod"}))
		assert.False(t, isItemSelected(model.Item{Name: "my-pod", Namespace: "default", ClusterName: "staging"}),
			"the same name+namespace in a different cluster must not match")
		assert.False(t, isItemSelected(model.Item{Name: "my-pod", Namespace: "default"}),
			"the non-union key form must not match a cluster-scoped selection")
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
		assert.Contains(t, stripANSI(result), "World")
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
			// Trend arrows are cosmetic decorations and must be stripped
			// before parsing; otherwise arrowed rows collapse to 0 and break
			// CPU sorting (issue: "sorting by CPU doesn't work").
			{"↑ 1.3", 1300},
			{"↓ 710m", 710},
			{"↑ 250m", 250},
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

	t.Run("memory strips trend arrows", func(t *testing.T) {
		assert.Equal(t, int64(128*1024*1024), ParseResourceValue("↑ 128Mi", false))
		assert.Equal(t, int64(1024*1024*1024), ParseResourceValue("↓ 1Gi", false))
	})
}

func TestParseResourceValueOK(t *testing.T) {
	tests := []struct {
		name     string
		val      string
		isCPU    bool
		expected int64
		ok       bool
	}{
		{"plain cpu millicores", "710m", true, 710, true},
		{"plain cpu cores", "1.3", true, 1300, true},
		{"cpu with up arrow", "↑ 1.3", true, 1300, true},
		{"cpu with down arrow", "↓ 710m", true, 710, true},
		{"cpu zero is parseable", "0m", true, 0, true},
		{"cpu n/a not parseable", "n/a", true, 0, false},
		{"empty not parseable", "", true, 0, false},
		{"garbage not parseable", "abc", true, 0, false},
		{"memory mebibytes", "128Mi", false, 128 * 1024 * 1024, true},
		{"memory with arrow", "↑ 1Gi", false, 1024 * 1024 * 1024, true},
		{"memory n/a not parseable", "n/a", false, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseResourceValueOK(tt.val, tt.isCPU)
			assert.Equal(t, tt.ok, ok, "ok flag for %q", tt.val)
			if tt.ok {
				assert.Equal(t, tt.expected, got, "value for %q", tt.val)
			}
		})
	}
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

// TruncateStart is Truncate's mirror: it keeps the tail, which is where a
// text input's caret sits, so the user can see what they are typing.
func TestTruncateStart(t *testing.T) {
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
		{"needs truncation", "hello world", 6, "~world"},
		{"maxW 1", "hello", 1, "~"},
		{"empty string", "", 5, ""},
		{"unicode", "héllo", 4, "~llo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateStart(tt.s, tt.maxW)
			assert.Equal(t, tt.expected, got)
			assert.LessOrEqual(t, lipgloss.Width(got), max(tt.maxW, 0), "never exceeds maxW")
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
		"MissingRef",
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

// Emoji mode must not leave a row without an icon. A blank where every
// neighbour has a glyph starts the name a column short, which is how the
// Security sources rendered before they were given emoji of their own (#604).
func TestResolveIconEmojiFallsBackToUnicode(t *testing.T) {
	old := IconMode
	IconMode = "emoji"
	defer func() { IconMode = old }()

	assert.Equal(t, "◉", resolveIcon(model.Icon{Unicode: "◉", Simple: "[He]"}),
		"an icon with no emoji falls back to its unicode glyph")
	assert.Equal(t, "\U0001f9e0", resolveIcon(model.Icon{Unicode: "◉", Emoji: "\U0001f9e0"}),
		"an icon with an emoji still uses it")
	assert.Equal(t, "", resolveIcon(model.Icon{}), "an empty icon stays empty")
}

// The bug in #604: a selector-dependent emoji measured two columns in lipgloss
// and one in a terminal that had not announced grapheme clustering, so the row
// ended a column short and the panel border landed early.
func TestIconCellKeepsOneWidthAcrossGlyphs(t *testing.T) {
	oldMode, oldCore := IconMode, UnicodeCoreActive
	IconMode = "emoji"
	defer func() { IconMode, UnicodeCoreActive = oldMode, oldCore }()

	// Endpoints and Pods: one needs the selector to be two columns, one does not.
	selectorDependent := model.Icon{Unicode: "→", Emoji: "\u27a1\ufe0f"} // right arrow + selector
	intrinsicallyWide := model.Icon{Unicode: "□", Emoji: "\U0001f4e6"}   // package

	t.Run("terminal measures with wcwidth", func(t *testing.T) {
		UnicodeCoreActive = false
		a, b := iconCell(selectorDependent), iconCell(intrinsicallyWide)

		assert.NotContains(t, a, "\ufe0f",
			"the selector must go, or lipgloss counts two columns where the terminal draws one")
		assert.Equal(t, iconCellWidth, lipgloss.Width(a))
		assert.Equal(t, iconCellWidth, lipgloss.Width(b))
		assert.Equal(t, ansi.WcWidth.StringWidth(a), ansi.GraphemeWidth.StringWidth(a),
			"both width methods must agree, whichever one the renderer picked")
	})

	t.Run("terminal announced grapheme clustering", func(t *testing.T) {
		UnicodeCoreActive = true
		a := iconCell(selectorDependent)

		assert.Contains(t, a, "\ufe0f", "the emoji keeps its colour presentation here")
		assert.Equal(t, iconCellWidth, lipgloss.Width(a))
	})
}

// Other icon modes have glyphs of one known width and settled spacing, so the
// emoji cell must not reach them.
func TestIconCellLeavesOtherModesAlone(t *testing.T) {
	oldMode := IconMode
	defer func() { IconMode = oldMode }()

	icon := model.Icon{Unicode: "→", Simple: "[EP]", Emoji: "\u27a1\ufe0f"}
	for mode, want := range map[string]string{"unicode": "→", "simple": "[EP]", "none": ""} {
		IconMode = mode
		assert.Equal(t, want, iconCell(icon), "mode %q", mode)
	}
}
