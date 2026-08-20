package app

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// pdbWarning holds data about a PodDisruptionBudget at risk.
type pdbWarning struct {
	name               string
	namespace          string
	minAvailable       string
	currentHealthy     string
	disruptionsAllowed string
}

// nodeInfo holds per-node resource usage data.
type nodeInfo struct {
	name                                 string
	cpuUsed, cpuAlloc, memUsed, memAlloc int64
}

// podStats holds aggregated pod status counts.
type podStats struct {
	total     int
	running   int
	failed    int
	pending   int
	succeeded int
	crashLoop int
}

// dashboardData holds all fetched data for the cluster dashboard.
type dashboardData struct {
	nodeItems       []model.Item
	nodeCount       int
	readyNodes      int
	podCapacity     int64 // sum of nodes' allocatable pods (max schedulable)
	pods            podStats
	nsCount         int
	warningEvents   []model.Item
	allWarnings     []model.Item
	pdbWarnings     []pdbWarning
	nodes           []nodeInfo
	totalCPUUsed    int64
	totalCPUAlloc   int64
	totalMemUsed    int64
	totalMemAlloc   int64
	nodeMetricsErr  error
	pinnedSummaries []pinnedSummaryResult
}

// monitoringData retains the raw alert payload behind monitoringPreview so the
// monitoring dashboard can be re-rendered on a theme change or resize without
// re-querying Prometheus. Mirrors dashboardData's per-context retention.
type monitoringData struct {
	alerts []k8s.AlertInfo
	errMsg string // non-empty when the monitoring backend was unreachable
}

// loadDashboard fans out the cluster dashboard fetch into parallel
// Low-priority scheduler tasks: 6 fixed sections plus one per pinned
// summary. Each emits a dashboardPartialMsg as it completes;
// handleDashboardPartial accumulates them and re-renders progressively.
//
// Side benefit beyond preemption: even on a healthy cluster, the
// dashboard renders incrementally instead of staying blank for ~20s.
func (m Model) loadDashboard() tea.Cmd {
	kctx := m.effectiveContext()
	return m.loadDashboardFor(kctx)
}

func (m Model) loadDashboardFor(kctx string) tea.Cmd {
	gen := m.requestGen
	client := m.client
	base := bgtaskTarget(kctx, "")

	// A prior fan-out for this same (context, gen) may still be mid-flight
	// (e.g. this reload was triggered by a pin toggle before the previous
	// dashboard load finished). Its accumulator's `expected` count was seeded
	// from *that* fan-out's total. If this fresh fan-out has a different
	// total (a different pin count), letting both feed the same accumulator
	// would mix section data across fan-outs or complete against the wrong
	// count. Evict it so this fan-out always starts from a clean slate. The
	// nil check guards test fixtures that build a bare Model{}.
	if m.dashboardAcc != nil {
		delete(m.dashboardAcc, dashboardAccKey(kctx, gen))
	}

	// Each section gets a unique target so the coalesce-by-sig logic
	// treats them as distinct tasks rather than collapsing them into one.
	sectionTarget := func(key string) string { return base + "#" + key }

	pins := m.effectivePinnedSummaries()
	discovered := m.discoveredResources[kctx]
	// Nothing pinned anywhere: fall back to the built-in defaults, unless the
	// user explicitly emptied pinned_summaries in config ("[]" disables
	// defaults too - see defaultPinsDisabled). Unresolved defaults (a CRD this
	// cluster lacks) are silently dropped rather than rendering "(not
	// installed)" noise, so total must count only the ones that will
	// actually resolve.
	silentSkip := false
	if len(pins) == 0 && !defaultPinsDisabled() {
		pins = defaultPinnedSummaries
		silentSkip = true
	}
	scheduledPins := len(pins)
	if silentSkip {
		scheduledPins = countResolvedPins(pins, discovered)
	}
	total := 6 + scheduledPins

	fixed := []struct {
		section dashboardSection
		fetch   func(ctx context.Context) dashboardData
	}{
		{dashboardSectionNodes, func(ctx context.Context) dashboardData { return fetchDashboardNodes(ctx, kctx, client) }},
		{dashboardSectionPods, func(ctx context.Context) dashboardData { return fetchDashboardPods(ctx, kctx, client) }},
		{dashboardSectionNamespaces, func(ctx context.Context) dashboardData { return fetchDashboardNamespaces(ctx, kctx, client) }},
		{dashboardSectionEvents, func(ctx context.Context) dashboardData { return fetchDashboardEvents(ctx, kctx, client) }},
		{dashboardSectionPDB, func(ctx context.Context) dashboardData { return fetchDashboardPDB(ctx, kctx, client) }},
		{dashboardSectionNodeMetrics, func(ctx context.Context) dashboardData {
			// Node metrics needs node items as input. Re-fetch them inside
			// this section to keep it self-contained. A node-list failure
			// surfaces as nodeMetricsErr so the dashboard renders an explicit
			// "metrics unavailable" instead of zeros.
			nodeItems, err := client.GetResources(ctx, kctx, "", model.ResourceTypeEntry{
				Kind: "Node", APIGroup: "", APIVersion: "v1", Resource: "nodes", Namespaced: false,
			})
			if err != nil {
				return dashboardData{nodeMetricsErr: err}
			}
			return fetchDashboardNodeMetrics(ctx, kctx, client, nodeItems)
		}},
	}

	cmds := make([]tea.Cmd, 0, total)
	for _, s := range fixed {
		key := s.section.String()
		fetch := s.fetch
		cmds = append(cmds, m.scheduleK8sCall(scheduler.PriorityLow, scheduler.KindDashboard,
			"Dashboard: "+key, sectionTarget(key),
			func(ctx context.Context) tea.Msg {
				return dashboardPartialMsg{context: kctx, gen: gen, key: key, total: total, data: fetch(ctx)}
			}))
	}

	cmds = append(cmds, m.pinnedSummaryCmds(kctx, gen, client, pins, discovered, total, silentSkip, sectionTarget)...)
	return tea.Batch(cmds...)
}

