package app

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// scheduleStatusClear returns a command that sends a clear message after a delay.
func scheduleStatusClear() tea.Cmd {
	return tea.Tick(5*time.Second, func(_ time.Time) tea.Msg {
		return statusMessageExpiredMsg{}
	})
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
	"Press # to browse security findings by source",
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

// scheduleDescribeRefresh returns a command that sends a describeRefreshTickMsg after 2 seconds.
func scheduleDescribeRefresh() tea.Cmd {
	return tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return describeRefreshTickMsg{}
	})
}

// openInBrowser opens the given URL in the user's default browser using
// platform-specific commands (open on macOS, xdg-open on Linux, start on Windows).
func openInBrowser(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "linux":
			cmd = exec.Command("xdg-open", url)
		case "windows":
			cmd = exec.Command("cmd", "/c", "start", url)
		default:
			return actionResultMsg{err: fmt.Errorf("browser open not supported on %s", runtime.GOOS)}
		}
		if err := cmd.Start(); err != nil {
			return actionResultMsg{err: fmt.Errorf("failed to open browser: %w", err)}
		}
		return actionResultMsg{message: "Opened " + url}
	}
}

// copyToSystemClipboard copies text to the system clipboard using platform-specific tools.
func copyToSystemClipboard(text string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("pbcopy")
		case "linux":
			cmd = exec.Command("xclip", "-selection", "clipboard")
		default:
			return actionResultMsg{err: fmt.Errorf("clipboard not supported on %s", runtime.GOOS)}
		}
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err != nil {
			return actionResultMsg{err: fmt.Errorf("clipboard: %w", err)}
		}
		return actionResultMsg{message: "Copied to clipboard"}
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
	kubeconfigPaths := m.client.KubeconfigPaths()

	return func() tea.Msg {
		// Get the selector for this parent resource.
		selector := kubectlGetPodSelector(kubectlPath, kubeconfigPaths, ns, kind, name, kctx)
		if selector == "" {
			return podLogSelectMsg{err: fmt.Errorf("could not determine pod selector for %s/%s", kind, name)}
		}

		// Fetch pods matching the selector.
		args := []string{"get", "pods", "-l", selector, "-n", ns, "--context", kctx, "-o", "json"}
		cmd := exec.Command(kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPaths)
		logger.Info("Running kubectl command", "cmd", cmd.String())
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

func (m Model) bulkDeleteResources() tea.Cmd {
	items := m.bulkItems
	ctx := m.actionCtx.context
	rt := m.actionCtx.resourceType
	client := m.client
	ns := m.actionNamespace()

	return func() tea.Msg {
		var succeeded, failed int
		var errors []string
		for _, item := range items {
			itemNs := ns
			if item.Namespace != "" {
				itemNs = item.Namespace
			}
			logger.Info("Bulk deleting", "resource", rt.Resource, "name", item.Name, "namespace", itemNs)
			err := client.DeleteResource(ctx, itemNs, rt, item.Name)
			if err != nil {
				failed++
				errors = append(errors, fmt.Sprintf("%s: %s", item.Name, err.Error()))
			} else {
				succeeded++
			}
		}
		return bulkActionResultMsg{succeeded: succeeded, failed: failed, errors: errors}
	}
}

func (m Model) bulkForceDeleteResources() tea.Cmd {
	items := m.bulkItems
	ctx := m.actionCtx.context
	rt := m.actionCtx.resourceType
	client := m.client
	ns := m.actionNamespace()

	return func() tea.Msg {
		kubectlPath, err := exec.LookPath("kubectl")
		if err != nil {
			return bulkActionResultMsg{failed: len(items), errors: []string{"kubectl not found"}}
		}

		var succeeded, failed int
		var errors []string
		for _, item := range items {
			itemNs := ns
			if item.Namespace != "" {
				itemNs = item.Namespace
			}
			logger.Info("Bulk force deleting", "resource", rt.Resource, "name", item.Name, "namespace", itemNs)

			// Remove finalizers first.
			patchArgs := []string{
				"patch", rt.Resource, item.Name, "--context", ctx,
				"--type", "merge", "-p", `{"metadata":{"finalizers":null}}`,
			}
			if rt.Namespaced {
				patchArgs = append(patchArgs, "-n", itemNs)
			}
			patchCmd := exec.Command(kubectlPath, patchArgs...)
			patchCmd.Env = append(os.Environ(), "KUBECONFIG="+client.KubeconfigPaths())
			logger.Info("Running kubectl command", "cmd", patchCmd.String())
			patchCmd.Run() //nolint:errcheck

			// Force delete.
			deleteArgs := []string{
				"delete", rt.Resource, item.Name, "--context", ctx,
				"--grace-period=0", "--force",
			}
			if rt.Namespaced {
				deleteArgs = append(deleteArgs, "-n", itemNs)
			}
			cmd := exec.Command(kubectlPath, deleteArgs...)
			cmd.Env = append(os.Environ(), "KUBECONFIG="+client.KubeconfigPaths())
			logger.Info("Running kubectl command", "cmd", cmd.String())
			if output, err := cmd.CombinedOutput(); err != nil {
				logger.Error("kubectl bulk force delete failed", "cmd", cmd.String(), "error", err, "output", string(output))
				failed++
				errors = append(errors, fmt.Sprintf("%s: %s", item.Name, strings.TrimSpace(string(output))))
			} else {
				succeeded++
			}
		}
		return bulkActionResultMsg{succeeded: succeeded, failed: failed, errors: errors}
	}
}

func (m Model) bulkScaleResources(replicas int32) tea.Cmd {
	items := m.bulkItems
	ctx := m.actionCtx.context
	client := m.client
	ns := m.actionNamespace()

	return func() tea.Msg {
		var succeeded, failed int
		var errors []string
		for _, item := range items {
			itemNs := ns
			if item.Namespace != "" {
				itemNs = item.Namespace
			}
			logger.Info("Bulk scaling", "name", item.Name, "replicas", replicas, "namespace", itemNs)
			err := client.ScaleResource(ctx, itemNs, item.Name, m.actionCtx.kind, replicas)
			if err != nil {
				failed++
				errors = append(errors, fmt.Sprintf("%s: %s", item.Name, err.Error()))
			} else {
				succeeded++
			}
		}
		return bulkActionResultMsg{succeeded: succeeded, failed: failed, errors: errors}
	}
}

func (m Model) bulkRestartResources() tea.Cmd {
	items := m.bulkItems
	ctx := m.actionCtx.context
	client := m.client
	ns := m.actionNamespace()

	return func() tea.Msg {
		var succeeded, failed int
		var errors []string
		for _, item := range items {
			itemNs := ns
			if item.Namespace != "" {
				itemNs = item.Namespace
			}
			logger.Info("Bulk restarting", "name", item.Name, "namespace", itemNs)
			err := client.RestartResource(ctx, itemNs, item.Name, m.actionCtx.kind)
			if err != nil {
				failed++
				errors = append(errors, fmt.Sprintf("%s: %s", item.Name, err.Error()))
			} else {
				succeeded++
			}
		}
		return bulkActionResultMsg{succeeded: succeeded, failed: failed, errors: errors}
	}
}

func (m Model) batchPatchLabels(key, value string, remove bool, isAnnotation bool) tea.Cmd {
	items := m.bulkItems
	ctx := m.actionCtx.context
	rt := m.actionCtx.resourceType
	gvr := schema.GroupVersionResource{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
	}
	client := m.client
	ns := m.namespace

	return func() tea.Msg {
		var success, failed int
		for _, item := range items {
			var patch map[string]interface{}
			if remove {
				patch = map[string]interface{}{key: nil}
			} else {
				patch = map[string]interface{}{key: value}
			}
			itemNs := item.Namespace
			if itemNs == "" {
				itemNs = ns
			}
			var err error
			if isAnnotation {
				err = client.PatchAnnotations(context.Background(), ctx, itemNs, item.Name, gvr, patch)
			} else {
				err = client.PatchLabels(context.Background(), ctx, itemNs, item.Name, gvr, patch)
			}
			if err != nil {
				failed++
			} else {
				success++
			}
		}
		return bulkActionResultMsg{succeeded: success, failed: failed}
	}
}

func (m Model) applyFromClipboard() tea.Cmd {
	ctx := m.nav.Context
	ns := m.effectiveNamespace()
	if ns == "" {
		ns = "default"
	}

	// Read from clipboard.
	var clipCmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		clipCmd = exec.Command("pbpaste")
	case "linux":
		clipCmd = exec.Command("xclip", "-selection", "clipboard", "-o")
	default:
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("clipboard not supported on %s", runtime.GOOS)}
		}
	}

	clipContent, err := clipCmd.Output()
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("reading clipboard: %w", err)}
		}
	}

	if len(strings.TrimSpace(string(clipContent))) == 0 {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("clipboard is empty")}
		}
	}

	// Write to temp file for editor review.
	tmpFile, err := os.CreateTemp("", "k-paste-*.yaml")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("creating temp file: %w", err)}
		}
	}
	if _, err := tmpFile.Write(clipContent); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("writing temp file: %w", err)}
		}
	}
	_ = tmpFile.Close()

	// Open in editor for review/editing before applying.
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	tmpPath := tmpFile.Name()

	// Record modification time before opening editor.
	origModTime := time.Time{}
	if fi, err := os.Stat(tmpPath); err == nil {
		origModTime = fi.ModTime()
	}

	cmd := exec.Command(editor, tmpPath)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			_ = os.Remove(tmpPath)
			return actionResultMsg{err: fmt.Errorf("editor: %w", err)}
		}
		return templateApplyMsg{tmpFile: tmpPath, context: ctx, ns: ns, origModTime: origModTime}
	})
}

