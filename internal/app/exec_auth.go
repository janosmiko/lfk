package app

import (
	"bufio"
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

const execAuthLoginAction = "__exec_auth_login__"

type execAuthState struct {
	tried   map[string]bool // contexts already offered a login
	context string          // context of the open box or running login, "" when idle
	cmd     loginCommand
	attempt int  // 1-based attempt about to run
	running bool // a login runs until its execAuthDoneMsg or a cancel
	ptyCmd  *exec.Cmd
	ptmx    *os.File // set once ptyCmd runs in the embedded terminal
}

type execAuthPromptMsg struct {
	context string
	cmd     loginCommand
}

type execAuthDoneMsg struct {
	context string
	err     error
}

// maybeExecAuth offers a login once per context, and only for the current
// context (hovering a context also loads). The cmd resolves the login off
// the Update goroutine, since the SSO check spawns aws.
func (m *Model) maybeExecAuth(ctx string, err error) tea.Cmd {
	if !k8s.IsExecPluginExitError(err) || ctx == "" || ctx == UnionContextSentinel ||
		ctx != m.nav.Context || m.nav.Level == model.LevelClusters ||
		m.demoMode || m.execAuth.tried[ctx] || !m.execAuthCanPrompt() {
		return nil
	}
	if m.execAuth.tried == nil {
		m.execAuth.tried = make(map[string]bool)
	}
	m.execAuth.tried[ctx] = true
	client, kubectlCtx := m.client, m.kubectlContext(ctx)
	return func() tea.Msg {
		info, ok := client.ExecPluginForContext(ctx)
		return execAuthPromptMsg{context: ctx, cmd: resolveLoginCommand(ctx, kubectlCtx, info, ok, awsProfileIsSSO)}
	}
}

// execAuthBusy reports whether the login box is on screen or a login runs.
func (m Model) execAuthBusy() bool {
	return (m.overlay == overlayConfirm && m.pendingAction == execAuthLoginAction) || m.execAuth.running
}

// execAuthCanPrompt keeps the box off viewers and live shells, where it
// would take the user's keystrokes.
func (m Model) execAuthCanPrompt() bool {
	return m.mode == modeExplorer && m.overlay == overlayNone && !m.execAuthBusy()
}

func (m Model) updateExecAuthPrompt(msg execAuthPromptMsg) Model {
	// A path that closes overlays without a cancel leaves the state behind.
	if m.execAuth.context != "" && !m.execAuthBusy() {
		m.execAuth = execAuthState{tried: m.execAuth.tried}
		if m.pendingAction == execAuthLoginAction {
			m.pendingAction, m.confirmTitle, m.confirmQuestion = "", "", ""
		}
	}
	// Clearing the guard lets the next failed load ask again.
	if !m.execAuthCanPrompt() || msg.context != m.nav.Context {
		delete(m.execAuth.tried, msg.context)
		return m
	}
	m.execAuth.context, m.execAuth.cmd, m.execAuth.attempt = msg.context, msg.cmd, 1
	return m.openExecAuthConfirm()
}

func (m Model) openExecAuthConfirm() Model {
	q := ui.SanitizeTerminalText(m.execAuth.context) + " needs login. Run: " + m.execAuth.cmd.display
	if m.execAuth.attempt > 1 {
		q = fmt.Sprintf("Login failed. Retry? (attempt %d/%d) %s", m.execAuth.attempt, execAuthMaxAttempts, q)
	}
	m.pendingAction = execAuthLoginAction
	m.confirmAction = ""
	m.confirmTitle = "Login required"
	m.confirmQuestion = q
	m.blast.reset()
	m.deps.reset()
	m.overlay = overlayConfirm
	return m
}

func (m Model) clearExecAuthConfirm() Model {
	m.overlay = overlayNone
	m.pendingAction = ""
	m.confirmTitle = ""
	m.confirmQuestion = ""
	return m
}

func (m Model) cancelExecAuthLogin() (tea.Model, tea.Cmd) {
	m = m.clearExecAuthConfirm()
	ctx := m.execAuth.context
	m.execAuth = execAuthState{tried: m.execAuth.tried}
	m.setStatusMessage(fmt.Sprintf("Login to %s skipped, press %s to ask again",
		ui.SanitizeTerminalText(ctx), ui.ActiveKeybindings.Refresh), false)
	return m, scheduleStatusClear()
}

func (m Model) startExecAuthLogin() (tea.Model, tea.Cmd) {
	m = m.clearExecAuthConfirm()
	ctx := m.execAuth.context
	cmd := &execAuthCmd{
		context:        ctx,
		kubectlContext: m.kubectlContext(ctx),
		login:          m.execAuth.cmd,
		kubeconfig:     m.client.KubeconfigPathForContext(ctx),
	}
	m.execAuth.running = true
	if execAuthUsesPTY() {
		return m.startExecAuthPTY(cmd)
	}
	return m, tea.Exec(cmd, func(err error) tea.Msg {
		return execAuthDoneMsg{context: ctx, err: err}
	})
}

func (m Model) updateExecAuthDone(msg execAuthDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil && msg.context == m.execAuth.context && m.execAuth.attempt < execAuthMaxAttempts {
		m.execAuth.attempt++
		m.execAuth.running, m.execAuth.ptyCmd, m.execAuth.ptmx = false, nil, nil
		m.setErrorFromErr("Login failed: ", msg.err)
		return m.openExecAuthConfirm(), scheduleStatusClear()
	}
	m.execAuth = execAuthState{tried: m.execAuth.tried}
	if msg.err != nil {
		m.setErrorFromErr("Auth failed: ", msg.err)
		return m, scheduleStatusClear()
	}
	done := "Authenticated to " + ui.SanitizeTerminalText(msg.context)
	cmds := []tea.Cmd{scheduleStatusClear()}
	if msg.context != m.nav.Context {
		m.setStatusMessage(done, false)
		return m, cmds[0]
	}
	// The guard stays set: a plugin that caches no token would loop otherwise.
	mdl, cmd := m.directActionRefresh()
	m = mdl.(Model)
	// Set after the refresh, since it replaces the status with "Refreshing...".
	m.setStatusMessage(done, false)
	cmds = append(cmds, cmd)
	if m.shouldFireDiscoveryFor(msg.context) {
		m.markDiscoveryStarted(msg.context)
		cmds = append(cmds, m.discoverAPIResources(msg.context))
	}
	return m, tea.Batch(cmds...)
}

// execAuthCmd runs the login out of process, not through client-go, because
// client-go caches os.Stderr per authenticator and that is a capture pipe here.
type execAuthCmd struct {
	context        string
	kubectlContext string // kubeconfig's original context name
	login          loginCommand
	kubeconfig     string
	stdin          io.Reader
	stdout         io.Writer
	stderr         io.Writer
}

func (c *execAuthCmd) SetStdin(r io.Reader)  { c.stdin = r }
func (c *execAuthCmd) SetStdout(w io.Writer) { c.stdout = w }
func (c *execAuthCmd) SetStderr(w io.Writer) { c.stderr = w }

func (c *execAuthCmd) Run() error {
	_, _ = fmt.Fprintf(c.stdout, "lfk: logging in to %s: %s\n", ui.SanitizeTerminalText(c.context), c.login.display)
	cmd, err := c.newLoginCmd()
	if err != nil {
		return c.fail(err)
	}
	if err := cmd.Run(); err != nil {
		return c.fail(err)
	}
	return nil
}

func (c *execAuthCmd) newLoginCmd() (*exec.Cmd, error) {
	if len(c.login.argv) == 0 {
		return nil, errors.New("no login command")
	}
	bin, args := c.login.argv[0], c.login.argv[1:]
	if c.login.kubectl {
		p, err := k8s.KubectlPath()
		if err != nil {
			return nil, fmt.Errorf("kubectl not found: %w", err)
		}
		bin, args = p, c.login.argv
	}
	cmd := exec.Command(bin, args...)
	cmd.Env = os.Environ()
	if c.kubeconfig != "" {
		cmd.Env = append(cmd.Env, "KUBECONFIG="+c.kubeconfig)
	}
	if c.kubectlContext != "" {
		cmd.Env = append(cmd.Env, "KUBECTL_CONTEXT="+c.kubectlContext)
	}
	cmd.Env = append(cmd.Env, c.login.env...)
	cmd.Stdin = c.stdin
	// kubectl prints only the version JSON, a login prints its device code.
	cmd.Stdout = c.stdout
	if c.login.kubectl {
		cmd.Stdout = io.Discard
	}
	// The plugin prompt goes to stderr, but tea's stderr is a capture pipe.
	cmd.Stderr = c.stdout
	return cmd, nil
}

func (c *execAuthCmd) fail(err error) error {
	_, _ = fmt.Fprintf(c.stdout, "\n%v\nPress Enter to return to lfk\n", err)
	_, _ = bufio.NewReader(c.stdin).ReadString('\n')
	return err
}
