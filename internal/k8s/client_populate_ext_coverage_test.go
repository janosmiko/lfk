package k8s

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- populateResourceDetailsExt: IngressClass ---

func TestPopulateExt_IngressClassCoverage(t *testing.T) {
	t.Run("default ingress class", func(t *testing.T) {
		obj := map[string]any{
			"metadata": map[string]any{
				"annotations": map[string]any{
					"ingressclass.kubernetes.io/is-default-class": "true",
				},
			},
		}
		ti := &model.Item{Name: "nginx"}
		populateResourceDetailsExt(ti, obj, "IngressClass", nil, nil)
		assert.Equal(t, "nginx (default)", ti.Name)
		assert.Equal(t, "default", ti.Status)
	})

	t.Run("non-default ingress class", func(t *testing.T) {
		obj := map[string]any{
			"metadata": map[string]any{
				"annotations": map[string]any{
					"ingressclass.kubernetes.io/is-default-class": "false",
				},
			},
		}
		ti := &model.Item{Name: "traefik"}
		populateResourceDetailsExt(ti, obj, "IngressClass", nil, nil)
		assert.Equal(t, "traefik", ti.Name)
	})
}

// --- populateResourceDetailsExt: StorageClass ---

func TestPopulateExt_StorageClassCoverage(t *testing.T) {
	t.Run("default storage class with all fields", func(t *testing.T) {
		obj := map[string]any{
			"metadata": map[string]any{
				"annotations": map[string]any{
					"storageclass.kubernetes.io/is-default-class": "true",
				},
			},
			"provisioner":          "kubernetes.io/gce-pd",
			"reclaimPolicy":        "Delete",
			"volumeBindingMode":    "WaitForFirstConsumer",
			"allowVolumeExpansion": true,
		}
		ti := &model.Item{Name: "standard"}
		populateResourceDetailsExt(ti, obj, "StorageClass", nil, nil)

		assert.Equal(t, "standard (default)", ti.Name)
		assert.Equal(t, "default", ti.Status)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "kubernetes.io/gce-pd", colMap["Provisioner"])
		assert.Equal(t, "Delete", colMap["Reclaim Policy"])
		assert.Equal(t, "WaitForFirstConsumer", colMap["Binding Mode"])
		assert.Equal(t, "true", colMap["Allow Expansion"])
	})

	t.Run("storage class without default annotation", func(t *testing.T) {
		obj := map[string]any{
			"metadata":    map[string]any{},
			"provisioner": "ebs.csi.aws.com",
		}
		ti := &model.Item{Name: "gp3"}
		populateResourceDetailsExt(ti, obj, "StorageClass", nil, nil)
		assert.Equal(t, "gp3", ti.Name)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "ebs.csi.aws.com", colMap["Provisioner"])
	})
}

// --- populateResourceDetailsExt: ServiceAccount ---

func TestPopulateExt_ServiceAccountCoverage(t *testing.T) {
	t.Run("service account with secrets and image pull secrets", func(t *testing.T) {
		obj := map[string]any{
			"secrets": []any{
				map[string]any{"name": "sa-token-abc"},
			},
			"automountServiceAccountToken": true,
			"imagePullSecrets": []any{
				map[string]any{"name": "registry-creds"},
				map[string]any{"name": "docker-hub"},
			},
		}
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "ServiceAccount", nil, nil)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "1", colMap["Secrets"])
		assert.Equal(t, "true", colMap["Automount Token"])
		assert.Contains(t, colMap["Image Pull Secrets"], "registry-creds")
		assert.Contains(t, colMap["Image Pull Secrets"], "docker-hub")
	})

	t.Run("service account with automount false", func(t *testing.T) {
		obj := map[string]any{
			"automountServiceAccountToken": false,
		}
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "ServiceAccount", nil, nil)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "false", colMap["Automount Token"])
	})
}

// --- populateResourceDetailsExt: PriorityClass ---

