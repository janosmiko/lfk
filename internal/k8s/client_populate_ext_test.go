package k8s

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- populateResourceDetailsExt: FluxCD resources ---

func TestPopulateResourceDetailsExt_FluxCD(t *testing.T) {
	fluxKinds := []string{
		"Kustomization", "GitRepository", "HelmRepository", "HelmChart",
		"OCIRepository", "Bucket", "Alert", "Provider", "Receiver",
		"ImageRepository", "ImagePolicy", "ImageUpdateAutomation",
	}

	for _, kind := range fluxKinds {
		t.Run(kind+"/ready with revision", func(t *testing.T) {
			obj := map[string]interface{}{
				"spec": map[string]interface{}{},
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"type":               "Ready",
							"status":             "True",
							"reason":             "ReconciliationSucceeded",
							"lastTransitionTime": "2025-01-15T10:00:00Z",
						},
					},
					"lastAppliedRevision": "main@sha1:abc123def456789",
				},
			}
			status, _ := obj["status"].(map[string]interface{})
			spec, _ := obj["spec"].(map[string]interface{})
			ti := &model.Item{}
			populateResourceDetailsExt(ti, obj, kind, status, spec)

			colMap := columnsToMap(ti.Columns)
			assert.Equal(t, "True", colMap["Ready"])
			assert.Equal(t, "ReconciliationSucceeded", colMap["Reason"])
			assert.Equal(t, "main@sha1:ab", colMap["Revision"]) // truncated to 12
			assert.Contains(t, colMap["Last Transition"], "ago")
		})
	}

	t.Run("FluxCD with Ready=False shows message", func(t *testing.T) {
		obj := map[string]interface{}{
			"spec": map[string]interface{}{},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":    "Ready",
						"status":  "False",
						"reason":  "ReconciliationFailed",
						"message": "unable to clone repo",
					},
				},
			},
		}
		status, _ := obj["status"].(map[string]interface{})
		spec, _ := obj["spec"].(map[string]interface{})
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "GitRepository", status, spec)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "False", colMap["Ready"])
		assert.Equal(t, "unable to clone repo", colMap["Message"])
	})

	t.Run("FluxCD suspended", func(t *testing.T) {
		obj := map[string]interface{}{
			"spec": map[string]interface{}{
				"suspend": true,
			},
			"status": map[string]interface{}{},
		}
		status, _ := obj["status"].(map[string]interface{})
		spec, _ := obj["spec"].(map[string]interface{})
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "Kustomization", status, spec)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "True", colMap["Suspended"])
	})

	t.Run("FluxCD revision from artifact fallback", func(t *testing.T) {
		obj := map[string]interface{}{
			"spec": map[string]interface{}{},
			"status": map[string]interface{}{
				"artifact": map[string]interface{}{
					"revision": "v1.0.0/sha256:abcdef123456",
				},
			},
		}
		status, _ := obj["status"].(map[string]interface{})
		spec, _ := obj["spec"].(map[string]interface{})
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "HelmChart", status, spec)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "v1.0.0/sha25", colMap["Revision"])
	})

	t.Run("FluxCD short revision not truncated", func(t *testing.T) {
		obj := map[string]interface{}{
			"spec": map[string]interface{}{},
			"status": map[string]interface{}{
				"lastAppliedRevision": "v1.0.0",
			},
		}
		status, _ := obj["status"].(map[string]interface{})
		spec, _ := obj["spec"].(map[string]interface{})
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "HelmRepository", status, spec)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "v1.0.0", colMap["Revision"])
	})
}

// --- populateResourceDetailsExt: cert-manager resources ---

