package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// kindNames flattens the children's (Kind, Name) pairs in order — the helper
// many of the table-driven cases below want.
func kindNames(children []*model.ResourceNode) [][2]string {
	out := make([][2]string, 0, len(children))
	for _, ch := range children {
		out = append(out, [2]string{ch.Kind, ch.Name})
	}
	return out
}

func TestAppendPodRefs_ServiceAccount(t *testing.T) {
	t.Run("default SA when serviceAccountName empty", func(t *testing.T) {
		node := &model.ResourceNode{Kind: "Pod", Namespace: "ns"}
		obj := map[string]any{"spec": map[string]any{}}

		appendPodRefs(node, obj, "ns", nil)

		require.Len(t, node.Children, 1)
		assert.Equal(t, "ServiceAccount", node.Children[0].Kind)
		assert.Equal(t, "default", node.Children[0].Name)
		assert.Equal(t, "refs", node.Children[0].Group)
		assert.Equal(t, "ns", node.Children[0].Namespace)
	})

	t.Run("custom serviceAccountName is used", func(t *testing.T) {
		node := &model.ResourceNode{Kind: "Pod", Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{"serviceAccountName": "my-sa"},
		}

		appendPodRefs(node, obj, "ns", nil)

		require.Len(t, node.Children, 1)
		assert.Equal(t, "my-sa", node.Children[0].Name)
	})

	t.Run("automountServiceAccountToken=false suppresses SA", func(t *testing.T) {
		node := &model.ResourceNode{Kind: "Pod", Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"serviceAccountName":           "my-sa",
				"automountServiceAccountToken": false,
			},
		}

		appendPodRefs(node, obj, "ns", nil)

		assert.Empty(t, node.Children)
	})

	t.Run("automountServiceAccountToken=true keeps SA", func(t *testing.T) {
		node := &model.ResourceNode{Kind: "Pod", Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"serviceAccountName":           "my-sa",
				"automountServiceAccountToken": true,
			},
		}

		appendPodRefs(node, obj, "ns", nil)

		require.Len(t, node.Children, 1)
		assert.Equal(t, "my-sa", node.Children[0].Name)
	})
}

func TestAppendPodRefs_EnvAndEnvFrom(t *testing.T) {
	t.Run("env.valueFrom.secretKeyRef and configMapKeyRef", func(t *testing.T) {
		node := &model.ResourceNode{Kind: "Pod", Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"automountServiceAccountToken": false,
				"containers": []any{
					map[string]any{
						"name": "app",
						"env": []any{
							map[string]any{
								"name": "DB_PASS",
								"valueFrom": map[string]any{
									"secretKeyRef": map[string]any{"name": "db-secret", "key": "pass"},
								},
							},
							map[string]any{
								"name": "CFG",
								"valueFrom": map[string]any{
									"configMapKeyRef": map[string]any{"name": "app-cm", "key": "cfg"},
								},
							},
						},
					},
				},
			},
		}

		appendPodRefs(node, obj, "ns", nil)

		assert.ElementsMatch(t, [][2]string{
			{"ConfigMap", "app-cm"},
			{"Secret", "db-secret"},
		}, kindNames(node.Children))
	})

	t.Run("envFrom.secretRef and configMapRef", func(t *testing.T) {
		node := &model.ResourceNode{Kind: "Pod", Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"automountServiceAccountToken": false,
				"containers": []any{
					map[string]any{
						"name": "app",
						"envFrom": []any{
							map[string]any{"secretRef": map[string]any{"name": "envsec"}},
							map[string]any{"configMapRef": map[string]any{"name": "envcm"}},
						},
					},
				},
			},
		}

		appendPodRefs(node, obj, "ns", nil)

		assert.ElementsMatch(t, [][2]string{
			{"ConfigMap", "envcm"},
			{"Secret", "envsec"},
		}, kindNames(node.Children))
	})

	t.Run("ephemeralContainers env refs are also collected", func(t *testing.T) {
		node := &model.ResourceNode{Kind: "Pod", Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"automountServiceAccountToken": false,
				"ephemeralContainers": []any{
					map[string]any{
						"name": "debugger",
						"envFrom": []any{
							map[string]any{"configMapRef": map[string]any{"name": "debug-cm"}},
						},
					},
				},
			},
		}

		appendPodRefs(node, obj, "ns", nil)

		require.Len(t, node.Children, 1)
		assert.Equal(t, "ConfigMap", node.Children[0].Kind)
		assert.Equal(t, "debug-cm", node.Children[0].Name)
	})

	t.Run("initContainers env refs are also collected", func(t *testing.T) {
		node := &model.ResourceNode{Kind: "Pod", Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"automountServiceAccountToken": false,
				"initContainers": []any{
					map[string]any{
						"name": "init",
						"env": []any{
							map[string]any{
								"valueFrom": map[string]any{
									"secretKeyRef": map[string]any{"name": "init-secret"},
								},
							},
						},
					},
				},
			},
		}

		appendPodRefs(node, obj, "ns", nil)

		require.Len(t, node.Children, 1)
		assert.Equal(t, "Secret", node.Children[0].Kind)
		assert.Equal(t, "init-secret", node.Children[0].Name)
	})

	t.Run("env without valueFrom is skipped", func(t *testing.T) {
		node := &model.ResourceNode{Kind: "Pod", Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"automountServiceAccountToken": false,
				"containers": []any{
					map[string]any{
						"name": "app",
						"env": []any{
							map[string]any{"name": "FOO", "value": "bar"},
						},
					},
				},
			},
		}

		appendPodRefs(node, obj, "ns", nil)

		assert.Empty(t, node.Children)
	})
}

