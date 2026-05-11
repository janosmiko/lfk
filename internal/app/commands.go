package app

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// scheduleStatusClear returns a command that sends a clear message after a delay.
func scheduleStatusClear() tea.Cmd {
	return tea.Tick(5*time.Second, func(_ time.Time) tea.Msg {
		return statusMessageExpiredMsg{}
	})
}

// logExecCmd emits a structured Info entry for a subprocess invocation,
// recording the full command line plus the KUBECONFIG path extracted from
// cmd.Env so postmortems can tell which kubeconfig was active when the
// command ran. label distinguishes event kinds (e.g. "Running kubectl
// command", "Running helm command").
func logExecCmd(label string, cmd *exec.Cmd) {
	var kubeconfig string
	for _, e := range cmd.Env {
		if rest, ok := strings.CutPrefix(e, "KUBECONFIG="); ok {
			kubeconfig = rest
			break
		}
	}
	logger.Info(label, "cmd", cmd.String(), "kubeconfig", kubeconfig)
}

// startupTips is the list of tips shown randomly on startup.
var startupTips = []string{
	"Press ? to see all keybindings",
	"Press / to search, f to filter resources",
	"Press n/N to jump between search matches",
	"Press Space to select items, Ctrl+Space for range selection",
	"Press x to open the action menu for selected resources",
	"Press t to open a new tab, ] and [ to switch tabs",
	"Press \\ to change namespace, Shift+A to toggle all-namespaces",
	"Press F to toggle fullscreen mode",
	"Press L to view logs, x to open action menu",
	"Press I to explore any API resource with kubectl explain",
	"Press U to check RBAC permissions for a resource type",
	"Press @ to open the monitoring dashboard",
	"Press o to jump to the parent/owner of a resource",
	"Press m<key> to save a bookmark, ' to open bookmarks",
	"Press y to copy resource name, Y to copy YAML",
	"Press . for quick filter presets (failing pods, not-ready, etc.)",
	"Press T to preview different color themes, in preview mode press t to hide theme background",
	"Press e to edit resources with your $EDITOR",
	"Press v to describe a resource (like kubectl describe)",
	"Press p to pin/unpin CRD groups for quick access",
	"Press Shift+H to surface rarely used resource types (CSI, webhooks, leases, advanced core)",
	"In the secret/configmap/labels editor: Tab switches fields, Enter saves, Cmd+V pastes",
	"Use abbreviated search: type 'po' for Pods, 'deploy' for Deployments",
	"In log viewer: s for timestamps, c for previous terminated container, \\ to filter pods/containers",
	"Configure custom actions per resource type in ~/.config/lfk/config.yaml",
	"Press Ctrl+G to search and remove finalizers across resources",
	"Press , to show/hide and reorder columns in the resource list",
	"Press >/< to change sort column, = to reverse sort order, - to reset sorting",
	"Disable tips with 'tips: false' in ~/.config/lfk/config.yaml",
}

// scheduleStartupTip sends a random tip after a short delay to let the UI settle.
func scheduleStartupTip() tea.Cmd {
	tip := startupTips[rand.IntN(len(startupTips))]
	return tea.Tick(500*time.Millisecond, func(_ time.Time) tea.Msg {
		return startupTipMsg{tip: tip}
	})
}

// scheduleWatchTick returns a command that sends a watchTickMsg after the interval.
func scheduleWatchTick(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(_ time.Time) tea.Msg {
		return watchTickMsg{}
	})
}

const previewDebounceDelay = 300 * time.Millisecond

func schedulePreviewDebounce(gen uint64) tea.Cmd {
	return tea.Tick(previewDebounceDelay, func(_ time.Time) tea.Msg {
		return previewDebounceTickMsg{gen: gen}
	})
}

// scheduleDescribeRefresh returns a command that sends a describeRefreshTickMsg after 2 seconds.
func scheduleDescribeRefresh() tea.Cmd {
	return tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return describeRefreshTickMsg{}
	})
}

