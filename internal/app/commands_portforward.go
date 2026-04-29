package app

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/logger"
)

// --- Port forwarding commands ---

// execKubectlPortForward starts a port forward as a background process via the manager.
func (m Model) execKubectlPortForward(portMapping string) tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return portForwardStartedMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	resourceKind := "pod"
	if m.actionCtx.kind == "Service" {
		resourceKind = "svc"
	}

	// Parse the port mapping.
	parts := strings.SplitN(portMapping, ":", 2)
	localPort := parts[0]
	remotePort := localPort
	if len(parts) == 2 {
		remotePort = parts[1]
	}

	mgr := m.portForwardMgr
	name := m.actionCtx.name
	kctx := m.actionCtx.context
	kubeconfigPaths := m.client.KubeconfigPathForContext(kctx)
	kubectlKctx := m.kubectlContext(kctx)

	logger.Info("Running kubectl port-forward", "resource", resourceKind+"/"+name, "localPort", localPort, "remotePort", remotePort, "namespace", ns, "context", kctx)
	return func() tea.Msg {
		id, err := mgr.Start(kubectlPath, kubeconfigPaths, resourceKind, name, ns, kctx, kubectlKctx, localPort, remotePort)
		if err != nil {
			logger.Error("kubectl port-forward failed", "resource", resourceKind+"/"+name, "error", err)
		}
		return portForwardStartedMsg{id: id, localPort: localPort, remotePort: remotePort, err: err}
	}
}

// stopPortForward stops a port forward by ID.
func (m Model) stopPortForward(id int) tea.Cmd {
	mgr := m.portForwardMgr
	return func() tea.Msg {
		err := mgr.Stop(id)
		return portForwardStoppedMsg{id: id, err: err}
	}
}

// restartPortForward restarts a stopped/failed port forward by ID.
// It removes the old entry and starts a new one with the same parameters.
func (m Model) restartPortForward(id int) tea.Cmd {
	mgr := m.portForwardMgr
	entry := mgr.GetEntry(id)
	if entry == nil {
		return func() tea.Msg {
			return portForwardStartedMsg{err: fmt.Errorf("port forward %d not found", id)}
		}
	}
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return portForwardStartedMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}
	resourceKind := entry.ResourceKind
	name := entry.ResourceName
	ns := entry.Namespace
	kctx := entry.Context
	kubeconfigPaths := m.client.KubeconfigPathForContext(kctx)
	kubectlKctx := m.kubectlContext(kctx)
	remotePort := entry.RemotePort
	// Use "0" for local port to get a fresh random port assignment.
	localPort := "0"

	mgr.Remove(id)

	return func() tea.Msg {
		newID, err := mgr.Start(kubectlPath, kubeconfigPaths, resourceKind, name, ns, kctx, kubectlKctx, localPort, remotePort)
		if err != nil {
			logger.Error("kubectl port-forward restart failed", "resource", resourceKind+"/"+name, "error", err)
		}
		return portForwardStartedMsg{id: newID, localPort: localPort, remotePort: remotePort, err: err}
	}
}

// waitForPortForwardUpdate returns a command that waits for a port forward state change.
func (m Model) waitForPortForwardUpdate() tea.Cmd {
	ch := make(chan struct{}, 1)
	m.portForwardMgr.SetUpdateCallback(func() {
		select {
		case ch <- struct{}{}:
		default:
		}
	})
	return func() tea.Msg {
		<-ch
		return portForwardUpdateMsg{}
	}
}
