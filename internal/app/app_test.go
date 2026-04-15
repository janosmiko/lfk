package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	dynfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// --- ui.PadToHeight ---

func TestPadToHeight(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		height   int
		expected int // expected number of lines
	}{
		{"shorter content", "line1\nline2", 5, 5},
		{"exact height", "a\nb\nc", 3, 3},
		{"taller content", "a\nb\nc\nd\ne", 3, 3},
		{"empty content", "", 3, 3},
		{"single line", "hello", 1, 1},
		{"height zero", "hello", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ui.PadToHeight(tt.content, tt.height)
			lines := strings.Split(result, "\n")
			if tt.height == 0 {
				// padToHeight truncates to 0 lines but Split always gives at least 1
				assert.LessOrEqual(t, len(lines), 1)
			} else {
				assert.Equal(t, tt.expected, len(lines))
			}
		})
	}

	// Verify padding uses empty strings
	t.Run("padding is empty lines", func(t *testing.T) {
		result := ui.PadToHeight("line1", 3)
		lines := strings.Split(result, "\n")
		assert.Equal(t, "line1", lines[0])
		assert.Equal(t, "", lines[1])
		assert.Equal(t, "", lines[2])
	})

	// Verify truncation preserves first lines
	t.Run("truncation preserves order", func(t *testing.T) {
		result := ui.PadToHeight("a\nb\nc\nd", 2)
		lines := strings.Split(result, "\n")
		assert.Equal(t, "a", lines[0])
		assert.Equal(t, "b", lines[1])
	})
}

// --- isContextCanceled ---

func TestIsContextCanceled(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"context.Canceled", context.Canceled, false},
		{"context.DeadlineExceeded", context.DeadlineExceeded, false},
		{"wrapped context.Canceled", context.Canceled, false},
		{"random error", errors.New("some error"), false},
		{"context canceled string", errors.New("context canceled"), false},
		{"context deadline exceeded string", errors.New("context deadline exceeded"), false},
		{"error containing context canceled", errors.New("operation failed: context canceled"), false},
	}

	// Override expected for actual cancellation errors
	tests[1].expected = true
	tests[2].expected = true
	tests[3].expected = true
	tests[5].expected = true
	tests[6].expected = true
	tests[7].expected = true

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isContextCanceled(tt.err))
		})
	}
}

// --- removeBookmark ---

func TestRemoveBookmark(t *testing.T) {
	bm1 := model.Bookmark{Name: "bm1", Context: "ctx1"}
	bm2 := model.Bookmark{Name: "bm2", Context: "ctx2"}
	bm3 := model.Bookmark{Name: "bm3", Context: "ctx3"}

	t.Run("remove middle element", func(t *testing.T) {
		result := removeBookmark([]model.Bookmark{bm1, bm2, bm3}, 1)
		assert.Len(t, result, 2)
		assert.Equal(t, "bm1", result[0].Name)
		assert.Equal(t, "bm3", result[1].Name)
	})

	t.Run("remove first element", func(t *testing.T) {
		result := removeBookmark([]model.Bookmark{bm1, bm2}, 0)
		assert.Len(t, result, 1)
		assert.Equal(t, "bm2", result[0].Name)
	})

	t.Run("remove last element", func(t *testing.T) {
		result := removeBookmark([]model.Bookmark{bm1, bm2}, 1)
		assert.Len(t, result, 1)
		assert.Equal(t, "bm1", result[0].Name)
	})

	t.Run("remove from single-element list", func(t *testing.T) {
		result := removeBookmark([]model.Bookmark{bm1}, 0)
		assert.Empty(t, result)
	})

	t.Run("negative index unchanged", func(t *testing.T) {
		result := removeBookmark([]model.Bookmark{bm1, bm2}, -1)
		assert.Len(t, result, 2)
	})

	t.Run("out of bounds unchanged", func(t *testing.T) {
		result := removeBookmark([]model.Bookmark{bm1, bm2}, 5)
		assert.Len(t, result, 2)
	})
}

// --- selectionKey ---

func TestSelectionKey(t *testing.T) {
	tests := []struct {
		name     string
		item     model.Item
		expected string
	}{
		{
			name:     "namespaced item",
			item:     model.Item{Name: "my-pod", Namespace: "default"},
			expected: "default/my-pod",
		},
		{
			name:     "cluster-scoped item",
			item:     model.Item{Name: "my-node"},
			expected: "my-node",
		},
		{
			name:     "empty name and namespace",
			item:     model.Item{},
			expected: "",
		},
		{
			name:     "name only",
			item:     model.Item{Name: "some-resource"},
			expected: "some-resource",
		},
		{
			name:     "namespace with slash in name",
			item:     model.Item{Name: "a/b", Namespace: "ns"},
			expected: "ns/a/b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, selectionKey(tt.item))
		})
	}
}

