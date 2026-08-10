package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// TestLaunchKubeshark_RefusesInDemoMode guards against launchKubeshark ever
// resolving k8s.KubectlPath() in demo mode: that call returns this process's
// own executable, so an unguarded launch would kubectl port-forward through
// "this binary" instead of a real kubectl. The kubeshark backend chip is
// normally unreachable in demo mode too (no kubeshark-hub Service is seeded),
// but this guard must hold even if a caller reaches launchKubeshark directly.
func TestLaunchKubeshark_RefusesInDemoMode(t *testing.T) {
	c, err := k8s.NewDemoClient()
	require.NoError(t, err)
	defer c.Shutdown()

	m := baseFinalModel()
	m.client = c
	m.actionCtx.context = c.CurrentContext()

	cmd := m.launchKubeshark(model.Item{Name: "web", Namespace: "demo"})
	require.NotNil(t, cmd)

	msg := cmd()
	result, ok := msg.(kubesharkLaunchedMsg)
	require.True(t, ok, "expected kubesharkLaunchedMsg, got %T", msg)
	require.Error(t, result.err)
	require.True(t, strings.Contains(result.err.Error(), "not available in demo mode"),
		"expected a demo-mode refusal, got: %v", result.err)
}
