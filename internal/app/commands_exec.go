package app

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// --- Action commands ---

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

	ctx, cancel := context.WithCancel(context.Background())
	m.logCancel = cancel

	ch := make(chan string, 256)
	m.logCh = ch

	// Run selector discovery and kubectl logs entirely in a background
	// goroutine so that OIDC browser auth doesn't freeze the TUI.
	go func() {
		defer close(ch)

		var args []string
		switch kind {
		case "Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob", "Service":
			// Try to get the pod selector via kubectl so we can follow ALL pods.
			// kubectl logs deployment/<name> only follows a single pod.
			selector := kubectlGetPodSelector(kubectlPath, kubeconfigPaths, ns, kind, name, kctx)
			if selector != "" {
				args = []string{
					"logs", "-l", selector, "--all-containers=true", "--prefix", "-f",
					"--max-log-requests=20", "-n", ns, "--context", kctx,
				}
			} else {
				// Fallback: use resource reference (follows only one pod).
				resourceRef := strings.ToLower(kind) + "/" + name
				args = []string{"logs", resourceRef, "--all-containers=true", "--prefix", "-f", "-n", ns, "--context", kctx}
			}
		default:
			args = []string{"logs", "-f", name, "-n", ns, "--context", kctx}
			if containerName != "" {
				args = append(args, "-c", containerName)
			} else if kind == "Pod" {
				args = append(args, "--all-containers=true", "--prefix")
			}
		}

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
			return
		}

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
	m.logTitle = fmt.Sprintf("Logs: %d resources", len(items))

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

		var args []string
		switch kind {
		case "Pod":
			args = []string{"logs", item.Name, "--all-containers=true", "--prefix", "-f", "-n", itemNs, "--context", kctx}
		default:
			resourceRef := strings.ToLower(kind) + "/" + item.Name
			args = []string{"logs", resourceRef, "--all-containers=true", "--prefix", "-f", "-n", itemNs, "--context", kctx}
		}

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

func (m Model) execKubectlExec() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	args := []string{"exec", "-it", m.actionCtx.name, "-n", ns, "--context", m.actionCtx.context}
	if m.actionCtx.containerName != "" {
		args = append(args, "-c", m.actionCtx.containerName)
	}
	args = append(args, "--", "/bin/sh", "-c", "clear; (bash || ash || sh)")

	logger.Info("Starting kubectl exec", "args", strings.Join(args, " "))
	cmd := exec.Command(kubectlPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())

	if ui.ConfigTerminalMode == "pty" {
		cols := m.width - 4
		rows := m.height - 6
		if cols < 20 {
			cols = 80
		}
		if rows < 5 {
			rows = 24
		}
		title := fmt.Sprintf("Exec: %s/%s", m.actionNamespace(), m.actionCtx.name)
		return startPTYExecCmd(cmd, title, cols, rows)
	}

	return tea.ExecProcess(clearBeforeExec(cmd), func(err error) tea.Msg {
		return actionResultMsg{message: "Exec session ended", err: err}
	})
}

func (m Model) execKubectlAttach() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	args := []string{"attach", "-it", m.actionCtx.name, "-n", ns, "--context", m.actionCtx.context}
	if m.actionCtx.containerName != "" {
		args = append(args, "-c", m.actionCtx.containerName)
	}

	cmd := exec.Command(kubectlPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
	logger.Info("Running kubectl command", "cmd", cmd.String())

	if ui.ConfigTerminalMode == "pty" {
		cols := m.width - 4
		rows := m.height - 6
		if cols < 20 {
			cols = 80
		}
		if rows < 5 {
			rows = 24
		}
		title := fmt.Sprintf("Attach: %s/%s", m.actionNamespace(), m.actionCtx.name)
		return startPTYExecCmd(cmd, title, cols, rows)
	}

	return tea.ExecProcess(clearBeforeExec(cmd), func(err error) tea.Msg {
		return actionResultMsg{message: "Attach session ended", err: err}
	})
}

