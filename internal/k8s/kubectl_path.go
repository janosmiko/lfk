package k8s

import (
	"os"
	"os/exec"
)

// KubectlPath resolves the kubectl binary every call site should invoke.
// KUBECTL_BIN overrides PATH lookup unconditionally (used by tests and by
// demo mode to point at a stub instead of a real cluster-connected
// kubectl); when unset it falls back to exec.LookPath("kubectl").
func KubectlPath() (string, error) {
	if v := os.Getenv("KUBECTL_BIN"); v != "" {
		return v, nil
	}
	return exec.LookPath("kubectl")
}
