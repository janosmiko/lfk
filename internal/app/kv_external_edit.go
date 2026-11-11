package app

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

// externalEditSecretAction is the pendingAction label that routes the
// plain-text warning back to the Secret editor.
const externalEditSecretAction = "Edit Secret Value"

// kvExternalEditMsg carries a value edited in $EDITOR back to the K/V
// editor that opened it.
type kvExternalEditMsg struct {
	overlay overlayKind
	key     string
	value   string
	err     error
}

func (m Model) beginSecretExternalEdit() (tea.Model, tea.Cmd) {
	visible := m.secretVisibleKeys()
	if m.secretCursor < 0 || m.secretCursor >= len(visible) {
		return m, nil
	}
	key := visible[m.secretCursor]
	m.confirmAction = key
	m.pendingAction = externalEditSecretAction
	m.confirmTitle = "Edit in $EDITOR"
	where := "on disk"
	if editTempRoot() != "" {
		where = "in RAM (/dev/shm)"
	}
	m.confirmQuestion = fmt.Sprintf("Write the decoded value of %s in plain text to a temp file %s? Editor swap or undo files can keep a copy.", key, where)
	m.blast.reset()
	m.deps.reset()
	m.overlay = overlayConfirm
	return m, nil
}

func (m Model) commitSecretExternalEdit() (tea.Model, tea.Cmd) {
	key := m.confirmAction
	m = m.clearSecretExternalEditConfirm()
	if m.secretData == nil {
		return m, nil
	}
	value, ok := m.secretData.Data[key]
	if !ok {
		return m, nil
	}
	return m, editValueInEditor(overlaySecretEditor, key, value, true)
}

func (m Model) clearSecretExternalEditConfirm() Model {
	m.confirmAction = ""
	m.pendingAction = ""
	m.confirmTitle = ""
	m.confirmQuestion = ""
	m.overlay = overlaySecretEditor
	return m
}

func (m Model) beginConfigMapExternalEdit() (tea.Model, tea.Cmd) {
	visible := m.configMapVisibleKeys()
	if m.configMapCursor < 0 || m.configMapCursor >= len(visible) {
		return m, nil
	}
	key := visible[m.configMapCursor]
	return m, editValueInEditor(overlayConfigMapEditor, key, m.configMapData.Data[key], false)
}

// updateKVExternalEdit stores the edited value as a pending change. The
// user saves it with Enter, like an inline edit.
func (m Model) updateKVExternalEdit(msg kvExternalEditMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setStatusMessage("Edit in $EDITOR failed: "+msg.err.Error(), true)
		return m, scheduleStatusClear()
	}
	var data map[string]string
	switch {
	case msg.overlay == overlaySecretEditor && m.overlay == overlaySecretEditor && m.secretData != nil:
		data = m.secretData.Data
	case msg.overlay == overlayConfigMapEditor && m.overlay == overlayConfigMapEditor && m.configMapData != nil:
		data = m.configMapData.Data
	}
	old, ok := data[msg.key]
	if !ok {
		m.setStatusMessage("Editor closed or key removed, edit of "+msg.key+" discarded", true)
		return m, scheduleStatusClear()
	}
	if old == msg.value {
		m.setStatusMessage("No changes to "+msg.key, false)
		return m, scheduleStatusClear()
	}
	data[msg.key] = msg.value
	m.setStatusMessage("Updated "+msg.key+", press enter to save", false)
	return m, scheduleStatusClear()
}

// editValueInEditor opens value in the user's editor and returns the
// result as a kvExternalEditMsg. sensitive turns off editor copies
// (swap, undo, backup) for editors that support it.
func editValueInEditor(target overlayKind, key, value string, sensitive bool) tea.Cmd {
	e := &valueEdit{target: target, key: key, orig: value, sensitive: sensitive}
	return tea.Exec(e, e.result)
}

// result covers bubbletea failing to release the terminal, in which
// case Run never ran and e.msg is empty.
func (e *valueEdit) result(err error) tea.Msg {
	if e.msg == (kvExternalEditMsg{}) {
		return kvExternalEditMsg{overlay: e.target, key: e.key, err: err}
	}
	return e.msg
}

// valueEdit writes the temp file only when bubbletea runs the editor, so
// a command that never runs leaves no plain-text file behind.
type valueEdit struct {
	target         overlayKind
	key, orig      string
	sensitive      bool
	stdin          io.Reader
	stdout, stderr io.Writer
	msg            kvExternalEditMsg
}

func (e *valueEdit) SetStdin(r io.Reader)  { e.stdin = r }
func (e *valueEdit) SetStdout(w io.Writer) { e.stdout = w }
func (e *valueEdit) SetStderr(w io.Writer) { e.stderr = w }

