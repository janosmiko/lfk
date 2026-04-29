package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// writeConfigFile writes a temporary YAML config file for tests.
func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	return path
}

// TestLogTailLinesShortFromConfig_SetTo25 verifies that log_tail_lines_short: 25
// is applied correctly.
func TestLogTailLinesShortFromConfig_SetTo25(t *testing.T) {
	prev := ConfigLogTailLinesShort
	t.Cleanup(func() { ConfigLogTailLinesShort = prev })
	ConfigLogTailLinesShort = 10 // reset to default before test

	path := writeConfigFile(t, "log_tail_lines_short: 25\n")
	LoadConfig(path)

	assert.Equal(t, 25, ConfigLogTailLinesShort)
}

// TestLogTailLinesShortFromConfig_ZeroPreservesDefault verifies that
// log_tail_lines_short: 0 leaves the default (10) unchanged.
func TestLogTailLinesShortFromConfig_ZeroPreservesDefault(t *testing.T) {
	prev := ConfigLogTailLinesShort
	t.Cleanup(func() { ConfigLogTailLinesShort = prev })
	ConfigLogTailLinesShort = 10 // reset to default before test

	path := writeConfigFile(t, "log_tail_lines_short: 0\n")
	LoadConfig(path)

	assert.Equal(t, 10, ConfigLogTailLinesShort, "zero value must not override default")
}

// TestLogTailLinesShortFromConfig_NegativePreservesDefault verifies that a
// negative log_tail_lines_short value leaves the default unchanged.
func TestLogTailLinesShortFromConfig_NegativePreservesDefault(t *testing.T) {
	prev := ConfigLogTailLinesShort
	t.Cleanup(func() { ConfigLogTailLinesShort = prev })
	ConfigLogTailLinesShort = 10 // reset to default before test

	path := writeConfigFile(t, "log_tail_lines_short: -5\n")
	LoadConfig(path)

	assert.Equal(t, 10, ConfigLogTailLinesShort, "negative value must not override default")
}

// TestLogTailLinesShortFromConfig_UnsetPreservesDefault verifies that omitting
// log_tail_lines_short in config leaves the default unchanged.
func TestLogTailLinesShortFromConfig_UnsetPreservesDefault(t *testing.T) {
	prev := ConfigLogTailLinesShort
	t.Cleanup(func() { ConfigLogTailLinesShort = prev })
	ConfigLogTailLinesShort = 10 // reset to default before test

	path := writeConfigFile(t, "log_tail_lines: 500\n") // unrelated field only
	LoadConfig(path)

	assert.Equal(t, 10, ConfigLogTailLinesShort, "unset field must not change default")
}

// TestLogTailLinesShortDefaultValue verifies the package-level default is 10.
func TestLogTailLinesShortDefaultValue(t *testing.T) {
	// We cannot re-run init, so we just verify the constant the code documents.
	// If someone changes the default they must update this assertion.
	assert.Equal(t, 10, ConfigLogTailLinesShort,
		"default ConfigLogTailLinesShort must be 10 per spec; update this test if intentionally changed")
}
