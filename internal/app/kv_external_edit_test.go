package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

func secretEditorModel() Model {
	return Model{
		overlay: overlaySecretEditor,
		secretData: &model.SecretData{
			Keys: []string{"password", "app.yml"},
			Data: map[string]string{"password": "s3cret", "app.yml": "a: 1\n"},
		},
		secretDataOriginal: map[string]string{"password": "s3cret", "app.yml": "a: 1\n"},
		secretCursor:       0,
		tabs:               []TabState{{}},
		width:              80,
		height:             40,
	}
}

func configMapEditorModel() Model {
	return Model{
		overlay: overlayConfigMapEditor,
		configMapData: &model.ConfigMapData{
			Keys: []string{"application.yml"},
			Data: map[string]string{"application.yml": "server:\n  port: 8080\n"},
		},
		configMapDataOriginal: map[string]string{"application.yml": "server:\n  port: 8080\n"},
		tabs:                  []TabState{{}},
		width:                 80,
		height:                40,
	}
}

func TestSecretEditor_ShiftEAsksBeforeWritingPlainText(t *testing.T) {
	m := secretEditorModel()
	ret, cmd := m.handleSecretEditorKey(keyMsg("E"))
	r := ret.(Model)
	assert.Nil(t, cmd, "no editor may start before the user confirms")
	assert.Equal(t, overlayConfirm, r.overlay)
	assert.Equal(t, externalEditSecretAction, r.pendingAction)
	assert.Equal(t, "password", r.confirmAction)
	assert.Contains(t, r.confirmQuestion, "plain text")
}

func TestSecretEditor_ExternalEditCancelReturnsToEditor(t *testing.T) {
	m := secretEditorModel()
	ret, _ := m.handleSecretEditorKey(keyMsg("E"))
	ret, cmd := ret.(Model).handleConfirmOverlayKey(keyMsg("n"))
	r := ret.(Model)
	assert.Nil(t, cmd)
	assert.Equal(t, overlaySecretEditor, r.overlay)
	assert.Empty(t, r.pendingAction)
	assert.NotNil(t, r.secretData, "the editor and its pending changes stay alive")
}

func TestSecretEditor_ExternalEditConfirmStartsEditor(t *testing.T) {
	m := secretEditorModel()
	ret, _ := m.handleSecretEditorKey(keyMsg("E"))
	ret, cmd := ret.(Model).handleConfirmOverlayKey(keyMsg("y"))
	r := ret.(Model)
	assert.NotNil(t, cmd)
	assert.Equal(t, overlaySecretEditor, r.overlay)
	assert.Empty(t, r.pendingAction)
	assert.False(t, r.loading)
}

func TestConfigMapEditor_ShiftEStartsEditorWithoutConfirm(t *testing.T) {
	m := configMapEditorModel()
	ret, cmd := m.handleConfigMapEditorKey(keyMsg("E"))
	assert.NotNil(t, cmd)
	assert.Equal(t, overlayConfigMapEditor, ret.(Model).overlay)
}

