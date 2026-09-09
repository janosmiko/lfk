package ui

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/stretchr/testify/assert"
)

// Regression guard (CWE-532): a pasted secret under kubeconfig_dir must
// never reach the log file, only a description of its shape.
func TestApplyKubeconfigDirsSetting_InvalidValueNotLogged(t *testing.T) {
	origLogger := logger.Logger
	t.Cleanup(func() { logger.Logger = origLogger })

	origDirs := ConfigKubeconfigDirs
	t.Cleanup(func() { ConfigKubeconfigDirs = origDirs })

	tests := []struct {
		name    string
		raw     string
		secret  string
		wantLog string
	}{
		{
			name:    "map with secret token",
			raw:     `{"token":"super-secret-value-12345"}`,
			secret:  "super-secret-value-12345",
			wantLog: "map",
		},
		{
			name:    "number",
			raw:     "9876543210",
			secret:  "9876543210",
			wantLog: "number",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger.Logger = slog.New(slog.NewTextHandler(&buf, nil))

			applyKubeconfigDirsSetting(&kubeconfigDirsSetting{raw: tc.raw, invalid: true})

			out := buf.String()
			assert.NotContains(t, out, tc.raw, "raw config value must not be logged")
			assert.NotContains(t, out, tc.secret, "secret value must not be logged")
			assert.Contains(t, out, tc.wantLog, "log must describe the unrecognised shape")
		})
	}
}