func TestPopulateExt_PriorityClassCoverage(t *testing.T) {
	t.Run("default priority class", func(t *testing.T) {
		obj := map[string]any{
			"spec": map[string]any{
				"globalDefault": true,
			},
		}
		ti := &model.Item{Name: "high-priority"}
		spec := obj["spec"].(map[string]any)
		populateResourceDetailsExt(ti, obj, "PriorityClass", nil, spec)
		assert.Equal(t, "high-priority (default)", ti.Name)
		assert.Equal(t, "default", ti.Status)
	})

	t.Run("non-default priority class", func(t *testing.T) {
		obj := map[string]any{
			"spec": map[string]any{
				"globalDefault": false,
			},
		}
		ti := &model.Item{Name: "low-priority"}
		spec := obj["spec"].(map[string]any)
		populateResourceDetailsExt(ti, obj, "PriorityClass", nil, spec)
		assert.Equal(t, "low-priority", ti.Name)
	})
}

// --- populateResourceDetailsExt: FluxCD ---

func TestPopulateExt_FluxCDCoverage(t *testing.T) {
	t.Run("kustomization with Ready condition and revision", func(t *testing.T) {
		obj := map[string]any{
			"spec": map[string]any{
				"suspend": false,
			},
			"status": map[string]any{
				"conditions": []any{
					map[string]any{
						"type":   "Ready",
						"status": "True",
						"reason": "ReconciliationSucceeded",
					},
				},
				"lastAppliedRevision": "main@sha1:abc123def456ghi",
			},
		}
		ti := &model.Item{}
		status := obj["status"].(map[string]any)
		spec := obj["spec"].(map[string]any)
		populateResourceDetailsExt(ti, obj, "Kustomization", status, spec)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "True", colMap["Ready"])
		assert.Equal(t, "ReconciliationSucceeded", colMap["Reason"])
		assert.Equal(t, "main@sha1:ab", colMap["Revision"])
	})

	t.Run("kustomization suspended", func(t *testing.T) {
		obj := map[string]any{
			"spec": map[string]any{
				"suspend": true,
			},
			"status": map[string]any{},
		}
		ti := &model.Item{}
		status := obj["status"].(map[string]any)
		spec := obj["spec"].(map[string]any)
		populateResourceDetailsExt(ti, obj, "Kustomization", status, spec)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "True", colMap["Suspended"])
	})

	t.Run("git repository with artifact revision fallback", func(t *testing.T) {
		obj := map[string]any{
			"status": map[string]any{
				"artifact": map[string]any{
					"revision": "sha256:abcdef123456",
				},
			},
		}
		ti := &model.Item{}
		status := obj["status"].(map[string]any)
		populateResourceDetailsExt(ti, obj, "GitRepository", status, nil)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "sha256:abcde", colMap["Revision"])
	})

	t.Run("flux resource with short revision (no truncation needed)", func(t *testing.T) {
		obj := map[string]any{
			"status": map[string]any{
				"lastAppliedRevision": "short",
			},
		}
		ti := &model.Item{}
		status := obj["status"].(map[string]any)
		populateResourceDetailsExt(ti, obj, "HelmChart", status, nil)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "short", colMap["Revision"])
	})

	t.Run("flux Ready False with message", func(t *testing.T) {
		obj := map[string]any{
			"status": map[string]any{
				"conditions": []any{
					map[string]any{
						"type":    "Ready",
						"status":  "False",
						"message": "reconciliation failed",
					},
				},
			},
		}
		ti := &model.Item{}
		status := obj["status"].(map[string]any)
		populateResourceDetailsExt(ti, obj, "HelmRepository", status, nil)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "False", colMap["Ready"])
		assert.Equal(t, "reconciliation failed", colMap["Message"])
	})
}

// --- populateResourceDetailsExt: cert-manager ---

