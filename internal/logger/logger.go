package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	// Logger is the package-level structured logger.
	// It defaults to a no-op (discard) logger so callers are safe before Init.
	Logger *slog.Logger

	logFile *os.File
	once    sync.Once
)

func init() {
	Logger = slog.New(slog.NewJSONHandler(io.Discard, nil))
}

// Init opens (or creates) the log file at logPath, sets up a JSON handler,
// and assigns the package-level Logger. If logPath is empty the default
// ~/.local/share/lfk/lfk.log is used. Parent directories are created as needed.
func Init(logPath string) error {
	if logPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		logPath = filepath.Join(home, ".local", "share", "lfk", "lfk.log")
	}

	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}

	logFile = f
	Logger = slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	return nil
}

// Close flushes and closes the log file. Safe to call multiple times.
func Close() {
	once.Do(func() {
		if logFile != nil {
			_ = logFile.Close()
		}
	})
}

// Info logs at INFO level.
func Info(msg string, args ...any) {
	Logger.Info(msg, args...)
}

// Error logs at ERROR level.
func Error(msg string, args ...any) {
	Logger.Error(msg, args...)
}

// Debug logs at DEBUG level.
func Debug(msg string, args ...any) {
	Logger.Debug(msg, args...)
}

// Warn logs at WARN level.
func Warn(msg string, args ...any) {
	Logger.Warn(msg, args...)
}

// klogWriter is an io.Writer that forwards klog output to our structured logger.
type klogWriter struct{}

func (klogWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(p))
	if msg != "" {
		Logger.Warn(Redact(msg), "source", "klog")
	}
	return len(p), nil
}

// KlogWriter returns an io.Writer that forwards output to the application logger.
func KlogWriter() io.Writer {
	return klogWriter{}
}

// StderrCapture intercepts writes to os.Stderr via an os.Pipe and logs them.
// This captures output from exec credential plugins (e.g., AWS SSO errors)
// that write directly to stderr and would otherwise corrupt the TUI.
type StderrCapture struct {
	r       *os.File
	w       *os.File
	done    chan struct{}
	MsgChan chan string // buffered channel for captured messages
}

// NewStderrCapture creates a pipe-based stderr capture. The write end can
// replace os.Stderr; a background goroutine reads from the pipe and logs
// each line via the application logger.
func NewStderrCapture() *StderrCapture {
	r, w, err := os.Pipe()
	if err != nil {
		// If we can't create the pipe, return a no-op capture.
		return &StderrCapture{done: make(chan struct{})}
	}
	sc := &StderrCapture{r: r, w: w, done: make(chan struct{}), MsgChan: make(chan string, 32)}
	go sc.readLoop()
	return sc
}

// Writer returns the write end of the pipe, suitable for assigning to os.Stderr.
func (sc *StderrCapture) Writer() *os.File {
	if sc.w != nil {
		return sc.w
	}
	// Fallback: return a file that discards (shouldn't happen).
	f, _ := os.Open(os.DevNull)
	return f
}

// Close shuts down the capture. Call after restoring the original os.Stderr.
func (sc *StderrCapture) Close() {
	if sc.w != nil {
		_ = sc.w.Close()
	}
	if sc.done != nil {
		<-sc.done
	}
	if sc.r != nil {
		_ = sc.r.Close()
	}
}

func (sc *StderrCapture) readLoop() {
	defer close(sc.done)
	if sc.r == nil {
		return
	}
	buf := make([]byte, 4096)
	for {
		n, err := sc.r.Read(buf)
		if n > 0 {
			msg := strings.TrimSpace(string(buf[:n]))
			if msg != "" {
				// Redact tokens/credentials before logging or surfacing to the
				// TUI. Exec credential plugins (AWS SSO, gke-gcloud-auth-plugin,
				// OIDC helpers) routinely emit short-lived tokens to stderr,
				// and the TUI message also flows through setStatusMessage which
				// re-logs it — so we must redact at the source.
				redacted := Redact(msg)
				Logger.Error(redacted, "source", "stderr")
				select {
				case sc.MsgChan <- redacted:
				default:
					// Channel full, drop the message (it's still logged).
				}
			}
		}
		if err != nil {
			return
		}
	}
}
