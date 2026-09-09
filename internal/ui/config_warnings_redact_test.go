package ui

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
)

// Regression guard (CWE-532): a config warning reports which key is wrong,
// never what the user typed there. A secret pasted into the wrong key must
// stay out of the log file.
func TestConfigWarnings_NeverEchoUserValue(t *testing.T) {
	origLogger := logger.Logger
	t.Cleanup(func() { logger.Logger = origLogger })

	const secret = "hunter2-secret-token-xyz"

	tests := []struct {
		name  string
		yaml  string
		leaks []string
		want  string
	}{
		{
			name:  "rightsizing strategy",
			yaml:  "rightsizing_defaults:\n  strategy: " + secret + "\n",
			leaks: []string{secret},
			want:  "rightsizing_defaults.strategy",
		},
		{
			name:  "rightsizing headroom",
			yaml:  "rightsizing_defaults:\n  headroom: 987654.321\n",
			leaks: []string{"987654.321", "987654"},
			want:  "rightsizing_defaults.headroom",
		},
		{
			name:  "scrollback_lines clamp",
			yaml:  "scrollback_lines: 987654321\n",
			leaks: []string{"987654321"},
			want:  "scrollback_lines",
		},
		{
			name:  "log_viewer.max_lines clamp",
			yaml:  "log_viewer:\n  max_lines: 987654321\n",
			leaks: []string{"987654321"},
			want:  "log_viewer.max_lines",
		},
		{
			name:  "union_sets unknown color",
			yaml:  "union_sets:\n  - name: prod\n    contexts:\n      - context: a\n        color: " + secret + "\n",
			leaks: []string{secret},
			want:  "unknown color",
		},
		{
			name:  "union_sets nameless entry",
			yaml:  "union_sets:\n  - contexts:\n      - context: " + secret + "\n",
			leaks: []string{secret},
			want:  "no name",
		},
		{
			name:  "union_sets duplicate context",
			yaml:  "union_sets:\n  - name: " + secret + "\n    contexts:\n      - context: " + secret + "\n      - context: " + secret + "\n",
			leaks: []string{secret},
			want:  "repeats a context",
		},
		{
			name:  "union_sets duplicate set name",
			yaml:  "union_sets:\n  - name: " + secret + "\n    contexts:\n      - context: a\n  - name: " + secret + "\n    contexts:\n      - context: b\n",
			leaks: []string{secret},
			want:  "duplicate union_sets name",
		},
		{
			name:  "views invalid column spec",
			yaml:  "views:\n  pods:\n    columns:\n      - \"" + secret + "|Z\"\n",
			leaks: []string{secret},
			want:  "invalid view config",
		},
		{
			name:  "views invalid jsonpath",
			yaml:  "views:\n  pods:\n    columns:\n      - \"col:" + secret + "\"\n",
			leaks: []string{secret},
			want:  "invalid view config",
		},
		{
			name:  "resource_columns invalid column spec",
			yaml:  "resource_columns:\n  pods:\n    - \"" + secret + "|Z\"\n",
			leaks: []string{secret},
			want:  "invalid resource_columns config",
		},
	}

	prevStrategy := model.ConfigDefaultRightsizingStrategy
	prevHeadroom := model.ConfigDefaultRightsizingHeadroom
	prevViews := ConfigViews
	prevSets := ConfigUnionSets
	t.Cleanup(func() {
		model.ConfigDefaultRightsizingStrategy = prevStrategy
		model.ConfigDefaultRightsizingHeadroom = prevHeadroom
		ConfigViews = prevViews
		ConfigUnionSets = prevSets
	})

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger.Logger = slog.New(slog.NewTextHandler(&buf, nil))

			LoadConfig(writeConfigFile(t, tc.yaml))

			out := buf.String()
			assert.Contains(t, out, tc.want, "warning must name the offending key")
			for _, leak := range tc.leaks {
				assert.NotContains(t, out, leak, "raw config value must not be logged")
			}
		})
	}
}
