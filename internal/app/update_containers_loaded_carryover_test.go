package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// containerRowWithUsage is a container row as it looks AFTER metrics
// enrichment: the usage columns are present. A fresh GetContainers response
// never carries them, which is the whole point of the carry-over.
func containerRowWithUsage(name, cpu, mem string) model.Item {
	return model.Item{
		Name: name,
		Kind: "Container",
		Columns: []model.KeyValue{
			{Key: "CPU", Value: cpu},
			{Key: "MEM", Value: mem},
			{Key: "Image", Value: "nginx:1.27"},
		},
	}
}

// containerRowFresh is a container row as buildContainerItem produces it: no
// usage columns at all.
func containerRowFresh(name string) model.Item {
	return model.Item{
		Name:    name,
		Kind:    "Container",
		Columns: []model.KeyValue{{Key: "Image", Value: "nginx:1.27"}},
	}
}

// A watch tick re-lists containers, and the fresh items carry no usage. Without
// a carry-over the CPU and MEM columns vanish until the throttled metrics fetch
// lands, which is up to metrics_interval away, so the columns visibly blink out
// and back on every tick. The pod and node path calls carryOverMetricsColumns
// for exactly this reason.
func TestUpdateContainersLoaded_CarriesUsageColumnsAcrossAWatchTick(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelContainers
	m.middleItems = []model.Item{
		containerRowWithUsage("app", "250m", "512Mi"),
		containerRowWithUsage("sidecar", "n/a", "n/a"),
	}

	got, _ := m.updateContainersLoaded(containersLoadedMsg{
		items: []model.Item{containerRowFresh("app"), containerRowFresh("sidecar")},
		gen:   m.requestGen,
	})
	mdl, ok := got.(Model)
	require.True(t, ok)

	require.Len(t, mdl.middleItems, 2)
	assert.Equal(t, "250m", getColumnValue(mdl.middleItems[0], "CPU"),
		"CPU must survive the refresh rather than blinking out")
	assert.Equal(t, "512Mi", getColumnValue(mdl.middleItems[0], "MEM"),
		"MEM must survive the refresh rather than blinking out")
	// An n/a placeholder is a real value to carry: dropping it makes the
	// column disappear for a metrics-less container, which blinks just as
	// visibly as a numeric one.
	assert.Equal(t, "n/a", getColumnValue(mdl.middleItems[1], "CPU"),
		"an n/a placeholder must be carried too")
}

// The preview pane lists containers through the same message, so it blinks the
// same way. Fixing only the main list would leave the identical bug on a
// neighbouring surface.
func TestUpdateContainersLoaded_PreviewCarriesUsageColumnsToo(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelContainers
	m.rightItems = []model.Item{containerRowWithUsage("app", "250m", "512Mi")}

	got, _ := m.updateContainersLoaded(containersLoadedMsg{
		items:      []model.Item{containerRowFresh("app")},
		forPreview: true,
		gen:        m.requestGen,
	})
	mdl, ok := got.(Model)
	require.True(t, ok)

	require.Len(t, mdl.rightItems, 1)
	assert.Equal(t, "250m", getColumnValue(mdl.rightItems[0], "CPU"),
		"the container preview must not blink either")
}

// Drilling from a pod list into that pod's containers leaves the pod rows in
// middleItems. A pod and a container can share a name, so without the Kind
// guard the pod's usage would be donated to the container row and shown as
// though it were the container's own.
func TestUpdateContainersLoaded_DoesNotCarryFromANonContainerList(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelContainers
	podRow := containerRowWithUsage("app", "9999m", "9999Mi")
	podRow.Kind = "Pod"
	m.middleItems = []model.Item{podRow}

	got, _ := m.updateContainersLoaded(containersLoadedMsg{
		items: []model.Item{containerRowFresh("app")},
		gen:   m.requestGen,
	})
	mdl, ok := got.(Model)
	require.True(t, ok)

	require.Len(t, mdl.middleItems, 1)
	assert.Empty(t, getColumnValue(mdl.middleItems[0], "CPU"),
		"a pod row must not donate its usage to a same-named container")
}
