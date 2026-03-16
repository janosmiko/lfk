package app

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// loadDashboard fetches cluster summary data and renders a dashboard.
// renderBar renders a horizontal bar graph like [████████░░░░░░░░] 52%.
// The filled portion is colored based on usage percentage: green (<75%), orange (75-90%), red (>90%).
func renderBar(used, total int64, width int) string {
	if total <= 0 {
		return "[" + strings.Repeat("\u2591", width) + "] N/A"
	}
	pct := float64(used) / float64(total) * 100
	if pct > 100 {
		pct = 100
	}
	filled := int(pct / 100 * float64(width))
	if filled > width {
		filled = width
	}
	empty := width - filled

	filledStr := strings.Repeat("\u2588", filled)
	emptyStr := strings.Repeat("\u2591", empty)

	var style lipgloss.Style
	switch {
	case pct >= 90:
		style = ui.StatusFailed
	case pct >= 75:
		style = ui.StatusPending
	default:
		style = ui.StatusRunning
	}

	return "[" + style.Render(filledStr) + emptyStr + "] " + fmt.Sprintf("%.0f%%", pct)
}

// renderStackedBar renders a stacked bar showing proportions of multiple segments.
func renderStackedBar(segments []struct {
	count int
	style lipgloss.Style
}, total, width int,
) string {
	if total <= 0 {
		return "[" + strings.Repeat("\u2591", width) + "]"
	}
	bar := ""
	used := 0
	for i, seg := range segments {
		chars := int(float64(seg.count) / float64(total) * float64(width))
		// Last segment gets remaining chars to avoid rounding issues.
		if i == len(segments)-1 {
			chars = width - used
		}
		if chars < 0 {
			chars = 0
		}
		if used+chars > width {
			chars = width - used
		}
		bar += seg.style.Render(strings.Repeat("\u2588", chars))
		used += chars
	}
	if used < width {
		bar += strings.Repeat("\u2591", width-used)
	}
	return "[" + bar + "]"
}

