package app

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/app/bgtasks"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// --- Log streaming commands ---

// startLogStream launches kubectl logs as a subprocess, creates a channel, and
// starts a goroutine that reads stdout line by line. Returns a tea.Cmd that
// reads the first line from the channel.
func (m *Model) startLogStream() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	kind := m.actionCtx.kind
	name := m.actionCtx.name
	kctx := m.actionCtx.context
	containerName := m.actionCtx.containerName
	kubeconfigPaths := m.client.KubeconfigPaths()
	logPrevious := m.logPrevious
	sinceDuration := m.logSinceDuration
	tailLines := m.logTailLines
	if tailLines == 0 {
		tailLines = ui.ConfigLogTailLines
	}

	// Capture selected containers for filtering (empty = show all).
	// Only filter client-side when in --all-containers mode (no -c flag).
	// When containerName is set, kubectl already filters server-side via -c,
	// and log lines lack the [pod/name/container] prefix that matchesContainerFilter needs.
	var selectedContainers []string
	if containerName == "" {
		selectedContainers = append([]string(nil), m.logSelectedContainers...)
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.logCancel = cancel

	ch := make(chan string, 256)
	m.logCh = ch

	// Run selector discovery and kubectl logs entirely in a background
	// goroutine so that OIDC browser auth doesn't freeze the TUI.
	go func() {
		defer close(ch)

		var args []string
		followFlag := "-f"
		if logPrevious {
			followFlag = "--previous"
		}
		switch kind {
		case "Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob", "Service":
			// Try to get the pod selector via kubectl so we can follow ALL pods.
			// kubectl logs deployment/<name> only follows a single pod.
			selector := kubectlGetPodSelector(kubectlPath, kubeconfigPaths, ns, kind, name, kctx)
			if selector != "" {
				args = []string{
					"logs", "-l", selector, "--all-containers=true", "--prefix", followFlag,
					"--max-log-requests=20", "--ignore-errors", "-n", ns, "--context", kctx,
				}
			} else {
				// Fallback: use resource reference (follows only one pod).
				resourceRef := strings.ToLower(kind) + "/" + name
				args = []string{
					"logs", resourceRef, "--all-containers=true", "--prefix", followFlag,
					"--max-log-requests=20", "--ignore-errors", "-n", ns, "--context", kctx,
				}
			}
		default:
			args = []string{"logs", followFlag, name, "-n", ns, "--context", kctx}
			if containerName != "" {
				args = append(args, "-c", containerName)
			} else if kind == "Pod" {
				// --ignore-errors prevents the stream from dying when init
				// containers haven't started yet (all-containers includes them).
				args = append(args, "--all-containers=true", "--prefix", "--max-log-requests=20", "--ignore-errors")
			}
		}

		// Add --tail for initial loading (not for --previous mode since it's already finite).
		if tailLines > 0 && !logPrevious {
			args = append(args, fmt.Sprintf("--tail=%d", tailLines))
		}

		// Constrain the stream to a recent time window when the user
		// set one via `T`.  --since is incompatible with --previous
		// (kubectl errors), so skip it in previous-container mode.
		// Convert any `d` suffix to hours — kubectl doesn't understand
		// "30d" directly but accepts "720h".
		if sinceDuration != "" && !logPrevious {
			args = append(args, "--since="+kubectlSinceArg(sinceDuration))
		}

		// Always include --timestamps so toggling visibility doesn't need a restart.
		args = append(args, "--timestamps")

		logger.Info("Starting kubectl logs", "args", strings.Join(args, " "))

		cmd := exec.CommandContext(ctx, kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			logger.Error("Failed to create stdout pipe", "error", err)
			return
		}
		cmd.Stderr = cmd.Stdout

		if err := cmd.Start(); err != nil {
			logger.Error("Failed to start kubectl logs", "error", err)
			select {
			case ch <- fmt.Sprintf("[error] Failed to start kubectl logs: %v", err):
			case <-ctx.Done():
			}
			return
		}

		defer cmd.Wait() //nolint:errcheck
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			// Filter by selected containers when using --prefix mode.
			if len(selectedContainers) > 0 && !matchesContainerFilter(line, selectedContainers) {
				continue
			}
			select {
			case ch <- line:
			case <-ctx.Done():
				return
			}
		}
	}()

	return m.waitForLogLine()
}

