package app

import (
	"fmt"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) updateContainerPortsLoaded(msg containerPortsLoadedMsg) Model {
	m.loading = false
	if msg.err != nil {
		// No ports discovered, still open the overlay for manual entry.
		m.addLogEntry("WRN", fmt.Sprintf("Could not discover ports: %v", msg.err))
		m.pfAvailablePorts = nil
		m.pfPortCursor = -1
	} else {
		m.pfAvailablePorts = nil
		for _, p := range msg.ports {
			m.pfAvailablePorts = append(m.pfAvailablePorts, ui.PortInfo{
				Port:     strconv.Itoa(int(p.ContainerPort)),
				Name:     p.Name,
				Protocol: p.Protocol,
			})
		}
		if len(m.pfAvailablePorts) > 0 {
			m.pfPortCursor = 0
		} else {
			m.pfPortCursor = -1
		}
	}
	m.portForwardInput.Clear()
	m.overlay = overlayPortForward
	return m
}

func (m Model) updatePortForwardStarted(msg portForwardStartedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Port forward failed: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.pfLastCreatedID = msg.id
	if msg.localPort == "0" {
		m.setStatusMessage(fmt.Sprintf("Port forward started (resolving local port...) -> %s", msg.remotePort), false)
		m.addLogEntry("INF", fmt.Sprintf("Port forward %d started: localhost:? -> %s (random port)", msg.id, msg.remotePort))
	} else {
		m.setStatusMessage(fmt.Sprintf("Port forward started on localhost:%s -> %s", msg.localPort, msg.remotePort), false)
		m.addLogEntry("INF", fmt.Sprintf("Port forward %d started: localhost:%s -> %s", msg.id, msg.localPort, msg.remotePort))
	}
	// Navigate to the Port Forwards list.
	m.navigateToPortForwards()
	m.saveCurrentPortForwards()
	cmds := []tea.Cmd{m.waitForPortForwardUpdate()}
	return m, tea.Batch(cmds...)
}

func (m Model) updatePortForwardStopped(msg portForwardStoppedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Error stopping port forward: ", msg.err)
	} else {
		m.setStatusMessage("Port forward stopped", false)
	}
	m.saveCurrentPortForwards()
	// Refresh the port forwards list if viewing it.
	cmds := []tea.Cmd{scheduleStatusClear()}
	if m.nav.Level == model.LevelResources && m.nav.ResourceType.Kind == "__port_forwards__" {
		cmds = append(cmds, m.refreshCurrentLevel())
	}
	return m, tea.Batch(cmds...)
}

func (m Model) updatePortForwardUpdate(msg portForwardUpdateMsg) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{m.waitForPortForwardUpdate()}
	if msg.err != nil {
		// Mirror to slog so the failure survives an lfk close/restart, not
		// just the in-memory errorLog overlay.
		logger.Error("Port-forward update error", "error", msg.err)
		m.addLogEntry("ERR", msg.err.Error())
	}
	// Log newly failed port forwards.
	for _, e := range m.portForwardMgr.Entries() {
		if e.Status == k8s.PortForwardFailed && e.Error != "" {
			if _, seen := m.pfLoggedErrors[e.ID]; !seen {
				logger.Error("Port-forward failed",
					"id", e.ID,
					"resource", fmt.Sprintf("%s/%s", e.ResourceKind, e.ResourceName),
					"namespace", e.Namespace,
					"context", e.Context,
					"error", e.Error)
				m.addLogEntry("ERR", fmt.Sprintf("Port forward %s/%s failed: %s", e.ResourceKind, e.ResourceName, e.Error))
				if m.pfLoggedErrors == nil {
					m.pfLoggedErrors = make(map[int]struct{})
				}
				m.pfLoggedErrors[e.ID] = struct{}{}
			}
		}
	}
	// Show resolved port notification for recently created port forward.
	if m.pfLastCreatedID > 0 {
		for _, e := range m.portForwardMgr.Entries() {
			if e.ID == m.pfLastCreatedID && e.LocalPort != "0" && e.LocalPort != "" {
				m.setStatusMessage(fmt.Sprintf("Port forward active on localhost:%s -> %s", e.LocalPort, e.RemotePort), false)
				m.addLogEntry("INF", fmt.Sprintf("Port forward %d resolved: localhost:%s -> %s", e.ID, e.LocalPort, e.RemotePort))
				m.pfLastCreatedID = 0
				m.saveCurrentPortForwards()
				cmds = append(cmds, scheduleStatusClear())
				break
			}
		}
	}
	if m.nav.Level == model.LevelResources && m.nav.ResourceType.Kind == "__port_forwards__" {
		m.setMiddleItems(m.portForwardItems())
		m.clampCursor()
	}
	return m, tea.Batch(cmds...)
}
