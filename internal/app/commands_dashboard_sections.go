package app

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// dashboardSection enumerates the parallel fetches that compose the
// cluster dashboard. Each section runs as a separate scheduled task so
// foreground work can preempt mid-flight.
type dashboardSection int

const (
	dashboardSectionNodes dashboardSection = iota
	dashboardSectionPods
	dashboardSectionNamespaces
	dashboardSectionEvents
	dashboardSectionPDB
	dashboardSectionNodeMetrics
)

// pinnedSummaryResult is one pinned resource type's status rollup, rendered
// as inline dashboard metric rows below Pods (dashboardPinnedRows). index
// preserves pin order across the unordered fan-out. notFound flags a pin key
// absent from the cluster's discovery (e.g. a CRD not installed here).
type pinnedSummaryResult struct {
	index       int
	key         string
	displayName string
	summary     ui.ListSummary
	notFound    bool
	err         error
}

// dashboardWidths holds the bar/separator widths used when composing the
// dashboard. They scale with the width the dashboard is rendered at so the
// bars use the available space (wide in fullscreen, compact in the right
// preview pane) without overflowing the column.
type dashboardWidths struct {
	bar     int // cluster bars: node health, pod status, CPU, Mem
	node    int // per-node CPU/Mem bars (two side by side)
	sep     int // horizontal separator rule
	label   int // metric-row label column width (for inline label/bar/summary alignment)
	content int // total content width. Inline rows are truncated to it as a safety net
}

// dashboardMetricLines lays out one cluster metric over two lines: the label +
// bar on the first, and the status summary on its own indented line. Keeping
// the summary off the bar line means a long breakdown (e.g. Running · Pending ·
// Failed · Succeeded) never shrinks the bar. Both lines are truncated to the
// column width so a long summary can't wrap and tear the layout.
func dashboardMetricLines(label, bar, summary string, w dashboardWidths) []string {
	// A label longer than the column (a long pinned CRD display name) is
	// truncated here rather than just left-unpadded: leaving it full-length
	// would push that row's bar to a different column than every other row.
	label = ui.Truncate(label, w.label)
	pad := max(w.label-lipgloss.Width(label), 0)
	barLine := "  " + ui.HelpKeyStyle.Render(label) + strings.Repeat(" ", pad) + "  " + bar
	sumLine := strings.Repeat(" ", 2+w.label+2) + summary
	return []string{
		ansi.Truncate(barLine, w.content, ""),
		ansi.Truncate(sumLine, w.content, ""),
	}
}

// dashboardContentWidth returns the column content width the dashboard will be
// rendered into for the current display mode, matching the view layout.
func (m Model) dashboardContentWidth(twoCol bool) int {
	if !m.fullscreenDashboard {
		// Right preview pane inner width — explorerColumnWidths centralizes
		// the layout math so hideLeftPane / fullscreen modes flow through.
		_, _, rightW := m.explorerColumnWidths()
		return max(rightW-2, 20)
	}
	innerW := m.width - 4 // ActiveColumnStyle border+padding
	if twoCol {
		return max(innerW*60/100, 20) // left column cap (see dashboardColumnWidths)
	}
	return max(innerW, 20)
}

// dashboardWidths derives the metric-row widths from the target content width.
// Bars are uniform and as wide as the column allows. The summary lives on its
// own line, so it no longer constrains the bar. maxPinnedLabel is the longest
// label any pinned row will render (see maxPinnedLabelWidth) - the label
// column widens to fit it, capped at 14, so a long CRD display name never
// pushes that row's bar out of alignment with Nodes/Pods/CPU/Mem. The
// reservation (labelCol + 11) leaves room for the "  " indents, brackets, and
// renderBar's " NNN%" suffix so the bar line still fits contentW.
func (m Model) dashboardWidths(twoCol bool, maxPinnedLabel int) dashboardWidths {
	contentW := m.dashboardContentWidth(twoCol)
	labelCol := min(max(maxPinnedLabel, 5), 14) // "Nodes" / "Pods" / "CPU" / "Mem" at minimum
	// Per-node row is `      CPU [bar] NN%   MEM [bar] NN%`. The two bars share
	// a fixed 31-col overhead (indents, "CPU"/"MEM" labels, brackets, "%",
	// gaps), so split the rest between them to reach the same right edge as the
	// top bars instead of staying noticeably narrower.
	const perNodeOverhead = 31
	return dashboardWidths{
		bar:     min(max(contentW-labelCol-11, 8), 100),
		node:    min(max((contentW-perNodeOverhead)/2, 3), 60),
		sep:     min(max(contentW-2, 16), 120),
		label:   labelCol,
		content: contentW,
	}
}

