package app

import (
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

func randomSuffix(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[rand.IntN(len(chars))]
	}
	return string(b)
}

func cleanANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && (s[j] < 'A' || s[j] > 'Z') && (s[j] < 'a' || s[j] > 'z') {
				j++
			}
			if j < len(s) {
				j++
			}
			i = j
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
