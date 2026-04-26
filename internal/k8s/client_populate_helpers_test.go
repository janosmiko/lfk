package k8s

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- populateMetadataFields ---

func TestPopulateMetadataFields(t *testing.T) {
	t.Run("extracts labels, finalizers, and annotations", func(t *testing.T) {
		ti := &model.Item{}
		obj := map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"app":                          "nginx",
					"app.kubernetes.io/instance":   "my-release",
					"app.kubernetes.io/managed-by": "Helm",
				},
				"finalizers": []interface{}{
					"kubernetes.io/pvc-protection",
				},
				"annotations": map[string]interface{}{
					"note": "test",
				},
			},
		}

		populateMetadataFields(ti, obj)

		colMap := columnsToMap(ti.Columns)
		// managed-by label should be filtered out.
		assert.NotContains(t, colMap["Labels"], "managed-by")
		assert.Contains(t, colMap["Labels"], "app=nginx")
		assert.Contains(t, colMap["Labels"], "app.kubernetes.io/instance=my-release")
		assert.Equal(t, "kubernetes.io/pvc-protection", colMap["Finalizers"])
		assert.Contains(t, colMap["Annotations"], "note=test")
	})

	t.Run("filters helm.sh/chart label", func(t *testing.T) {
		ti := &model.Item{}
		obj := map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"app":           "web",
					"helm.sh/chart": "my-chart-1.0.0",
				},
			},
		}

		populateMetadataFields(ti, obj)

		colMap := columnsToMap(ti.Columns)
		assert.NotContains(t, colMap["Labels"], "helm.sh/chart")
		assert.Contains(t, colMap["Labels"], "app=web")
	})

	t.Run("nil metadata does nothing", func(t *testing.T) {
		ti := &model.Item{}
		obj := map[string]interface{}{}

		populateMetadataFields(ti, obj)

		assert.Empty(t, ti.Columns)
	})

	t.Run("empty labels and finalizers produce no columns", func(t *testing.T) {
		ti := &model.Item{}
		obj := map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels":     map[string]interface{}{},
				"finalizers": []interface{}{},
			},
		}

		populateMetadataFields(ti, obj)

		assert.Empty(t, ti.Columns)
	})

	t.Run("truncates long annotation values", func(t *testing.T) {
		longValue := ""
		for range 200 {
			longValue += "x"
		}
		ti := &model.Item{}
		obj := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					"long-key": longValue,
				},
			},
		}

		populateMetadataFields(ti, obj)

		colMap := columnsToMap(ti.Columns)
		assert.LessOrEqual(t, len(colMap["Annotations"]), 200)
		assert.Contains(t, colMap["Annotations"], "...")
	})

	t.Run("only managed-by labels produces no labels column", func(t *testing.T) {
		ti := &model.Item{}
		obj := map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"app.kubernetes.io/managed-by": "Helm",
					"helm.sh/chart":                "test-1.0",
				},
			},
		}

		populateMetadataFields(ti, obj)

		for _, kv := range ti.Columns {
			assert.NotEqual(t, "Labels", kv.Key,
				"should not produce a Labels column when all labels are filtered")
		}
	})

	t.Run("multiple finalizers joined with comma", func(t *testing.T) {
		ti := &model.Item{}
		obj := map[string]interface{}{
			"metadata": map[string]interface{}{
				"finalizers": []interface{}{
					"kubernetes.io/pvc-protection",
					"custom-finalizer.io/cleanup",
				},
			},
		}

		populateMetadataFields(ti, obj)

		colMap := columnsToMap(ti.Columns)
		assert.Contains(t, colMap["Finalizers"], "kubernetes.io/pvc-protection")
		assert.Contains(t, colMap["Finalizers"], "custom-finalizer.io/cleanup")
	})
}

// --- populateContainerImages ---

