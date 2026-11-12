package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

// externalEditSecretAction is the pendingAction label that routes the
// plain-text warning back to the Secret editor.
const externalEditSecretAction = "Edit Secret Value"

// kvExternalEditMsg carries a value edited in $EDITOR back to the K/V
// editor that opened it.
type kvExternalEditMsg struct {
	// owner is the *model.SecretData or *model.ConfigMapData that was open
	// when the editor started. A reload swaps the pointer, so a late result
	// cannot land in another resource that has the same key.
	owner      any
	key        string
	value      string
	err        error
	cleanupErr error
	// quick marks an editor that returned within a second, which is how a
	// GUI editor without its wait flag behaves.
	quick bool
}

// refuseBinaryExternalEdit blocks values a text editor can re-encode on
// save, which would corrupt them without a visible change.
func (m Model) refuseBinaryExternalEdit(key, value string) (Model, bool) {
	if utf8.ValidString(value) && !strings.ContainsRune(value, 0) {
		return m, false
	}
	m.setStatusMessage(key+" is binary, not opened in $EDITOR", true)
	return m, true
}

func (m Model) beginSecretExternalEdit() (tea.Model, tea.Cmd) {
	visible := m.secretVisibleKeys()
	if m.secretCursor < 0 || m.secretCursor >= len(visible) {
		return m, nil
	}
	key := visible[m.secretCursor]
	if m, refused := m.refuseBinaryExternalEdit(key, m.secretData.Data[key]); refused {
		return m, scheduleStatusClear()
	}
	m.confirmAction = key
	m.pendingAction = externalEditSecretAction
	m.confirmTitle = "Edit in $EDITOR"
	where := "on disk"
	if editTempRoot() != "" {
		where = "in /dev/shm (memory that can swap to disk)"
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
	return m, editValueInEditor(m.secretData, key, value, true)
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
	if m, refused := m.refuseBinaryExternalEdit(key, m.configMapData.Data[key]); refused {
		return m, scheduleStatusClear()
	}
	return m, editValueInEditor(m.configMapData, key, m.configMapData.Data[key], false)
}

// updateKVExternalEdit stores the edited value as a pending change. The
// user saves it with Enter, like an inline edit.
func (m Model) updateKVExternalEdit(msg kvExternalEditMsg) (tea.Model, tea.Cmd) {
	m = m.applyKVExternalEdit(msg)
	if msg.cleanupErr != nil {
		m.setStatusMessage("Temp file not removed, delete it by hand: "+msg.cleanupErr.Error(), true)
	}
	return m, scheduleStatusClear()
}

func (m Model) applyKVExternalEdit(msg kvExternalEditMsg) Model {
	if msg.err != nil {
		m.setStatusMessage("Edit in $EDITOR failed: "+msg.err.Error(), true)
		return m
	}
	var data map[string]string
	switch {
	case m.overlay == overlaySecretEditor && m.secretData != nil && msg.owner == any(m.secretData):
		data = m.secretData.Data
	case m.overlay == overlayConfigMapEditor && m.configMapData != nil && msg.owner == any(m.configMapData):
		data = m.configMapData.Data
	}
	old, ok := data[msg.key]
	switch {
	case !ok:
		m.setStatusMessage("Editor closed, reloaded or key removed, edit of "+msg.key+" discarded", true)
	case old == msg.value && msg.quick:
		m.setStatusMessage("No changes to "+msg.key+". A GUI editor needs its wait flag, like code --wait", false)
	case old == msg.value:
		m.setStatusMessage("No changes to "+msg.key, false)
	default:
		data[msg.key] = msg.value
		m.setStatusMessage("Updated "+msg.key+", press enter to save", false)
	}
	return m
}

// editValueInEditor opens value in the user's editor and returns the
// result as a kvExternalEditMsg. sensitive turns off editor copies
// (swap, undo, backup) for editors that support it.
func editValueInEditor(owner any, key, value string, sensitive bool) tea.Cmd {
	e := &valueEdit{base: kvExternalEditMsg{owner: owner, key: key}, orig: value, sensitive: sensitive}
	return tea.Exec(e, e.result)
}

// result covers bubbletea failing to release the terminal, in which
// case Run never ran and e.msg is empty.
func (e *valueEdit) result(err error) tea.Msg {
	if e.msg == (kvExternalEditMsg{}) {
		msg := e.base
		msg.err = err
		return msg
	}
	return e.msg
}

// valueEdit writes the temp file only when bubbletea runs the editor, so
// a command that never runs leaves no plain-text file behind.
type valueEdit struct {
	base           kvExternalEditMsg
	orig           string
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
	sweepStaleEditDirs(root)
	dir, path, err := writeEditTempFile(root, e.base.key, e.orig)
	if err != nil {
		e.msg = e.base
		e.msg.err = err
		return err
	}
	argv := editorArgv(editorCommand(), path, e.sensitive)
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = e.stdin, e.stdout, e.stderr
	start := time.Now()
	editorErr := cmd.Run()
	e.msg = finishEditTempFile(dir, path, e.base, e.orig, editorErr)
	e.msg.quick = time.Since(start) < time.Second
	return nil
}

// sweepStaleEditDirsCmd runs the sweep at startup, so a value left by a
// killed lfk does not wait for the next external edit.
func sweepStaleEditDirsCmd(root string) tea.Cmd {
	return func() tea.Msg {
		sweepStaleEditDirs(root)
		return nil
	}
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

const editDirPrefix = "lfk-edit-"

// sweepStaleEditDirs removes temp dirs left behind when lfk was killed
// while the editor was open. A dir name holds the pid of the lfk that
// made it, and a dir is kept while that process is alive.
func sweepStaleEditDirs(root string) {
	if root == "" {
		root = os.TempDir()
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, ent := range entries {
		rest, ok := strings.CutPrefix(ent.Name(), editDirPrefix)
		if !ok || !ent.IsDir() {
			continue
		}
		pidStr, _, _ := strings.Cut(rest, "-")
		if pid, err := strconv.Atoi(pidStr); err == nil && !processAlive(pid) {
			_ = os.RemoveAll(filepath.Join(root, ent.Name()))
		}
	}
}

// processAlive errs toward true, so a reused pid only keeps a stale dir.
func processAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	defer func() { _ = p.Release() }()
	if runtime.GOOS == "windows" {
		return true // FindProcess fails on Windows when the process is gone.
	}
	err = p.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}

// writeEditTempFile writes value to a 0600 file in a fresh 0700 dir. The
// file is named after the key so the editor picks the right file type.
func writeEditTempFile(root, key, value string) (dir, path string, err error) {
	dir, err = os.MkdirTemp(root, fmt.Sprintf("%s%d-*", editDirPrefix, os.Getpid()))
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
func finishEditTempFile(dir, path string, base kvExternalEditMsg, orig string, editorErr error) (msg kvExternalEditMsg) {
	msg = base
	defer func() { msg.cleanupErr = os.RemoveAll(dir) }()
	if editorErr != nil {
		msg.err = fmt.Errorf("editor: %w", editorErr)
		return msg
	}
	b, err := os.ReadFile(path)
	if err != nil {
		msg.err = fmt.Errorf("reading temp file: %w", err)
		return msg
	}
	msg.value = keepTrailingNewline(orig, string(b))
	return msg
}

// editTempRoot prefers /dev/shm (tmpfs on Linux), which keeps the value
// off disk unless the system swaps. Other systems use the per-user temp dir.
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
