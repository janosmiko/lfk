// Package democli implements the hidden "__demo-kubectl" subcommand that
// --demo mode re-enters instead of a real kubectl binary (see
// k8s.KubectlPath). Every existing kubectl call site in the app keeps
// running unmodified; only the resolved binary path changes, so this
// package must accept the same argument shapes the app already passes.
package democli

import (
	"context"
	"fmt"
	"io"
)

// Run dispatches args (the kubectl-style argv, verb first) to the matching
// verb handler. Verbs the app never needs to serve in demo mode (exec,
// port-forward, drain, debug, apply, and anything else unimplemented) print
// a single clear refusal line and return a non-nil error so the caller exits
// non-zero — never falling through to a real binary.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "__demo-kubectl: no kubectl verb given") //nolint:errcheck
		return fmt.Errorf("no kubectl verb given")
	}

	verb, rest := args[0], args[1:]
	switch verb {
	case "logs":
		return runLogs(ctx, rest, stdout)
	case "get":
		return runGet(rest, stdout)
	case "describe":
		return runDescribe(rest, stdout)
	default:
		msg := fmt.Sprintf("%s is not available in demo mode", verb)
		fmt.Fprintln(stderr, msg) //nolint:errcheck
		return fmt.Errorf("%s", msg)
	}
}