func TestPopulateContainerImages(t *testing.T) {
	t.Run("extracts images from template spec", func(t *testing.T) {
		ti := &model.Item{}
		spec := map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{"image": "nginx:1.25"},
						map[string]interface{}{"image": "sidecar:v2"},
					},
				},
			},
		}

		populateContainerImages(ti, spec)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "nginx:1.25, sidecar:v2", colMap["Images"])
	})

	t.Run("no template does nothing", func(t *testing.T) {
		ti := &model.Item{}
		spec := map[string]interface{}{}

		populateContainerImages(ti, spec)

		assert.Empty(t, ti.Columns)
	})

	t.Run("no containers does nothing", func(t *testing.T) {
		ti := &model.Item{}
		spec := map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{},
			},
		}

		populateContainerImages(ti, spec)

		assert.Empty(t, ti.Columns)
	})

	t.Run("non-map template spec does nothing", func(t *testing.T) {
		ti := &model.Item{}
		spec := map[string]interface{}{
			"template": "not-a-map",
		}

		populateContainerImages(ti, spec)

		assert.Empty(t, ti.Columns)
	})

	t.Run("container without image is skipped", func(t *testing.T) {
		ti := &model.Item{}
		spec := map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{"name": "no-image"},
						map[string]interface{}{"image": "real:v1"},
					},
				},
			},
		}

		populateContainerImages(ti, spec)

		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "real:v1", colMap["Images"])
	})
}

// --- extractContainerResources ---

func TestExtractContainerResources(t *testing.T) {
	t.Run("single container with all resources", func(t *testing.T) {
		containers := []interface{}{
			map[string]interface{}{
				"resources": map[string]interface{}{
					"requests": map[string]interface{}{
						"cpu":    "100m",
						"memory": "128Mi",
					},
					"limits": map[string]interface{}{
						"cpu":    "500m",
						"memory": "256Mi",
					},
				},
			},
		}

		cpuReq, cpuLim, memReq, memLim := extractContainerResources(containers)

		assert.Equal(t, "100m", cpuReq)
		assert.Equal(t, "500m", cpuLim)
		assert.Equal(t, "128Mi", memReq)
		assert.Equal(t, "256Mi", memLim)
	})

	t.Run("multiple containers sum with plus sign", func(t *testing.T) {
		containers := []interface{}{
			map[string]interface{}{
				"resources": map[string]interface{}{
					"requests": map[string]interface{}{"cpu": "100m"},
				},
			},
			map[string]interface{}{
				"resources": map[string]interface{}{
					"requests": map[string]interface{}{"cpu": "200m"},
				},
			},
		}

		cpuReq, _, _, _ := extractContainerResources(containers)

		assert.Equal(t, "100m+200m", cpuReq)
	})

	t.Run("empty containers return empty strings", func(t *testing.T) {
		cpuReq, cpuLim, memReq, memLim := extractContainerResources(nil)

		assert.Empty(t, cpuReq)
		assert.Empty(t, cpuLim)
		assert.Empty(t, memReq)
		assert.Empty(t, memLim)
	})

	t.Run("container without resources skipped", func(t *testing.T) {
		containers := []interface{}{
			map[string]interface{}{"name": "no-resources"},
			map[string]interface{}{
				"resources": map[string]interface{}{
					"requests": map[string]interface{}{"cpu": "50m"},
				},
			},
		}

		cpuReq, _, _, _ := extractContainerResources(containers)

		assert.Equal(t, "50m", cpuReq)
	})

	t.Run("non-map entry skipped", func(t *testing.T) {
		containers := []interface{}{
			"not-a-map",
			map[string]interface{}{
				"resources": map[string]interface{}{
					"limits": map[string]interface{}{"memory": "512Mi"},
				},
			},
		}

		_, _, _, memLim := extractContainerResources(containers)

		assert.Equal(t, "512Mi", memLim)
	})
}

// --- extractTemplateResources ---

func TestExtractTemplateResources(t *testing.T) {
	t.Run("extracts from template.spec.containers", func(t *testing.T) {
		spec := map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"resources": map[string]interface{}{
								"requests": map[string]interface{}{"cpu": "250m", "memory": "64Mi"},
							},
						},
					},
				},
			},
		}

		cpuReq, cpuLim, memReq, memLim := extractTemplateResources(spec)

		assert.Equal(t, "250m", cpuReq)
		assert.Empty(t, cpuLim)
		assert.Equal(t, "64Mi", memReq)
		assert.Empty(t, memLim)
	})

	t.Run("no template returns empty", func(t *testing.T) {
		cpuReq, cpuLim, memReq, memLim := extractTemplateResources(map[string]interface{}{})

		assert.Empty(t, cpuReq)
		assert.Empty(t, cpuLim)
		assert.Empty(t, memReq)
		assert.Empty(t, memLim)
	})

	t.Run("no spec in template returns empty", func(t *testing.T) {
		spec := map[string]interface{}{
			"template": map[string]interface{}{},
		}

		cpuReq, cpuLim, memReq, memLim := extractTemplateResources(spec)

		assert.Empty(t, cpuReq)
		assert.Empty(t, cpuLim)
		assert.Empty(t, memReq)
		assert.Empty(t, memLim)
	})
}

