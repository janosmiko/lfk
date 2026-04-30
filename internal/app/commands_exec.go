package app

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/app/bgtasks"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/ui"
)

// --- Action commands ---

// runInteractiveShellExec returns a tea.Cmd that runs an interactive shell
// command. The behaviour is selected by ui.ConfigTerminalMode:
//
//   - TerminalModeExec: hand the host terminal to cmd via tea.ExecProcess
//     (lfk suspends for the duration). clearBefore controls whether cmd is
//     wrapped in clearBeforeExec to flush TUI artifacts before the shell
//     paints.
//   - TerminalModeMux: open cmd in a new window/pane of the surrounding
//     multiplexer (tmux or zellij). lfk stays foregrounded. Returns an
//     actionResultMsg error if no multiplexer is detected.
//
// TerminalModePTY is handled separately by the callers (startPTYExecCmd)
// and never reaches this helper.
//
// title is the window/tab name when the multiplexer supports one;
// sessionLabel is used for status messages ("Exec", "Attach", "Debug",
// "Debug pod", "Node shell").
func runInteractiveShellExec(cmd *exec.Cmd, title, sessionLabel string, clearBefore bool) tea.Cmd {
	if ui.ConfigTerminalMode == ui.TerminalModeMux {
		mx := detectMultiplexer(nil, nil)
		if mx == nil {
			return func() tea.Msg {
				return actionResultMsg{
					err: fmt.Errorf("terminal mode 'mux' requires running inside tmux or zellij — none detected; switch to pty/exec or set TMUX/ZELLIJ"),
				}
			}
		}
		wrapped := mx.wrap(cmd, title, os.Environ())
		mxName := mx.name
		// tmux groups panes into windows; zellij's `run` opens a floating
		// pane within the current tab. Use the term that matches the
		// multiplexer the user is in so the status message matches what
		// they actually see.
		paneNoun := "window"
		if mxName == "zellij" {
			paneNoun = "pane"
		}
		return func() tea.Msg {
			logExecCmd("Opening "+sessionLabel+" in "+mxName, wrapped)
			output, err := wrapped.CombinedOutput()
			if err != nil {
				logger.Error(sessionLabel+" multiplexer spawn failed", "error", err, "output", string(output))
				return actionResultMsg{
					err: fmt.Errorf("opening %s in new %s %s: %w: %s",
						sessionLabel, mxName, paneNoun, err, strings.TrimSpace(string(output))),
				}
			}
			return actionResultMsg{
				message: fmt.Sprintf("%s opened in new %s %s", sessionLabel, mxName, paneNoun),
			}
		}
	}
	// TerminalModeExec (and any unrecognised value): suspend lfk and hand
	// the host terminal to cmd.
	fallback := cmd
	if clearBefore {
		fallback = clearBeforeExec(cmd)
	}
	return tea.ExecProcess(fallback, func(err error) tea.Msg {
		if err != nil {
			logger.Error(sessionLabel+" session failed", "cmd", cmd.String(), "error", err)
		}
		return actionResultMsg{
			message: sessionLabel + " session ended",
			err:     err,
		}
	})
}

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

func (m Model) execKubectlEdit() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	rt := m.actionCtx.resourceType
	args := []string{"edit", rt.Resource, m.actionCtx.name, "--context", m.kubectlContext(m.actionCtx.context)}
	if rt.Namespaced {
		args = append(args, "-n", ns)
	}

	cmd := exec.Command(kubectlPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPathForContext(m.actionCtx.context))
	logExecCmd("Running kubectl command", cmd)
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
	args := []string{"describe", rt.Resource, name, "--context", m.kubectlContext(m.actionCtx.context)}
	if rt.Namespaced {
		args = append(args, "-n", ns)
	}

	title := fmt.Sprintf("Describe: %s/%s", rt.Resource, name)

	return m.trackBgTask(bgtasks.KindSubprocess, title, bgtaskTarget(m.actionCtx.context, ns), func() tea.Msg {
		cmd := exec.Command(kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPathForContext(m.actionCtx.context))
		logExecCmd("Running kubectl command", cmd)
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
	})
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
	// `kubectl debug` already clears before attaching to the ephemeral
	// container; an extra clearBeforeExec would briefly flash the TUI's
	// reset sequence over the ephemeral session start.
	return runInteractiveShellExec(cmd, title, "Debug", false)
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

