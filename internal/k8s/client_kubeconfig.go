package k8s

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"k8s.io/client-go/tools/clientcmd"
)

// KubeconfigPaths returns the colon-separated kubeconfig paths used by this client.
func (c *Client) KubeconfigPaths() string {
	return strings.Join(c.loadingRules.Precedence, ":")
}

// KubeconfigPathForContext returns the kubeconfig file path that defines the
// given context. The argument is the lfk display name (which may have been
// disambiguated from the original kubeconfig context name). Falls back to
// the first path in the precedence list when the name is not registered, so
// commands invoked before the contexts map is hydrated (or against unknown
// names) still get a sensible KUBECONFIG.
//
// Subprocess invocations (kubectl, helm, etc.) must use this single source
// file rather than KubeconfigPaths because clientcmd's merge collapses
// clusters and users that share names across files — see issue #23 and
// restConfigForContext for the in-process equivalent.
func (c *Client) KubeconfigPathForContext(displayName string) string {
	c.configMu.RLock()
	defer c.configMu.RUnlock()
	return c.kubeconfigPathForContextLocked(displayName)
}

// kubeconfigPathForContextLocked is the lock-free body of
// KubeconfigPathForContext. The caller must hold configMu (read or write).
// restConfigForContext reuses it while already holding the read lock.
func (c *Client) kubeconfigPathForContextLocked(displayName string) string {
	if info, ok := c.contexts[displayName]; ok {
		return info.sourcePath
	}
	// Fallback to the first file. loadingRules is set at construction and never
	// swapped, so it needs no lock, but we are already under RLock here.
	if len(c.loadingRules.Precedence) > 0 {
		return c.loadingRules.Precedence[0]
	}
	return ""
}

// OriginalContextName returns the context name as written in the source
// kubeconfig file for the given lfk display name. Subprocesses (kubectl
// --context, helm --kube-context) must be passed this value, because the
// disambiguated display name only exists inside lfk and won't resolve in the
// merged kubeconfig kubectl loads. Returns the input unchanged when the name
// is not registered (preserves the no-collision and external-context cases).
func (c *Client) OriginalContextName(displayName string) string {
	c.configMu.RLock()
	defer c.configMu.RUnlock()
	if info, ok := c.contexts[displayName]; ok {
		return info.original
	}
	return displayName
}

// HostForContext returns the API server URL recorded in the kubeconfig for
// the given lfk display name, or "" when the rest config can't be built (no
// matching cluster, malformed kubeconfig, etc.). Used to key per-host disk
// caches under ~/.kube/cache/discovery so they share the same lifecycle as
// kubectl/k9s — `kubectl api-resources --invalidate-cache` wipes both.
//
// Tests can pre-seed c.testHostByDisplay to bypass kubeconfig resolution
// entirely (most fake clients have no Cluster definition with a server URL).
func (c *Client) HostForContext(displayName string) string {
	if c == nil {
		return ""
	}
	if h, ok := c.testHostByDisplay[displayName]; ok {
		return h
	}
	cfg, err := c.restConfigForContext(displayName)
	if err != nil || cfg == nil {
		return ""
	}
	return cfg.Host
}

