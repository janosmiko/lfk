package ui

import (
	"reflect"
	"testing"
)

func TestBrowserCommand_macOS(t *testing.T) {
	got := browserCommand("darwin", "https://example.com")
	want := []string{"open", "https://example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestBrowserCommand_Linux(t *testing.T) {
	got := browserCommand("linux", "https://example.com")
	want := []string{"xdg-open", "https://example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestBrowserCommand_Windows(t *testing.T) {
	// rundll32 url.dll,FileProtocolHandler does not invoke cmd.exe and so
	// avoids `cmd /c start <url>` re-parsing the URL with shell metacharacter
	// semantics (`&`, `|`, `^`, `%`).
	got := browserCommand("windows", "https://example.com")
	want := []string{"rundll32", "url.dll,FileProtocolHandler", "https://example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestBrowserCommand_Windows_NoShellInvocation guards against regressions back
// to `cmd /c start`, which is unsafe because cmd.exe re-parses everything
// after `start` and treats `&` / `|` / `^` as command separators.
func TestBrowserCommand_Windows_NoShellInvocation(t *testing.T) {
	got := browserCommand("windows", "https://example.com/?a=1&b=2")
	if len(got) == 0 {
		t.Fatal("browserCommand returned empty argv")
	}
	if got[0] == "cmd" || got[0] == "cmd.exe" {
		t.Errorf("argv[0] = %q, must not invoke cmd.exe", got[0])
	}
}

func TestBrowserCommand_Unknown(t *testing.T) {
	got := browserCommand("plan9", "https://example.com")
	if got != nil {
		t.Errorf("got %v, want nil for unknown OS", got)
	}
}