func (m Model) execKubectlEdit() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	rt := m.actionCtx.resourceType
	args := []string{"edit", rt.Resource, m.actionCtx.name, "--context", m.actionCtx.context}
	if rt.Namespaced {
		args = append(args, "-n", ns)
	}

	cmd := exec.Command(kubectlPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
	logger.Info("Running kubectl command", "cmd", cmd.String())
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			logger.Error("kubectl edit failed", "cmd", cmd.String(), "error", err)
		}
		return actionResultMsg{message: "Edit completed", err: err}
	})
}

func (m Model) execKubectlDescribe() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	rt := m.actionCtx.resourceType
	name := m.actionCtx.name
	args := []string{"describe", rt.Resource, name, "--context", m.actionCtx.context}
	if rt.Namespaced {
		args = append(args, "-n", ns)
	}

	title := fmt.Sprintf("Describe: %s/%s", rt.Resource, name)

	return func() tea.Msg {
		cmd := exec.Command(kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
		logger.Info("Running kubectl command", "cmd", cmd.String())
		output, err := cmd.CombinedOutput()
		if err != nil {
			logger.Error("kubectl describe failed", "cmd", cmd.String(), "error", err, "output", string(output))
			return describeLoadedMsg{
				title: title,
				err:   fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output))),
			}
		}
		return describeLoadedMsg{
			content: string(output),
			title:   title,
		}
	}
}

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
	kubeconfigPaths := m.client.KubeconfigPaths()
	name := m.actionCtx.name
	kctx := m.actionCtx.context

	logger.Info("Running kubectl port-forward", "resource", resourceKind+"/"+name, "localPort", localPort, "remotePort", remotePort, "namespace", ns, "context", kctx)
	return func() tea.Msg {
		id, err := mgr.Start(kubectlPath, kubeconfigPaths, resourceKind, name, ns, kctx, localPort, remotePort)
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
	kubeconfigPaths := m.client.KubeconfigPaths()
	resourceKind := entry.ResourceKind
	name := entry.ResourceName
	ns := entry.Namespace
	kctx := entry.Context
	remotePort := entry.RemotePort
	// Use "0" for local port to get a fresh random port assignment.
	localPort := "0"

	mgr.Remove(id)

	return func() tea.Msg {
		newID, err := mgr.Start(kubectlPath, kubeconfigPaths, resourceKind, name, ns, kctx, localPort, remotePort)
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

func (m Model) execKubectlDebug() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	args := []string{"debug", m.actionCtx.name, "-it", "--image=busybox", "--context", m.actionCtx.context, "-n", ns}

	cmd := exec.Command(kubectlPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
	logger.Info("Running kubectl command", "cmd", cmd.String())

	if ui.ConfigTerminalMode == "pty" {
		cols := m.width - 4
		rows := m.height - 6
		if cols < 20 {
			cols = 80
		}
		if rows < 5 {
			rows = 24
		}
		title := fmt.Sprintf("Debug: %s/%s", m.actionNamespace(), m.actionCtx.name)
		return startPTYExecCmd(cmd, title, cols, rows)
	}

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return actionResultMsg{message: "Debug session ended", err: err}
	})
}

// runDebugPod runs a standalone alpine debug pod in the target namespace.
// The pod is named lfk-debug-<5-random-chars> and is auto-removed on exit.
func (m Model) runDebugPod() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	ctx := m.actionCtx.context
	podName := "lfk-debug-" + randomSuffix(5)

	args := []string{
		"run", podName, "--image=alpine", "--rm", "-it",
		"--restart=Never", "-n", ns, "--context", ctx, "--", "sh",
	}

	logger.Info("Running debug pod", "pod", podName, "namespace", ns, "context", ctx)

	cmd := exec.Command(kubectlPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())

	if ui.ConfigTerminalMode == "pty" {
		cols := m.width - 4
		rows := m.height - 6
		if cols < 20 {
			cols = 80
		}
		if rows < 5 {
			rows = 24
		}
		title := fmt.Sprintf("Debug Pod: %s/%s", ns, podName)
		return startPTYExecCmd(cmd, title, cols, rows)
	}

	return tea.ExecProcess(clearBeforeExec(cmd), func(err error) tea.Msg {
		if err != nil {
			logger.Error("kubectl run debug pod failed", "cmd", cmd.String(), "error", err)
		}
		return actionResultMsg{message: "Debug pod session ended", err: err}
	})
}