// collectContexts walks each kubeconfig file once and produces a map of
// disambiguated display names → contextInfo, plus the deterministic display
// order and the resolved current-context display name.
//
// When two or more files declare the same context name, every occurrence is
// preserved by suffixing the display name with the source file's basename
// (e.g. "dev (dev-envs)" / "dev (itg-k8s)"). This is essential for issue #23:
// clientcmd merges duplicates into one entry, hiding every file but the
// first; surfacing each as its own UI entry lets the user actually drill into
// the cluster they want.
//
// fallbackCurrent is the current-context that clientcmd's merged config
// already resolved (first-writer-wins). collectContexts uses it to decide
// which display name should be marked "current" when multiple files declare
// the same name. If no file sets a current-context, it returns "".
func collectContexts(paths []string, fallbackCurrent string) (map[string]contextInfo, []string, string) {
	type fileContext struct {
		sourcePath string
		original   string
		namespace  string
		isCurrent  bool
	}

	// Group entries by their original name so collisions are easy to spot.
	// Stable iteration order across files comes from `paths`, and within a
	// file from a sorted slice of context names (Go map iteration is
	// randomised).
	entriesByName := make(map[string][]fileContext)
	var orderedNames []string

	for _, path := range paths {
		cfg, err := clientcmd.LoadFromFile(path)
		if err != nil {
			continue
		}
		names := make([]string, 0, len(cfg.Contexts))
		for name := range cfg.Contexts {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			ctx := cfg.Contexts[name]
			ns := ""
			if ctx != nil {
				ns = ctx.Namespace
			}
			if _, seen := entriesByName[name]; !seen {
				orderedNames = append(orderedNames, name)
			}
			entriesByName[name] = append(entriesByName[name], fileContext{
				sourcePath: path,
				original:   name,
				namespace:  ns,
				isCurrent:  name == cfg.CurrentContext,
			})
		}
	}

	contexts := make(map[string]contextInfo)
	order := make([]string, 0, len(orderedNames))

	for _, original := range orderedNames {
		entries := entriesByName[original]
		if len(entries) == 1 {
			display := original
			contexts[display] = contextInfo{
				display:    display,
				original:   original,
				sourcePath: entries[0].sourcePath,
				namespace:  entries[0].namespace,
			}
			order = append(order, display)
			continue
		}
		// Collision: suffix every entry with its source file's basename so
		// each becomes selectable. Using "name (basename)" keeps the
		// original name as the visible prefix, which matches how kubectl
		// users typically scan a context list.
		for _, e := range entries {
			display := original + " (" + contextDisplayHint(e.sourcePath) + ")"
			// In the unlikely event that two files share both context name
			// AND basename (e.g. ~/.kube/config.d/sub/dev.yaml and
			// ~/.kube/config.d/dev.yaml), append the full path to keep the
			// display name unique. Falls back to the absolute path so the
			// user can still tell entries apart.
			if _, clash := contexts[display]; clash {
				display = original + " (" + e.sourcePath + ")"
			}
			contexts[display] = contextInfo{
				display:    display,
				original:   original,
				sourcePath: e.sourcePath,
				namespace:  e.namespace,
			}
			order = append(order, display)
		}
	}

	// Decide the current context's display name. Prefer the value clientcmd
	// already merged (fallbackCurrent) so lfk's choice agrees with what
	// kubectl would pick when handed the same files. When that name is
	// ambiguous, pick the entry from the earliest file in the precedence
	// list — that mirrors first-writer-wins.
	current := ""
	if fallbackCurrent != "" {
		// Single-occurrence: display == original.
		if info, ok := contexts[fallbackCurrent]; ok {
			current = info.display
		} else {
			// Disambiguated: walk paths in order, pick first match.
			for _, path := range paths {
				for _, info := range contexts {
					if info.original == fallbackCurrent && info.sourcePath == path {
						current = info.display
						break
					}
				}
				if current != "" {
					break
				}
			}
		}
	}

	sort.Strings(order)
	return contexts, order, current
}

// contextDisplayHint returns a short label for use in a disambiguated context
// display name. It strips the directory prefix and the ".yaml"/".yml"
// extension so the suffix in the UI stays compact.
func contextDisplayHint(path string) string {
	base := filepath.Base(path)
	for _, ext := range []string{".yaml", ".yml", ".conf", ".kubeconfig"} {
		if trimmed, ok := strings.CutSuffix(base, ext); ok {
			return trimmed
		}
	}
	return base
}

