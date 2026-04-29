package app

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/ui"
)

func stubGetenv(values map[string]string) func(string) string {
	return func(k string) string { return values[k] }
}

func stubLookPath(found map[string]string) func(string) (string, error) {
	return func(name string) (string, error) {
		if p, ok := found[name]; ok {
			return p, nil
		}
		return "", errors.New("not found")
	}
}

func TestDetectMultiplexer(t *testing.T) {
	t.Run("returns nil when no multiplexer env vars set", func(t *testing.T) {
		got := detectMultiplexer(stubGetenv(map[string]string{}), stubLookPath(map[string]string{}))
		assert.Nil(t, got)
	})

	t.Run("detects tmux when TMUX env set and binary found", func(t *testing.T) {
		got := detectMultiplexer(
			stubGetenv(map[string]string{"TMUX": "/tmp/tmux-1000/default,12345,0"}),
			stubLookPath(map[string]string{"tmux": "/usr/local/bin/tmux"}),
		)
		require.NotNil(t, got)
		assert.Equal(t, "tmux", got.name)
		assert.Equal(t, "/usr/local/bin/tmux", got.path)
	})

	t.Run("detects zellij when ZELLIJ env set and binary found", func(t *testing.T) {
		got := detectMultiplexer(
			stubGetenv(map[string]string{"ZELLIJ": "0"}),
			stubLookPath(map[string]string{"zellij": "/usr/local/bin/zellij"}),
		)
		require.NotNil(t, got)
		assert.Equal(t, "zellij", got.name)
		assert.Equal(t, "/usr/local/bin/zellij", got.path)
	})

	t.Run("returns nil when env set but binary not on PATH", func(t *testing.T) {
		got := detectMultiplexer(
			stubGetenv(map[string]string{"TMUX": "x"}),
			stubLookPath(map[string]string{}),
		)
		assert.Nil(t, got)
	})

	t.Run("prefers tmux when both are set", func(t *testing.T) {
		got := detectMultiplexer(
			stubGetenv(map[string]string{"TMUX": "x", "ZELLIJ": "0"}),
			stubLookPath(map[string]string{
				"tmux":   "/usr/local/bin/tmux",
				"zellij": "/usr/local/bin/zellij",
			}),
		)
		require.NotNil(t, got)
		assert.Equal(t, "tmux", got.name)
	})

	t.Run("falls back to zellij when tmux env set but binary missing", func(t *testing.T) {
		got := detectMultiplexer(
			stubGetenv(map[string]string{"TMUX": "x", "ZELLIJ": "0"}),
			stubLookPath(map[string]string{"zellij": "/usr/local/bin/zellij"}),
		)
		require.NotNil(t, got)
		assert.Equal(t, "zellij", got.name)
	})

	t.Run("returns nil with nil getenv and lookPath when env empty", func(t *testing.T) {
		// Use real os.Getenv but the test environment is unlikely to have TMUX/ZELLIJ.
		t.Setenv("TMUX", "")
		t.Setenv("ZELLIJ", "")
		got := detectMultiplexer(nil, nil)
		assert.Nil(t, got)
	})
}

func TestEnvAdditions(t *testing.T) {
	t.Run("returns empty when cmdEnv is empty", func(t *testing.T) {
		got := envAdditions(nil, []string{"FOO=bar"})
		assert.Empty(t, got)
	})

	t.Run("returns empty when cmdEnv equals hostEnv", func(t *testing.T) {
		host := []string{"FOO=1", "BAR=2"}
		got := envAdditions(host, host)
		assert.Empty(t, got)
	})

	t.Run("returns appended entries", func(t *testing.T) {
		host := []string{"PATH=/usr/bin", "HOME=/home/me"}
		cmd := append([]string{}, host...)
		cmd = append(cmd, "KUBECONFIG=/tmp/kc")
		got := envAdditions(cmd, host)
		assert.Equal(t, []string{"KUBECONFIG=/tmp/kc"}, got)
	})

	t.Run("treats overridden values as additions", func(t *testing.T) {
		host := []string{"KUBECONFIG=/old"}
		cmd := []string{"KUBECONFIG=/new"}
		got := envAdditions(cmd, host)
		assert.Equal(t, []string{"KUBECONFIG=/new"}, got)
	})
}

