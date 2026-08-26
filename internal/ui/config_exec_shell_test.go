package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyExecShellsConfig_KeepsDefaultsWhenUnset(t *testing.T) {
	t.Cleanup(func() { ConfigExecShells = DefaultExecShells })
	applyExecShellsConfig(nil)
	assert.Equal(t, DefaultExecShells, ConfigExecShells)
}

func TestApplyExecShellsConfig_UsesConfiguredList(t *testing.T) {
	t.Cleanup(func() { ConfigExecShells = DefaultExecShells })
	applyExecShellsConfig([]string{"zsh", "bash --login", "sh"})
	assert.Equal(t, []string{"zsh", "bash --login", "sh"}, ConfigExecShells)
}

func TestApplyExecShellsConfig_TrimsAndDropsBlanks(t *testing.T) {
	t.Cleanup(func() { ConfigExecShells = DefaultExecShells })
	applyExecShellsConfig([]string{"  bash -i  ", "", "   ", "sh"})
	assert.Equal(t, []string{"bash -i", "sh"}, ConfigExecShells)
}

// TestApplyExecShellsConfig_DropsShellMetacharacters guards the generated
// bootstrap. An entry carrying a command separator or a substitution would run
// as a second command inside the container rather than as a shell name.
func TestApplyExecShellsConfig_DropsShellMetacharacters(t *testing.T) {
	t.Cleanup(func() { ConfigExecShells = DefaultExecShells })
	for _, bad := range []string{
		"sh; rm -rf /",
		"sh && curl evil",
		"$(id)",
		"`id`",
		"sh | tee /tmp/x",
		"sh\nrm -rf /",
		"sh > /tmp/x",
		"sh 'quoted'",
	} {
		applyExecShellsConfig([]string{bad, "sh"})
		assert.Equal(t, []string{"sh"}, ConfigExecShells, "entry %q must be dropped", bad)
	}
}

func TestApplyExecShellsConfig_KeepsDefaultsWhenAllEntriesInvalid(t *testing.T) {
	t.Cleanup(func() { ConfigExecShells = DefaultExecShells })
	applyExecShellsConfig([]string{"sh; id", ""})
	assert.Equal(t, DefaultExecShells, ConfigExecShells, "an all-invalid list must not leave the user with no shell")
}
