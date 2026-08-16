package democli

import (
	"fmt"
	"io"
	"strings"
)

// runGet dispatches by shape: the CronJob log path in
// internal/app/commands_logs_cronjob.go issues "get cronjob NAME", "get
// jobs", and "get pods -l SELECTOR", answered by get_cronjob.go.
func runGet(args []string, stdout io.Writer) error {
	positional, selector, hasSelector := parseGetArgs(args)

	switch {
	case len(positional) >= 1 && positional[0] == "cronjob":
		name := ""
		if len(positional) > 1 {
			name = positional[1]
		}
		return writeCronJobUID(stdout, name)
	case len(positional) >= 1 && positional[0] == "jobs":
		return writeOwnedJobsList(stdout)
	case len(positional) >= 1 && positional[0] == "pods" && hasSelector:
		return writePodNamesForSelector(stdout, selector)
	default:
		return runGetResourceSelector(positional, stdout)
	}
}

// parseGetArgs collects every positional argument (in order) and the "-l"
// selector value, if any. It skips the values of flags this stub never
// inspects otherwise (-n, --context, -o).
func parseGetArgs(args []string) (positional []string, selector string, hasSelector bool) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-n", "--context", "-o":
			i++ // skip the flag's value, unused by this stub
		case "-l":
			i++
			if i < len(args) {
				selector, hasSelector = args[i], true
			}
		default:
			if !strings.HasPrefix(args[i], "-") {
				positional = append(positional, args[i])
			}
		}
	}
	return positional, selector, hasSelector
}

// runGetResourceSelector answers kubectlGetPodSelector's `get <kind>/<name>
// -o json` (internal/app/commands_logs.go) with a synthetic selector.
func runGetResourceSelector(positional []string, stdout io.Writer) error {
	var resourceRef string
	if len(positional) > 0 {
		resourceRef = positional[0]
	}

	kind, name, ok := strings.Cut(resourceRef, "/")
	if !ok {
		name = resourceRef
	}

	var body string
	if strings.EqualFold(kind, "service") {
		// Service selectors are a flat map, not matchLabels-wrapped.
		body = fmt.Sprintf(`{"spec":{"selector":{"app":%q}}}`, name)
	} else {
		body = fmt.Sprintf(`{"spec":{"selector":{"matchLabels":{"app":%q}}}}`, name)
	}

	_, err := fmt.Fprintln(stdout, body)
	return err
}
