package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- populateResourceDetailsExt: FluxCD non-map condition entry ---

func TestPopulateResourceDetailsExt_FluxCD_NonMapCondition(t *testing.T) {
	// A non-map condition entry in FluxCD status should be skipped (line 76-77).
	obj := map[string]interface{}{
		"spec": map[string]interface{}{},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				"not-a-map",
				map[string]interface{}{
					"type":   "Ready",
					"status": "True",
					"reason": "Success",
				},
			},
		},
	}
	status, _ := obj["status"].(map[string]interface{})
	spec, _ := obj["spec"].(map[string]interface{})
	ti := &model.Item{}
	populateResourceDetailsExt(ti, obj, "GitRepository", status, spec)

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "True", colMap["Ready"])
	assert.Equal(t, "Success", colMap["Reason"])
}

// --- populateResourceDetailsExt: Event dispatching ---

func TestPopulateResourceDetailsExt_Event(t *testing.T) {
	// The Event case should dispatch to populateEvent (line 116-117).
	obj := map[string]interface{}{
		"type":          "Warning",
		"reason":        "FailedScheduling",
		"message":       "0/3 nodes are available",
		"lastTimestamp": "2026-01-15T10:00:00Z",
		"count":         float64(5),
		"involvedObject": map[string]interface{}{
			"kind": "Pod",
			"name": "my-pod",
		},
		"source": map[string]interface{}{
			"component": "default-scheduler",
		},
	}
	ti := &model.Item{}
	populateResourceDetailsExt(ti, obj, "Event", nil, nil)

	assert.Equal(t, "Warning", ti.Status)
	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "Pod/my-pod", colMap["Object"])
	assert.Equal(t, "FailedScheduling", colMap["Reason"])
	assert.Equal(t, "0/3 nodes are available", colMap["Message"])
	assert.Equal(t, "5", colMap["Count"])
	assert.Equal(t, "default-scheduler", colMap["Source"])
}

// --- populateArgoCDApplication: sync errors (non-map resource entry) ---

func TestPopulateArgoCDApplication_SyncResultNonMapResource(t *testing.T) {
	// A non-map resource entry in syncResult should be skipped (line 255-256).
	obj := map[string]interface{}{}
	status := map[string]interface{}{
		"health": map[string]interface{}{
			"status": "Healthy",
		},
		"sync": map[string]interface{}{
			"status": "Synced",
		},
		"operationState": map[string]interface{}{
			"phase": "Succeeded",
			"syncResult": map[string]interface{}{
				"resources": []interface{}{
					"not-a-map",
					map[string]interface{}{
						"kind":    "Deployment",
						"name":    "my-app",
						"status":  "SyncFailed",
						"message": "apply failed",
					},
				},
			},
		},
	}
	ti := &model.Item{}
	populateArgoCDApplication(ti, obj, status, nil, "Application")

	colMap := columnsToMap(ti.Columns)
	assert.Contains(t, colMap["Sync Errors"], "Deployment/my-app: apply failed")
}

// --- populateLimitRange: non-map limit entry ---

func TestPopulateLimitRange_NonMapLimitEntry(t *testing.T) {
	// A non-map limit entry should be skipped (line 448-449).
	spec := map[string]interface{}{
		"limits": []interface{}{
			"not-a-map",
			map[string]interface{}{
				"type": "Pod",
				"max": map[string]interface{}{
					"cpu": "2",
				},
			},
		},
	}

	ti := &model.Item{}
	populateLimitRange(ti, spec)

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "2", colMap["Pod Max cpu"])
	assert.Len(t, ti.Columns, 1, "non-map entry should be skipped")
}

// --- populateResourceDetailsExt: cert-manager non-map condition ---

func TestPopulateResourceDetailsExt_CertManager_NonMapCondition(t *testing.T) {
	obj := map[string]interface{}{
		"spec": map[string]interface{}{
			"secretName": "tls-secret",
		},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				"not-a-map",
				map[string]interface{}{
					"type":   "Ready",
					"status": "True",
				},
			},
		},
	}
	status, _ := obj["status"].(map[string]interface{})
	spec, _ := obj["spec"].(map[string]interface{})
	ti := &model.Item{}
	populateResourceDetailsExt(ti, obj, "Certificate", status, spec)

	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "True", colMap["Ready"])
}
