package ui

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenBrowser opens the given URL in the system's default browser.
// Returns an error if the OS-specific opener can't be invoked or if the
// current OS isn't supported.
func OpenBrowser(url string) error {
	args := browserCommand(runtime.GOOS, url)
	if len(args) == 0 {
		return fmt.Errorf("no browser opener for GOOS=%s; URL: %s", runtime.GOOS, url)
	}
	return exec.Command(args[0], args[1:]...).Start()
}

// browserCommand returns the argv for opening a URL on the given GOOS.
// Returns nil for unsupported OSes.
//
// Windows uses rundll32 url.dll,FileProtocolHandler rather than `cmd /c start`
// because the latter re-parses the URL via cmd.exe metacharacter semantics
// (`&`, `|`, `^`, `%`) — a single `&` in a URL turns the rest into a separate
// command that cmd.exe will execute.
func browserCommand(goos, url string) []string {
	switch goos {
	case "darwin":
		return []string{"open", url}
	case "linux":
		return []string{"xdg-open", url}
	case "windows":
		return []string{"rundll32", "url.dll,FileProtocolHandler", url}
	default:
		return nil
	}
}
