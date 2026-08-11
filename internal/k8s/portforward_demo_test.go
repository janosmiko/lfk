package k8s

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// In demo mode KubectlPath returns this process's own executable, so a call
// site that does not wrap its arguments re-enters lfk with kubectl-shaped
// argv. The root command parses "port-forward pod/p 8080:80 --context ..."
// as its own flags and opens a second TUI. Every exec of the kubectl path
// must go through DemoKubectlArgs (TASK-875).
func TestPortForwardStart_RoutesArgsThroughTheDemoHelper(t *testing.T) {
	SetDemoMode(true)
	defer SetDemoMode(false)

	// The fake stands in for the re-exec'd lfk binary: it records the argv it
	// was given, then fails so the manager settles instead of running on.
	dir := t.TempDir()
	fake := filepath.Join(dir, "fake-kubectl")
	argvFile := filepath.Join(dir, "argv")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argvFile + "\nexit 1\n"
	require.NoError(t, os.WriteFile(fake, []byte(script), 0o700))

	mgr := NewPortForwardManager()
	_, err := mgr.Start(fake, "/dev/null", "Pod", "p", "ns", "ctx", "ctx", "8080", "80")
	require.NoError(t, err)

	var argv []string
	require.Eventually(t, func() bool {
		b, readErr := os.ReadFile(argvFile)
		if readErr != nil {
			return false
		}
		argv = strings.Fields(strings.TrimSpace(string(b)))
		return len(argv) > 0
	}, portForwardSettleTimeout, 20*time.Millisecond, "the fake binary should have recorded its arguments")

	require.NotEmpty(t, argv)
	assert.Equal(t, "__demo-kubectl", argv[0],
		"demo mode must route into the demo kubectl, which refuses port-forward by name")
	assert.Equal(t, "port-forward", argv[1], "the kubectl verb must follow the subcommand unchanged")
}

func TestPortForwardStart_LeavesArgsAloneOutsideDemoMode(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "fake-kubectl")
	argvFile := filepath.Join(dir, "argv")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argvFile + "\nexit 1\n"
	require.NoError(t, os.WriteFile(fake, []byte(script), 0o700))

	mgr := NewPortForwardManager()
	_, err := mgr.Start(fake, "/dev/null", "Pod", "p", "ns", "ctx", "ctx", "8080", "80")
	require.NoError(t, err)

	var argv []string
	require.Eventually(t, func() bool {
		b, readErr := os.ReadFile(argvFile)
		if readErr != nil {
			return false
		}
		argv = strings.Fields(strings.TrimSpace(string(b)))
		return len(argv) > 0
	}, portForwardSettleTimeout, 20*time.Millisecond)

	require.NotEmpty(t, argv)
	assert.Equal(t, "port-forward", argv[0], "a real kubectl must be called exactly as before")
}