func (s dashboardSection) String() string {
	switch s {
	case dashboardSectionNodes:
		return "nodes"
	case dashboardSectionPods:
		return "pods"
	case dashboardSectionNamespaces:
		return "namespaces"
	case dashboardSectionEvents:
		return "events"
	case dashboardSectionPDB:
		return "pdbs"
	case dashboardSectionNodeMetrics:
		return "metrics"
	default:
		return "unknown"
	}
}

// fetchDashboardNodes fetches node items and counts ready nodes.
func fetchDashboardNodes(ctx context.Context, kctx string, client *k8s.Client) dashboardData {
	var data dashboardData
	nodeItems, err := client.GetResources(ctx, kctx, "", model.ResourceTypeEntry{
		Kind: "Node", APIGroup: "", APIVersion: "v1", Resource: "nodes", Namespaced: false,
	}, k8s.PreferCache())
	if err == nil {
		data.nodeItems = nodeItems
		data.nodeCount = len(nodeItems)
		data.podCapacity = sumPodCapacity(nodeItems)
		for _, n := range nodeItems {
			if n.Status == "Ready" {
				data.readyNodes++
			}
		}
	}
	return data
}

// fetchDashboardPods fetches pod items and tallies pod stats.
func fetchDashboardPods(ctx context.Context, kctx string, client *k8s.Client) dashboardData {
	var data dashboardData
	podItems, err := client.GetResources(ctx, kctx, "", model.ResourceTypeEntry{
		Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true,
	}, k8s.PreferCache())
	if err == nil {
		data.pods = countPodStats(podItems)
	}
	return data
}

// fetchDashboardNamespaces fetches the namespace count.
func fetchDashboardNamespaces(ctx context.Context, kctx string, client *k8s.Client) dashboardData {
	var data dashboardData
	namespaces, _ := client.GetNamespaces(ctx, kctx)
	data.nsCount = len(namespaces)
	return data
}

// fetchDashboardEvents fetches warning events for the dashboard.
func fetchDashboardEvents(ctx context.Context, kctx string, client *k8s.Client) dashboardData {
	var data dashboardData
	data.warningEvents, data.allWarnings = fetchWarningEvents(ctx, kctx, client)
	return data
}

// fetchDashboardPDB fetches PodDisruptionBudget warnings.
func fetchDashboardPDB(ctx context.Context, kctx string, client *k8s.Client) dashboardData {
	var data dashboardData
	data.pdbWarnings = fetchPDBWarnings(ctx, kctx, client)
	return data
}

// fetchDashboardNodeMetrics fetches per-node resource usage metrics.
func fetchDashboardNodeMetrics(ctx context.Context, kctx string, client *k8s.Client, nodeItems []model.Item) dashboardData {
	var data dashboardData
	data.nodes, data.totalCPUUsed, data.totalCPUAlloc, data.totalMemUsed, data.totalMemAlloc, data.nodeMetricsErr = fetchNodeMetrics(ctx, kctx, client, nodeItems)
	return data
}

// mergeDashboardSection merges a per-section partial result into the
// accumulator. Per-field assignment skips zero-values so a section
// that hasn't arrived yet doesn't clobber another section's data.
func mergeDashboardSection(acc *dashboardData, partial dashboardData) {
	if partial.nodeItems != nil {
		acc.nodeItems = partial.nodeItems
		acc.nodeCount = partial.nodeCount
		acc.readyNodes = partial.readyNodes
		acc.podCapacity = partial.podCapacity
	}
	if partial.pods.total > 0 {
		acc.pods = partial.pods
	}
	if partial.nsCount > 0 {
		acc.nsCount = partial.nsCount
	}
	if partial.warningEvents != nil || partial.allWarnings != nil {
		acc.warningEvents = partial.warningEvents
		acc.allWarnings = partial.allWarnings
	}
	if partial.pdbWarnings != nil {
		acc.pdbWarnings = partial.pdbWarnings
	}
	if partial.nodes != nil || partial.nodeMetricsErr != nil {
		acc.nodes = partial.nodes
		acc.totalCPUUsed = partial.totalCPUUsed
		acc.totalCPUAlloc = partial.totalCPUAlloc
		acc.totalMemUsed = partial.totalMemUsed
		acc.totalMemAlloc = partial.totalMemAlloc
		acc.nodeMetricsErr = partial.nodeMetricsErr
	}
	if len(partial.pinnedSummaries) > 0 {
		acc.pinnedSummaries = append(acc.pinnedSummaries, partial.pinnedSummaries...)
	}
}