// dashboardAccumulator collects partial section results until all expected
// sections (msg.total: 6 fixed + one per pinned summary) have arrived, then
// composes the final dashboardLoadedMsg.
type dashboardAccumulator struct {
	gen      uint64
	data     dashboardData
	received map[string]bool // by dashboardPartialMsg.key
	expected int
	count    int
}

func dashboardAccKey(kctx string, gen uint64) string {
	return kctx + ":" + strconv.FormatUint(gen, 10)
}

// handleDashboardPartial accumulates a section result and emits a
// single dashboardLoadedMsg only after all expected sections have arrived.
// This avoids flickering the dashboard layout on every watch tick
// (each tick fires one partial fetch per section. Rendering on each one
// would repeatedly clear sections that haven't arrived yet).
//
// Stale messages (different context or different requestGen) are
// dropped silently AND any half-built accumulator for that stale
// (context, gen) is evicted — otherwise navigating away mid-refresh
// would leak partial entries in m.dashboardAcc forever.
func (m Model) handleDashboardPartial(msg dashboardPartialMsg) (Model, tea.Cmd) {
	if msg.context != m.dashboardPreviewTargetContext() || msg.gen != m.requestGen {
		// Drop any partial accumulator left behind for this stale
		// (context, gen). The guarded m.dashboardAcc init lets us skip
		// the delete when the map is nil (test fixtures).
		if m.dashboardAcc != nil {
			delete(m.dashboardAcc, dashboardAccKey(msg.context, msg.gen))
		}
		return m, nil
	}
	key := dashboardAccKey(msg.context, msg.gen)
	if m.dashboardAcc == nil {
		// Lazy-init: production app_init.go pre-allocates this map, but
		// test fixtures with bare Model{} don't. The stale-drop branch
		// above already guards a nil map. Mirror that here so a current
		// partial arriving before init can't panic.
		m.dashboardAcc = make(map[string]*dashboardAccumulator)
	}
	acc, ok := m.dashboardAcc[key]
	if !ok {
		acc = &dashboardAccumulator{gen: msg.gen, received: make(map[string]bool), expected: msg.total}
		m.dashboardAcc[key] = acc
	} else {
		// A coalesced old fan-out (smaller total) can race a fresh one (larger
		// total, e.g. a pin added mid-flight) on the same (context, gen).
		// Awaiting the larger fan-out guarantees a full frame. The smaller
		// fan-out's keys are a subset delivered by surviving coalesced tasks.
		acc.expected = max(acc.expected, msg.total)
	}
	if !acc.received[msg.key] {
		acc.received[msg.key] = true
		acc.count++
		mergeDashboardSection(&acc.data, msg.data)
	}

	// Defer the render until every section has arrived so the user sees
	// either the prior (still-valid) dashboard frame or the new one in
	// full — never a half-populated state.
	if acc.count < acc.expected {
		return m, nil
	}

	data := acc.data
	delete(m.dashboardAcc, key)
	return m, func() tea.Msg {
		return dashboardLoadedMsg{data: data, context: msg.context}
	}
}