// randomSuffix generates a random lowercase alphanumeric string of the given length.
func randomSuffix(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[rand.IntN(len(chars))]
	}
	return string(b)
}

// runDebugPodWithPVC runs an alpine debug pod with a PVC mounted at /data.
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
				"volumeMounts": [{"name": "data", "mountPath": "/data"}]
			}],
			"volumes": [{"name": "data", "persistentVolumeClaim": {"claimName": "%s"}}],
			"restartPolicy": "Never"
		}
	}`, podName, pvcName)

	// Use kubectl run with overrides to mount the PVC.
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
	return m.trackBgTask(bgtasks.KindMutation, fmt.Sprintf("Delete %s/%s", rt.Resource, name), bgtaskTarget(ctx, ns), func() tea.Msg {
		err := m.client.DeleteResource(ctx, ns, rt, name)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Deleted %s/%s", rt.Resource, name)}
	})
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
		"delete", rt.Resource, name, "--context", m.kubectlContext(ctx),
		"--grace-period=0", "--force",
	}
	if rt.Namespaced {
		deleteArgs = append(deleteArgs, "-n", ns)
	}

	return m.trackBgTask(bgtasks.KindMutation, fmt.Sprintf("Force delete %s/%s", rt.Resource, name), bgtaskTarget(ctx, ns), func() tea.Msg {
		cmd := exec.Command(kubectlPath, deleteArgs...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPathForContext(ctx))
		logExecCmd("Running kubectl command", cmd)
		if output, err := cmd.CombinedOutput(); err != nil {
			logger.Error("kubectl force delete failed", "cmd", cmd.String(), "error", err, "output", string(output))
			return actionResultMsg{err: fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))}
		}
		return actionResultMsg{message: fmt.Sprintf("Force deleted %s/%s", rt.Resource, name)}
	})
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
		"patch", rt.Resource, name, "--context", m.kubectlContext(ctx),
		"--type", "merge", "-p", `{"metadata":{"finalizers":null}}`,
	}
	if rt.Namespaced {
		patchArgs = append(patchArgs, "-n", ns)
	}

	return m.trackBgTask(bgtasks.KindMutation, fmt.Sprintf("Remove finalizers: %s/%s", rt.Resource, name), bgtaskTarget(ctx, ns), func() tea.Msg {
		cmd := exec.Command(kubectlPath, patchArgs...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPathForContext(ctx))
		logExecCmd("Running kubectl command", cmd)
		if output, err := cmd.CombinedOutput(); err != nil {
			logger.Error("kubectl patch failed", "cmd", cmd.String(), "error", err, "output", string(output))
			return actionResultMsg{err: fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))}
		}
		return actionResultMsg{message: fmt.Sprintf("Finalizers removed from %s/%s", rt.Resource, name)}
	})
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
	args := []string{"uninstall", name, "-n", ns, "--kube-context", m.kubectlContext(ctx)}

	cmd := exec.Command(helmPath, args...)
	logExecCmd("Running helm command", cmd)
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
	kubeconfigPaths := m.client.KubeconfigPathForContext(ctx)
	// helm only knows the kubeconfig's literal context name, not lfk's
	// disambiguated display form, so translate before embedding the script.
	helmCtx := m.kubectlContext(ctx)

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
		helmPath, name, ns, helmCtx, kubeconfigPaths,
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

// helmDiff fetches the default chart values and user-supplied values,
// then returns them for side-by-side comparison in the diff viewer.
// It tries to resolve the repo-qualified chart name via "helm search repo"
// to get true defaults; falls back to "helm get values --all" if the chart
// is not in a configured repo.
func (m Model) helmDiff() tea.Cmd {
	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return func() tea.Msg {
			return diffLoadedMsg{err: fmt.Errorf("helm not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	kubeconfigPaths := m.client.KubeconfigPathForContext(ctx)

	return m.trackBgTask(bgtasks.KindSubprocess, "Helm diff: "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		env := append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)

		// Resolve bare chart name from the release (e.g. "cilium-1.16.0" -> "cilium").
		chartName := resolveHelmChartName(helmPath, name, ns, m.kubectlContext(ctx), kubeconfigPaths)

		// Try to get true default values via "helm show values <repo/chart>".
		defaultOut, leftLabel := helmShowDefaultValues(helmPath, chartName, env)

		// If we couldn't get true defaults, fall back to merged values.
		if defaultOut == "" {
			logger.Info("helm show values unavailable, falling back to --all", "chart", chartName)
			allArgs := []string{"get", "values", name, "--all", "-n", ns, "--kube-context", m.kubectlContext(ctx), "-o", "yaml"}
			allCmd := exec.Command(helmPath, allArgs...)
			allCmd.Env = env
			allOut, allErr := allCmd.CombinedOutput()
			if allErr != nil {
				return diffLoadedMsg{err: fmt.Errorf("getting all values: %w: %s", allErr, strings.TrimSpace(string(allOut)))}
			}
			defaultOut = string(allOut)
			leftLabel = "All Values (defaults + overrides)"
		}

		// Get user-supplied values only.
		userArgs := []string{"get", "values", name, "-n", ns, "--kube-context", m.kubectlContext(ctx), "-o", "yaml"}
		userCmd := exec.Command(helmPath, userArgs...)
		userCmd.Env = env
		logExecCmd("Running helm command", userCmd)
		userOut, userErr := userCmd.CombinedOutput()
		if userErr != nil {
			return diffLoadedMsg{err: fmt.Errorf("getting user values: %w: %s", userErr, strings.TrimSpace(string(userOut)))}
		}

		return diffLoadedMsg{
			left:      defaultOut,
			right:     string(userOut),
			leftName:  leftLabel,
			rightName: "User Values",
		}
	})
}

// resolveHelmChartName extracts the bare chart name from "helm list" output.
// Returns empty string on failure. ctx must already be the kubeconfig's
// literal context name (callers should translate via Model.kubectlContext).
func resolveHelmChartName(helmPath, release, ns, ctx, kubeconfigPaths string) string {
	args := []string{"list", "-n", ns, "--kube-context", ctx, "--filter", "^" + release + "$", "-o", "json"}
	cmd := exec.Command(helmPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

	out := strings.TrimSpace(string(output))
	_, after, found := strings.Cut(out, `"chart":"`)
	if !found {
		return ""
	}
	chartVersion, _, found := strings.Cut(after, `"`)
	if !found {
		return ""
	}

	// Strip trailing -<semver> to get chart name.
	parts := strings.Split(chartVersion, "-")
	if last := len(parts) - 1; last > 0 && len(parts[last]) > 0 && parts[last][0] >= '0' && parts[last][0] <= '9' {
		return strings.Join(parts[:last], "-")
	}
	return chartVersion
}