func (m Model) loadDashboard() tea.Cmd {
	kctx := m.nav.Context
	client := m.client
	reqCtx := m.reqCtx
	return func() tea.Msg {
		var lines []string
		lines = append(lines, "")

		// Fetch nodes.
		nodeItems, err := client.GetResources(reqCtx, kctx, "", model.ResourceTypeEntry{
			Kind: "Node", APIGroup: "", APIVersion: "v1", Resource: "nodes", Namespaced: false,
		})
		nodeCount := 0
		readyNodes := 0
		if err == nil {
			nodeCount = len(nodeItems)
			for _, n := range nodeItems {
				if n.Status == "Ready" {
					readyNodes++
				}
			}
		}

		// Fetch all pods across namespaces.
		podItems, err := client.GetResources(reqCtx, kctx, "", model.ResourceTypeEntry{
			Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true,
		})
		podCount := 0
		runningPods := 0
		failedPods := 0
		pendingPods := 0
		crashLoopPods := 0
		if err == nil {
			podCount = len(podItems)
			for _, p := range podItems {
				switch p.Status {
				case "Running":
					runningPods++
				case "CrashLoopBackOff":
					failedPods++
					crashLoopPods++
				case "Failed", "Error", "ImagePullBackOff", "ErrImagePull", "OOMKilled":
					failedPods++
				case "Pending", "ContainerCreating", "Init":
					pendingPods++
				}
			}
		}

		// Fetch namespaces.
		namespaces, _ := client.GetNamespaces(reqCtx, kctx)
		nsCount := len(namespaces)

		// Fetch warning events.
		eventItems, _ := client.GetResources(reqCtx, kctx, "", model.ResourceTypeEntry{
			Kind: "Event", APIGroup: "", APIVersion: "v1", Resource: "events", Namespaced: true,
		})
		var warningEvents []model.Item
		for _, e := range eventItems {
			if e.Status == "Warning" {
				warningEvents = append(warningEvents, e)
			}
		}
		// Sort by creation time (most recent first) and limit to 10.
		sort.Slice(warningEvents, func(i, j int) bool {
			return warningEvents[i].CreatedAt.After(warningEvents[j].CreatedAt)
		})
		if len(warningEvents) > 10 {
			warningEvents = warningEvents[:10]
		}

		// Fetch PodDisruptionBudgets to detect violations.
		type pdbWarning struct {
			name               string
			namespace          string
			minAvailable       string
			currentHealthy     string
			disruptionsAllowed string
		}
		var pdbWarnings []pdbWarning
		pdbItems, pdbErr := client.GetResources(reqCtx, kctx, "", model.ResourceTypeEntry{
			Kind: "PodDisruptionBudget", APIGroup: "policy", APIVersion: "v1", Resource: "poddisruptionbudgets", Namespaced: true,
		})
		if pdbErr == nil {
			for _, pdb := range pdbItems {
				var minAvail, currentHealthy, disruptionsAllowed string
				var disruptionsVal int64 = -1
				var currentVal int64 = -1
				var minAvailVal int64 = -1
				for _, kv := range pdb.Columns {
					switch kv.Key {
					case "Min Available":
						minAvail = kv.Value
						// Try to parse as integer; percentage values won't parse.
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
				// Flag PDBs where no disruptions are allowed or healthy pods are at/below minimum.
				atRisk := disruptionsVal == 0
				if !atRisk && minAvailVal >= 0 && currentVal >= 0 {
					atRisk = currentVal <= minAvailVal
				}
				if atRisk {
					pdbWarnings = append(pdbWarnings, pdbWarning{
						name:               pdb.Name,
						namespace:          pdb.Namespace,
						minAvailable:       minAvail,
						currentHealthy:     currentHealthy,
						disruptionsAllowed: disruptionsAllowed,
					})
				}
			}
		}

		// Node metrics: per-node and totals.
		nodeMetrics, _ := client.GetAllNodeMetrics(reqCtx, kctx)
		type nodeInfo struct {
			name                                 string
			cpuUsed, cpuAlloc, memUsed, memAlloc int64
		}
		nodes := make([]nodeInfo, 0, len(nodeItems))
		var totalCPUUsed, totalCPUAlloc, totalMemUsed, totalMemAlloc int64
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

		// Build dashboard content.
		lines = append(lines, ui.DimStyle.Bold(true).Render("  CLUSTER OVERVIEW"))
		lines = append(lines, "")

		// Nodes section.
		nodeStatus := ui.StatusRunning.Render(fmt.Sprintf("%d Ready", readyNodes))
		if readyNodes < nodeCount {
			notReady := nodeCount - readyNodes
			nodeStatus += " " + ui.StatusFailed.Render(fmt.Sprintf("%d NotReady", notReady))
		}
		lines = append(lines, fmt.Sprintf("  %s %s  %s",
			ui.HelpKeyStyle.Render("Nodes:"),
			ui.NormalStyle.Render(fmt.Sprintf("%d", nodeCount)),
			nodeStatus))

		// Node readiness bar.
		if nodeCount > 0 {
			nodeBar := renderBar(int64(readyNodes), int64(nodeCount), 30)
			lines = append(lines, fmt.Sprintf("  %s %s",
				ui.HelpKeyStyle.Render("           "),
				nodeBar))
		}

		lines = append(lines, "")

		// Namespaces.
		lines = append(lines, fmt.Sprintf("  %s %s",
			ui.HelpKeyStyle.Render("Namespaces:"),
			ui.NormalStyle.Render(fmt.Sprintf("%d", nsCount))))

		lines = append(lines, "")
		lines = append(lines, ui.DimStyle.Render("  "+strings.Repeat("\u2500", 50)))

		// Pods section.
		podStatus := ui.StatusRunning.Render(fmt.Sprintf("%d Running", runningPods))
		if failedPods > 0 {
			podStatus += " " + ui.StatusFailed.Render(fmt.Sprintf("%d Failed", failedPods))
		}
		if pendingPods > 0 {
			podStatus += " " + ui.StatusPending.Render(fmt.Sprintf("%d Pending", pendingPods))
		}
		lines = append(lines, fmt.Sprintf("  %s %s  %s",
			ui.HelpKeyStyle.Render("Pods:"),
			ui.NormalStyle.Render(fmt.Sprintf("%d", podCount)),
			podStatus))

		// Pod status stacked bar.
		if podCount > 0 {
			segments := []struct {
				count int
				style lipgloss.Style
			}{
				{runningPods, ui.StatusRunning},
				{pendingPods, ui.StatusPending},
				{failedPods, ui.StatusFailed},
			}
			podBar := renderStackedBar(segments, podCount, 30)
			lines = append(lines, fmt.Sprintf("  %s %s",
				ui.HelpKeyStyle.Render("           "),
				podBar))
		}

		lines = append(lines, "")
		lines = append(lines, ui.DimStyle.Render("  "+strings.Repeat("\u2500", 50)))

		// Cluster resources.
		if totalCPUAlloc > 0 || totalMemAlloc > 0 {
			lines = append(lines, ui.DimStyle.Bold(true).Render("  CLUSTER RESOURCES"))
			lines = append(lines, "")
			if totalCPUAlloc > 0 {
				cpuBar := renderBar(totalCPUUsed, totalCPUAlloc, 30)
				lines = append(lines, fmt.Sprintf("  %s %s  %s / %s",
					ui.HelpKeyStyle.Render("CPU:"),
					cpuBar,
					ui.FormatCPU(totalCPUUsed),
					ui.FormatCPU(totalCPUAlloc)))
			}
			if totalMemAlloc > 0 {
				memBar := renderBar(totalMemUsed, totalMemAlloc, 30)
				lines = append(lines, fmt.Sprintf("  %s %s  %s / %s",
					ui.HelpKeyStyle.Render("Mem:"),
					memBar,
					ui.FormatMemory(totalMemUsed),
					ui.FormatMemory(totalMemAlloc)))
			}
			lines = append(lines, "")
			lines = append(lines, ui.DimStyle.Render("  "+strings.Repeat("\u2500", 50)))
		}

		// Per-node breakdown.
		if len(nodes) > 0 && (totalCPUAlloc > 0 || totalMemAlloc > 0) {
			lines = append(lines, ui.DimStyle.Bold(true).Render("  NODES"))
			lines = append(lines, "")

			// Find max node name length for alignment.
			maxNameLen := 0
			for _, n := range nodes {
				if len(n.name) > maxNameLen {
					maxNameLen = len(n.name)
				}
			}
			if maxNameLen > 48 {
				maxNameLen = 48
			}

			for _, n := range nodes {
				name := n.name
				if len(name) > maxNameLen {
					name = name[:maxNameLen]
				}

				// Status indicator dot.
				statusDot := ui.StatusRunning.Render("\u25cf")
				for _, ni := range nodeItems {
					if ni.Name == n.name && ni.Status != "Ready" {
						statusDot = ui.StatusFailed.Render("\u25cf")
						break
					}
				}

				// Role info.
				role := ""
				for _, ni := range nodeItems {
					if ni.Name == n.name {
						for _, kv := range ni.Columns {
							if kv.Key == "Role" {
								role = kv.Value
								break
							}
						}
						break
					}
				}
				roleStr := ""
				if role != "" {
					roleStr = " " + ui.DimStyle.Render("["+role+"]")
				}

				cpuBar := renderBar(n.cpuUsed, n.cpuAlloc, 15)
				memBar := renderBar(n.memUsed, n.memAlloc, 15)
				// Node name on first line, bars on second line to avoid wrapping.
				lines = append(lines, fmt.Sprintf("  %s %s%s",
					statusDot, name, roleStr))
				lines = append(lines, fmt.Sprintf("      %s %s   %s %s",
					ui.HelpKeyStyle.Render("CPU"), cpuBar,
					ui.HelpKeyStyle.Render("MEM"), memBar))
			}
			lines = append(lines, "")
		}

		// Warnings.
		lines = append(lines, ui.DimStyle.Bold(true).Render("  WARNINGS"))
		lines = append(lines, "")
		hasWarnings := false
		if failedPods > 0 {
			lines = append(lines, ui.StatusFailed.Render(fmt.Sprintf("  ! %d pod(s) in failed state", failedPods)))
			hasWarnings = true
		}
		notReadyWorkerNodes := 0
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
					notReadyWorkerNodes++
				}
			}
		}
		if notReadyWorkerNodes > 0 {
			lines = append(lines, ui.StatusFailed.Render(fmt.Sprintf("  ! %d worker node(s) not ready", notReadyWorkerNodes)))
			hasWarnings = true
		}
		if crashLoopPods > 0 {
			lines = append(lines, ui.StatusFailed.Render(fmt.Sprintf("  ! %d pod(s) in CrashLoopBackOff", crashLoopPods)))
			hasWarnings = true
		}
		// PDB violation warnings.
		if len(pdbWarnings) > 0 {
			lines = append(lines, "")
			lines = append(lines, ui.DimStyle.Bold(true).Render("  PDB WARNINGS"))
			lines = append(lines, "")
			for _, pw := range pdbWarnings {
				lines = append(lines, fmt.Sprintf("  %s %s/%s",
					ui.StatusPending.Render("\u2298"),
					ui.DimStyle.Render(pw.namespace),
					ui.StatusPending.Render(pw.name)))
				detail := fmt.Sprintf("       MinAvail=%s  Healthy=%s  DisruptionsAllowed=%s",
					pw.minAvailable, pw.currentHealthy, pw.disruptionsAllowed)
				lines = append(lines, ui.DimStyle.Render(detail))
			}
			hasWarnings = true
		}
		// Recent warning events.
		if len(warningEvents) > 0 {
			lines = append(lines, "")
			lines = append(lines, ui.DimStyle.Bold(true).Render("  RECENT WARNING EVENTS"))
			lines = append(lines, "")
			for _, ev := range warningEvents {
				reason := ""
				object := ""
				message := ""
				count := ""
				for _, kv := range ev.Columns {
					switch kv.Key {
					case "Reason":
						reason = kv.Value
					case "Object":
						object = kv.Value
					case "Message":
						msg := kv.Value
						if len(msg) > 60 {
							msg = msg[:57] + "..."
						}
						message = msg
					case "Count":
						count = kv.Value
					}
				}
				// Format: warning icon [Age] (xN) Reason: Object - Message
				countLabel := ""
				if count != "" && count != "1" {
					countLabel = ui.DimStyle.Render(fmt.Sprintf("(x%s) ", count))
				}
				line := fmt.Sprintf("  %s %s %s%s %s",
					ui.StatusPending.Render("\u26a0"),
					ui.DimStyle.Render(fmt.Sprintf("%-4s", ev.Age)),
					countLabel,
					ui.StatusFailed.Render(reason+":"),
					ui.NormalStyle.Render(object))
				lines = append(lines, line)
				if message != "" {
					lines = append(lines, fmt.Sprintf("       %s", ui.DimStyle.Render(message)))
				}
			}
			hasWarnings = true
		}
		if !hasWarnings {
			lines = append(lines, ui.StatusRunning.Render("  No warnings"))
		}

		lines = append(lines, "")

		return dashboardLoadedMsg{content: strings.Join(lines, "\n"), context: kctx}
	}
}

