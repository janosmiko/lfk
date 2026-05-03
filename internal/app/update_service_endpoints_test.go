package app

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// findCol returns the first column matching key, or "" when absent.
func findCol(item *model.Item, key string) string {
	for _, kv := range item.Columns {
		if kv.Key == key {
			return kv.Value
		}
	}
	return ""
}

func TestUpdatePreviewServiceEndpointsLoaded_InjectsRollupColumns(t *testing.T) {
	m := Model{
		requestGen: 7,
		namespace:  "default",
		middleItems: []model.Item{
			{Name: "my-svc", Namespace: "default"},
		},
	}
	msg := previewServiceEndpointsLoadedMsg{
		gen: 7, ctx: "kctx", ns: "default", name: "my-svc",
		data: &k8s.ServiceEndpoints{
			Ready:    2,
			NotReady: 1,
			Block:    "10.0.0.1 → pod/foo on node-a\n10.0.0.2 → pod/bar on node-b\n10.0.0.3 → pod/baz on node-c (NotReady)",
		},
	}
	result := m.updatePreviewServiceEndpointsLoaded(msg)

	assert.Equal(t, "2 ready / 1 not ready",
		findCol(&result.middleItems[0], "Backing Endpoints"),
		"summary line records ready / not-ready totals so a broken Service is obvious in the table view")
	assert.Contains(t, findCol(&result.middleItems[0], "Endpoints"),
		"10.0.0.3 → pod/baz on node-c (NotReady)",
		"per-endpoint multi-line block is injected as the existing Endpoints renderer key — same column the Endpoints/EndpointSlices preview uses, so the layout matches")
}

func TestUpdatePreviewServiceEndpointsLoaded_StaleGenIgnored(t *testing.T) {
	// Stale response from a prior hover must NOT touch the items — the
	// user has already moved on. Mirrors the secret-data handler's
	// stale-gen check.
	m := Model{
		requestGen:  10,
		namespace:   "default",
		middleItems: []model.Item{{Name: "my-svc", Namespace: "default"}},
	}
	msg := previewServiceEndpointsLoadedMsg{
		gen: 1, // stale
		ctx: "kctx", ns: "default", name: "my-svc",
		data: &k8s.ServiceEndpoints{Ready: 99},
	}
	result := m.updatePreviewServiceEndpointsLoaded(msg)
	assert.Empty(t, findCol(&result.middleItems[0], "Backing Endpoints"),
		"stale response must not inject columns")
}

func TestUpdatePreviewServiceEndpointsLoaded_ErrorIgnored(t *testing.T) {
	// Failed fetches must not write anything to the items — keep the
	// previous (possibly empty) state until the next successful fetch.
	m := Model{
		requestGen:  5,
		middleItems: []model.Item{{Name: "my-svc", Namespace: "default"}},
		namespace:   "default",
	}
	msg := previewServiceEndpointsLoadedMsg{
		gen: 5, ctx: "kctx", ns: "default", name: "my-svc",
		err: errors.New("boom"),
	}
	result := m.updatePreviewServiceEndpointsLoaded(msg)
	assert.Empty(t, findCol(&result.middleItems[0], "Backing Endpoints"),
		"failed fetches must not inject anything")
}

// Regression for the stale-cache bug the user hit: deleting both pods
// behind a Service used to leave the rollup showing them as ready
// because the cached ServiceEndpoints was returned on the next hover.
// Cache was removed; this test pins that the handler injects whatever
// the latest message carries — so a fresh fetch after pod churn always
// overwrites the prior column with the new state.
func TestUpdatePreviewServiceEndpointsLoaded_FreshFetchReplacesStaleData(t *testing.T) {
	item := model.Item{
		Name: "my-svc", Namespace: "default",
		Columns: []model.KeyValue{
			{Key: "Backing Endpoints", Value: "2 ready / 0 not ready"},
			{Key: "Endpoints", Value: "10.0.0.1 → pod/old-1\n10.0.0.2 → pod/old-2"},
		},
	}
	m := Model{
		requestGen:  4,
		namespace:   "default",
		middleItems: []model.Item{item},
	}
	// Pods just got recreated; the new EndpointSlice has them as not-
	// ready while they pass startup probes. The fresh fetch must
	// replace the prior 2-ready snapshot.
	msg := previewServiceEndpointsLoadedMsg{
		gen: 4, ctx: "kctx", ns: "default", name: "my-svc",
		data: &k8s.ServiceEndpoints{
			Ready: 0, NotReady: 2,
			Block: "10.0.0.3 → pod/new-1 (NotReady)\n10.0.0.4 → pod/new-2 (NotReady)",
		},
	}
	result := m.updatePreviewServiceEndpointsLoaded(msg)
	assert.Equal(t, "0 ready / 2 not ready",
		findCol(&result.middleItems[0], "Backing Endpoints"),
		"fresh fetch after pod churn must overwrite the stale ready count")
	assert.NotContains(t, findCol(&result.middleItems[0], "Endpoints"), "pod/old-1",
		"old pod entries must not survive the rebuild")
}

