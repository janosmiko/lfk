package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- populateResourceDetails: Pod with PodInitializing + Failed status ---

func TestPopulateResourceDetails_Pod_InitializingFailedStatus(t *testing.T) {
	// When init container reason is "PodInitializing" and pod status is "Failed",
	// the reason should be cleared (line 81-83).
	obj := map[string]any{
		"spec": map[string]any{
			"containers": []any{
				map[string]any{"name": "app"},
			},
		},
		"status": map[string]any{
			"phase": "Failed",
			"initContainerStatuses": []any{
				map[string]any{
					"ready": false,
					"state": map[string]any{
						"waiting": map[string]any{
							"reason": "PodInitializing",
						},
					},
				},
			},
			"containerStatuses": []any{
				map[string]any{
					"name":         "app",
					"ready":        false,
					"restartCount": float64(0),
					"state": map[string]any{
						"waiting": map[string]any{
							"reason": "PodInitializing",
						},
					},
				},
			},
		},
	}

	ti := &model.Item{Status: "Failed"}
	populateResourceDetails(ti, obj, "Pod")

	// With PodInitializing + Failed, reason gets cleared and status stays "Failed".
	assert.Equal(t, "Failed", ti.Status)
}

// --- populateResourceDetails: Ingress with service name only (no port map) ---

func TestPopulateResourceDetails_Ingress_DefaultBackendServiceNameOnly(t *testing.T) {
	// When a default backend has a service with name but no port map (line 336-338).
	obj := map[string]any{
		"spec": map[string]any{
			"defaultBackend": map[string]any{
				"service": map[string]any{
					"name": "my-backend",
				},
			},
		},
	}

	ti := &model.Item{}
	populateResourceDetails(ti, obj, "Ingress")

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "my-backend", colMap["Default Backend"])
}

func TestPopulateResourceDetails_Ingress_DefaultBackendPortName(t *testing.T) {
	// When a default backend has a service with a named port instead of numeric (line 333-334).
	obj := map[string]any{
		"spec": map[string]any{
			"defaultBackend": map[string]any{
				"service": map[string]any{
					"name": "my-backend",
					"port": map[string]any{
						"name": "https",
					},
				},
			},
		},
	}

	ti := &model.Item{}
	populateResourceDetails(ti, obj, "Ingress")

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "my-backend:https", colMap["Default Backend"])
}

func TestPopulateResourceDetails_Ingress_LoadBalancerHostname(t *testing.T) {
	// When a load balancer ingress entry has hostname instead of IP (line 392-394).
	obj := map[string]any{
		"spec": map[string]any{
			"rules": []any{
				map[string]any{
					"host": "example.com",
				},
			},
		},
		"status": map[string]any{
			"loadBalancer": map[string]any{
				"ingress": []any{
					map[string]any{
						"hostname": "lb.example.com",
					},
				},
			},
		},
	}

	ti := &model.Item{}
	populateResourceDetails(ti, obj, "Ingress")

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "lb.example.com", colMap["Address"])
}

// --- populateResourceDetails: HPA with non-map metric entries ---

func TestPopulateResourceDetails_HPA_NonMapSpecMetric(t *testing.T) {
	// Non-map metric entries in spec.metrics should be skipped (line 632-633).
	obj := map[string]any{
		"spec": map[string]any{
			"maxReplicas": float64(5),
			"metrics": []any{
				"not-a-map",
				map[string]any{
					"type": "Resource",
					"resource": map[string]any{
						"name": "cpu",
						"target": map[string]any{
							"type":               "Utilization",
							"averageUtilization": float64(70),
						},
					},
				},
			},
		},
		"status": map[string]any{
			"currentReplicas": float64(1),
			"desiredReplicas": float64(1),
		},
	}

	ti := &model.Item{}
	populateResourceDetails(ti, obj, "HorizontalPodAutoscaler")

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "70%", colMap["Target Cpu"])
}

func TestPopulateResourceDetails_HPA_NonMapCurrentMetric(t *testing.T) {
	// Non-map metric entries in status.currentMetrics should be skipped (line 705-706).
	obj := map[string]any{
		"spec": map[string]any{
			"maxReplicas": float64(5),
		},
		"status": map[string]any{
			"currentReplicas": float64(1),
			"desiredReplicas": float64(1),
			"currentMetrics": []any{
				"not-a-map",
				map[string]any{
					"type": "Resource",
					"resource": map[string]any{
						"name": "cpu",
						"current": map[string]any{
							"averageUtilization": float64(45),
						},
					},
				},
			},
		},
	}

	ti := &model.Item{}
	populateResourceDetails(ti, obj, "HorizontalPodAutoscaler")

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "45%", colMap["Current Cpu"])
}

func TestPopulateResourceDetails_HPA_NonMapCondition(t *testing.T) {
	// Non-map condition entries in status.conditions should be skipped (line 749-750).
	obj := map[string]any{
		"spec": map[string]any{
			"maxReplicas": float64(3),
		},
		"status": map[string]any{
			"currentReplicas": float64(3),
			"desiredReplicas": float64(3),
			"conditions": []any{
				"not-a-map",
				map[string]any{
					"type":    "ScalingLimited",
					"status":  "True",
					"message": "limited",
				},
			},
		},
	}

	ti := &model.Item{}
	populateResourceDetails(ti, obj, "HorizontalPodAutoscaler")

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "limited", colMap["Scaling Limited"])
}
