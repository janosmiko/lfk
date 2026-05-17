package app

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// captureLogger swaps logger.Logger for a text-handler logger writing to buf
// and returns a restore function. Used by union-routing tests to observe
// per-call context arguments without adding production-only hooks.
func captureLogger(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	orig := logger.Logger
	logger.Logger = slog.New(slog.NewTextHandler(buf, nil))
	t.Cleanup(func() { logger.Logger = orig })
	return buf
}

// TestBulkScaleResources_UnionRoutesPerItemCluster ensures the Scale bulk path
// uses each row's ClusterName as the target context, not the loop-invariant
// actionCtx. Mirrors the per-row pattern already established in
// bulkDeleteResources / bulkRestartResources. The Scale action is currently
// gated out of union mode by isUnionAllowedActionForKind, but the routing
// must be correct in case the allow-list is ever extended (and so the
// function obeys the documented per-row contract on model.Item.ClusterName).
func TestBulkScaleResources_UnionRoutesPerItemCluster(t *testing.T) {
	buf := captureLogger(t)

	// No pre-loaded objects: ScaleResource will fail at GetScale with
	// NotFound (rather than panic on the Scale subresource conversion that
	// happens when an unrelated Deployment object is in the fake clientset).
	// What we care about is the routing log line emitted before the call.
	m := baseModelWithFakeClient()
	m.reqCtx = context.Background()
	m.bulkItems = []model.Item{
		{Name: "blue-app", Namespace: "ns1", ClusterName: "blue"},
		{Name: "green-app", Namespace: "ns1", ClusterName: "green"},
	}
	m.actionCtx = actionContext{
		context:      UnionContextSentinel,
		kind:         "Deployment",
		resourceType: model.ResourceTypeEntry{Resource: "deployments"},
	}

	cmd := m.bulkScaleResources(3)
	require.NotNil(t, cmd)
	msg, ok := cmd().(bulkActionResultMsg)
	require.True(t, ok)
	// Both calls fail (no objects to scale); we assert on routing, not outcome.
	_ = msg

	logs := buf.String()
	assert.Contains(t, logs, "context=blue", "blue row must route to 'blue' cluster")
	assert.Contains(t, logs, "context=green", "green row must route to 'green' cluster")
	assert.NotContains(t, logs, "context=__union__", "must never route to the union sentinel")
}

// TestBatchPatchLabels_UnionRoutesPerItemCluster covers the same per-row
// routing contract for the label/annotation patch bulk action.
func TestBatchPatchLabels_UnionRoutesPerItemCluster(t *testing.T) {
	buf := captureLogger(t)

	m := baseModelWithFakeClient()
	m.reqCtx = context.Background()
	m.bulkItems = []model.Item{
		{Name: "pod-blue", Namespace: "ns1", ClusterName: "blue"},
		{Name: "pod-green", Namespace: "ns1", ClusterName: "green"},
	}
	m.actionCtx = actionContext{
		context:      UnionContextSentinel,
		resourceType: model.ResourceTypeEntry{Resource: "pods", APIVersion: "v1", Namespaced: true},
	}

	cmd := m.batchPatchLabels("env", "prod", false, false)
	require.NotNil(t, cmd)
	msg, ok := cmd().(bulkActionResultMsg)
	require.True(t, ok)
	// The shared fake dyn client doesn't track patches against named objects
	// we never registered, so failures are expected; what matters is the
	// routing log shows per-row context.
	_ = msg

	logs := buf.String()
	assert.Contains(t, logs, "context=blue", "blue row must route to 'blue' cluster")
	assert.Contains(t, logs, "context=green", "green row must route to 'green' cluster")
	assert.NotContains(t, logs, "context=__union__", "must never route to the union sentinel")
}