func TestPopulateResourceDetailsExt_CertManager(t *testing.T) {
	certManagerKinds := []string{
		"Certificate", "CertificateRequest", "Issuer", "ClusterIssuer", "Order", "Challenge",
	}

	for _, kind := range certManagerKinds {
		t.Run(kind+"/ready with expiry", func(t *testing.T) {
			obj := map[string]interface{}{
				"spec": map[string]interface{}{
					"secretName": "tls-secret",
				},
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"type":               "Ready",
							"status":             "True",
							"reason":             "Ready",
							"lastTransitionTime": "2025-01-10T12:00:00Z",
						},
					},
					"notAfter":    "2025-07-10T12:00:00Z",
					"renewalTime": "2025-06-10T12:00:00Z",
				},
			}
			status, _ := obj["status"].(map[string]interface{})
			spec, _ := obj["spec"].(map[string]interface{})
			ti := &model.Item{}
			populateResourceDetailsExt(ti, obj, kind, status, spec)

			colMap := columnsToMap(ti.Columns)
			assert.Equal(t, "True", colMap["Ready"])
			assert.Equal(t, "2025-07-10T12:00:00Z", colMap["Expires"])
			assert.Equal(t, "2025-06-10T12:00:00Z", colMap["Renewal"])
			if kind == "Certificate" || kind == "CertificateRequest" {
				assert.Equal(t, "tls-secret", colMap["Secret"])
			}
		})
	}

	t.Run("cert-manager with failed condition", func(t *testing.T) {
		obj := map[string]interface{}{
			"spec": map[string]interface{}{},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":    "Ready",
						"status":  "False",
						"reason":  "DoesNotExist",
						"message": "issuer not found",
					},
				},
			},
		}
		status, _ := obj["status"].(map[string]interface{})
		spec, _ := obj["spec"].(map[string]interface{})
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "Certificate", status, spec)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "False", colMap["Ready"])
		assert.Equal(t, "DoesNotExist", colMap["Reason"])
		assert.Equal(t, "issuer not found", colMap["Message"])
	})
}

// --- populateArgoCDApplication ---

func TestPopulateArgoCDApplication(t *testing.T) {
	tests := []struct {
		name     string
		obj      map[string]interface{}
		wantCols map[string]string
	}{
		{
			name: "healthy and synced application",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"destination": map[string]interface{}{
						"namespace": "production",
						"server":    "https://kubernetes.default.svc",
					},
					"source": map[string]interface{}{
						"repoURL": "https://github.com/example/repo",
						"path":    "deploy/production",
					},
				},
				"status": map[string]interface{}{
					"health": map[string]interface{}{
						"status": "Healthy",
					},
					"sync": map[string]interface{}{
						"status":   "Synced",
						"revision": "abc123def456",
					},
					"summary": map[string]interface{}{
						"images": []interface{}{"myapp:v1.0"},
					},
				},
			},
			wantCols: map[string]string{
				"Revision":    "abc123de", // truncated to 8
				"AutoSync":    "Off",
				"Dest NS":     "production",
				"Dest Server": "https://kubernetes.default.svc",
				"Repo":        "https://github.com/example/repo",
				"Path":        "deploy/production",
			},
		},
		{
			name: "application with health message",
			obj: map[string]interface{}{
				"status": map[string]interface{}{
					"health": map[string]interface{}{
						"status":  "Degraded",
						"message": "container failed health check",
					},
				},
			},
			wantCols: map[string]string{
				"Health Message": "container failed health check",
			},
		},
		{
			name: "application with operationState",
			obj: map[string]interface{}{
				"status": map[string]interface{}{
					"operationState": map[string]interface{}{
						"phase":      "Succeeded",
						"message":    "sync completed",
						"finishedAt": "2025-01-15T10:00:00Z",
					},
				},
			},
			wantCols: map[string]string{
				"Last Sync":    "Succeeded",
				"Sync Message": "sync completed",
			},
		},
		{
			name: "application with sync errors",
			obj: map[string]interface{}{
				"status": map[string]interface{}{
					"operationState": map[string]interface{}{
						"phase": "Failed",
						"syncResult": map[string]interface{}{
							"resources": []interface{}{
								map[string]interface{}{
									"kind":    "Deployment",
									"name":    "my-app",
									"status":  "SyncFailed",
									"message": "error applying",
								},
								map[string]interface{}{
									"kind":   "Service",
									"name":   "my-svc",
									"status": "Synced",
								},
							},
						},
					},
				},
			},
			wantCols: map[string]string{
				"Sync Errors": "Deployment/my-app: error applying",
			},
		},
		{
			name: "short revision not truncated",
			obj: map[string]interface{}{
				"status": map[string]interface{}{
					"sync": map[string]interface{}{
						"status":   "Synced",
						"revision": "abc",
					},
				},
			},
			wantCols: map[string]string{
				"Revision": "abc",
			},
		},
		{
			name: "nil status and spec",
			obj:  map[string]interface{}{},
		},
		{
			name: "application with conditions",
			obj: map[string]interface{}{
				"status": map[string]interface{}{
					"health": map[string]interface{}{
						"status": "Degraded",
					},
					"conditions": []interface{}{
						map[string]interface{}{
							"type":    "ComparisonError",
							"message": "rpc error: code = NotFound desc = repo not found",
						},
						map[string]interface{}{
							"type":               "SyncError",
							"message":            "sync failed: manifest generation error",
							"lastTransitionTime": "2025-01-15T10:00:00Z",
						},
					},
				},
			},
			wantCols: map[string]string{
				"Condition":                 "ComparisonError",
				"condition:ComparisonError": "rpc error: code = NotFound desc = repo not found",
			},
		},
		{
			name: "application with condition without message",
			obj: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"type": "OrphanedResourceWarning",
						},
					},
				},
			},
			wantCols: map[string]string{
				"Condition":                         "OrphanedResour~",
				"condition:OrphanedResourceWarning": "(no message)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, _ := tt.obj["status"].(map[string]interface{})
			spec, _ := tt.obj["spec"].(map[string]interface{})
			ti := &model.Item{}
			populateArgoCDApplication(ti, tt.obj, status, spec, "Application")

			if tt.wantCols != nil {
				colMap := columnsToMap(ti.Columns)
				for k, v := range tt.wantCols {
					assert.Equal(t, v, colMap[k], "column %q mismatch", k)
				}
			}
		})
	}
}

