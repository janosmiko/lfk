package app

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/ui"
)

func TestCommandBar_ErrorLogOmitsCommandText(t *testing.T) {
	t.Setenv("KUBECTL_BIN", "/bin/true")
	prevMode := ui.ConfigTerminalMode
	t.Cleanup(func() { ui.ConfigTerminalMode = prevMode })

	for _, mode := range []string{ui.TerminalModePTY, ui.TerminalModeExec} {
		for _, input := range []string{"! curl --token=secret123 host", "k get pods --token=secret123"} {
			t.Run(mode+"/"+input, func(t *testing.T) {
				ui.ConfigTerminalMode = mode
				m := baseModelWithFakeClient()
				m.nav.Context = "prod-ctx"
				_, cmd := m.executeCommandBarInput(input)
				assert.NotNil(t, cmd)
				for _, e := range m.errorLog {
					assert.NotContains(t, e.Message, "secret123")
				}
			})
		}
	}
}

func TestCommandBar_LogsProgramAndContextNotArguments(t *testing.T) {
	t.Setenv("KUBECTL_BIN", "/bin/true")
	prevMode := ui.ConfigTerminalMode
	t.Cleanup(func() { ui.ConfigTerminalMode = prevMode })

	for _, mode := range []string{ui.TerminalModePTY, ui.TerminalModeExec} {
		for _, tc := range []struct{ name, input, program string }{
			{"shell", "! TOKEN=s3cr3t-placeholder-value curl host", "Running shell command"},
			{"kubectl", "k get pods --token s3cr3t-placeholder-value", "Running kubectl command"},
		} {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				ui.ConfigTerminalMode = mode
				var buf bytes.Buffer
				prev := logger.Logger
				logger.Logger = slog.New(slog.NewJSONHandler(&buf, nil))
				t.Cleanup(func() { logger.Logger = prev })

				m := baseModelWithFakeClient()
				m.nav.Context = "prod-ctx"
				_, cmd := m.executeCommandBarInput(tc.input)
				assert.NotNil(t, cmd)

				out := buf.String()
				assert.NotContains(t, out, "s3cr3t-placeholder-value")
				if mode == ui.TerminalModeExec && tc.name == "shell" {
					return
				}
				assert.Contains(t, out, tc.program)
				if tc.name == "shell" {
					assert.NotContains(t, out, "curl")
					assert.NotContains(t, out, "TOKEN")
				}
				assert.Contains(t, out, "prod-ctx")
			})
		}
	}
}
