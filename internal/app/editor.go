package app

import (
	"os"
	"runtime"

	"github.com/google/shlex"
)

// editorCommand resolves the user's preferred editor following kubectl's
// convention: KUBE_EDITOR takes precedence over EDITOR, with a
// platform-specific fallback (notepad on Windows, vi elsewhere).
//
// The returned slice is split with shell-style quoting so values like
// "code --wait" or "vim -p" pass flags correctly. The first element is
// the binary to exec; remaining elements are args to prepend before the
// caller's file argument.
//
// On parse failure (unbalanced quotes), falls back to the platform
// default so the editor still launches.
func editorCommand() []string {
	raw := os.Getenv("KUBE_EDITOR")
	if raw == "" {
		raw = os.Getenv("EDITOR")
	}
	if raw == "" {
		return []string{platformDefaultEditor()}
	}
	parts, err := shlex.Split(raw)
	if err != nil || len(parts) == 0 {
		return []string{platformDefaultEditor()}
	}
	return parts
}

func platformDefaultEditor() string {
	if runtime.GOOS == "windows" {
		return "notepad"
	}
	return "vi"
}
