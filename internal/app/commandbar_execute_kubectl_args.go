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

// injectKubectlDefaults scans the args for -n/--namespace and
// -A/--all-namespaces flags. If they are not present, it injects the current
// namespace from the model. For built-in verbs it puts --context <current>
// first: kubectl uses the last --context, so a real user flag still wins, while
// a fake one (after "--", or the value of another flag) cannot fall back to the
// kubeconfig current-context. Plugin and unknown verbs get it appended, because
// kubectl rejects flags before a plugin name. Subcommands that don't accept
// namespace flags (explain, api-resources, version, config, cordon, etc.)
// have only --context injected — injecting -A or -n on those would cause
// kubectl to reject the command with "unknown flag".
func (m *Model) injectKubectlDefaults(args []string) []string {
	hasNamespace := false
	hasAllNamespaces := false

	for _, a := range args {
		switch {
		case a == "-n" || a == "--namespace" || strings.HasPrefix(a, "--namespace="):
			hasNamespace = true
		case a == "-A" || a == "--all-namespaces":
			hasAllNamespaces = true
		}
	}

	result := make([]string, 0, len(args)+4)
	pin := m.nav.Context != ""
	builtin := kubectlBuiltinVerbs[firstNonFlagToken(args)]
	if pin && builtin {
		result = append(result, "--context", m.kubectlContext(m.nav.Context))
	}
	result = append(result, args...)

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

	if pin && !builtin && len(kubectlContextCandidates(args)) == 0 {
		result = append(result, "--context", m.kubectlContext(m.nav.Context))
	}
	return result
}

// kubectlBuiltinVerbs is the set of top-level kubectl commands (from
// `kubectl --help`, without plugins). Any other verb is a plugin.
var kubectlBuiltinVerbs = map[string]bool{
	"create": true, "expose": true, "run": true, "set": true, "explain": true,
	"get": true, "edit": true, "delete": true, "rollout": true, "scale": true,
	"autoscale": true, "certificate": true, "cluster-info": true, "top": true,
	"cordon": true, "uncordon": true, "drain": true, "taint": true,
	"describe": true, "logs": true, "attach": true, "exec": true,
	"port-forward": true, "proxy": true, "cp": true, "auth": true,
	"debug": true, "events": true, "diff": true, "apply": true, "patch": true,
	"replace": true, "wait": true, "kustomize": true, "label": true,
	"annotate": true, "completion": true, "api-resources": true,
	"api-versions": true, "config": true, "kuberc": true, "plugin": true,
	"version": true, "help": true, "options": true,
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
	"--as-user-extra":         true,
	"--cache-dir":             true,
	"--certificate-authority": true,
	"--client-certificate":    true,
	"--client-key":            true,
	"--cluster":               true,
	"--context":               true,
	"--kubeconfig":            true,
	"--kuberc":                true,
	"--log-flush-frequency":   true,
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
	"-v":                      true,
	"--v":                     true,
	"--vmodule":               true,
}

// argsFromVerb walks args, skipping leading kubectl global flags (and the
// values of those that consume a separate token), and returns the slice
// starting at the first positional token, typically the subcommand verb.
// Returns nil if every token is a flag or the input is empty.
func argsFromVerb(args []string) []string {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			return args[i:]
		}
		// `--flag=value`: nothing extra to consume.
		if strings.Contains(a, "=") {
			continue
		}
		if kubectlGlobalFlagsWithValue[a] && i+1 < len(args) {
			i++ // skip the value
		}
	}
	return nil
}

// firstNonFlagToken returns the subcommand verb, or "" if there is none.
func firstNonFlagToken(args []string) string {
	if rest := argsFromVerb(args); len(rest) > 0 {
		return rest[0]
	}
	return ""
}
