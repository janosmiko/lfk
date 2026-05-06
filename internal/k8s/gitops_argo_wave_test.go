package k8s

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSyncWaveTimeline_ZeroValueIsUsable(t *testing.T) {
	var tl SyncWaveTimeline
	assert.Equal(t, "", tl.AppName)
	assert.Nil(t, tl.Phases)
	assert.Nil(t, tl.LastOperation)
}

func TestUnknownWaveSentinel(t *testing.T) {
	assert.Equal(t, math.MinInt, unknownWave)
}

func TestResolveSyncWaveGVR(t *testing.T) {
	tests := []struct {
		name           string
		group, version string
		kind           string
		want           string // formatted as "group/version/resource"
	}{
		{"core pod", "", "v1", "Pod", "/v1/pods"},
		{"core service", "", "v1", "Service", "/v1/services"},
		{"core ingress override", "networking.k8s.io", "v1", "Ingress", "networking.k8s.io/v1/ingresses"},
		{"apps deployment", "apps", "v1", "Deployment", "apps/v1/deployments"},
		{"network policy plural", "networking.k8s.io", "v1", "NetworkPolicy", "networking.k8s.io/v1/networkpolicies"},
		{"hpa pluralizes via -s", "autoscaling", "v2", "HorizontalPodAutoscaler", "autoscaling/v2/horizontalpodautoscalers"},
		{"endpoints stays plural", "", "v1", "Endpoints", "/v1/endpoints"},
		{"crd", "argoproj.io", "v1alpha1", "Application", "argoproj.io/v1alpha1/applications"},
		{"empty version defaults v1", "", "", "Pod", "/v1/pods"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveSyncWaveGVR(tt.group, tt.version, tt.kind)
			gotS := got.Group + "/" + got.Version + "/" + got.Resource
			assert.Equal(t, tt.want, gotS)
		})
	}
}

func TestGroupSyncWaveResources_OrdersWavesAscending(t *testing.T) {
	in := map[string][]waveResourceWithWave{
		"Sync": {
			{wave: 1, res: SyncWaveResource{Kind: "Service", Name: "api"}},
			{wave: 0, res: SyncWaveResource{Kind: "ConfigMap", Name: "config"}},
			{wave: unknownWave, res: SyncWaveResource{Kind: "Ingress", Name: "ing"}},
			{wave: 1, res: SyncWaveResource{Kind: "Deployment", Name: "api"}},
		},
	}
	phases := groupSyncWaveResources(in)
	// Fixed render order emits all 7 standard phases; locate Sync by name
	// rather than position so the test stays resilient to phase reorderings.
	var sync *SyncWavePhase
	for i := range phases {
		if phases[i].Name == "Sync" {
			sync = &phases[i]
			break
		}
	}
	require.NotNil(t, sync)
	require.Len(t, sync.Waves, 3)
	assert.Equal(t, 0, sync.Waves[0].Wave)
	assert.Equal(t, 1, sync.Waves[1].Wave)
	assert.Equal(t, unknownWave, sync.Waves[2].Wave)
	// Within wave 1, sort by (Group, Kind, Namespace, Name) → Deployment before Service.
	assert.Equal(t, "Deployment", sync.Waves[1].Resources[0].Kind)
	assert.Equal(t, "Service", sync.Waves[1].Resources[1].Kind)
}

func TestGroupSyncWaveResources_FixedPhaseOrder(t *testing.T) {
	in := map[string][]waveResourceWithWave{
		"PostSync":     {{wave: 0, res: SyncWaveResource{Kind: "Job", Name: "post"}}},
		"PreSync":      {{wave: 0, res: SyncWaveResource{Kind: "Job", Name: "pre"}}},
		"Sync":         {{wave: 0, res: SyncWaveResource{Kind: "Pod", Name: "p"}}},
		"SyncFail":     {{wave: 0, res: SyncWaveResource{Kind: "Job", Name: "fail"}}},
		"PostSyncFail": {{wave: 0, res: SyncWaveResource{Kind: "Job", Name: "pfail"}}},
		"BogusPhase":   {{wave: 0, res: SyncWaveResource{Kind: "Pod", Name: "x"}}},
	}
	phases := groupSyncWaveResources(in)
	names := make([]string, len(phases))
	for i, p := range phases {
		names[i] = p.Name
	}
	// All 7 standard ArgoCD phases are emitted in fixed render order; the
	// 5 with content keep their resources, PreDelete/PostDelete (absent
	// from input) come back empty, and BogusPhase is dropped.
	assert.Equal(t, []string{"PreSync", "Sync", "PostSync", "SyncFail", "PostSyncFail", "PreDelete", "PostDelete"}, names)
	// Confirm the empty defaults at the tail have no waves.
	assert.Empty(t, phases[5].Waves) // PreDelete
	assert.Empty(t, phases[6].Waves) // PostDelete
}

