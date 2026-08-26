package app

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/ui"
)

// fakeShellDir builds a PATH directory holding stub executables that print
// their own name. The stubs make the bootstrap's shell choice observable
// without a container.
func fakeShellDir(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		script := "#!/bin/sh\necho " + name + "\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(script), 0o700))
	}
	return dir
}

// runBootstrap runs cmd under /bin/sh with PATH limited to dir. This mirrors
// how kubectl exec runs it inside a container. It returns the stdout lines and
// the stderr text.
func runBootstrap(t *testing.T, cmdText, dir string) (stdout []string, stderr string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell bootstrap does not run on Windows")
	}
	cmd := exec.Command("/bin/sh", "-c", cmdText)
	cmd.Env = []string{"PATH=" + dir}
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	require.NoError(t, cmd.Run(), "bootstrap must exit cleanly, stderr: %s", errOut.String())
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	return lines, errOut.String()
}

// runDefaultBootstrap runs the bootstrap the default shell list produces.
func runDefaultBootstrap(t *testing.T, dir string) (stdout []string, stderr string) {
	t.Helper()
	return runBootstrap(t, buildLinuxExecShellCmd(ui.DefaultExecShells), dir)
}

func TestLinuxExecShellCmd_PrefersBash(t *testing.T) {
	out, _ := runDefaultBootstrap(t, fakeShellDir(t, "bash", "ash", "sh"))
	assert.Equal(t, []string{"bash"}, out)
}

func TestLinuxExecShellCmd_FallsBackToAsh(t *testing.T) {
	out, _ := runDefaultBootstrap(t, fakeShellDir(t, "ash", "sh"))
	assert.Equal(t, []string{"ash"}, out)
}

func TestLinuxExecShellCmd_FallsBackToSh(t *testing.T) {
	out, _ := runDefaultBootstrap(t, fakeShellDir(t, "sh"))
	assert.Equal(t, []string{"sh"}, out)
}

func TestLinuxExecShellCmd_ClearsScreenWhenClearExists(t *testing.T) {
	out, _ := runDefaultBootstrap(t, fakeShellDir(t, "clear", "bash"))
	assert.Equal(t, []string{"clear", "bash"}, out)
}

// TestLinuxExecShellCmd_SilentWhenClearMissing guards the reported bug. Minimal
// images ship no `clear`, and an unguarded call printed "clear: not found"
// before the shell started.
func TestLinuxExecShellCmd_SilentWhenClearMissing(t *testing.T) {
	out, errOut := runDefaultBootstrap(t, fakeShellDir(t, "bash"))
	assert.Equal(t, []string{"bash"}, out)
	assert.Empty(t, errOut, "a missing clear must not print an error")
}

func TestBuildLinuxExecShellCmd_HonoursConfiguredOrder(t *testing.T) {
	cmdText := buildLinuxExecShellCmd([]string{"ash", "bash", "sh"})
	out, _ := runBootstrap(t, cmdText, fakeShellDir(t, "bash", "ash", "sh"))
	assert.Equal(t, []string{"ash"}, out, "the configured order must win over the default one")
}

func TestBuildLinuxExecShellCmd_SingleShellRunsUnconditionally(t *testing.T) {
	cmdText := buildLinuxExecShellCmd([]string{"zsh"})
	out, _ := runBootstrap(t, cmdText, fakeShellDir(t, "zsh", "bash"))
	assert.Equal(t, []string{"zsh"}, out)
}

// TestBuildLinuxExecShellCmd_PassesFlags covers entries that carry arguments,
// such as the default `bash -i`.
func TestBuildLinuxExecShellCmd_PassesFlags(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\necho \"bash $*\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bash"), []byte(script), 0o700))
	out, _ := runBootstrap(t, buildLinuxExecShellCmd([]string{"bash -i"}), dir)
	assert.Equal(t, []string{"bash -i"}, out)
}

func TestBuildLinuxExecShellCmd_EmptyListFallsBackToDefaults(t *testing.T) {
	assert.Equal(t, buildLinuxExecShellCmd(ui.DefaultExecShells), buildLinuxExecShellCmd(nil))
}

// TestBuildLinuxExecShellCmd_DefaultMatchesShippedCommand pins the generated
// default to the hand-written command it replaced, so making the order
// configurable cannot change what an unconfigured user gets.
func TestBuildLinuxExecShellCmd_DefaultMatchesShippedCommand(t *testing.T) {
	const shipped = "command -v clear >/dev/null 2>&1 && clear; " +
		"if command -v bash >/dev/null 2>&1; then exec bash -i; " +
		"elif command -v ash >/dev/null 2>&1; then exec ash; else exec sh; fi"
	assert.Equal(t, shipped, buildLinuxExecShellCmd(ui.DefaultExecShells))
}