// helmShowDefaultValues tries to get default chart values via "helm show values".
// It first searches configured repos for the repo-qualified name. Returns the
// output and label on success, or empty string on failure.
func helmShowDefaultValues(helmPath, chartName string, env []string) (string, string) {
	if chartName == "" {
		return "", ""
	}

	// Search repos for the chart to get the repo-qualified name (e.g. "cilium/cilium").
	searchArgs := []string{"search", "repo", chartName, "-o", "json"}
	searchCmd := exec.Command(helmPath, searchArgs...)
	searchCmd.Env = env
	logExecCmd("Running helm command", searchCmd)
	searchOut, searchErr := searchCmd.CombinedOutput()
	if searchErr != nil {
		return "", ""
	}

	// Parse first matching "name" field from JSON array: [{"name":"repo/chart",...}]
	repoChart := parseFirstJSONField(string(searchOut), "name", chartName)
	if repoChart == "" {
		return "", ""
	}

	// Get the default values from the chart.
	showArgs := []string{"show", "values", repoChart}
	showCmd := exec.Command(helmPath, showArgs...)
	showCmd.Env = env
	logExecCmd("Running helm command", showCmd)
	showOut, showErr := showCmd.CombinedOutput()
	if showErr != nil {
		return "", ""
	}
	return string(showOut), "Default Values (" + repoChart + ")"
}

