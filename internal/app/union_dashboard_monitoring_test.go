package app

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
)

func testAlert(name string) k8s.AlertInfo {
	return k8s.AlertInfo{
		Name: name, State: "firing", Severity: "critical",
		Since: time.Now().Add(-time.Hour), Labels: map[string]string{"namespace": "kube-system", "alertname": name},
	}
}

func TestComposeUnionMonitoring_MergesAlertsTaggedWithContext(t *testing.T) {
	data := map[string]monitoringData{
		"blue":  {alerts: []k8s.AlertInfo{testAlert("AlphaDown")}},
		"green": {alerts: []k8s.AlertInfo{testAlert("BetaDown"), testAlert("GammaDown")}},
	}
	out := stripANSI(composeUnionMonitoring([]string{"blue", "green"}, data))

	assert.Contains(t, out, "3 total")
	assert.Contains(t, out, "CONTEXT")
	rows := map[string]int{}
	for line := range strings.SplitSeq(out, "\n") {
		if strings.Contains(line, "firing") {
			for _, c := range []string{"blue", "green"} {
				if strings.Contains(line, c) {
					rows[c]++
				}
			}
		}
	}
	assert.Equal(t, map[string]int{"blue": 1, "green": 2}, rows)
	for _, want := range []string{"AlphaDown", "BetaDown", "GammaDown"} {
		assert.Contains(t, out, want)
	}
}

func TestComposeUnionMonitoring_DoesNotMutateSources(t *testing.T) {
	src := []k8s.AlertInfo{testAlert("A")}
	composeUnionMonitoring([]string{"blue"}, map[string]monitoringData{"blue": {alerts: src}})
	assert.Empty(t, src[0].Context)
}

func TestComposeUnionMonitoring_FailedAndLoadingMembersListed(t *testing.T) {
	data := map[string]monitoringData{
		"blue": {alerts: []k8s.AlertInfo{testAlert("A")}},
		"red":  {errMsg: "no prometheus"},
	}
	out := stripANSI(composeUnionMonitoring([]string{"blue", "red", "slow"}, data))

	assert.Contains(t, out, "1 total", "failed member adds no alerts")
	assert.Contains(t, out, "x red: no prometheus")
	assert.Contains(t, out, "slow loading")
	assert.Contains(t, out, "ok blue")
}

func TestComposeUnionMonitoring_AllMembersQuietShowsNoAlerts(t *testing.T) {
	out := stripANSI(composeUnionMonitoring([]string{"blue"}, map[string]monitoringData{"blue": {}}))
	assert.Contains(t, out, "No active alerts")
}

func TestComposeMonitoring_SingleClusterHasNoContextColumn(t *testing.T) {
	out := stripANSI(composeMonitoring("c", []k8s.AlertInfo{testAlert("A")}, ""))
	assert.NotContains(t, out, "CONTEXT")
	assert.Contains(t, out, "STATE      SEVERITY     SINCE          NAMESPACE")
}

func TestUpdateMonitoringDashboard_AggregateAcceptsMembers(t *testing.T) {
	m := unionTypesModel("__monitoring__")
	got := m.updateMonitoringDashboard(monitoringDashboardMsg{context: "blue", alerts: []k8s.AlertInfo{testAlert("A")}})
	require.Contains(t, got.monitoringData, "blue")
	out := stripANSI(got.monitoringPreview)
	assert.Contains(t, out, "1 total")
	assert.Contains(t, out, "green loading")

	stale := m.updateMonitoringDashboard(monitoringDashboardMsg{context: "stale"})
	assert.NotContains(t, stale.monitoringData, "stale")
}
