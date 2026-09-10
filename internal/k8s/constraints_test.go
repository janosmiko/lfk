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

func TestTargetFromRaw_CronJobUsesJobTemplatePodTemplate(t *testing.T) {
	raw := map[string]any{
		"kind": "CronJob",
		"metadata": map[string]any{
			"namespace": "default",
		},
		"spec": map[string]any{
			"jobTemplate": map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"metadata": map[string]any{
							"labels": map[string]any{"app": "nightly"},
						},
						"spec": map[string]any{
							"priorityClassName": "batch",
							"nodeSelector":      map[string]any{"disk": "hdd"},
							"containers": []any{
								map[string]any{
									"name":      "job",
									"resources": map[string]any{"requests": map[string]any{"cpu": "1"}},
								},
							},
						},
					},
				},
			},
		},
	}

	target := targetFromRaw(raw)

	assert.Equal(t, "default", target.Namespace)
	assert.Equal(t, map[string]string{"app": "nightly"}, target.PodLabels)
	assert.Equal(t, "batch", target.PriorityClassName)
	assert.Equal(t, map[string]string{"disk": "hdd"}, target.NodeSelector)
	if assert.Len(t, target.Containers, 1) {
		assert.Equal(t, "job", target.Containers[0].Name)
	}
}

func TestTargetFromRaw_KeepsRequiredAffinityMatchFields(t *testing.T) {
	raw := map[string]any{
		"kind": "Pod",
		"spec": map[string]any{
			"affinity": map[string]any{
				"nodeAffinity": map[string]any{
					"requiredDuringSchedulingIgnoredDuringExecution": map[string]any{
						"nodeSelectorTerms": []any{
							map[string]any{
								"matchFields": []any{
									map[string]any{
										"key": "metadata.name", "operator": "In", "values": []any{"node-1"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	target := targetFromRaw(raw)

	if assert.Len(t, target.Affinity, 1) {
		assert.Empty(t, target.Affinity[0].MatchExpressions)
		assert.Equal(t, []NodeSelectorRequirement{
			{Key: "metadata.name", Operator: "In", Values: []string{"node-1"}},
		}, target.Affinity[0].MatchFields)
	}
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

func TestTargetFromRaw_ReadsInitContainersSeparately(t *testing.T) {
	raw := map[string]any{
		"kind": "Pod",
		"spec": map[string]any{
			"initContainers": []any{
				map[string]any{
					"name":      "init",
					"resources": map[string]any{"requests": map[string]any{"cpu": "2"}},
				},
			},
			"containers": []any{
				map[string]any{
					"name":      "app",
					"resources": map[string]any{"requests": map[string]any{"cpu": "500m"}},
				},
			},
		},
	}

	target := targetFromRaw(raw)

	if assert.Len(t, target.InitContainers, 1) {
		assert.Equal(t, "init", target.InitContainers[0].Name)
		assert.Equal(t, "2", target.InitContainers[0].Requests["cpu"])
	}
	if assert.Len(t, target.Containers, 1) {
		assert.Equal(t, "app", target.Containers[0].Name)
	}
}