// pinnedSummaryCmds builds one scheduled cmd per pinned summary key. A key
// unresolved against the cluster's discovery (CRD absent or discovery still
// warming) normally gets a synchronous notFound placeholder instead of a
// scheduled fetch, so the section count still balances against total. A pin
// resolved before discovery finishes renders the "(not installed in this
// cluster)" placeholder until the next dashboard refresh re-resolves it - a
// known transient, not a permanent misclassification.
//
// silentSkip is set when pins is the built-in default set (nothing pinned):
// an unresolved default is dropped entirely instead - no cmd, no placeholder
// - since a default set should never surface "not installed" noise for CRDs
// the user never asked to pin. The caller (loadDashboardFor) must size total
// to match: countResolvedPins gives the same count this loop actually
// schedules.
func (m Model) pinnedSummaryCmds(kctx string, gen uint64, client *k8s.Client, pins []string, discovered []model.ResourceTypeEntry, total int, silentSkip bool, sectionTarget func(string) string) []tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(pins))
	for i, pk := range pins {
		key := "pinned:" + pk
		entry, ok := resolvePinnedSummaryEntry(discovered, pk)
		if !ok {
			if silentSkip {
				continue
			}
			res := pinnedSummaryResult{index: i, key: pk, displayName: pk, notFound: true}
			cmds = append(cmds, func() tea.Msg {
				return dashboardPartialMsg{
					context: kctx, gen: gen, key: key, total: total,
					data: dashboardData{pinnedSummaries: []pinnedSummaryResult{res}},
				}
			})
			continue
		}
		index := i
		cmds = append(cmds, m.scheduleK8sCall(scheduler.PriorityLow, scheduler.KindDashboard,
			"Dashboard: "+model.DisplayNameFor(entry)+" summary", sectionTarget(key),
			func(ctx context.Context) tea.Msg {
				return dashboardPartialMsg{
					context: kctx, gen: gen, key: key, total: total,
					data: fetchPinnedSummary(ctx, kctx, client, index, pk, entry),
				}
			}))
	}
	return cmds
}

// resolvePinnedSummaryEntry finds the discovery entry for a version-agnostic
// pin key ("group/resource").
func resolvePinnedSummaryEntry(entries []model.ResourceTypeEntry, key string) (model.ResourceTypeEntry, bool) {
	for _, e := range entries {
		if e.APIGroup+"/"+e.Resource == key {
			return e, true
		}
	}
	return model.ResourceTypeEntry{}, false
}

// countResolvedPins counts how many pin keys resolve against discovered.
// loadDashboardFor uses this to size the fan-out total when defaults are in
// play: pinnedSummaryCmds schedules nothing for an unresolved default (see
// silentSkip above), so an unresolved one must not count toward total either,
// or the accumulator would wait forever for a section that never arrives.
func countResolvedPins(pins []string, discovered []model.ResourceTypeEntry) int {
	n := 0
	for _, pk := range pins {
		if _, ok := resolvePinnedSummaryEntry(discovered, pk); ok {
			n++
		}
	}
	return n
}

// fetchPinnedSummary lists one pinned resource type cluster-wide and rolls it
// up with the same summary builder the preview band uses.
func fetchPinnedSummary(ctx context.Context, kctx string, client *k8s.Client, index int, key string, entry model.ResourceTypeEntry) dashboardData {
	res := pinnedSummaryResult{index: index, key: key, displayName: model.DisplayNameFor(entry)}
	items, err := client.GetResources(ctx, kctx, "", entry, k8s.PreferCache())
	if err != nil {
		res.err = err
		logger.Warn("Pinned summary list failed", "key", key, "error", err)
	} else {
		res.summary = ui.BuildListSummary(entry.Kind, items)
	}
	return dashboardData{pinnedSummaries: []pinnedSummaryResult{res}}
}

