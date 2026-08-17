package democli

import (
	"strconv"
	"strings"
)

// logArgs is the subset of `kubectl logs` flags the app actually passes
// (see internal/app/commands_logs.go, previewlog.go, logshared.go).
type logArgs struct {
	target     string // pod name, "kind/name" resource ref, or the -l selector value
	isSelector bool
	namespace  string
	kctx       string
	container  string
	follow     bool
	prefix     bool
	tail       int // -1 means unset (no --tail flag was passed)
}

// parseLogArgs reads kubectl-style argv (flags in "--flag value" or
// "--flag=value" form, one leading positional resource ref/pod name) into a
// logArgs. Unrecognized flags are accepted and ignored — this parser only
// needs to understand what the app's call sites actually emit.
func parseLogArgs(args []string) logArgs {
	a := logArgs{tail: -1}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-f":
			a.follow = true
		case arg == "--prefix":
			a.prefix = true
		case arg == "-l":
			i++
			if i < len(args) {
				a.target, a.isSelector = args[i], true
			}
		case strings.HasPrefix(arg, "-l="):
			a.target, a.isSelector = strings.TrimPrefix(arg, "-l="), true
		case arg == "-n":
			i++
			if i < len(args) {
				a.namespace = args[i]
			}
		case strings.HasPrefix(arg, "-n="):
			a.namespace = strings.TrimPrefix(arg, "-n=")
		case arg == "--context":
			i++
			if i < len(args) {
				a.kctx = args[i]
			}
		case strings.HasPrefix(arg, "--context="):
			a.kctx = strings.TrimPrefix(arg, "--context=")
		case arg == "-c":
			i++
			if i < len(args) {
				a.container = args[i]
			}
		case strings.HasPrefix(arg, "-c="):
			a.container = strings.TrimPrefix(arg, "-c=")
		case strings.HasPrefix(arg, "--tail="):
			if n, err := strconv.Atoi(strings.TrimPrefix(arg, "--tail=")); err == nil {
				a.tail = n
			}
		case strings.HasPrefix(arg, "-"):
			// Accepted no-ops: --all-containers=true, --max-log-requests=N,
			// --ignore-errors, --timestamps, --previous.
		default:
			if a.target == "" {
				a.target = arg
			}
		}
	}
	return a
}

// podName derives a stable, readable pod identity from the parsed target.
// A literal pod name is used as-is, a "kind/name" resource ref contributes
// its name, and a label selector ("app=web,tier=x") contributes the first
// label's value. Different targets deterministically produce different
// identities, which is what seeds a different (but reproducible) log stream
// per pod.
func (a logArgs) podName() string {
	if a.target == "" {
		return "demo-pod"
	}
	if !a.isSelector {
		if _, name, ok := strings.Cut(a.target, "/"); ok {
			return name
		}
		return a.target
	}
	first := a.target
	if idx := strings.IndexByte(first, ','); idx >= 0 {
		first = first[:idx]
	}
	if _, val, ok := strings.Cut(first, "="); ok {
		return val
	}
	return first
}