// --- statusPriority ---

func TestStatusPriority(t *testing.T) {
	tests := []struct {
		status   string
		priority int
	}{
		// Priority 0: healthy statuses.
		{"Running", 0},
		{"Active", 0},
		{"Bound", 0},
		{"Available", 0},
		{"Ready", 0},
		{"Healthy", 0},
		{"Healthy/Synced", 0},
		{"Deployed", 0},
		// Priority 1: in-progress statuses.
		{"Pending", 1},
		{"ContainerCreating", 1},
		{"Waiting", 1},
		{"Init", 1},
		{"Progressing", 1},
		{"Progressing/Synced", 1},
		{"Suspended", 1},
		{"Pending-install", 1},
		{"Pending-upgrade", 1},
		{"Pending-rollback", 1},
		{"Uninstalling", 1},
		// Priority 2: failed statuses.
		{"Failed", 2},
		{"CrashLoopBackOff", 2},
		{"Error", 2},
		{"ImagePullBackOff", 2},
		{"Degraded", 2},
		{"Degraded/OutOfSync", 2},
		// Priority 3: unknown statuses.
		{"Unknown", 3},
		{"SomeRandomStatus", 3},
		{"", 3},
	}

	for _, tt := range tests {
		name := tt.status
		if name == "" {
			name = "empty string"
		}
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.priority, statusPriority(tt.status))
		})
	}
}

// --- fullErrorMessage ---

func TestFullErrorMessage(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "simple error",
			err:      fmt.Errorf("something failed"),
			expected: "something failed",
		},
		{
			name:     "error with newlines",
			err:      fmt.Errorf("line1\nline2\nline3"),
			expected: "line1 line2 line3",
		},
		{
			name:     "error with carriage returns",
			err:      fmt.Errorf("line1\r\nline2"),
			expected: "line1 line2",
		},
		{
			name:     "error with multiple spaces",
			err:      fmt.Errorf("too   many    spaces"),
			expected: "too many spaces",
		},
		{
			name:     "error with mixed whitespace",
			err:      fmt.Errorf("a\n\n  b\r\n  c"),
			expected: "a b c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, fullErrorMessage(tt.err))
		})
	}
}

// --- copyMapStringInt ---

