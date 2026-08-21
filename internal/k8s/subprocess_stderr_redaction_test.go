package k8s

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gcpTokenStderr is the shape an exec credential plugin (gke-gcloud-auth-plugin)
// leaves on kubectl's stderr when it fails after minting a token.
const gcpTokenStderr = "ya29.a1b2c3d4e5f6g7h8i9j0ZZTOPSECRETTAILMARKER"

// A kubectl port-forward that dies hands its raw stderr to recordProcessExit,
// which stores it in entry.Error and logs it. That text is untrusted external
// output and must not reach the log file or the overlay unredacted.
func TestPortForwardManager_RedactsProcessExitStderr(t *testing.T) {
	buf := &bytes.Buffer{}
	orig := logger.Logger
	logger.Logger = slog.New(slog.NewTextHandler(buf, nil))
	defer func() { logger.Logger = orig }()

	mgr := NewPortForwardManager()
	entry := &PortForwardEntry{ID: 7, Status: PortForwardRunning}
	stderr := "unable to connect: " + gcpTokenStderr + "\npassword: hunter2"
	mgr.recordProcessExit(entry, errors.New("exit status 1"), stderr)

	require.Equal(t, PortForwardFailed, entry.Status)
	assert.NotContains(t, entry.Error, gcpTokenStderr, "the raw token must not survive in entry.Error")
	assert.NotContains(t, entry.Error, "hunter2", "the raw password must not survive in entry.Error")
	assert.Contains(t, entry.Error, "[REDACTED-GCP-TOKEN]")
	assert.Contains(t, entry.Error, "unable to connect", "the non-secret diagnostic text must be kept")

	logged := buf.String()
	assert.NotContains(t, logged, gcpTokenStderr, "the raw token must not reach the log")
	assert.NotContains(t, logged, "hunter2", "the raw password must not reach the log")
}

// The stderr-less branch falls back to the process error, which is redacted on
// the same path.
func TestPortForwardManager_RedactsProcessExitError(t *testing.T) {
	mgr := NewPortForwardManager()
	entry := &PortForwardEntry{ID: 8, Status: PortForwardRunning}
	mgr.recordProcessExit(entry, errors.New("dial postgres://admin:hunter2@db:5432 failed"), "")

	require.Equal(t, PortForwardFailed, entry.Status)
	assert.NotContains(t, entry.Error, "hunter2")
	assert.Contains(t, entry.Error, "[REDACTED-CREDS]")
}

// classifyCaptureExit folds backend stderr into CaptureEntry.LastError, which
// the traffic-capture overlay renders. Same untrusted source, same rule.
func TestClassifyCaptureExit_RedactsStderr(t *testing.T) {
	status, msg := classifyCaptureExit(
		BackendKubeshark, nil, errors.New("exit status 1"),
		"auth failed: "+gcpTokenStderr, nil)

	require.Equal(t, CaptureFailed, status)
	assert.NotContains(t, msg, gcpTokenStderr, "the raw token must not survive in LastError")
	assert.Contains(t, msg, "[REDACTED-GCP-TOKEN]")
	assert.Contains(t, msg, "auth failed", "the non-secret diagnostic text must be kept")
}

// LastError keeps only the tail of a long stderr. Redaction has to run before
// that trim: trimming first can cut a token's prefix off, leaving a tail that
// no pattern matches and that then gets stored raw.
func TestClassifyCaptureExit_RedactsBeforeTrimmingLongStderr(t *testing.T) {
	// Size the parts so the tail-keeping cut lands 10 bytes into the token,
	// past the "ya29." prefix the pattern needs to recognize it.
	const pad = 100
	const cutInto = 10
	suffix := strings.Repeat("z", pad+cutInto+captureLastErrorMaxLen-pad-len(gcpTokenStderr)-1)
	stderr := strings.Repeat("x", pad) + gcpTokenStderr + " " + suffix
	require.Equal(t, pad+cutInto, len(stderr)-captureLastErrorMaxLen, "the cut must land inside the token")

	status, msg := classifyCaptureExit(BackendKubeshark, nil, errors.New("exit status 1"), stderr, nil)

	require.Equal(t, CaptureFailed, status)
	tail := gcpTokenStderr[10:]
	assert.NotContains(t, msg, tail, "a trim-first order would leave this raw token fragment behind")
	assert.Contains(t, msg, "[REDACTED-GCP-TOKEN]")
}
