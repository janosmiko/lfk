package ui

import (
	"bytes"
	"fmt"
	"os"
	"reflect"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// colorModeDarkSeq and colorModeLightSeq are the CSI ?997 sequences the
// terminal sends in response to a CSI ?996n query or a CSI ?2031h
// color-palette-update notification.
var (
	colorModeDarkSeq  = []byte("\x1b[?997;1n")
	colorModeLightSeq = []byte("\x1b[?997;2n")
)

// ParseColorModeMsg inspects an arbitrary bubbletea message to determine
// whether it is an unrecognized CSI sequence carrying a CSI ?997 dark/light
// color-mode report (Color Palette Update Notifications protocol).
//
// bubbletea wraps unknown CSI sequences in an unexported named []byte type
// (unknownCSISequenceMsg). We use reflection to reach the raw bytes and match
// against the known sequences without importing that private type.
// A named-type check (Name() != "") ensures plain []byte values are ignored.
//
// Returns (dark=true, true) for a dark-mode report, (dark=false, true) for a
// light-mode report, and (_, false) when msg is unrelated.
func ParseColorModeMsg(msg tea.Msg) (dark bool, ok bool) {
	rv := reflect.ValueOf(msg)
	rt := reflect.TypeOf(msg)
	// Require a named slice-of-bytes (like unknownCSISequenceMsg), not plain []byte.
	if rv.Kind() != reflect.Slice ||
		rt.Elem().Kind() != reflect.Uint8 ||
		rt.Name() == "" {
		return false, false
	}
	b := rv.Bytes()
	switch {
	case bytes.Equal(b, colorModeDarkSeq):
		return true, true
	case bytes.Equal(b, colorModeLightSeq):
		return false, true
	}
	return false, false
}

// EnableColorModeCmd returns a tea.Cmd that subscribes to Color Palette Update
// Notifications (CSI ?2031h) and queries the current preference (CSI ?996n).
// The terminal responds with CSI ?997;1n (dark) or CSI ?997;2n (light), which
// bubbletea delivers as an unknownCSISequenceMsg parsed by ParseColorModeMsg.
func EnableColorModeCmd() tea.Cmd {
	return func() tea.Msg {
		_, _ = fmt.Fprint(os.Stdout, ansi.SetModeLightDark+ansi.RequestLightDarkReport)
		return nil
	}
}

// DisableColorModeNotifications unsubscribes from Color Palette Update
// Notifications by sending CSI ?2031l. Call this after the bubbletea program
// exits so the terminal stops sending unsolicited CSI ?997 reports.
func DisableColorModeNotifications() {
	_, _ = fmt.Fprint(os.Stdout, ansi.ResetModeLightDark)
}

// SetColorMode applies ConfigDarkColorscheme (dark=true) or
// ConfigLightColorscheme (dark=false) as the active color scheme.
// No-op when the relevant scheme is not configured or is not a known built-in.
func SetColorMode(dark bool) {
	var schemeName string
	if dark && ConfigDarkColorscheme != "" {
		schemeName = ConfigDarkColorscheme
	} else if !dark && ConfigLightColorscheme != "" {
		schemeName = ConfigLightColorscheme
	}
	if schemeName == "" {
		return
	}
	if scheme, ok := BuiltinSchemes()[schemeName]; ok {
		ActiveSchemeName = schemeName
		ApplyTheme(scheme)
	}
}

// ColorModeEnabled reports whether Color Palette Update Notifications are
// configured (i.e. at least one of the dark/light schemes is set).
func ColorModeEnabled() bool {
	return ConfigDarkColorscheme != "" || ConfigLightColorscheme != ""
}