func TestPopulateExt_CertManagerCoverage(t *testing.T) {
	t.Run("certificate with Ready, expiry, and secret", func(t *testing.T) {
		obj := map[string]any{
			"spec": map[string]any{
				"secretName": "tls-cert",
			},
			"status": map[string]any{
				"conditions": []any{
					map[string]any{
						"type":   "Ready",
						"status": "True",
						"reason": "Ready",
					},
				},
				"notAfter":    "2027-01-01T00:00:00Z",
				"renewalTime": "2026-12-01T00:00:00Z",
			},
		}
		ti := &model.Item{}
		status := obj["status"].(map[string]any)
		spec := obj["spec"].(map[string]any)
		populateResourceDetailsExt(ti, obj, "Certificate", status, spec)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "True", colMap["Ready"])
		assert.Equal(t, "Ready", colMap["Reason"])
		assert.Equal(t, "2027-01-01T00:00:00Z", colMap["Expires"])
		assert.Equal(t, "2026-12-01T00:00:00Z", colMap["Renewal"])
		assert.Equal(t, "tls-cert", colMap["Secret"])
	})

	t.Run("certificate not Ready with message", func(t *testing.T) {
		obj := map[string]any{
			"status": map[string]any{
				"conditions": []any{
					map[string]any{
						"type":    "Ready",
						"status":  "False",
						"message": "certificate issuance failed",
					},
				},
			},
		}
		ti := &model.Item{}
		status := obj["status"].(map[string]any)
		populateResourceDetailsExt(ti, obj, "CertificateRequest", status, nil)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "False", colMap["Ready"])
		assert.Equal(t, "certificate issuance failed", colMap["Message"])
	})
}

// --- populateResourceDetailsExt: default (unknown CRD) ---

func TestPopulateExt_DefaultCRDCoverage(t *testing.T) {
	t.Run("unknown CRD with top-level status fields", func(t *testing.T) {
		status := map[string]any{
			"phase":   "Active",
			"message": "all good",
			"reason":  "Reconciled",
		}
		obj := map[string]any{
			"status": status,
		}
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "UnknownCRD", status, nil)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "Active", colMap["Phase"])
		assert.Equal(t, "all good", colMap["Message"])
		assert.Equal(t, "Reconciled", colMap["Reason"])
	})

	t.Run("unknown CRD with map status fields", func(t *testing.T) {
		status := map[string]any{
			"health": map[string]any{
				"status":  "Healthy",
				"message": "all components OK",
			},
		}
		obj := map[string]any{
			"status": status,
		}
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "UnknownCRD", status, nil)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "Healthy", colMap["Health Status"])
		assert.Equal(t, "all components OK", colMap["Health Message"])
	})

	t.Run("unknown CRD with conditions falls back to generic extraction", func(t *testing.T) {
		status := map[string]any{
			"conditions": []any{
				map[string]any{
					"type":   "Ready",
					"status": "True",
				},
			},
		}
		obj := map[string]any{
			"status": status,
		}
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "UnknownCRD", status, nil)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "True", colMap["Ready"])
	})

	t.Run("nil status does nothing", func(t *testing.T) {
		obj := map[string]any{}
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "UnknownCRD", nil, nil)
		assert.Empty(t, ti.Columns)
	})
}

// --- populateResourceDetailsExt: Event (basic) ---

func TestPopulateExt_EventCoverage(t *testing.T) {
	t.Run("event populates status and columns", func(t *testing.T) {
		obj := map[string]any{
			"type":    "Warning",
			"reason":  "FailedScheduling",
			"message": "0/3 nodes are available",
			"involvedObject": map[string]any{
				"kind": "Pod",
				"name": "my-pod",
			},
			"count":          float64(5),
			"lastTimestamp":  "2026-03-22T10:00:00Z",
			"firstTimestamp": "2026-03-22T09:00:00Z",
		}
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "Event", nil, nil)

		// populateEvent sets ti.Status from obj["type"], not a column.
		assert.Equal(t, "Warning", ti.Status)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "FailedScheduling", colMap["Reason"])
		assert.Contains(t, colMap["Message"], "0/3 nodes")
		assert.Equal(t, "Pod/my-pod", colMap["Object"])
		assert.Equal(t, fmt.Sprintf("%d", 5), colMap["Count"])
	})
}