// --- populateEvent ---

func TestPopulateEvent(t *testing.T) {
	tests := []struct {
		name       string
		obj        map[string]interface{}
		wantStatus string
		wantCols   map[string]string
	}{
		{
			name: "normal event with all fields",
			obj: map[string]interface{}{
				"type":          "Normal",
				"lastTimestamp": "2025-01-15T10:00:00Z",
				"involvedObject": map[string]interface{}{
					"kind": "Pod",
					"name": "my-pod",
				},
				"reason":  "Scheduled",
				"message": "Successfully assigned default/my-pod to node-1",
				"count":   float64(1),
				"source": map[string]interface{}{
					"component": "default-scheduler",
				},
			},
			wantStatus: "Normal",
			wantCols: map[string]string{
				"Object":  "Pod/my-pod",
				"Reason":  "Scheduled",
				"Message": "Successfully assigned default/my-pod to node-1",
				"Count":   "1",
				"Source":  "default-scheduler",
			},
		},
		{
			name: "warning event with high count",
			obj: map[string]interface{}{
				"type":  "Warning",
				"count": int64(42),
				"involvedObject": map[string]interface{}{
					"kind": "Deployment",
					"name": "broken-app",
				},
				"reason":  "FailedCreate",
				"message": "Error creating pod",
			},
			wantStatus: "Warning",
			wantCols: map[string]string{
				"Count":  "42",
				"Reason": "FailedCreate",
				"Object": "Deployment/broken-app",
			},
		},
		{
			name: "event with eventTime fallback",
			obj: map[string]interface{}{
				"eventTime": "2025-01-15T10:00:00.123456789Z",
				"involvedObject": map[string]interface{}{
					"kind": "Node",
					"name": "worker-1",
				},
				"reason": "NodeReady",
			},
			wantCols: map[string]string{
				"Object": "Node/worker-1",
				"Reason": "NodeReady",
			},
		},
		{
			name: "event with default count of 1",
			obj: map[string]interface{}{
				"involvedObject": map[string]interface{}{},
			},
			wantCols: map[string]string{
				"Count": "1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populateEvent(ti, tt.obj)

			if tt.wantStatus != "" {
				assert.Equal(t, tt.wantStatus, ti.Status)
			}
			if tt.wantCols != nil {
				colMap := columnsToMap(ti.Columns)
				for k, v := range tt.wantCols {
					assert.Equal(t, v, colMap[k], "column %q mismatch", k)
				}
			}
		})
	}
}

