package k8s

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- Pod status override branches (not covered by existing tests) ---

func TestPopulate_PodInitContainerFailureOverride(t *testing.T) {
	obj := map[string]any{
		"spec": map[string]any{
			"containers": []any{
				map[string]any{"name": "app"},
			},
		},
		"status": map[string]any{
			"phase": "Pending",
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
			"initContainerStatuses": []any{
				map[string]any{
					"name":  "init",
					"ready": false,
					"state": map[string]any{
						"terminated": map[string]any{
							"reason": "Error",
						},
					},
				},
			},
		},
	}
	ti := model.Item{Status: "Pending"}
	populateResourceDetails(&ti, obj, "Pod")
	assert.Equal(t, "Error", ti.Status)
}

func TestPopulate_PodRunningNotReadyBecomesNotReady(t *testing.T) {
	obj := map[string]any{
		"spec": map[string]any{
			"containers": []any{
				map[string]any{"name": "app"},
				map[string]any{"name": "sidecar"},
			},
		},
		"status": map[string]any{
			"phase": "Running",
			"containerStatuses": []any{
				map[string]any{
					"name":         "app",
					"ready":        true,
					"restartCount": float64(0),
				},
				map[string]any{
					"name":         "sidecar",
					"ready":        false,
					"restartCount": float64(0),
				},
			},
		},
	}
	ti := model.Item{Status: "Running"}
	populateResourceDetails(&ti, obj, "Pod")
	assert.Equal(t, "NotReady", ti.Status)
}

func TestPopulate_PodSucceededKeepsStatus(t *testing.T) {
	obj := map[string]any{
		"spec": map[string]any{
			"containers": []any{
				map[string]any{"name": "app"},
			},
		},
		"status": map[string]any{
			"phase": "Succeeded",
			"containerStatuses": []any{
				map[string]any{
					"name":         "app",
					"ready":        false,
					"restartCount": float64(0),
				},
			},
		},
	}
	ti := model.Item{Status: "Succeeded"}
	populateResourceDetails(&ti, obj, "Pod")
	assert.Equal(t, "Succeeded", ti.Status)
}

func TestPopulate_PodFailedPreferredOverPodInitializing(t *testing.T) {
	obj := map[string]any{
		"spec": map[string]any{
			"containers": []any{
				map[string]any{"name": "app"},
			},
		},
		"status": map[string]any{
			"phase": "Failed",
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
			"initContainerStatuses": []any{
				map[string]any{
					"name":  "init",
					"ready": false,
					"state": map[string]any{
						"waiting": map[string]any{
							"reason": "PodInitializing",
						},
					},
				},
			},
		},
	}
	ti := model.Item{Status: "Failed"}
	populateResourceDetails(&ti, obj, "Pod")
	// PodInitializing + Failed => reason cleared, status stays "Failed"
	// because the else-if only sets NotReady when status is "Running".
	assert.Equal(t, "Failed", ti.Status)
}

func TestPopulate_PodNilStatusReturnsEarly(t *testing.T) {
	obj := map[string]any{
		"spec": map[string]any{
			"containers": []any{
				map[string]any{"name": "app"},
			},
		},
	}
	ti := model.Item{}
	populateResourceDetails(&ti, obj, "Pod")
	assert.Empty(t, ti.Ready)
}

func TestPopulate_PodRestartCountInt64(t *testing.T) {
	obj := map[string]any{
		"spec": map[string]any{
			"containers": []any{
				map[string]any{"name": "app"},
			},
		},
		"status": map[string]any{
			"containerStatuses": []any{
				map[string]any{
					"name":         "app",
					"ready":        true,
					"restartCount": int64(5),
				},
			},
		},
	}
	ti := model.Item{Status: "Running"}
	populateResourceDetails(&ti, obj, "Pod")
	assert.Equal(t, "5", ti.Restarts)
}

// --- Service: additional branches ---

func TestPopulate_ServiceLoadBalancerHostname(t *testing.T) {
	obj := map[string]any{
		"spec": map[string]any{
			"type": "LoadBalancer",
		},
		"status": map[string]any{
			"loadBalancer": map[string]any{
				"ingress": []any{
					map[string]any{"hostname": "my-lb.example.com"},
				},
			},
		},
	}
	ti := model.Item{}
	populateResourceDetails(&ti, obj, "Service")

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "my-lb.example.com", colMap["External Address"])
}

func TestPopulate_ServiceExternalIPs(t *testing.T) {
	obj := map[string]any{
		"spec": map[string]any{
			"type":        "ClusterIP",
			"externalIPs": []any{"1.2.3.4", "5.6.7.8"},
		},
	}
	ti := model.Item{}
	populateResourceDetails(&ti, obj, "Service")

	colMap := columnsToMap(ti.Columns)
	assert.Contains(t, colMap["External IPs"], "1.2.3.4")
	assert.Contains(t, colMap["External IPs"], "5.6.7.8")
}

