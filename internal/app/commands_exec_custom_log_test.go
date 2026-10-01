package app

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/logger"
)

func TestExecCustomAction_LogsLabelAndContextNotCommand(t *testing.T) {
	var buf bytes.Buffer
	prev := logger.Logger
	logger.Logger = slog.New(slog.NewJSONHandler(&buf, nil))
	t.Cleanup(func() { logger.Logger = prev })

	m := testModelExec()
	m.actionCtx.context = "prod-ctx"
	_ = m.execCustomAction("my-action", "echo s3cr3t-placeholder-value")

	out := buf.String()
	assert.NotContains(t, out, "s3cr3t-placeholder-value")
	assert.Contains(t, out, "my-action")
	assert.Contains(t, out, "prod-ctx")
}