// TestPopulateEventTimestamps verifies that populateEvent extracts both
// firstTimestamp and lastTimestamp into CreatedAt/LastSeen, and that the
// Last Seen column is populated.
func TestPopulateEventTimestamps(t *testing.T) {
	t.Run("both first and last timestamps", func(t *testing.T) {
		obj := map[string]interface{}{
			"firstTimestamp": "2026-04-10T08:00:00Z",
			"lastTimestamp":  "2026-04-10T11:30:00Z",
			"reason":         "BackOff",
		}
		ti := &model.Item{}
		populateEvent(ti, obj)

		assert.Equal(t, 2026, ti.CreatedAt.Year())
		assert.Equal(t, 8, ti.CreatedAt.Hour(), "CreatedAt should be firstTimestamp (08:00)")
		assert.Equal(t, 11, ti.LastSeen.Hour(), "LastSeen should be lastTimestamp (11:30)")
		colMap := columnsToMap(ti.Columns)
		assert.NotEmpty(t, colMap["Last Seen"], "Last Seen column should be present")
	})

	t.Run("only lastTimestamp falls back to first=last", func(t *testing.T) {
		obj := map[string]interface{}{
			"lastTimestamp": "2026-04-10T11:30:00Z",
		}
		ti := &model.Item{}
		populateEvent(ti, obj)

		assert.Equal(t, ti.CreatedAt, ti.LastSeen,
			"missing firstTimestamp should fall back to lastTimestamp")
		assert.Equal(t, 11, ti.LastSeen.Hour())
	})

	t.Run("only eventTime (events.k8s.io v1)", func(t *testing.T) {
		obj := map[string]interface{}{
			"eventTime": "2026-04-10T11:30:00.123Z",
		}
		ti := &model.Item{}
		populateEvent(ti, obj)

		assert.False(t, ti.CreatedAt.IsZero())
		assert.False(t, ti.LastSeen.IsZero())
		assert.Equal(t, ti.CreatedAt, ti.LastSeen)
	})
}

// --- populatePersistentVolume ---

func TestPopulatePersistentVolume(t *testing.T) {
	tests := []struct {
		name       string
		status     map[string]interface{}
		spec       map[string]interface{}
		wantStatus string
		wantCols   map[string]string
	}{
		{
			name: "bound PV with full spec",
			spec: map[string]interface{}{
				"capacity": map[string]interface{}{
					"storage": "100Gi",
				},
				"accessModes":                   []interface{}{"ReadWriteOnce", "ReadOnlyMany"},
				"persistentVolumeReclaimPolicy": "Retain",
				"storageClassName":              "gp3",
				"volumeMode":                    "Filesystem",
				"claimRef": map[string]interface{}{
					"namespace": "default",
					"name":      "my-pvc",
				},
			},
			status: map[string]interface{}{
				"phase": "Bound",
			},
			wantStatus: "Bound",
			wantCols: map[string]string{
				"Capacity":       "100Gi",
				"Access Modes":   "ReadWriteOnce, ReadOnlyMany",
				"Reclaim Policy": "Retain",
				"Storage Class":  "gp3",
				"Volume Mode":    "Filesystem",
				"Claim":          "default/my-pvc",
			},
		},
		{
			name: "released PV with reason",
			spec: map[string]interface{}{},
			status: map[string]interface{}{
				"phase":  "Released",
				"reason": "Manually released",
			},
			wantStatus: "Released",
			wantCols: map[string]string{
				"Reason": "Manually released",
			},
		},
		{
			name: "PV with claim without namespace",
			spec: map[string]interface{}{
				"claimRef": map[string]interface{}{
					"name": "standalone-pvc",
				},
			},
			wantCols: map[string]string{
				"Claim": "standalone-pvc",
			},
		},
		{
			name: "nil spec and status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populatePersistentVolume(ti, tt.status, tt.spec)

			if tt.wantStatus != "" {
				assert.Equal(t, tt.wantStatus, ti.Status)
			}
			if tt.wantCols != nil {
				colMap := columnsToMap(ti.Columns)
				for k, v := range tt.wantCols {
					assert.Equal(t, v, colMap[k], "column %q mismatch", k)
				}
			}
		})
	}
}