func TestAppendPodRefs_Volumes(t *testing.T) {
	t.Run("secret volume uses secretName", func(t *testing.T) {
		node := &model.ResourceNode{Kind: "Pod", Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"automountServiceAccountToken": false,
				"volumes": []any{
					map[string]any{
						"name":   "secrets",
						"secret": map[string]any{"secretName": "vol-secret"},
					},
				},
			},
		}

		appendPodRefs(node, obj, "ns", nil)

		require.Len(t, node.Children, 1)
		assert.Equal(t, "Secret", node.Children[0].Kind)
		assert.Equal(t, "vol-secret", node.Children[0].Name)
	})

	t.Run("configMap volume uses name", func(t *testing.T) {
		node := &model.ResourceNode{Kind: "Pod", Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"automountServiceAccountToken": false,
				"volumes": []any{
					map[string]any{
						"name":      "cfg",
						"configMap": map[string]any{"name": "vol-cm"},
					},
				},
			},
		}

		appendPodRefs(node, obj, "ns", nil)

		require.Len(t, node.Children, 1)
		assert.Equal(t, "ConfigMap", node.Children[0].Kind)
		assert.Equal(t, "vol-cm", node.Children[0].Name)
	})

	t.Run("PVC volume uses claimName", func(t *testing.T) {
		node := &model.ResourceNode{Kind: "Pod", Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"automountServiceAccountToken": false,
				"volumes": []any{
					map[string]any{
						"name":                  "data",
						"persistentVolumeClaim": map[string]any{"claimName": "data-pvc"},
					},
				},
			},
		}

		appendPodRefs(node, obj, "ns", nil)

		require.Len(t, node.Children, 1)
		assert.Equal(t, "PersistentVolumeClaim", node.Children[0].Kind)
		assert.Equal(t, "data-pvc", node.Children[0].Name)
	})

	t.Run("projected volume sources collect secret and configMap", func(t *testing.T) {
		node := &model.ResourceNode{Kind: "Pod", Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"automountServiceAccountToken": false,
				"volumes": []any{
					map[string]any{
						"name": "proj",
						"projected": map[string]any{
							"sources": []any{
								map[string]any{"secret": map[string]any{"name": "proj-secret"}},
								map[string]any{"configMap": map[string]any{"name": "proj-cm"}},
								map[string]any{"serviceAccountToken": map[string]any{"path": "token"}},
								map[string]any{"downwardAPI": map[string]any{}},
							},
						},
					},
				},
			},
		}

		appendPodRefs(node, obj, "ns", nil)

		assert.ElementsMatch(t, [][2]string{
			{"ConfigMap", "proj-cm"},
			{"Secret", "proj-secret"},
		}, kindNames(node.Children))
	})
}

func TestAppendPodRefs_ImagePullSecrets(t *testing.T) {
	node := &model.ResourceNode{Kind: "Pod", Namespace: "ns"}
	obj := map[string]any{
		"spec": map[string]any{
			"automountServiceAccountToken": false,
			"imagePullSecrets": []any{
				map[string]any{"name": "regcred"},
				map[string]any{"name": "regcred-backup"},
			},
		},
	}

	appendPodRefs(node, obj, "ns", nil)

	assert.ElementsMatch(t, [][2]string{
		{"Secret", "regcred"},
		{"Secret", "regcred-backup"},
	}, kindNames(node.Children))
}

