package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

func (m Model) applyFromClipboard() tea.Cmd {
	ctx := m.nav.Context
	ns := m.effectiveNamespace()
	if ns == "" {
		ns = "default"
	}

	// Read from clipboard. atotto/clipboard covers macOS, Linux (X11 + Wayland)
	// and Windows uniformly.
	clipContent, err := clipboard.ReadAll()
	if err != nil {
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("reading clipboard: %w", err)}
		}
	}

	if len(strings.TrimSpace(clipContent)) == 0 {
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
	if _, err := tmpFile.WriteString(clipContent); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("writing temp file: %w", err)}
		}
	}
	_ = tmpFile.Close()

	// Open in editor for review/editing before applying.
	editor := editorCommand()
	tmpPath := tmpFile.Name()

	// Record modification time before opening editor.
	origModTime := time.Time{}
	if fi, err := os.Stat(tmpPath); err == nil {
		origModTime = fi.ModTime()
	}

	cmd := exec.Command(editor[0], append(editor[1:], tmpPath)...)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			_ = os.Remove(tmpPath)
			return actionResultMsg{err: fmt.Errorf("editor: %w", err)}
		}
		return templateApplyMsg{tmpFile: tmpPath, context: ctx, ns: ns, origModTime: origModTime}
	})
}

// applyTemplate creates a temp file with the template YAML, opens it in
// $KUBE_EDITOR or $EDITOR, then applies it with kubectl after the editor exits.
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
	editor := editorCommand()

	tmpPath := tmpFile.Name()

	// Record modification time before opening editor.
	origModTime := time.Time{}
	if fi, err := os.Stat(tmpPath); err == nil {
		origModTime = fi.ModTime()
	}

	cmd := exec.Command(editor[0], append(editor[1:], tmpPath)...)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			_ = os.Remove(tmpPath)
			return actionResultMsg{err: fmt.Errorf("editor: %w", err)}
		}
		return templateApplyMsg{tmpFile: tmpPath, context: ctx, ns: ns, origModTime: origModTime}
	})
}

// applyTemplateFile runs kubectl apply -f on the given temp file and cleans it up.
// In demo mode it instead routes through Client.ApplyManifest: the demo
// backend's fake dynamic client lives only in this process, so a kubectl
// subprocess would write into a separate tracker the UI never reads from —
// see internal/k8s.Client.ApplyManifest.
func (m Model) applyTemplateFile(tmpFile, ctx, ns string) tea.Cmd {
	if m.client.IsDemo() {
		return m.applyTemplateFileDemo(tmpFile, ctx, ns)
	}

	kubectlPath, err := k8s.KubectlPath()
	if err != nil {
		_ = os.Remove(tmpFile)
		return func() tea.Msg {
			return actionResultMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
	}

	return func() tea.Msg {
		defer func() { _ = os.Remove(tmpFile) }()

		args := []string{"apply", "-f", tmpFile, "--context", m.kubectlContext(ctx)}
		if ns != "" {
			args = append(args, "-n", ns)
		}
		cmd := exec.Command(kubectlPath, args...)
		cmd.Env = append(os.Environ(), "KUBECONFIG="+m.client.KubeconfigPathForContext(ctx))
		logExecCmd("Running kubectl command", cmd)
		output, err := cmd.CombinedOutput()
		if err != nil {
			logger.Error("kubectl apply failed", "cmd", cmd.String(), "error", err, "output", string(output))
			return actionResultMsg{err: fmt.Errorf("kubectl apply: %s", strings.TrimSpace(string(output)))}
		}
		// Templates are rare and may create a Namespace (either directly
		// via the "Namespace" template or an edited YAML that adds one),
		// so always invalidate on success. Worst case is one extra
		// GetNamespaces round-trip per template apply.
		return actionResultMsg{
			message:                  strings.TrimSpace(string(output)),
			invalidateNamespaceCache: true,
		}
	}
}

// applyTemplateFileDemo applies the temp file's manifest in-process through
// Client.ApplyManifest and cleans it up, mirroring applyTemplateFile's
// kubectl path without shelling out.
func (m Model) applyTemplateFileDemo(tmpFile, ctx, ns string) tea.Cmd {
	return func() tea.Msg {
		defer func() { _ = os.Remove(tmpFile) }()

		content, err := os.ReadFile(tmpFile)
		if err != nil {
			return actionResultMsg{err: fmt.Errorf("reading manifest: %w", err)}
		}
		if err := m.client.ApplyManifest(context.Background(), ctx, ns, string(content)); err != nil {
			return actionResultMsg{err: fmt.Errorf("apply: %w", err)}
		}
		return actionResultMsg{
			message:                  "applied",
			invalidateNamespaceCache: true,
		}
	}
}