func TestGroupSyncWaveResources_AlwaysShowsStandardPhases(t *testing.T) {
	// An empty input map must still produce all 7 standard phases with
	// empty Waves so the renderer can annotate them (none in last operation).
	phases := groupSyncWaveResources(map[string][]waveResourceWithWave{})
	require.Len(t, phases, 7)
	want := []string{"PreSync", "Sync", "PostSync", "SyncFail", "PostSyncFail", "PreDelete", "PostDelete"}
	for i, p := range phases {
		assert.Equal(t, want[i], p.Name)
		assert.Empty(t, p.Waves, "%s should have no waves", p.Name)
	}
}

func TestParseManagedResources(t *testing.T) {
	statusResources := []any{
		map[string]any{
			"group":     "apps",
			"version":   "v1",
			"kind":      "Deployment",
			"namespace": "default",
			"name":      "api",
			"status":    "OutOfSync",
			"health":    map[string]any{"status": "Progressing"},
		},
		map[string]any{
			"version":   "v1", // core API: no group
			"kind":      "ConfigMap",
			"namespace": "default",
			"name":      "config",
			"status":    "Synced",
		},
		map[string]any{
			"kind":      "Service",
			"namespace": "default",
			"name":      "missing-svc",
			"status":    "Missing",
			// no health when Missing
		},
		"not a map", // defensively skipped
	}
	got := parseManagedResources(statusResources)
	require.Len(t, got, 3)

	assert.Equal(t, "apps", got[0].Group)
	assert.Equal(t, "Deployment", got[0].Kind)
	assert.Equal(t, "OutOfSync", got[0].SyncStatus)
	assert.Equal(t, "Progressing", got[0].HealthStatus)
	assert.False(t, got[0].IsHook)

	assert.Equal(t, "", got[1].Group)
	assert.Equal(t, "ConfigMap", got[1].Kind)
	assert.Equal(t, "Synced", got[1].SyncStatus)

	assert.Equal(t, "Missing", got[2].SyncStatus)
	assert.Equal(t, "", got[2].HealthStatus)
}

func TestParseHookResources(t *testing.T) {
	syncResult := map[string]any{
		"resources": []any{
			// PreSync hook
			map[string]any{
				"group":     "batch",
				"version":   "v1",
				"kind":      "Job",
				"namespace": "default",
				"name":      "db-migrate",
				"hookType":  "PreSync",
				"hookPhase": "Succeeded",
				"status":    "Synced",
				"syncPhase": "PreSync",
				"message":   "applied",
			},
			// PostSync hook with error
			map[string]any{
				"group":     "batch",
				"version":   "v1",
				"kind":      "Job",
				"namespace": "default",
				"name":      "smoke-test",
				"hookType":  "PostSync",
				"hookPhase": "Failed",
				"status":    "SyncFailed",
				"syncPhase": "PostSync",
				"message":   "exit code 1",
			},
			// Non-hook (regular synced resource) — must be ignored here
			map[string]any{
				"group":     "apps",
				"version":   "v1",
				"kind":      "Deployment",
				"namespace": "default",
				"name":      "api",
				"status":    "Synced",
				"syncPhase": "Sync",
			},
		},
	}
	hooks := parseHookResources(syncResult)
	require.Len(t, hooks, 2)

	preSync := hooks["PreSync"]
	require.Len(t, preSync, 1)
	assert.True(t, preSync[0].res.IsHook)
	assert.Equal(t, "Succeeded", preSync[0].res.HookPhase)
	assert.Equal(t, "Synced", preSync[0].res.OpStatus)
	assert.Equal(t, "applied", preSync[0].res.Message)

	postSync := hooks["PostSync"]
	require.Len(t, postSync, 1)
	assert.Equal(t, "Failed", postSync[0].res.HookPhase)
	assert.Equal(t, "SyncFailed", postSync[0].res.OpStatus)
}