// kubectlGetPodSelector runs kubectl to extract the pod selector labels for a
// parent resource (Deployment, StatefulSet, etc.). It uses kubectl rather than
// the Go client so that OIDC tokens are discovered/cached by the same credential
// helper that kubectl uses, avoiding a separate browser auth flow.
func kubectlGetPodSelector(kubectlPath, kubeconfigPaths, ns, kind, name, kctx string) string {
	// For CronJobs there's no direct pod selector.
	if kind == "CronJob" {
		return ""
	}

	resourceRef := strings.ToLower(kind) + "/" + name

	getArgs := []string{
		"get", resourceRef,
		"-n", ns, "--context", kctx,
		"-o", "json",
	}

	cmd := exec.Command(kubectlPath, getArgs...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
	logger.Info("Running kubectl command", "cmd", cmd.String())
	out, err := cmd.Output()
	if err != nil {
		logger.Error("Failed to get pod selector via kubectl", "cmd", cmd.String(), "resource", resourceRef, "error", err)
		return ""
	}

	// Parse the JSON to extract the selector.
	var obj struct {
		Spec struct {
			Selector json.RawMessage `json:"selector"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(out, &obj); err != nil {
		logger.Error("Failed to parse kubectl output", "error", err)
		return ""
	}

	var labels map[string]string

	if kind == "Service" {
		// Service selector is a plain map.
		if err := json.Unmarshal(obj.Spec.Selector, &labels); err != nil {
			logger.Error("Failed to parse service selector", "error", err)
			return ""
		}
	} else {
		// Deployment/StatefulSet/DaemonSet/Job selector has matchLabels.
		var sel struct {
			MatchLabels map[string]string `json:"matchLabels"`
		}
		if err := json.Unmarshal(obj.Spec.Selector, &sel); err != nil {
			logger.Error("Failed to parse selector", "error", err)
			return ""
		}
		labels = sel.MatchLabels
	}

	if len(labels) == 0 {
		return ""
	}

	parts := make([]string, 0, len(labels))
	for k, v := range labels {
		parts = append(parts, k+"="+v)
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

// matchesContainerFilter checks whether a prefixed log line belongs to one of
// the selected containers. The prefix format is "[pod/<podname>/<containername>] ...".
// If the line doesn't have a prefix (no bracket), it is passed through.
func matchesContainerFilter(line string, selectedContainers []string) bool {
	if len(line) == 0 || line[0] != '[' {
		// No prefix, pass through (non-prefixed lines like error messages).
		return true
	}
	closeBracket := strings.Index(line, "] ")
	if closeBracket < 0 {
		return true // not a standard prefix line
	}
	prefix := line[1:closeBracket] // "pod/<podname>/<containername>"
	// kubectl --prefix format uses slashes: pod/<podname>/<containername>
	lastSlash := strings.LastIndex(prefix, "/")
	if lastSlash < 0 {
		return true // unexpected format
	}
	containerName := prefix[lastSlash+1:]
	for _, sc := range selectedContainers {
		if sc == containerName {
			return true
		}
	}
	return false
}

// waitForLogLine returns a tea.Cmd that reads the next line from the log channel.
func (m Model) waitForLogLine() tea.Cmd {
	ch := m.logCh
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return logLineMsg{done: true, ch: ch}
		}
		return logLineMsg{line: line, ch: ch}
	}
}

// startMultiLogStream spawns one kubectl logs process per selected item and
// merges their output into a single log channel. This supports streaming logs
// from multiple pods or parent resources simultaneously.
func (m *Model) startMultiLogStream(items []model.Item) (tea.Model, tea.Cmd) {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return m, func() tea.Msg { return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)} }
	}

	// Initialize log viewer state.
	m.mode = modeLogs
	m.logLines = nil
	m.logVisibleIndices = nil
	m.logRules = nil
	m.logIncludeMode = IncludeAny
	m.logFilterChain = NewFilterChain(nil, IncludeAny, m.logSeverityDetector)
	m.logScroll = 0
	m.logFollow = true
	m.logWrap = false
	m.logLineNumbers = true
	m.logTimestamps = false
	m.logPrevious = false
	m.logIsMulti = true
	m.logMultiItems = items
	m.logTitle = fmt.Sprintf("Logs: %d resources", len(items))
	m.logTailLines = ui.ConfigLogTailLines
	m.logHasMoreHistory = false // too complex to deduplicate across multiple streams
	m.logLoadingHistory = false
	m.logCursor = 0 // will track end as lines stream in with follow mode
	m.logVisualMode = false
	m.logVisualStart = 0

	ctx, cancel := context.WithCancel(context.Background())
	m.logCancel = cancel
	ch := make(chan string, 256)
	m.logCh = ch

	kctx := m.nav.Context
	ns := m.resolveNamespace()

	var wg sync.WaitGroup
	for _, item := range items {
		itemNs := ns
		if item.Namespace != "" {
			itemNs = item.Namespace
		}

		kind := item.Kind
		if kind == "" {
			kind = m.nav.ResourceType.Kind
		}

		followFlag := "-f"
		if m.logPrevious {
			followFlag = "--previous"
		}
		var args []string
		switch kind {
		case "Pod":
			args = []string{
				"logs", item.Name, "--all-containers=true", "--prefix", followFlag,
				"--max-log-requests=20", "-n", itemNs, "--context", kctx,
			}
		default:
			resourceRef := strings.ToLower(kind) + "/" + item.Name
			args = []string{
				"logs", resourceRef, "--all-containers=true", "--prefix", followFlag,
				"--max-log-requests=20", "-n", itemNs, "--context", kctx,
			}
		}

		// Add --tail for initial loading.
		if m.logTailLines > 0 {
			args = append(args, fmt.Sprintf("--tail=%d", m.logTailLines))
		}

		// --since keeps the multi-stream bounded to the user-requested
		// window, matching the single-stream behaviour.  Previous-mode
		// is never set here (startMultiLogStream resets it), so no
		// compatibility guard is needed.
		if m.logSinceDuration != "" {
			args = append(args, "--since="+kubectlSinceArg(m.logSinceDuration))
		}

		args = append(args, "--timestamps")

		logger.Info("Starting multi-log kubectl", "item", item.Name, "args", strings.Join(args, " "))
		m.addLogEntry("DBG", "kubectl "+strings.Join(args, " "))

		cmd := exec.CommandContext(ctx, kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			logger.Error("Failed to create stdout pipe for multi-log", "item", item.Name, "error", err)
			continue
		}
		cmd.Stderr = cmd.Stdout

		if err := cmd.Start(); err != nil {
			logger.Error("Failed to start kubectl logs for multi-log", "item", item.Name, "error", err)
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
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
		}()
	}

	// Close the channel once all goroutines finish.
	go func() {
		wg.Wait()
		close(ch)
	}()

	return m, m.waitForLogLine()
}

// restartMultiLogStream restarts a multi-log stream using stored items,
// preserving current viewer settings (used when toggling timestamps).
func (m Model) restartMultiLogStream() (Model, tea.Cmd) {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return m, func() tea.Msg { return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)} }
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.logCancel = cancel
	ch := make(chan string, 256)
	m.logCh = ch

	kctx := m.nav.Context
	ns := m.resolveNamespace()
	items := m.logMultiItems

	var wg sync.WaitGroup
	for _, item := range items {
		itemNs := ns
		if item.Namespace != "" {
			itemNs = item.Namespace
		}

		kind := item.Kind
		if kind == "" {
			kind = m.nav.ResourceType.Kind
		}

		followFlag := "-f"
		if m.logPrevious {
			followFlag = "--previous"
		}
		var args []string
		switch kind {
		case "Pod":
			args = []string{
				"logs", item.Name, "--all-containers=true", "--prefix", followFlag,
				"--max-log-requests=20", "-n", itemNs, "--context", kctx,
			}
		default:
			resourceRef := strings.ToLower(kind) + "/" + item.Name
			args = []string{
				"logs", resourceRef, "--all-containers=true", "--prefix", followFlag,
				"--max-log-requests=20", "-n", itemNs, "--context", kctx,
			}
		}

		// Add --tail for initial loading.
		if m.logTailLines > 0 {
			args = append(args, fmt.Sprintf("--tail=%d", m.logTailLines))
		}

		if m.logSinceDuration != "" {
			args = append(args, "--since="+kubectlSinceArg(m.logSinceDuration))
		}

		args = append(args, "--timestamps")

		cmd := exec.CommandContext(ctx, kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			continue
		}
		cmd.Stderr = cmd.Stdout

		if err := cmd.Start(); err != nil {
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
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
		}()
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	return m, m.waitForLogLine()
}

// --- Log history and persistence ---

// fetchOlderLogs fetches an additional batch of older log lines using a
// one-shot kubectl logs call (no -f). The result is returned as a logHistoryMsg.
func (m *Model) fetchOlderLogs() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg { return logHistoryMsg{err: err} }
	}

	ns := m.actionNamespace()
	kind := m.actionCtx.kind
	name := m.actionCtx.name
	kctx := m.actionCtx.context
	containerName := m.actionCtx.containerName
	kubeconfigPaths := m.client.KubeconfigPaths()
	sinceDuration := m.logSinceDuration
	newTail := m.logTailLines + ui.ConfigLogTailLines
	prevTotal := len(m.logLines)
	// Only filter client-side when in --all-containers mode (no -c flag).
	var selectedContainers []string
	if containerName == "" {
		selectedContainers = append([]string(nil), m.logSelectedContainers...)
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.logHistoryCancel = cancel

	return m.trackBgTask(bgtasks.KindSubprocess, "Log history: "+kind+"/"+name, bgtaskTarget(kctx, ns), func() tea.Msg {
		defer cancel()

		var args []string //nolint:prealloc
		switch kind {
		case "Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob", "Service":
			selector := kubectlGetPodSelector(kubectlPath, kubeconfigPaths, ns, kind, name, kctx)
			if selector != "" {
				args = []string{
					"logs", "-l", selector, "--all-containers=true", "--prefix",
					"--max-log-requests=20", "-n", ns, "--context", kctx,
				}
			} else {
				resourceRef := strings.ToLower(kind) + "/" + name
				args = []string{
					"logs", resourceRef, "--all-containers=true", "--prefix",
					"--max-log-requests=20", "-n", ns, "--context", kctx,
				}
			}
		case "Pod":
			// When multi-container filtering is active, always use --all-containers --prefix
			// so that matchesContainerFilter can parse the prefix and filter correctly.
			if len(selectedContainers) > 0 || containerName == "" {
				args = []string{
					"logs", name, "--all-containers=true", "--prefix",
					"--max-log-requests=20", "-n", ns, "--context", kctx,
				}
			} else {
				args = []string{
					"logs", name, "-c", containerName, "-n", ns, "--context", kctx,
				}
			}
		default:
			return logHistoryMsg{err: fmt.Errorf("unsupported kind for log history: %s", kind)}
		}

		args = append(args, fmt.Sprintf("--tail=%d", newTail))
		// Honour the active --since window on back-scroll too; going
		// beyond the window is pointless since kubectl won't return
		// anything older than the cutoff anyway.
		if sinceDuration != "" {
			args = append(args, "--since="+kubectlSinceArg(sinceDuration))
		}
		args = append(args, "--timestamps")

		kubeconfigEnv := "KUBECONFIG=" + kubeconfigPaths
		cmd := exec.CommandContext(ctx, kubectlPath, args...)
		cmd.Env = append(os.Environ(), kubeconfigEnv)

		output, err := cmd.Output()
		if err != nil {
			return logHistoryMsg{err: err, prevTotal: prevTotal}
		}

		var lines []string
		for _, line := range strings.Split(string(output), "\n") {
			if line == "" {
				continue
			}
			// Filter by selected containers (same as live stream filtering).
			if len(selectedContainers) > 0 && !matchesContainerFilter(line, selectedContainers) {
				continue
			}
			lines = append(lines, line)
		}

		return logHistoryMsg{lines: lines, prevTotal: prevTotal}
	})
}

// maybeLoadMoreHistory triggers a background fetch of older log lines
// when the user has scrolled to the top and more history may be available.
func (m *Model) maybeLoadMoreHistory() tea.Cmd {
	if m.logScroll == 0 && m.logHasMoreHistory && !m.logLoadingHistory && !m.logPrevious {
		m.logLoadingHistory = true
		return m.fetchOlderLogs()
	}
	return nil
}

// saveLoadedLogs writes the currently buffered log lines to a file under /tmp.
func (m *Model) saveLoadedLogs() (string, error) {
	name := sanitizeFilename(m.actionCtx.name)
	path := fmt.Sprintf("%s/lfk-logs-%s-%d.log", os.TempDir(), name, time.Now().Unix())
	content := strings.Join(m.logLines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// saveAllLogs runs a one-shot kubectl logs (without --tail) and writes everything to a file.
func (m *Model) saveAllLogs() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg { return logSaveAllMsg{err: err} }
	}

	ns := m.actionNamespace()
	kind := m.actionCtx.kind
	name := m.actionCtx.name
	kctx := m.actionCtx.context
	containerName := m.actionCtx.containerName
	kubeconfigPaths := m.client.KubeconfigPaths()
	logPrevious := m.logPrevious
	sanitized := sanitizeFilename(name)

	return m.trackBgTask(bgtasks.KindSubprocess, "Save all logs: "+kind+"/"+name, bgtaskTarget(kctx, ns), func() tea.Msg {
		var args []string
		switch kind {
		case "Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob", "Service":
			selector := kubectlGetPodSelector(kubectlPath, kubeconfigPaths, ns, kind, name, kctx)
			if selector != "" {
				args = []string{
					"logs", "-l", selector, "--all-containers=true", "--prefix",
					"--max-log-requests=20", "--timestamps", "-n", ns, "--context", kctx,
				}
			} else {
				resourceRef := strings.ToLower(kind) + "/" + name
				args = []string{
					"logs", resourceRef, "--all-containers=true", "--prefix",
					"--timestamps", "--max-log-requests=20", "-n", ns, "--context", kctx,
				}
			}
		case "Pod":
			if containerName != "" {
				args = []string{
					"logs", name, "-c", containerName, "--prefix", "--timestamps",
					"-n", ns, "--context", kctx,
				}
			} else {
				args = []string{
					"logs", name, "--all-containers=true", "--prefix", "--timestamps",
					"--max-log-requests=20", "-n", ns, "--context", kctx,
				}
			}
		default:
			return logSaveAllMsg{err: fmt.Errorf("unsupported kind: %s", kind)}
		}

		if logPrevious {
			args = append(args, "--previous")
		}

		cmd := exec.CommandContext(context.Background(), kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
		output, err := cmd.Output()
		if err != nil {
			return logSaveAllMsg{err: err}
		}

		path := fmt.Sprintf("/tmp/lfk-logs-%s-%d-all.log", sanitized, time.Now().Unix())
		if err := os.WriteFile(path, output, 0o644); err != nil {
			return logSaveAllMsg{err: err}
		}
		return logSaveAllMsg{path: path}
	})
}

// sanitizeFilename replaces characters not suitable for filenames.
func sanitizeFilename(s string) string {
	return strings.NewReplacer("/", "_", "\\", "_", ":", "_", " ", "_").Replace(s)
}
