package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/logger"
)

// PortForwardState represents a single persisted port forward.
type PortForwardState struct {
	ResourceKind string `json:"resource_kind" yaml:"resource_kind"`
	ResourceName string `json:"resource_name" yaml:"resource_name"`
	Namespace    string `json:"namespace" yaml:"namespace"`
	Context      string `json:"context" yaml:"context"`
	LocalPort    string `json:"local_port" yaml:"local_port"`
	RemotePort   string `json:"remote_port" yaml:"remote_port"`
}

// PortForwardStates is the top-level struct for persisting port forwards.
type PortForwardStates struct {
	PortForwards []PortForwardState `json:"port_forwards" yaml:"port_forwards"`
}

// portForwardStatePath returns the path to the port forward state file.
func portForwardStatePath() string {
	stateDir := os.Getenv("XDG_STATE_HOME")
	if stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		stateDir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateDir, "lfk", "portforwards.yaml")
}

// loadPortForwardState reads saved port forwards from disk.
func loadPortForwardState() *PortForwardStates {
	path := portForwardStatePath()
	if path == "" {
		return &PortForwardStates{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("Failed to read port-forward state", "error", err, "path", path)
		}
		return &PortForwardStates{}
	}
	var s PortForwardStates
	if err := yaml.Unmarshal(data, &s); err != nil {
		logger.Warn("Port-forward state file is corrupt; ignoring", "error", err, "path", path)
		return &PortForwardStates{}
	}
	return &s
}

// savePortForwardState writes port forward state to disk.
func savePortForwardState(s *PortForwardStates) error {
	path := portForwardStatePath()
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// saveCurrentPortForwards persists all running port forwards to disk.
func (m *Model) saveCurrentPortForwards() {
	entries := m.portForwardMgr.Entries()
	var states []PortForwardState
	for _, e := range entries {
		if e.Status == k8s.PortForwardRunning || e.Status == k8s.PortForwardStarting {
			states = append(states, PortForwardState{
				ResourceKind: e.ResourceKind,
				ResourceName: e.ResourceName,
				Namespace:    e.Namespace,
				Context:      e.Context,
				LocalPort:    e.LocalPort,
				RemotePort:   e.RemotePort,
			})
		}
	}
	if err := savePortForwardState(&PortForwardStates{PortForwards: states}); err != nil {
		logger.Error("Failed to persist port-forward state", "error", err, "count", len(states))
	}
}

// restorePortForwards re-establishes saved port forwards from the previous session.
// Returns tea.Cmds that will start each port forward asynchronously.
func (m *Model) restorePortForwards() []tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		logger.Error("Cannot restore port forwards: kubectl not found", "error", err)
		m.addLogEntry("ERR", fmt.Sprintf("Cannot restore port forwards: kubectl not found: %v", err))
		return nil
	}
	mgr := m.portForwardMgr

	cmds := make([]tea.Cmd, 0, len(m.pendingPortForwards.PortForwards))
	for _, pf := range m.pendingPortForwards.PortForwards {
		kubeconfigPath := m.client.KubeconfigPathForContext(pf.Context)
		kubectlContext := m.client.OriginalContextName(pf.Context)
		cmds = append(cmds, func() tea.Msg {
			// Reuse the saved local port so users get the same port on restart.
			id, err := mgr.Start(kubectlPath, kubeconfigPath, pf.ResourceKind, pf.ResourceName, pf.Namespace, pf.Context, kubectlContext, pf.LocalPort, pf.RemotePort)
			if err != nil {
				logger.Error("Failed to restore port forward",
					"resource", fmt.Sprintf("%s/%s", pf.ResourceKind, pf.ResourceName),
					"error", err)
				return portForwardUpdateMsg{err: fmt.Errorf("restore port forward %s/%s: %w", pf.ResourceKind, pf.ResourceName, err)}
			}
			logger.Info("Restored port forward",
				"id", id,
				"resource", fmt.Sprintf("%s/%s", pf.ResourceKind, pf.ResourceName),
				"remotePort", pf.RemotePort)
			return portForwardUpdateMsg{}
		})
	}

	if len(cmds) > 0 {
		count := len(m.pendingPortForwards.PortForwards)
		m.addLogEntry("INF", fmt.Sprintf("Restoring %d port forward(s) from previous session", count))
		// Start listening for port forward updates.
		cmds = append(cmds, m.waitForPortForwardUpdate())
	}

	return cmds
}
