package democli

import (
	"fmt"
	"io"
	"strings"

	"github.com/janosmiko/lfk/internal/k8s/demo"
)

// runDescribe emulates `kubectl describe pod <name> -n <ns> --context <ctx>`
// (internal/k8s/describe_pod.go), rendering plain-text describe output from
// the demo seed data in internal/k8s/demo.
func runDescribe(args []string, stdout io.Writer) error {
	var kind, name, namespace, kctx string
	var positional []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-n":
			i++
			if i < len(args) {
				namespace = args[i]
			}
		case "--context":
			i++
			if i < len(args) {
				kctx = args[i]
			}
		default:
			if !strings.HasPrefix(args[i], "-") {
				positional = append(positional, args[i])
			}
		}
	}
	if len(positional) > 0 {
		kind = positional[0]
	}
	if len(positional) > 1 {
		name = positional[1]
	}

	_, err := io.WriteString(stdout, renderDescribe(kind, name, namespace, kctx))
	return err
}

// renderDescribe builds a plain-text kubectl-describe-shaped body for the
// known demo pods, falling back to a minimal generic body for anything else
// (namespace/context are still echoed so the caller sees the object it
// asked about, rather than an error).
func renderDescribe(kind, name, namespace, kctx string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Name:         %s\n", name)
	fmt.Fprintf(&b, "Namespace:    %s\n", namespace)
	fmt.Fprintf(&b, "Context:      %s\n", kctx)

	switch name {
	case demo.PodWebHealthy1, demo.PodWebHealthy2:
		fmt.Fprintf(&b, "Node:         %s\n", demo.NodeWorker1)
		b.WriteString("Status:       Running\n")
		b.WriteString("Containers:\n  web:\n    Image:          ghcr.io/example/web:1.4.2\n" +
			"    State:          Running\n    Ready:          True\n    Restart Count:  0\n")
		b.WriteString("Conditions:\n  Ready  True\n")
	case demo.PodWebCrashLoop:
		fmt.Fprintf(&b, "Node:         %s\n", demo.NodeWorker1)
		b.WriteString("Status:       Running\n")
		b.WriteString("Containers:\n  web:\n    Image:          ghcr.io/example/web:1.4.2\n" +
			"    State:          Waiting\n    Reason:         CrashLoopBackOff\n" +
			"    Last State:     Terminated\n    Reason:         Error\n    Exit Code:      1\n" +
			"    Restart Count:  7\n")
		b.WriteString("Conditions:\n  Ready  False\n")
		b.WriteString("Events:\n  Type     Reason   Age   Message\n  ----     ------   ---   -------\n" +
			"  Warning  BackOff  2m    Back-off restarting failed container=web\n")
	case demo.PodDBMigrate:
		fmt.Fprintf(&b, "Node:         %s\n", demo.NodeWorker1)
		b.WriteString("Status:       Succeeded\n")
		b.WriteString("Containers:\n  migrate:\n    State:          Terminated\n" +
			"    Reason:         Completed\n    Exit Code:      0\n")
	default:
		fmt.Fprintf(&b, "Kind:         %s\n", kind)
		b.WriteString("Status:       Running\n")
		b.WriteString("(no demo seed data for this object; showing a generic description)\n")
	}

	return b.String()
}
