package app

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/app/bgtasks"
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
	crashLoop int
}

// dashboardData holds all fetched data for the cluster dashboard.
type dashboardData struct {
	nodeItems      []model.Item
	nodeCount      int
	readyNodes     int
	pods           podStats
	nsCount        int
	warningEvents  []model.Item
	allWarnings    []model.Item
	pdbWarnings    []pdbWarning
	nodes          []nodeInfo
	totalCPUUsed   int64
	totalCPUAlloc  int64
	totalMemUsed   int64
	totalMemAlloc  int64
	nodeMetricsErr error
}

// loadDashboard fetches cluster summary data and renders a dashboard.
func (m Model) loadDashboard() tea.Cmd {
	kctx := m.nav.Context
	client := m.client
	reqCtx := m.reqCtx
	return m.trackBgTask(
		bgtasks.KindDashboard,
		"Cluster dashboard",
		bgtaskTarget(kctx, ""),
		func() tea.Msg {
			data := fetchDashboardData(reqCtx, kctx, client)

			var lines []string
			lines = append(lines, "")
			lines = dashboardHeaderSection(lines, data)
			lines = dashboardResourcesSection(lines, data)
			lines = dashboardNodesSection(lines, data)
			lines = dashboardWarningsSection(lines, data)
			lines = dashboardInlineEventsSection(lines, data.warningEvents)
			lines = append(lines, "")

			eventLines := dashboardEventsColumn(data.allWarnings)

			return dashboardLoadedMsg{
				content: strings.Join(lines, "\n"),
				events:  strings.Join(eventLines, "\n"),
				context: kctx,
			}
		},
	)
}

// fetchDashboardData fetches all cluster data needed for the dashboard.
func fetchDashboardData(reqCtx context.Context, kctx string, client *k8s.Client) dashboardData {
	var data dashboardData

	// Fetch nodes.
	nodeItems, err := client.GetResources(reqCtx, kctx, "", model.ResourceTypeEntry{
		Kind: "Node", APIGroup: "", APIVersion: "v1", Resource: "nodes", Namespaced: false,
	})
	if err == nil {
		data.nodeItems = nodeItems
		data.nodeCount = len(nodeItems)
		for _, n := range nodeItems {
			if n.Status == "Ready" {
				data.readyNodes++
			}
		}
	}

	// Fetch pods.
	podItems, err := client.GetResources(reqCtx, kctx, "", model.ResourceTypeEntry{
		Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true,
	})
	if err == nil {
		data.pods = countPodStats(podItems)
	}

	// Fetch namespaces.
	namespaces, _ := client.GetNamespaces(reqCtx, kctx)
	data.nsCount = len(namespaces)

	// Fetch warning events.
	data.warningEvents, data.allWarnings = fetchWarningEvents(reqCtx, kctx, client)

	// Fetch PDB warnings.
	data.pdbWarnings = fetchPDBWarnings(reqCtx, kctx, client)

	// Node metrics.
	data.nodes, data.totalCPUUsed, data.totalCPUAlloc, data.totalMemUsed, data.totalMemAlloc, data.nodeMetricsErr = fetchNodeMetrics(reqCtx, kctx, client, nodeItems)

	return data
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
	})
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
	kctx := m.nav.Context
	client := m.client
	ns := m.effectiveNamespace()
	return m.trackBgTask(
		bgtasks.KindDashboard,
		"Monitoring dashboard",
		bgtaskTarget(kctx, ns),
		func() tea.Msg {
			var lines []string
			lines = append(lines, "")
			lines = append(lines, ui.DimStyle.Bold(true).Render("  MONITORING OVERVIEW"))
			lines = append(lines, "")

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			alerts, err := client.GetAllActiveAlerts(ctx, kctx, ns)
			if err != nil {
				lines = append(lines, ui.DimStyle.Render("  Prometheus/Alertmanager not reachable"))
				lines = append(lines, ui.DimStyle.Render("  "+err.Error()))
				lines = append(lines, "")
				lines = append(lines, ui.DimStyle.Render("  Searched in well-known namespaces:"))
				lines = append(lines, ui.DimStyle.Render("  monitoring, prometheus, observability, kube-prometheus-stack"))
				lines = append(lines, "")
				return monitoringDashboardMsg{content: strings.Join(lines, "\n"), context: kctx}
			}

			lines = monitoringAlertSummary(lines, alerts)
			lines = append(lines, "")
			sortAlerts(alerts)
			lines = monitoringAlertTable(lines, alerts)

			lines = append(lines, "")
			return monitoringDashboardMsg{content: strings.Join(lines, "\n"), context: kctx}
		},
	)
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
	var stateStr string
	switch a.State {
	case "firing":
		stateStr = ui.StatusFailed.Render(fmt.Sprintf("%-10s", a.State))
	case "pending":
		stateStr = ui.StatusProgressing.Render(fmt.Sprintf("%-10s", a.State))
	default:
		stateStr = ui.DimStyle.Render(fmt.Sprintf("%-10s", a.State))
	}

	var sevStr string
	switch a.Severity {
	case "critical":
		sevStr = ui.StatusFailed.Bold(true).Render(fmt.Sprintf("%-12s", a.Severity))
	case "warning":
		sevStr = ui.StatusProgressing.Render(fmt.Sprintf("%-12s", a.Severity))
	default:
		sevStr = ui.DimStyle.Render(fmt.Sprintf("%-12s", a.Severity))
	}

	sinceStr := ""
	if !a.Since.IsZero() {
		sinceStr = formatTimeAgo(a.Since)
	}
	sinceCol := ui.DimStyle.Render(fmt.Sprintf("%-14s", sinceStr))
	nsCol := ui.DimStyle.Render(fmt.Sprintf("%-12s", a.Labels["namespace"]))

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
		lines = append(lines, ui.DimStyle.Render(fmt.Sprintf("      %s=%s", k, labels[k])))
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
