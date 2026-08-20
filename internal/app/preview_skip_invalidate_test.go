package app

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The fingerprint is recorded at dispatch time, so a containers fetch that
// fails afterwards must drop it. Otherwise the next suppressed tick matches
// the unchanged resourceVersion and skips the retry forever.
func TestPreviewSkip_ContainersErrorForcesRefetch(t *testing.T) {
	t.Parallel()
	m := newPreviewSkipTestModel(t)
	m.fullYAMLPreview = true
	m.suppressBgtasks = true
	m.scheduler.StartWorkers()
	t.Cleanup(m.scheduler.StopWorkers)

	previewFetchKinds(m.loadPreviewResources()) // primes the fingerprint at RV 100

	mdl, _ := m.updateContainersLoaded(containersLoadedMsg{
		err:        errors.New("connection refused"),
		forPreview: true,
		gen:        m.requestGen,
	})
	m = mdl.(Model)

	kinds := previewFetchKinds(m.loadPreviewResources())
	assert.True(t, kinds["containers"], "a failed containers fetch must not latch the skip on the next watch tick")
	assert.True(t, kinds["yaml"], "a failed containers fetch must also re-arm the YAML fetch, which shares the gate")
}

// TestPreviewSkip_ContainersCancelForcesRefetch covers the same leg for a
// torn-down request context: the handler returns early with no items, so the
// preview is just as stale as on an outright error.
func TestPreviewSkip_ContainersCancelForcesRefetch(t *testing.T) {
	t.Parallel()
	m := newPreviewSkipTestModel(t)
	m.fullYAMLPreview = true
	m.suppressBgtasks = true
	m.scheduler.StartWorkers()
	t.Cleanup(m.scheduler.StopWorkers)

	previewFetchKinds(m.loadPreviewResources()) // primes the fingerprint at RV 100

	mdl, _ := m.updateContainersLoaded(containersLoadedMsg{
		err:        context.Canceled,
		forPreview: true,
		gen:        m.requestGen,
	})
	m = mdl.(Model)

	kinds := previewFetchKinds(m.loadPreviewResources())
	assert.True(t, kinds["containers"], "a canceled containers fetch must not latch the skip on the next watch tick")
}

// TestPreviewSkip_YAMLErrorForcesRefetch covers the failed-fetch leg for the
// preview YAML fetch, which blanks previewYAML and would otherwise leave the
// pane empty until the object's resourceVersion moved on.
func TestPreviewSkip_YAMLErrorForcesRefetch(t *testing.T) {
	t.Parallel()
	m := newPreviewSkipTestModel(t)
	m.fullYAMLPreview = true
	m.suppressBgtasks = true
	m.scheduler.StartWorkers()
	t.Cleanup(m.scheduler.StopWorkers)

	previewFetchKinds(m.loadPreviewResources()) // primes the fingerprint at RV 100

	m = m.updatePreviewYAMLLoaded(previewYAMLLoadedMsg{
		err: errors.New("the server could not find the requested resource"),
		gen: m.requestGen,
	})

	kinds := previewFetchKinds(m.loadPreviewResources())
	assert.True(t, kinds["yaml"], "a failed YAML fetch must not latch the skip on the next watch tick")
	assert.True(t, kinds["containers"], "a failed YAML fetch must also re-arm the containers fetch, which shares the gate")
}

// An abandoned submission emits no message, so nothing could clear the
// fingerprint. CancelStaleByGen reclaims only Low-priority work, and both
// gated fetches are High — downgrading either one would latch the skip.
func TestPreviewSkip_GatedFetchesSurviveGenSupersede(t *testing.T) {
	t.Parallel()
	m := newPreviewSkipTestModel(t)
	m.fullYAMLPreview = true
	m.suppressBgtasks = true
	// Non-zero gen: Gen == 0 is staleByGen's blanket exemption, which would
	// make the tasks survive for the wrong reason.
	m.requestGen = 1
	// No StartWorkers: keep both submissions queued so the reclaim pass sees
	// them.

	require.NotNil(t, m.loadContainers(true))
	require.NotNil(t, m.loadPreviewYAML())
	require.Equal(t, 2, m.scheduler.QueueLen("test-ctx"))

	m.scheduler.CancelStaleByGen("test-ctx", m.requestGen+1)

	assert.Equal(t, 2, m.scheduler.QueueLen("test-ctx"),
		"the gated containers and YAML fetches must outlive a newer generation, since an abandoned fetch emits no message that could clear the fingerprint")
}
