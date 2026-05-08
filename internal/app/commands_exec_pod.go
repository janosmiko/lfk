package app

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) execKubectlExec() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	args := []string{"exec", "-it", m.actionCtx.name, "-n", ns, "--context", m.kubectlContext(m.actionCtx.context)}
	if m.actionCtx.containerName != "" {
		args = append(args, "-c", m.actionCtx.containerName)
	}
	args = append(args, "--", "/bin/sh", "-c", "clear; command -v bash >/dev/null && exec bash || { command -v ash >/dev/null && exec ash || exec sh; }")

	logger.Info("Starting kubectl exec", "args", strings.Join(args, " "))
	cmd := exec.Command(kubectlPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPathForContext(m.actionCtx.context))

	if ui.ConfigTerminalMode == ui.TerminalModePTY {
		cols := m.width
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

	title := fmt.Sprintf("Exec: %s/%s", m.actionNamespace(), m.actionCtx.name)
	return runInteractiveShellExec(cmd, title, "Exec", true)
}

func (m Model) execKubectlAttach() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	args := []string{"attach", "-it", m.actionCtx.name, "-n", ns, "--context", m.kubectlContext(m.actionCtx.context)}
	if m.actionCtx.containerName != "" {
		args = append(args, "-c", m.actionCtx.containerName)
	}

	cmd := exec.Command(kubectlPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPathForContext(m.actionCtx.context))
	logExecCmd("Running kubectl command", cmd)

	if ui.ConfigTerminalMode == ui.TerminalModePTY {
		cols := m.width
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

	title := fmt.Sprintf("Attach: %s/%s", m.actionNamespace(), m.actionCtx.name)
	return runInteractiveShellExec(cmd, title, "Attach", true)
}

