package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
	"github.com/stretchr/testify/assert"
)

// Arriving at the container level with sparkline mode already on must request a
// container series without another keypress. listMetricsCmds never sees
// containers, since they load through updateContainersLoaded, so the range
// fetch has to be dispatched from there or the mode looks like it was lost.
func TestContainerMetricsCmds_SparkModeAlsoRequestsHistory(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelContainers
	m.nav.OwnedName = "api-1"
	m.nav.Namespace = "default"
	m.metricsSpark = ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark}

	cmds := m.containerMetricsCmds()

	assert.Len(t, cmds, 2, "sparkline mode needs the instant fetch AND the range fetch")
}

// Numeric mode must not pay for a range query nobody draws.
func TestContainerMetricsCmds_NumericModeSkipsHistory(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelContainers
	m.nav.OwnedName = "api-1"
	m.nav.Namespace = "default"

	cmds := m.containerMetricsCmds()

	assert.Len(t, cmds, 1, "numeric mode needs only the instant fetch")
}

// containerMetricsCmds has a pointer receiver but is called from
// updateContainersLoaded, which has a value receiver. The throttle stamp lands
// in the metricsLastFetch MAP, which survives the copy because a map is a
// reference. The dashboard path had this same shape and its throttle silently
// never fired, because that map was left nil by the constructor and the lazy
// make landed on the discarded copy. This pins that the stamp sticks.
func TestContainerMetricsCmds_SparklineThrottleStampSurvivesTheValueCopy(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelContainers
	m.nav.OwnedName = "api-1"
	m.nav.Namespace = "default"
	m.metricsSpark = ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark}

	first := m.containerMetricsCmds()
	second := m.containerMetricsCmds()

	assert.Len(t, first, 2, "the first call requests instant and range")
	assert.Len(t, second, 1,
		"the range fetch must be throttled on the second call, which only holds if the stamp persisted")
}
