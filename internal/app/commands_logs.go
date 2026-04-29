package app

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"slices"
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

// Auto-reconnect constants for multi-container Pod log streams.
//
// When following all containers of a Pod, kubectl exits as soon as all
// currently-running containers have finished. For a pod that is still in the
// init phase, that happens every time an init container completes. We
// restart the stream after a short delay so the next container is picked up
// automatically.
//
// The delay is deliberately small: the init-container → next-container
// transition typically takes well under a second, and every extra millisecond
// between streams is log content we'll miss (`--tail=0` on reconnect means
// any lines produced during the gap are gone). We cap retries so a fully
// terminated pod doesn't spin forever.
const (
	logAutoReconnectMaxAttempts = 20
	logAutoReconnectDelay       = 250 * time.Millisecond
)

// isKubectlTransientError recognizes noise that kubectl writes to stderr
// (which startLogStream merges into stdout) while --ignore-errors keeps the
// stream alive. These lines describe pending initialization, not real
// application log output — surfacing them just clutters the viewer.
func isKubectlTransientError(line string) bool {
	// "error: container \"X\" in pod \"Y\" is waiting to start: PodInitializing"
	// "error: container \"X\" in pod \"Y\" is waiting to start: ContainerCreating"
	// etc.
	if strings.HasPrefix(line, "error: container ") && strings.Contains(line, " is waiting to start: ") {
		return true
	}
	// "Error from server (BadRequest): container X is not valid for pod Y"
	// (kubectl emits this before an init container has been created)
	if strings.HasPrefix(line, "Error from server (BadRequest):") && strings.Contains(line, " is not valid for pod ") {
		return true
	}
	return false
}