func TestUpdatePreviewServiceEndpointsLoaded_RefreshOverwritesPriorColumns(t *testing.T) {
	// Second roundtrip after pods come and go must replace the prior
	// per-endpoint block, not append a duplicate. Tests the
	// strip-then-append pattern in injectServiceEndpointColumns.
	item := model.Item{
		Name: "my-svc", Namespace: "default",
		Columns: []model.KeyValue{
			{Key: "Backing Endpoints", Value: "1 ready / 0 not ready"},
			{Key: "Endpoints", Value: "10.0.0.1 → pod/old"},
		},
	}
	m := Model{
		requestGen:  4,
		namespace:   "default",
		middleItems: []model.Item{item},
	}
	msg := previewServiceEndpointsLoadedMsg{
		gen: 4, ctx: "kctx", ns: "default", name: "my-svc",
		data: &k8s.ServiceEndpoints{
			Ready: 2, Block: "10.0.0.1 → pod/new\n10.0.0.2 → pod/also-new",
		},
	}
	result := m.updatePreviewServiceEndpointsLoaded(msg)

	got := findCol(&result.middleItems[0], "Endpoints")
	assert.Contains(t, got, "pod/new")
	assert.Contains(t, got, "pod/also-new")
	assert.NotContains(t, got, "pod/old", "prior Endpoints value must be replaced, not appended to")

	// Count occurrences of the summary key — must be exactly one.
	var summaryCount int
	for _, kv := range result.middleItems[0].Columns {
		if kv.Key == "Backing Endpoints" {
			summaryCount++
		}
	}
	assert.Equal(t, 1, summaryCount, "summary KV must not be duplicated on refresh")
}

func TestUpdatePreviewServiceEndpointsLoaded_NamespaceMismatchSkipped(t *testing.T) {
	// Two services with the same name in different namespaces — the
	// rollup arriving for kube-system/my-svc must not be injected into
	// the default/my-svc row. Mirrors the namespace gate in the secret
	// data handler.
	m := Model{
		requestGen: 2,
		namespace:  "default",
		middleItems: []model.Item{
			{Name: "my-svc", Namespace: "default"},
			{Name: "my-svc", Namespace: "kube-system"},
		},
	}
	msg := previewServiceEndpointsLoadedMsg{
		gen: 2, ctx: "kctx", ns: "kube-system", name: "my-svc",
		data: &k8s.ServiceEndpoints{Ready: 5, Block: "10.0.0.5 → pod/sys"},
	}
	result := m.updatePreviewServiceEndpointsLoaded(msg)

	assert.Empty(t, findCol(&result.middleItems[0], "Backing Endpoints"),
		"default namespace row must not pick up kube-system's rollup")
	assert.Equal(t, "5 ready / 0 not ready",
		findCol(&result.middleItems[1], "Backing Endpoints"),
		"kube-system row gets its own rollup")
}

func TestIsServiceWithoutEndpoints_HeadlessServiceSkipped(t *testing.T) {
	headless := &model.Item{
		Columns: []model.KeyValue{
			{Key: "Type", Value: "ClusterIP"},
			{Key: "Cluster IP", Value: "None"},
		},
	}
	assert.True(t, isServiceWithoutEndpoints(headless),
		"Headless services (Cluster IP=None) have no backing EndpointSlices to roll up — skip the fetch")
}

func TestIsServiceWithoutEndpoints_ExternalNameSkipped(t *testing.T) {
	external := &model.Item{
		Columns: []model.KeyValue{
			{Key: "Type", Value: "ExternalName"},
		},
	}
	assert.True(t, isServiceWithoutEndpoints(external),
		"ExternalName services resolve via DNS, not EndpointSlices — skip the fetch")
}

func TestIsServiceWithoutEndpoints_NormalServiceFetched(t *testing.T) {
	normal := &model.Item{
		Columns: []model.KeyValue{
			{Key: "Type", Value: "ClusterIP"},
			{Key: "Cluster IP", Value: "10.96.0.1"},
		},
	}
	assert.False(t, isServiceWithoutEndpoints(normal),
		"a normal ClusterIP service with a real IP must NOT be skipped — that's the whole point of the rollup")
}