// --- addResourceColumns ---

func TestAddResourceColumns(t *testing.T) {
	t.Run("adds all non-empty columns", func(t *testing.T) {
		ti := &model.Item{}

		addResourceColumns(ti, "100m", "500m", "128Mi", "256Mi")

		assert.Len(t, ti.Columns, 4)
		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "100m", colMap["CPU Req"])
		assert.Equal(t, "500m", colMap["CPU Lim"])
		assert.Equal(t, "128Mi", colMap["Mem Req"])
		assert.Equal(t, "256Mi", colMap["Mem Lim"])
	})

	t.Run("skips empty values", func(t *testing.T) {
		ti := &model.Item{}

		addResourceColumns(ti, "100m", "", "128Mi", "")

		assert.Len(t, ti.Columns, 2)
		colMap := columnsToMap(ti.Columns)
		assert.Equal(t, "100m", colMap["CPU Req"])
		assert.Equal(t, "128Mi", colMap["Mem Req"])
	})

	t.Run("all empty produces no columns", func(t *testing.T) {
		ti := &model.Item{}

		addResourceColumns(ti, "", "", "", "")

		assert.Empty(t, ti.Columns)
	})
}

func TestNextCronFire(t *testing.T) {
	now := time.Date(2026, 4, 26, 4, 33, 0, 0, time.UTC)

	t.Run("every-5-minutes schedule fires at the next 5-minute mark", func(t *testing.T) {
		next, ok := nextCronFire("*/5 * * * *", "", now)
		assert.True(t, ok)
		assert.Equal(t, time.Date(2026, 4, 26, 4, 35, 0, 0, time.UTC), next)
	})

	t.Run("daily-midnight schedule respects America/New_York", func(t *testing.T) {
		next, ok := nextCronFire("0 0 * * *", "America/New_York", now)
		assert.True(t, ok)
		assert.True(t, next.After(now))
		assert.Equal(t, 4, next.UTC().Hour())
		assert.Equal(t, 27, next.UTC().Day())
	})

	t.Run("invalid schedule returns false", func(t *testing.T) {
		_, ok := nextCronFire("not a cron expression", "", now)
		assert.False(t, ok)
	})

	t.Run("invalid timezone returns false", func(t *testing.T) {
		_, ok := nextCronFire("*/5 * * * *", "Not/A_Real_Zone", now)
		assert.False(t, ok)
	})

	t.Run("empty schedule returns false", func(t *testing.T) {
		_, ok := nextCronFire("", "", now)
		assert.False(t, ok)
	})

	t.Run("predefined @hourly schedule", func(t *testing.T) {
		next, ok := nextCronFire("@hourly", "", now)
		assert.True(t, ok)
		assert.Equal(t, time.Date(2026, 4, 26, 5, 0, 0, 0, time.UTC), next)
	})

	t.Run("empty timeZone defaults to UTC even when now is local-zoned", func(t *testing.T) {
		// In production, populateCronJobDetails passes time.Now() — which
		// carries the host's local timezone. A CronJob with an empty
		// spec.timeZone is fired by kube-controller-manager in its own
		// timezone (UTC on every managed control plane). The helper must
		// not silently use the user's local timezone for absolute-hour
		// schedules, or the Next column will be off by the user's UTC
		// offset (e.g. a CET user sees `0 9 * * *` as 9am CET = 8am UTC).
		cet := time.FixedZone("CET", 1*60*60) // UTC+1
		nowInCET := time.Date(2026, 4, 26, 7, 0, 0, 0, cet)

		next, ok := nextCronFire("0 9 * * *", "", nowInCET)
		assert.True(t, ok)
		assert.Equal(t, time.Date(2026, 4, 26, 9, 0, 0, 0, time.UTC), next.UTC(),
			"empty timeZone must be evaluated as UTC (Kubernetes default), "+
				"not whatever zone the caller's now happens to be in")
	})
}
