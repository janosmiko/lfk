package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// ---- Flux enrichments ----

func TestFluxEnrichments_Interval(t *testing.T) {
	for _, kind := range []string{
		"Kustomization", "GitRepository", "HelmRepository", "HelmChart",
		"OCIRepository", "Bucket", "Alert", "Provider", "Receiver",
		"ImageRepository", "ImagePolicy", "ImageUpdateAutomation",
	} {
		t.Run(kind+"/interval present", func(t *testing.T) {
			obj := map[string]any{
				"spec": map[string]any{
					"interval": "5m0s",
				},
			}
			status, _ := obj["status"].(map[string]any)
			spec, _ := obj["spec"].(map[string]any)
			ti := &model.Item{}
			populateResourceDetailsExt(ti, obj, kind, status, spec)

			colMap := columnsToMap(ti.Columns)
			assert.Equal(t, "5m0s", colMap["Interval"], "kind=%s", kind)
		})

		t.Run(kind+"/interval absent emits no column", func(t *testing.T) {
			obj := map[string]any{
				"spec": map[string]any{},
			}
			status, _ := obj["status"].(map[string]any)
			spec, _ := obj["spec"].(map[string]any)
			ti := &model.Item{}
			populateResourceDetailsExt(ti, obj, kind, status, spec)

			colMap := columnsToMap(ti.Columns)
			_, hasInterval := colMap["Interval"]
			assert.False(t, hasInterval, "kind=%s should not emit Interval when absent", kind)
		})
	}
}

func TestFluxEnrichments_Kustomization(t *testing.T) {
	t.Run("source ref emits Source column", func(t *testing.T) {
		obj := map[string]any{
			"spec": map[string]any{
				"sourceRef": map[string]any{
					"kind": "GitRepository",
					"name": "flux-system",
				},
				"path": "./clusters/prod",
			},
		}
		status, _ := obj["status"].(map[string]any)
		spec, _ := obj["spec"].(map[string]any)
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "Kustomization", status, spec)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "GitRepository/flux-system", colMap["Source"])
		assert.Equal(t, "./clusters/prod", colMap["Path"])
	})

	t.Run("empty path is suppressed", func(t *testing.T) {
		obj := map[string]any{
			"spec": map[string]any{
				"sourceRef": map[string]any{
					"kind": "GitRepository",
					"name": "flux-system",
				},
			},
		}
		status, _ := obj["status"].(map[string]any)
		spec, _ := obj["spec"].(map[string]any)
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "Kustomization", status, spec)

		colMap := columnsToMap(ti.Columns)
		_, hasPath := colMap["Path"]
		assert.False(t, hasPath, "empty path should not emit Path column")
	})

	t.Run("missing sourceRef emits no Source column", func(t *testing.T) {
		obj := map[string]any{
			"spec": map[string]any{},
		}
		status, _ := obj["status"].(map[string]any)
		spec, _ := obj["spec"].(map[string]any)
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "Kustomization", status, spec)

		colMap := columnsToMap(ti.Columns)
		_, hasSource := colMap["Source"]
		assert.False(t, hasSource, "missing sourceRef should not emit Source column")
	})

	t.Run("sourceRef with only kind (no name) emits no Source", func(t *testing.T) {
		obj := map[string]any{
			"spec": map[string]any{
				"sourceRef": map[string]any{
					"kind": "GitRepository",
				},
			},
		}
		status, _ := obj["status"].(map[string]any)
		spec, _ := obj["spec"].(map[string]any)
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "Kustomization", status, spec)

		colMap := columnsToMap(ti.Columns)
		_, hasSource := colMap["Source"]
		assert.False(t, hasSource, "sourceRef with missing name should not emit Source")
	})
}