func TestUpdateKVExternalEdit(t *testing.T) {
	t.Run("edited value becomes a pending change", func(t *testing.T) {
		m := configMapEditorModel()
		ret, _ := m.updateKVExternalEdit(kvExternalEditMsg{owner: m.configMapData, key: "application.yml", value: "server:\n  port: 9090\n"})
		r := ret.(Model)
		assert.Equal(t, "server:\n  port: 9090\n", r.configMapData.Data["application.yml"])
		assert.True(t, r.configMapDataDirty())
	})
	t.Run("secret value becomes a pending change", func(t *testing.T) {
		m := secretEditorModel()
		ret, _ := m.updateKVExternalEdit(kvExternalEditMsg{owner: m.secretData, key: "password", value: "new"})
		r := ret.(Model)
		assert.Equal(t, "new", r.secretData.Data["password"])
		assert.True(t, r.secretDataDirty())
	})
	t.Run("unchanged value leaves the editor clean", func(t *testing.T) {
		m := configMapEditorModel()
		ret, _ := m.updateKVExternalEdit(kvExternalEditMsg{owner: m.configMapData, key: "application.yml", value: "server:\n  port: 8080\n"})
		r := ret.(Model)
		assert.False(t, r.configMapDataDirty())
	})
	t.Run("editor closed meanwhile drops the result", func(t *testing.T) {
		m := configMapEditorModel()
		m.overlay = overlayNone
		ret, _ := m.updateKVExternalEdit(kvExternalEditMsg{owner: m.configMapData, key: "application.yml", value: "x"})
		assert.Equal(t, "server:\n  port: 8080\n", ret.(Model).configMapData.Data["application.yml"])
	})
	t.Run("deleted key drops the result", func(t *testing.T) {
		m := configMapEditorModel()
		ret, _ := m.updateKVExternalEdit(kvExternalEditMsg{owner: m.configMapData, key: "gone", value: "x"})
		_, ok := ret.(Model).configMapData.Data["gone"]
		assert.False(t, ok)
	})
	t.Run("editor error leaves the value and reports it", func(t *testing.T) {
		m := configMapEditorModel()
		ret, _ := m.updateKVExternalEdit(kvExternalEditMsg{owner: m.configMapData, key: "application.yml", err: errors.New("boom")})
		r := ret.(Model)
		assert.Equal(t, "server:\n  port: 8080\n", r.configMapData.Data["application.yml"])
		assert.True(t, r.statusMessageErr)
	})
	t.Run("result for a reloaded resource with the same key is dropped", func(t *testing.T) {
		m := secretEditorModel()
		launched := m.secretData
		m.secretData = &model.SecretData{Keys: []string{"password"}, Data: map[string]string{"password": "other"}}
		ret, _ := m.updateKVExternalEdit(kvExternalEditMsg{owner: launched, key: "password", value: "new"})
		assert.Equal(t, "other", ret.(Model).secretData.Data["password"])
	})
	t.Run("failed temp cleanup is reported", func(t *testing.T) {
		m := configMapEditorModel()
		ret, _ := m.updateKVExternalEdit(kvExternalEditMsg{owner: m.configMapData, key: "application.yml", value: "x", cleanupErr: errors.New("busy")})
		r := ret.(Model)
		assert.Equal(t, "x", r.configMapData.Data["application.yml"], "the edit still applies")
		assert.True(t, r.statusMessageErr)
		assert.Contains(t, r.statusMessage, "Temp file not removed")
	})
}

func TestKeepTrailingNewline(t *testing.T) {
	tests := []struct{ orig, edited, want string }{
		{"abc", "abc\n", "abc"},     // editor added a final newline
		{"abc\n", "abc\n", "abc\n"}, // original had one
		{"abc", "abc\n\n", "abc\n"}, // user added a blank line
		{"", "\n", ""},              // empty value stays empty
		{"abc", "xyz", "xyz"},       // no newline at all
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, keepTrailingNewline(tc.orig, tc.edited), "orig=%q edited=%q", tc.orig, tc.edited)
	}
}

func TestEditTempFileName(t *testing.T) {
	tests := map[string]string{
		"application.yml": "application.yml",
		"tls.crt":         "tls.crt",
		"../etc/passwd":   ".._etc_passwd",
		"a/b":             "a_b",
		"..":              "value",
		".":               "value",
		"":                "value",
	}
	for in, want := range tests {
		assert.Equal(t, want, editTempFileName(in), "key %q", in)
	}
}

func TestNoCopyEditorArgs(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	assert.NotEmpty(t, noCopyEditorArgs("nvim"))
	assert.NotEmpty(t, noCopyEditorArgs("/usr/bin/vim"))
	assert.NotEmpty(t, noCopyEditorArgs("vim.exe"))
	assert.Contains(t, noCopyEditorArgs("vim"), "-n")
	assert.Empty(t, noCopyEditorArgs("code"))
	assert.Empty(t, noCopyEditorArgs("vi"), "vi may be busybox/nvi, which reject -n")
}

func TestNoCopyEditorArgs_ViSymlinkedToVim(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks")
	}
	bin := t.TempDir()
	real := filepath.Join(bin, "vim.basic")
	require.NoError(t, os.WriteFile(real, []byte("#!/bin/sh\n"), 0o700))
	require.NoError(t, os.Symlink(real, filepath.Join(bin, "vi")))
	t.Setenv("PATH", bin)
	assert.Contains(t, noCopyEditorArgs("vi"), "-n")
}

func TestEditorArgv_NoCopyFlagsBeforeUserArgs(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	argv := editorArgv([]string{"vim", "--"}, "/tmp/x", true)
	assert.Equal(t, "vim", argv[0])
	assert.Equal(t, "-n", argv[1])
	assert.Equal(t, []string{"--", "/tmp/x"}, argv[len(argv)-2:])
	assert.Equal(t, []string{"vim", "--", "/tmp/x"}, editorArgv([]string{"vim", "--"}, "/tmp/x", false))
}