func TestParseHookResources_NilInput(t *testing.T) {
	assert.Empty(t, parseHookResources(nil))
}

func TestOverlayOpStatus(t *testing.T) {
	managed := []SyncWaveResource{
		{Group: "apps", Kind: "Deployment", Namespace: "default", Name: "api", SyncStatus: "OutOfSync"},
		{Group: "", Kind: "Service", Namespace: "default", Name: "api", SyncStatus: "Missing"},
		{Group: "", Kind: "ConfigMap", Namespace: "default", Name: "config", SyncStatus: "Synced"},
	}
	syncResult := map[string]any{
		"resources": []any{
			// Match: same (group, kind, namespace, name), no hookType.
			map[string]any{
				"group": "apps", "version": "v1", "kind": "Deployment",
				"namespace": "default", "name": "api",
				"status":  "SyncFailed",
				"message": "Deployment.apps \"api\" is invalid",
			},
			// Hook with same name+kind: must NOT overlay onto Service/api.
			map[string]any{
				"group": "", "version": "v1", "kind": "Service",
				"namespace": "default", "name": "api",
				"hookType": "PreSync", "status": "Synced",
			},
		},
	}
	out := overlayOpStatus(managed, syncResult)
	require.Len(t, out, 3)
	// Deployment got the overlay.
	assert.Equal(t, "SyncFailed", out[0].OpStatus)
	assert.Contains(t, out[0].Message, "is invalid")
	// Service did NOT match (its only match in syncResult is a hook).
	assert.Equal(t, "", out[1].OpStatus)
	// ConfigMap had no match at all.
	assert.Equal(t, "", out[2].OpStatus)
}

func TestOverlayOpStatus_NilSyncResult(t *testing.T) {
	managed := []SyncWaveResource{{Kind: "Pod", Name: "p"}}
	out := overlayOpStatus(managed, nil)
	require.Len(t, out, 1)
	assert.Equal(t, "", out[0].OpStatus)
}

func TestParseOperationSummary(t *testing.T) {
	op := map[string]any{
		"phase":      "Succeeded",
		"message":    "successfully synced",
		"startedAt":  "2026-05-05T10:00:00Z",
		"finishedAt": "2026-05-05T10:00:30Z",
		"syncResult": map[string]any{
			"revision": "8a3c4d1f0e2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d",
		},
	}
	got := parseOperationSummary(op)
	require.NotNil(t, got)
	assert.Equal(t, "Succeeded", got.Phase)
	assert.Equal(t, "successfully synced", got.Message)
	assert.Equal(t, "8a3c4d1", got.Revision) // truncated to 7 chars
	assert.False(t, got.StartedAt.IsZero())
	assert.False(t, got.FinishedAt.IsZero())
}

func TestParseOperationSummary_RunningHasZeroFinished(t *testing.T) {
	op := map[string]any{
		"phase":     "Running",
		"startedAt": "2026-05-05T10:00:00Z",
		// no finishedAt
	}
	got := parseOperationSummary(op)
	require.NotNil(t, got)
	assert.Equal(t, "Running", got.Phase)
	assert.True(t, got.FinishedAt.IsZero())
}

func TestParseOperationSummary_NilReturnsNil(t *testing.T) {
	assert.Nil(t, parseOperationSummary(nil))
}