// parseFirstJSONField finds the first "name":"value" in a JSON array where value
// ends with the given suffix (bare chart name). Returns the value or empty string.
func parseFirstJSONField(jsonStr, field, suffix string) string {
	needle := `"` + field + `":"`
	rest := jsonStr
	for {
		_, after, found := strings.Cut(rest, needle)
		if !found {
			return ""
		}
		value, remaining, found := strings.Cut(after, `"`)
		if !found {
			return ""
		}
		// Match: value ends with /chartName or equals chartName exactly.
		if value == suffix || strings.HasSuffix(value, "/"+suffix) {
			return value
		}
		rest = remaining
	}
}

// helmUpgrade runs "helm upgrade" interactively via tea.ExecProcess.
func (m Model) helmUpgrade() tea.Cmd {
	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("helm not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	kubeconfigPaths := m.client.KubeconfigPathForContext(ctx)

	// Build a shell script that resolves the chart and runs helm upgrade --reuse-values.
	script := fmt.Sprintf(`
set -e
HELM=%q
RELEASE=%q
NS=%q
CTX=%q
export KUBECONFIG=%q

CHART_VERSION=$($HELM list -n "$NS" --kube-context "$CTX" --filter "^${RELEASE}$" -o json 2>/dev/null \
  | sed -n 's/.*"chart":"\([^"]*\)".*/\1/p' | head -1)
CHART_NAME=$(echo "$CHART_VERSION" | sed 's/-[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*.*$//')
if [ -z "$CHART_NAME" ]; then
  echo "Could not determine chart for release $RELEASE."
  echo "Run manually: helm upgrade $RELEASE <CHART> -n $NS --kube-context $CTX --reuse-values"
  exit 1
fi

echo "Upgrading $RELEASE with chart $CHART_NAME..."
$HELM upgrade "$RELEASE" "$CHART_NAME" -n "$NS" --kube-context "$CTX" --reuse-values
`,
		helmPath, name, ns, m.kubectlContext(ctx), kubeconfigPaths,
	)

	cmd := exec.Command("sh", "-c", script)
	cmd.Env = os.Environ()
	logger.Info("Running helm upgrade", "release", name, "namespace", ns, "context", ctx)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			logger.Error("helm upgrade failed", "release", name, "error", err)
			return actionResultMsg{err: fmt.Errorf("helm upgrade: %w", err)}
		}
		return actionResultMsg{message: fmt.Sprintf("Upgraded %s", name)}
	})
}

// --- Vulnerability scanning commands ---

// vulnScanImage runs trivy to scan a container image for vulnerabilities
// and returns the output for display in the describe viewer.
func (m Model) vulnScanImage(image string) tea.Cmd {
	trivyPath, err := exec.LookPath("trivy")
	if err != nil {
		return func() tea.Msg {
			return describeLoadedMsg{
				title: "Vulnerability Scan",
				err:   fmt.Errorf("trivy not found in PATH: %w (install: https://aquasecurity.github.io/trivy)", err),
			}
		}
	}

	title := fmt.Sprintf("Vuln Scan: %s", image)
	return m.trackBgTask(bgtasks.KindSubprocess, title, "", func() tea.Msg {
		args := []string{"image", "--scanners", "vuln", "--format", "table", "--no-progress", image}
		cmd := exec.Command(trivyPath, args...)
		cmd.Env = os.Environ()
		logExecCmd("Running trivy command", cmd)
		output, cmdErr := cmd.CombinedOutput()
		content := cleanANSI(strings.TrimSpace(string(output)))
		if cmdErr != nil {
			logger.Error("trivy scan failed", "cmd", cmd.String(), "error", cmdErr, "output", content)
			if content == "" {
				return describeLoadedMsg{title: title, err: fmt.Errorf("trivy scan failed: %w", cmdErr)}
			}
			// Show trivy output even on non-zero exit (may contain partial results).
			return describeLoadedMsg{content: content, title: title}
		}
		if content == "" {
			content = "No vulnerabilities found."
		}
		return describeLoadedMsg{content: content, title: title}
	})
}