// Run reports every outcome through e.msg. The callback ignores the
// returned error.
func (e *valueEdit) Run() error {
	root := editTempRoot()
	sweepStaleEditDirs(root, time.Now().Add(-staleEditDirAge))
	dir, path, err := writeEditTempFile(root, e.key, e.orig)
	if err != nil {
		e.msg = kvExternalEditMsg{overlay: e.target, key: e.key, err: err}
		return err
	}
	argv := editorArgv(editorCommand(), path, e.sensitive)
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = e.stdin, e.stdout, e.stderr
	e.msg = finishEditTempFile(dir, path, e.target, e.key, e.orig, cmd.Run())
	return nil
}

// editorArgv puts the no-copy flags before the user's own args, so a
// "--" in $EDITOR cannot turn them into file names.
func editorArgv(editor []string, path string, sensitive bool) []string {
	argv := []string{editor[0]}
	if sensitive {
		argv = append(argv, noCopyEditorArgs(editor[0])...)
	}
	argv = append(argv, editor[1:]...)
	return append(argv, path)
}

// staleEditDirAge is long, because another lfk can still have its editor open.
const staleEditDirAge = 24 * time.Hour

// sweepStaleEditDirs removes temp dirs left behind when lfk was killed
// while the editor was open.
func sweepStaleEditDirs(root string, before time.Time) {
	if root == "" {
		root = os.TempDir()
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, ent := range entries {
		if !ent.IsDir() || !strings.HasPrefix(ent.Name(), "lfk-edit-") {
			continue
		}
		if fi, err := ent.Info(); err == nil && fi.ModTime().Before(before) {
			_ = os.RemoveAll(filepath.Join(root, ent.Name()))
		}
	}
}

// writeEditTempFile writes value to a 0600 file in a fresh 0700 dir. The
// file is named after the key so the editor picks the right file type.
func writeEditTempFile(root, key, value string) (dir, path string, err error) {
	dir, err = os.MkdirTemp(root, "lfk-edit-*")
	if err != nil {
		return "", "", fmt.Errorf("creating temp dir: %w", err)
	}
	path = filepath.Join(dir, editTempFileName(key))
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return "", "", fmt.Errorf("writing temp file: %w", err)
	}
	return dir, path, nil
}

// finishEditTempFile reads the edited file and removes the temp dir on
// every path, so a decoded Secret value does not stay on disk.
func finishEditTempFile(dir, path string, target overlayKind, key, orig string, editorErr error) kvExternalEditMsg {
	defer func() { _ = os.RemoveAll(dir) }()
	if editorErr != nil {
		return kvExternalEditMsg{overlay: target, key: key, err: fmt.Errorf("editor: %w", editorErr)}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return kvExternalEditMsg{overlay: target, key: key, err: fmt.Errorf("reading temp file: %w", err)}
	}
	return kvExternalEditMsg{overlay: target, key: key, value: keepTrailingNewline(orig, string(b))}
}

// editTempRoot prefers /dev/shm (RAM-backed on Linux) so the value never
// reaches a disk. Other systems use the per-user temp dir.
func editTempRoot() string {
	if fi, err := os.Stat("/dev/shm"); err == nil && fi.IsDir() {
		return "/dev/shm"
	}
	return ""
}

func editTempFileName(key string) string {
	name := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		}
		return '_'
	}, key)
	if name == "" || name == "." || name == ".." {
		return "value"
	}
	return name
}

// keepTrailingNewline drops the final newline most editors add on save,
// so opening and closing a value without edits leaves it unchanged.
func keepTrailingNewline(orig, edited string) string {
	if strings.HasSuffix(orig, "\n") {
		return edited
	}
	return strings.TrimSuffix(edited, "\n")
}

// noCopyEditorArgs turns off swap, shada/viminfo, undo and backup files.
// The name is resolved through symlinks, because vi is often vim
// (vi -> vim.basic). Real vi is left out: busybox vi and nvi reject -n.
func noCopyEditorArgs(editor string) []string {
	if p, err := exec.LookPath(editor); err == nil {
		if real, err := filepath.EvalSymlinks(p); err == nil {
			editor = real
		}
	}
	name := strings.TrimSuffix(filepath.Base(editor), ".exe")
	if name == "vim" || name == "nvim" || strings.HasPrefix(name, "vim.") {
		return []string{
			"-n", "-i", "NONE",
			"--cmd", "set nomodeline",
			"-c", "set noundofile nobackup nowritebackup noswapfile",
		}
	}
	return nil
}
