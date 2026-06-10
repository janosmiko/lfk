package app

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/logger"
)

// runHelmCmdResult runs a prepared helm command to completion and maps the
// outcome to an actionResultMsg. Used for the non-interactive helm operations
// (uninstall, upgrade), which must NOT take over the terminal via
// tea.ExecProcess — that is reserved for editHelmValues, which spawns the
// user's $EDITOR. Output is captured so a failure surfaces in the status bar
// instead of a suspended-TUI screen.
func runHelmCmdResult(cmd *exec.Cmd, successMsg, failPrefix string) tea.Msg {
	logExecCmd("Running helm command", cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("helm command failed", "cmd", cmd.String(), "error", err, "output", string(out))
		return actionResultMsg{err: fmt.Errorf("%s: %w: %s", failPrefix, err, strings.TrimSpace(string(out)))}
	}
	return actionResultMsg{message: successMsg}
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
	kubeconfigPaths := m.client.KubeconfigPathForContext(ctx)
	args := []string{"uninstall", name, "-n", ns, "--kube-context", m.kubectlContext(ctx)}

	cmd := exec.Command(helmPath, args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
	return m.trackBgTask(scheduler.KindSubprocess, "Helm uninstall: "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		return runHelmCmdResult(cmd, fmt.Sprintf("Uninstalled %s", name), "helm uninstall")
	})
}

// helmExecEnv hands the helm path, release, namespace, context, and kubeconfig
// to the sh -c scripts as environment variables. These values (notably the
// kubeconfig context name) are user/CI-controlled and may contain shell
// metacharacters; passing them via the environment instead of interpolating
// into the script text means the shell never parses them as code.
func helmExecEnv(helmPath, release, ns, ctx, kubeconfig string) []string {
	return append(os.Environ(),
		"HELM="+helmPath,
		"RELEASE="+release,
		"NS="+ns,
		"CTX="+ctx,
		"KUBECONFIG="+kubeconfig,
	)
}

// helmEditScript drives "edit helm values": fetch values into a temp file, open
// the user's editor, and re-apply on change. All inputs arrive via the
// environment ($HELM/$RELEASE/$NS/$CTX/$KUBECONFIG) — no value is interpolated.
const helmEditScript = `
set -e

TMPFILE=$(mktemp "${TMPDIR:-/tmp}/helm-values-XXXXXX.yaml")

$HELM get values "$RELEASE" -n "$NS" --kube-context "$CTX" -o yaml > "$TMPFILE" 2>&1
# Replace bare 'null' with a helpful comment
if [ "$(cat "$TMPFILE" | tr -d '[:space:]')" = "null" ]; then
  echo "# Add your values here" > "$TMPFILE"
fi

# Save checksum before editing
BEFORE=$(md5sum "$TMPFILE" 2>/dev/null || md5 -q "$TMPFILE" 2>/dev/null || cat "$TMPFILE")

${KUBE_EDITOR:-${EDITOR:-${VISUAL:-vi}}} "$TMPFILE"

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
`

// helmUpgradeScript drives "helm upgrade --reuse-values" after resolving the
// chart name. All inputs arrive via the environment — no value is interpolated.
const helmUpgradeScript = `
set -e

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
`

func buildHelmEditCmd(helmPath, release, ns, ctx, kubeconfig string) *exec.Cmd {
	cmd := exec.Command("sh", "-c", helmEditScript)
	cmd.Env = helmExecEnv(helmPath, release, ns, ctx, kubeconfig)
	return cmd
}

func buildHelmUpgradeCmd(helmPath, release, ns, ctx, kubeconfig string) *exec.Cmd {
	cmd := exec.Command("sh", "-c", helmUpgradeScript)
	cmd.Env = helmExecEnv(helmPath, release, ns, ctx, kubeconfig)
	return cmd
}

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
	helmCtx := m.kubectlContext(ctx)

	cmd := buildHelmEditCmd(helmPath, name, ns, helmCtx, kubeconfigPaths)
	logger.Info("Running helm edit values", "release", name, "namespace", ns, "context", ctx)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			logger.Error("helm edit values failed", "release", name, "error", err)
			return actionResultMsg{err: fmt.Errorf("helm edit values: %w", err)}
		}
		return actionResultMsg{message: fmt.Sprintf("Values updated for %s", name)}
	})
}

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

	return m.trackBgTask(scheduler.KindSubprocess, "Helm diff: "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		env := append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)

		chartName := resolveHelmChartName(helmPath, name, ns, m.kubectlContext(ctx), kubeconfigPaths)

		defaultOut, leftLabel := helmShowDefaultValues(helmPath, chartName, env)

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

	parts := strings.Split(chartVersion, "-")
	if last := len(parts) - 1; last > 0 && len(parts[last]) > 0 && parts[last][0] >= '0' && parts[last][0] <= '9' {
		return strings.Join(parts[:last], "-")
	}
	return chartVersion
}

func helmShowDefaultValues(helmPath, chartName string, env []string) (string, string) {
	if chartName == "" {
		return "", ""
	}

	searchArgs := []string{"search", "repo", chartName, "-o", "json"}
	searchCmd := exec.Command(helmPath, searchArgs...)
	searchCmd.Env = env
	logExecCmd("Running helm command", searchCmd)
	searchOut, searchErr := searchCmd.CombinedOutput()
	if searchErr != nil {
		return "", ""
	}

	repoChart := parseFirstJSONField(string(searchOut), "name", chartName)
	if repoChart == "" {
		return "", ""
	}

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
		if value == suffix || strings.HasSuffix(value, "/"+suffix) {
			return value
		}
		rest = remaining
	}
}

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

	cmd := buildHelmUpgradeCmd(helmPath, name, ns, m.kubectlContext(ctx), kubeconfigPaths)
	logger.Info("Running helm upgrade", "release", name, "namespace", ns, "context", ctx)
	return m.trackBgTask(scheduler.KindSubprocess, "Helm upgrade: "+name, bgtaskTarget(ctx, ns), func() tea.Msg {
		return runHelmCmdResult(cmd, fmt.Sprintf("Upgraded %s", name), "helm upgrade")
	})
}

func (m Model) rollbackHelmRelease(revision int) tea.Cmd {
	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return func() tea.Msg {
			return helmRollbackDoneMsg{err: fmt.Errorf("helm not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	name := m.actionCtx.name
	ctx := m.actionCtx.context
	kubeconfigPaths := m.client.KubeconfigPathForContext(ctx)

	return m.trackBgTask(scheduler.KindSubprocess, fmt.Sprintf("Helm rollback: %s@%d", name, revision), bgtaskTarget(ctx, ns), func() tea.Msg {
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