func TestFetchWaveAnnotations_ReadsFromAnnotation(t *testing.T) {
	deploy := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name":      "api",
				"namespace": "default",
				"annotations": map[string]any{
					"argocd.argoproj.io/sync-wave": "5",
				},
			},
		},
	}
	cm := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "config",
				"namespace": "default",
				// no annotation
			},
		},
	}
	dc := newFakeDynClient(deploy, cm)

	c := newFakeClient(nil, dc)
	in := []SyncWaveResource{
		{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "default", Name: "api"},
		{Group: "", Version: "v1", Kind: "ConfigMap", Namespace: "default", Name: "config"},
		// Resource with SyncStatus=Missing must be skipped, no fetch attempted.
		{Group: "", Version: "v1", Kind: "Service", Namespace: "default", Name: "missing", SyncStatus: "Missing"},
	}
	got, err := c.fetchWaveAnnotations(context.Background(), "", in)
	require.NoError(t, err)
	require.Len(t, got, 3)

	assert.Equal(t, 5, got[0].wave)
	assert.Equal(t, "Deployment", got[0].res.Kind)

	// Annotation absent → wave 0 (ArgoCD's default sync-wave).
	assert.Equal(t, 0, got[1].wave)
	// Missing resource was not fetched, lands at unknownWave.
	assert.Equal(t, unknownWave, got[2].wave)
}

func TestFetchWaveAnnotations_MissingAnnotationDefaultsToZero(t *testing.T) {
	// A resource that exists in the cluster but has no sync-wave annotation
	// must land at wave 0, matching ArgoCD's default sync-wave behavior.
	cm := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      "config",
				"namespace": "default",
				// no annotations at all
			},
		},
	}
	dc := newFakeDynClient(cm)
	c := newFakeClient(nil, dc)
	in := []SyncWaveResource{
		{Group: "", Version: "v1", Kind: "ConfigMap", Namespace: "default", Name: "config"},
	}
	got, err := c.fetchWaveAnnotations(context.Background(), "", in)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, 0, got[0].wave, "missing annotation must default to wave 0")
}

func TestFetchWaveAnnotations_PerResourceErrorIsNonFatal(t *testing.T) {
	// Empty fake client: GET will return NotFound for any name.
	dc := newFakeDynClient()
	c := newFakeClient(nil, dc)
	in := []SyncWaveResource{
		{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "default", Name: "ghost"},
	}
	got, err := c.fetchWaveAnnotations(context.Background(), "", in)
	require.NoError(t, err) // top-level call succeeds
	require.Len(t, got, 1)
	assert.Equal(t, unknownWave, got[0].wave)
}

func TestFetchWaveAnnotations_BadAnnotationLandsAtUnknown(t *testing.T) {
	deploy := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name":      "api",
				"namespace": "default",
				"annotations": map[string]any{
					"argocd.argoproj.io/sync-wave": "not-an-int",
				},
			},
		},
	}
	dc := newFakeDynClient(deploy)
	c := newFakeClient(nil, dc)
	in := []SyncWaveResource{
		{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "default", Name: "api"},
	}
	got, _ := c.fetchWaveAnnotations(context.Background(), "", in)
	require.Len(t, got, 1)
	assert.Equal(t, unknownWave, got[0].wave)
}