// renderBar renders a horizontal bar graph like [████████░░░░░░░░] 52%.
// The filled portion is colored based on usage percentage: green (<75%), orange (75-90%), red (>90%).
func renderBar(used, total int64, width int) string {
	if total <= 0 {
		return "[" + strings.Repeat("░", width) + "]  N/A"
	}
	pct := float64(used) / float64(total) * 100
	if pct > 100 {
		pct = 100
	}
	filled := int(pct / 100 * float64(width))
	filled = min(filled, width)
	empty := width - filled

	filledStr := strings.Repeat("█", filled)
	emptyStr := strings.Repeat("░", empty)

	var style lipgloss.Style
	switch {
	case pct >= 90:
		style = ui.StatusFailed
	case pct >= 75:
		style = ui.StatusProgressing
	default:
		style = ui.StatusRunning
	}

	// Pad the percentage to a fixed width ("  5%", " 14%", "100%") so bars
	// placed side by side (per-node CPU/MEM) stay column-aligned regardless of
	// the digit count.
	return "[" + style.Render(filledStr) + emptyStr + "] " + fmt.Sprintf("%3.0f%%", pct)
}

// renderStackedBar renders a stacked bar showing proportions of multiple segments.
func renderStackedBar(segments []struct {
	count int
	style lipgloss.Style
}, total, width int,
) string {
	if total <= 0 {
		return "[" + strings.Repeat("░", width) + "]"
	}
	// When the segments fully pack the total, the last one absorbs the rounding
	// remainder so no stray empty cells appear. When they sum to less than the
	// total (e.g. scheduled pods below cluster pod capacity), the shortfall is
	// genuine headroom and must render as empty cells, so the last segment is
	// sized by its own proportion instead.
	sumCounts := 0
	for _, seg := range segments {
		sumCounts += seg.count
	}
	packed := sumCounts >= total
	var barBuilder strings.Builder
	used := 0
	for i, seg := range segments {
		chars := int(float64(seg.count) / float64(total) * float64(width))
		// Give any non-zero segment at least one cell so a handful of Failed or
		// Pending pods in a large cluster stay visible instead of rounding away.
		if seg.count > 0 && chars == 0 {
			chars = 1
		}
		// Last segment absorbs the rounding remainder, but only when packed;
		// otherwise the shortfall is real headroom and must stay empty.
		if i == len(segments)-1 && packed {
			chars = width - used
		}
		if chars < 0 {
			chars = 0
		}
		if used+chars > width {
			chars = width - used
		}
		barBuilder.WriteString(seg.style.Render(strings.Repeat("█", chars)))
		used += chars
	}
	if used < width {
		barBuilder.WriteString(strings.Repeat("░", width-used))
	}
	return "[" + barBuilder.String() + "]"
}

// dashboardHeaderSection renders the cluster header, node, namespace, and pod sections.
func dashboardHeaderSection(lines []string, data dashboardData, w dashboardWidths) []string {
	lines = append(lines, ui.DimStyle.Bold(true).Render("  CLUSTER DASHBOARD"))
	lines = append(lines, "")

	// Nodes: health bar (green Ready / red NotReady) + inline summary.
	if data.nodeCount > 0 {
		segments := []struct {
			count int
			style lipgloss.Style
		}{
			{data.readyNodes, ui.StatusRunning},
			{data.nodeCount - data.readyNodes, ui.StatusFailed},
		}
		nodeBar := renderStackedBar(segments, data.nodeCount, w.bar)
		lines = append(lines, dashboardMetricLines("Nodes", nodeBar, nodeSummaryStr(data), w)...)
	}
	lines = append(lines, "")

	// Namespaces.
	lines = append(lines, fmt.Sprintf("  %s %s",
		ui.HelpKeyStyle.Render("Namespaces:"),
		ui.NormalStyle.Render(fmt.Sprintf("%d", data.nsCount))))
	lines = append(lines, "")

	// Pods: status bar (green Running / amber Pending / red Failed / grey
	// Succeeded) + inline breakdown. "Other" (Terminating/Unknown + rounding
	// slack) is the neutral last segment so renderStackedBar's remainder-fill
	// never paints leftover space red as phantom failures. The denominator is
	// the cluster's pod capacity (sum of nodes' allocatable pods) when known, so
	// the unfilled tail reads as unallocated headroom rather than 100% usage.
	if data.pods.total > 0 {
		segments := []struct {
			count int
			style lipgloss.Style
		}{
			{data.pods.running, ui.StatusRunning},
			{data.pods.pending, ui.StatusProgressing},
			{data.pods.failed, ui.StatusFailed},
			{data.pods.succeeded, ui.StatusOther},
			{podOther(data.pods), ui.DimStyle},
		}
		denom := podBarDenominator(data)
		podBar := renderStackedBar(segments, denom, w.bar)
		lines = append(lines, dashboardMetricLines("Pods", podBar, podSummaryStr(data), w)...)
	}
	// No trailing blank here: pinned rows (dashboardPinnedRows) render
	// directly below Pods inside the same block. composeDashboard adds the
	// separating blank after them.

	return lines
}