// randomSuffix generates a random lowercase alphanumeric string of the given length.
func randomSuffix(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[rand.IntN(len(chars))]
	}
	return string(b)
}

// runDebugPodWithPVC runs an alpine debug pod with a PVC mounted at /mnt/data.
func (m Model) runDebugPodWithPVC() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	ctx := m.actionCtx.context
	pvcName := m.actionCtx.name
	podName := "lfk-debug-pvc-" + randomSuffix(5)

	// Create a pod manifest with the PVC mounted.
	manifest := fmt.Sprintf(`{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {"name": "%s"},
		"spec": {
			"containers": [{
				"name": "debug",
				"image": "alpine",
				"command": ["sh"],
				"stdin": true,
				"tty": true,
				"volumeMounts": [{"name": "data", "mountPath": "/mnt/data"}]
			}],
			"volumes": [{"name": "data", "persistentVolumeClaim": {"claimName": "%s"}}],
			"restartPolicy": "Never"
		}
	}`, podName, pvcName)

	// Use kubectl run with overrides to mount the PVC.
	args := []string{
		"run", podName, "--image=alpine", "-it", "--rm",
		"--restart=Never", "--context", ctx, "-n", ns,
		"--overrides", manifest, "--", "sh",
	}

	cmd := exec.Command(kubectlPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
	logger.Info("Running kubectl command", "cmd", cmd.String())
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			logger.Error("kubectl run debug pod with PVC failed", "cmd", cmd.String(), "error", err)
		}
		return actionResultMsg{message: "Debug pod session ended", err: err}
	})
}

func (m Model) deleteResource() tea.Cmd {
	// Special handling for Helm releases.
	if m.actionCtx.resourceType.APIGroup == "_helm" {
		return m.uninstallHelmRelease()
	}

	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	rt := m.actionCtx.resourceType
	name := m.actionCtx.name
	logger.Info("Deleting resource", "resource", rt.Resource, "name", name, "namespace", ns, "context", ctx)
	return func() tea.Msg {
		err := m.client.DeleteResource(ctx, ns, rt, name)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Deleted %s/%s", rt.Resource, name)}
	}
}

func (m Model) forceDeleteResource() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	rt := m.actionCtx.resourceType
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	logger.Info("Force deleting resource", "resource", rt.Resource, "name", name, "namespace", ns, "context", ctx)

	deleteArgs := []string{
		"delete", rt.Resource, name, "--context", ctx,
		"--grace-period=0", "--force",
	}
	if rt.Namespaced {
		deleteArgs = append(deleteArgs, "-n", ns)
	}

	return func() tea.Msg {
		cmd := exec.Command(kubectlPath, deleteArgs...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
		logger.Info("Running kubectl command", "cmd", cmd.String())
		if output, err := cmd.CombinedOutput(); err != nil {
			logger.Error("kubectl force delete failed", "cmd", cmd.String(), "error", err, "output", string(output))
			return actionResultMsg{err: fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))}
		}
		return actionResultMsg{message: fmt.Sprintf("Force deleted %s/%s", rt.Resource, name)}
	}
}

