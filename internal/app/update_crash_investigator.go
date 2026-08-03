package app

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/ui"
)

// updateCrashInvestigation handles the crashInvestigationMsg dispatched
// after k8s.GetCrashInvestigation completes. It clears the loading flag,
// reports errors via the status line, preserves user-facing tab/container/
// log-mode/scroll across refreshes, and applies first-open defaults.
func (m Model) updateCrashInvestigation(msg crashInvestigationMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setStatusMessage(fmt.Sprintf("Crash investigation failed: %v", msg.err), true)
		return m, scheduleStatusClear()
	}
	if msg.info == nil {
		m.setStatusMessage("Crash investigation returned no data", true)
		return m, scheduleStatusClear()
	}

	// Preserve user's tab/container/log-mode/scroll across refreshes.
	prev := m.crashInv
	m.crashInv = crashInvState{
		data:            msg.info,
		activeContainer: prev.activeContainer,
		activeTab:       prev.activeTab,
		showPrevious:    prev.showPrevious,
		scroll:          prev.scroll,
	}
	if m.crashInv.scroll == nil {
		m.crashInv.scroll = map[crashInvScrollKey]int{}
	}

	// First-open defaults.
	if m.overlay != overlayCrashInvestigator {
		m.crashInv.activeTab = crashInvTabSummary
		m.crashInv.showPrevious = true
		m.crashInv.activeContainer = pickInitialActiveContainer(msg.info)
	} else if findContainerInfo(msg.info, m.crashInv.activeContainer) == nil {
		// Refresh removed the previously-active container.
		m.crashInv.activeContainer = pickInitialActiveContainer(msg.info)
	}

	m.overlay = overlayCrashInvestigator
	return m, nil
}

// pickInitialActiveContainer returns the first container that looks
// unhealthy (init takes precedence to surface init-CLB pods early); falls
// back to the first app container, then the first init container.
func pickInitialActiveContainer(info *k8s.CrashInvestigation) string {
	for _, c := range info.InitContainers {
		if isFailingContainer(c) {
			return c.Name
		}
	}
	for _, c := range info.AppContainers {
		if isFailingContainer(c) {
			return c.Name
		}
	}
	if len(info.AppContainers) > 0 {
		return info.AppContainers[0].Name
	}
	if len(info.InitContainers) > 0 {
		return info.InitContainers[0].Name
	}
	return ""
}

// isFailingContainer reports whether a container has hit a CLB-class
// state or has restarted at least once.
func isFailingContainer(c k8s.ContainerCrash) bool {
	if c.RestartCount > 0 {
		return true
	}
	switch c.StateReason {
	case "CrashLoopBackOff", "Error", "ImagePullBackOff", "ErrImagePull", "CreateContainerConfigError":
		return true
	}
	return false
}

// findContainerInfo locates a container by name in either the init or
// app slice; returns nil if not found.
func findContainerInfo(info *k8s.CrashInvestigation, name string) *k8s.ContainerCrash {
	for i := range info.InitContainers {
		if info.InitContainers[i].Name == name {
			return &info.InitContainers[i]
		}
	}
	for i := range info.AppContainers {
		if info.AppContainers[i].Name == name {
			return &info.AppContainers[i]
		}
	}
	return nil
}

// renderOverlayCrashInvestigator builds the presentation entry from
// k8s.CrashInvestigation and delegates to the pure UI renderer.
func (m Model) renderOverlayCrashInvestigator() (string, int, int) {
	// OverlayStyle adds 6 cols of horizontal chrome (2 border + 2*2 padding)
	// and 4 rows of vertical chrome (2 border + 2*1 padding) on top of (w, h).
	// Reserve enough terminal margin so the rendered overlay never exceeds
	// the terminal width — otherwise the terminal soft-wraps the line and the
	// next overlay row's `│` border appears mid-line on the wrap continuation.
	w, h := min(160, m.width-8), min(35, m.height-6)
	if m.crashInv.data == nil {
		return "", w, h
	}
	d := m.crashInv.data
	entry := ui.CrashInvestigatorEntry{
		PodName:         d.Pod.Name,
		Namespace:       d.Pod.Namespace,
		Phase:           d.Pod.Phase,
		PodIP:           d.Pod.PodIP,
		Node:            d.Pod.Node,
		QoSClass:        d.Pod.QoSClass,
		Age:             d.Pod.Age,
		OwnerKind:       d.Pod.OwnerKind,
		OwnerName:       d.Pod.OwnerName,
		Describe:        d.Describe,
		DescribeError:   d.DescribeError,
		ActiveContainer: m.crashInv.activeContainer,
		Tab:             ui.CrashTab(m.crashInv.activeTab),
		ShowPrevious:    m.crashInv.showPrevious,
	}
	for _, c := range d.InitContainers {
		entry.InitContainers = append(entry.InitContainers, toCrashContainerEntry(c))
	}
	for _, c := range d.AppContainers {
		entry.AppContainers = append(entry.AppContainers, toCrashContainerEntry(c))
	}
	for _, ev := range d.Events {
		entry.Events = append(entry.Events, ui.CrashEventEntry{
			Type:    ev.Type,
			Reason:  ev.Reason,
			Age:     formatCrashEventAge(ev.LastTimestamp.Time),
			Source:  ev.Source.Component,
			Message: strings.TrimSpace(ev.Message),
		})
	}
	scroll := m.crashInv.scroll[m.scrollKey()]
	return ui.RenderCrashInvestigatorOverlay(entry, scroll, w, h), w, h
}

// toCrashContainerEntry maps a k8s.ContainerCrash into the UI's
// presentation-only CrashContainerEntry, flattening the nullable
// LastTermination pointer into bool + scalar fields the renderer expects.
func toCrashContainerEntry(c k8s.ContainerCrash) ui.CrashContainerEntry {
	out := ui.CrashContainerEntry{
		Name:         c.Name,
		IsInit:       c.IsInit,
		Image:        c.Image,
		State:        c.State,
		StateReason:  c.StateReason,
		Ready:        c.Ready,
		RestartCount: c.RestartCount,
		PreviousLog:  c.PreviousLog,
		CurrentLog:   c.CurrentLog,
		LogError:     c.LogError,
	}
	if c.LastTermination != nil {
		out.HasLastTerm = true
		out.LastReason = c.LastTermination.Reason
		out.LastExitCode = c.LastTermination.ExitCode
		out.LastSignal = c.LastTermination.Signal
		out.LastFinished = c.LastTermination.FinishedAt
		out.LastMessage = c.LastTermination.Message
	}
	return out
}

// formatCrashEventAge returns a short humanized "5m" / "1h" / "2d" age
// for an event's LastTimestamp. Zero values render as an em-dash.
func formatCrashEventAge(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours())/24)
	}
}