// nodeSummaryStr is the inline status summary for the Nodes row, e.g.
// "14 Ready" or "12 Ready · 2 NotReady".
func nodeSummaryStr(d dashboardData) string {
	s := ui.StatusRunning.Render(fmt.Sprintf("%d Ready", d.readyNodes))
	if d.readyNodes < d.nodeCount {
		s += dashboardSummarySep + ui.StatusFailed.Render(fmt.Sprintf("%d NotReady", d.nodeCount-d.readyNodes))
	}
	return s
}

// podOther is the count of pods not in a counted phase (Terminating, Unknown,
// etc.) plus any rounding slack. Kept non-negative.
func podOther(p podStats) int {
	return max(p.total-p.running-p.pending-p.failed-p.succeeded, 0)
}

// podBarDenominator is the value the pod bar is measured against: cluster pod
// capacity when it exceeds the scheduled total, otherwise the scheduled total
// itself (capacity unknown, or already saturated). Using capacity makes the
// bar's empty tail represent unallocated schedulable slots.
func podBarDenominator(d dashboardData) int {
	if d.podCapacity > int64(d.pods.total) {
		return int(d.podCapacity)
	}
	return d.pods.total
}

// podUnallocated is the number of free schedulable pod slots: cluster capacity
// minus the pods already scheduled. Zero when capacity is unknown or saturated.
func podUnallocated(d dashboardData) int {
	return int(max(d.podCapacity-int64(d.pods.total), 0))
}

// podSummaryStr is the inline status breakdown for the Pods row, e.g.
// "361 Running · 39 Failed · 24 Succeeded". Only non-zero states are shown;
// Succeeded is grey (terminal, not an error), Failed red, Pending amber, and
// any uncounted pods show as a neutral "Other" so they never read as failures.
// "Unallocated" (free schedulable slots = pod capacity − scheduled) is appended
// when the cluster's pod capacity is known, mirroring the bar's empty tail. The
// categories and order mirror the bar segments so bar and text agree.
func podSummaryStr(d dashboardData) string {
	parts := make([]string, 0, 5)
	if d.pods.running > 0 {
		parts = append(parts, ui.StatusRunning.Render(fmt.Sprintf("%d Running", d.pods.running)))
	}
	if d.pods.pending > 0 {
		parts = append(parts, ui.StatusProgressing.Render(fmt.Sprintf("%d Pending", d.pods.pending)))
	}
	if d.pods.failed > 0 {
		parts = append(parts, ui.StatusFailed.Render(fmt.Sprintf("%d Failed", d.pods.failed)))
	}
	if d.pods.succeeded > 0 {
		parts = append(parts, ui.StatusOther.Render(fmt.Sprintf("%d Succeeded", d.pods.succeeded)))
	}
	if other := podOther(d.pods); other > 0 {
		parts = append(parts, ui.DimStyle.Render(fmt.Sprintf("%d Other", other)))
	}
	if unalloc := podUnallocated(d); unalloc > 0 {
		parts = append(parts, ui.DimStyle.Render(fmt.Sprintf("%d Unallocated", unalloc)))
	}
	if len(parts) == 0 {
		return ui.DimStyle.Render("no pods")
	}
	return strings.Join(parts, dashboardSummarySep)
}

func cpuSummaryStr(d dashboardData) string {
	return ui.NormalStyle.Render(ui.FormatCPU(d.totalCPUUsed) + " / " + ui.FormatCPU(d.totalCPUAlloc))
}

func memSummaryStr(d dashboardData) string {
	return ui.NormalStyle.Render(ui.FormatMemory(d.totalMemUsed) + " / " + ui.FormatMemory(d.totalMemAlloc))
}

