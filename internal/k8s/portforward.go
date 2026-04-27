// Package k8s provides Kubernetes API access for the TUI application.
package k8s

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/janosmiko/lfk/internal/logger"
)

// PortForwardStatus represents the state of a port forward.
type PortForwardStatus string

const (
	PortForwardStarting PortForwardStatus = "Starting"
	PortForwardRunning  PortForwardStatus = "Running"
	PortForwardStopped  PortForwardStatus = "Stopped"
	PortForwardFailed   PortForwardStatus = "Failed"
)

// PortForwardEntry represents a single active port forward.
type PortForwardEntry struct {
	ID           int
	ResourceKind string // "pod" or "svc"
	ResourceName string
	Namespace    string
	Context      string
	LocalPort    string
	RemotePort   string
	Status       PortForwardStatus
	Error        string
	StartedAt    time.Time
	cmd          *exec.Cmd
	cancel       context.CancelFunc
}

// PortForwardManager manages active port forwards.
type PortForwardManager struct {
	mu       sync.Mutex
	entries  []*PortForwardEntry
	nextID   int
	onUpdate func() // callback when entries change
}

// NewPortForwardManager creates a new port forward manager.
func NewPortForwardManager() *PortForwardManager {
	return &PortForwardManager{
		nextID: 1,
	}
}

// SetUpdateCallback sets a callback that is invoked when entries change.
func (m *PortForwardManager) SetUpdateCallback(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onUpdate = fn
}

// Entries returns a copy of all port forward entries.
func (m *PortForwardManager) Entries() []PortForwardEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]PortForwardEntry, len(m.entries))
	for i, e := range m.entries {
		result[i] = *e
	}
	return result
}

// ActiveCount returns the number of active (running) port forwards.
func (m *PortForwardManager) ActiveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, e := range m.entries {
		if e.Status == PortForwardRunning || e.Status == PortForwardStarting {
			count++
		}
	}
	return count
}

// Start starts a new port forward using kubectl port-forward as a background process.
// It returns the entry ID for tracking. The port forward starts in PortForwardStarting
// status and transitions to PortForwardRunning only after kubectl confirms readiness
// by writing "Forwarding from" to stdout.
//
// displayContext is the lfk-side name (potentially disambiguated when several
// kubeconfigs share a context name) used in the entry/UI; kubectlContext is
// the literal name from the source kubeconfig and is what we hand to
// `kubectl --context`. When the two are equal, callers can pass the same
// value twice. See issue #23.
func (m *PortForwardManager) Start(kubectlPath, kubeconfigPaths, resourceKind, resourceName, namespace, displayContext, kubectlContext, localPort, remotePort string) (int, error) {
	m.mu.Lock()
	id := m.nextID
	m.nextID++
	m.mu.Unlock()

	target := resourceKind + "/" + resourceName
	portMapping := localPort + ":" + remotePort
	args := []string{"port-forward", target, portMapping, "-n", namespace, "--context", kubectlContext}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, kubectlPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return 0, fmt.Errorf("creating stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return 0, fmt.Errorf("creating stderr pipe: %w", err)
	}

	entry := &PortForwardEntry{
		ID:           id,
		ResourceKind: resourceKind,
		ResourceName: resourceName,
		Namespace:    namespace,
		Context:      displayContext,
		LocalPort:    localPort,
		RemotePort:   remotePort,
		Status:       PortForwardStarting,
		StartedAt:    time.Now(),
		cmd:          cmd,
		cancel:       cancel,
	}

	logger.Info("Running kubectl command", "cmd", cmd.String())
	if err := cmd.Start(); err != nil {
		cancel()
		logger.Error("kubectl port-forward failed to start", "cmd", cmd.String(), "error", err)
		return 0, fmt.Errorf("starting port-forward: %w", err)
	}

	m.mu.Lock()
	m.entries = append(m.entries, entry)
	onUpdate := m.onUpdate
	m.mu.Unlock()

	if onUpdate != nil {
		onUpdate()
	}

	// Capture stderr in background.
	var stderrBuf bytes.Buffer
	go func() {
		_, _ = io.Copy(&stderrBuf, stderrPipe)
	}()

	// Monitor stdout for readiness confirmation and process lifecycle.
	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		ready := false
		for scanner.Scan() {
			line := scanner.Text()
			logger.Info("port-forward stdout", "id", id, "line", line)
			if !ready && strings.Contains(line, "Forwarding from") {
				ready = true
				// Parse the actual local port from the output.
				// Format: "Forwarding from 127.0.0.1:<port> -> <remotePort>"
				// or: "Forwarding from [::1]:<port> -> <remotePort>"
				var resolvedPort string
				if arrowIdx := strings.Index(line, " -> "); arrowIdx > 0 {
					addrPart := line[:arrowIdx]
					if lastColon := strings.LastIndex(addrPart, ":"); lastColon >= 0 {
						resolvedPort = addrPart[lastColon+1:]
					}
				}
				m.mu.Lock()
				if resolvedPort != "" {
					logger.Info("port-forward resolved local port", "id", id, "resolvedPort", resolvedPort)
					entry.LocalPort = resolvedPort
				}
				if entry.Status == PortForwardStarting {
					entry.Status = PortForwardRunning
				}
				onUpdate := m.onUpdate
				m.mu.Unlock()
				if onUpdate != nil {
					onUpdate()
				}
			}
		}

		// Wait for process to finish.
		err := cmd.Wait()
		m.mu.Lock()
		if entry.Status == PortForwardRunning || entry.Status == PortForwardStarting {
			if err != nil {
				entry.Status = PortForwardFailed
				errMsg := strings.TrimSpace(stderrBuf.String())
				if errMsg != "" {
					entry.Error = errMsg
				} else {
					entry.Error = err.Error()
				}
				logger.Error("port-forward failed", "id", id, "error", entry.Error)
			} else {
				entry.Status = PortForwardStopped
			}
		}
		onUpdate := m.onUpdate
		m.mu.Unlock()
		if onUpdate != nil {
			onUpdate()
		}
	}()

	return id, nil
}

