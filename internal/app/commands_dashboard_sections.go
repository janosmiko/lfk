package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// renderBar renders a horizontal bar graph like [████████░░░░░░░░] 52%.
// The filled portion is colored based on usage percentage: green (<75%), orange (75-90%), red (>90%).
func renderBar(used, total int64, width int) string {
	if total <= 0 {
		return "[" + strings.Repeat("░", width) + "] N/A"
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

	return "[" + style.Render(filledStr) + emptyStr + "] " + fmt.Sprintf("%.0f%%", pct)
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
	var barBuilder strings.Builder
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
		barBuilder.WriteString(seg.style.Render(strings.Repeat("█", chars)))
		used += chars
	}
	if used < width {
		barBuilder.WriteString(strings.Repeat("░", width-used))
	}
	return "[" + barBuilder.String() + "]"
}

// dashboardHeaderSection renders the cluster header, node, namespace, and pod sections.
func dashboardHeaderSection(lines []string, data dashboardData) []string {
	lines = append(lines, ui.DimStyle.Bold(true).Render("  CLUSTER DASHBOARD"))
	lines = append(lines, "")

	// Nodes section.
	nodeStatus := ui.StatusRunning.Render(fmt.Sprintf("%d Ready", data.readyNodes))
	if data.readyNodes < data.nodeCount {
		notReady := data.nodeCount - data.readyNodes
		nodeStatus += " " + ui.StatusFailed.Render(fmt.Sprintf("%d NotReady", notReady))
	}
	lines = append(lines, fmt.Sprintf("  %s %s  %s",
		ui.HelpKeyStyle.Render("Nodes:"),
		ui.NormalStyle.Render(fmt.Sprintf("%d", data.nodeCount)),
		nodeStatus))
	if data.nodeCount > 0 {
		nodeBar := renderBar(int64(data.readyNodes), int64(data.nodeCount), 30)
		lines = append(lines, fmt.Sprintf("  %s %s",
			ui.HelpKeyStyle.Render("           "),
			nodeBar))
	}
	lines = append(lines, "")

	// Namespaces.
	lines = append(lines, fmt.Sprintf("  %s %s",
		ui.HelpKeyStyle.Render("Namespaces:"),
		ui.NormalStyle.Render(fmt.Sprintf("%d", data.nsCount))))
	lines = append(lines, "")
	lines = append(lines, ui.DimStyle.Render("  "+strings.Repeat("─", 50)))

	// Pods section.
	podStatus := ui.StatusRunning.Render(fmt.Sprintf("%d Running", data.pods.running))
	if data.pods.failed > 0 {
		podStatus += " " + ui.StatusFailed.Render(fmt.Sprintf("%d Failed", data.pods.failed))
	}
	if data.pods.pending > 0 {
		podStatus += " " + ui.StatusProgressing.Render(fmt.Sprintf("%d Pending", data.pods.pending))
	}
	lines = append(lines, fmt.Sprintf("  %s %s  %s",
		ui.HelpKeyStyle.Render("Pods:"),
		ui.NormalStyle.Render(fmt.Sprintf("%d", data.pods.total)),
		podStatus))
	if data.pods.total > 0 {
		segments := []struct {
			count int
			style lipgloss.Style
		}{
			{data.pods.running, ui.StatusRunning},
			{data.pods.pending, ui.StatusProgressing},
			{data.pods.failed, ui.StatusFailed},
		}
		podBar := renderStackedBar(segments, data.pods.total, 30)
		lines = append(lines, fmt.Sprintf("  %s %s",
			ui.HelpKeyStyle.Render("           "),
			podBar))
	}
	lines = append(lines, "")
	lines = append(lines, ui.DimStyle.Render("  "+strings.Repeat("─", 50)))

	return lines
}

// dashboardResourcesSection renders the cluster resources (CPU/Mem) section.
func dashboardResourcesSection(lines []string, data dashboardData) []string {
	if data.totalCPUAlloc <= 0 && data.totalMemAlloc <= 0 {
		return lines
	}
	lines = append(lines, ui.DimStyle.Bold(true).Render("  CLUSTER RESOURCES"))
	if data.nodeMetricsErr != nil {
		lines = append(lines, ui.StatusProgressing.Render("  (metrics-server unavailable)"))
	}
	lines = append(lines, "")
	if data.totalCPUAlloc > 0 {
		cpuBar := renderBar(data.totalCPUUsed, data.totalCPUAlloc, 30)
		lines = append(lines, fmt.Sprintf("  %s %s  %s / %s",
			ui.HelpKeyStyle.Render("CPU:"),
			cpuBar,
			ui.FormatCPU(data.totalCPUUsed),
			ui.FormatCPU(data.totalCPUAlloc)))
	}
	if data.totalMemAlloc > 0 {
		memBar := renderBar(data.totalMemUsed, data.totalMemAlloc, 30)
		lines = append(lines, fmt.Sprintf("  %s %s  %s / %s",
			ui.HelpKeyStyle.Render("Mem:"),
			memBar,
			ui.FormatMemory(data.totalMemUsed),
			ui.FormatMemory(data.totalMemAlloc)))
	}
	lines = append(lines, "")
	lines = append(lines, ui.DimStyle.Render("  "+strings.Repeat("─", 50)))
	return lines
}