// dashboardSummarySep separates status counts within an inline summary.
const dashboardSummarySep = " · "

// composeDashboard renders the dashboard content + events column for the given
// data at the current display width. Pure w.r.t. the model except for reading
// width / fullscreen state, so it can be re-run whenever those change.
func (m Model) composeDashboard(data dashboardData) (content, events string) {
	// The fullscreen cluster dashboard is always two-column (the right column
	// always shows at least "RECENT EVENTS"). There, warnings move to the top
	// of the right column, above the events. In the non-fullscreen preview pane
	// everything stacks in the single column.
	twoCol := m.fullscreenDashboard
	w := m.dashboardWidths(twoCol, maxPinnedLabelWidth(data))

	var left []string
	left = append(left, "")
	left = dashboardHeaderSection(left, data, w)
	// Pinned rows render directly below Pods, inside the same block - no
	// separator, no header - so the trailing blank that used to close
	// dashboardHeaderSection is added here instead, after the pinned rows.
	left = dashboardPinnedRows(left, data, w)
	left = append(left, "")
	left = dashboardResourcesSection(left, data, w)
	left = dashboardNodesSection(left, data, w)
	if !twoCol {
		left = dashboardWarningsSection(left, data, w)
		left = dashboardInlineEventsSection(left, data.warningEvents, w)
	}
	left = append(left, "")
	content = strings.Join(left, "\n")

	var right []string
	if twoCol {
		if warn := dashboardWarningsColumn(data); len(warn) > 0 {
			right = append(right, warn...)
			// Divide warnings from the events below. Size it to match
			// wrapEventsColumn's wrap width (rightW-4). That helper adds the
			// leading "  " pad, so the rule renders as a single full-width line
			// rather than wrapping.
			_, rightW := dashboardColumnWidths(content, m.width-2)
			right = append(right, ui.DimStyle.Render(strings.Repeat("─", max(rightW-4, 10))))
		}
	}
	right = append(right, dashboardEventsColumn(data.allWarnings)...)
	events = strings.Join(right, "\n")
	return content, events
}

// sumPodCapacity adds up each node's allocatable pod count (the "Pods Alloc"
// column populated from .status.allocatable.pods). This is the maximum number
// of pods the cluster can schedule, used as the pod bar's denominator so the
// unfilled tail reads as unallocated headroom rather than 100% utilization.
func sumPodCapacity(nodeItems []model.Item) int64 {
	var total int64
	for _, n := range nodeItems {
		for _, kv := range n.Columns {
			if kv.Key == "Pods Alloc" {
				if v, err := strconv.ParseInt(kv.Value, 10, 64); err == nil {
					total += v
				}
				break
			}
		}
	}
	return total
}

// countPodStats tallies pod statuses.
func countPodStats(podItems []model.Item) podStats {
	ps := podStats{total: len(podItems)}
	for _, p := range podItems {
		switch p.Status {
		case "Running":
			ps.running++
		case "CrashLoopBackOff":
			ps.failed++
			ps.crashLoop++
		case "Failed", "Error", "ImagePullBackOff", "ErrImagePull", "OOMKilled":
			ps.failed++
		case "Pending", "ContainerCreating", "Init":
			ps.pending++
		case "Succeeded", "Completed":
			ps.succeeded++
		}
	}
	return ps
}

// fetchWarningEvents fetches events and returns (limited for inline, all for column).
// Events are ordered most-recently-observed first (LastSeen, not CreatedAt) so a
// long-running incident that just fired again sits at the top of the dashboard
// instead of being buried under one-off events that happened to start later.
func fetchWarningEvents(reqCtx context.Context, kctx string, client *k8s.Client) (limited, all []model.Item) {
	eventItems, _ := client.GetResources(reqCtx, kctx, "", model.ResourceTypeEntry{
		Kind: "Event", APIGroup: "", APIVersion: "v1", Resource: "events", Namespaced: true,
		FieldSelector: "type=Warning",
	})
	// The field selector is a server-side optimization only. The fake/demo
	// dynamic client ignores it, so the client-side filter below stays.
	var warnings []model.Item
	for _, e := range eventItems {
		if e.Status == "Warning" {
			warnings = append(warnings, e)
		}
	}
	sort.Slice(warnings, func(i, j int) bool {
		return warnings[i].LastSeen.After(warnings[j].LastSeen)
	})
	all = warnings
	limited = warnings
	if len(limited) > 10 {
		limited = limited[:10]
	}
	return limited, all
}

