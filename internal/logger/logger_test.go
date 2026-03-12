package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetLogger resets the package-level logger state for testing.
func resetLogger() {
	if logFile != nil {
		logFile.Close()
	}
	logFile = nil
	once = sync.Once{}
	Logger = slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func TestDefaultLoggerIsNoop(t *testing.T) {
	defer resetLogger()
	// Before Init, the logger should exist and not panic.
	assert.NotNil(t, Logger)
	// Should safely discard output.
	Logger.Info("test message")
	Logger.Error("test error")
	Logger.Debug("test debug")
}

func TestInit(t *testing.T) {
	defer resetLogger()

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	resetLogger()
	err := Init(logPath)
	require.NoError(t, err)

	// Verify log file was created.
	_, err = os.Stat(logPath)
	assert.NoError(t, err)

	// Verify logger writes to file.
	Info("test init message")
	data, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "test init message")
}

func TestInitCreatesDirectories(t *testing.T) {
	defer resetLogger()

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "deep", "nested", "dir", "test.log")

	resetLogger()
	err := Init(logPath)
	require.NoError(t, err)

	_, err = os.Stat(logPath)
	assert.NoError(t, err)
}

func TestKlogWriter(t *testing.T) {
	w := KlogWriter()
	assert.NotNil(t, w)

	// Should implement io.Writer and not panic.
	n, err := w.Write([]byte("test klog message\n"))
	assert.NoError(t, err)
	assert.Greater(t, n, 0)
}

func TestStderrCapture(t *testing.T) {
	sc := NewStderrCapture()
	require.NotNil(t, sc)
	require.NotNil(t, sc.MsgChan)

	// Writer should return a valid file.
	w := sc.Writer()
	assert.NotNil(t, w)

	// Write a message to the capture.
	msg := "captured stderr message"
	_, err := w.Write([]byte(msg))
	assert.NoError(t, err)

	// Read the message from the channel with timeout.
	select {
	case received := <-sc.MsgChan:
		assert.Equal(t, msg, received)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for captured message")
	}

	// Close should not panic.
	sc.Close()
}

func TestStderrCaptureMultipleMessages(t *testing.T) {
	sc := NewStderrCapture()
	require.NotNil(t, sc)

	w := sc.Writer()

	// Write multiple messages with delays to ensure they are separate reads.
	for i := 0; i < 5; i++ {
		_, err := w.Write([]byte("msg"))
		assert.NoError(t, err)
		time.Sleep(10 * time.Millisecond)
	}

	// Read messages.
	count := 0
	timeout := time.After(2 * time.Second)
loop:
	for count < 5 {
		select {
		case <-sc.MsgChan:
			count++
		case <-timeout:
			break loop
		}
	}
	assert.GreaterOrEqual(t, count, 1)

	sc.Close()
}

func TestStderrCaptureEmpty(t *testing.T) {
	sc := NewStderrCapture()
	require.NotNil(t, sc)

	// Writing whitespace-only content should not send a message.
	w := sc.Writer()
	_, err := w.Write([]byte("   \n"))
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	select {
	case msg := <-sc.MsgChan:
		t.Fatalf("unexpected message for whitespace-only input: %q", msg)
	default:
		// Expected: no message for whitespace.
	}

	sc.Close()
}

func TestLogHelpers(t *testing.T) {
	defer resetLogger()

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "helpers.log")

	resetLogger()
	err := Init(logPath)
	require.NoError(t, err)

	// All helpers should write without panicking.
	Info("info msg", "key", "val")
	Error("error msg", "code", 42)
	Debug("debug msg")
	Warn("warn msg", "detail", "something")

	data, err := os.ReadFile(logPath)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "info msg")
	assert.Contains(t, content, "error msg")
	assert.Contains(t, content, "debug msg")
	assert.Contains(t, content, "warn msg")
}