// openInBrowser opens the given URL in the user's default browser.
//
// Delegates to ui.OpenBrowser, which routes Windows through
// `rundll32 url.dll,FileProtocolHandler` instead of `cmd /c start`.
// The earlier `cmd /c start` path here re-parsed the URL through
// cmd.exe metacharacter semantics, so a single `&` in a query
// string (e.g. `?a=1&b=2`) would split the URL and execute the
// remainder as a shell command — both a correctness and a security
// issue for URLs derived from cluster data.
func openInBrowser(url string) tea.Cmd {
	return func() tea.Msg {
		if err := ui.OpenBrowser(url); err != nil {
			return actionResultMsg{err: fmt.Errorf("failed to open browser: %w", err)}
		}
		return actionResultMsg{message: "Opened " + url}
	}
}

// copyToSystemClipboard copies text to the system clipboard.
//
// Backed by atotto/clipboard so macOS (pbcopy), Linux X11 (xsel/xclip),
// Linux Wayland (wl-copy) and Windows (native syscall) all work without
// per-platform branches here.
//
// Returns nil on success: every caller already calls setStatusMessage with a
// context-specific message (e.g. "Copied 1 line", "Copied value of <key>")
// before dispatching this command. Returning a generic "Copied to clipboard"
// here would race back through updateActionResult and overwrite the more
// useful caller message — visible to the user as a flicker.
func copyToSystemClipboard(text string) tea.Cmd {
	return func() tea.Msg {
		if err := clipboard.WriteAll(text); err != nil {
			return actionResultMsg{err: fmt.Errorf("clipboard: %w", err)}
		}
		return nil
	}
}

// loadPodsForAction fetches pods owned by the action target resource (for exec/attach on parent resources).
func (m Model) loadPodsForAction() tea.Cmd {
	kctx := m.actionCtx.context
	ns := m.actionNamespace()
	kind := m.actionCtx.kind
	name := m.actionCtx.name
	return func() tea.Msg {
		items, err := m.client.GetOwnedResources(context.Background(), kctx, ns, kind, name)
		return podSelectMsg{items: items, err: err}
	}
}

// loadPodsForLogAction fetches pods matching the parent resource's selector using kubectl.
// Uses kubectl instead of the Go client to avoid separate OIDC auth flows.
func (m Model) loadPodsForLogAction() tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return func() tea.Msg {
			return podLogSelectMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	ns := m.actionNamespace()
	kind := m.actionCtx.kind
	name := m.actionCtx.name
	kctx := m.actionCtx.context
	kubeconfigPaths := m.client.KubeconfigPathForContext(kctx)

	return func() tea.Msg {
		// Get the selector for this parent resource.
		selector := kubectlGetPodSelector(kubectlPath, kubeconfigPaths, ns, kind, name, m.kubectlContext(kctx))
		if selector == "" {
			return podLogSelectMsg{err: fmt.Errorf("could not determine pod selector for %s/%s", kind, name)}
		}

		// Fetch pods matching the selector.
		args := []string{"get", "pods", "-l", selector, "-n", ns, "--context", m.kubectlContext(kctx), "-o", "json"}
		cmd := exec.Command(kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
		logExecCmd("Running kubectl command", cmd)
		out, err := cmd.Output()
		if err != nil {
			logger.Error("kubectl get pods failed", "cmd", cmd.String(), "error", err)
			return podLogSelectMsg{err: fmt.Errorf("failed to list pods: %w", err)}
		}

		var podList struct {
			Items []struct {
				Metadata struct {
					Name      string `json:"name"`
					Namespace string `json:"namespace"`
				} `json:"metadata"`
				Status struct {
					Phase string `json:"phase"`
				} `json:"status"`
			} `json:"items"`
		}
		if err := json.Unmarshal(out, &podList); err != nil {
			return podLogSelectMsg{err: fmt.Errorf("failed to parse pod list: %w", err)}
		}

		var items []model.Item
		for _, pod := range podList.Items {
			items = append(items, model.Item{
				Name:      pod.Metadata.Name,
				Namespace: pod.Metadata.Namespace,
				Kind:      "Pod",
				Status:    pod.Status.Phase,
			})
		}
		return podLogSelectMsg{items: items}
	}
}

// loadContainersForAction fetches the container list for the action target pod.
func (m Model) loadContainersForAction() tea.Cmd {
	kctx := m.actionCtx.context
	ns := m.actionNamespace()
	podName := m.actionCtx.name
	return func() tea.Msg {
		items, err := m.client.GetContainers(context.Background(), kctx, ns, podName)
		// Reverse order for the selector: regular containers first (reversed),
		// then init/sidecar containers (reversed), so the most relevant
		// container is at the top.
		for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
			items[i], items[j] = items[j], items[i]
		}
		return containerSelectMsg{items: items, err: err}
	}
}

