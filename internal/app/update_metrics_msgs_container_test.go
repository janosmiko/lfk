package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateContainerMetricsEnrichedWithData verifies a container row is
// enriched from the per-container usage map, keyed by container name.
func TestUpdateContainerMetricsEnrichedWithData(t *testing.T) {
	m := basePush80Model()
	m.middleItems = []model.Item{
		{Name: "app", Kind: "Container"},
		{Name: "sidecar", Kind: "Container"},
	}

	metrics := map[string]k8s.ContainerUsage{
		"app": {CPUMilli: 250, MemBytes: 128 * 1024 * 1024},
	}
	result, cmd := m.Update(containerMetricsEnrichedMsg{metrics: metrics, gen: m.requestGen})
	mdl := result.(Model)
	assert.Nil(t, cmd)

	require.Equal(t, "250m", getColumnValue(mdl.middleItems[0], "CPU"))
	require.Equal(t, "128Mi", getColumnValue(mdl.middleItems[0], "MEM"))
}

// A map miss that fell through to FormatCPU/FormatMemory would render a
// fabricated "0m"/"0B" instead of "n/a" - the exact-string equality below
// catches that, where a loop-with-if would pass vacuously.
func TestUpdateContainerMetricsEnrichedMissingIsExactlyNA(t *testing.T) {
	m := basePush80Model()
	m.middleItems = []model.Item{
		{Name: "app", Kind: "Container"},
	}
	metrics := map[string]k8s.ContainerUsage{} // no entry for "app"

	result, _ := m.Update(containerMetricsEnrichedMsg{metrics: metrics, gen: m.requestGen})
	mdl := result.(Model)

	require.Equal(t, "n/a", getColumnValue(mdl.middleItems[0], "CPU"))
	require.Equal(t, "n/a", getColumnValue(mdl.middleItems[0], "MEM"))
}

// TestUpdateContainerMetricsEnrichedStaleGen verifies a reply for a
// superseded generation is dropped without touching the middle items.
func TestUpdateContainerMetricsEnrichedStaleGen(t *testing.T) {
	m := basePush80Model()
	m.requestGen = 10
	m.middleItems = []model.Item{{Name: "app", Kind: "Container"}}

	result, cmd := m.Update(containerMetricsEnrichedMsg{
		metrics: map[string]k8s.ContainerUsage{"app": {CPUMilli: 999, MemBytes: 999}},
		gen:     5,
	})
	mdl := result.(Model)
	assert.Nil(t, cmd)
	assert.Empty(t, mdl.middleItems[0].Columns, "stale reply must not write any column")
}