// Stop stops a port forward by its ID.
func (m *PortForwardManager) Stop(id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, e := range m.entries {
		if e.ID == id {
			if e.Status != PortForwardRunning && e.Status != PortForwardStarting {
				return fmt.Errorf("port forward %d is not running", id)
			}
			e.cancel()
			e.Status = PortForwardStopped
			logger.Info("Port-forward stopped",
				"id", id,
				"resource", fmt.Sprintf("%s/%s", e.ResourceKind, e.ResourceName),
				"namespace", e.Namespace,
				"context", e.Context,
				"localPort", e.LocalPort,
				"remotePort", e.RemotePort)
			if m.onUpdate != nil {
				m.onUpdate()
			}
			return nil
		}
	}
	return fmt.Errorf("port forward %d not found", id)
}

// Remove removes a stopped/failed port forward entry from the list.
func (m *PortForwardManager) Remove(id int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, e := range m.entries {
		if e.ID == id {
			// Stop it first if still running or starting.
			wasRunning := e.Status == PortForwardRunning || e.Status == PortForwardStarting
			if wasRunning {
				e.cancel()
			}
			m.entries = append(m.entries[:i], m.entries[i+1:]...)
			logger.Info("Port-forward removed",
				"id", id,
				"resource", fmt.Sprintf("%s/%s", e.ResourceKind, e.ResourceName),
				"namespace", e.Namespace,
				"context", e.Context,
				"wasRunning", wasRunning)
			if m.onUpdate != nil {
				m.onUpdate()
			}
			return
		}
	}
}

// GetEntry returns a copy of the port forward entry with the given ID, or nil if not found.
func (m *PortForwardManager) GetEntry(id int) *PortForwardEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.entries {
		if e.ID == id {
			copy := *e
			return &copy
		}
	}
	return nil
}

// StopAll stops all active port forwards. Called during application shutdown.
func (m *PortForwardManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, e := range m.entries {
		if e.Status == PortForwardRunning || e.Status == PortForwardStarting {
			e.cancel()
			e.Status = PortForwardStopped
		}
	}
}

// ContainerPort represents a port exposed by a container.
type ContainerPort struct {
	Name          string
	ContainerPort int32
	Protocol      string
}