// cleanANSI removes ANSI escape sequences from a string.
func cleanANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			// Skip until the terminating letter.
			j := i + 2
			for j < len(s) && (s[j] < 'A' || s[j] > 'Z') && (s[j] < 'a' || s[j] > 'z') {
				j++
			}
			if j < len(s) {
				j++ // skip the terminating letter
			}
			i = j
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// --- PVC commands ---

func (m Model) resizePVC(newSize string) tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	logger.Info("Resizing PVC", "name", name, "newSize", newSize, "namespace", ns, "context", ctx)
	return m.trackBgTask(bgtasks.KindMutation, "Resize PVC: "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		err := m.client.ResizePVC(ctx, ns, name, newSize)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Resize requested for %s to %s", name, newSize)}
	})
}

// --- Deployment and scaling commands ---

func (m Model) scaleResource(replicas int32) tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	kind := m.actionCtx.kind
	logger.Info("Scaling resource", "kind", kind, "name", name, "replicas", replicas, "namespace", ns, "context", ctx)
	return m.trackBgTask(bgtasks.KindMutation, fmt.Sprintf("Scale %s/%s → %d", kind, name, replicas), bgtaskTarget(ctx, ns), func() tea.Msg {
		err := m.client.ScaleResource(ctx, ns, name, kind, replicas)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Scaled %s to %d replicas", name, replicas)}
	})
}

func (m Model) restartResource() tea.Cmd {
	ctx := m.actionCtx.context
	ns := m.actionNamespace()
	name := m.actionCtx.name
	kind := m.actionCtx.kind
	logger.Info("Restarting resource", "kind", kind, "name", name, "namespace", ns, "context", ctx)
	return m.trackBgTask(bgtasks.KindMutation, fmt.Sprintf("Restart %s/%s", kind, name), bgtaskTarget(ctx, ns), func() tea.Msg {
		err := m.client.RestartResource(ctx, ns, name, kind)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("Restarting %s", name)}
	})
}

// rollbackDeployment performs the actual rollback.
func (m Model) rollbackDeployment(revision int64) tea.Cmd {
	kctx := m.nav.Context
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	client := m.client

	return m.trackBgTask(bgtasks.KindMutation, fmt.Sprintf("Rollback Deployment: %s@%d", name, revision), bgtaskTarget(kctx, ns), func() tea.Msg {
		err := client.RollbackDeployment(context.Background(), kctx, ns, name, revision)
		return rollbackDoneMsg{err: err}
	})
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
	kubeconfigPaths := m.client.KubeconfigPathForContext(ctx)

	return m.trackBgTask(bgtasks.KindSubprocess, fmt.Sprintf("Helm rollback: %s@%d", name, revision), bgtaskTarget(ctx, ns), func() tea.Msg {
		args := []string{"rollback", name, fmt.Sprintf("%d", revision), "-n", ns, "--kube-context", m.kubectlContext(ctx)}
		cmd := exec.Command(helmPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
		logExecCmd("Running helm command", cmd)
		output, cmdErr := cmd.CombinedOutput()
		if cmdErr != nil {
			logger.Error("helm rollback failed", "cmd", cmd.String(), "error", cmdErr, "output", string(output))
			return helmRollbackDoneMsg{err: fmt.Errorf("%w: %s", cmdErr, strings.TrimSpace(string(output)))}
		}
		return helmRollbackDoneMsg{}
	})
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
		"drain", name, "--context", m.kubectlContext(m.actionCtx.context),
		"--ignore-daemonsets", "--delete-emptydir-data",
	}

	cmd := exec.Command(kubectlPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPathForContext(m.actionCtx.context))
	logExecCmd("Running kubectl command", cmd)
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
	args := []string{subcmd, name, "--context", m.kubectlContext(m.actionCtx.context)}

	return m.trackBgTask(bgtasks.KindMutation, fmt.Sprintf("%s node: %s", subcmd, name), m.actionCtx.context, func() tea.Msg {
		cmd := exec.Command(kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPathForContext(m.actionCtx.context))
		logExecCmd("Running kubectl command", cmd)
		output, err := cmd.CombinedOutput()
		if err != nil {
			logger.Error("kubectl node command failed", "cmd", cmd.String(), "error", err, "output", string(output))
			return actionResultMsg{err: fmt.Errorf("%s %s: %s", subcmd, name, strings.TrimSpace(string(output)))}
		}
		return actionResultMsg{message: strings.TrimSpace(string(output))}
	})
}

