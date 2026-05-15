package app

import (
	"strings"
)

// commandAffectsNamespaces reports whether the kubectl invocation (passed
// without the "kubectl"/"k" prefix) looks like it mutates the cluster's
// Namespace list. Matches `create|delete|replace (ns|namespace|namespaces) ...`.
// Read-only verbs (`get`, `describe`) are excluded so they don't force
// an unnecessary cache refresh, and `apply -f <file>` is not inspected
// (template applies invalidate unconditionally instead). False
// positives only cost one extra GetNamespaces call, so the heuristic
// favours breadth.
func commandAffectsNamespaces(args []string) bool {
	if len(args) < 2 {
		return false
	}
	switch args[0] {
	case "create", "delete", "replace":
	default:
		return false
	}
	for _, a := range args[1:] {
		switch a {
		case "ns", "namespace", "namespaces":
			return true
		}
	}
	return false
}

// injectKubectlDefaults scans the args for --context, -n/--namespace, and
// -A/--all-namespaces flags. If they are not present, it injects the current
// context and namespace from the model. Subcommands that don't accept
// namespace flags (explain, api-resources, version, config, cordon, etc.)
// have only --context injected — injecting -A or -n on those would cause
// kubectl to reject the command with "unknown flag".
func (m *Model) injectKubectlDefaults(args []string) []string {
	hasContext := false
	hasNamespace := false
	hasAllNamespaces := false

	for _, a := range args {
		switch {
		case a == "--context" || strings.HasPrefix(a, "--context="):
			hasContext = true
		case a == "-n" || a == "--namespace" || strings.HasPrefix(a, "--namespace="):
			hasNamespace = true
		case a == "-A" || a == "--all-namespaces":
			hasAllNamespaces = true
		}
	}

	result := make([]string, len(args))
	copy(result, args)

	if !hasContext && m.nav.Context != "" {
		result = append(result, "--context", m.kubectlContext(m.nav.Context))
	}

	if commandSupportsNamespaceFlags(args) && !hasNamespace && !hasAllNamespaces {
		hasResourceNames := positionalArgCount(args) > 2
		ns := m.effectiveNamespace()
		if ns != "" {
			result = append(result, "-n", ns)
		} else if m.allNamespaces {
			if hasResourceNames {
				// kubectl can't use -A with resource names. Look up the namespace
				// of the first named resource from the currently loaded items.
				if foundNS := m.findItemNamespace(args); foundNS != "" {
					result = append(result, "-n", foundNS)
				}
				// If not found, run without -n (kubectl uses kubeconfig default).
			} else {
				result = append(result, "-A")
			}
		}
	}

	return result
}

// commandSupportsNamespaceFlags reports whether the given kubectl subcommand
// accepts -n/--namespace or -A/--all-namespaces flags. Returns false for
// commands that don't query namespaced resources at all: documentation
// (explain, help, options), cluster-wide introspection (api-resources,
// api-versions, version, cluster-info, config), node-scoped operations
// (cordon, uncordon, drain, taint), and offline tooling (kustomize,
// convert, completion, plugin). Injecting -A or -n on these makes kubectl
// reject the whole invocation.
//
// Global flags (e.g. `--context foo`, `--request-timeout=5s`) can appear
// before the verb — kubectl uses Cobra, which parses persistent flags
// from any position — so the lookup must skip leading flag tokens to
// locate the real subcommand. Otherwise `:k --context foo explain pod`
// would still get `-A` injected and fail.
func commandSupportsNamespaceFlags(args []string) bool {
	verb := firstNonFlagToken(args)
	if verb == "" {
		return false
	}
	switch verb {
	case "explain",
		"api-resources", "api-versions",
		"version", "cluster-info",
		"config", "options", "plugin", "completion", "help",
		"cordon", "uncordon", "drain", "taint",
		"kustomize", "convert":
		return false
	}
	return true
}

// kubectlGlobalFlagsWithValue is the set of kubectl global/persistent
// flags whose value is supplied as the next token (e.g. `--context foo`),
// so the parser must consume two tokens when it sees them. The `--flag=value`
// form is handled by HasPrefix below and doesn't need to appear here.
// Source: `kubectl options`.
var kubectlGlobalFlagsWithValue = map[string]bool{
	"--as":                    true,
	"--as-group":              true,
	"--as-uid":                true,
	"--cache-dir":             true,
	"--certificate-authority": true,
	"--client-certificate":    true,
	"--client-key":            true,
	"--cluster":               true,
	"--context":               true,
	"--kubeconfig":            true,
	"--namespace":             true,
	"-n":                      true,
	"--password":              true,
	"--profile":               true,
	"--profile-output":        true,
	"--request-timeout":       true,
	"--server":                true,
	"-s":                      true,
	"--tls-server-name":       true,
	"--token":                 true,
	"--user":                  true,
	"--username":              true,
}

// firstNonFlagToken walks args, skipping leading kubectl global flags
// (and the values of those that consume a separate token), and returns
// the first positional token — typically the subcommand verb. Returns
// "" if every token is a flag or the input is empty.
func firstNonFlagToken(args []string) string {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			return a
		}
		// `--flag=value`: nothing extra to consume.
		if strings.Contains(a, "=") {
			continue
		}
		if kubectlGlobalFlagsWithValue[a] && i+1 < len(args) {
			i++ // skip the value
		}
	}
	return ""
}