// scheduleLogStreamRestart returns a tea.Cmd that waits for
// logAutoReconnectDelay and then emits a logStreamRestartMsg carrying the
// channel of the stream that just ended. The Update handler correlates on
// that channel to ignore stale restarts after the user has switched pods or
// exited logs mode.
//
// The wait is registered as a background task so the title-bar task
// indicator shows it — that's the app-wide place for "something is
// progressing" feedback, instead of injecting status lines into the log
// viewer itself.
func (m Model) scheduleLogStreamRestart(ch chan string) tea.Cmd {
	reg := m.bgtasks
	id := reg.Start(bgtasks.KindContainers, "Waiting for next container", "")
	return tea.Tick(logAutoReconnectDelay, func(_ time.Time) tea.Msg {
		reg.Finish(id)
		return logStreamRestartMsg{ch: ch}
	})
}

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
	kubeconfigPaths := m.client.KubeconfigPathForContext(kctx)
	logPrevious := m.logPrevious
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

	// On auto-reconnect, force --tail=0 so we only pick up new lines from
	// the next container rather than re-pulling history we already have.
	reconnecting := m.logReconnecting

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
			selector := kubectlGetPodSelector(kubectlPath, kubeconfigPaths, ns, kind, name, m.kubectlContext(kctx))
			if selector != "" {
				args = []string{
					"logs", "-l", selector, "--all-containers=true", "--prefix", followFlag,
					"--max-log-requests=20", "--ignore-errors", "-n", ns, "--context", m.kubectlContext(kctx),
				}
			} else {
				// Fallback: use resource reference (follows only one pod).
				resourceRef := strings.ToLower(kind) + "/" + name
				args = []string{
					"logs", resourceRef, "--all-containers=true", "--prefix", followFlag,
					"--max-log-requests=20", "--ignore-errors", "-n", ns, "--context", m.kubectlContext(kctx),
				}
			}
		default:
			args = []string{"logs", followFlag, name, "-n", ns, "--context", m.kubectlContext(kctx)}
			if containerName != "" {
				args = append(args, "-c", containerName)
			} else if kind == "Pod" {
				// --ignore-errors prevents the stream from dying when init
				// containers haven't started yet (all-containers includes them).
				args = append(args, "--all-containers=true", "--prefix", "--max-log-requests=20", "--ignore-errors")
			}
		}

		// --tail handling:
		//   * reconnect: always pass --tail=0 so we don't re-pull history
		//     we already rendered for the previous container.
		//   * --previous: already a finite backlog — no --tail.
		//   * initial load: honor the configured tail size if positive.
		switch {
		case reconnecting:
			args = append(args, "--tail=0")
		case logPrevious:
			// leave tail off
		case tailLines > 0:
			args = append(args, fmt.Sprintf("--tail=%d", tailLines))
		}

		// Always include --timestamps so toggling visibility doesn't need a restart.
		args = append(args, "--timestamps")

		logger.Info("Starting kubectl logs", "args", strings.Join(args, " "), "kubeconfig", kubeconfigPaths)

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
		// kubectl (with --ignore-errors) keeps re-emitting the same
		// 'container X is waiting to start' line every retry. We want the
		// user to see the message once so they know what's happening, but
		// not a flood. Dedup within this kubectl run; reconnect resets.
		seenTransient := map[string]bool{}
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if isKubectlTransientError(line) {
				if seenTransient[line] {
					continue
				}
				seenTransient[line] = true
			}
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
//
// kctx must be the kubeconfig's *original* context name (the one kubectl will
// recognise), not lfk's potentially disambiguated display name. Callers in
// the app package should use Model.kubectlContext to translate before
// invoking this helper.
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
	logExecCmd("Running kubectl command", cmd)
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
	return slices.Contains(selectedContainers, containerName)
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
				"--max-log-requests=20", "-n", itemNs, "--context", m.kubectlContext(kctx),
			}
		default:
			resourceRef := strings.ToLower(kind) + "/" + item.Name
			args = []string{
				"logs", resourceRef, "--all-containers=true", "--prefix", followFlag,
				"--max-log-requests=20", "-n", itemNs, "--context", m.kubectlContext(kctx),
			}
		}

		// Add --tail for initial loading.
		if m.logTailLines > 0 {
			args = append(args, fmt.Sprintf("--tail=%d", m.logTailLines))
		}

		args = append(args, "--timestamps")

		m.addLogEntry("DBG", "kubectl "+strings.Join(args, " "))

		cmd := exec.CommandContext(ctx, kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPathForContext(kctx))
		logger.Info("Starting multi-log kubectl",
			"item", item.Name,
			"cmd", cmd.String(),
			"kubeconfig", m.client.KubeconfigPathForContext(kctx))
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
				"--max-log-requests=20", "-n", itemNs, "--context", m.kubectlContext(kctx),
			}
		default:
			resourceRef := strings.ToLower(kind) + "/" + item.Name
			args = []string{
				"logs", resourceRef, "--all-containers=true", "--prefix", followFlag,
				"--max-log-requests=20", "-n", itemNs, "--context", m.kubectlContext(kctx),
			}
		}

		// Add --tail for initial loading.
		if m.logTailLines > 0 {
			args = append(args, fmt.Sprintf("--tail=%d", m.logTailLines))
		}

		args = append(args, "--timestamps")

		cmd := exec.CommandContext(ctx, kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPathForContext(kctx))
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			logger.Error("Failed to open kubectl logs stdout pipe (multi-pod)", "error", err, "pod", item.Name, "namespace", itemNs)
			continue
		}
		cmd.Stderr = cmd.Stdout

		if err := cmd.Start(); err != nil {
			logger.Error("Failed to start kubectl logs (multi-pod)", "error", err, "pod", item.Name, "namespace", itemNs, "cmd", cmd.String())
			continue
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
	kubeconfigPaths := m.client.KubeconfigPathForContext(kctx)
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
			selector := kubectlGetPodSelector(kubectlPath, kubeconfigPaths, ns, kind, name, m.kubectlContext(kctx))
			if selector != "" {
				args = []string{
					"logs", "-l", selector, "--all-containers=true", "--prefix",
					"--max-log-requests=20", "-n", ns, "--context", m.kubectlContext(kctx),
				}
			} else {
				resourceRef := strings.ToLower(kind) + "/" + name
				args = []string{
					"logs", resourceRef, "--all-containers=true", "--prefix",
					"--max-log-requests=20", "-n", ns, "--context", m.kubectlContext(kctx),
				}
			}
		case "Pod":
			// When multi-container filtering is active, always use --all-containers --prefix
			// so that matchesContainerFilter can parse the prefix and filter correctly.
			if len(selectedContainers) > 0 || containerName == "" {
				args = []string{
					"logs", name, "--all-containers=true", "--prefix",
					"--max-log-requests=20", "-n", ns, "--context", m.kubectlContext(kctx),
				}
			} else {
				args = []string{
					"logs", name, "-c", containerName, "-n", ns, "--context", m.kubectlContext(kctx),
				}
			}
		default:
			return logHistoryMsg{err: fmt.Errorf("unsupported kind for log history: %s", kind)}
		}

		args = append(args, fmt.Sprintf("--tail=%d", newTail))
		args = append(args, "--timestamps")

		kubeconfigEnv := "KUBECONFIG=" + kubeconfigPaths
		cmd := exec.CommandContext(ctx, kubectlPath, args...)
		cmd.Env = append(os.Environ(), kubeconfigEnv)

		output, err := cmd.Output()
		if err != nil {
			return logHistoryMsg{err: err, prevTotal: prevTotal}
		}

		var lines []string
		for line := range strings.SplitSeq(string(output), "\n") {
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
//
// The trigger requires the cursor to be at line 0, not just logScroll==0.
// In Tail Logs mode (10 lines into a ~30-line viewport) logScroll is pinned
// at 0 from startup, so without this guard every up-navigation — k, ctrl+u,
// ctrl+b, gg, mouse wheel up — would immediately fetch older history even
// when the user has only moved a single line up from the bottom.
func (m *Model) maybeLoadMoreHistory() tea.Cmd {
	if m.logScroll == 0 && m.logCursor <= 0 && m.logHasMoreHistory && !m.logLoadingHistory && !m.logPrevious {
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
	kubeconfigPaths := m.client.KubeconfigPathForContext(kctx)
	logPrevious := m.logPrevious
	sanitized := sanitizeFilename(name)

	return m.trackBgTask(bgtasks.KindSubprocess, "Save all logs: "+kind+"/"+name, bgtaskTarget(kctx, ns), func() tea.Msg {
		var args []string
		switch kind {
		case "Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob", "Service":
			selector := kubectlGetPodSelector(kubectlPath, kubeconfigPaths, ns, kind, name, m.kubectlContext(kctx))
			if selector != "" {
				args = []string{
					"logs", "-l", selector, "--all-containers=true", "--prefix",
					"--max-log-requests=20", "--timestamps", "-n", ns, "--context", m.kubectlContext(kctx),
				}
			} else {
				resourceRef := strings.ToLower(kind) + "/" + name
				args = []string{
					"logs", resourceRef, "--all-containers=true", "--prefix",
					"--timestamps", "--max-log-requests=20", "-n", ns, "--context", m.kubectlContext(kctx),
				}
			}
		case "Pod":
			if containerName != "" {
				args = []string{
					"logs", name, "-c", containerName, "--prefix", "--timestamps",
					"-n", ns, "--context", m.kubectlContext(kctx),
				}
			} else {
				args = []string{
					"logs", name, "--all-containers=true", "--prefix", "--timestamps",
					"--max-log-requests=20", "-n", ns, "--context", m.kubectlContext(kctx),
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
		// Match commands_exec.go convention: log every kubectl invocation
		// before running so the slog file records what we actually ran.
		logExecCmd("Running kubectl command", cmd)
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