func TestFluxEnrichments_URL(t *testing.T) {
	for _, kind := range []string{"GitRepository", "HelmRepository", "OCIRepository", "Bucket"} {
		t.Run(kind+"/url present emits URL column", func(t *testing.T) {
			obj := map[string]any{
				"spec": map[string]any{
					"url": "https://github.com/fluxcd/flux2",
				},
			}
			status, _ := obj["status"].(map[string]any)
			spec, _ := obj["spec"].(map[string]any)
			ti := &model.Item{}
			populateResourceDetailsExt(ti, obj, kind, status, spec)

			colMap := columnsToMap(ti.Columns)
			assert.Equal(t, "https://github.com/fluxcd/flux2", colMap["URL"], "kind=%s", kind)
		})

		t.Run(kind+"/url absent emits no URL column", func(t *testing.T) {
			obj := map[string]any{
				"spec": map[string]any{},
			}
			status, _ := obj["status"].(map[string]any)
			spec, _ := obj["spec"].(map[string]any)
			ti := &model.Item{}
			populateResourceDetailsExt(ti, obj, kind, status, spec)

			colMap := columnsToMap(ti.Columns)
			_, hasURL := colMap["URL"]
			assert.False(t, hasURL, "kind=%s should not emit URL when absent", kind)
		})
	}
}

func TestFluxEnrichments_HelmChart(t *testing.T) {
	t.Run("chart present emits Chart column", func(t *testing.T) {
		obj := map[string]any{
			"spec": map[string]any{
				"chart": "nginx",
			},
		}
		status, _ := obj["status"].(map[string]any)
		spec, _ := obj["spec"].(map[string]any)
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "HelmChart", status, spec)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "nginx", colMap["Chart"])
	})

	t.Run("chart absent emits no Chart column", func(t *testing.T) {
		obj := map[string]any{
			"spec": map[string]any{},
		}
		status, _ := obj["status"].(map[string]any)
		spec, _ := obj["spec"].(map[string]any)
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "HelmChart", status, spec)

		colMap := columnsToMap(ti.Columns)
		_, hasChart := colMap["Chart"]
		assert.False(t, hasChart, "empty chart should not emit Chart column")
	})
}

// ---- Argo enrichments ----

func TestArgoEnrichments_HealthStatus(t *testing.T) {
	t.Run("health.status emits Health column", func(t *testing.T) {
		obj := map[string]any{
			"status": map[string]any{
				"health": map[string]any{
					"status": "Degraded",
				},
			},
		}
		status, _ := obj["status"].(map[string]any)
		spec, _ := obj["spec"].(map[string]any)
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "Application", status, spec)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "Degraded", colMap["Health"])
	})

	t.Run("health.status absent emits no Health column", func(t *testing.T) {
		obj := map[string]any{
			"status": map[string]any{
				"health": map[string]any{},
			},
		}
		status, _ := obj["status"].(map[string]any)
		spec, _ := obj["spec"].(map[string]any)
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "Application", status, spec)

		colMap := columnsToMap(ti.Columns)
		_, hasHealth := colMap["Health"]
		assert.False(t, hasHealth)
	})

	t.Run("Health column coexists with Health Message column", func(t *testing.T) {
		obj := map[string]any{
			"status": map[string]any{
				"health": map[string]any{
					"status":  "Degraded",
					"message": "container OOM",
				},
			},
		}
		status, _ := obj["status"].(map[string]any)
		spec, _ := obj["spec"].(map[string]any)
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "Application", status, spec)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "Degraded", colMap["Health"])
		assert.Equal(t, "container OOM", colMap["Health Message"])
	})
}

func TestArgoEnrichments_SyncStatus(t *testing.T) {
	t.Run("sync.status emits Sync Status column", func(t *testing.T) {
		obj := map[string]any{
			"status": map[string]any{
				"sync": map[string]any{
					"status":   "OutOfSync",
					"revision": "deadbeef",
				},
			},
		}
		status, _ := obj["status"].(map[string]any)
		spec, _ := obj["spec"].(map[string]any)
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "Application", status, spec)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "OutOfSync", colMap["Sync Status"])
		// Revision still present
		assert.Equal(t, "deadbeef", colMap["Revision"])
	})

	t.Run("sync.status absent emits no Sync Status column", func(t *testing.T) {
		obj := map[string]any{
			"status": map[string]any{
				"sync": map[string]any{
					"revision": "deadbeef",
				},
			},
		}
		status, _ := obj["status"].(map[string]any)
		spec, _ := obj["spec"].(map[string]any)
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "Application", status, spec)

		colMap := columnsToMap(ti.Columns)
		_, hasSyncStatus := colMap["Sync Status"]
		assert.False(t, hasSyncStatus)
	})
}