// applyTemplate creates a temp file with the template YAML, opens it in $EDITOR,
// then applies it with kubectl after the editor exits.
func (m Model) applyTemplate(tmpl model.ResourceTemplate) tea.Cmd {
	ns := m.effectiveNamespace()
	if ns == "" {
		ns = "default"
	}
	ctx := m.nav.Context

	// Replace NAMESPACE placeholder.
	yamlContent := strings.ReplaceAll(tmpl.YAML, "NAMESPACE", ns)

	// Write to temp file.
	tmpFile, err := os.CreateTemp("", "k-template-*.yaml")
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("creating temp file: %w", err)}
		}
	}
	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("writing temp file: %w", err)}
		}
	}
	_ = tmpFile.Close()

	// Determine editor.
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	tmpPath := tmpFile.Name()

	// Record modification time before opening editor.
	origModTime := time.Time{}
	if fi, err := os.Stat(tmpPath); err == nil {
		origModTime = fi.ModTime()
	}

	cmd := exec.Command(editor, tmpPath)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			_ = os.Remove(tmpPath)
			return actionResultMsg{err: fmt.Errorf("editor: %w", err)}
		}
		return templateApplyMsg{tmpFile: tmpPath, context: ctx, ns: ns, origModTime: origModTime}
	})
}