// TestGetSyncWaveTimelineSkeleton_FastPath verifies the Phase 1 fetch:
// Application GET + parse only. Even when the cluster has annotations on
// the managed resources, the skeleton must NOT consult them — every
// managed resource lands at unknownWave so the renderer surfaces "wave ?"
// while the slow fan-out runs in the background. Loading must be true.
func TestGetSyncWaveTimelineSkeleton_FastPath(t *testing.T) {
	// Note: the deploy/cm have annotations the FULL path would read, but
	// the skeleton must skip the per-resource fan-out entirely. We assert
	// they land at unknownWave to prove the fan-out was skipped.
	deploy := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name":        "api",
				"namespace":   "default",
				"annotations": map[string]any{"argocd.argoproj.io/sync-wave": "1"},
			},
		},
	}
	cm := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1", "kind": "ConfigMap",
			"metadata": map[string]any{
				"name": "config", "namespace": "default",
				"annotations": map[string]any{"argocd.argoproj.io/sync-wave": "0"},
			},
		},
	}
	app := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]any{"name": "my-app", "namespace": "argocd"},
			"status": map[string]any{
				"resources": []any{
					map[string]any{"group": "apps", "version": "v1", "kind": "Deployment", "namespace": "default", "name": "api", "status": "Synced", "health": map[string]any{"status": "Healthy"}},
					map[string]any{"version": "v1", "kind": "ConfigMap", "namespace": "default", "name": "config", "status": "Synced"},
				},
				"operationState": map[string]any{
					"phase":      "Succeeded",
					"startedAt":  "2026-05-05T10:00:00Z",
					"finishedAt": "2026-05-05T10:00:30Z",
					"syncResult": map[string]any{
						"revision": "abcdef0123456789",
						"resources": []any{
							map[string]any{"group": "batch", "version": "v1", "kind": "Job", "namespace": "default", "name": "migrate", "hookType": "PreSync", "hookPhase": "Succeeded", "syncPhase": "PreSync", "status": "Synced"},
						},
					},
				},
			},
		},
	}
	dc := newFakeDynClient(app, deploy, cm)
	c := newFakeClient(nil, dc)

	tl, err := c.GetSyncWaveTimelineSkeleton(context.Background(), "", "argocd", "my-app")
	require.NoError(t, err)
	require.NotNil(t, tl)

	// Loading flag is the headline guarantee — the renderer keys off it.
	assert.True(t, tl.Loading, "skeleton must set Loading: true")

	// Header data is fully populated even on the skeleton path.
	assert.Equal(t, "my-app", tl.AppName)
	assert.Equal(t, "argocd", tl.AppNamespace)
	assert.Equal(t, "Succeeded", tl.LivePhase)
	require.NotNil(t, tl.LastOperation)
	assert.Equal(t, "abcdef0", tl.LastOperation.Revision)

	// All 7 standard phases come back so the renderer can paint the
	// pipeline structure immediately.
	require.Len(t, tl.Phases, 7)
	phaseByName := map[string]SyncWavePhase{}
	for _, p := range tl.Phases {
		phaseByName[p.Name] = p
	}

	// Hooks parsed from operationState (no per-resource GETs needed).
	preSync := phaseByName["PreSync"]
	require.Len(t, preSync.Waves, 1)
	require.Len(t, preSync.Waves[0].Resources, 1)
	assert.True(t, preSync.Waves[0].Resources[0].IsHook)

	// Managed resources land at unknownWave despite the cluster having
	// the annotations the full path would read — proving the skeleton
	// skipped the per-resource fan-out.
	sync := phaseByName["Sync"]
	require.Len(t, sync.Waves, 1, "all managed resources must collapse into a single unknownWave bucket on the skeleton path")
	assert.Equal(t, unknownWave, sync.Waves[0].Wave,
		"skeleton must place all managed resources at unknownWave; annotations are not yet fetched")
	require.Len(t, sync.Waves[0].Resources, 2)
}

func TestGetSyncWaveTimelineSkeleton_AppNotFound(t *testing.T) {
	dc := newFakeDynClient()
	c := newFakeClient(nil, dc)
	_, err := c.GetSyncWaveTimelineSkeleton(context.Background(), "", "argocd", "ghost")
	require.Error(t, err)
}