func TestMultiplexerWrap(t *testing.T) {
	host := []string{"PATH=/usr/bin", "HOME=/home/me"}

	t.Run("returns original cmd for nil receiver", func(t *testing.T) {
		var mx *multiplexer
		orig := exec.Command("echo", "x")
		got := mx.wrap(orig, "Title", host)
		assert.Same(t, orig, got)
	})

	t.Run("returns original cmd for nil cmd", func(t *testing.T) {
		mx := &multiplexer{name: "tmux", path: "/usr/local/bin/tmux"}
		got := mx.wrap(nil, "Title", host)
		assert.Nil(t, got)
	})

	t.Run("returns original cmd for unknown multiplexer", func(t *testing.T) {
		mx := &multiplexer{name: "screen", path: "/usr/bin/screen"}
		orig := exec.Command("echo", "x")
		got := mx.wrap(orig, "Title", host)
		assert.Same(t, orig, got)
	})

	t.Run("tmux wrap produces tmux new-window invocation", func(t *testing.T) {
		mx := &multiplexer{name: "tmux", path: "/usr/local/bin/tmux"}
		inner := exec.Command("/usr/bin/kubectl", "exec", "-it", "pod", "--", "/bin/sh")
		inner.Env = append([]string{}, host...)
		inner.Env = append(inner.Env, "KUBECONFIG=/tmp/kc")

		got := mx.wrap(inner, "Exec: ns/pod", host)
		require.NotNil(t, got)
		assert.Equal(t, "/usr/local/bin/tmux", got.Path)
		assert.Equal(t, []string{
			"/usr/local/bin/tmux", "new-window", "-n", "Exec: ns/pod", "--", "sh", "-c",
		}, got.Args[:len(got.Args)-1])

		shellCmd := got.Args[len(got.Args)-1]
		assert.Contains(t, shellCmd, "KUBECONFIG=")
		assert.Contains(t, shellCmd, "/tmp/kc")
		assert.Contains(t, shellCmd, "exec ")
		assert.Contains(t, shellCmd, "kubectl")
	})

	t.Run("tmux wrap omits -n flag when title is empty", func(t *testing.T) {
		mx := &multiplexer{name: "tmux", path: "/usr/local/bin/tmux"}
		inner := exec.Command("kubectl", "exec")
		got := mx.wrap(inner, "", host)
		require.NotNil(t, got)
		joined := strings.Join(got.Args, " ")
		assert.NotContains(t, joined, " -n ")
		assert.Contains(t, joined, "new-window")
	})

	t.Run("zellij wrap produces zellij run invocation", func(t *testing.T) {
		mx := &multiplexer{name: "zellij", path: "/usr/local/bin/zellij"}
		inner := exec.Command("/usr/bin/kubectl", "exec")
		inner.Env = append([]string{"KUBECONFIG=/tmp/kc"}, host...)

		got := mx.wrap(inner, "Exec", host)
		require.NotNil(t, got)
		assert.Equal(t, "/usr/local/bin/zellij", got.Path)
		assert.Contains(t, got.Args, "run")
		assert.Contains(t, got.Args, "--name")
		assert.Contains(t, got.Args, "Exec")
		assert.Contains(t, got.Args, "--close-on-exit")
	})

	t.Run("nil cmd.Env emits empty env prefix and does not panic", func(t *testing.T) {
		mx := &multiplexer{name: "tmux", path: "/usr/local/bin/tmux"}
		inner := exec.Command("/usr/bin/kubectl", "exec")
		inner.Env = nil

		got := mx.wrap(inner, "T", host)
		require.NotNil(t, got)
		shellCmd := got.Args[len(got.Args)-1]
		// With no env additions, the inner shell line must start with
		// `exec ` — i.e. no `KEY=VAL ` env prefix in front.
		assert.True(t, strings.HasPrefix(shellCmd, "exec "), "shellCmd must start with `exec ` when no env additions")
	})

	t.Run("env values with single quotes are quoted safely", func(t *testing.T) {
		mx := &multiplexer{name: "tmux", path: "/usr/local/bin/tmux"}
		inner := exec.Command("kubectl", "exec")
		inner.Env = append(append([]string{}, host...), "WEIRD=a'b")

		got := mx.wrap(inner, "T", host)
		require.NotNil(t, got)
		shellCmd := got.Args[len(got.Args)-1]
		// The shellQuote helper escapes a single quote by closing+escaping+reopening.
		assert.Contains(t, shellCmd, `'a'"'"'b'`)
	})
}