func TestPopulate_ServiceSessionAffinityClientIP(t *testing.T) {
	obj := map[string]any{
		"spec": map[string]any{
			"type":            "ClusterIP",
			"sessionAffinity": "ClientIP",
		},
	}
	ti := model.Item{}
	populateResourceDetails(&ti, obj, "Service")

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "ClientIP", colMap["Session Affinity"])
}

func TestPopulate_ServiceSessionAffinityNoneOmitted(t *testing.T) {
	obj := map[string]any{
		"spec": map[string]any{
			"type":            "ClusterIP",
			"sessionAffinity": "None",
		},
	}
	ti := model.Item{}
	populateResourceDetails(&ti, obj, "Service")

	colMap := columnsToMap(ti.Columns)
	_, found := colMap["Session Affinity"]
	assert.False(t, found)
}

func TestPopulate_ServiceNilSpec(t *testing.T) {
	obj := map[string]any{}
	ti := model.Item{}
	populateResourceDetails(&ti, obj, "Service")
	assert.Empty(t, ti.Columns)
}

// --- Ingress: additional branches ---

func TestPopulate_IngressDefaultBackendPortNumber(t *testing.T) {
	obj := map[string]any{
		"spec": map[string]any{
			"defaultBackend": map[string]any{
				"service": map[string]any{
					"name": "backend-svc",
					"port": map[string]any{
						"number": float64(8080),
					},
				},
			},
		},
	}
	ti := model.Item{}
	populateResourceDetails(&ti, obj, "Ingress")

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "backend-svc:8080", colMap["Default Backend"])
}

func TestPopulate_IngressDefaultBackendPortName(t *testing.T) {
	obj := map[string]any{
		"spec": map[string]any{
			"defaultBackend": map[string]any{
				"service": map[string]any{
					"name": "api-svc",
					"port": map[string]any{
						"name": "http",
					},
				},
			},
		},
	}
	ti := model.Item{}
	populateResourceDetails(&ti, obj, "Ingress")

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "api-svc:http", colMap["Default Backend"])
}

func TestPopulate_IngressDefaultBackendNoPort(t *testing.T) {
	obj := map[string]any{
		"spec": map[string]any{
			"defaultBackend": map[string]any{
				"service": map[string]any{
					"name": "simple-svc",
				},
			},
		},
	}
	ti := model.Item{}
	populateResourceDetails(&ti, obj, "Ingress")

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "simple-svc", colMap["Default Backend"])
}

func TestPopulate_IngressTLSAndURL(t *testing.T) {
	obj := map[string]any{
		"spec": map[string]any{
			"rules": []any{
				map[string]any{
					"host": "app.example.com",
					"http": map[string]any{
						"paths": []any{
							map[string]any{
								"path": "/api",
							},
						},
					},
				},
			},
			"tls": []any{
				map[string]any{
					"hosts": []any{"app.example.com"},
				},
			},
		},
	}
	ti := model.Item{}
	populateResourceDetails(&ti, obj, "Ingress")

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "https://app.example.com/api", colMap["__ingress_url"])
	assert.Contains(t, colMap["TLS Hosts"], "app.example.com")
}

func TestPopulate_IngressHTTPUrlNoTLS(t *testing.T) {
	obj := map[string]any{
		"spec": map[string]any{
			"rules": []any{
				map[string]any{
					"host": "plain.example.com",
				},
			},
		},
	}
	ti := model.Item{}
	populateResourceDetails(&ti, obj, "Ingress")

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "http://plain.example.com", colMap["__ingress_url"])
}

func TestPopulate_IngressLBHostname(t *testing.T) {
	obj := map[string]any{
		"spec": map[string]any{},
		"status": map[string]any{
			"loadBalancer": map[string]any{
				"ingress": []any{
					map[string]any{"hostname": "lb.aws.com"},
				},
			},
		},
	}
	ti := model.Item{}
	populateResourceDetails(&ti, obj, "Ingress")

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "lb.aws.com", colMap["Address"])
}

// --- ConfigMap ---

func TestPopulate_ConfigMapData(t *testing.T) {
	obj := map[string]any{
		"data": map[string]any{
			"config.yaml": "key: value\n",
			"app.conf":    "setting=1",
		},
	}
	ti := model.Item{}
	populateResourceDetails(&ti, obj, "ConfigMap")

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "setting=1", colMap["data:app.conf"])
	assert.Equal(t, "key: value\n", colMap["data:config.yaml"])
}

// --- Secret ---

func TestPopulate_SecretBase64(t *testing.T) {
	obj := map[string]any{
		"type": "Opaque",
		"data": map[string]any{
			"password": base64.StdEncoding.EncodeToString([]byte("s3cr3t")),
		},
	}
	ti := model.Item{}
	populateResourceDetails(&ti, obj, "Secret")

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "s3cr3t", colMap["secret:password"])
	assert.Equal(t, "Opaque", colMap["Type"])
}