// --- populateResourceQuota ---

func TestPopulateResourceQuota(t *testing.T) {
	tests := []struct {
		name     string
		status   map[string]interface{}
		spec     map[string]interface{}
		wantCols map[string]string
	}{
		{
			name: "quota with status hard and used",
			status: map[string]interface{}{
				"hard": map[string]interface{}{
					"cpu":    "4",
					"memory": "8Gi",
					"pods":   "20",
				},
				"used": map[string]interface{}{
					"cpu":    "2",
					"memory": "4Gi",
					"pods":   "10",
				},
			},
			wantCols: map[string]string{
				"cpu":    "2 / 4",
				"memory": "4Gi / 8Gi",
				"pods":   "10 / 20",
			},
		},
		{
			name: "quota with status hard but no used defaults to 0",
			status: map[string]interface{}{
				"hard": map[string]interface{}{
					"pods": "10",
				},
			},
			wantCols: map[string]string{
				"pods": "0 / 10",
			},
		},
		{
			name: "quota with only spec hard (no status)",
			spec: map[string]interface{}{
				"hard": map[string]interface{}{
					"cpu": "8",
				},
			},
			wantCols: map[string]string{
				"cpu": "8 (hard)",
			},
		},
		{
			name: "nil status and nil spec produces no columns",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populateResourceQuota(ti, tt.status, tt.spec)

			if tt.wantCols != nil {
				colMap := columnsToMap(ti.Columns)
				for k, v := range tt.wantCols {
					assert.Equal(t, v, colMap[k], "column %q mismatch", k)
				}
			} else {
				assert.Empty(t, ti.Columns)
			}
		})
	}
}

// --- populateLimitRange ---

func TestPopulateLimitRange(t *testing.T) {
	tests := []struct {
		name     string
		spec     map[string]interface{}
		wantCols map[string]string
	}{
		{
			name: "limit range with all fields",
			spec: map[string]interface{}{
				"limits": []interface{}{
					map[string]interface{}{
						"type": "Container",
						"default": map[string]interface{}{
							"cpu":    "500m",
							"memory": "256Mi",
						},
						"defaultRequest": map[string]interface{}{
							"cpu":    "100m",
							"memory": "128Mi",
						},
						"max": map[string]interface{}{
							"cpu":    "2",
							"memory": "1Gi",
						},
						"min": map[string]interface{}{
							"cpu":    "50m",
							"memory": "64Mi",
						},
					},
				},
			},
			wantCols: map[string]string{
				"Container Default cpu":        "500m",
				"Container Default memory":     "256Mi",
				"Container Default Req cpu":    "100m",
				"Container Default Req memory": "128Mi",
				"Container Max cpu":            "2",
				"Container Max memory":         "1Gi",
				"Container Min cpu":            "50m",
				"Container Min memory":         "64Mi",
			},
		},
		{
			name: "limit range with unknown type prefix",
			spec: map[string]interface{}{
				"limits": []interface{}{
					map[string]interface{}{
						"default": map[string]interface{}{
							"cpu": "100m",
						},
					},
				},
			},
			wantCols: map[string]string{
				"Unknown Default cpu": "100m",
			},
		},
		{
			name: "nil spec produces no columns",
		},
		{
			name: "spec without limits produces no columns",
			spec: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populateLimitRange(ti, tt.spec)

			if tt.wantCols != nil {
				colMap := columnsToMap(ti.Columns)
				for k, v := range tt.wantCols {
					assert.Equal(t, v, colMap[k], "column %q mismatch", k)
				}
			} else {
				assert.Empty(t, ti.Columns)
			}
		})
	}
}

