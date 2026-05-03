package k8s

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
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
	if info, ok := c.contexts[displayName]; ok {
		return info.sourcePath
	}
	// Fallback to the first file.
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
func buildKubeconfigPaths() []string {
	var paths []string

	// KUBECONFIG env var (colon-separated on unix).
	if env := os.Getenv("KUBECONFIG"); env != "" {
		paths = append(paths, filepath.SplitList(env)...)
	}

	home, err := os.UserHomeDir()
	if err == nil {
		// Default kubeconfig.
		defaultPath := filepath.Join(home, ".kube", "config")
		if !containsPath(paths, defaultPath) {
			paths = append(paths, defaultPath)
		}

		// config.d directory - recursively find all files (follows symlinks).
		paths = append(paths, collectConfigDirPaths(filepath.Join(home, ".kube", "config.d"))...)
	}

	// Dedup by canonical path. The same kubeconfig can land in `paths`
	// twice when KUBECONFIG points at a file inside ~/.kube/config.d/, or
	// when one path is "foo.yaml" and another is "./foo.yaml", or when a
	// symlink resolves to a file the walk also visits directly. Without
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

func containsPath(paths []string, target string) bool {
	return slices.Contains(paths, target)
}