// triggerCronJob creates a Job from a CronJob.
func (m Model) triggerCronJob() tea.Cmd {
	ns := m.actionCtx.namespace
	name := m.actionCtx.name
	kctx := m.actionCtx.context
	client := m.client

	return m.trackBgTask(bgtasks.KindMutation, "Trigger CronJob: "+name, bgtaskTarget(kctx, ns), func() tea.Msg {
		jobName, err := client.TriggerCronJob(context.Background(), kctx, ns, name)
		return triggerCronJobMsg{jobName: jobName, err: err}
	})
}

// nodeShellOverrides builds the JSON pod-spec override that kubectl run
// applies to the auto-generated pod. The pod is privileged with hostPID,
// hostIPC, and hostNetwork enabled, pinned to the target node, and runs
// nsenter into PID 1's namespaces.
//
// Why we don't use `kubectl debug node/<name>`: that path's profiles all
// mount `hostPath: /` at `/host` so callers can `chroot /host`. On
// SELinux-enforcing distros (openSUSE MicroOS, Talos, Bottlerocket) the
// host-root mount keeps the container labelled `container_t`, where
// `nsenter` cannot read `/proc/1/ns/*` (EPERM). A clean privileged pod
// without that mount gets labelled `spc_t` (super privileged container)
// where namespace entry succeeds. This is the same shape k9s and Lens
// produce for their node shell feature.
func nodeShellOverrides(podName, nodeName string) (string, error) {
	spec := map[string]any{
		"apiVersion": "v1",
		"spec": map[string]any{
			"hostPID":     true,
			"hostIPC":     true,
			"hostNetwork": true,
			"nodeName":    nodeName,
			"tolerations": []map[string]any{{"operator": "Exists"}},
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

// nodeShellArgs builds the kubectl-run arguments that, combined with the
// override JSON, launch a privileged debug pod on the target node and
// attach to it. See nodeShellOverrides for why we avoid `kubectl debug`.
func nodeShellArgs(podName, kctx, overrides string) []string {
	return []string{
		"run", podName,
		"-n", "default",
		"--rm", "-it", "--restart=Never",
		"--image=busybox",
		"--context", kctx,
		"--overrides=" + overrides,
	}
}

// execKubectlNodeShell launches a privileged debug pod on the target node
// and attaches the user's terminal to a shell entered via nsenter.
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

	args := nodeShellArgs(podName, m.kubectlContext(ctx), overrides)
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

	return m.trackBgTask(bgtasks.KindSubprocess, "Explain: "+target, kctx, func() tea.Msg {
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

// execKubectlExplainRecursive runs kubectl explain --recursive and searches for matching fields.
func (m Model) execKubectlExplainRecursive(resource, apiVersion, query string) tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return explainRecursiveMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	kctx := m.nav.Context
	kubeconfigPaths := m.client.KubeconfigPathForContext(kctx)

	return m.trackBgTask(bgtasks.KindSubprocess, "Explain (recursive): "+resource, kctx, func() tea.Msg {
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

// --- Custom action commands ---

// execCustomAction runs a user-defined custom action command via sh -c.
// The command is executed with the terminal handed over via tea.ExecProcess,
// allowing interactive commands to work properly.
func (m Model) execCustomAction(expandedCmd string) tea.Cmd {
	ctx := m.actionCtx.context
	if ctx == "" {
		ctx = m.nav.Context
	}
	cmd := exec.Command("sh", "-c", expandedCmd)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPathForContext(ctx))
	logExecCmd("Running custom action", cmd)

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			logger.Error("Custom action failed", "cmd", cmd.String(), "error", err)
			return actionResultMsg{err: fmt.Errorf("custom action failed: %w", err)}
		}
		return actionResultMsg{message: "Custom action completed"}
	})
}
