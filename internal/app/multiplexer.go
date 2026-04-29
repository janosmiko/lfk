package app

import (
	"os"
	"os/exec"
	"strings"

	"github.com/janosmiko/lfk/internal/logger"
)

// multiplexer represents an outer terminal multiplexer (tmux/zellij) that
// lfk is running inside. When detected, exec-mode interactive shells can be
// spawned in a new window/tab of the multiplexer instead of replacing lfk
// via tea.ExecProcess. The host terminal then renders the shell directly,
// so text selection, scrollback, and copy/paste all work natively.
type multiplexer struct {
	name string // "tmux" or "zellij"
	path string // absolute path to the multiplexer binary
}

// detectMultiplexer reports the outer multiplexer if lfk is running inside
// one and the corresponding binary is on PATH. Returns nil otherwise.
//
// getenv and lookPath are injected for testability; pass nil to use the
// real os.Getenv / exec.LookPath.
func detectMultiplexer(getenv func(string) string, lookPath func(string) (string, error)) *multiplexer {
	if getenv == nil {
		getenv = os.Getenv
	}
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	candidates := []struct {
		name   string
		envVar string
	}{
		{"tmux", "TMUX"},
		{"zellij", "ZELLIJ"},
	}
	for _, c := range candidates {
		if getenv(c.envVar) == "" {
			continue
		}
		path, err := lookPath(c.name)
		if err != nil {
			continue
		}
		return &multiplexer{name: c.name, path: path}
	}
	return nil
}

// wrap returns a one-shot *exec.Cmd that, when run, asks the surrounding
// multiplexer to open a new window (tmux) or floating pane (zellij) running
// the original cmd. The returned command exits as soon as the multiplexer
// has registered the new pane — the user's shell continues independently
// of lfk.
//
// Env additions made by the caller on top of hostEnv (typically KUBECONFIG)
// are propagated to the new pane via inline shell variable assignments,
// since neither tmux's nor zellij's spawn API consistently forwards parent
// env. hostEnv is the lfk process environment, captured by the caller from
// os.Environ() so this function stays testable.
//
// PRECONDITION: cmd.Env must be set by the caller — typically as
// `append(os.Environ(), "KUBECONFIG=...")`. If cmd.Env is nil there is
// nothing to diff against hostEnv, so per-context overrides like
// KUBECONFIG will silently fall back to whatever env the multiplexer
// server inherited at session start. wrap logs a warning in that case to
// make the misuse loud.
func (mx *multiplexer) wrap(cmd *exec.Cmd, title string, hostEnv []string) *exec.Cmd {
	if mx == nil || cmd == nil {
		return cmd
	}
	if cmd.Env == nil {
		logger.Warn("multiplexer.wrap called with nil cmd.Env — per-context overrides (e.g. KUBECONFIG) will not propagate to the new pane",
			"multiplexer", mx.name)
	}

	var envPrefix strings.Builder
	for _, kv := range envAdditions(cmd.Env, hostEnv) {
		idx := strings.IndexByte(kv, '=')
		if idx <= 0 {
			continue
		}
		envPrefix.WriteString(kv[:idx])
		envPrefix.WriteByte('=')
		envPrefix.WriteString(shellQuote(kv[idx+1:]))
		envPrefix.WriteByte(' ')
	}

	quoted := make([]string, 0, len(cmd.Args))
	for _, a := range cmd.Args {
		quoted = append(quoted, shellQuote(a))
	}
	shellCmd := envPrefix.String() + "exec " + strings.Join(quoted, " ")

	switch mx.name {
	case "tmux":
		args := []string{"new-window"}
		if title != "" {
			args = append(args, "-n", title)
		}
		args = append(args, "--", "sh", "-c", shellCmd)
		wrapped := exec.Command(mx.path, args...)
		wrapped.Env = hostEnv
		return wrapped
	case "zellij":
		args := []string{"run"}
		if title != "" {
			args = append(args, "--name", title)
		}
		args = append(args, "--close-on-exit", "--", "sh", "-c", shellCmd)
		wrapped := exec.Command(mx.path, args...)
		wrapped.Env = hostEnv
		return wrapped
	}
	return cmd
}

// envAdditions returns entries in cmdEnv that are not present verbatim in
// hostEnv. The caller pattern in lfk is `cmd.Env = append(os.Environ(),
// "KUBECONFIG=...")`, so additions are typically the appended overrides.
func envAdditions(cmdEnv, hostEnv []string) []string {
	if len(cmdEnv) == 0 {
		return nil
	}
	base := make(map[string]struct{}, len(hostEnv))
	for _, kv := range hostEnv {
		base[kv] = struct{}{}
	}
	out := make([]string, 0)
	for _, kv := range cmdEnv {
		if _, ok := base[kv]; ok {
			continue
		}
		out = append(out, kv)
	}
	return out
}