// dashboardResourcesSection renders the cluster resources (CPU/Mem) section.
// spark draws a usage-history sparkline under each bar. False renders the
// section exactly as it did before sparklines existed.
func dashboardResourcesSection(lines []string, data dashboardData, w dashboardWidths, spark bool) []string {
	if data.totalCPUAlloc <= 0 && data.totalMemAlloc <= 0 {
		return lines
	}
	lines = append(lines, ui.DimStyle.Render("  "+strings.Repeat("─", w.sep)))
	lines = append(lines, ui.DimStyle.Bold(true).Render("  CLUSTER RESOURCES"))
	if data.nodeMetricsErr != nil {
		lines = append(lines, ui.StatusProgressing.Render("  (metrics-server unavailable)"))
	}
	lines = append(lines, "")
	if data.totalCPUAlloc > 0 {
		cpuBar := renderBar(data.totalCPUUsed, data.totalCPUAlloc, w.bar)
		lines = append(lines, dashboardMetricLines("CPU", cpuBar, cpuSummaryStr(data), w)...)
		lines = appendDashboardSparkline(lines, data.cpuSeries, w, spark)
	}
	if data.totalMemAlloc > 0 {
		memBar := renderBar(data.totalMemUsed, data.totalMemAlloc, w.bar)
		lines = append(lines, dashboardMetricLines("Mem", memBar, memSummaryStr(data), w)...)
		lines = appendDashboardSparkline(lines, data.memSeries, w, spark)
	}
	lines = append(lines, "")
	return lines
}

// appendDashboardSparkline adds a usage-history line under a resource bar,
// sized to the bar's full width rather than the table's column cap: nothing
// truncates this section, unlike a table cell.
func appendDashboardSparkline(lines []string, series k8s.MetricSeries, w dashboardWidths, spark bool) []string {
	if !spark {
		return lines
	}
	drawn := ui.RenderSparkline(series.Points, w.bar)
	if drawn == "" {
		return lines
	}
	// Matches the lead dashboardMetricLines gives its summary line, so the
	// sparkline sits under the bar rather than under the label.
	lead := strings.Repeat(" ", 2+w.label+2)
	// Truncated like every sibling line: w.bar has a floor of 8, so a narrow
	// pane with a long label leaves lead+bar wider than the content column.
	line := ansi.Truncate(lead+drawn, w.content, "")
	return append(lines, ui.DimStyle.Render(line))
}

// dashboardNodesSection renders the per-node breakdown.
func dashboardNodesSection(lines []string, data dashboardData, w dashboardWidths) []string {
	if len(data.nodes) == 0 || (data.totalCPUAlloc <= 0 && data.totalMemAlloc <= 0) {
		return lines
	}
	lines = append(lines, ui.DimStyle.Render("  "+strings.Repeat("─", w.sep)))
	lines = append(lines, ui.DimStyle.Bold(true).Render("  NODES"))
	lines = append(lines, "")

	// Sanitize before measuring: a control character changes the width, and
	// the old byte slice below could also cut a rune in half.
	maxNameLen := 0
	for _, n := range data.nodes {
		if w := lipgloss.Width(ui.SanitizeTerminalText(n.name)); w > maxNameLen {
			maxNameLen = w
		}
	}
	if maxNameLen > 48 {
		maxNameLen = 48
	}

	for _, n := range data.nodes {
		name := ui.Truncate(ui.SanitizeTerminalText(n.name), maxNameLen)
		statusDot := nodeStatusDot(data.nodeItems, n.name)
		roleStr := nodeRoleStr(data.nodeItems, n.name)

		cpuBar := renderBar(n.cpuUsed, n.cpuAlloc, w.node)
		memBar := renderBar(n.memUsed, n.memAlloc, w.node)
		nameLine := fmt.Sprintf("  %s %s%s", statusDot, name, roleStr)
		barLine := fmt.Sprintf("      %s %s   %s %s",
			ui.HelpKeyStyle.Render("CPU"), cpuBar,
			ui.HelpKeyStyle.Render("MEM"), memBar)
		// Truncate to the column so a narrow pane can't wrap the row.
		lines = append(lines, ansi.Truncate(nameLine, w.content, ""))
		lines = append(lines, ansi.Truncate(barLine, w.content, ""))
	}
	lines = append(lines, "")
	return lines
}

