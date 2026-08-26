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

// runLinuxExecShellCmd runs the bootstrap under /bin/sh with PATH limited to
// dir. This mirrors how kubectl exec runs it inside a container. It returns the
// stdout lines and the stderr text.
func runLinuxExecShellCmd(t *testing.T, dir string) (stdout []string, stderr string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell bootstrap does not run on Windows")
	}
	cmd := exec.Command("/bin/sh", "-c", linuxExecShellCmd)
	cmd.Env = []string{"PATH=" + dir}
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	require.NoError(t, cmd.Run(), "bootstrap must exit cleanly, stderr: %s", errOut.String())
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	return lines, errOut.String()
}

func TestLinuxExecShellCmd_PrefersBash(t *testing.T) {
	out, _ := runLinuxExecShellCmd(t, fakeShellDir(t, "bash", "ash", "sh"))
	assert.Equal(t, []string{"bash"}, out)
}

func TestLinuxExecShellCmd_FallsBackToAsh(t *testing.T) {
	out, _ := runLinuxExecShellCmd(t, fakeShellDir(t, "ash", "sh"))
	assert.Equal(t, []string{"ash"}, out)
}

func TestLinuxExecShellCmd_FallsBackToSh(t *testing.T) {
	out, _ := runLinuxExecShellCmd(t, fakeShellDir(t, "sh"))
	assert.Equal(t, []string{"sh"}, out)
}

func TestLinuxExecShellCmd_ClearsScreenWhenClearExists(t *testing.T) {
	out, _ := runLinuxExecShellCmd(t, fakeShellDir(t, "clear", "bash"))
	assert.Equal(t, []string{"clear", "bash"}, out)
}

// TestLinuxExecShellCmd_SilentWhenClearMissing guards the reported bug: minimal
// images ship no `clear`, and an unguarded call printed "clear: not found"
// before the shell started.
func TestLinuxExecShellCmd_SilentWhenClearMissing(t *testing.T) {
	out, errOut := runLinuxExecShellCmd(t, fakeShellDir(t, "bash"))
	assert.Equal(t, []string{"bash"}, out)
	assert.Empty(t, errOut, "a missing clear must not print an error")
}