func (m Model) removeFinalizers() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	rt := m.actionCtx.resourceType
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	logger.Info("Removing finalizers from resource", "resource", rt.Resource, "name", name, "namespace", ns, "context", ctx)

	patchArgs := []string{
		"patch", rt.Resource, name, "--context", ctx,
		"--type", "merge", "-p", `{"metadata":{"finalizers":null}}`,
	}
	if rt.Namespaced {
		patchArgs = append(patchArgs, "-n", ns)
	}

	return func() tea.Msg {
		cmd := exec.Command(kubectlPath, patchArgs...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
		logger.Info("Running kubectl command", "cmd", cmd.String())
		if output, err := cmd.CombinedOutput(); err != nil {
			logger.Error("kubectl patch failed", "cmd", cmd.String(), "error", err, "output", string(output))
			return actionResultMsg{err: fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))}
		}
		return actionResultMsg{message: fmt.Sprintf("Finalizers removed from %s/%s", rt.Resource, name)}
	}
}

func (m Model) uninstallHelmRelease() tea.Cmd {
	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("helm not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	args := []string{"uninstall", name, "-n", ns, "--kube-context", ctx}

	cmd := exec.Command(helmPath, args...)
	logger.Info("Running helm command", "cmd", cmd.String())
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			logger.Error("helm uninstall failed", "cmd", cmd.String(), "error", err)
		}
		return actionResultMsg{message: fmt.Sprintf("Uninstalled %s", name), err: err}
	})
}

// editHelmValues fetches current values, writes them to a temp file, and opens
// in $EDITOR for viewing and editing.
// Uses a shell script via tea.ExecProcess so the editor can take over the terminal.
func (m Model) editHelmValues() tea.Cmd {
	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("helm not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	kubeconfigPaths := m.client.KubeconfigPaths()

	// Build a shell script that: gets values -> writes to temp file -> opens editor ->
	// checks for changes -> applies with helm upgrade --reuse-values using the
	// chart reference from `helm list`.
	script := fmt.Sprintf(`
set -e
HELM=%q
RELEASE=%q
NS=%q
CTX=%q
export KUBECONFIG=%q

TMPFILE=$(mktemp /tmp/helm-values-${RELEASE}-XXXXXX.yaml)

$HELM get values "$RELEASE" -n "$NS" --kube-context "$CTX" -o yaml > "$TMPFILE" 2>&1
# Replace bare 'null' with a helpful comment
if [ "$(cat "$TMPFILE" | tr -d '[:space:]')" = "null" ]; then
  echo "# Add your values here" > "$TMPFILE"
fi

# Save checksum before editing
BEFORE=$(md5sum "$TMPFILE" 2>/dev/null || md5 -q "$TMPFILE" 2>/dev/null || cat "$TMPFILE")

${EDITOR:-${VISUAL:-vi}} "$TMPFILE"

AFTER=$(md5sum "$TMPFILE" 2>/dev/null || md5 -q "$TMPFILE" 2>/dev/null || cat "$TMPFILE")

if [ "$BEFORE" = "$AFTER" ]; then
  rm -f "$TMPFILE"
  echo "No changes detected."
  exit 0
fi

# Parse the chart-version string from helm list JSON, then strip the version
# suffix to get the chart name for repo-based resolution.
CHART_VERSION=$($HELM list -n "$NS" --kube-context "$CTX" --filter "^${RELEASE}$" -o json 2>/dev/null \
  | sed -n 's/.*"chart":"\([^"]*\)".*/\1/p' | head -1)
# Strip trailing -<semver> (e.g. "nginx-ingress-1.2.3" -> "nginx-ingress").
CHART_NAME=$(echo "$CHART_VERSION" | sed 's/-[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*.*$//')
if [ -z "$CHART_NAME" ]; then
  echo ""
  echo "Could not determine chart for release $RELEASE."
  echo "Your edited values have been saved to: $TMPFILE"
  echo "Apply manually with:"
  echo "  helm upgrade $RELEASE <CHART> -n $NS --kube-context $CTX --reuse-values -f $TMPFILE"
  exit 1
fi

echo "Applying values with chart $CHART_NAME..."
if ! $HELM upgrade "$RELEASE" "$CHART_NAME" -n "$NS" --kube-context "$CTX" --reuse-values -f "$TMPFILE" 2>&1; then
  echo ""
  echo "Upgrade failed. Your edited values have been saved to: $TMPFILE"
  echo "You may need to specify the full chart reference. Apply manually with:"
  echo "  helm upgrade $RELEASE <REPO/CHART> -n $NS --kube-context $CTX --reuse-values -f $TMPFILE"
  exit 1
fi
rm -f "$TMPFILE"
`,
		helmPath, name, ns, ctx, kubeconfigPaths,
	)

	cmd := exec.Command("sh", "-c", script)
	cmd.Env = os.Environ()
	logger.Info("Running helm edit values", "release", name, "namespace", ns, "context", ctx)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			logger.Error("helm edit values failed", "release", name, "error", err)
			return actionResultMsg{err: fmt.Errorf("helm edit values: %w", err)}
		}
		return actionResultMsg{message: fmt.Sprintf("Values updated for %s", name)}
	})
}