func (m Model) execKubectlDebug() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	args := []string{"debug", m.actionCtx.name, "-it", "--image=busybox", "--context", m.kubectlContext(m.actionCtx.context), "-n", ns}

	cmd := exec.Command(kubectlPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPathForContext(m.actionCtx.context))
	logExecCmd("Running kubectl command", cmd)

	if ui.ConfigTerminalMode == ui.TerminalModePTY {
		cols := m.width
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

	title := fmt.Sprintf("Debug: %s/%s", m.actionNamespace(), m.actionCtx.name)
	return runInteractiveShellExec(cmd, title, "Debug", false)
}

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
		"--restart=Never", "-n", ns, "--context", m.kubectlContext(ctx), "--", "sh",
	}

	logger.Info("Running debug pod", "pod", podName, "namespace", ns, "context", ctx)

	cmd := exec.Command(kubectlPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPathForContext(ctx))

	if ui.ConfigTerminalMode == ui.TerminalModePTY {
		cols := m.width
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

	title := fmt.Sprintf("Debug Pod: %s/%s", ns, podName)
	return runInteractiveShellExec(cmd, title, "Debug pod", true)
}

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
				"volumeMounts": [{"name": "data", "mountPath": "/data"}]
			}],
			"volumes": [{"name": "data", "persistentVolumeClaim": {"claimName": "%s"}}],
			"restartPolicy": "Never"
		}
	}`, podName, pvcName)

	args := []string{
		"run", podName, "--image=alpine", "-it", "--rm",
		"--restart=Never", "--context", m.kubectlContext(ctx), "-n", ns,
		"--overrides", manifest, "--", "sh",
	}

	cmd := exec.Command(kubectlPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPathForContext(ctx))
	logExecCmd("Running kubectl command", cmd)

	if ui.ConfigTerminalMode == ui.TerminalModePTY {
		cols := m.width
		rows := m.height - 6
		if cols < 20 {
			cols = 80
		}
		if rows < 5 {
			rows = 24
		}
		title := fmt.Sprintf("Debug PVC: %s/%s → %s", ns, pvcName, podName)
		return startPTYExecCmd(cmd, title, cols, rows)
	}

	title := fmt.Sprintf("Debug PVC: %s/%s → %s", ns, pvcName, podName)
	return runInteractiveShellExec(cmd, title, "Debug pod", true)
}

// nodeShellNamespace is the namespace nodeshell pods are always created in.
//
// nodeshell needs system-node-critical priority so the kubelet admits the pod
// on nodes with DiskPressure / MemoryPressure / PIDPressure conditions
// (admission rejects non-critical pods regardless of tolerations). The
// built-in Priority admission plugin restricts that priority class to pods
// in kube-system, so the namespace is pinned here.
const nodeShellNamespace = "kube-system"

func nodeShellOverrides(podName, nodeName string) (string, error) {
	spec := map[string]any{
		"apiVersion": "v1",
		"spec": map[string]any{
			"hostPID":     true,
			"hostIPC":     true,
			"hostNetwork": true,
			"nodeName":    nodeName,
			// system-node-critical lifts the pod above the kubelet eviction
			// manager's admission gate so it can land on nodes reporting
			// DiskPressure / PIDPressure / MemoryPressure conditions. The
			// priority class is built into Kubernetes since 1.11 and is
			// reserved for the kube-system namespace (see nodeShellNamespace).
			"priorityClassName": "system-node-critical",
			// Tolerate every taint on every effect so the pod can land on
			// control-plane, NotReady, Unschedulable, or pressure-tainted
			// nodes. {operator: Exists} with no key/effect already matches
			// all taints per the Kubernetes spec; the per-effect entries are
			// kept for readability and to make the intent grep-able.
			"tolerations": []map[string]any{
				{"operator": "Exists"},
				{"operator": "Exists", "effect": "NoSchedule"},
				{"operator": "Exists", "effect": "PreferNoSchedule"},
				{"operator": "Exists", "effect": "NoExecute"},
			},
			"containers": []map[string]any{{
				"name":  podName,
				"image": "busybox",
				"stdin": true,
				"tty":   true,
				"securityContext": map[string]any{
					"privileged": true,
				},
				"command": []string{
					"nsenter",
					"--target", "1",
					"--mount", "--uts", "--ipc", "--net", "--pid",
					"--", "/bin/sh",
				},
			}},
		},
	}
	b, err := json.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("failed to marshal node shell pod spec: %w", err)
	}
	return string(b), nil
}

func nodeShellArgs(podName, namespace, kctx, overrides string) []string {
	if namespace == "" {
		namespace = "default"
	}
	return []string{
		"run", podName,
		"-n", namespace,
		"--rm", "-it", "--restart=Never",
		"--image=busybox",
		"--context", kctx,
		"--overrides=" + overrides,
	}
}

func (m Model) execKubectlNodeShell() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	nodeName := m.actionCtx.name
	ctx := m.actionCtx.context
	podName := "lfk-node-shell-" + randomSuffix(5)

	overrides, err := nodeShellOverrides(podName, nodeName)
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: err}
		}
	}

	args := nodeShellArgs(podName, nodeShellNamespace, m.kubectlContext(ctx), overrides)
	cmd := exec.Command(kubectlPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPathForContext(ctx))
	logExecCmd("Running kubectl command", cmd)

	if ui.ConfigTerminalMode == ui.TerminalModePTY {
		cols := m.width
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

	title := fmt.Sprintf("Node Shell: %s", nodeName)
	return runInteractiveShellExec(cmd, title, "Node shell", true)
}

func (m Model) execKubectlExplain(resource, apiVersion, fieldPath string) tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return explainLoadedMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	kctx := m.nav.Context
	kubeconfigPaths := m.client.KubeconfigPathForContext(kctx)

	target := resource
	if fieldPath != "" {
		target = resource + "." + fieldPath
	}

	title := resource
	if apiVersion != "" {
		title = resource + " (" + apiVersion + ")"
	}
	if fieldPath != "" {
		title = title + " > " + strings.ReplaceAll(fieldPath, ".", " > ")
	}

	return m.trackBgTask(scheduler.KindSubprocess, "Explain: "+target, kctx, func() tea.Msg {
		args := []string{"explain", target, "--context", m.kubectlContext(kctx)}
		if apiVersion != "" {
			args = append(args, "--api-version", apiVersion)
		}
		cmd := exec.Command(kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
		logExecCmd("Running kubectl command", cmd)
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
	})
}

func (m Model) execKubectlExplainRecursive(resource, apiVersion, query string) tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return explainRecursiveMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	kctx := m.nav.Context
	kubeconfigPaths := m.client.KubeconfigPathForContext(kctx)

	return m.trackBgTask(scheduler.KindSubprocess, "Explain (recursive): "+resource, kctx, func() tea.Msg {
		args := []string{"explain", resource, "--recursive", "--context", m.kubectlContext(kctx)}
		if apiVersion != "" {
			args = append(args, "--api-version", apiVersion)
		}
		cmd := exec.Command(kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
		logExecCmd("Running kubectl command", cmd)
		output, cmdErr := cmd.CombinedOutput()
		if cmdErr != nil {
			return explainRecursiveMsg{
				err: fmt.Errorf("%w: %s", cmdErr, strings.TrimSpace(string(output))),
			}
		}

		matches := parseRecursiveExplainForSearch(string(output), query)
		return explainRecursiveMsg{matches: matches, query: query}
	})
}