// loadContainersForLogFilter fetches the container list for the current pod in the log viewer.
// Returns a logContainersLoadedMsg with container names for the filter overlay.
func (m Model) loadContainersForLogFilter() tea.Cmd {
	kctx := m.actionCtx.context
	ns := m.actionNamespace()
	podName := m.actionCtx.name
	return func() tea.Msg {
		items, err := m.client.GetContainers(context.Background(), kctx, ns, podName)
		if err != nil {
			return logContainersLoadedMsg{err: err}
		}
		var names []string
		for _, item := range items {
			names = append(names, item.Name)
		}
		return logContainersLoadedMsg{containers: names}
	}
}

// clearBeforeExec wraps cmd to clear the terminal screen before running it.
// This ensures the TUI artifacts are removed when switching to interactive mode.
func clearBeforeExec(cmd *exec.Cmd) *exec.Cmd {
	return clearBeforeExecForOS(cmd, runtime.GOOS)
}

// clearBeforeExecForOS is the testable inner form. On Windows the sh -c
// wrap is skipped because sh.exe is not on a standard PATH there —
// wrapping kubectl in `sh -c "printf '\033c' && exec kubectl …"` would
// make the parent process fail to start (issue #194: "exit status 2",
// terminal flashes and closes). The cost is one cosmetic clear-screen
// on the way into an interactive shell on Windows.
func clearBeforeExecForOS(cmd *exec.Cmd, goos string) *exec.Cmd {
	if goos == "windows" {
		return cmd
	}
	// Build a shell command: clear screen with ANSI reset, then exec the original command.
	quoted := make([]string, 0, len(cmd.Args))
	for _, arg := range cmd.Args {
		quoted = append(quoted, shellQuote(arg))
	}
	shellCmd := fmt.Sprintf(`printf '\033c' && exec %s`, strings.Join(quoted, " "))
	wrapped := exec.Command("sh", "-c", shellCmd)
	wrapped.Env = cmd.Env
	wrapped.Dir = cmd.Dir
	wrapped.Stdin = cmd.Stdin
	wrapped.Stdout = cmd.Stdout
	wrapped.Stderr = cmd.Stderr
	return wrapped
}

// shellQuote quotes a string for safe use in a shell command.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// findCustomAction looks up a custom action by kind and label from the user config.
func findCustomAction(kind, label string) (ui.CustomAction, bool) {
	actions, ok := ui.ConfigCustomActions[kind]
	if !ok {
		return ui.CustomAction{}, false
	}
	for _, ca := range actions {
		if ca.Label == label {
			return ca, true
		}
	}
	return ui.CustomAction{}, false
}

// expandCustomActionTemplate substitutes template variables in a custom action command string.
// Supported variables: {name}, {namespace}, {context}, {kind}, and any column key
// from the resource item (e.g., {nodeName}, {IP}) with the key stripped of spaces and
// lowercased for matching.
func expandCustomActionTemplate(cmdTemplate string, actx actionContext) string {
	result := cmdTemplate
	result = strings.ReplaceAll(result, "{name}", actx.name)
	result = strings.ReplaceAll(result, "{namespace}", actx.namespace)
	result = strings.ReplaceAll(result, "{context}", actx.context)
	result = strings.ReplaceAll(result, "{kind}", actx.kind)

	// Substitute column-based variables. The user writes {columnKey} where columnKey
	// matches the column's Key field (case-insensitive, spaces removed). For example,
	// a column with Key="Node" can be referenced as {Node} or {node}.
	for _, kv := range actx.columns {
		// Exact match first (e.g., {Node} for Key="Node").
		result = strings.ReplaceAll(result, "{"+kv.Key+"}", kv.Value)
		// Also support camelCase-style references (e.g., {nodeName} for Key="Node").
		lowerKey := strings.ToLower(strings.ReplaceAll(kv.Key, " ", ""))
		if lowerKey != kv.Key {
			result = strings.ReplaceAll(result, "{"+lowerKey+"}", kv.Value)
		}
	}

	return result
}
