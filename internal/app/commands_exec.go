package app

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/ui"
)

// --- Action commands ---

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

// --- Debug commands ---

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

	if ui.ConfigTerminalMode == "pty" {
		cols := m.width - 4
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

	return tea.ExecProcess(clearBeforeExec(cmd), func(err error) tea.Msg {
		if err != nil {
			logger.Error("kubectl run debug pod with PVC failed", "cmd", cmd.String(), "error", err)
		}
		return actionResultMsg{message: "Debug pod session ended", err: err}
	})
}

// --- Resource management commands ---

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

// --- Helm commands ---

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

// --- Deployment and scaling commands ---

func (m Model) scaleResource(replicas int32) tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	kind := m.actionCtx.kind
	logger.Info("Scaling resource", "kind", kind, "name", name, "replicas", replicas, "namespace", ns, "context", ctx)
	return func() tea.Msg {
		err := m.client.ScaleResource(ctx, ns, name, kind, replicas)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Scaled %s to %d replicas", name, replicas)}
	}
}

func (m Model) restartResource() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	kind := m.actionCtx.kind
	logger.Info("Restarting resource", "kind", kind, "name", name, "namespace", ns, "context", ctx)
	return func() tea.Msg {
		err := m.client.RestartResource(ctx, ns, name, kind)
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

// --- Node commands ---

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

// --- Explain commands ---

// execKubectlExplain runs kubectl explain for a resource (optionally at a field path)
// and returns the parsed output as an explainLoadedMsg.
// apiVersion is the "group/version" string for --api-version flag (empty for core resources).
func (m Model) execKubectlExplain(resource, apiVersion, fieldPath string) tea.Cmd {
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
	if apiVersion != "" {
		title = resource + " (" + apiVersion + ")"
	}
	if fieldPath != "" {
		title = title + " > " + strings.ReplaceAll(fieldPath, ".", " > ")
	}

	return func() tea.Msg {
		args := []string{"explain", target, "--context", kctx}
		if apiVersion != "" {
			args = append(args, "--api-version", apiVersion)
		}
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

// execKubectlExplainRecursive runs kubectl explain --recursive and searches for matching fields.
func (m Model) execKubectlExplainRecursive(resource, apiVersion, query string) tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return explainRecursiveMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	kctx := m.nav.Context
	kubeconfigPaths := m.client.KubeconfigPaths()

	return func() tea.Msg {
		args := []string{"explain", resource, "--recursive", "--context", kctx}
		if apiVersion != "" {
			args = append(args, "--api-version", apiVersion)
		}
		cmd := exec.Command(kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
		logger.Info("Running kubectl command", "cmd", cmd.String())
		output, cmdErr := cmd.CombinedOutput()
		if cmdErr != nil {
			return explainRecursiveMsg{
				err: fmt.Errorf("%w: %s", cmdErr, strings.TrimSpace(string(output))),
			}
		}

		matches := parseRecursiveExplainForSearch(string(output), query)
		return explainRecursiveMsg{matches: matches, query: query}
	}
}

// --- Custom action commands ---

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
