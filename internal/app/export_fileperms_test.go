package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Resource YAML exports can contain Secret objects (base64 data: tokens, TLS
// keys). They must be owner-only, never world-readable in the working dir.
func TestWriteSecureFileIsOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "Secret_db.yaml")

	require.NoError(t, writeSecureFile(p, []byte("data:\n  password: c2VjcmV0")))

	info, err := os.Stat(p)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
		"secret-bearing export must be 0600")
}

// A predictable export filename (<kind>_<name>.template.yaml) combined with
// O_TRUNC following a pre-existing symlink lets an attacker who plants a
// symlink at that path have the export overwrite whatever the symlink
// targets. writeSecureFile must replace the symlink itself (rename onto the
// path) rather than write through it.
func TestWriteSecureFileDoesNotFollowSymlink(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(dir, "victim.yaml")
	require.NoError(t, os.WriteFile(victim, []byte("ORIGINAL SECRET CONTENT"), 0o600))

	link := filepath.Join(dir, "pod_web.template.yaml")
	require.NoError(t, os.Symlink(victim, link))

	require.NoError(t, writeSecureFile(link, []byte("manifest: exported")))

	victimContent, err := os.ReadFile(victim)
	require.NoError(t, err)
	assert.Equal(t, "ORIGINAL SECRET CONTENT", string(victimContent),
		"export through a symlink must not overwrite the symlink's target")

	linkContent, err := os.ReadFile(link)
	require.NoError(t, err)
	assert.Equal(t, "manifest: exported", string(linkContent))
}

// Overwriting an export that already exists world-readable must tighten the
// mode — os.WriteFile alone does not chmod a pre-existing file.
func TestWriteSecureFileTightensExistingMode(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "Secret_db.yaml")
	require.NoError(t, os.WriteFile(p, []byte("old"), 0o644))

	require.NoError(t, writeSecureFile(p, []byte("new")))

	info, err := os.Stat(p)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	got, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, "new", string(got))
}

// Pod logs frequently carry bearer tokens / connection strings. Saved log files
// must be 0600, live under the OS temp dir (not a hardcoded /tmp that breaks on
// Windows), and use a random suffix so an attacker can't pre-plant a symlink at
// a predictable path (TOCTOU).
func TestWriteTempLogOwnerOnlyAndInTempDir(t *testing.T) {
	path, err := writeTempLog("lfk-logs-unit-*.log", []byte("line1\nline2\n"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(path) })

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	assert.Equal(t, filepath.Clean(os.TempDir()), filepath.Dir(path),
		"log must be written under os.TempDir(), never a hardcoded /tmp")
}

func TestSaveLoadedLogsIsOwnerOnly(t *testing.T) {
	m := baseModelCov()
	m.logView.lines = []string{"line1", "line2", "line3"}
	m.actionCtx = actionContext{name: "test-pod"}

	path, err := m.saveLoadedLogs()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(path) })

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