// fetchPDBWarnings detects PodDisruptionBudgets at risk.
func fetchPDBWarnings(reqCtx context.Context, kctx string, client *k8s.Client) []pdbWarning {
	pdbItems, pdbErr := client.GetResources(reqCtx, kctx, "", model.ResourceTypeEntry{
		Kind: "PodDisruptionBudget", APIGroup: "policy", APIVersion: "v1", Resource: "poddisruptionbudgets", Namespaced: true,
	})
	if pdbErr != nil {
		return nil
	}
	var warnings []pdbWarning
	for _, pdb := range pdbItems {
		if pw, atRisk := parsePDBWarning(pdb); atRisk {
			warnings = append(warnings, pw)
		}
	}
	return warnings
}

// parsePDBWarning checks a single PDB for at-risk conditions.
func parsePDBWarning(pdb model.Item) (pdbWarning, bool) {
	var minAvail, currentHealthy, disruptionsAllowed string
	var disruptionsVal int64 = -1
	var currentVal int64 = -1
	var minAvailVal int64 = -1
	for _, kv := range pdb.Columns {
		switch kv.Key {
		case "Min Available":
			minAvail = kv.Value
			if v, err := strconv.ParseInt(kv.Value, 10, 64); err == nil {
				minAvailVal = v
			}
		case "Current Healthy":
			currentHealthy = kv.Value
			if v, err := strconv.ParseInt(kv.Value, 10, 64); err == nil {
				currentVal = v
			}
		case "Disruptions Allowed":
			disruptionsAllowed = kv.Value
			if v, err := strconv.ParseInt(kv.Value, 10, 64); err == nil {
				disruptionsVal = v
			}
		}
	}
	atRisk := disruptionsVal == 0
	if !atRisk && minAvailVal >= 0 && currentVal >= 0 {
		atRisk = currentVal <= minAvailVal
	}
	return pdbWarning{
		name:               pdb.Name,
		namespace:          pdb.Namespace,
		minAvailable:       minAvail,
		currentHealthy:     currentHealthy,
		disruptionsAllowed: disruptionsAllowed,
	}, atRisk
}

// fetchNodeMetrics collects per-node resource usage and totals.
func fetchNodeMetrics(reqCtx context.Context, kctx string, client *k8s.Client, nodeItems []model.Item) (
	nodes []nodeInfo, totalCPUUsed, totalCPUAlloc, totalMemUsed, totalMemAlloc int64, metricsErr error,
) {
	nodeMetrics, metricsErr := client.GetAllNodeMetrics(reqCtx, kctx)
	if metricsErr != nil {
		logger.Warn("Failed to fetch node metrics (metrics-server may not be installed)", "error", metricsErr)
	}
	nodes = make([]nodeInfo, 0, len(nodeItems))
	for _, ni := range nodeItems {
		info := nodeInfo{name: ni.Name}
		if nm, ok := nodeMetrics[ni.Name]; ok {
			info.cpuUsed = nm.CPU
			info.memUsed = nm.Memory
			totalCPUUsed += nm.CPU
			totalMemUsed += nm.Memory
		}
		for _, kv := range ni.Columns {
			switch kv.Key {
			case "CPU Alloc":
				v := ui.ParseResourceValue(kv.Value, true)
				info.cpuAlloc = v
				totalCPUAlloc += v
			case "Mem Alloc":
				v := ui.ParseResourceValue(kv.Value, false)
				info.memAlloc = v
				totalMemAlloc += v
			}
		}
		nodes = append(nodes, info)
	}
	return nodes, totalCPUUsed, totalCPUAlloc, totalMemUsed, totalMemAlloc, metricsErr
}

// loadMonitoringDashboard fetches active Prometheus alerts and renders a monitoring dashboard.
func (m Model) loadMonitoringDashboard() tea.Cmd {
	kctx := m.effectiveContext()
	return m.loadMonitoringDashboardFor(kctx)
}

