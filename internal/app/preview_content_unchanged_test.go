package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// newPreviewSkipTestModel builds a Model at LevelResources hovering
// web-0 at resourceVersion "100".
func newPreviewSkipTestModel(t *testing.T) Model {
	t.Helper()
	m := newLoadResourcesTestModel(t)
	m.cacheFingerprints = make(map[string]string)
	m.previewContentFingerprints = make(map[string]string)
	m.middleItems = []model.Item{withPodRV("web-0", "100")}
	m.setCursor(0)
	return m
}

func withPodRV(name, rv string) model.Item {
	return model.Item{
		Name:      name,
		Namespace: "default",
		Kind:      "Pod",
		Raw: map[string]any{
			"metadata": map[string]any{
				"name":            name,
				"namespace":       "default",
				"resourceVersion": rv,
			},
		},
	}
}

// previewFetchKinds executes every cmd in the batch loadPreviewResources
// returns and reports which sub-fetch message types were present.
func previewFetchKinds(cmd tea.Cmd) map[string]bool {
	kinds := make(map[string]bool)
	for _, c := range flattenBatch(cmd) {
		switch c().(type) {
		case containersLoadedMsg:
			kinds["containers"] = true
		case previewYAMLLoadedMsg:
			kinds["yaml"] = true
		case metricsLoadedMsg:
			kinds["metrics"] = true
		case previewEventsLoadedMsg:
			kinds["events"] = true
		}
	}
	return kinds
}

// Case (a): unchanged RV on the second watch tick skips containers/YAML.
// Metrics and events never bump the pod's RV, so they still fire every tick.
func TestPreviewSkip_SameRVOnWatchTick(t *testing.T) {
	t.Parallel()
	m := newPreviewSkipTestModel(t)
	m.fullYAMLPreview = true
	m.suppressBgtasks = true
	m.scheduler.StartWorkers()
	t.Cleanup(m.scheduler.StopWorkers)

	first := previewFetchKinds(m.loadPreviewResources())
	assert.True(t, first["containers"], "first tick (no fingerprint yet) must fetch containers")
	assert.True(t, first["yaml"], "first tick (no fingerprint yet) must fetch YAML")

	second := previewFetchKinds(m.loadPreviewResources())
	assert.False(t, second["containers"], "second tick with unchanged RV must skip containers re-fetch")
	assert.False(t, second["yaml"], "second tick with unchanged RV must skip YAML re-fetch")
	assert.True(t, second["metrics"], "metrics must still fetch every tick regardless of RV")
	assert.True(t, second["events"], "events must still fetch every tick regardless of RV")
}

// TestPreviewSkip_ManualRefreshAlwaysFetches covers case (b): a shift+r
// style manual refresh (suppressBgtasks false) must never skip, even when
// the RV fingerprint from a prior watch tick matches.
func TestPreviewSkip_ManualRefreshAlwaysFetches(t *testing.T) {
	t.Parallel()
	m := newPreviewSkipTestModel(t)
	m.fullYAMLPreview = true
	m.scheduler.StartWorkers()
	t.Cleanup(m.scheduler.StopWorkers)

	m.suppressBgtasks = true
	previewFetchKinds(m.loadPreviewResources()) // primes the fingerprint at RV 100

	m.suppressBgtasks = false // shift+r / drill-in: never suppressed
	kinds := previewFetchKinds(m.loadPreviewResources())
	assert.True(t, kinds["containers"], "manual refresh must always fetch containers")
	assert.True(t, kinds["yaml"], "manual refresh must always fetch YAML")
}

// TestPreviewSkip_CursorMovedAlwaysFetches covers case (c): moving the
// cursor to a different pod between ticks must not skip, since that pod has
// no recorded fingerprint under its own SelectionKey.
func TestPreviewSkip_CursorMovedAlwaysFetches(t *testing.T) {
	t.Parallel()
	m := newPreviewSkipTestModel(t)
	m.fullYAMLPreview = true
	m.suppressBgtasks = true
	m.scheduler.StartWorkers()
	t.Cleanup(m.scheduler.StopWorkers)

	previewFetchKinds(m.loadPreviewResources()) // primes web-0's fingerprint

	m.middleItems = []model.Item{withPodRV("web-1", "100")}
	m.setCursor(0)
	kinds := previewFetchKinds(m.loadPreviewResources())
	assert.True(t, kinds["containers"], "a different pod under the cursor must always fetch containers")
	assert.True(t, kinds["yaml"], "a different pod under the cursor must always fetch YAML")
}

// TestPreviewSkip_BumpedRVAlwaysFetches covers case (d): a second tick whose
// RV moved on must fetch, proving the gate compares actual values rather
// than just presence of a fingerprint.
func TestPreviewSkip_BumpedRVAlwaysFetches(t *testing.T) {
	t.Parallel()
	m := newPreviewSkipTestModel(t)
	m.fullYAMLPreview = true
	m.suppressBgtasks = true
	m.scheduler.StartWorkers()
	t.Cleanup(m.scheduler.StopWorkers)

	previewFetchKinds(m.loadPreviewResources()) // primes fingerprint at RV 100

	m.middleItems = []model.Item{withPodRV("web-0", "200")}
	kinds := previewFetchKinds(m.loadPreviewResources())
	assert.True(t, kinds["containers"], "a bumped resourceVersion must always fetch containers")
	assert.True(t, kinds["yaml"], "a bumped resourceVersion must always fetch YAML")
}

// TestPreviewSkip_DifferentContextNeverSkips covers case (e): a same-named
// pod at the same RV in a different context must never reuse the prior
// context's fingerprint.
func TestPreviewSkip_DifferentContextNeverSkips(t *testing.T) {
	t.Parallel()
	m := newPreviewSkipTestModel(t)
	m.fullYAMLPreview = true
	m.suppressBgtasks = true
	m.scheduler.StartWorkers()
	t.Cleanup(m.scheduler.StopWorkers)

	previewFetchKinds(m.loadPreviewResources()) // primes web-0's fingerprint under "test-ctx"

	m.nav.Context = "other-ctx"
	kinds := previewFetchKinds(m.loadPreviewResources())
	assert.True(t, kinds["containers"], "a different context must always fetch containers, even with the same namespace/name/RV")
	assert.True(t, kinds["yaml"], "a different context must always fetch YAML, even with the same namespace/name/RV")
}

// TestPreviewSkip_FingerprintStaysBounded proves the store holds at most the
// currently hovered item, not one entry per item ever seen.
func TestPreviewSkip_FingerprintStaysBounded(t *testing.T) {
	t.Parallel()
	m := newPreviewSkipTestModel(t)
	m.suppressBgtasks = true
	m.scheduler.StartWorkers()
	t.Cleanup(m.scheduler.StopWorkers)

	previewFetchKinds(m.loadPreviewResources()) // primes web-0's fingerprint

	m.middleItems = []model.Item{withPodRV("web-1", "100")}
	m.setCursor(0)
	previewFetchKinds(m.loadPreviewResources()) // primes web-1's fingerprint

	assert.Len(t, m.previewContentFingerprints, 1, "the fingerprint store must never hold more than the currently hovered item")
}
