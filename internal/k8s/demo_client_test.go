package k8s

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s/demo"
	"github.com/janosmiko/lfk/internal/model"
)

// TestNewClient_NeverSetsInjectedFields guards acceptance criterion 6: the
// demo backend must never be selected without the --demo flag. NewClient
// (the real kubeconfig path) must never populate the injected* fields that
// *ForContext helpers check before building a real client.
func TestNewClient_NeverSetsInjectedFields(t *testing.T) {
	kubecfg := `
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://127.0.0.1:1
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    namespace: default
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user: {}
`
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "kubeconfig")
	require.NoError(t, os.WriteFile(cfgPath, []byte(kubecfg), 0o600))

	c, err := NewClient(cfgPath, nil, true)
	require.NoError(t, err)

	assert.Nil(t, c.injectedClientset)
	assert.Nil(t, c.injectedDynClient)
	assert.Nil(t, c.injectedMetaClient)
	assert.False(t, c.IsDemo())
}

// TestNewDemoClient_ListsPodsAndDeployments exercises GetResources against
// the demo-backed client for the two resource types acceptance criteria
// call out explicitly.
func TestNewDemoClient_ListsPodsAndDeployments(t *testing.T) {
	c, err := NewDemoClient()
	require.NoError(t, err)
	require.True(t, c.IsDemo())

	ctx := t.Context()

	podsRT := model.ResourceTypeEntry{APIGroup: "", APIVersion: "v1", Resource: "pods", Kind: "Pod", Namespaced: true}
	pods, err := c.GetResources(ctx, c.CurrentContext(), demo.NamespaceDemo, podsRT)
	require.NoError(t, err)
	assert.NotEmpty(t, pods)

	deploymentsRT := model.ResourceTypeEntry{APIGroup: "apps", APIVersion: "v1", Resource: "deployments", Kind: "Deployment", Namespaced: true}
	deployments, err := c.GetResources(ctx, c.CurrentContext(), demo.NamespaceDemo, deploymentsRT)
	require.NoError(t, err)
	assert.NotEmpty(t, deployments)
}

// TestNewDemoClient_NoKubeconfigNeeded is acceptance criterion 1: demo
// startup must work with no kubeconfig on disk at all. Pointing HOME and
// KUBECONFIG at an empty temp dir simulates a machine with none configured.
func TestNewDemoClient_NoKubeconfigNeeded(t *testing.T) {
	emptyDir := t.TempDir()
	t.Setenv("HOME", emptyDir)
	t.Setenv("KUBECONFIG", filepath.Join(emptyDir, "does-not-exist"))

	c, err := NewDemoClient()
	require.NoError(t, err)
	assert.NotNil(t, c)
	assert.True(t, c.IsDemo())
}

// TestNewDemoClient_TickerLifetime guards the demo ticker's goroutine
// lifetime: NewDemoClient must start it, and Client.Shutdown must stop it,
// so a demo session never leaks a background goroutine past the client
// that owns it.
func TestNewDemoClient_TickerLifetime(t *testing.T) {
	baseline := runtime.NumGoroutine()

	c, err := NewDemoClient()
	require.NoError(t, err)
	require.NotNil(t, c.demoTicker, "NewDemoClient must start a demo ticker")

	require.Eventually(t, func() bool {
		return runtime.NumGoroutine() > baseline
	}, time.Second, 10*time.Millisecond, "expected a new goroutine after NewDemoClient")

	c.Shutdown()

	require.Eventually(t, func() bool {
		return runtime.NumGoroutine() <= baseline+1 // +1 tolerance for runtime jitter
	}, time.Second, 10*time.Millisecond, "Shutdown did not drain the demo ticker goroutine")
}
