package app

import (
	"strings"

	"k8s.io/client-go/tools/clientcmd"
)

var kubectlReadOnlyVerbs = map[string]bool{
	"get": true, "describe": true, "logs": true, "top": true, "explain": true,
	"api-resources": true, "api-versions": true, "version": true,
	"cluster-info": true, "events": true, "diff": true, "help": true, "options": true,
}

// kubectlReadOnlySubcommands lists the read-only subcommands of verbs that
// also have mutating ones. Anything not listed is treated as mutating.
var kubectlReadOnlySubcommands = map[string]map[string]bool{
	"auth":    {"can-i": true, "whoami": true},
	"config":  {"view": true, "get-contexts": true, "current-context": true},
	"rollout": {"status": true, "history": true},
}

var kubectlGlobalBoolFlags = map[string]bool{
	"--disable-compression": true, "--insecure-skip-tls-verify": true,
	"--match-server-version": true, "--warnings-as-errors": true,
	"-h": true, "--help": true,
}

// hasUnknownPreVerbFlag reports whether a dash token before the verb is not a
// known kubectl global flag. A flag we cannot classify may consume the next
// token, so the verb we read could be wrong.
func hasUnknownPreVerbFlag(args []string) bool {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			return false
		}
		name, _, hasValue := strings.Cut(a, "=")
		switch {
		case kubectlGlobalFlagsWithValue[name]:
			if !hasValue {
				i++
			}
		case kubectlGlobalBoolFlags[name]:
		default:
			return true
		}
	}
	return false
}

// isReadOnlyKubectl reports whether the kubectl args (without the kubectl or
// k prefix) are on the read-only allowlist. Unknown verbs and plugins fail closed.
func isReadOnlyKubectl(args []string) (verb string, ok bool) {
	rest := argsFromVerb(args)
	if len(rest) == 0 {
		return "", false
	}
	verb = rest[0]
	if hasUnknownPreVerbFlag(args) {
		return verb, false
	}
	if kubectlReadOnlyVerbs[verb] {
		return verb, true
	}
	if subs, has := kubectlReadOnlySubcommands[verb]; has {
		// A flag before the subcommand may consume the next token, so
		// only the token right after the verb is trusted.
		return verb, len(rest) > 1 && subs[rest[1]]
	}
	return verb, false
}

// kubectlContextCandidates returns every value that any --context token could
// carry, wherever it sits in the args. Over-approximating is deliberate: kubectl
// grammar (flag values, "--") makes the real target hard to parse.
func kubectlContextCandidates(args []string) []string {
	var out []string
	for i, a := range args {
		if v, ok := strings.CutPrefix(a, "--context="); ok {
			out = append(out, v)
		} else if a == "--context" {
			if i+1 < len(args) {
				out = append(out, args[i+1])
			} else {
				out = append(out, "")
			}
		}
	}
	return out
}

func (m Model) contextReadOnly(ctx string) bool {
	if ctx == "" || ctx == m.nav.Context {
		return m.readOnlyForContext(m.nav.Context)
	}
	if m.cliReadOnly || m.readOnly || m.effectiveContextReadOnly(ctx) {
		return true
	}
	// kubectl sees the original name, but read_only is keyed on the display
	// name. Duplicated names get a "name (file)" display name.
	if m.client == nil {
		return false
	}
	items, _ := m.client.GetContexts()
	for _, it := range items {
		if m.client.OriginalContextName(it.Name) == ctx && m.effectiveContextReadOnly(it.Name) {
			return true
		}
	}
	return false
}

// kubectlBlockedByReadOnly reports whether a command bar kubectl invocation
// must be refused. It blocks a non-allowlisted verb if the current context or
// any --context candidate is read-only, or a candidate is empty (kubectl then
// falls back to the kubeconfig current-context).
func (m Model) kubectlBlockedByReadOnly(args []string) (string, bool) {
	verb, ok := isReadOnlyKubectl(args)
	if ok {
		return verb, false
	}
	if verb == "" {
		verb = "command"
	}
	if m.navTargetReadOnly() {
		return verb, true
	}
	// kubectl passes plugins the raw argv and never applies --context to them.
	if !kubectlBuiltinVerbs[firstNonFlagToken(args)] && m.fallbackContextReadOnly() {
		return verb, true
	}
	for _, c := range kubectlContextCandidates(args) {
		if c == "" || m.contextReadOnly(c) {
			return verb, true
		}
	}
	return verb, false
}

// navTargetReadOnly resolves read-only for the context a command without
// --context runs against. At the cluster picker nav.Context is empty and
// kubectl uses the kubeconfig current-context, so check that one instead.
func (m Model) navTargetReadOnly() bool {
	if m.nav.Context != "" {
		return m.readOnlyForContext(m.nav.Context)
	}
	if m.cliReadOnly {
		return true
	}
	if m.client == nil {
		return true
	}
	cur := m.client.CurrentContext()
	if cur == "" {
		return false
	}
	return m.effectiveContextReadOnly(cur) || m.fallbackContextReadOnly()
}

// fallbackContextReadOnly resolves read-only for the current-context field of
// the kubeconfig file kubectlEnv selects, which kubectl uses whenever --context
// does not apply. Fails closed when that context cannot be determined.
func (m Model) fallbackContextReadOnly() bool {
	if m.client == nil {
		return true
	}
	ctx := m.nav.Context
	if ctx == "" {
		ctx = m.client.CurrentContext()
	}
	path := m.client.KubeconfigPathForContext(ctx)
	// No path or no current-context means kubectl has no target to mutate.
	if path == "" {
		return m.cliReadOnly || m.readOnly
	}
	cfg, err := clientcmd.LoadFromFile(path)
	if err != nil {
		return true
	}
	if cfg.CurrentContext == "" {
		return m.cliReadOnly || m.readOnly
	}
	return m.contextReadOnly(cfg.CurrentContext)
}
