package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/ui"
)

// loggerUIMsg carries one deduplicated log event from the logger
// package into the Bubbletea Update loop. The Update handler appends
// it to m.errorLog so silent background failures (metrics-server
// unreachable, RBAC denied, kubeconfig exec plugin broken) show up in
// the in-app log overlay rather than only the on-disk log file.
type loggerUIMsg logger.UIEntry

// waitForLoggerUI subscribes to the logger UI channel. Returns the
// next entry as a loggerUIMsg; the Update handler re-arms by issuing
// another waitForLoggerUI command. Mirrors the StderrCapture
// subscription pattern (see waitForStderr).
func waitForLoggerUI() tea.Cmd {
	return func() tea.Msg {
		e, ok := <-logger.UIChan()
		if !ok {
			return nil
		}
		return loggerUIMsg(e)
	}
}

// formatLoggerUIArgs renders the slog-style args slice ([k1, v1, k2,
// v2, ...]) as "k1=v1 k2=v2" for the in-app overlay. Odd-length args
// (a key with no value) fall back to rendering the trailing key alone.
func formatLoggerUIArgs(args []any) string {
	if len(args) == 0 {
		return ""
	}
	var b strings.Builder
	for i := 0; i < len(args); i += 2 {
		if i > 0 {
			b.WriteByte(' ')
		}
		key := fmt.Sprintf("%v", args[i])
		if i+1 >= len(args) {
			b.WriteString(key)
			continue
		}
		val := fmt.Sprintf("%v", args[i+1])
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(val)
	}
	return b.String()
}

func (m *Model) appendLoggerUIEntry(e logger.UIEntry) {
	line := e.Message
	if extra := formatLoggerUIArgs(e.Args); extra != "" {
		line = line + " " + extra
	}
	m.appendErrorLogEntry(ui.ErrorLogEntry{
		Time:    e.Time,
		Message: line,
		Level:   e.Level,
	}, 500)
}