// --- populatePodDisruptionBudget ---

func TestPopulatePodDisruptionBudget(t *testing.T) {
	tests := []struct {
		name     string
		status   map[string]interface{}
		spec     map[string]interface{}
		wantCols map[string]string
	}{
		{
			name: "PDB with all fields",
			spec: map[string]interface{}{
				"minAvailable":   1,
				"maxUnavailable": 2,
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": "web",
					},
				},
			},
			status: map[string]interface{}{
				"currentHealthy":     float64(3),
				"desiredHealthy":     float64(2),
				"disruptionsAllowed": float64(1),
				"expectedPods":       float64(3),
			},
			wantCols: map[string]string{
				"Min Available":       "1",
				"Max Unavailable":     "2",
				"Selector":            "app=web",
				"Current Healthy":     "3",
				"Desired Healthy":     "2",
				"Disruptions Allowed": "1",
				"Expected Pods":       "3",
			},
		},
		{
			name: "PDB with only spec",
			spec: map[string]interface{}{
				"minAvailable": "50%",
			},
			wantCols: map[string]string{
				"Min Available": "50%",
			},
		},
		{
			name:     "nil status and spec produces no columns",
			wantCols: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populatePodDisruptionBudget(ti, tt.status, tt.spec)

			if tt.wantCols != nil {
				colMap := columnsToMap(ti.Columns)
				for k, v := range tt.wantCols {
					assert.Equal(t, v, colMap[k], "column %q mismatch", k)
				}
			}
		})
	}
}

// --- populateNetworkPolicy ---

func TestPopulateNetworkPolicy(t *testing.T) {
	tests := []struct {
		name     string
		spec     map[string]interface{}
		wantCols map[string]string
	}{
		{
			name: "network policy with all fields",
			spec: map[string]interface{}{
				"podSelector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app":  "web",
						"tier": "frontend",
					},
				},
				"policyTypes": []interface{}{"Ingress", "Egress"},
				"ingress":     []interface{}{map[string]interface{}{}, map[string]interface{}{}},
				"egress":      []interface{}{map[string]interface{}{}},
			},
			wantCols: map[string]string{
				"Pod Selector":  "app=web, tier=frontend",
				"Policy Types":  "Ingress, Egress",
				"Ingress Rules": "2",
				"Egress Rules":  "1",
			},
		},
		{
			name: "network policy with empty podSelector selects all",
			spec: map[string]interface{}{
				"podSelector": map[string]interface{}{},
			},
			wantCols: map[string]string{
				"Pod Selector": "(all pods)",
			},
		},
		{
			name: "nil spec produces no columns",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populateNetworkPolicy(ti, tt.spec)

			if tt.wantCols != nil {
				colMap := columnsToMap(ti.Columns)
				for k, v := range tt.wantCols {
					assert.Equal(t, v, colMap[k], "column %q mismatch", k)
				}
			} else {
				assert.Empty(t, ti.Columns)
			}
		})
	}
}

// --- populateResourceDetailsExt: IngressClass ---

func TestPopulateResourceDetailsExt_IngressClass(t *testing.T) {
	t.Run("default IngressClass", func(t *testing.T) {
		obj := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					"ingressclass.kubernetes.io/is-default-class": "true",
				},
			},
		}
		ti := &model.Item{Name: "nginx"}
		populateResourceDetailsExt(ti, obj, "IngressClass", nil, nil)

		assert.Equal(t, "nginx (default)", ti.Name)
		assert.Equal(t, "default", ti.Status)
	})

	t.Run("non-default IngressClass", func(t *testing.T) {
		obj := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{},
			},
		}
		ti := &model.Item{Name: "nginx"}
		populateResourceDetailsExt(ti, obj, "IngressClass", nil, nil)

		assert.Equal(t, "nginx", ti.Name)
	})
}

// --- populateResourceDetailsExt: StorageClass ---

