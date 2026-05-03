package app

import (
	"sort"
	"strings"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// completeKubectl returns position-aware suggestions for kubectl commands.
// It handles flag value completion, flag name completion, and positional
// arguments (subcommand, resource type, resource name).
func completeKubectl(tokens []token, m *Model) []ui.Suggestion {
	if len(tokens) == 0 {
		return nil
	}

	// Determine effective tokens by skipping the kubectl/k prefix.
	effective := tokens
	if len(tokens) > 0 {
		first := strings.ToLower(tokens[0].text)
		if first == "kubectl" || first == "k" {
			effective = tokens[1:]
		}
	}

	// Empty effective tokens: the user has typed exactly "k" or
	// "kubectl" with no following word yet. Offer the subcommand list
	// as a preview of what they can type next — same content the user
	// would see after typing a trailing space.
	if len(effective) == 0 {
		return filterSuggestionsTyped(kubectlSubcommandList(), "", "subcommand")
	}

	lastToken := effective[len(effective)-1]
	prefix := strings.ToLower(lastToken.text)

	// Flag value completion: check if the previous token is a known flag.
	if len(effective) >= 2 {
		prevToken := strings.ToLower(effective[len(effective)-2].text)
		switch prevToken {
		case "-n", "--namespace":
			return filterSuggestionsFuzzy(m.namespaceNames(), prefix, "namespace")
		case "-o", "--output":
			return filterSuggestionsFuzzy(outputFormatsComplete(), prefix, "format")
		}
	}

	// Flag name completion: current token starts with "-".
	if strings.HasPrefix(lastToken.text, "-") {
		// Find the subcommand (first non-flag token) for subcommand-specific flags.
		subcommand := ""
		for _, tok := range effective {
			if !strings.HasPrefix(tok.text, "-") {
				subcommand = strings.ToLower(tok.text)
				break
			}
		}
		flags := kubectlFlagsForSubcommand(subcommand)
		return filterSuggestionsTyped(flags, prefix, "flag")
	}

	// Position-based completion.
	// Calculate effective position: index of the current token among non-flag tokens.
	pos := effectivePosition(effective)

	switch pos {
	case 0:
		// Subcommand position.
		return filterSuggestionsTyped(kubectlSubcommandList(), prefix, "subcommand")
	case 1:
		// Resource type position.
		return completeResourceJump(prefix, m.resourceTypeItems())
	default:
		// Resource name position -- only suggest if the kubectl resource type
		// matches the currently viewed resource type (names are only available
		// for the resource type currently loaded in the explorer).
		names := resourceNamesForKubectl(m, effective)
		return filterSuggestionsFuzzy(names, prefix, "name")
	}
}

// kubectlSubcommandList returns all keys from kubectlSubcommandSet, sorted.
func kubectlSubcommandList() []string {
	list := make([]string, 0, len(kubectlSubcommandSet))
	for k := range kubectlSubcommandSet {
		list = append(list, k)
	}
	sort.Strings(list)
	return list
}

// kubectlFlagsForSubcommand returns flags relevant to the given kubectl subcommand.
// Common flags are always included; subcommand-specific flags are appended.
func kubectlFlagsForSubcommand(subcommand string) []string {
	// Common flags for all subcommands.
	flags := []string{
		"-n", "--namespace",
		"-o", "--output",
		"-l", "--selector",
		"-A", "--all-namespaces",
		"--sort-by",
		"--field-selector",
		"--show-labels",
		"-w", "--watch",
		"--no-headers",
		"-f", "--filename",
		"--dry-run=client", "--dry-run=server",
		"--context",
	}

	// Subcommand-specific flags.
	switch subcommand {
	case "delete":
		flags = append(flags,
			"--force",
			"--grace-period=0",
			"--grace-period=",
			"--cascade=foreground",
			"--cascade=background",
			"--cascade=orphan",
			"--now",
			"--wait",
			"--timeout=",
		)
	case "apply":
		flags = append(flags,
			"--server-side",
			"--force-conflicts",
			"--prune",
			"--validate=true",
			"--validate=false",
			"--record",
		)
	case "get":
		flags = append(flags,
			"--show-kind",
			"--show-managed-fields",
			"--chunk-size=",
			"--ignore-not-found",
			"--raw",
		)
	case "describe":
		flags = append(flags,
			"--show-events",
		)
	case "logs":
		flags = append(flags,
			"-c", "--container",
			"--all-containers",
			"-p", "--previous",
			"--since=",
			"--since-time=",
			"--tail=",
			"--timestamps",
			"--prefix",
			"--max-log-requests=",
		)
	case "exec":
		flags = append(flags,
			"-c", "--container",
			"-i", "--stdin",
			"-t", "--tty",
			"--",
		)
	case "scale":
		flags = append(flags,
			"--replicas=",
			"--current-replicas=",
			"--timeout=",
		)
	case "rollout":
		flags = append(flags,
			"--to-revision=",
			"--timeout=",
		)
	case "label", "annotate":
		flags = append(flags,
			"--overwrite",
			"--resource-version=",
		)
	case "patch":
		flags = append(flags,
			"--type=strategic",
			"--type=merge",
			"--type=json",
			"-p",
		)
	case "create":
		flags = append(flags,
			"--save-config",
			"--validate=true",
			"--validate=false",
		)
	case "edit":
		flags = append(flags,
			"--save-config",
			"--validate=true",
			"--validate=false",
		)
	case "drain":
		flags = append(flags,
			"--force",
			"--grace-period=",
			"--ignore-daemonsets",
			"--delete-emptydir-data",
			"--timeout=",
			"--pod-selector=",
		)
	case "cordon", "uncordon":
		// No extra flags.
	case "taint":
		flags = append(flags,
			"--overwrite",
		)
	case "port-forward":
		flags = append(flags,
			"--address=",
		)
	case "cp":
		flags = append(flags,
			"-c", "--container",
			"--no-preserve",
			"--retries=",
		)
	case "top":
		flags = append(flags,
			"--containers",
			"--no-headers",
			"--sort-by=cpu",
			"--sort-by=memory",
		)
	}

	return flags
}

// outputFormatsComplete returns kubectl output format values.
func outputFormatsComplete() []string {
	return []string{
		"json", "yaml", "wide", "name",
		"jsonpath=", "custom-columns=",
	}
}

// resourceNamesForKubectl returns resource instance names from middleItems only
// when the kubectl command's resource type matches the currently viewed type.
// effective is the token slice with kubectl/k prefix already stripped.
func resourceNamesForKubectl(m *Model, effective []token) []string {
	// Find the resource type token (position 1 among non-flag tokens).
	cmdResourceType := ""
	nonFlagIdx := 0
	for _, tok := range effective {
		if strings.HasPrefix(tok.text, "-") {
			continue
		}
		if nonFlagIdx == 1 {
			cmdResourceType = strings.ToLower(tok.text)
			break
		}
		nonFlagIdx++
	}

	if cmdResourceType == "" {
		return nil
	}

	// Resolve to canonical plural resource name.
	cmdResourceType = m.resolveResourceType(cmdResourceType)

	// Determine the target namespace from flags, defaulting to current.
	cmdNamespace := m.effectiveNamespace()
	for i, tok := range effective {
		lower := strings.ToLower(tok.text)
		if (lower == "-n" || lower == "--namespace") && i+1 < len(effective) {
			cmdNamespace = effective[i+1].text
			break
		}
	}

	// Fast path: if we're viewing this exact resource type and namespace, use middleItems.
	currentResource := strings.ToLower(m.nav.ResourceType.Resource)
	if m.nav.Level >= model.LevelResources && cmdResourceType == currentResource && cmdNamespace == m.effectiveNamespace() {
		return resourceNames(m)
	}

	// Otherwise, always use async cache (fetches if not cached).
	return m.cachedResourceNames(cmdResourceType, cmdNamespace)
}

// toSingular converts a plural Kubernetes resource name to its singular form.
func toSingular(plural string) string {
	switch {
	case strings.HasSuffix(plural, "ies"):
		// policies -> policy
		return plural[:len(plural)-3] + "y"
	case strings.HasSuffix(plural, "ses") || strings.HasSuffix(plural, "xes") || strings.HasSuffix(plural, "zes"):
		// ingresses -> ingress
		return plural[:len(plural)-2]
	case strings.HasSuffix(plural, "s"):
		// pods -> pod
		return plural[:len(plural)-1]
	default:
		return plural
	}
}

// kubectlPrefixSuggestions returns the two kubectl dispatch prefixes ("k"
// and "kubectl") as autocomplete entries. They are NOT builtin commands
// (kubectl goes through its own classifier branch), but they must be
// discoverable in the same dropdown as builtins so users don't need to
// know the full prefix to find them.
func kubectlPrefixSuggestions() []ui.Suggestion {
	return []ui.Suggestion{
		{Text: "k", Category: "kubectl"},
		{Text: "kubectl", Category: "kubectl"},
	}
}
