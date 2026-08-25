package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestLoadContainerMetricsForListReturnsCmd(t *testing.T) {
	m := basePush80Model()
	m.nav.OwnedName = "pod-1"
	cmd := m.loadContainerMetricsForList()
	assert.NotNil(t, cmd)
}

func TestLoadContainerMetricsForListSkipsAtUnionSentinel(t *testing.T) {
	m := basePush80Model()
	m.unionMode = true
	m.nav.Context = UnionContextSentinel
	cmd := m.loadContainerMetricsForList()
	assert.Nil(t, cmd)
}

// TestUpdateContainersLoadedDispatchesContainerMetricsFetch pins deliverable
// #5: a non-preview container list load must submit a metrics fetch, gated
// by allowMetricsFetch("Container") like the pod/node list loaders.
func TestUpdateContainersLoadedDispatchesContainerMetricsFetch(t *testing.T) {
	m := basePush80Model()
	m.scheduler = scheduler.New(0)
	m.nav.Level = model.LevelContainers
	m.nav.OwnedName = "pod-1"

	msg := containersLoadedMsg{
		items: []model.Item{{Name: "app", Kind: "Container"}},
		gen:   m.requestGen,
	}
	_, _ = m.updateContainersLoaded(msg)

	assert.Equal(t, 1, m.scheduler.QueueLenByPriority("test-ctx", scheduler.PriorityLow),
		"non-preview container list load must dispatch a container metrics fetch")
}

// TestUpdateContainersLoadedForPreviewSkipsContainerMetricsFetch guards the
// "non-preview path" scoping: a hover preview load must not also fire the
// list-wide metrics fetch.
func TestUpdateContainersLoadedForPreviewSkipsContainerMetricsFetch(t *testing.T) {
	m := basePush80Model()
	m.scheduler = scheduler.New(0)
	m.nav.Level = model.LevelContainers
	m.nav.OwnedName = "pod-1"

	msg := containersLoadedMsg{
		items:      []model.Item{{Name: "app", Kind: "Container"}},
		forPreview: true,
		gen:        m.requestGen,
	}
	_, _ = m.updateContainersLoaded(msg)

	assert.Equal(t, 0, m.scheduler.QueueLenByPriority("test-ctx", scheduler.PriorityLow))
}
