package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTargetFromRaw_DeploymentUsesPodTemplate(t *testing.T) {
	raw := map[string]any{
		"kind": "Deployment",
		"metadata": map[string]any{
			"namespace": "default",
			"labels":    map[string]any{"app.kubernetes.io/name": "web"},
		},
		"spec": map[string]any{
			"template": map[string]any{
				"metadata": map[string]any{
					"labels": map[string]any{"app": "web", "tier": "frontend"},
				},
				"spec": map[string]any{
					"priorityClassName": "high",
					"nodeSelector":      map[string]any{"disk": "nvme"},
				},
			},
		},
	}

	target := targetFromRaw(raw)

	assert.Equal(t, "default", target.Namespace)
	assert.Equal(t, map[string]string{"app.kubernetes.io/name": "web"}, target.Labels)
	assert.Equal(t, map[string]string{"app": "web", "tier": "frontend"}, target.PodLabels)
	assert.Equal(t, "high", target.PriorityClassName)
	assert.Equal(t, map[string]string{"disk": "nvme"}, target.NodeSelector)
}

func TestTargetFromRaw_PodUsesOwnSpec(t *testing.T) {
	raw := map[string]any{
		"kind": "Pod",
		"metadata": map[string]any{
			"namespace": "default",
			"labels":    map[string]any{"app": "web"},
		},
		"spec": map[string]any{
			"nodeSelector": map[string]any{"disk": "nvme"},
			"tolerations": []any{
				map[string]any{"key": "dedicated", "operator": "Equal", "value": "gpu", "effect": "NoSchedule"},
			},
			"containers": []any{
				map[string]any{
					"name": "app",
					"resources": map[string]any{
						"requests": map[string]any{"cpu": "500m", "memory": float64(1)},
					},
				},
			},
		},
	}

	target := targetFromRaw(raw)

	assert.Equal(t, map[string]string{"app": "web"}, target.PodLabels)
	assert.Equal(t, map[string]string{"disk": "nvme"}, target.NodeSelector)
	if assert.Len(t, target.Tolerations, 1) {
		assert.Equal(t, "dedicated", target.Tolerations[0].Key)
	}
	if assert.Len(t, target.Containers, 1) {
		assert.Equal(t, "500m", target.Containers[0].Requests["cpu"])
		assert.Equal(t, "1", target.Containers[0].Requests["memory"])
	}
}