// buildKubeconfigPaths assembles the list of kubeconfig file paths to load.
//
// Resolution order:
//  1. KUBECONFIG env var (colon-separated).
//  2. ~/.kube/config (only when home lookup succeeds AND KUBECONFIG is unset).
//  3. Files under each path in kubeconfigDirs, falling back to a single-element
//     [~/.kube/config.d/] when the slice is empty. An absolute, non-tilde
//     directory is honored even when home lookup fails — the only reason to
//     require a home directory is for tilde expansion or the default fallback.
//
// KUBECONFIG is exclusive by default, matching kubectl/k9s: when it is set
// (and `exclusive` is true), lfk does NOT add the default ~/.kube/config,
// nor auto-scan the default ~/.kube/config.d/. Only directories the user
// explicitly requested (--kubeconfig-dir / KUBECONFIG_DIR / kubeconfig_dir)
// are still merged on top, because those are deliberate opt-ins rather than
// implicit defaults. Passing exclusive=false (kubeconfig_exclusive: false /
// --kubeconfig-exclusive=false / LFK_KUBECONFIG_EXCLUSIVE=false) restores
// the historical merge-everything behavior.
func buildKubeconfigPaths(kubeconfigDirs []string, exclusive bool) []string {
	var paths []string

	// KUBECONFIG env var (colon-separated on unix). Empty entries (a stray
	// "KUBECONFIG=:" or a trailing colon) are dropped; when nothing
	// non-empty remains the variable counts as unset, so the default
	// discovery still applies instead of silently loading zero clusters.
	envPaths := trimNonEmpty(filepath.SplitList(os.Getenv("KUBECONFIG")))
	paths = append(paths, envPaths...)
	kubeconfigExclusive := exclusive && len(envPaths) > 0

	home, homeErr := os.UserHomeDir()
	if homeErr == nil && !kubeconfigExclusive {
		// Default kubeconfig. The dedup pass below collapses it when
		// KUBECONFIG already lists the same file.
		paths = append(paths, filepath.Join(home, ".kube", "config"))
		// Fall back to the default discovery directory when no override.
		if len(kubeconfigDirs) == 0 {
			kubeconfigDirs = []string{filepath.Join(home, ".kube", "config.d")}
		}
	}

	// Walk each kubeconfig directory. Skip silently when expansion is
	// required but home lookup failed — main.go's ValidateKubeconfigDirs
	// is the user-facing surface for this case, and at that point the
	// CLI/env/config layer has already errored out before we get here.
	for _, dir := range kubeconfigDirs {
		if dir == "" {
			continue
		}
		if strings.HasPrefix(dir, "~") && homeErr != nil {
			continue
		}
		paths = append(paths, collectConfigDirPaths(expandTilde(dir, home))...)
	}

	// Dedup by canonical path. The same kubeconfig can land in `paths`
	// twice when KUBECONFIG points at a file inside ~/.kube/config.d/, or
	// when one path is "foo.yaml" and another is "./foo.yaml", or when a
	// symlink resolves to a file the walk also visits directly, or when
	// two --kubeconfig-dir entries point at overlapping trees. Without
	// this pass collectContexts loads the same file twice and emits each
	// context as two "disambiguated" rows in the cluster list.
	return dedupKubeconfigPaths(paths)
}

// dedupKubeconfigPaths removes paths that resolve to the same underlying
// file, preserving the first occurrence's order. Comparison uses
// filepath.EvalSymlinks (canonical absolute path) so cosmetic differences
// like trailing slashes, "./" prefixes, or symlink redirection collapse to
// one entry. Paths that fail to resolve (missing file, dangling symlink)
// keep their original spelling — clientcmd will still try to load them and
// log an error if the file isn't readable, which is more informative than
// silently dropping them here.
func dedupKubeconfigPaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		key := p
		if resolved, err := filepath.EvalSymlinks(p); err == nil {
			key = resolved
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, p)
	}
	return out
}

// collectConfigDirPaths returns all file paths under dir. If dir is a symlink
// to a directory, the symlink is followed so WalkDir can descend into the real
// target. Returns nil when dir is missing, is not a directory, or is a
// dangling symlink.
//
// Why EvalSymlinks first: filepath.WalkDir does not follow symbolic links;
// when the root path is itself a symlink to a directory, its DirEntry reports
// IsDir()=false (Lstat treats symlinks as non-directories), so the callback
// would add the symlink path as a "file" and clientcmd would later fail with
// "read ...: is a directory".
func collectConfigDirPaths(dir string) []string {
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return nil
	}
	var out []string
	_ = filepath.WalkDir(resolved, func(path string, d os.DirEntry, err error) error {
		// Silently skip entries that can't be read (permission denied, etc.)
		// so a single unreadable subdir doesn't abort the whole walk.
		if err == nil && !d.IsDir() {
			out = append(out, path)
		}
		return nil
	})
	return out
}

