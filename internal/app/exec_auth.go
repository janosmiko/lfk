package app

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

const execAuthMaxAttempts = 3

type execAuthDoneMsg struct {
	context string
	err     error
}

// maybeExecAuth suspends lfk once per context so an exec credential plugin
// can prompt on the real terminal. Returns nil when the error is not a
// plugin exit failure, the context is not the current one (hovering a
// context also loads), or the context was already tried.
func (m *Model) maybeExecAuth(ctx string, err error) tea.Cmd {
	if !k8s.IsExecPluginExitError(err) || ctx == "" || ctx == UnionContextSentinel ||
		ctx != m.nav.Context || m.nav.Level == model.LevelClusters ||
		m.demoMode || m.execAuthTried[ctx] {
		return nil
	}
	if m.execAuthTried == nil {
		m.execAuthTried = make(map[string]bool)
	}
	m.execAuthTried[ctx] = true
	cmd := &execAuthCmd{
		context:        ctx,
		kubectlContext: m.kubectlContext(ctx),
		kubeconfig:     m.client.KubeconfigPathForContext(ctx),
	}
	return tea.Exec(cmd, func(err error) tea.Msg {
		return execAuthDoneMsg{context: ctx, err: err}
	})
}

func (m Model) updateExecAuthDone(msg execAuthDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Auth failed: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.setStatusMessage("Authenticated to "+ui.SanitizeTerminalText(msg.context), false)
	cmds := []tea.Cmd{scheduleStatusClear()}
	if msg.context != m.nav.Context {
		return m, cmds[0]
	}
	// The guard stays set: a plugin that caches no token would loop otherwise.
	mdl, cmd := m.directActionRefresh()
	m = mdl.(Model)
	cmds = append(cmds, cmd)
	if m.shouldFireDiscoveryFor(msg.context) {
		m.markDiscoveryStarted(msg.context)
		cmds = append(cmds, m.discoverAPIResources(msg.context))
	}
	return m, tea.Batch(cmds...)
}

// execAuthCmd runs kubectl, not in-process client-go, because client-go
// caches os.Stderr per authenticator and that is a capture pipe here.
type execAuthCmd struct {
	context        string
	kubectlContext string
	kubeconfig     string
	stdin          io.Reader
	stdout         io.Writer
	stderr         io.Writer
}

func (c *execAuthCmd) SetStdin(r io.Reader)  { c.stdin = r }
func (c *execAuthCmd) SetStdout(w io.Writer) { c.stdout = w }
func (c *execAuthCmd) SetStderr(w io.Writer) { c.stderr = w }

func (c *execAuthCmd) Run() error {
	_, _ = fmt.Fprintf(c.stdout, "lfk: %s auth plugin needs input\n", ui.SanitizeTerminalText(c.context))
	bin, err := k8s.KubectlPath()
	if err != nil {
		return c.fail(fmt.Errorf("kubectl not found: %w", err))
	}
	for attempt := 1; ; attempt++ {
		var stderr bytes.Buffer
		err = c.newKubectlCmd(bin, &stderr).Run()
		if err == nil {
			return nil
		}
		// Other failures, such as an unreachable cluster, would only wait out the long timeout again.
		if attempt == execAuthMaxAttempts || !k8s.IsExecPluginExitError(errors.New(stderr.String())) {
			return c.fail(err)
		}
		_, _ = fmt.Fprintf(c.stdout, "\nLogin failed, retrying (attempt %d/%d)\n", attempt+1, execAuthMaxAttempts)
	}
}

func (c *execAuthCmd) newKubectlCmd(bin string, stderr io.Writer) *exec.Cmd {
	// The timeout also counts the time spent at the plugin prompt, so keep it long.
	cmd := exec.Command(bin, k8s.DemoKubectlArgs([]string{
		"get", "--raw", "/version", "--context", c.kubectlContext, "--request-timeout=5m",
	})...)
	cmd.Env = os.Environ()
	if c.kubeconfig != "" {
		cmd.Env = append(cmd.Env, "KUBECONFIG="+c.kubeconfig)
	}
	cmd.Stdin = c.stdin
	cmd.Stdout = io.Discard
	// The plugin prompt goes to stderr, but tea's stderr is a capture pipe.
	cmd.Stderr = io.MultiWriter(c.stdout, stderr)
	return cmd
}

func (c *execAuthCmd) fail(err error) error {
	_, _ = fmt.Fprintf(c.stdout, "\n%v\nPress Enter to return to lfk\n", err)
	_, _ = bufio.NewReader(c.stdin).ReadString('\n')
	return err
}