// dashboardNodesSection renders the per-node breakdown.
func dashboardNodesSection(lines []string, data dashboardData) []string {
	if len(data.nodes) == 0 || (data.totalCPUAlloc <= 0 && data.totalMemAlloc <= 0) {
		return lines
	}
	lines = append(lines, ui.DimStyle.Bold(true).Render("  NODES"))
	lines = append(lines, "")

	maxNameLen := 0
	for _, n := range data.nodes {
		if len(n.name) > maxNameLen {
			maxNameLen = len(n.name)
		}
	}
	if maxNameLen > 48 {
		maxNameLen = 48
	}

	for _, n := range data.nodes {
		name := n.name
		if len(name) > maxNameLen {
			name = name[:maxNameLen]
		}
		statusDot := nodeStatusDot(data.nodeItems, n.name)
		roleStr := nodeRoleStr(data.nodeItems, n.name)

		cpuBar := renderBar(n.cpuUsed, n.cpuAlloc, 15)
		memBar := renderBar(n.memUsed, n.memAlloc, 15)
		lines = append(lines, fmt.Sprintf("  %s %s%s", statusDot, name, roleStr))
		lines = append(lines, fmt.Sprintf("      %s %s   %s %s",
			ui.HelpKeyStyle.Render("CPU"), cpuBar,
			ui.HelpKeyStyle.Render("MEM"), memBar))
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
					return " " + ui.DimStyle.Render("["+kv.Value+"]")
				}
			}
			return ""
		}
	}
	return ""
}

// dashboardWarningsSection renders the warnings (pod/node health issues + PDB).
func dashboardWarningsSection(lines []string, data dashboardData) []string {
	hasHealthWarnings := data.pods.failed > 0 || data.pods.crashLoop > 0
	notReadyWorkers := countNotReadyWorkerNodes(data.nodeItems)
	if notReadyWorkers > 0 {
		hasHealthWarnings = true
	}
	if !hasHealthWarnings && len(data.pdbWarnings) == 0 {
		return lines
	}

	lines = append(lines, ui.DimStyle.Bold(true).Render("  WARNINGS"))
	lines = append(lines, "")
	if data.pods.failed > 0 {
		lines = append(lines, ui.StatusFailed.Render(fmt.Sprintf("  ! %d pod(s) in failed state", data.pods.failed)))
	}
	if notReadyWorkers > 0 {
		lines = append(lines, ui.StatusFailed.Render(fmt.Sprintf("  ! %d worker node(s) not ready", notReadyWorkers)))
	}
	if data.pods.crashLoop > 0 {
		lines = append(lines, ui.StatusFailed.Render(fmt.Sprintf("  ! %d pod(s) in CrashLoopBackOff", data.pods.crashLoop)))
	}
	if len(data.pdbWarnings) > 0 {
		lines = append(lines, "")
		lines = append(lines, ui.DimStyle.Bold(true).Render("  PDB WARNINGS"))
		lines = append(lines, "")
		for _, pw := range data.pdbWarnings {
			lines = append(lines, fmt.Sprintf("  %s %s/%s",
				ui.StatusProgressing.Render("⊘"),
				ui.DimStyle.Render(pw.namespace),
				ui.StatusProgressing.Render(pw.name)))
			detail := fmt.Sprintf("       MinAvail=%s  Healthy=%s  DisruptionsAllowed=%s",
				pw.minAvailable, pw.currentHealthy, pw.disruptionsAllowed)
			lines = append(lines, ui.DimStyle.Render(detail))
		}
	}
	lines = append(lines, "")
	lines = append(lines, ui.DimStyle.Render("  "+strings.Repeat("─", 50)))
	return lines
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
func extractEventFields(ev model.Item) eventColumnFields {
	var f eventColumnFields
	for _, kv := range ev.Columns {
		switch kv.Key {
		case "Reason":
			f.reason = kv.Value
		case "Object":
			f.object = kv.Value
		case "Message":
			f.message = kv.Value
		case "Count":
			f.count = kv.Value
		}
	}
	return f
}

// dashboardInlineEventsSection renders the inline warning events section.
func dashboardInlineEventsSection(lines []string, warningEvents []model.Item) []string {
	if len(warningEvents) == 0 {
		return lines
	}
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
			nsLabel = ui.DimStyle.Render("[" + ev.Namespace + "] ")
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
