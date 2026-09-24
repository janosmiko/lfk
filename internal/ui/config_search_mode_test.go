package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func snapshotSearchModeGlobal(t *testing.T) {
	t.Helper()
	original := ConfigDefaultSearchMode
	t.Cleanup(func() { ConfigDefaultSearchMode = original })
}

func TestSearchMode_InvalidFallsBack(t *testing.T) {
	snapshotSearchModeGlobal(t)

	path := writeConfigFile(t, "search_mode: bogus\n")
	LoadConfig(path)

	assert.Equal(t, DefaultSearchModeAuto, ConfigDefaultSearchMode)
}

func TestSearchMode_OmittedKeepsDefault(t *testing.T) {
	snapshotSearchModeGlobal(t)

	path := writeConfigFile(t, "confirm_on_exit: true\n")
	LoadConfig(path)

	assert.Equal(t, DefaultSearchModeAuto, ConfigDefaultSearchMode)
}

func TestSearchMode_AcceptsEachMode(t *testing.T) {
	for _, mode := range []string{DefaultSearchModeAuto, DefaultSearchModeLiteral, DefaultSearchModeFuzzy, DefaultSearchModeRegex} {
		t.Run(mode, func(t *testing.T) {
			snapshotSearchModeGlobal(t)

			path := writeConfigFile(t, "search_mode: "+mode+"\n")
			LoadConfig(path)

			assert.Equal(t, mode, ConfigDefaultSearchMode)
		})
	}
}
