package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/k8s/demo"
)

// TestApplyTemplateFile_DemoModeAppliesThroughDynamicClient is acceptance
// criterion 5: a demo-mode apply must change the object as read back
// through the dynamic client, never through a kubectl subprocess. PATH is
// cleared so any accidental kubectl resolution would fail loudly instead
// of silently applying against whatever kubectl happens to be installed.
func TestApplyTemplateFile_DemoModeAppliesThroughDynamicClient(t *testing.T) {
	c, err := k8s.NewDemoClient()
	require.NoError(t, err)
	defer c.Shutdown()

	t.Setenv("PATH", t.TempDir())

	m := Model{client: c}

	manifest := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: apply-test-cm\ndata:\n  hello: world\n"
	tmpPath := filepath.Join(t.TempDir(), "apply-test.yaml")
	require.NoError(t, os.WriteFile(tmpPath, []byte(manifest), 0o600))

	cmd := m.applyTemplateFile(tmpPath, c.CurrentContext(), demo.NamespaceDemo)
	require.NotNil(t, cmd)

	msg := cmd()
	result, ok := msg.(actionResultMsg)
	require.True(t, ok, "expected actionResultMsg, got %T", msg)
	require.NoError(t, result.err)

	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	obj, err := c.RawDynamicForContext(c.CurrentContext()).Resource(gvr).Namespace(demo.NamespaceDemo).
		Get(t.Context(), "apply-test-cm", metav1.GetOptions{})
	require.NoError(t, err, "applied ConfigMap must be readable back through the dynamic client")

	value, found, err := unstructured.NestedString(obj.Object, "data", "hello")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "world", value)

	_, statErr := os.Stat(tmpPath)
	assert.True(t, os.IsNotExist(statErr), "temp file should be removed after apply")
}

// TestApplyTemplateFile_DemoModeNeverResolvesKubectl guards the other half
// of acceptance criterion 5: demo mode must never even look for kubectl,
// since a resolved kubectl would shell out to a subprocess whose fake
// backend the UI can never read from.
func TestApplyTemplateFile_DemoModeNeverResolvesKubectl(t *testing.T) {
	c, err := k8s.NewDemoClient()
	require.NoError(t, err)
	defer c.Shutdown()

	// No PATH at all: any lookup for a kubectl binary fails.
	t.Setenv("PATH", "")

	m := Model{client: c}

	manifest := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: apply-test-cm2\ndata:\n  hello: world\n"
	tmpPath := filepath.Join(t.TempDir(), "apply-test2.yaml")
	require.NoError(t, os.WriteFile(tmpPath, []byte(manifest), 0o600))

	cmd := m.applyTemplateFile(tmpPath, c.CurrentContext(), demo.NamespaceDemo)
	require.NotNil(t, cmd)

	msg := cmd()
	result, ok := msg.(actionResultMsg)
	require.True(t, ok, "expected actionResultMsg, got %T", msg)
	assert.NoError(t, result.err, "demo apply must succeed even with an empty PATH")
}