func TestPopulate_SecretInvalidBase64Skipped(t *testing.T) {
	obj := map[string]any{
		"type": "Opaque",
		"data": map[string]any{
			"broken": "!!!not-valid-base64!!!",
		},
	}
	ti := model.Item{}
	populateResourceDetails(&ti, obj, "Secret")

	colMap := columnsToMap(ti.Columns)
	_, found := colMap["secret:broken"]
	assert.False(t, found)
}

// --- Node ---

func TestPopulate_NodeRolesAndTaints(t *testing.T) {
	obj := map[string]any{
		"metadata": map[string]any{
			"labels": map[string]any{
				"node-role.kubernetes.io/control-plane": "",
				"node-role.kubernetes.io/worker":        "",
			},
		},
		"spec": map[string]any{
			"taints": []any{
				map[string]any{
					"key":    "node-role.kubernetes.io/control-plane",
					"effect": "NoSchedule",
				},
				map[string]any{
					"key":    "dedicated",
					"value":  "gpu",
					"effect": "NoExecute",
				},
			},
		},
		"status": map[string]any{
			"addresses": []any{
				map[string]any{"type": "InternalIP", "address": "10.0.0.5"},
			},
			"allocatable": map[string]any{
				"cpu":    "4",
				"memory": "8Gi",
			},
			"nodeInfo": map[string]any{
				"kubeletVersion":          "v1.29.0",
				"osImage":                 "Ubuntu 22.04",
				"containerRuntimeVersion": "containerd://1.7.2",
			},
		},
	}
	ti := model.Item{}
	populateResourceDetails(&ti, obj, "Node")

	colMap := columnsToMap(ti.Columns)
	assert.Contains(t, colMap["Role"], "control-plane")
	assert.Contains(t, colMap["Role"], "worker")
	assert.Equal(t, "10.0.0.5", colMap["InternalIP"])
	assert.Equal(t, "4", colMap["CPU Alloc"])
	assert.Equal(t, "8Gi", colMap["Mem Alloc"])
	assert.Equal(t, "v1.29.0", colMap["Version"])
	assert.Equal(t, "Ubuntu 22.04", colMap["OS"])
	assert.Equal(t, "containerd://1.7.2", colMap["Runtime"])
	assert.Contains(t, colMap["Taints"], "dedicated=gpu:NoExecute")
}

func TestPopulate_NodeEmptyRoleSuffix(t *testing.T) {
	obj := map[string]any{
		"metadata": map[string]any{
			"labels": map[string]any{
				"node-role.kubernetes.io/": "",
			},
		},
		"status": map[string]any{},
	}
	ti := model.Item{}
	populateResourceDetails(&ti, obj, "Node")

	colMap := columnsToMap(ti.Columns)
	_, found := colMap["Role"]
	assert.False(t, found)
}

// --- PVC ---

func TestPopulate_PVCBound(t *testing.T) {
	obj := map[string]any{
		"spec": map[string]any{
			"resources": map[string]any{
				"requests": map[string]any{
					"storage": "10Gi",
				},
			},
			"volumeName":       "pv-123",
			"accessModes":      []any{"ReadWriteOnce"},
			"storageClassName": "standard",
			"volumeMode":       "Filesystem",
		},
		"status": map[string]any{
			"phase": "Bound",
			"capacity": map[string]any{
				"storage": "10Gi",
			},
		},
	}
	ti := model.Item{}
	populateResourceDetails(&ti, obj, "PersistentVolumeClaim")

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "Bound", ti.Status)
	assert.Equal(t, "10Gi", colMap["Capacity"])
	assert.Equal(t, "10Gi", colMap["Request"])
	assert.Equal(t, "pv-123", colMap["Volume"])
	assert.Equal(t, "ReadWriteOnce", colMap["Access Modes"])
	assert.Equal(t, "standard", colMap["Storage Class"])
	assert.Equal(t, "Filesystem", colMap["Volume Mode"])
}

// --- CronJob ---

func TestPopulate_CronJobFields(t *testing.T) {
	obj := map[string]any{
		"spec": map[string]any{
			"schedule": "*/5 * * * *",
			"suspend":  false,
		},
		"status": map[string]any{
			"lastScheduleTime": "2026-03-22T10:00:00Z",
		},
	}
	ti := model.Item{}
	populateResourceDetails(&ti, obj, "CronJob")

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "*/5 * * * *", colMap["Schedule"])
	assert.Equal(t, "false", colMap["Suspend"])
	assert.Equal(t, "2026-03-22T10:00:00Z", colMap["Last Schedule"])
}

// --- Job ---

