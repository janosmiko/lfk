package k8s

import (
	"os"
	"os/exec"
)

// demoMode is set once at startup by SetDemoMode when --demo is passed. It
// defaults to off so every non-demo run resolves a real kubectl exactly as
// before.
var demoMode bool

// SetDemoMode turns demo-mode kubectl resolution on or off. Call it once at
// startup, before any KubectlPath call, when the --demo flag is set.
func SetDemoMode(on bool) {
	demoMode = on
}

// DemoModeEnabled reports whether demo-mode kubectl resolution is active.
func DemoModeEnabled() bool {
	return demoMode
}

// KubectlPath resolves the kubectl binary every call site should invoke.
// In demo mode it returns this process's own executable, so every call site
// re-enters the __demo-kubectl subcommand instead of a kubectl on the user's
// PATH — this takes priority over KUBECTL_BIN so no code path can reach a
// real cluster while --demo is set.
// Outside demo mode, KUBECTL_BIN overrides PATH lookup unconditionally (used
// by tests); when unset it falls back to exec.LookPath("kubectl").
func KubectlPath() (string, error) {
	if demoMode {
		return os.Executable()
	}
	if v := os.Getenv("KUBECTL_BIN"); v != "" {
		return v, nil
	}
	return exec.LookPath("kubectl")
}

// DemoKubectlArgs prepends the hidden __demo-kubectl subcommand name to args
// when demo-mode kubectl resolution is active. KubectlPath returns this
// process's own executable in that mode, but the re-exec'd binary is still a
// full lfk CLI: kubectl-shaped argv (e.g. "edit pod foo --context demo")
// would otherwise be parsed by the root command itself (matching its own
// --context/--namespace flags) and launch a second TUI instead of routing
// into the demo kubectl emulation in internal/democli. Every call site that
// builds an exec.Command(kubectlPath, ...) must wrap its args with this.
func DemoKubectlArgs(args []string) []string {
	if !demoMode {
		return args
	}
	return append([]string{"__demo-kubectl"}, args...)
}