func TestSweepStaleEditDirs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses true as a short-lived process")
	}
	exited := exec.Command("true")
	require.NoError(t, exited.Run())
	deadPid := exited.Process.Pid

	root := t.TempDir()
	dead := filepath.Join(root, fmt.Sprintf("lfk-edit-%d-123", deadPid))
	live := filepath.Join(root, fmt.Sprintf("lfk-edit-%d-456", os.Getpid()))
	noPid := filepath.Join(root, "lfk-edit-abc")
	other := filepath.Join(root, fmt.Sprintf("other-%d-1", deadPid))
	for _, d := range []string{dead, live, noPid, other} {
		require.NoError(t, os.Mkdir(d, 0o700))
	}

	sweepStaleEditDirs(root)

	assert.NoDirExists(t, dead)
	assert.DirExists(t, live, "a running lfk can still have its editor open")
	assert.DirExists(t, noPid)
	assert.DirExists(t, other)
}

func TestValueEdit_RunEditsAndRemovesTempFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh as the editor")
	}
	out := t.TempDir()
	seenValue, seenPath := filepath.Join(out, "value"), filepath.Join(out, "path")
	t.Setenv("KUBE_EDITOR", `sh -c 'cat "$1" > `+seenValue+`; printf %s "$1" > `+seenPath+`; printf new > "$1"' sh`)
	e := &valueEdit{base: kvExternalEditMsg{key: "password"}, orig: "s3cret", sensitive: true}
	require.NoError(t, e.Run())

	assert.Equal(t, kvExternalEditMsg{key: "password", value: "new"}, e.msg)
	v, err := os.ReadFile(seenValue)
	require.NoError(t, err)
	assert.Equal(t, "s3cret", string(v), "editor got the decoded value")
	p, err := os.ReadFile(seenPath)
	require.NoError(t, err)
	assert.Equal(t, "password", filepath.Base(string(p)))
	assert.NoDirExists(t, filepath.Dir(string(p)), "temp dir removed after the edit")
}

func TestValueEdit_ResultWhenRunNeverRan(t *testing.T) {
	owner := &model.SecretData{}
	e := &valueEdit{base: kvExternalEditMsg{owner: owner, key: "password"}}
	msg := e.result(errors.New("release terminal")).(kvExternalEditMsg)
	assert.Same(t, owner, msg.owner)
	assert.Equal(t, "password", msg.key)
	assert.Error(t, msg.err)
}

func TestValueEdit_RunReportsTempFileError(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	if editTempRoot() != "" {
		t.Skip("/dev/shm is used instead of TMPDIR")
	}
	e := &valueEdit{base: kvExternalEditMsg{key: "k"}, orig: "v"}
	assert.Error(t, e.Run())
	assert.Error(t, e.result(nil).(kvExternalEditMsg).err)
}

func TestWriteEditTempFile_PrivateAndRemoved(t *testing.T) {
	dir, path, err := writeEditTempFile(t.TempDir(), "app.yml", "a: 1\n")
	require.NoError(t, err)

	if runtime.GOOS != "windows" { // Windows reports 0777/0666 and uses ACLs
		di, err := os.Stat(dir)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o700), di.Mode().Perm())
		fi, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), fi.Mode().Perm())
	}
	assert.Equal(t, "app.yml", filepath.Base(path))
	assert.Contains(t, filepath.Base(dir), fmt.Sprintf("lfk-edit-%d-", os.Getpid()))

	require.NoError(t, os.WriteFile(path, []byte("a: 2\n"), 0o600))
	msg := finishEditTempFile(dir, path, kvExternalEditMsg{key: "app.yml"}, "a: 1\n", nil)
	assert.Equal(t, kvExternalEditMsg{key: "app.yml", value: "a: 2\n"}, msg)
	_, err = os.Stat(dir)
	assert.True(t, os.IsNotExist(err), "temp dir must be removed after reading")
}

func TestFinishEditTempFile_EditorErrorStillRemovesDir(t *testing.T) {
	dir, path, err := writeEditTempFile(t.TempDir(), "password", "s3cret")
	require.NoError(t, err)
	msg := finishEditTempFile(dir, path, kvExternalEditMsg{key: "password"}, "s3cret", errors.New("exit 1"))
	assert.Error(t, msg.err)
	_, err = os.Stat(dir)
	assert.True(t, os.IsNotExist(err))
}
