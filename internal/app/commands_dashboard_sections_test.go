package app

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	fake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

func TestFetchDashboardNodes_ListFailureSetsNodesErr(t *testing.T) {
	dyn := dynfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{
			{Group: "", Version: "v1", Resource: "nodes"}: "NodeList",
		})
	dyn.PrependReactor("list", "nodes", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("dial tcp: refused")
	})
	client := k8s.NewTestClient(fake.NewClientset(), dyn)

	data := fetchDashboardNodes(t.Context(), "test-ctx", client)

	require.Error(t, data.nodesErr)
	assert.Contains(t, data.nodesErr.Error(), "refused")
}

func TestMergeDashboardSection_NodesErrPropagatesAndClears(t *testing.T) {
	acc := dashboardData{}
	mergeDashboardSection(&acc, dashboardData{nodesErr: errors.New("boom")})
	require.Error(t, acc.nodesErr)

	mergeDashboardSection(&acc, dashboardData{nodeItems: []model.Item{{Name: "n1"}}, nodeCount: 1})
	assert.NoError(t, acc.nodesErr, "a nodes success clears the error")
	assert.Equal(t, 1, acc.nodeCount)
}

func TestMergeDashboardSection_OtherSectionKeepsNodesErr(t *testing.T) {
	acc := dashboardData{nodesErr: errors.New("boom")}
	mergeDashboardSection(&acc, dashboardData{nsCount: 3})
	assert.Error(t, acc.nodesErr)
}

func TestCountNotReadyWorkerNodesIgnoresCordon(t *testing.T) {
	nodes := []model.Item{
		{Name: "w1", Status: "Ready"},
		{Name: "w2", Status: "Ready,SchedulingDisabled"},
		{Name: "w3", Status: "NotReady"},
		{Name: "cp1", Status: "NotReady", Columns: []model.KeyValue{{Key: "Role", Value: "control-plane"}}},
	}
	assert.Equal(t, 1, countNotReadyWorkerNodes(nodes))
}

func TestNodeStatusDotIgnoresCordon(t *testing.T) {
	nodes := []model.Item{
		{Name: "w1", Status: "Ready"},
		{Name: "w2", Status: "Ready,SchedulingDisabled"},
		{Name: "w3", Status: "NotReady,SchedulingDisabled"},
	}
	healthy := nodeStatusDot(nodes, "w1")
	assert.Equal(t, healthy, nodeStatusDot(nodes, "w2"), "a cordoned but ready node is not a failure")
	assert.NotEqual(t, healthy, nodeStatusDot(nodes, "w3"))
}