func TestPopulate_JobZeroFailuresOmitted(t *testing.T) {
	obj := map[string]any{
		"status": map[string]any{
			"failed":    float64(0),
			"succeeded": float64(1),
		},
	}
	ti := model.Item{}
	populateResourceDetails(&ti, obj, "Job")

	colMap := columnsToMap(ti.Columns)
	_, found := colMap["Failed"]
	assert.False(t, found)
}

// --- HPA: additional metric type branches ---

func TestPopulate_HPAPodsMetric(t *testing.T) {
	obj := map[string]any{
		"spec": map[string]any{
			"maxReplicas": float64(5),
			"metrics": []any{
				map[string]any{
					"type": "Pods",
					"pods": map[string]any{
						"metric": map[string]any{
							"name": "requests_per_second",
						},
						"target": map[string]any{
							"averageValue": "100",
						},
					},
				},
			},
		},
		"status": map[string]any{
			"currentReplicas": float64(3),
			"desiredReplicas": float64(3),
			"currentMetrics": []any{
				map[string]any{
					"type": "Pods",
					"pods": map[string]any{
						"metric": map[string]any{
							"name": "requests_per_second",
						},
						"current": map[string]any{
							"averageValue": "85",
						},
					},
				},
			},
		},
	}
	ti := model.Item{}
	populateResourceDetails(&ti, obj, "HorizontalPodAutoscaler")

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "100", colMap["Target requests_per_second"])
	assert.Equal(t, "85", colMap["Current requests_per_second"])
}

func TestPopulate_HPAObjectMetric(t *testing.T) {
	obj := map[string]any{
		"spec": map[string]any{
			"maxReplicas": float64(10),
			"metrics": []any{
				map[string]any{
					"type": "Object",
					"object": map[string]any{
						"metric": map[string]any{
							"name": "queue_depth",
						},
						"target": map[string]any{
							"value": "50",
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
	ti := model.Item{}
	populateResourceDetails(&ti, obj, "HorizontalPodAutoscaler")

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "50", colMap["Target queue_depth"])
}

func TestPopulate_HPANonMapMetricSkipped(t *testing.T) {
	obj := map[string]any{
		"spec": map[string]any{
			"maxReplicas": float64(5),
			"metrics": []any{
				"not-a-map",
			},
		},
		"status": map[string]any{
			"currentReplicas": float64(1),
			"desiredReplicas": float64(1),
			"currentMetrics": []any{
				"not-a-map",
			},
		},
	}
	ti := model.Item{}
	populateResourceDetails(&ti, obj, "HorizontalPodAutoscaler")
	assert.Equal(t, fmt.Sprintf("%d/%d (%d-%d)", 1, 1, 0, 5), ti.Ready)
}

func TestPopulate_HPAScalingLimitedFalseIgnored(t *testing.T) {
	obj := map[string]any{
		"spec": map[string]any{
			"maxReplicas": float64(5),
		},
		"status": map[string]any{
			"currentReplicas": float64(1),
			"desiredReplicas": float64(1),
			"conditions": []any{
				map[string]any{
					"type":   "ScalingLimited",
					"status": "False",
				},
			},
		},
	}
	ti := model.Item{}
	populateResourceDetails(&ti, obj, "HorizontalPodAutoscaler")

	colMap := columnsToMap(ti.Columns)
	_, found := colMap["Scaling Limited"]
	assert.False(t, found)
}

func TestPopulate_HPAResourceAverageValue(t *testing.T) {
	obj := map[string]any{
		"spec": map[string]any{
			"maxReplicas": float64(5),
			"metrics": []any{
				map[string]any{
					"type": "Resource",
					"resource": map[string]any{
						"name": "memory",
						"target": map[string]any{
							"type":         "AverageValue",
							"averageValue": "500Mi",
						},
					},
				},
			},
		},
		"status": map[string]any{
			"currentReplicas": float64(1),
			"desiredReplicas": float64(1),
			"currentMetrics": []any{
				map[string]any{
					"type": "Resource",
					"resource": map[string]any{
						"name": "memory",
						"current": map[string]any{
							"averageValue": "256Mi",
						},
					},
				},
			},
		},
	}
	ti := model.Item{}
	populateResourceDetails(&ti, obj, "HorizontalPodAutoscaler")

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "500Mi", colMap["Target Memory"])
	assert.Equal(t, "256Mi", colMap["Current Memory"])
}

// --- Unknown kind falls through to ext ---

func TestPopulate_UnknownKindFallsToExt(t *testing.T) {
	obj := map[string]any{
		"status": map[string]any{
			"conditions": []any{
				map[string]any{
					"type":   "Ready",
					"status": "True",
				},
			},
		},
	}
	ti := model.Item{}
	populateResourceDetails(&ti, obj, "CustomWidget")

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "True", colMap["Ready"])
}