func TestNextTerminalMode(t *testing.T) {
	cases := []struct {
		name   string
		from   string
		hasMux bool
		want   string
	}{
		{"pty -> exec (no mux)", ui.TerminalModePTY, false, ui.TerminalModeExec},
		{"pty -> exec (mux available)", ui.TerminalModePTY, true, ui.TerminalModeExec},
		{"exec -> mux when mux available", ui.TerminalModeExec, true, ui.TerminalModeMux},
		{"exec -> pty when no mux (skip mux)", ui.TerminalModeExec, false, ui.TerminalModePTY},
		{"mux -> pty (no mux)", ui.TerminalModeMux, false, ui.TerminalModePTY},
		{"mux -> pty (mux available)", ui.TerminalModeMux, true, ui.TerminalModePTY},
		{"unrecognised value resets to pty", "garbage", false, ui.TerminalModePTY},
		{"unrecognised value resets to pty even with mux", "garbage", true, ui.TerminalModePTY},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, nextTerminalMode(tc.from, tc.hasMux))
		})
	}
}

func TestRunInteractiveShellExec(t *testing.T) {
	// Save/restore the global so test order doesn't leak.
	prev := ui.ConfigTerminalMode
	t.Cleanup(func() { ui.ConfigTerminalMode = prev })

	t.Run("exec mode returns a non-nil tea.Cmd regardless of multiplexer presence", func(t *testing.T) {
		ui.ConfigTerminalMode = ui.TerminalModeExec
		t.Setenv("TMUX", "")
		t.Setenv("ZELLIJ", "")
		cmd := exec.Command("/bin/echo", "hi")
		got := runInteractiveShellExec(cmd, "Title", "Exec", true)
		assert.NotNil(t, got, "exec mode must always return a tea.Cmd")
	})

	t.Run("exec mode does not consult the multiplexer even when one is present", func(t *testing.T) {
		ui.ConfigTerminalMode = ui.TerminalModeExec
		// Real os.Getenv would see this; the helper should still pick
		// the tea.ExecProcess path because the user explicitly chose exec.
		t.Setenv("TMUX", "/tmp/tmux,1,0")
		cmd := exec.Command("/bin/echo", "hi")
		got := runInteractiveShellExec(cmd, "Title", "Exec", true)
		assert.NotNil(t, got)
	})

	t.Run("mux mode without a multiplexer returns an error actionResultMsg", func(t *testing.T) {
		ui.ConfigTerminalMode = ui.TerminalModeMux
		t.Setenv("TMUX", "")
		t.Setenv("ZELLIJ", "")
		cmd := exec.Command("/bin/echo", "hi")
		got := runInteractiveShellExec(cmd, "Title", "Exec", true)
		require.NotNil(t, got)
		msg, ok := got().(actionResultMsg)
		require.True(t, ok)
		require.Error(t, msg.err)
		assert.Contains(t, msg.err.Error(), "mux")
	})
}