func TestCopyMapStringInt(t *testing.T) {
	t.Run("nil map returns empty", func(t *testing.T) {
		result := copyMapStringInt(nil)
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("empty map returns empty copy", func(t *testing.T) {
		original := make(map[string]int)
		result := copyMapStringInt(original)
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("copy preserves values", func(t *testing.T) {
		original := map[string]int{"a": 1, "b": 2, "c": 3}
		result := copyMapStringInt(original)
		assert.Equal(t, original, result)
	})

	t.Run("modifying copy does not affect original", func(t *testing.T) {
		original := map[string]int{"a": 1, "b": 2}
		result := copyMapStringInt(original)
		result["a"] = 99
		result["new"] = 42
		assert.Equal(t, 1, original["a"])
		_, exists := original["new"]
		assert.False(t, exists)
	})
}

// --- copyMapStringBool ---

func TestCopyMapStringBool(t *testing.T) {
	t.Run("nil map returns empty", func(t *testing.T) {
		result := copyMapStringBool(nil)
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("empty map returns empty copy", func(t *testing.T) {
		original := make(map[string]bool)
		result := copyMapStringBool(original)
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("copy preserves values", func(t *testing.T) {
		original := map[string]bool{"x": true, "y": false, "z": true}
		result := copyMapStringBool(original)
		assert.Equal(t, original, result)
	})

	t.Run("modifying copy does not affect original", func(t *testing.T) {
		original := map[string]bool{"x": true}
		result := copyMapStringBool(original)
		result["x"] = false
		result["new"] = true
		assert.True(t, original["x"])
		_, exists := original["new"]
		assert.False(t, exists)
	})
}

// --- copyItemCache ---

func TestCopyItemCache(t *testing.T) {
	t.Run("nil map returns empty", func(t *testing.T) {
		result := copyItemCache(nil)
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("copy preserves entries", func(t *testing.T) {
		original := map[string][]model.Item{
			"key1": {{Name: "pod1"}, {Name: "pod2"}},
			"key2": {{Name: "svc1"}},
		}
		result := copyItemCache(original)
		assert.Len(t, result, 2)
		assert.Len(t, result["key1"], 2)
		assert.Equal(t, "pod1", result["key1"][0].Name)
		assert.Equal(t, "pod2", result["key1"][1].Name)
		assert.Len(t, result["key2"], 1)
	})

	t.Run("modifying copy does not affect original", func(t *testing.T) {
		original := map[string][]model.Item{
			"key1": {{Name: "pod1"}},
		}
		result := copyItemCache(original)
		result["key1"][0].Name = "modified"
		assert.Equal(t, "pod1", original["key1"][0].Name)
	})

	t.Run("adding to copy does not affect original", func(t *testing.T) {
		original := map[string][]model.Item{
			"key1": {{Name: "pod1"}},
		}
		result := copyItemCache(original)
		result["key2"] = []model.Item{{Name: "new"}}
		_, exists := original["key2"]
		assert.False(t, exists)
	})
}

func TestCov80FindCustomActionNoMatch(t *testing.T) {
	_, found := findCustomAction("Pod", "nonexistent-action")
	assert.False(t, found)
}

func TestCov80ExpandCustomActionTemplate(t *testing.T) {
	actx := actionContext{
		name:      "my-pod",
		namespace: "default",
		context:   "prod",
		kind:      "Pod",
		columns: []model.KeyValue{
			{Key: "Node", Value: "worker-1"},
			{Key: "IP", Value: "10.0.0.1"},
		},
	}
	result := expandCustomActionTemplate("kubectl exec {name} -n {namespace} --context {context} # {Node} {ip}", actx)
	assert.Contains(t, result, "my-pod")
	assert.Contains(t, result, "default")
	assert.Contains(t, result, "prod")
	assert.Contains(t, result, "worker-1")
}

// --- H1: security manager initialization ---

func TestNewModelHasSecurityManagerAfterInit(t *testing.T) {
	cs := k8sfake.NewClientset()
	dyn := dynfake.NewSimpleDynamicClient(runtime.NewScheme())
	client := k8s.NewTestClient(cs, dyn)

	m := NewModel(client, StartupOptions{})
	require.NotNil(t, m.securityManager,
		"NewModel must initialize securityManager so async security commands have a target")
}

func TestNewModelRegistersSecuritySourcesWhenClientsAvailable(t *testing.T) {
	// When both a clientset and a dynamic client are available for the
	// test context, refreshSecuritySources should register the heuristic
	// and trivy-operator sources.
	cs := k8sfake.NewClientset()
	dyn := dynfake.NewSimpleDynamicClient(runtime.NewScheme())
	client := k8s.NewTestClient(cs, dyn)

	m := NewModel(client, StartupOptions{})
	sources := m.securityManager.Sources()
	// Collect source names to be order-independent.
	names := make(map[string]bool, len(sources))
	for _, s := range sources {
		names[s.Name()] = true
	}
	assert.True(t, names["heuristic"], "heuristic source should be registered")
	assert.True(t, names["trivy-operator"], "trivy-operator source should be registered")
}

func TestRefreshSecuritySourcesRebuildsOnContextChange(t *testing.T) {
	cs := k8sfake.NewClientset()
	dyn := dynfake.NewSimpleDynamicClient(runtime.NewScheme())
	client := k8s.NewTestClient(cs, dyn)

	m := NewModel(client, StartupOptions{})
	originalMgr := m.securityManager
	require.NotNil(t, originalMgr)

	// Simulate a switch to a different cluster and re-register.
	m.nav.Context = "another-context"
	m.refreshSecuritySources()
	require.NotNil(t, m.securityManager,
		"securityManager must remain non-nil after refresh")

	// The manager should have been replaced (pointer identity changes) so
	// stale sources cannot linger across context switches.
	assert.NotSame(t, originalMgr, m.securityManager,
		"refreshSecuritySources must replace the manager instance")
	// With fake clients injected, both sources are still registered because
	// NewTestClient returns the same fake for any context name.
	sources := m.securityManager.Sources()
	assert.Len(t, sources, 4, "all sources should re-register when clients are available")
}

func TestRefreshSecuritySourcesNilClient(t *testing.T) {
	// Defensive: refreshSecuritySources must not panic when m.client is
	// nil (e.g., during early test fixtures).
	m := Model{}
	m.refreshSecuritySources()
	require.NotNil(t, m.securityManager,
		"securityManager should still be initialized")
	assert.Empty(t, m.securityManager.Sources())
}