// nodeStatusDot returns a colored dot indicating whether a node is Ready.
func nodeStatusDot(nodeItems []model.Item, name string) string {
	for _, ni := range nodeItems {
		if ni.Name == name && ni.Status != "Ready" {
			return ui.StatusFailed.Render("●")
		}
	}
	return ui.StatusRunning.Render("●")
}

// nodeRoleStr returns a styled role label for a node.
func nodeRoleStr(nodeItems []model.Item, name string) string {
	for _, ni := range nodeItems {
		if ni.Name == name {
			for _, kv := range ni.Columns {
				if kv.Key == "Role" && kv.Value != "" {
					return " " + ui.DimStyle.Render("["+ui.SanitizeTerminalText(kv.Value)+"]")
				}
			}
			return ""
		}
	}
	return ""
}

// dashboardWarningBody renders the warning lines (pod/node health + PDB) with
// no separators or surrounding blanks. Returns nil when there's nothing to
// warn about. Shared by the single-column section and the two-column right
// column so both read identically.
func dashboardWarningBody(data dashboardData) []string {
	notReadyWorkers := countNotReadyWorkerNodes(data.nodeItems)
	hasHealthWarnings := data.pods.failed > 0 || data.pods.crashLoop > 0 || notReadyWorkers > 0
	if !hasHealthWarnings && len(data.pdbWarnings) == 0 {
		return nil
	}

	var out []string
	out = append(out, ui.DimStyle.Bold(true).Render("  WARNINGS"), "")
	if data.pods.failed > 0 {
		out = append(out, ui.StatusFailed.Render(fmt.Sprintf("  ! %d pod(s) in failed state", data.pods.failed)))
	}
	if notReadyWorkers > 0 {
		out = append(out, ui.StatusFailed.Render(fmt.Sprintf("  ! %d worker node(s) not ready", notReadyWorkers)))
	}
	if data.pods.crashLoop > 0 {
		out = append(out, ui.StatusFailed.Render(fmt.Sprintf("  ! %d pod(s) in CrashLoopBackOff", data.pods.crashLoop)))
	}
	if len(data.pdbWarnings) > 0 {
		out = append(out, "", ui.DimStyle.Bold(true).Render("  PDB WARNINGS"), "")
		for _, pw := range data.pdbWarnings {
			out = append(out, fmt.Sprintf("  %s %s/%s",
				ui.StatusProgressing.Render("⊘"),
				ui.DimStyle.Render(ui.SanitizeTerminalText(pw.namespace)),
				ui.StatusProgressing.Render(ui.SanitizeTerminalText(pw.name))))
			out = append(out, ui.DimStyle.Render(fmt.Sprintf("       MinAvail=%s  Healthy=%s  DisruptionsAllowed=%s",
				pw.minAvailable, pw.currentHealthy, pw.disruptionsAllowed)))
		}
	}
	return out
}

// dashboardWarningsSection renders the warnings for the single-column layout,
// led by a separator so it's clearly divided from the section above it.
func dashboardWarningsSection(lines []string, data dashboardData, w dashboardWidths) []string {
	body := dashboardWarningBody(data)
	if len(body) == 0 {
		return lines
	}
	lines = append(lines, ui.DimStyle.Render("  "+strings.Repeat("─", w.sep)))
	lines = append(lines, body...)
	lines = append(lines, "")
	return lines
}

// dashboardWarningsColumn renders the warnings for the top of the two-column
// right column (above RECENT EVENTS). No separator — the right column wraps
// each line, so a full-width rule would look wrong there.
func dashboardWarningsColumn(data dashboardData) []string {
	body := dashboardWarningBody(data)
	if len(body) == 0 {
		return nil
	}
	return append([]string{""}, body...)
}