func TestAppendPodRefs_DedupAndOrder(t *testing.T) {
	t.Run("same secret via env and envFrom emits once", func(t *testing.T) {
		node := &model.ResourceNode{Kind: "Pod", Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"automountServiceAccountToken": false,
				"containers": []any{
					map[string]any{
						"name": "a",
						"env": []any{
							map[string]any{
								"valueFrom": map[string]any{
									"secretKeyRef": map[string]any{"name": "shared"},
								},
							},
						},
						"envFrom": []any{
							map[string]any{"secretRef": map[string]any{"name": "shared"}},
						},
					},
					map[string]any{
						"name": "b",
						"envFrom": []any{
							map[string]any{"secretRef": map[string]any{"name": "shared"}},
						},
					},
				},
			},
		}

		appendPodRefs(node, obj, "ns", nil)

		require.Len(t, node.Children, 1)
		assert.Equal(t, "shared", node.Children[0].Name)
	})

	t.Run("emission order: SA, ConfigMap, Secret, PVC", func(t *testing.T) {
		node := &model.ResourceNode{Kind: "Pod", Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"serviceAccountName": "sa1",
				"volumes": []any{
					map[string]any{"persistentVolumeClaim": map[string]any{"claimName": "p1"}},
					map[string]any{"secret": map[string]any{"secretName": "s1"}},
					map[string]any{"configMap": map[string]any{"name": "c1"}},
				},
			},
		}

		appendPodRefs(node, obj, "ns", nil)

		got := kindNames(node.Children)
		require.Equal(t, [][2]string{
			{"ServiceAccount", "sa1"},
			{"ConfigMap", "c1"},
			{"Secret", "s1"},
			{"PersistentVolumeClaim", "p1"},
		}, got)
	})
}

func TestAppendPodRefs_Edges(t *testing.T) {
	t.Run("nil spec emits no refs", func(t *testing.T) {
		node := &model.ResourceNode{Kind: "Pod", Namespace: "ns"}
		appendPodRefs(node, map[string]any{}, "ns", nil)
		assert.Empty(t, node.Children)
	})

	t.Run("empty ref names are skipped", func(t *testing.T) {
		node := &model.ResourceNode{Kind: "Pod", Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"automountServiceAccountToken": false,
				"imagePullSecrets": []any{
					map[string]any{"name": ""},
					map[string]any{"name": "ok"},
				},
			},
		}

		appendPodRefs(node, obj, "ns", nil)

		require.Len(t, node.Children, 1)
		assert.Equal(t, "ok", node.Children[0].Name)
	})

	t.Run("non-map ref maps are tolerated", func(t *testing.T) {
		node := &model.ResourceNode{Kind: "Pod", Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"automountServiceAccountToken": false,
				"volumes": []any{
					"not-a-map",
					map[string]any{"secret": map[string]any{"secretName": "ok"}},
				},
			},
		}

		appendPodRefs(node, obj, "ns", nil)

		require.Len(t, node.Children, 1)
		assert.Equal(t, "ok", node.Children[0].Name)
	})

	t.Run("preserves existing children", func(t *testing.T) {
		existing := &model.ResourceNode{Name: "app", Kind: "Container"}
		node := &model.ResourceNode{
			Kind:      "Pod",
			Namespace: "ns",
			Children:  []*model.ResourceNode{existing},
		}
		obj := map[string]any{
			"spec": map[string]any{
				"automountServiceAccountToken": false,
				"containers": []any{
					map[string]any{
						"name": "app",
						"envFrom": []any{
							map[string]any{"secretRef": map[string]any{"name": "s"}},
						},
					},
				},
			},
		}

		appendPodRefs(node, obj, "ns", nil)

		require.Len(t, node.Children, 2)
		assert.Equal(t, "Container", node.Children[0].Kind)
		assert.Equal(t, "Secret", node.Children[1].Kind)
		assert.Equal(t, "refs", node.Children[1].Group)
	})
}

// --- existsFn behavior: required vs optional, MissingRef ---

