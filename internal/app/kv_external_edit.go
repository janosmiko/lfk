package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	m.confirmQuestion = fmt.Sprintf("Write the decoded value of %s to a temp file in plain text? Editor swap or undo files can keep a copy.", key)
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
	dir, path, err := writeEditTempFile(key, value)
	if err != nil {
		return func() tea.Msg {
			return kvExternalEditMsg{overlay: target, key: key, err: err}
		}
	}
	editor := editorCommand()
	args := append([]string{}, editor[1:]...)
	if sensitive {
		args = append(args, noCopyEditorArgs(editor[0])...)
	}
	cmd := exec.Command(editor[0], append(args, path)...)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return finishEditTempFile(dir, path, target, key, value, err)
	})
}

// writeEditTempFile writes value to a 0600 file in a fresh 0700 dir. The
// file is named after the key so the editor picks the right file type.
func writeEditTempFile(key, value string) (dir, path string, err error) {
	dir, err = os.MkdirTemp(editTempRoot(), "lfk-edit-*")
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
func finishEditTempFile(dir, path string, target overlayKind, key, orig string, editorErr error) tea.Msg {
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
// Plain vi is left out: busybox vi and nvi reject -n.
func noCopyEditorArgs(editor string) []string {
	switch filepath.Base(editor) {
	case "vim", "nvim":
		return []string{"-n", "-i", "NONE", "-c", "set noundofile nobackup nowritebackup"}
	}
	return nil
}
