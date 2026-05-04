package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// --- appendContainerNodes ---

func TestAppendContainerNodes(t *testing.T) {
	t.Run("adds init and regular containers as children", func(t *testing.T) {
		podNode := &model.ResourceNode{
			Name:      "my-pod",
			Kind:      "Pod",
			Namespace: "default",
		}
		obj := map[string]any{
			"spec": map[string]any{
				"initContainers": []any{
					map[string]any{"name": "init-db"},
					map[string]any{"name": "init-config"},
				},
				"containers": []any{
					map[string]any{"name": "app"},
					map[string]any{"name": "sidecar"},
				},
			},
		}

		appendContainerNodes(podNode, obj)

		require.Len(t, podNode.Children, 4)
		assert.Equal(t, "init-db", podNode.Children[0].Name)
		assert.Equal(t, "Container", podNode.Children[0].Kind)
		assert.Equal(t, "default", podNode.Children[0].Namespace)
		assert.Equal(t, "init-config", podNode.Children[1].Name)
		assert.Equal(t, "app", podNode.Children[2].Name)
		assert.Equal(t, "sidecar", podNode.Children[3].Name)
	})

	t.Run("no spec returns without adding children", func(t *testing.T) {
		podNode := &model.ResourceNode{
			Name:      "my-pod",
			Kind:      "Pod",
			Namespace: "default",
		}
		obj := map[string]any{}

		appendContainerNodes(podNode, obj)

		assert.Nil(t, podNode.Children)
	})

	t.Run("nil spec returns without adding children", func(t *testing.T) {
		podNode := &model.ResourceNode{
			Name:      "my-pod",
			Kind:      "Pod",
			Namespace: "default",
		}
		obj := map[string]any{
			"spec": nil,
		}

		appendContainerNodes(podNode, obj)

		assert.Nil(t, podNode.Children)
	})

	t.Run("only regular containers no init containers", func(t *testing.T) {
		podNode := &model.ResourceNode{
			Name:      "my-pod",
			Kind:      "Pod",
			Namespace: "ns",
		}
		obj := map[string]any{
			"spec": map[string]any{
				"containers": []any{
					map[string]any{"name": "web"},
				},
			},
		}

		appendContainerNodes(podNode, obj)

		require.Len(t, podNode.Children, 1)
		assert.Equal(t, "web", podNode.Children[0].Name)
	})

	t.Run("only init containers no regular containers", func(t *testing.T) {
		podNode := &model.ResourceNode{
			Name:      "my-pod",
			Kind:      "Pod",
			Namespace: "ns",
		}
		obj := map[string]any{
			"spec": map[string]any{
				"initContainers": []any{
					map[string]any{"name": "setup"},
				},
			},
		}

		appendContainerNodes(podNode, obj)

		require.Len(t, podNode.Children, 1)
		assert.Equal(t, "setup", podNode.Children[0].Name)
	})

	t.Run("non-map container entries are skipped", func(t *testing.T) {
		podNode := &model.ResourceNode{
			Name:      "my-pod",
			Kind:      "Pod",
			Namespace: "ns",
		}
		obj := map[string]any{
			"spec": map[string]any{
				"containers": []any{
					"not-a-map",
					42,
					map[string]any{"name": "valid"},
				},
			},
		}

		appendContainerNodes(podNode, obj)

		require.Len(t, podNode.Children, 1)
		assert.Equal(t, "valid", podNode.Children[0].Name)
	})

	t.Run("container with empty name is skipped", func(t *testing.T) {
		podNode := &model.ResourceNode{
			Name:      "my-pod",
			Kind:      "Pod",
			Namespace: "ns",
		}
		obj := map[string]any{
			"spec": map[string]any{
				"containers": []any{
					map[string]any{"name": ""},
					map[string]any{"name": "valid"},
				},
			},
		}

		appendContainerNodes(podNode, obj)

		require.Len(t, podNode.Children, 1)
		assert.Equal(t, "valid", podNode.Children[0].Name)
	})

	t.Run("container without name key is skipped", func(t *testing.T) {
		podNode := &model.ResourceNode{
			Name:      "my-pod",
			Kind:      "Pod",
			Namespace: "ns",
		}
		obj := map[string]any{
			"spec": map[string]any{
				"containers": []any{
					map[string]any{"image": "nginx"},
					map[string]any{"name": "named"},
				},
			},
		}

		appendContainerNodes(podNode, obj)

		require.Len(t, podNode.Children, 1)
		assert.Equal(t, "named", podNode.Children[0].Name)
	})

	t.Run("children inherit parent namespace", func(t *testing.T) {
		podNode := &model.ResourceNode{
			Name:      "my-pod",
			Kind:      "Pod",
			Namespace: "custom-ns",
		}
		obj := map[string]any{
			"spec": map[string]any{
				"containers": []any{
					map[string]any{"name": "app"},
				},
			},
		}

		appendContainerNodes(podNode, obj)

		require.Len(t, podNode.Children, 1)
		assert.Equal(t, "custom-ns", podNode.Children[0].Namespace)
	})

	t.Run("containers list that is not a slice is ignored", func(t *testing.T) {
		podNode := &model.ResourceNode{
			Name:      "my-pod",
			Kind:      "Pod",
			Namespace: "ns",
		}
		obj := map[string]any{
			"spec": map[string]any{
				"containers": "not-a-slice",
			},
		}

		appendContainerNodes(podNode, obj)

		assert.Nil(t, podNode.Children)
	})

	t.Run("status block populates Container Status from running+ready", func(t *testing.T) {
		podNode := &model.ResourceNode{Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"containers": []any{map[string]any{"name": "app"}},
			},
			"status": map[string]any{
				"containerStatuses": []any{
					map[string]any{
						"name":  "app",
						"ready": true,
						"state": map[string]any{"running": map[string]any{"startedAt": "2024-01-01T00:00:00Z"}},
					},
				},
			},
		}

		appendContainerNodes(podNode, obj)

		require.Len(t, podNode.Children, 1)
		assert.Equal(t, "Running", podNode.Children[0].Status)
	})

	t.Run("running but not ready becomes NotReady", func(t *testing.T) {
		podNode := &model.ResourceNode{Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"containers": []any{map[string]any{"name": "app"}},
			},
			"status": map[string]any{
				"containerStatuses": []any{
					map[string]any{
						"name":  "app",
						"ready": false,
						"state": map[string]any{"running": map[string]any{}},
					},
				},
			},
		}

		appendContainerNodes(podNode, obj)

		require.Len(t, podNode.Children, 1)
		assert.Equal(t, "NotReady", podNode.Children[0].Status)
	})

	t.Run("waiting state becomes Waiting", func(t *testing.T) {
		podNode := &model.ResourceNode{Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"containers": []any{map[string]any{"name": "app"}},
			},
			"status": map[string]any{
				"containerStatuses": []any{
					map[string]any{
						"name":  "app",
						"state": map[string]any{"waiting": map[string]any{"reason": "ContainerCreating"}},
					},
				},
			},
		}

		appendContainerNodes(podNode, obj)

		require.Len(t, podNode.Children, 1)
		assert.Equal(t, "Waiting", podNode.Children[0].Status)
	})

	t.Run("terminated with Completed reason becomes Completed", func(t *testing.T) {
		podNode := &model.ResourceNode{Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"initContainers": []any{map[string]any{"name": "init"}},
			},
			"status": map[string]any{
				"initContainerStatuses": []any{
					map[string]any{
						"name":  "init",
						"state": map[string]any{"terminated": map[string]any{"reason": "Completed"}},
					},
				},
			},
		}

		appendContainerNodes(podNode, obj)

		require.Len(t, podNode.Children, 1)
		assert.Equal(t, "Completed", podNode.Children[0].Status)
	})

	t.Run("terminated with non-Completed reason becomes Terminated", func(t *testing.T) {
		podNode := &model.ResourceNode{Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"containers": []any{map[string]any{"name": "app"}},
			},
			"status": map[string]any{
				"containerStatuses": []any{
					map[string]any{
						"name":  "app",
						"state": map[string]any{"terminated": map[string]any{"reason": "Error"}},
					},
				},
			},
		}

		appendContainerNodes(podNode, obj)

		require.Len(t, podNode.Children, 1)
		assert.Equal(t, "Terminated", podNode.Children[0].Status)
	})

	t.Run("missing status block leaves Status empty", func(t *testing.T) {
		podNode := &model.ResourceNode{Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"containers": []any{map[string]any{"name": "app"}},
			},
		}

		appendContainerNodes(podNode, obj)

		require.Len(t, podNode.Children, 1)
		assert.Equal(t, "", podNode.Children[0].Status)
	})

	t.Run("status block present but container missing defaults to Waiting", func(t *testing.T) {
		// Mirrors containerStatusFromPod's typed default — a pod that has a
		// status block but the kubelet hasn't reported this container yet.
		podNode := &model.ResourceNode{Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"containers": []any{map[string]any{"name": "app"}},
			},
			"status": map[string]any{
				"containerStatuses": []any{}, // empty list
			},
		}

		appendContainerNodes(podNode, obj)

		require.Len(t, podNode.Children, 1)
		assert.Equal(t, "Waiting", podNode.Children[0].Status)
	})

	t.Run("preserves existing children", func(t *testing.T) {
		existing := &model.ResourceNode{Name: "existing", Kind: "Volume"}
		podNode := &model.ResourceNode{
			Name:      "my-pod",
			Kind:      "Pod",
			Namespace: "ns",
			Children:  []*model.ResourceNode{existing},
		}
		obj := map[string]any{
			"spec": map[string]any{
				"containers": []any{
					map[string]any{"name": "app"},
				},
			},
		}

		appendContainerNodes(podNode, obj)

		require.Len(t, podNode.Children, 2)
		assert.Equal(t, "existing", podNode.Children[0].Name)
		assert.Equal(t, "app", podNode.Children[1].Name)
	})
}