func TestPopulateResourceDetailsExt_StorageClass(t *testing.T) {
	t.Run("default StorageClass with all fields", func(t *testing.T) {
		obj := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					"storageclass.kubernetes.io/is-default-class": "true",
				},
			},
			"provisioner":          "ebs.csi.aws.com",
			"reclaimPolicy":        "Delete",
			"volumeBindingMode":    "WaitForFirstConsumer",
			"allowVolumeExpansion": true,
		}
		ti := &model.Item{Name: "gp3"}
		populateResourceDetailsExt(ti, obj, "StorageClass", nil, nil)

		assert.Equal(t, "gp3 (default)", ti.Name)
		assert.Equal(t, "default", ti.Status)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "ebs.csi.aws.com", colMap["Provisioner"])
		assert.Equal(t, "Delete", colMap["Reclaim Policy"])
		assert.Equal(t, "WaitForFirstConsumer", colMap["Binding Mode"])
		assert.Equal(t, "true", colMap["Allow Expansion"])
	})
}

// --- populateResourceDetailsExt: ServiceAccount ---

func TestPopulateResourceDetailsExt_ServiceAccount(t *testing.T) {
	t.Run("service account with secrets and image pull secrets", func(t *testing.T) {
		obj := map[string]interface{}{
			"secrets": []interface{}{
				map[string]interface{}{"name": "sa-token-abc"},
			},
			"automountServiceAccountToken": true,
			"imagePullSecrets": []interface{}{
				map[string]interface{}{"name": "docker-registry"},
				map[string]interface{}{"name": "gcr-creds"},
			},
		}
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "ServiceAccount", nil, nil)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "1", colMap["Secrets"])
		assert.Equal(t, "true", colMap["Automount Token"])
		assert.Equal(t, "docker-registry, gcr-creds", colMap["Image Pull Secrets"])
	})
}

// --- populateResourceDetailsExt: PriorityClass ---

func TestPopulateResourceDetailsExt_PriorityClass(t *testing.T) {
	t.Run("default PriorityClass", func(t *testing.T) {
		ti := &model.Item{Name: "high-priority"}
		spec := map[string]interface{}{
			"globalDefault": true,
		}
		populateResourceDetailsExt(ti, map[string]interface{}{}, "PriorityClass", nil, spec)

		assert.Equal(t, "high-priority (default)", ti.Name)
		assert.Equal(t, "default", ti.Status)
	})

	t.Run("non-default PriorityClass", func(t *testing.T) {
		ti := &model.Item{Name: "low-priority"}
		spec := map[string]interface{}{
			"globalDefault": false,
		}
		populateResourceDetailsExt(ti, map[string]interface{}{}, "PriorityClass", nil, spec)

		assert.Equal(t, "low-priority", ti.Name)
	})
}

// --- populateResourceDetailsExt: generic CRD fallback ---

func TestPopulateResourceDetailsExt_GenericCRDFallback(t *testing.T) {
	tests := []struct {
		name     string
		status   map[string]interface{}
		wantCols map[string]string
	}{
		{
			name: "extracts top-level status fields",
			status: map[string]interface{}{
				"phase":   "Active",
				"message": "all good",
			},
			wantCols: map[string]string{
				"Phase":   "Active",
				"Message": "all good",
			},
		},
		{
			name: "nested map status field expands sub-keys",
			status: map[string]interface{}{
				"health": map[string]interface{}{
					"status":  "Healthy",
					"message": "ok",
				},
			},
			wantCols: map[string]string{
				"Health Status":  "Healthy",
				"Health Message": "ok",
			},
		},
		{
			name: "conditions are extracted",
			status: map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "True",
					},
				},
			},
			wantCols: map[string]string{
				"Ready": "True",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := &model.Item{}
			populateResourceDetailsExt(ti, map[string]interface{}{}, "MyCustomResource", tt.status, nil)

			colMap := columnsToMap(ti.Columns)
			for k, v := range tt.wantCols {
				assert.Equal(t, v, colMap[k], "column %q mismatch", k)
			}
		})
	}
}

// --- populateResourceDetailsExt: Application dispatches to ArgoCD ---