func (m Model) scaleDeployment(replicas int32) tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	logger.Info("Scaling deployment", "name", name, "replicas", replicas, "namespace", ns, "context", ctx)
	return func() tea.Msg {
		err := m.client.ScaleDeployment(ctx, ns, name, replicas)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Scaled %s to %d replicas", name, replicas)}
	}
}

func (m Model) restartDeployment() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	logger.Info("Restarting deployment", "name", name, "namespace", ns, "context", ctx)
	return func() tea.Msg {
		err := m.client.RestartDeployment(ctx, ns, name)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Restarting %s", name)}
	}
}

// rollbackDeployment performs the actual rollback.
func (m Model) rollbackDeployment(revision int64) tea.Cmd {
	kctx := m.nav.Context
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	client := m.client

	return func() tea.Msg {
		err := client.RollbackDeployment(context.Background(), kctx, ns, name, revision)
		return rollbackDoneMsg{err: err}
	}
}

// rollbackHelmRelease performs a Helm rollback to the specified revision.
func (m Model) rollbackHelmRelease(revision int) tea.Cmd {
	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return func() tea.Msg {
			return helmRollbackDoneMsg{err: fmt.Errorf("helm not found: %w", err)}
		}
	}

	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	kubeconfigPaths := m.client.KubeconfigPaths()

	return func() tea.Msg {
		args := []string{"rollback", name, fmt.Sprintf("%d", revision), "-n", ns, "--kube-context", ctx}
		cmd := exec.Command(helmPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
		logger.Info("Running helm command", "cmd", cmd.String())
		output, cmdErr := cmd.CombinedOutput()
		if cmdErr != nil {
			logger.Error("helm rollback failed", "cmd", cmd.String(), "error", cmdErr, "output", string(output))
			return helmRollbackDoneMsg{err: fmt.Errorf("%w: %s", cmdErr, strings.TrimSpace(string(output)))}
		}
		return helmRollbackDoneMsg{}
	}
}

func (m Model) execKubectlCordon() tea.Cmd {
	return m.execKubectlNodeCmd("cordon")
}

func (m Model) execKubectlUncordon() tea.Cmd {
	return m.execKubectlNodeCmd("uncordon")
}

func (m Model) execKubectlDrain() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}
	name := m.actionCtx.name
	args := []string{
		"drain", name, "--context", m.actionCtx.context,
		"--ignore-daemonsets", "--delete-emptydir-data",
	}

	cmd := exec.Command(kubectlPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
	logger.Info("Running kubectl command", "cmd", cmd.String())
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			logger.Error("kubectl drain failed", "cmd", cmd.String(), "error", err)
			return actionResultMsg{err: fmt.Errorf("drain %s: %w", name, err)}
		}
		return actionResultMsg{message: fmt.Sprintf("Drained %s", name)}
	})
}

