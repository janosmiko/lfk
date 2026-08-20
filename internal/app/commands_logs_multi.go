package app

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// startMultiLogStream spawns one kubectl logs process per selected item and
// merges their output into a single log channel. This supports streaming logs
// from multiple pods or parent resources simultaneously.
func (m *Model) startMultiLogStream(items []model.Item) (tea.Model, tea.Cmd) {
	kubectlPath, err := k8s.KubectlPath()
	if err != nil {
		return m, func() tea.Msg { return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)} }
	}

	previousMode := m.mode

	// Initialize log viewer state.
	m.mode = modeLogs
	m.resetLogBuffer()
	m.logView.scroll = 0
	m.logView.follow = true
	m.logView.wrap = ui.ConfigLogWrap
	m.logView.lineNumbers = true
	m.logView.timestamps = ui.ConfigLogShowTimestamps
	m.logView.hidePrefixes = !ui.ConfigLogShowPrefixes
	m.logView.previewVisible = ui.ConfigLogShowPreview
	m.logView.previous = false
	m.logView.isMulti = true
	m.logView.multiItems = items
	m.logView.title = fmt.Sprintf("Logs: %d resources", len(items))
	m.logView.tailLines = ui.ConfigLogTailLines
	m.logView.hasMoreHistory = false // too complex to deduplicate across multiple streams
	m.logView.loadingHistory = false
	m.logView.cursor = 0 // will track end as lines stream in with follow mode
	m.logView.visualMode = false
	m.logView.visualStart = 0

	ctx, cancel := context.WithCancel(context.Background())
	m.logView.cancel = cancel
	ch := make(chan string, 256)
	m.logView.ch = ch

	kctx := m.nav.Context
	ns := m.resolveNamespace()

	var wg sync.WaitGroup
	started := 0
	var lastErr error
	for _, item := range items {
		if err := m.startMultiLogItem(ctx, &wg, ch, kubectlPath, kctx, ns, item, true); err != nil {
			lastErr = err
			continue
		}
		started++
	}

	// Close the channel once all goroutines finish.
	go func() {
		wg.Wait()
		close(ch)
	}()

	if started == 0 {
		cancel()
		m.mode = previousMode
		msg := fmt.Sprintf("Failed to start logs for %d resources", len(items))
		if lastErr != nil {
			msg = fmt.Sprintf("%s: %v", msg, lastErr)
		}
		m.setStatusMessage(msg, true)
		return m, scheduleStatusClear()
	}

	return m, m.waitForLogLine()
}

// verbose is true for the initial start (DBG entry, info-level command
// trace) and false for a restart, which logs its own quieter messages.
func (m *Model) startMultiLogItem(
	ctx context.Context, wg *sync.WaitGroup, ch chan<- string,
	kubectlPath, kctx, ns string, item model.Item, verbose bool,
) error {
	itemCtx := kctx
	if item.ClusterName != "" {
		itemCtx = item.ClusterName
	}
	itemNs := ns
	if item.Namespace != "" {
		itemNs = item.Namespace
	}

	kind := item.Kind
	if kind == "" {
		kind = m.nav.ResourceType.Kind
	}

	followFlag := "-f"
	if m.logView.previous {
		followFlag = "--previous"
	}
	var args []string
	switch kind {
	case "Pod":
		args = []string{
			"logs", item.Name, "--all-containers=true", "--prefix", followFlag,
			"--max-log-requests=20", "-n", itemNs, "--context", m.kubectlContext(itemCtx),
		}
	default:
		resourceRef := strings.ToLower(kind) + "/" + item.Name
		args = []string{
			"logs", resourceRef, "--all-containers=true", "--prefix", followFlag,
			"--max-log-requests=20", "-n", itemNs, "--context", m.kubectlContext(itemCtx),
		}
	}

	// Add --tail for initial loading.
	if m.logView.tailLines > 0 {
		args = append(args, fmt.Sprintf("--tail=%d", m.logView.tailLines))
	}

	args = append(args, "--timestamps")

	if verbose {
		m.addLogEntry("DBG", "kubectl "+logger.Redact(strings.Join(args, " ")))
	}

	cmd := exec.CommandContext(ctx, kubectlPath, k8s.DemoKubectlArgs(args)...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPathForContext(itemCtx))
	if verbose {
		logger.Info("Starting multi-log kubectl",
			"item", item.Name,
			"context", logger.Redact(itemCtx),
			"cmd", logger.Redact(cmd.String()),
			"kubeconfig", logger.Redact(m.client.KubeconfigPathForContext(itemCtx)))
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		if verbose {
			logger.Error("Failed to create stdout pipe for multi-log", "item", item.Name, "error", err)
		} else {
			logger.Error("Failed to open kubectl logs stdout pipe (multi-pod)", "error", err, "pod", item.Name, "namespace", logger.Redact(itemNs), "context", logger.Redact(itemCtx))
		}
		return err
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		if verbose {
			logger.Error("Failed to start kubectl logs for multi-log", "item", item.Name, "error", err)
		} else {
			logger.Error("Failed to start kubectl logs (multi-pod)", "error", err, "pod", item.Name, "namespace", logger.Redact(itemNs), "context", logger.Redact(itemCtx), "cmd", logger.Redact(cmd.String()))
		}
		return err
	}

	wg.Go(func() {
		defer cmd.Wait() //nolint:errcheck
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
		for scanner.Scan() {
			select {
			case ch <- scanner.Text():
			case <-ctx.Done():
				return
			}
		}
	})

	return nil
}

// restartMultiLogStream restarts a multi-log stream using stored items,
// preserving current viewer settings (used when toggling timestamps).
func (m Model) restartMultiLogStream() (Model, tea.Cmd) {
	kubectlPath, err := k8s.KubectlPath()
	if err != nil {
		return m, func() tea.Msg { return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)} }
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.logView.cancel = cancel
	ch := make(chan string, 256)
	m.logView.ch = ch

	kctx := m.nav.Context
	ns := m.resolveNamespace()
	items := m.logView.multiItems

	var wg sync.WaitGroup
	for _, item := range items {
		_ = m.startMultiLogItem(ctx, &wg, ch, kubectlPath, kctx, ns, item, false)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	return m, m.waitForLogLine()
}