// applyTemplateFile runs kubectl apply -f on the given temp file and cleans it up.
func (m Model) applyTemplateFile(tmpFile, ctx, ns string) tea.Cmd {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		_ = os.Remove(tmpFile)
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	return func() tea.Msg {
		defer func() { _ = os.Remove(tmpFile) }()

		args := []string{"apply", "-f", tmpFile, "--context", ctx}
		if ns != "" {
			args = append(args, "-n", ns)
		}
		cmd := exec.Command(kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPaths())
		logger.Info("Running kubectl command", "cmd", cmd.String())
		output, err := cmd.CombinedOutput()
		if err != nil {
			logger.Error("kubectl apply failed", "cmd", cmd.String(), "error", err, "output", string(output))
			return actionResultMsg{err: fmt.Errorf("kubectl apply: %s", strings.TrimSpace(string(output)))}
		}
		return actionResultMsg{message: strings.TrimSpace(string(output))}
	}
}

// copyYAMLToClipboard fetches the YAML for the selected resource and sends it for clipboard copy.
func (m Model) copyYAMLToClipboard() tea.Cmd {
	kctx := m.nav.Context
	ns := m.resolveNamespace()

	switch m.nav.Level {
	case model.LevelResources:
		sel := m.selectedMiddleItem()
		if sel == nil {
			return nil
		}
		rt := m.nav.ResourceType
		name := sel.Name
		itemNs := ns
		if sel.Namespace != "" {
			itemNs = sel.Namespace
		}
		return func() tea.Msg {
			content, err := m.client.GetResourceYAML(context.Background(), kctx, itemNs, rt, name)
			return yamlClipboardMsg{content: content, err: err}
		}
	case model.LevelOwned:
		sel := m.selectedMiddleItem()
		if sel == nil {
			return nil
		}
		name := sel.Name
		itemNs := ns
		if sel.Namespace != "" {
			itemNs = sel.Namespace
		}
		if sel.Kind == "Pod" {
			return func() tea.Msg {
				content, err := m.client.GetPodYAML(context.Background(), kctx, itemNs, name)
				return yamlClipboardMsg{content: content, err: err}
			}
		}
		rt, ok := m.resolveOwnedResourceType(sel)
		if !ok {
			return func() tea.Msg {
				return yamlClipboardMsg{err: fmt.Errorf("unknown resource type: %s", sel.Kind)}
			}
		}
		return func() tea.Msg {
			content, err := m.client.GetResourceYAML(context.Background(), kctx, itemNs, rt, name)
			return yamlClipboardMsg{content: content, err: err}
		}
	case model.LevelContainers:
		podName := m.nav.OwnedName
		return func() tea.Msg {
			content, err := m.client.GetPodYAML(context.Background(), kctx, ns, podName)
			return yamlClipboardMsg{content: content, err: err}
		}
	}
	return nil
}

