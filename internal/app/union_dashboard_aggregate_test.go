package app

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/model"
)

func memberDash(nodes, ready, pods int, warns ...model.Item) dashboardData {
	items := make([]model.Item, nodes)
	for i := range items {
		items[i] = model.Item{Name: "node" + string(rune('a'+i)), Status: "Ready"}
	}
	return dashboardData{
		nodeItems: items, nodeCount: nodes, readyNodes: ready,
		pods:         podStats{total: pods, running: pods},
		nsCount:      4,
		podCapacity:  110,
		totalCPUUsed: 1000, totalCPUAlloc: 4000,
		totalMemUsed: 1 << 30, totalMemAlloc: 4 << 30,
		warningEvents: warns, allWarnings: warns,
	}
}

func warnEvent(name string, at time.Time) model.Item {
	return model.Item{Name: name, Namespace: "ns", Status: "Warning", LastSeen: at}
}

func TestMergeMemberDashboards_SumsTotals(t *testing.T) {
	by := map[string]dashboardData{
		"a": memberDash(2, 2, 10),
		"b": memberDash(3, 2, 5),
	}
	got, members := mergeMemberDashboards([]string{"a", "b"}, by)

	assert.Equal(t, 5, got.nodeCount)
	assert.Equal(t, 4, got.readyNodes)
	assert.Equal(t, 15, got.pods.total)
	assert.Equal(t, 15, got.pods.running)
	assert.Equal(t, 8, got.nsCount)
	assert.Equal(t, int64(220), got.podCapacity)
	assert.Equal(t, int64(2000), got.totalCPUUsed)
	assert.Equal(t, int64(8000), got.totalCPUAlloc)
	assert.Equal(t, int64(2<<30), got.totalMemUsed)
	assert.Equal(t, int64(8<<30), got.totalMemAlloc)
	require.Len(t, got.nodeItems, 5)
	assert.Equal(t, "a/nodea", got.nodeItems[0].Name)
	assert.Nil(t, got.nodes, "per-node section is skipped")
	assert.Equal(t, []unionMember{{ctx: "a", state: unionMemberOK}, {ctx: "b", state: unionMemberOK}}, members)
}

func TestMergeMemberDashboards_EventsTaggedSortedCapped(t *testing.T) {
	now := time.Now()
	evA := make([]model.Item, 0, 20)
	evB := make([]model.Item, 0, 20)
	for i := range 20 {
		evA = append(evA, warnEvent("a", now.Add(-time.Duration(2*i)*time.Minute)))
		evB = append(evB, warnEvent("b", now.Add(-time.Duration(2*i+1)*time.Minute)))
	}
	by := map[string]dashboardData{"a": memberDash(1, 1, 1, evA...), "b": memberDash(1, 1, 1, evB...)}

	got, _ := mergeMemberDashboards([]string{"a", "b"}, by)

	require.Len(t, got.allWarnings, 40)
	require.Len(t, got.warningEvents, 10)
	assert.Equal(t, "a", got.allWarnings[0].ClusterName)
	assert.Equal(t, "b", got.allWarnings[1].ClusterName)
	for i := 1; i < len(got.allWarnings); i++ {
		assert.False(t, got.allWarnings[i].LastSeen.After(got.allWarnings[i-1].LastSeen))
	}
}

func TestMergeMemberDashboards_DoesNotMutateSources(t *testing.T) {
	ev := []model.Item{warnEvent("e", time.Now())}
	d := memberDash(1, 1, 1, ev...)
	d.pdbWarnings = []pdbWarning{{name: "p", namespace: "ns"}}
	by := map[string]dashboardData{"a": d}

	got, _ := mergeMemberDashboards([]string{"a"}, by)

	assert.Empty(t, ev[0].ClusterName)
	assert.Equal(t, "nodea", d.nodeItems[0].Name)
	assert.Equal(t, "ns", d.pdbWarnings[0].namespace)
	assert.Equal(t, "a/ns", got.pdbWarnings[0].namespace)
}

func TestMergeMemberDashboards_ErroredAndLoadingMembers(t *testing.T) {
	bad := memberDash(9, 9, 99)
	bad.nodesErr = errors.New("refused")
	by := map[string]dashboardData{"a": memberDash(2, 2, 10), "bad": bad}

	got, members := mergeMemberDashboards([]string{"a", "bad", "slow"}, by)

	assert.Equal(t, 2, got.nodeCount, "errored member adds no totals")
	assert.Equal(t, 10, got.pods.total)
	require.Len(t, members, 3)
	assert.Equal(t, unionMemberError, members[1].state)
	assert.EqualError(t, members[1].err, "refused")
	assert.Equal(t, unionMemberLoading, members[2].state)
}

func TestMergeMemberDashboards_MetricsErrIfAnyMember(t *testing.T) {
	b := memberDash(1, 1, 1)
	b.nodeMetricsErr = errors.New("no metrics")
	got, _ := mergeMemberDashboards([]string{"a", "b"}, map[string]dashboardData{"a": memberDash(1, 1, 1), "b": b})
	assert.Error(t, got.nodeMetricsErr)
}

