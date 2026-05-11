package app

import (
	"fmt"
	"strings"
	"time"
)

// StartupOptions holds command-line flag values that override default startup behavior.
type StartupOptions struct {
	Context       string
	UnionContexts []string // --union-context (repeatable): contexts to merge in union view
	UnionSet      string   // --union-set: name of a union_sets entry from config to expand into UnionContexts
	// UnionContextColors maps each union-mode context name to its
	// configured color. Populated by ResolveUnionSet from the per-set
	// `color:` fields in config; empty for --union-context CLI usage
	// (which has no place to specify colors). Drives the row-tile
	// renderer in the merged view.
	UnionContextColors map[string]string
	Namespaces         []string
	Kubeconfig         string
	KubeconfigDirs     []string // --kubeconfig-dir: repeatable; each occurrence adds a directory to scan.
	Config             string
	NoMouse            bool
	NoColor            bool          // --no-color: forces monochrome output regardless of env/config.
	ReadOnly           bool          // --read-only: blocks all mutating actions; sticky for the process.
	WatchInterval      time.Duration // 0 means not set — fall back to config/default.
}

// HasCLIOverrides returns true when any CLI flag was provided.
// Kubeconfig is intentionally excluded: it affects client construction,
// not session restore. The session override only matters for --context
// and namespace/union navigation flags.
func (o StartupOptions) HasCLIOverrides() bool {
	return o.Context != "" || o.UnionSet != "" || len(o.Namespaces) > 0 || len(o.UnionContexts) > 0
}

// IsUnionMode returns true when one or more --union-context flags were
// provided. A single union context is allowed and degenerates to a
// single-cluster view with a Context column.
func (o StartupOptions) IsUnionMode() bool {
	return len(o.UnionContexts) > 0
}

// MaxUnionContexts caps how many --union-context flags one invocation can
// merge. The watch loop fans out one parallel GetResources per context on
// every tick (default 2s), so the bound exists to keep that cost predictable
// — not because the apiserver or client struggles at small N. Three is the
// expected case (blue/green/canary); five is comfortable; eight gives
// headroom without committing to "scales to 20."
const MaxUnionContexts = 8

// UnionSetLookup resolves a --union-set name into its list of contexts,
// optional default namespace, and per-context color map. Returns ok=false
// when the name isn't defined in config. Injected so ResolveUnionSet stays
// testable without an import cycle on the ui package.
//
// The colors map keys are context names (a subset of contexts); values are
// color names from ui.ClusterColorNames. Contexts without a configured
// color are simply absent from the map — the renderer reserves a blank
// cell rather than tinting them.
type UnionSetLookup func(name string) (contexts []string, namespace string, colors map[string]string, ok bool)

// ResolveUnionSet expands --union-set into UnionContexts, UnionContextColors,
// and (when the CLI did not supply --namespace) Namespaces. Mutex with
// --union-context and --context: mixing the two ways of specifying union
// mode is more confusing than useful. CLI --namespace always overrides the
// per-set namespace so a user can keep a stable set definition in config
// and retarget the namespace at the prompt.
//
// Errors are returned as user-facing messages — runTUI surfaces them
// straight to stderr without further wrapping.
func ResolveUnionSet(opts StartupOptions, lookup UnionSetLookup) (StartupOptions, error) {
	if opts.UnionSet == "" {
		return opts, nil
	}
	if len(opts.UnionContexts) > 0 {
		return opts, fmt.Errorf("--union-set and --union-context are mutually exclusive")
	}
	if opts.Context != "" {
		return opts, fmt.Errorf("--union-set and --context are mutually exclusive")
	}
	if lookup == nil {
		return opts, fmt.Errorf("--union-set %q: no union_sets configured", opts.UnionSet)
	}
	contexts, namespace, colors, ok := lookup(opts.UnionSet)
	if !ok {
		return opts, fmt.Errorf("--union-set %q: not defined in config", opts.UnionSet)
	}
	opts.UnionContexts = contexts
	opts.UnionContextColors = colors
	// CLI --namespace wins. Only fall back to the set's namespace when the
	// user did not supply one. The downstream ValidateUnionOptions check
	// then enforces "namespace required for union mode" against whichever
	// source provided it.
	if len(opts.Namespaces) == 0 && namespace != "" {
		opts.Namespaces = []string{namespace}
	}
	return opts, nil
}

// ValidateUnionOptions checks the union-mode invariants: namespace required,
// mutually exclusive with --context, no duplicate contexts, no collision with
// the internal sentinel, count under MaxUnionContexts, and all contexts exist
// in the kubeconfig. contextExists is injected so this is unit-testable
// without a real Kubernetes client.
func ValidateUnionOptions(o StartupOptions, contextExists func(string) bool) error {
	if !o.IsUnionMode() {
		return nil
	}
	if len(o.Namespaces) == 0 {
		return fmt.Errorf("union mode requires a namespace (--namespace flag or namespace: in the union_sets entry)")
	}
	if len(o.Namespaces) != 1 || strings.TrimSpace(o.Namespaces[0]) == "" {
		return fmt.Errorf("union mode supports exactly one non-empty namespace (got %d)", len(o.Namespaces))
	}
	if o.Context != "" {
		return fmt.Errorf("--union-context and --context are mutually exclusive")
	}
	if len(o.UnionContexts) > MaxUnionContexts {
		return fmt.Errorf("--union-context accepts at most %d clusters (got %d)", MaxUnionContexts, len(o.UnionContexts))
	}
	seen := make(map[string]struct{}, len(o.UnionContexts))
	for _, uc := range o.UnionContexts {
		if uc == UnionContextSentinel {
			return fmt.Errorf("union context name %q is reserved", uc)
		}
		if _, dup := seen[uc]; dup {
			return fmt.Errorf("--union-context %q specified more than once", uc)
		}
		seen[uc] = struct{}{}
		if contextExists != nil && !contextExists(uc) {
			return fmt.Errorf("union context %q not found in kubeconfig", uc)
		}
	}
	return nil
}