// exportResourceToFile saves the selected resource YAML to a file.
func (m Model) exportResourceToFile() tea.Cmd {
	kctx := m.nav.Context
	ns := m.resolveNamespace()

	var fetchYAML func() (string, string, error) // returns (yaml, kindForFilename, error)

	switch m.nav.Level {
	case model.LevelResources:
		sel := m.selectedMiddleItem()
		if sel == nil {
			return nil
		}
		rt := m.nav.ResourceType
		name := sel.Name
		itemNs := ns
		if sel.Namespace != "" {
			itemNs = sel.Namespace
		}
		kind := strings.ToLower(rt.Kind)
		fetchYAML = func() (string, string, error) {
			content, err := m.client.GetResourceYAML(context.Background(), kctx, itemNs, rt, name)
			return content, kind, err
		}
	case model.LevelOwned:
		sel := m.selectedMiddleItem()
		if sel == nil {
			return nil
		}
		name := sel.Name
		itemNs := ns
		if sel.Namespace != "" {
			itemNs = sel.Namespace
		}
		if sel.Kind == "Pod" {
			fetchYAML = func() (string, string, error) {
				content, err := m.client.GetPodYAML(context.Background(), kctx, itemNs, name)
				return content, "pod", err
			}
		} else {
			rt, ok := m.resolveOwnedResourceType(sel)
			if !ok {
				return func() tea.Msg {
					return exportDoneMsg{err: fmt.Errorf("unknown resource type: %s", sel.Kind)}
				}
			}
			kind := strings.ToLower(rt.Kind)
			fetchYAML = func() (string, string, error) {
				content, err := m.client.GetResourceYAML(context.Background(), kctx, itemNs, rt, name)
				return content, kind, err
			}
		}
	case model.LevelContainers:
		podName := m.nav.OwnedName
		fetchYAML = func() (string, string, error) {
			content, err := m.client.GetPodYAML(context.Background(), kctx, ns, podName)
			return content, "pod", err
		}
	default:
		return nil
	}

	return func() tea.Msg {
		yaml, kind, err := fetchYAML()
		if err != nil {
			return exportDoneMsg{err: fmt.Errorf("fetching resource: %w", err)}
		}

		// Build filename: <kind>_<name>.yaml
		var name string
		switch m.nav.Level {
		case model.LevelContainers:
			name = m.nav.OwnedName
		default:
			sel := m.selectedMiddleItem()
			if sel != nil {
				name = sel.Name
			}
		}
		sanitized := strings.ReplaceAll(name, "/", "_")
		filename := fmt.Sprintf("%s_%s.yaml", kind, sanitized)

		if err := os.WriteFile(filename, []byte(yaml), 0o644); err != nil {
			return exportDoneMsg{err: fmt.Errorf("writing file: %w", err)}
		}

		abs, _ := filepath.Abs(filename)
		if abs == "" {
			abs = filename
		}
		return exportDoneMsg{path: abs}
	}
}

// clearBeforeExec wraps cmd to clear the terminal screen before running it.
// This ensures the TUI artifacts are removed when switching to interactive mode.
func clearBeforeExec(cmd *exec.Cmd) *exec.Cmd {
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
