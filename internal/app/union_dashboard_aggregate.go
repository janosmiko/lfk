package app

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

type unionMemberState int

const (
	unionMemberOK unionMemberState = iota
	unionMemberLoading
	unionMemberError
)

type unionMember struct {
	ctx   string
	state unionMemberState
	err   error
}

// mergeMemberDashboards sums the loaded members into one dashboardData and
// reports each member's state. It never writes through the source slices.
func mergeMemberDashboards(contexts []string, byCtx map[string]dashboardData) (dashboardData, []unionMember) {
	var out dashboardData
	members := make([]unionMember, 0, len(contexts))
	for _, kctx := range contexts {
		d, ok := byCtx[kctx]
		switch {
		case !ok:
			members = append(members, unionMember{ctx: kctx, state: unionMemberLoading})
			continue
		case d.nodesErr != nil:
			members = append(members, unionMember{ctx: kctx, state: unionMemberError, err: d.nodesErr})
			continue
		}
		members = append(members, unionMember{ctx: kctx, state: unionMemberOK})

		out.nodeCount += d.nodeCount
		out.readyNodes += d.readyNodes
		out.podCapacity += d.podCapacity
		out.nsCount += d.nsCount
		out.totalCPUUsed += d.totalCPUUsed
		out.totalCPUAlloc += d.totalCPUAlloc
		out.totalMemUsed += d.totalMemUsed
		out.totalMemAlloc += d.totalMemAlloc
		out.pods.total += d.pods.total
		out.pods.running += d.pods.running
		out.pods.failed += d.pods.failed
		out.pods.pending += d.pods.pending
		out.pods.succeeded += d.pods.succeeded
		out.pods.crashLoop += d.pods.crashLoop
		if d.nodeMetricsErr != nil && out.nodeMetricsErr == nil {
			out.nodeMetricsErr = d.nodeMetricsErr
		}
		for _, n := range d.nodeItems {
			n.Name = kctx + "/" + n.Name
			out.nodeItems = append(out.nodeItems, n)
		}
		for _, ev := range d.allWarnings {
			ev.ClusterName = kctx
			out.allWarnings = append(out.allWarnings, ev)
		}
		for _, p := range d.pdbWarnings {
			p.namespace = kctx + "/" + p.namespace
			out.pdbWarnings = append(out.pdbWarnings, p)
		}
	}
	slices.SortStableFunc(out.allWarnings, func(a, b model.Item) int {
		return b.LastSeen.Compare(a.LastSeen)
	})
	out.warningEvents = out.allWarnings[:min(len(out.allWarnings), 10)]
	return out, members
}

// composeUnionDashboard renders the aggregate cluster dashboard plus a MEMBERS list.
func (m Model) composeUnionDashboard() (content, events string) {
	merged, members := mergeMemberDashboards(m.unionContexts, m.dashboardData)
	content, events = m.composeDashboard(merged)
	w := m.dashboardWidths(m.fullscreenDashboard, 0)
	lines := append([]string{
		ui.DimStyle.Render("  " + strings.Repeat("─", w.sep)),
		ui.DimStyle.Bold(true).Render("  MEMBERS"),
		"",
	}, unionMemberLines(members)...)
	return content + strings.Join(lines, "\n") + "\n", events
}

func unionMemberLines(members []unionMember) []string {
	lines := make([]string, 0, len(members))
	for _, mem := range members {
		name := ui.SanitizeTerminalText(mem.ctx)
		switch mem.state {
		case unionMemberError:
			lines = append(lines, ui.StatusFailed.Render("  x "+name+": "+ui.SanitizeTerminalText(mem.err.Error())))
		case unionMemberLoading:
			lines = append(lines, ui.DimStyle.Render("  ... "+name+" loading"))
		default:
			lines = append(lines, ui.StatusRunning.Render("  ok ")+name)
		}
	}
	return lines
}

func eventClusterTag(ev model.Item) string {
	if ev.ClusterName == "" {
		return ""
	}
	return ui.DimStyle.Render("[" + ui.SanitizeTerminalText(ev.ClusterName) + "] ")
}

// unionAggregateMode reports which aggregate dashboard the union types-level
// preview is showing, if any.
func (m Model) unionAggregateMode() (unionDashboardMode, bool) {
	if !m.isUnionSentinel() || m.nav.Level != model.LevelResourceTypes {
		return "", false
	}
	sel := m.selectedMiddleItem()
	if sel == nil {
		return "", false
	}
	return unionDashboardModeFromExtra(sel.Extra)
}

// dashboardMsgWanted reports whether a dashboard result for kctx belongs on
// screen: the previewed context, or any member while that aggregate is active.
func (m Model) dashboardMsgWanted(kctx string, mode unionDashboardMode) bool {
	if kctx == m.dashboardPreviewTargetContext() {
		return true
	}
	active, ok := m.unionAggregateMode()
	return ok && active == mode && slices.Contains(m.unionContexts, kctx)
}

func anyMemberLoaded[T any](contexts []string, byCtx map[string]T) bool {
	return slices.ContainsFunc(contexts, func(c string) bool {
		_, ok := byCtx[c]
		return ok
	})
}

func (m Model) loadUnionDashboardAggregate(mode unionDashboardMode) tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.unionContexts))
	for _, kctx := range m.unionContexts {
		if mode == unionDashboardMonitoring {
			cmds = append(cmds, m.loadMonitoringDashboardFor(kctx))
		} else {
			cmds = append(cmds, m.loadDashboardFor(kctx))
		}
	}
	return tea.Batch(cmds...)
}