func TestGetSyncWaveTimeline_FullPath(t *testing.T) {
	deploy := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name":        "api",
				"namespace":   "default",
				"annotations": map[string]any{"argocd.argoproj.io/sync-wave": "1"},
			},
		},
	}
	cm := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1", "kind": "ConfigMap",
			"metadata": map[string]any{
				"name": "config", "namespace": "default",
				"annotations": map[string]any{"argocd.argoproj.io/sync-wave": "0"},
			},
		},
	}
	app := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]any{"name": "my-app", "namespace": "argocd"},
			"status": map[string]any{
				"resources": []any{
					map[string]any{"group": "apps", "version": "v1", "kind": "Deployment", "namespace": "default", "name": "api", "status": "Synced", "health": map[string]any{"status": "Healthy"}},
					map[string]any{"version": "v1", "kind": "ConfigMap", "namespace": "default", "name": "config", "status": "Synced"},
				},
				"operationState": map[string]any{
					"phase":      "Succeeded",
					"startedAt":  "2026-05-05T10:00:00Z",
					"finishedAt": "2026-05-05T10:00:30Z",
					"syncResult": map[string]any{
						"revision": "abcdef0123456789",
						"resources": []any{
							map[string]any{"group": "apps", "version": "v1", "kind": "Deployment", "namespace": "default", "name": "api", "status": "Synced"},
							map[string]any{"group": "batch", "version": "v1", "kind": "Job", "namespace": "default", "name": "migrate", "hookType": "PreSync", "hookPhase": "Succeeded", "syncPhase": "PreSync", "status": "Synced"},
						},
					},
				},
			},
		},
	}
	dc := newFakeDynClient(app, deploy, cm)
	c := newFakeClient(nil, dc)

	tl, err := c.GetSyncWaveTimeline(context.Background(), "", "argocd", "my-app")
	require.NoError(t, err)
	require.NotNil(t, tl)

	assert.False(t, tl.Loading, "full path must clear Loading")
	assert.Equal(t, "my-app", tl.AppName)
	assert.Equal(t, "argocd", tl.AppNamespace)
	assert.Equal(t, "Succeeded", tl.LivePhase)
	require.NotNil(t, tl.LastOperation)
	assert.Equal(t, "Succeeded", tl.LastOperation.Phase)
	assert.Equal(t, "abcdef0", tl.LastOperation.Revision)
	assert.Equal(t, "abcdef0", tl.Revision)

	// All 7 standard ArgoCD phases come back; only PreSync (hook) and
	// Sync (managed) carry resources for this fixture.
	require.Len(t, tl.Phases, 7)
	phaseByName := map[string]SyncWavePhase{}
	for _, p := range tl.Phases {
		phaseByName[p.Name] = p
	}

	preSync := phaseByName["PreSync"]
	require.NotEmpty(t, preSync.Name)
	require.Len(t, preSync.Waves, 1)
	require.Len(t, preSync.Waves[0].Resources, 1)
	assert.True(t, preSync.Waves[0].Resources[0].IsHook)

	sync := phaseByName["Sync"]
	require.Len(t, sync.Waves, 2)
	assert.Equal(t, 0, sync.Waves[0].Wave)
	assert.Equal(t, "ConfigMap", sync.Waves[0].Resources[0].Kind)
	assert.Equal(t, 1, sync.Waves[1].Wave)
	assert.Equal(t, "Deployment", sync.Waves[1].Resources[0].Kind)
	// Operation status overlay landed.
	assert.Equal(t, "Synced", sync.Waves[1].Resources[0].OpStatus)

	// PreDelete/PostDelete come back empty in this fixture — verifies the
	// "always show standard phases" behavior carries through the orchestrator.
	assert.Empty(t, phaseByName["PreDelete"].Waves)
	assert.Empty(t, phaseByName["PostDelete"].Waves)
}

func TestGetSyncWaveTimeline_AppNotFound(t *testing.T) {
	dc := newFakeDynClient()
	c := newFakeClient(nil, dc)
	_, err := c.GetSyncWaveTimeline(context.Background(), "", "argocd", "ghost")
	require.Error(t, err)
}

func TestGetSyncWaveTimeline_EmptyApp(t *testing.T) {
	app := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata":   map[string]any{"name": "empty", "namespace": "argocd"},
		},
	}
	dc := newFakeDynClient(app)
	c := newFakeClient(nil, dc)
	tl, err := c.GetSyncWaveTimeline(context.Background(), "", "argocd", "empty")
	require.NoError(t, err)
	assert.Equal(t, "empty", tl.AppName)
	// Even when the Application has no status, the timeline must still
	// emit the seven standard phases so the overlay's pinned sidebar
	// has something to render. Each phase is empty (no waves), and the
	// renderer surfaces this as "(none in last operation)".
	require.Len(t, tl.Phases, 7)
	assert.Equal(t, "PreSync", tl.Phases[0].Name)
	assert.Equal(t, "PostDelete", tl.Phases[6].Name)
	assert.Empty(t, tl.Phases[0].Waves)
	assert.Nil(t, tl.LastOperation)
	assert.Equal(t, "", tl.LivePhase)
}
