package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// snapshotLogViewerGlobals saves and restores every runtime global the
// log_viewer config maps into, so each test starts from known defaults.
func snapshotLogViewerGlobals(t *testing.T) {
	t.Helper()
	prevTail := ConfigLogTailLines
	prevShort := ConfigLogTailLinesShort
	prevAnsi := ConfigLogRenderAnsi
	prevPreview := ConfigLogShowPreview
	prevPrefixes := ConfigLogShowPrefixes
	prevTimestamps := ConfigLogShowTimestamps
	prevMaxLines := ConfigLogMaxLines
	t.Cleanup(func() {
		ConfigLogTailLines = prevTail
		ConfigLogTailLinesShort = prevShort
		ConfigLogRenderAnsi = prevAnsi
		ConfigLogShowPreview = prevPreview
		ConfigLogShowPrefixes = prevPrefixes
		ConfigLogShowTimestamps = prevTimestamps
		ConfigLogMaxLines = prevMaxLines
	})
	// Reset to compiled defaults before each test.
	ConfigLogTailLines = 100
	ConfigLogTailLinesShort = 10
	ConfigLogRenderAnsi = true
	ConfigLogShowPreview = true
	ConfigLogShowPrefixes = true
	ConfigLogShowTimestamps = false
	ConfigLogMaxLines = LogMaxLinesDefault
}

// TestLogViewer_GroupAppliesAllFields verifies the canonical log_viewer group
// wires every field into its runtime global.
func TestLogViewer_GroupAppliesAllFields(t *testing.T) {
	snapshotLogViewerGlobals(t)

	path := writeConfigFile(t, `log_viewer:
  tail_lines: 500
  tail_lines_short: 25
  render_ansi: false
  show_preview: false
  show_prefixes: false
  show_timestamps: true
`)
	LoadConfig(path)

	assert.Equal(t, 500, ConfigLogTailLines, "tail_lines")
	assert.Equal(t, 25, ConfigLogTailLinesShort, "tail_lines_short")
	assert.False(t, ConfigLogRenderAnsi, "render_ansi")
	assert.False(t, ConfigLogShowPreview, "show_preview")
	assert.False(t, ConfigLogShowPrefixes, "show_prefixes")
	assert.True(t, ConfigLogShowTimestamps, "show_timestamps")
}

// TestLogViewer_MaxLinesAppliesAndClamps verifies the buffer cap is wired and
// clamped to [LogMaxLinesMin, LogMaxLinesMax].
func TestLogViewer_MaxLinesAppliesAndClamps(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want int
	}{
		{"in range", "log_viewer:\n  max_lines: 25000\n", 25000},
		{"below floor", "log_viewer:\n  max_lines: 5\n", LogMaxLinesMin},
		{"above ceiling", "log_viewer:\n  max_lines: 9999999\n", LogMaxLinesMax},
		{"zero keeps default", "log_viewer:\n  max_lines: 0\n", LogMaxLinesDefault},
		{"omitted keeps default", "log_viewer:\n  show_timestamps: true\n", LogMaxLinesDefault},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			snapshotLogViewerGlobals(t)
			LoadConfig(writeConfigFile(t, tc.yaml))
			assert.Equal(t, tc.want, ConfigLogMaxLines)
		})
	}
}

// TestLogViewer_DeprecatedFlatAliasesStillApply verifies the legacy flat keys
// keep working for backward compatibility when no log_viewer group is present.
func TestLogViewer_DeprecatedFlatAliasesStillApply(t *testing.T) {
	snapshotLogViewerGlobals(t)

	path := writeConfigFile(t, `log_tail_lines: 333
log_tail_lines_short: 9
log_render_ansi: false
`)
	LoadConfig(path)

	assert.Equal(t, 333, ConfigLogTailLines, "log_tail_lines alias")
	assert.Equal(t, 9, ConfigLogTailLinesShort, "log_tail_lines_short alias")
	assert.False(t, ConfigLogRenderAnsi, "log_render_ansi alias")
}

// TestLogViewer_GroupOverridesFlatAlias verifies that when both a flat alias and
// its log_viewer equivalent are set, the canonical group wins.
func TestLogViewer_GroupOverridesFlatAlias(t *testing.T) {
	snapshotLogViewerGlobals(t)

	path := writeConfigFile(t, `log_tail_lines: 333
log_render_ansi: true
log_viewer:
  tail_lines: 777
  render_ansi: false
`)
	LoadConfig(path)

	assert.Equal(t, 777, ConfigLogTailLines, "log_viewer.tail_lines wins over flat")
	assert.False(t, ConfigLogRenderAnsi, "log_viewer.render_ansi wins over flat")
}

// TestLogViewer_OmittedKeysPreserveDefaults verifies a partial group leaves
// untouched globals at their defaults (pointer fields distinguish unset).
func TestLogViewer_OmittedKeysPreserveDefaults(t *testing.T) {
	snapshotLogViewerGlobals(t)

	path := writeConfigFile(t, `log_viewer:
  show_timestamps: true
`)
	LoadConfig(path)

	assert.True(t, ConfigLogShowTimestamps, "show_timestamps applied")
	assert.True(t, ConfigLogShowPreview, "show_preview default preserved")
	assert.True(t, ConfigLogShowPrefixes, "show_prefixes default preserved")
	assert.Equal(t, 100, ConfigLogTailLines, "tail_lines default preserved")
}

// TestLogViewer_NonPositiveTailLinesIgnored verifies a stray 0 keeps the default
// rather than zeroing the tail size.
func TestLogViewer_NonPositiveTailLinesIgnored(t *testing.T) {
	snapshotLogViewerGlobals(t)

	path := writeConfigFile(t, `log_viewer:
  tail_lines: 0
  tail_lines_short: -5
`)
	LoadConfig(path)

	assert.Equal(t, 100, ConfigLogTailLines, "tail_lines: 0 ignored")
	assert.Equal(t, 10, ConfigLogTailLinesShort, "tail_lines_short: -5 ignored")
}
