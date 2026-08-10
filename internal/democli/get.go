package democli

import (
	"fmt"
	"io"
	"strings"
)

// runGet emulates `kubectl get <kind>/<name> -n <ns> --context <ctx> -o
// json`, the only "get" shape the app issues (kubectlGetPodSelector in
// internal/app/commands_logs.go, to discover a parent resource's pod
// selector before following its logs). It returns a minimal JSON object
// carrying a synthetic selector derived from the resource name, which is
// enough for the caller to extract a usable "-l" value — the exact label set
// need not match real cluster data since demo log generation does not filter
// by it.
func runGet(args []string, stdout io.Writer) error {
	var resourceRef string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-n", "--context", "-o":
			i++ // skip the flag's value; unused by this stub
		default:
			if !strings.HasPrefix(args[i], "-") && resourceRef == "" {
				resourceRef = args[i]
			}
		}
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