func TestArgoEnrichments_Project(t *testing.T) {
	t.Run("Application emits Project when non-empty", func(t *testing.T) {
		obj := map[string]any{
			"spec": map[string]any{
				"project": "my-team",
			},
		}
		status, _ := obj["status"].(map[string]any)
		spec, _ := obj["spec"].(map[string]any)
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "Application", status, spec)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "my-team", colMap["Project"])
	})

	t.Run("Application with default project still emits Project", func(t *testing.T) {
		obj := map[string]any{
			"spec": map[string]any{
				"project": "default",
			},
		}
		status, _ := obj["status"].(map[string]any)
		spec, _ := obj["spec"].(map[string]any)
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "Application", status, spec)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "default", colMap["Project"])
	})

	t.Run("Application with empty project emits no Project column", func(t *testing.T) {
		obj := map[string]any{
			"spec": map[string]any{
				"project": "",
			},
		}
		status, _ := obj["status"].(map[string]any)
		spec, _ := obj["spec"].(map[string]any)
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "Application", status, spec)

		colMap := columnsToMap(ti.Columns)
		_, hasProject := colMap["Project"]
		assert.False(t, hasProject)
	})

	t.Run("ApplicationSet emits Project when non-empty", func(t *testing.T) {
		obj := map[string]any{
			"spec": map[string]any{
				"project": "platform",
			},
		}
		status, _ := obj["status"].(map[string]any)
		spec, _ := obj["spec"].(map[string]any)
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "ApplicationSet", status, spec)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "platform", colMap["Project"])
	})
}

// ---- cert-manager enrichments ----

func TestCertManagerEnrichments_Certificate(t *testing.T) {
	t.Run("dnsNames emits DNS Names column", func(t *testing.T) {
		obj := map[string]any{
			"spec": map[string]any{
				"dnsNames": []any{"example.com", "www.example.com", "api.example.com"},
			},
		}
		status, _ := obj["status"].(map[string]any)
		spec, _ := obj["spec"].(map[string]any)
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "Certificate", status, spec)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "example.com, www.example.com, api.example.com", colMap["DNS Names"])
	})

	t.Run("empty dnsNames emits no DNS Names column", func(t *testing.T) {
		obj := map[string]any{
			"spec": map[string]any{
				"dnsNames": []any{},
			},
		}
		status, _ := obj["status"].(map[string]any)
		spec, _ := obj["spec"].(map[string]any)
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "Certificate", status, spec)

		colMap := columnsToMap(ti.Columns)
		_, hasDNS := colMap["DNS Names"]
		assert.False(t, hasDNS)
	})

	t.Run("missing dnsNames emits no DNS Names column", func(t *testing.T) {
		obj := map[string]any{
			"spec": map[string]any{},
		}
		status, _ := obj["status"].(map[string]any)
		spec, _ := obj["spec"].(map[string]any)
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "Certificate", status, spec)

		colMap := columnsToMap(ti.Columns)
		_, hasDNS := colMap["DNS Names"]
		assert.False(t, hasDNS)
	})

	t.Run("issuerRef emits Issuer column", func(t *testing.T) {
		obj := map[string]any{
			"spec": map[string]any{
				"issuerRef": map[string]any{
					"name": "letsencrypt-prod",
					"kind": "ClusterIssuer",
				},
			},
		}
		status, _ := obj["status"].(map[string]any)
		spec, _ := obj["spec"].(map[string]any)
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "Certificate", status, spec)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "letsencrypt-prod", colMap["Issuer"])
	})

	t.Run("empty issuerRef name emits no Issuer column", func(t *testing.T) {
		obj := map[string]any{
			"spec": map[string]any{
				"issuerRef": map[string]any{
					"name": "",
				},
			},
		}
		status, _ := obj["status"].(map[string]any)
		spec, _ := obj["spec"].(map[string]any)
		ti := &model.Item{}
		populateResourceDetailsExt(ti, obj, "Certificate", status, spec)

		colMap := columnsToMap(ti.Columns)
		_, hasIssuer := colMap["Issuer"]
		assert.False(t, hasIssuer)
	})
}