// countNotReadyWorkerNodes counts worker nodes that are not Ready.
func countNotReadyWorkerNodes(nodeItems []model.Item) int {
	count := 0
	for _, ni := range nodeItems {
		if ni.Status != "Ready" {
			isControlPlane := false
			for _, kv := range ni.Columns {
				if kv.Key == "Role" && strings.Contains(kv.Value, "control-plane") {
					isControlPlane = true
					break
				}
			}
			if !isControlPlane {
				count++
			}
		}
	}
	return count
}

// eventColumnFields extracts reason, object, message, and count from event columns.
type eventColumnFields struct {
	reason, object, message, count string
}

// extractEventFields extracts common fields from an event's columns.
// Reason/Object/Message come straight from a cluster Event resource and
// render on a single dashboard line with no wrap support, so they're
// sanitized here - the single place both dashboard event sections pull
// from - before any caller truncates or measures their width.
func extractEventFields(ev model.Item) eventColumnFields {
	var f eventColumnFields
	for _, kv := range ev.Columns {
		switch kv.Key {
		case "Reason":
			f.reason = ui.SanitizeTerminalText(kv.Value)
		case "Object":
			f.object = ui.SanitizeTerminalText(kv.Value)
		case "Message":
			f.message = ui.SanitizeTerminalText(kv.Value)
		case "Count":
			f.count = ui.SanitizeTerminalText(kv.Value)
		}
	}
	return f
}

// dashboardInlineEventsSection renders the inline warning events section
// (single-column layout only), led by a separator to divide it from the
// section above.
func dashboardInlineEventsSection(lines []string, warningEvents []model.Item, w dashboardWidths) []string {
	if len(warningEvents) == 0 {
		return lines
	}
	lines = append(lines, ui.DimStyle.Render("  "+strings.Repeat("─", w.sep)))
	lines = append(lines, ui.DimStyle.Bold(true).Render("  RECENT WARNING EVENTS"))
	lines = append(lines, "")
	for _, ev := range warningEvents {
		f := extractEventFields(ev)
		msg := f.message
		if len(msg) > 60 {
			msg = msg[:57] + "..."
		}
		countLabel := ""
		if f.count != "" && f.count != "1" {
			countLabel = ui.DimStyle.Render(fmt.Sprintf("(x%s) ", f.count))
		}
		line := fmt.Sprintf("  %s %s %s%s %s",
			ui.StatusProgressing.Render("⚠"),
			ui.DimStyle.Render(fmt.Sprintf("%-4s", ev.Age)),
			countLabel,
			ui.StatusFailed.Render(f.reason+":"),
			ui.NormalStyle.Render(f.object))
		lines = append(lines, line)
		if msg != "" {
			lines = append(lines, fmt.Sprintf("       %s", ui.DimStyle.Render(msg)))
		}
	}
	return lines
}

// dashboardEventsColumn builds the dedicated events column for two-column layout.
func dashboardEventsColumn(allWarningEvents []model.Item) []string {
	var eventLines []string
	eventLines = append(eventLines, "")
	eventLines = append(eventLines, ui.DimStyle.Bold(true).Render("  RECENT EVENTS"))
	eventLines = append(eventLines, "")

	columnEvents := allWarningEvents
	if len(columnEvents) > 30 {
		columnEvents = columnEvents[:30]
	}

	if len(columnEvents) == 0 {
		eventLines = append(eventLines, ui.StatusRunning.Render("  No warning events"))
		return eventLines
	}

	for i, ev := range columnEvents {
		f := extractEventFields(ev)
		countLabel := ""
		if f.count != "" && f.count != "1" {
			countLabel = ui.DimStyle.Render(fmt.Sprintf("(x%s) ", f.count))
		}
		nsLabel := ""
		if ev.Namespace != "" {
			nsLabel = ui.DimStyle.Render("[" + ui.SanitizeTerminalText(ev.Namespace) + "] ")
		}
		line := fmt.Sprintf("  %s %s %s%s%s %s",
			ui.StatusProgressing.Render("⚠"),
			ui.DimStyle.Render(fmt.Sprintf("%-4s", ev.Age)),
			countLabel,
			nsLabel,
			ui.StatusFailed.Render(f.reason+":"),
			ui.NormalStyle.Render(f.object))
		eventLines = append(eventLines, line)
		if f.message != "" {
			eventLines = append(eventLines, fmt.Sprintf("       %s", ui.DimStyle.Render(f.message)))
		}
		if i < len(columnEvents)-1 {
			eventLines = append(eventLines, "")
		}
	}
	return eventLines
}