// loadMonitoringDashboard fetches active Prometheus alerts and renders a monitoring dashboard.
func (m Model) loadMonitoringDashboard() tea.Cmd {
	kctx := m.nav.Context
	client := m.client
	ns := m.resolveNamespace()
	return func() tea.Msg {
		var lines []string
		lines = append(lines, "")
		lines = append(lines, ui.DimStyle.Bold(true).Render("  MONITORING OVERVIEW"))
		lines = append(lines, "")

		// Fetch all active alerts with a timeout.
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

		// Summary counts.
		firing := 0
		pending := 0
		critical := 0
		warning := 0
		info := 0
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

		// Alert summary header.
		lines = append(lines, fmt.Sprintf("  %s %s",
			ui.HelpKeyStyle.Render("Alerts:"),
			ui.NormalStyle.Render(fmt.Sprintf("%d total", totalAlerts))))

		if totalAlerts == 0 {
			lines = append(lines, ui.StatusRunning.Render("  \u2713 No active alerts"))
		} else {
			// State breakdown.
			stateStr := ""
			if firing > 0 {
				stateStr += ui.StatusFailed.Render(fmt.Sprintf("%d firing", firing))
			}
			if pending > 0 {
				if stateStr != "" {
					stateStr += "  "
				}
				stateStr += ui.StatusPending.Render(fmt.Sprintf("%d pending", pending))
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
				sevStr += ui.StatusPending.Render(fmt.Sprintf("%d warning", warning))
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
		}

		lines = append(lines, "")

		// Sort alerts: critical firing first, then warning firing, then pending, then info.
		sort.SliceStable(alerts, func(i, j int) bool {
			severityOrder := map[string]int{"critical": 0, "warning": 1, "info": 2}
			stateOrder := map[string]int{"firing": 0, "pending": 1}
			si := stateOrder[alerts[i].State]*10 + severityOrder[alerts[i].Severity]
			sj := stateOrder[alerts[j].State]*10 + severityOrder[alerts[j].Severity]
			return si < sj
		})

		// Critical alerts section.
		if critical > 0 {
			lines = append(lines, ui.StatusFailed.Bold(true).Render("  CRITICAL ALERTS"))
			lines = append(lines, "")
			for _, a := range alerts {
				if a.Severity != "critical" {
					continue
				}
				stateIcon := "\u25cf"
				stateStyle := ui.StatusFailed
				if a.State == "pending" {
					stateStyle = ui.StatusPending
				}

				header := fmt.Sprintf("  %s %s",
					stateStyle.Bold(true).Render(stateIcon),
					ui.StatusFailed.Bold(true).Render(a.Name))

				if a.State == "pending" {
					header += " " + ui.StatusPending.Render("[pending]")
				}

				lines = append(lines, header)

				if a.Summary != "" {
					summary := a.Summary
					if len(summary) > 80 {
						summary = summary[:77] + "..."
					}
					lines = append(lines, "    "+ui.DimStyle.Render(summary))
				} else if a.Description != "" {
					desc := a.Description
					if len(desc) > 80 {
						desc = desc[:77] + "..."
					}
					lines = append(lines, "    "+ui.DimStyle.Render(desc))
				}

				if !a.Since.IsZero() {
					lines = append(lines, "    "+ui.DimStyle.Render("since "+formatTimeAgo(a.Since)))
				}

				// Show relevant labels (namespace, pod, deployment, etc.)
				if len(a.Labels) > 0 {
					labelParts := monitoringAlertLabels(a)
					if len(labelParts) > 0 {
						lines = append(lines, "    "+strings.Join(labelParts, " "))
					}
				}

				if a.GrafanaURL != "" {
					lines = append(lines, "    "+ui.HelpKeyStyle.Render("dashboard: "+a.GrafanaURL))
				}

				lines = append(lines, "")
			}
		}

		// Warning alerts section.
		if warning > 0 {
			lines = append(lines, ui.StatusPending.Bold(true).Render("  WARNING ALERTS"))
			lines = append(lines, "")
			for _, a := range alerts {
				if a.Severity != "warning" {
					continue
				}
				stateIcon := "\u25cf"
				stateStyle := ui.StatusPending
				if a.State == "pending" {
					stateStyle = ui.StatusPending
				}

				lines = append(lines, fmt.Sprintf("  %s %s",
					stateStyle.Render(stateIcon),
					ui.StatusPending.Render(a.Name)))

				if a.Summary != "" {
					summary := a.Summary
					if len(summary) > 80 {
						summary = summary[:77] + "..."
					}
					lines = append(lines, "    "+ui.DimStyle.Render(summary))
				} else if a.Description != "" {
					desc := a.Description
					if len(desc) > 80 {
						desc = desc[:77] + "..."
					}
					lines = append(lines, "    "+ui.DimStyle.Render(desc))
				}

				if !a.Since.IsZero() {
					lines = append(lines, "    "+ui.DimStyle.Render("since "+formatTimeAgo(a.Since)))
				}

				if len(a.Labels) > 0 {
					labelParts := monitoringAlertLabels(a)
					if len(labelParts) > 0 {
						lines = append(lines, "    "+strings.Join(labelParts, " "))
					}
				}

				lines = append(lines, "")
			}
		}

		// Info alerts section.
		if info > 0 {
			lines = append(lines, ui.DimStyle.Bold(true).Render("  INFO ALERTS"))
			lines = append(lines, "")
			for _, a := range alerts {
				if a.Severity == "critical" || a.Severity == "warning" {
					continue
				}
				lines = append(lines, fmt.Sprintf("  %s %s",
					ui.DimStyle.Render("\u25cf"),
					ui.NormalStyle.Render(a.Name)))

				if a.Summary != "" {
					summary := a.Summary
					if len(summary) > 80 {
						summary = summary[:77] + "..."
					}
					lines = append(lines, "    "+ui.DimStyle.Render(summary))
				}

				lines = append(lines, "")
			}
		}

		lines = append(lines, "")
		return monitoringDashboardMsg{content: strings.Join(lines, "\n"), context: kctx}
	}
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

// monitoringAlertLabels extracts relevant labels from an alert for display.
func monitoringAlertLabels(a k8s.AlertInfo) []string {
	var parts []string
	for _, key := range []string{"namespace", "pod", "deployment", "statefulset", "daemonset", "node", "service", "job", "container"} {
		if v, ok := a.Labels[key]; ok {
			parts = append(parts, ui.DimStyle.Render(key+"=")+ui.NormalStyle.Render(v))
		}
	}
	return parts
}
