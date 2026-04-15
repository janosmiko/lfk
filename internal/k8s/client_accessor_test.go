package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- CurrentContext ---

func TestCurrentContext(t *testing.T) {
	c := newTestClient(t)

	got := c.CurrentContext()
	assert.Equal(t, "test-context", got)
}

// --- DefaultNamespace ---

func TestDefaultNamespace(t *testing.T) {
	t.Run("returns configured namespace for known context", func(t *testing.T) {
		c := newTestClient(t)
		// The test kubeconfig sets namespace "default" for test-context.
		got := c.DefaultNamespace("test-context")
		assert.Equal(t, "default", got)
	})

	t.Run("returns default for unknown context", func(t *testing.T) {
		c := newTestClient(t)
		got := c.DefaultNamespace("nonexistent-context")
		assert.Equal(t, "default", got)
	})

	t.Run("returns default for empty context name", func(t *testing.T) {
		c := newTestClient(t)
		got := c.DefaultNamespace("")
		assert.Equal(t, "default", got)
	})
}

// --- KubeconfigPaths ---

func TestKubeconfigPaths(t *testing.T) {
	c := newTestClient(t)

	paths := c.KubeconfigPaths()
	// The test client uses a temp file as its kubeconfig.
	assert.NotEmpty(t, paths)
	assert.Contains(t, paths, "kubeconfig")
}

// --- GetContexts ---

func TestGetContexts(t *testing.T) {
	c := newTestClient(t)

	items, err := c.GetContexts()
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "test-context", items[0].Name)
	assert.Equal(t, "current", items[0].Status)
}

// --- ContextExists ---

func TestContextExists(t *testing.T) {
	tests := []struct {
		name        string
		contextName string
		want        bool
	}{
		{
			name:        "known context returns true",
			contextName: "test-context",
			want:        true,
		},
		{
			name:        "unknown context returns false",
			contextName: "nonexistent-context",
			want:        false,
		},
		{
			name:        "empty string returns false",
			contextName: "",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestClient(t)
			got := c.ContextExists(tt.contextName)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- dynamicForContext ---

func TestDynamicForContext_InvalidContext(t *testing.T) {
	c := newTestClient(t)
	_, err := c.dynamicForContext("nonexistent-context")
	assert.Error(t, err)
}

func TestDynamicForContext_ValidContext(t *testing.T) {
	c := newTestClient(t)
	dynClient, err := c.dynamicForContext("test-context")
	require.NoError(t, err)
	assert.NotNil(t, dynClient)
}