func TestAppendPodRefs_ExistenceCheck(t *testing.T) {
	t.Run("missing required ref gets MissingRef status", func(t *testing.T) {
		node := &model.ResourceNode{Kind: "Pod", Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"automountServiceAccountToken": false,
				"containers": []any{
					map[string]any{
						"name": "app",
						"envFrom": []any{
							map[string]any{"secretRef": map[string]any{"name": "missing"}},
						},
					},
				},
			},
		}
		exists := func(kind, name string) bool { return false }

		appendPodRefs(node, obj, "ns", exists)

		require.Len(t, node.Children, 1)
		assert.Equal(t, "missing", node.Children[0].Name)
		assert.Equal(t, model.MissingRefStatus, node.Children[0].Status)
	})

	t.Run("missing optional ref is not flagged", func(t *testing.T) {
		node := &model.ResourceNode{Kind: "Pod", Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"automountServiceAccountToken": false,
				"containers": []any{
					map[string]any{
						"name": "app",
						"envFrom": []any{
							map[string]any{
								"secretRef": map[string]any{
									"name":     "soft",
									"optional": true,
								},
							},
						},
					},
				},
			},
		}
		exists := func(kind, name string) bool { return false }

		appendPodRefs(node, obj, "ns", exists)

		require.Len(t, node.Children, 1)
		assert.Equal(t, "", node.Children[0].Status)
	})

	t.Run("existing ref has empty status", func(t *testing.T) {
		node := &model.ResourceNode{Kind: "Pod", Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"automountServiceAccountToken": false,
				"containers": []any{
					map[string]any{
						"name": "app",
						"envFrom": []any{
							map[string]any{"secretRef": map[string]any{"name": "ok"}},
						},
					},
				},
			},
		}
		exists := func(kind, name string) bool { return true }

		appendPodRefs(node, obj, "ns", exists)

		require.Len(t, node.Children, 1)
		assert.Equal(t, "", node.Children[0].Status)
	})

	t.Run("required wins when same ref appears as both required and optional", func(t *testing.T) {
		node := &model.ResourceNode{Kind: "Pod", Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"automountServiceAccountToken": false,
				"containers": []any{
					map[string]any{
						"name": "app",
						"envFrom": []any{
							// Optional first.
							map[string]any{
								"secretRef": map[string]any{
									"name":     "shared",
									"optional": true,
								},
							},
							// Required second — should upgrade the entry.
							map[string]any{"secretRef": map[string]any{"name": "shared"}},
						},
					},
				},
			},
		}
		exists := func(kind, name string) bool { return false }

		appendPodRefs(node, obj, "ns", exists)

		require.Len(t, node.Children, 1)
		assert.Equal(t, model.MissingRefStatus, node.Children[0].Status,
			"required reference must dominate optional dedup partner")
	})

	t.Run("PVC has no optional concept and is always required", func(t *testing.T) {
		node := &model.ResourceNode{Kind: "Pod", Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"automountServiceAccountToken": false,
				"volumes": []any{
					map[string]any{
						"persistentVolumeClaim": map[string]any{"claimName": "data"},
					},
				},
			},
		}
		exists := func(kind, name string) bool { return false }

		appendPodRefs(node, obj, "ns", exists)

		require.Len(t, node.Children, 1)
		assert.Equal(t, model.MissingRefStatus, node.Children[0].Status)
	})

	t.Run("ServiceAccount is always required", func(t *testing.T) {
		node := &model.ResourceNode{Kind: "Pod", Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{"serviceAccountName": "ghost"},
		}
		exists := func(kind, name string) bool { return false }

		appendPodRefs(node, obj, "ns", exists)

		require.Len(t, node.Children, 1)
		assert.Equal(t, "ServiceAccount", node.Children[0].Kind)
		assert.Equal(t, model.MissingRefStatus, node.Children[0].Status)
	})

	t.Run("imagePullSecret missing is flagged", func(t *testing.T) {
		node := &model.ResourceNode{Kind: "Pod", Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{
				"automountServiceAccountToken": false,
				"imagePullSecrets": []any{
					map[string]any{"name": "regcred"},
				},
			},
		}
		exists := func(kind, name string) bool { return false }

		appendPodRefs(node, obj, "ns", exists)

		require.Len(t, node.Children, 1)
		assert.Equal(t, model.MissingRefStatus, node.Children[0].Status)
	})

	t.Run("nil existsFn skips checks entirely", func(t *testing.T) {
		node := &model.ResourceNode{Kind: "Pod", Namespace: "ns"}
		obj := map[string]any{
			"spec": map[string]any{"serviceAccountName": "anything"},
		}

		appendPodRefs(node, obj, "ns", nil)

		require.Len(t, node.Children, 1)
		assert.Equal(t, "", node.Children[0].Status,
			"a nil existsFn must never produce MissingRef")
	})
}
