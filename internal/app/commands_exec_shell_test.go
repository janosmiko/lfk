package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateExecPodOSResolved verifies the resolved OS is recorded on the
// action context and an exec command is launched.
func TestUpdateExecPodOSResolved(t *testing.T) {
	t.Setenv("PATH", "/nonexistent") // make execKubectlExec return its error cmd (no kubectl)
	m := basePush80v3Model()
	m.actionCtx = actionContext{name: "win-pod", namespace: "default", context: "test-ctx", containerName: "app"}

	mm, cmd := m.updateExecPodOSResolved(execPodOSResolvedMsg{podOS: "windows"})
	m = mm.(Model)
	assert.Equal(t, "windows", m.actionCtx.os, "resolved OS must be recorded for shell selection")
	require.NotNil(t, cmd, "must launch an exec command")
}

func TestExecShellArgs_LinuxWithContainer(t *testing.T) {
	got := execShellArgs("pod-1", "ns", "ctx", "app", "linux")
	assert.Equal(t, []string{
		"exec", "-it", "pod-1", "-n", "ns", "--context", "ctx", "-c", "app",
		"--", "/bin/sh", "-c", linuxExecShellCmd,
	}, got)
}

func TestExecShellArgs_LinuxDefaultWhenUnknown(t *testing.T) {
	// Empty OS (detection failed/unknown) falls back to the Linux shell chain.
	got := execShellArgs("pod-1", "ns", "ctx", "", "")
	assert.Equal(t, []string{
		"exec", "-it", "pod-1", "-n", "ns", "--context", "ctx",
		"--", "/bin/sh", "-c", linuxExecShellCmd,
	}, got)
}

func TestExecShellArgs_Windows(t *testing.T) {
	got := execShellArgs("win-pod", "ns", "ctx", "app", "windows")
	assert.Equal(t, []string{
		"exec", "-it", "win-pod", "-n", "ns", "--context", "ctx", "-c", "app",
		"--", "cmd.exe", "/c", windowsExecShellCmd,
	}, got)
}

func TestExecShellArgs_WindowsCaseInsensitive(t *testing.T) {
	// spec.os.name is lowercase by API contract, but be defensive.
	got := execShellArgs("win-pod", "ns", "ctx", "", "Windows")
	assert.Contains(t, got, "cmd.exe")
	assert.NotContains(t, got, "/bin/sh")
}
