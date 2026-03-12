package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func TestExpandCustomActionTemplate(t *testing.T) {
	actx := actionContext{
		name:      "my-pod",
		namespace: "default",
		context:   "prod-cluster",
		kind:      "Pod",
		columns: []model.KeyValue{
			{Key: "Node", Value: "node-1"},
			{Key: "IP", Value: "10.0.0.5"},
			{Key: "Priority Class", Value: "high"},
		},
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "basic variable substitution",
			template: "kubectl logs {name} -n {namespace} --context {context}",
			expected: "kubectl logs my-pod -n default --context prod-cluster",
		},
		{
			name:     "kind substitution",
			template: "echo {kind}/{name}",
			expected: "echo Pod/my-pod",
		},
		{
			name:     "column substitution exact key",
			template: "ssh {Node}",
			expected: "ssh node-1",
		},
		{
			name:     "column substitution lowercase key",
			template: "ssh {node}",
			expected: "ssh node-1",
		},
		{
			name:     "column with spaces removed",
			template: "echo {priorityclass}",
			expected: "echo high",
		},
		{
			name:     "IP column",
			template: "curl http://{IP}:8080",
			expected: "curl http://10.0.0.5:8080",
		},
		{
			name:     "multiple variables in one command",
			template: "kubectl logs {name} -n {namespace} --context {context} > /tmp/{name}.log",
			expected: "kubectl logs my-pod -n default --context prod-cluster > /tmp/my-pod.log",
		},
		{
			name:     "no variables",
			template: "echo hello world",
			expected: "echo hello world",
		},
		{
			name:     "unknown variable left as-is",
			template: "echo {unknown}",
			expected: "echo {unknown}",
		},
		{
			name:     "empty template",
			template: "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandCustomActionTemplate(tt.template, actx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExpandCustomActionTemplateNoColumns(t *testing.T) {
	actx := actionContext{
		name:      "my-deploy",
		namespace: "staging",
		context:   "dev-cluster",
		kind:      "Deployment",
		columns:   nil,
	}

	result := expandCustomActionTemplate("kubectl rollout history {kind}/{name} -n {namespace} --context {context}", actx)
	assert.Equal(t, "kubectl rollout history Deployment/my-deploy -n staging --context dev-cluster", result)
}

func TestFindCustomAction(t *testing.T) {
	// Set up custom actions in the global config.
	original := ui.ConfigCustomActions
	defer func() { ui.ConfigCustomActions = original }()

	ui.ConfigCustomActions = map[string][]ui.CustomAction{
		"Pod": {
			{Label: "SSH to Node", Command: "ssh {Node}", Key: "S", Description: "SSH into the pod's node"},
			{Label: "Copy Logs", Command: "kubectl logs {name}", Key: "C", Description: "Copy pod logs"},
		},
		"Deployment": {
			{Label: "Image History", Command: "kubectl rollout history", Key: "H", Description: "Show rollout history"},
		},
	}

	t.Run("find existing action", func(t *testing.T) {
		ca, ok := findCustomAction("Pod", "SSH to Node")
		assert.True(t, ok)
		assert.Equal(t, "SSH to Node", ca.Label)
		assert.Equal(t, "ssh {Node}", ca.Command)
		assert.Equal(t, "S", ca.Key)
	})

	t.Run("find second action", func(t *testing.T) {
		ca, ok := findCustomAction("Pod", "Copy Logs")
		assert.True(t, ok)
		assert.Equal(t, "Copy Logs", ca.Label)
		assert.Equal(t, "C", ca.Key)
	})

	t.Run("find action for different kind", func(t *testing.T) {
		ca, ok := findCustomAction("Deployment", "Image History")
		assert.True(t, ok)
		assert.Equal(t, "Image History", ca.Label)
	})

	t.Run("action not found for kind", func(t *testing.T) {
		_, ok := findCustomAction("Pod", "Nonexistent")
		assert.False(t, ok)
	})

	t.Run("kind not found", func(t *testing.T) {
		_, ok := findCustomAction("Service", "SSH to Node")
		assert.False(t, ok)
	})

	t.Run("nil custom actions map", func(t *testing.T) {
		ui.ConfigCustomActions = nil
		_, ok := findCustomAction("Pod", "SSH to Node")
		assert.False(t, ok)
	})
}