func (m Model) execKubectlNodeCmd(subcmd string) tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}
	name := m.actionCtx.name
	args := []string{subcmd, name, "--context", m.actionCtx.context}

	return func() tea.Msg {
		cmd := exec.Command(kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
		logger.Info("Running kubectl command", "cmd", cmd.String())
		output, err := cmd.CombinedOutput()
		if err != nil {
			logger.Error("kubectl node command failed", "cmd", cmd.String(), "error", err, "output", string(output))
			return actionResultMsg{err: fmt.Errorf("%s %s: %s", subcmd, name, strings.TrimSpace(string(output)))}
		}
		return actionResultMsg{message: strings.TrimSpace(string(output))}
	}
}

// triggerCronJob creates a Job from a CronJob.
func (m Model) triggerCronJob() tea.Cmd {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	kctx := m.actionCtx.context
	client := m.client

	return func() tea.Msg {
		jobName, err := client.TriggerCronJob(context.Background(), kctx, ns, name)
		return triggerCronJobMsg{jobName: jobName, err: err}
	}
}

// execKubectlNodeShell opens a debug shell on a node using kubectl debug.
func (m Model) execKubectlNodeShell() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	nodeName := m.actionCtx.name
	ctx := m.actionCtx.context

	args := []string{
		"debug", "node/" + nodeName, "-it",
		"--image=busybox",
		"--context", ctx,
		"--", "chroot", "/host", "/bin/sh",
	}

	cmd := exec.Command(kubectlPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
	logger.Info("Running kubectl command", "cmd", cmd.String())

	if ui.ConfigTerminalMode == "pty" {
		cols := m.width - 4
		rows := m.height - 6
		if cols < 20 {
			cols = 80
		}
		if rows < 5 {
			rows = 24
		}
		title := fmt.Sprintf("Node Shell: %s", nodeName)
		return startPTYExecCmd(cmd, title, cols, rows)
	}

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return actionResultMsg{err: fmt.Errorf("node shell: %w", err)}
		}
		return actionResultMsg{message: "Node shell session ended"}
	})
}

// execKubectlExplain runs kubectl explain for a resource (optionally at a field path)
// and returns the parsed output as an explainLoadedMsg.
func (m Model) execKubectlExplain(resource, fieldPath string) tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return explainLoadedMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	kctx := m.nav.Context
	kubeconfigPaths := m.client.KubeconfigPaths()

	target := resource
	if fieldPath != "" {
		target = resource + "." + fieldPath
	}

	title := resource
	if fieldPath != "" {
		title = resource + " > " + strings.ReplaceAll(fieldPath, ".", " > ")
	}

	return func() tea.Msg {
		args := []string{"explain", target, "--context", kctx}
		cmd := exec.Command(kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
		logger.Info("Running kubectl command", "cmd", cmd.String())
		output, cmdErr := cmd.CombinedOutput()
		if cmdErr != nil {
			logger.Error("kubectl explain failed", "cmd", cmd.String(), "error", cmdErr, "output", string(output))
			return explainLoadedMsg{
				err: fmt.Errorf("%w: %s", cmdErr, strings.TrimSpace(string(output))),
			}
		}

		desc, fields := parseExplainOutput(string(output), fieldPath)
		return explainLoadedMsg{
			fields:      fields,
			description: desc,
			title:       title,
			path:        fieldPath,
		}
	}
}

// execCustomAction runs a user-defined custom action command via sh -c.
// The command is executed with the terminal handed over via tea.ExecProcess,
// allowing interactive commands to work properly.
func (m Model) execCustomAction(expandedCmd string) tea.Cmd {
	cmd := exec.Command("sh", "-c", expandedCmd)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
	logger.Info("Running custom action", "cmd", cmd.String())

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			logger.Error("Custom action failed", "cmd", cmd.String(), "error", err)
			return actionResultMsg{err: fmt.Errorf("custom action failed: %w", err)}
		}
		return actionResultMsg{message: "Custom action completed"}
	})
}