func TestCertManagerEnrichments_NonCertificateKinds(t *testing.T) {
	// DNS Names and Issuer should NOT appear for other cert-manager kinds.
	for _, kind := range []string{"Issuer", "ClusterIssuer", "CertificateRequest", "Order", "Challenge"} {
		t.Run(kind+"/no DNS Names or Issuer columns", func(t *testing.T) {
			obj := map[string]any{
				"spec": map[string]any{
					"dnsNames": []any{"example.com"},
					"issuerRef": map[string]any{
						"name": "letsencrypt-prod",
					},
				},
			}
			status, _ := obj["status"].(map[string]any)
			spec, _ := obj["spec"].(map[string]any)
			ti := &model.Item{}
			populateResourceDetailsExt(ti, obj, kind, status, spec)

			colMap := columnsToMap(ti.Columns)
			_, hasDNS := colMap["DNS Names"]
			_, hasIssuer := colMap["Issuer"]
			assert.False(t, hasDNS, "kind=%s should not emit DNS Names", kind)
			assert.False(t, hasIssuer, "kind=%s should not emit Issuer", kind)
		})
	}
}

// ---- CONDITIONS detail section is populated for the details pane ----

func TestConditionsDetail_CertManagerCertificate(t *testing.T) {
	// A Certificate with multiple conditions: the compact column summary must
	// still surface the Ready condition, and every condition must also be
	// captured on ti.Conditions for the CONDITIONS detail section (issue #340).
	obj := map[string]any{
		"status": map[string]any{
			"conditions": []any{
				map[string]any{
					"type": "Ready", "status": "True", "reason": "Ready",
					"message":            "Certificate is up to date and has not expired",
					"lastTransitionTime": "2026-04-01T09:55:54Z",
				},
				map[string]any{
					"type": "Issuing", "status": "False", "reason": "Issued",
					"message": "issuance succeeded",
				},
			},
		},
		"spec": map[string]any{},
	}
	ti := &model.Item{}
	populateResourceDetails(ti, obj, "Certificate")

	// Compact summary column (unchanged behavior).
	colMap := columnsToMap(ti.Columns)
	assert.Equal(t, "True", colMap["Ready"])

	// Full detail section: every condition, exactly once (no double-populate).
	if assert.Len(t, ti.Conditions, 2) {
		assert.Equal(t, "Ready", ti.Conditions[0].Type)
		assert.Equal(t, "Issuing", ti.Conditions[1].Type)
		assert.False(t, ti.Conditions[0].LastTransitionTime.IsZero())
	}
}

func TestConditionsDetail_FluxNoReadyFallbackNoDuplicate(t *testing.T) {
	// A Flux resource whose conditions lack a Ready entry falls back to the
	// generic column summary. The detail section must still contain each
	// condition exactly once (appendAllConditions runs once, not per branch).
	obj := map[string]any{
		"spec": map[string]any{},
		"status": map[string]any{
			"conditions": []any{
				map[string]any{"type": "Reconciling", "status": "True", "reason": "Progressing"},
				map[string]any{"type": "Stalled", "status": "False"},
			},
		},
	}
	ti := &model.Item{}
	populateResourceDetails(ti, obj, "Kustomization")

	assert.Len(t, ti.Conditions, 2, "each condition must appear exactly once")
}

func TestConditionsDetail_AcrossKinds(t *testing.T) {
	// Conditions are populated centrally for every kind, including handlers
	// that previously only built their own summary columns (ArgoCD Application,
	// HPA) and core workloads that never surfaced conditions before (issue #340).
	kinds := []struct {
		kind  string
		types []string
	}{
		{"Application", []string{"Healthy", "SyncError"}},    // ArgoCD
		{"HorizontalPodAutoscaler", []string{"AbleToScale"}}, // HPA
		{"Workflow", []string{"Completed"}},                  // Argo Workflows
		{"Pod", []string{"PodScheduled", "Ready"}},           // core workload
		{"FooBar", []string{"Ready"}},                        // unknown CRD (default branch)
	}
	for _, tc := range kinds {
		t.Run(tc.kind, func(t *testing.T) {
			conds := make([]any, len(tc.types))
			for i, ct := range tc.types {
				conds[i] = map[string]any{"type": ct, "status": "True"}
			}
			obj := map[string]any{
				"spec":   map[string]any{},
				"status": map[string]any{"conditions": conds},
			}
			ti := &model.Item{}
			populateResourceDetails(ti, obj, tc.kind)

			require.Len(t, ti.Conditions, len(tc.types))
			for i, ct := range tc.types {
				assert.Equal(t, ct, ti.Conditions[i].Type)
			}
		})
	}
}
