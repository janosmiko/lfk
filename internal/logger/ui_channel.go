package logger

import "time"

// UIEntry is one log line surfaced to the in-app log overlay.
type UIEntry struct {
	Time    time.Time
	Level   string // "WRN" / "ERR" / "INF"
	Message string
	Args    []any // slog-style key/value pairs as passed to the emitter
}

// uiChan is the buffered channel of log events the app's Bubbletea loop
// subscribes to. Buffered + non-blocking send: emit-path never stalls,
// and the in-app overlay can miss extreme bursts without breaking the
// underlying file logger.
var uiChan = make(chan UIEntry, 64)

// UIChan returns the read end of the in-app log channel. The app
// subscribes to it via a tea.Cmd that reads one entry and re-arms,
// mirroring the StderrCapture pattern.
func UIChan() <-chan UIEntry {
	return uiChan
}

// publishUI sends to the UI channel without blocking. Drops on full;
// the on-disk log already has the line, and dropping is preferable to
// stalling the emit path (or growing the channel unbounded in the
// presence of a wedged UI).
func publishUI(level, msg string, args []any) {
	e := UIEntry{Time: time.Now(), Level: level, Message: msg, Args: args}
	select {
	case uiChan <- e:
	default:
	}
}