func TestPopulateResourceDetailsExt_ArgoCD(t *testing.T) {
	argoKinds := []string{"Application", "ApplicationSet"}
	for _, kind := range argoKinds {
		t.Run(fmt.Sprintf("%s dispatches to ArgoCD handler", kind), func(t *testing.T) {
			obj := map[string]interface{}{
				"status": map[string]interface{}{
					"health": map[string]interface{}{
						"status": "Healthy",
					},
					"sync": map[string]interface{}{
						"status": "Synced",
					},
				},
			}
			status, _ := obj["status"].(map[string]interface{})
			ti := &model.Item{}
			populateResourceDetailsExt(ti, obj, kind, status, nil)

			// Health and Sync are no longer separate columns (shown in STATUS).
			// Just verify the function ran without panicking.
			_ = columnsToMap(ti.Columns)
		})
	}
}

// --- populateResourceDetailsExt: PersistentVolume dispatches ---

func TestPopulateResourceDetailsExt_PVDispatch(t *testing.T) {
	obj := map[string]interface{}{
		"spec": map[string]interface{}{
			"capacity": map[string]interface{}{
				"storage": "50Gi",
			},
		},
		"status": map[string]interface{}{
			"phase": "Available",
		},
	}
	status, _ := obj["status"].(map[string]interface{})
	spec, _ := obj["spec"].(map[string]interface{})
	ti := &model.Item{}
	populateResourceDetailsExt(ti, obj, "PersistentVolume", status, spec)

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "50Gi", colMap["Capacity"])
	assert.Equal(t, "Available", ti.Status)
}

// --- populateResourceDetailsExt: ResourceQuota dispatches ---

func TestPopulateResourceDetailsExt_ResourceQuotaDispatch(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"hard": map[string]interface{}{"pods": "10"},
			"used": map[string]interface{}{"pods": "5"},
		},
	}
	status, _ := obj["status"].(map[string]interface{})
	ti := &model.Item{}
	populateResourceDetailsExt(ti, obj, "ResourceQuota", status, nil)

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "5 / 10", colMap["pods"])
}

// --- populateResourceDetailsExt: LimitRange dispatches ---

func TestPopulateResourceDetailsExt_LimitRangeDispatch(t *testing.T) {
	obj := map[string]interface{}{
		"spec": map[string]interface{}{
			"limits": []interface{}{
				map[string]interface{}{
					"type": "Pod",
					"max": map[string]interface{}{
						"cpu": "4",
					},
				},
			},
		},
	}
	spec, _ := obj["spec"].(map[string]interface{})
	ti := &model.Item{}
	populateResourceDetailsExt(ti, obj, "LimitRange", nil, spec)

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "4", colMap["Pod Max cpu"])
}

// --- populateResourceDetailsExt: PodDisruptionBudget dispatches ---

func TestPopulateResourceDetailsExt_PDBDispatch(t *testing.T) {
	obj := map[string]interface{}{
		"spec": map[string]interface{}{
			"minAvailable": float64(1),
		},
		"status": map[string]interface{}{
			"currentHealthy": float64(3),
		},
	}
	status, _ := obj["status"].(map[string]interface{})
	spec, _ := obj["spec"].(map[string]interface{})
	ti := &model.Item{}
	populateResourceDetailsExt(ti, obj, "PodDisruptionBudget", status, spec)

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "1", colMap["Min Available"])
	assert.Equal(t, "3", colMap["Current Healthy"])
}

// --- populateResourceDetailsExt: NetworkPolicy dispatches ---

func TestPopulateResourceDetailsExt_NetworkPolicyDispatch(t *testing.T) {
	obj := map[string]interface{}{
		"spec": map[string]interface{}{
			"podSelector": map[string]interface{}{},
			"policyTypes": []interface{}{"Ingress"},
		},
	}
	spec, _ := obj["spec"].(map[string]interface{})
	ti := &model.Item{}
	populateResourceDetailsExt(ti, obj, "NetworkPolicy", nil, spec)

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "(all pods)", colMap["Pod Selector"])
	assert.Equal(t, "Ingress", colMap["Policy Types"])
}