// expandTilde resolves a leading "~" in a path to the given home directory.
// When the path is exactly "~" or starts with "~/", the tilde is replaced
// with home. Any other form (e.g. "~other/path") is returned unchanged so
// that filepath expansion errors are surfaced by downstream callers.
func expandTilde(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

// ResolveKubeconfigDirs picks the user's kubeconfig discovery directories by
// applying the documented precedence: CLI flags > env var > config file.
// Replacement semantics — the first layer that yields any non-empty entry
// wins outright; lower-priority layers are NOT merged in.
//
// Each entry is trimmed of surrounding whitespace; whitespace-only entries
// are dropped so they do not silently shadow a lower-priority layer that
// has real paths. The env var is split on the OS path-list separator
// (":" on unix, ";" on Windows), matching how KUBECONFIG itself is parsed.
// Returns nil when all three layers are empty, which signals "use the default
// ~/.kube/config.d" to the caller.
func ResolveKubeconfigDirs(cliFlags []string, envVar string, configValues []string) []string {
	if v := trimNonEmpty(cliFlags); len(v) > 0 {
		return v
	}
	if envVar = strings.TrimSpace(envVar); envVar != "" {
		if v := trimNonEmpty(filepath.SplitList(envVar)); len(v) > 0 {
			return v
		}
	}
	if v := trimNonEmpty(configValues); len(v) > 0 {
		return v
	}
	return nil
}

// trimNonEmpty trims whitespace from each entry and drops entries that are
// empty after trimming. Returns nil (not []string{}) when the input has no
// non-empty entries, so callers can use a single len() check.
func trimNonEmpty(in []string) []string {
	var out []string
	for _, s := range in {
		if v := strings.TrimSpace(s); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// ValidateKubeconfigDirs checks that every path exists and is a directory,
// returning a wrapped error on the first failure (so the user sees which
// path was bad). An empty/nil slice passes silently — caller falls back to
// default discovery. See ValidateKubeconfigDir for the per-path semantics
// including tilde expansion.
func ValidateKubeconfigDirs(paths []string) error {
	for _, p := range paths {
		if err := ValidateKubeconfigDir(p); err != nil {
			return err
		}
	}
	return nil
}

// ValidateKubeconfigDir checks that path exists and is a directory, returning
// a wrapped error otherwise. Empty path is treated as "no override" and
// passes silently — the caller falls back to default discovery. Tilde
// prefixes ("~", "~/...") are expanded against the user's home directory
// before stat; an unresolvable home is itself a validation error so a typo
// like "~/.kuibe/config.d" doesn't silently degrade to "no directory
// override applied".
func ValidateKubeconfigDir(path string) error {
	if path == "" {
		return nil
	}
	expanded := path
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("kubeconfig directory %q: cannot expand ~ (no home directory): %w", path, err)
		}
		expanded = expandTilde(path, home)
	}
	info, err := os.Stat(expanded)
	if err != nil {
		return fmt.Errorf("kubeconfig directory %q: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("kubeconfig directory %q: not a directory", path)
	}
	return nil
}

// resolveKubeconfigPaths returns the kubeconfig file list for NewClient:
// an explicit --kubeconfig override wins outright; otherwise discovery
// runs via buildKubeconfigPaths.
func resolveKubeconfigPaths(override string, kubeconfigDirs []string, exclusive bool) []string {
	if override != "" {
		return []string{override}
	}
	return buildKubeconfigPaths(kubeconfigDirs, exclusive)
}

// ResolveKubeconfigExclusive resolves whether a set KUBECONFIG suppresses
// the default discovery: the CLI flag (when explicitly passed) wins, then
// the LFK_KUBECONFIG_EXCLUSIVE env var, then the config file's
// kubeconfig_exclusive value (which defaults to true). Unparsable env
// values fall through.
func ResolveKubeconfigExclusive(cliSet, cliValue bool, envValue string, configValue bool) bool {
	if cliSet {
		return cliValue
	}
	if v, err := strconv.ParseBool(strings.TrimSpace(envValue)); err == nil {
		return v
	}
	return configValue
}