func unionTypesModel(extra string) Model {
	m := basePush80Model()
	m.unionMode = true
	m.unionContexts = []string{"blue", "green"}
	m.nav = model.NavigationState{Level: model.LevelResourceTypes, Context: UnionContextSentinel}
	m.dashboardAcc = make(map[string]*dashboardAccumulator)
	m.middleItems = []model.Item{{Name: "Dash", Kind: "__overview__", Extra: extra}}
	m.setCursor(0)
	return m
}

func TestDashboardMsgWanted_AggregateAcceptsMembersOnly(t *testing.T) {
	m := unionTypesModel("__overview__")
	assert.True(t, m.dashboardMsgWanted("blue", unionDashboardCluster))
	assert.True(t, m.dashboardMsgWanted("green", unionDashboardCluster))
	assert.False(t, m.dashboardMsgWanted("stale", unionDashboardCluster))
	assert.False(t, m.dashboardMsgWanted("blue", unionDashboardMonitoring), "other dashboard kind")
}

func TestDashboardMsgWanted_InactiveOutsideAggregate(t *testing.T) {
	m := unionTypesModel("pods")
	assert.False(t, m.dashboardMsgWanted("blue", unionDashboardCluster))

	single := basePush80Model()
	single.nav.Context = "only"
	assert.True(t, single.dashboardMsgWanted("only", unionDashboardCluster))
	assert.False(t, single.dashboardMsgWanted("blue", unionDashboardCluster))
}

func TestHandleDashboardPartial_AggregateAcceptsMemberAndKeepsGenCheck(t *testing.T) {
	m := unionTypesModel("__overview__")
	m.requestGen = 3

	got, cmd := m.handleDashboardPartial(dashboardPartialMsg{context: "blue", gen: 3, key: "nodes", total: 1, data: memberDash(1, 1, 1)})
	require.NotNil(t, cmd)
	loaded, ok := cmd().(dashboardLoadedMsg)
	require.True(t, ok)
	assert.Equal(t, "blue", loaded.context)
	assert.Empty(t, got.dashboardAcc)

	_, cmd = m.handleDashboardPartial(dashboardPartialMsg{context: "blue", gen: 2, key: "nodes", total: 1})
	assert.Nil(t, cmd, "stale gen dropped")
	_, cmd = m.handleDashboardPartial(dashboardPartialMsg{context: "stale", gen: 3, key: "nodes", total: 1})
	assert.Nil(t, cmd, "non-member dropped")
}

func TestUpdateDashboardLoaded_AggregateStoresMemberData(t *testing.T) {
	m := unionTypesModel("__overview__")
	got := m.updateDashboardLoaded(dashboardLoadedMsg{context: "green", data: memberDash(2, 2, 7)})
	assert.Equal(t, 7, got.dashboardData["green"].pods.total)
	assert.Contains(t, stripANSI(got.dashboardPreview), "MEMBERS")

	stale := m.updateDashboardLoaded(dashboardLoadedMsg{context: "stale", data: memberDash(1, 1, 1)})
	assert.NotContains(t, stale.dashboardData, "stale")
}

func queuedTargets(m Model) map[string]bool {
	out := map[string]bool{}
	for _, e := range m.scheduler.QueueSnapshot() {
		out[e.Target] = true
	}
	return out
}

func TestLoadPreview_UnionClusterEntryFansOutToEveryMember(t *testing.T) {
	m := unionTypesModel("__overview__")
	m.scheduler = scheduler.New(0)

	cmd := m.loadPreview()
	require.NotNil(t, cmd)

	targets := queuedTargets(m)
	for _, ctx := range []string{"blue", "green"} {
		assert.True(t, targets[ctx+"#nodes"], "member %s nodes section must be queued", ctx)
		assert.True(t, targets[ctx+"#pods"], "member %s pods section must be queued", ctx)
	}
}

func TestLoadPreview_UnionMonitoringEntryLoadsEveryMember(t *testing.T) {
	m := unionTypesModel("__monitoring__")
	m.scheduler = scheduler.New(0)

	require.NotNil(t, m.loadPreview())

	got := map[string]bool{}
	for _, task := range m.scheduler.Snapshot() {
		got[task.Target] = true
	}
	assert.True(t, got["blue / default"])
	assert.True(t, got["green / default"])
}

func TestRenderUnionClusterPreview_AggregatesTotalsEventsAndMemberErrors(t *testing.T) {
	m := unionTypesModel("__overview__")
	ev := warnEvent("e", time.Now())
	ev.Extra = ""
	bad := dashboardData{nodesErr: errors.New("dial refused")}
	m.dashboardData = map[string]dashboardData{
		"blue":  memberDash(2, 2, 10, ev),
		"green": bad,
	}
	m = m.recomposeDashboard()

	view := stripANSI(m.renderRightResourceTypes(100, 40))

	assert.Contains(t, view, "2 Ready")
	assert.Contains(t, view, "10 Running")
	assert.Contains(t, view, "[blue]")
	assert.Contains(t, view, "MEMBERS")
	assert.Contains(t, view, "x green: dial refused")
	assert.Contains(t, view, "ok blue")
}

func TestRenderUnionClusterPreview_LoadingUntilFirstMember(t *testing.T) {
	m := unionTypesModel("__overview__")
	m = m.recomposeDashboard()
	assert.Contains(t, stripANSI(m.renderRightResourceTypes(80, 20)), "Loading cluster dashboard")
}
