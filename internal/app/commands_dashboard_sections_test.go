package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

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