func (m Model) loadMonitoringDashboardFor(kctx string) tea.Cmd {
	client := m.client
	ns := m.effectiveNamespace()
	return m.trackBgTask(
		scheduler.KindDashboard,
		"Monitoring dashboard",
		bgtaskTarget(kctx, ns),
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			alerts, err := client.GetAllActiveAlerts(ctx, kctx, ns)
			errMsg := ""
			if err != nil {
				errMsg = err.Error()
				alerts = nil
			}
			return monitoringDashboardMsg{
				content: composeMonitoring(alerts, errMsg),
				alerts:  alerts,
				errMsg:  errMsg,
				context: kctx,
			}
		},
	)
}

// composeMonitoring renders the monitoring dashboard body from raw alert data.
// It takes no model state so it can run both inside the load goroutine and in
// recomposeMonitoring, where it re-renders with the current theme on a theme
// change. errMsg, when non-empty, signals the monitoring backend was
// unreachable and renders the connectivity hint instead of the alert table.
func composeMonitoring(alerts []k8s.AlertInfo, errMsg string) string {
	var lines []string
	lines = append(lines, "")
	lines = append(lines, ui.DimStyle.Bold(true).Render("  MONITORING OVERVIEW"))
	lines = append(lines, "")

	if errMsg != "" {
		lines = append(lines, ui.DimStyle.Render("  Prometheus/Alertmanager not reachable"))
		lines = append(lines, ui.DimStyle.Render("  "+errMsg))
		lines = append(lines, "")
		lines = append(lines, ui.DimStyle.Render("  Searched in well-known namespaces:"))
		lines = append(lines, ui.DimStyle.Render("  monitoring, prometheus, observability, kube-prometheus-stack"))
		lines = append(lines, "")
		return strings.Join(lines, "\n")
	}

	lines = monitoringAlertSummary(lines, alerts)
	lines = append(lines, "")
	sortAlerts(alerts)
	lines = monitoringAlertTable(lines, alerts)
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

// monitoringAlertSummary renders the alert summary header with state/severity counts.
func monitoringAlertSummary(lines []string, alerts []k8s.AlertInfo) []string {
	firing, pending, critical, warning, info := 0, 0, 0, 0, 0
	for _, a := range alerts {
		switch a.State {
		case "firing":
			firing++
		case "pending":
			pending++
		}
		switch a.Severity {
		case "critical":
			critical++
		case "warning":
			warning++
		default:
			info++
		}
	}

	totalAlerts := len(alerts)
	lines = append(lines, fmt.Sprintf("  %s %s",
		ui.HelpKeyStyle.Render("Alerts:"),
		ui.NormalStyle.Render(fmt.Sprintf("%d total", totalAlerts))))

	if totalAlerts == 0 {
		lines = append(lines, ui.StatusRunning.Render("  \u2713 No active alerts"))
		return lines
	}

	// State breakdown.
	stateStr := ""
	if firing > 0 {
		stateStr += ui.StatusFailed.Render(fmt.Sprintf("%d firing", firing))
	}
	if pending > 0 {
		if stateStr != "" {
			stateStr += "  "
		}
		stateStr += ui.StatusProgressing.Render(fmt.Sprintf("%d pending", pending))
	}
	if stateStr != "" {
		lines = append(lines, "           "+stateStr)
	}

	// Severity breakdown.
	sevStr := ""
	if critical > 0 {
		sevStr += ui.StatusFailed.Bold(true).Render(fmt.Sprintf("%d critical", critical))
	}
	if warning > 0 {
		if sevStr != "" {
			sevStr += "  "
		}
		sevStr += ui.StatusProgressing.Render(fmt.Sprintf("%d warning", warning))
	}
	if info > 0 {
		if sevStr != "" {
			sevStr += "  "
		}
		sevStr += ui.DimStyle.Render(fmt.Sprintf("%d info", info))
	}
	if sevStr != "" {
		lines = append(lines, "           "+sevStr)
	}
	return lines
}

// sortAlerts sorts alerts by state, severity, name, time, and namespace.
func sortAlerts(alerts []k8s.AlertInfo) {
	stateOrder := map[string]int{"firing": 0, "pending": 1}
	severityOrder := map[string]int{"critical": 0, "warning": 1, "info": 2}
	sort.SliceStable(alerts, func(i, j int) bool {
		si, sj := stateOrder[alerts[i].State], stateOrder[alerts[j].State]
		if si != sj {
			return si < sj
		}
		sevi, sevj := severityOrder[alerts[i].Severity], severityOrder[alerts[j].Severity]
		if sevi != sevj {
			return sevi < sevj
		}
		if alerts[i].Name != alerts[j].Name {
			return alerts[i].Name < alerts[j].Name
		}
		if !alerts[i].Since.Equal(alerts[j].Since) {
			return alerts[i].Since.After(alerts[j].Since)
		}
		return alerts[i].Labels["namespace"] < alerts[j].Labels["namespace"]
	})
}

// monitoringAlertTable renders the alert detail table rows.
func monitoringAlertTable(lines []string, alerts []k8s.AlertInfo) []string {
	if len(alerts) == 0 {
		return lines
	}

	excludeLabels := map[string]bool{
		"severity": true, "namespace": true, "prometheus": true,
		"__name__": true, "job": true, "instance": true, "endpoint": true,
	}

	header := fmt.Sprintf("  %-10s %-12s %-14s %-12s",
		"STATE", "SEVERITY", "SINCE", "NAMESPACE")
	lines = append(lines, ui.DimStyle.Bold(true).Render(header))
	lines = append(lines, "")

	for i, a := range alerts {
		lines = append(lines, renderAlertRow(a))
		lines = renderAlertLabels(lines, a.Labels, excludeLabels)
		if i < len(alerts)-1 {
			lines = append(lines, "")
		}
	}
	return lines
}

// renderAlertRow renders a single alert's main row.
func renderAlertRow(a k8s.AlertInfo) string {
	// State and Severity are Prometheus labels, the same untrusted map the
	// namespace column below reads. A value that matches none of the known
	// cases reaches the default branch and would otherwise render raw.
	state := ui.SanitizeTerminalText(a.State)
	severity := ui.SanitizeTerminalText(a.Severity)

	var stateStr string
	switch state {
	case "firing":
		stateStr = ui.StatusFailed.Render(fmt.Sprintf("%-10s", state))
	case "pending":
		stateStr = ui.StatusProgressing.Render(fmt.Sprintf("%-10s", state))
	default:
		stateStr = ui.DimStyle.Render(fmt.Sprintf("%-10s", state))
	}

	var sevStr string
	switch severity {
	case "critical":
		sevStr = ui.StatusFailed.Bold(true).Render(fmt.Sprintf("%-12s", severity))
	case "warning":
		sevStr = ui.StatusProgressing.Render(fmt.Sprintf("%-12s", severity))
	default:
		sevStr = ui.DimStyle.Render(fmt.Sprintf("%-12s", severity))
	}

	sinceStr := ""
	if !a.Since.IsZero() {
		sinceStr = formatTimeAgo(a.Since)
	}
	sinceCol := ui.DimStyle.Render(fmt.Sprintf("%-14s", sinceStr))
	nsCol := ui.DimStyle.Render(fmt.Sprintf("%-12s", ui.SanitizeTerminalText(a.Labels["namespace"])))

	return fmt.Sprintf("  %s %s %s %s", stateStr, sevStr, sinceCol, nsCol)
}

// renderAlertLabels renders the label lines for a single alert.
func renderAlertLabels(lines []string, labels map[string]string, exclude map[string]bool) []string {
	var labelKeys []string
	for k := range labels {
		if !exclude[k] {
			labelKeys = append(labelKeys, k)
		}
	}
	sort.Strings(labelKeys)
	for _, k := range labelKeys {
		// Prometheus alert labels are attacker-controllable (a rule author
		// or the metric source can set arbitrary label keys/values), so
		// sanitize both sides before they hit the screen.
		lines = append(lines, ui.DimStyle.Render(fmt.Sprintf("      %s=%s",
			ui.SanitizeTerminalText(k), ui.SanitizeTerminalText(labels[k]))))
	}
	return lines
}

// formatTimeAgo formats a time.Time as a human-readable relative duration.
func formatTimeAgo(t time.Time) string {
	ago := time.Since(t)
	switch {
	case ago < time.Minute:
		return fmt.Sprintf("%ds ago", int(ago.Seconds()))
	case ago < time.Hour:
		return fmt.Sprintf("%dm ago", int(ago.Minutes()))
	case ago < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(ago.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(ago.Hours()/24))
	}
}
