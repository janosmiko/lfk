package app

import (
	"strings"

	"github.com/janosmiko/lfk/internal/ui"
)

var kubectlReadOnlyVerbs = map[string]bool{
	"get": true, "describe": true, "logs": true, "top": true, "explain": true,
	"api-resources": true, "api-versions": true, "version": true,
	"cluster-info": true, "events": true, "diff": true,
}

// kubectlReadOnlySubcommands lists the read-only subcommands of verbs that
// also have mutating ones. Anything not listed is treated as mutating.
var kubectlReadOnlySubcommands = map[string]map[string]bool{
	"auth":    {"can-i": true, "whoami": true},
	"config":  {"view": true, "get-contexts": true, "current-context": true},
	"rollout": {"status": true, "history": true},
}

// isReadOnlyKubectl reports whether the kubectl args (without the kubectl or
// k prefix) are on the read-only allowlist. Unknown verbs and plugins fail closed.
func isReadOnlyKubectl(args []string) (verb string, ok bool) {
	rest := argsFromVerb(args)
	if len(rest) == 0 {
		return "", false
	}
	verb = rest[0]
	if kubectlReadOnlyVerbs[verb] {
		return verb, true
	}
	if subs, has := kubectlReadOnlySubcommands[verb]; has {
		return verb, subs[firstNonFlagToken(rest[1:])]
	}
	return verb, false
}

// explicitKubectlContext returns the value of a --context flag, if any.
func explicitKubectlContext(args []string) string {
	for i, a := range args {
		if v, ok := strings.CutPrefix(a, "--context="); ok {
			return v
		}
		if a == "--context" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// kubectlBlockedByReadOnly reports whether a command bar kubectl invocation
// must be refused. It returns the verb for the status message.
func (m Model) kubectlBlockedByReadOnly(args []string) (string, bool) {
	verb, ok := isReadOnlyKubectl(args)
	if ok {
		return verb, false
	}
	if verb == "" {
		verb = "command"
	}
	target := explicitKubectlContext(args)
	if target == "" || target == m.nav.Context || target == m.kubectlContext(m.nav.Context) {
		return verb, m.readOnlyForContext(m.nav.Context)
	}
	if m.cliReadOnly || m.readOnly {
		return verb, true
	}
	if v, set := m.contextROOverrides[target]; set {
		return verb, v
	}
	return verb, ui.ResolveReadOnly(target, false)
}
