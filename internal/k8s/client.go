// Package k8s provides Kubernetes API access for the TUI application.
package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/janosmiko/lfk/internal/model"
)

// secretGVR is the GroupVersionResource for Kubernetes Secrets.
var secretGVR = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}

// contextInfo decorates a kubeconfig context with its source file plus a
// display name that is unique across all loaded files. When several
// kubeconfigs declare the same context name, lfk disambiguates the display
// name (e.g. "dev (dev-envs)") so the user can still see and select each one
// — issue #23.
type contextInfo struct {
	// display is the unique name shown in the lfk UI. Equal to original
	// when no other file defines a context with the same name; otherwise of
	// the form "original (basename)".
	display string
	// original is the context name as written in the source kubeconfig
	// file. Subprocesses (kubectl --context, helm --kube-context) must be
	// passed this value, since the disambiguated display name only exists
	// inside lfk.
	original string
	// sourcePath is the kubeconfig file that defines the context.
	sourcePath string
	// namespace is the namespace recorded on the source file's context, or
	// empty when the file does not pin one.
	namespace string
}

// Client wraps Kubernetes API access.
type Client struct {
	rawConfig    api.Config
	loadingRules *clientcmd.ClientConfigLoadingRules

	// contexts indexes every loaded context by its lfk display name. Built
	// once at NewClient by collectContexts and treated as read-only after
	// construction so concurrent reads from tea.Cmd goroutines stay
	// race-free. Disambiguates the duplicate-name case that clientcmd's
	// merge silently collapses.
	contexts map[string]contextInfo

	// contextOrder preserves a deterministic display order for GetContexts.
	contextOrder []string

	// currentContext holds the display name of the global current-context.
	// Sourced from the first kubeconfig file in the precedence list, which
	// matches clientcmd's first-writer-wins merge rule for current-context.
	currentContext string

	// testClientset, testDynClient, and testMetaClient allow tests to inject
	// fake clients. When set, the corresponding *ForContext helpers return
	// these instead of building real clients from the kubeconfig.
	testClientset  any // kubernetes.Interface (avoid import cycle in non-test code)
	testDynClient  any // dynamic.Interface
	testMetaClient any // metadata.Interface

	// testHostByDisplay, when set, lets tests bypass kubeconfig host
	// resolution in HostForContext. Most fake test clients are constructed
	// without Cluster definitions (no server URL), so a real
	// restConfigForContext call would fail; this map provides synthetic
	// answers keyed by display name.
	testHostByDisplay map[string]string

	// secretLazyLoading, when true, routes Secret listing through the
	// metadata-only API so decoded values are lazy-fetched on hover instead
	// of being pulled up-front. Configured via the secret_lazy_loading
	// option; off by default so the list behaves like every other resource.
	secretLazyLoading bool

	// Guarded by discoveryMu; concurrent tea.Cmd goroutines may discover
	// across different contexts.
	discoveryMu      sync.Mutex
	discoveryClients map[string]*disk.CachedDiscoveryClient
}

// SetSecretLazyLoading toggles the metadata-only list path for Secrets.
// Typically called once at startup after loading the config file.
func (c *Client) SetSecretLazyLoading(enabled bool) {
	c.secretLazyLoading = enabled
}

// RBACCheck represents a single permission check result.
type RBACCheck struct {
	Verb    string
	Allowed bool
}

// AccessRule represents a single access rule from SelfSubjectRulesReview.
type AccessRule struct {
	Verbs         []string
	APIGroups     []string
	Resources     []string
	ResourceNames []string // empty means all names
}

// QuotaInfo holds resource quota data for a single ResourceQuota object.
type QuotaInfo struct {
	Name      string
	Namespace string
	Resources []QuotaResource
}

// QuotaResource holds usage data for a single resource within a quota.
type QuotaResource struct {
	Name    string  // e.g. "cpu", "memory", "pods", "services"
	Hard    string  // limit
	Used    string  // current usage
	Percent float64 // usage percentage (0-100)
}

// RBACSubject represents a unique subject (User, Group, or ServiceAccount) found
// in ClusterRoleBindings or RoleBindings.
type RBACSubject struct {
	Kind      string // "User", "Group", or "ServiceAccount"
	Name      string
	Namespace string // only populated for ServiceAccount
}

// DeploymentRevision represents a deployment revision history entry.
type DeploymentRevision struct {
	Revision  int64
	Name      string
	Replicas  int32
	Images    []string
	CreatedAt time.Time
}

// NewClient creates a new Kubernetes client, loading configs from:
// 1. KUBECONFIG env var
// 2. ~/.kube/config
// 3. All files in ~/.kube/config.d/ (recursively; symlinks to directories are followed)
func NewClient(kubeconfigOverride string) (*Client, error) {
	var kubeconfigPaths []string
	if kubeconfigOverride != "" {
		kubeconfigPaths = []string{kubeconfigOverride}
	} else {
		kubeconfigPaths = buildKubeconfigPaths()
	}

	loadingRules := &clientcmd.ClientConfigLoadingRules{
		Precedence: kubeconfigPaths,
	}

	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	rawConfig, err := kubeConfig.RawConfig()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig: %w", err)
	}

	contexts, order, current := collectContexts(kubeconfigPaths, rawConfig.CurrentContext)

	return &Client{
		rawConfig:      rawConfig,
		loadingRules:   loadingRules,
		contexts:       contexts,
		contextOrder:   order,
		currentContext: current,
	}, nil
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

// GetContexts returns all available kube contexts using their lfk display
// names (which match the original names when there are no collisions and are
// disambiguated as "name (basename)" when several files declare the same
// context name).
func (c *Client) GetContexts() ([]model.Item, error) {
	if len(c.contexts) == 0 {
		// Fallback for tests that construct a Client directly without
		// running NewClient: surface whatever rawConfig holds.
		items := make([]model.Item, 0, len(c.rawConfig.Contexts))
		for name := range c.rawConfig.Contexts {
			status := ""
			if name == c.rawConfig.CurrentContext {
				status = "current"
			}
			items = append(items, model.Item{Name: name, Status: status})
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
		return items, nil
	}
	items := make([]model.Item, 0, len(c.contextOrder))
	for _, display := range c.contextOrder {
		status := ""
		if display == c.currentContext {
			status = "current"
		}
		items = append(items, model.Item{Name: display, Status: status})
	}
	return items, nil
}

// CurrentContext returns the lfk display name of the current context.
func (c *Client) CurrentContext() string {
	if c.currentContext != "" {
		return c.currentContext
	}
	return c.rawConfig.CurrentContext
}

// ContextExists reports whether the lfk display name is defined.
func (c *Client) ContextExists(displayName string) bool {
	if _, ok := c.contexts[displayName]; ok {
		return true
	}
	// Fallback for clients constructed without collectContexts (tests).
	_, ok := c.rawConfig.Contexts[displayName]
	return ok
}

// DefaultNamespace returns the namespace configured for the given lfk display
// name, falling back to "default" if none is set.
func (c *Client) DefaultNamespace(displayName string) string {
	if info, ok := c.contexts[displayName]; ok && info.namespace != "" {
		return info.namespace
	}
	if ctx, ok := c.rawConfig.Contexts[displayName]; ok && ctx != nil && ctx.Namespace != "" {
		return ctx.Namespace
	}
	return "default"
}

// GetNamespaces returns namespaces for the given context.
func (c *Client) GetNamespaces(ctx context.Context, contextName string) ([]model.Item, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	nsList, err := cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing namespaces: %w", err)
	}

	items := make([]model.Item, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		items = append(items, model.Item{Name: ns.Name, Status: string(ns.Status.Phase)})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

// GetResources lists resources of a given type. For namespaced resources it
// scopes to the given namespace; for cluster-scoped resources it lists globally.
// When namespace is empty and the resource is namespaced, it lists across all namespaces.
//
// Secrets are fetched via the metadata-only API (PartialObjectMetadataList) to
// avoid pulling base64-encoded data over the wire. Helm release Secrets are
// large (100KB–1MB each) and would dominate list latency otherwise. The list
// items therefore carry only Name/Namespace/Age/Deletion/OwnerReferences — no
// "secret:<key>" data columns and no "Type" column. Per-secret data is loaded
// lazily by the UI layer when the user selects a specific secret.
func (c *Client) GetResources(ctx context.Context, contextName, namespace string, rt model.ResourceTypeEntry) ([]model.Item, error) {
	// Special handling for virtual resource types.
	if rt.APIGroup == "_helm" && rt.Resource == "releases" {
		return c.GetHelmReleases(ctx, contextName, namespace)
	}
	if rt.APIGroup == "_portforward" {
		return nil, nil // port forwards are managed locally, not via K8s API
	}

	// Secrets optionally use the metadata-only path to avoid transferring
	// large base64 data payloads (especially Helm release secrets). Gated
	// behind SetSecretLazyLoading so the default list behaviour stays
	// consistent with every other resource type; decoded values are then
	// loaded on hover at LevelResources.
	if c.secretLazyLoading && rt.APIGroup == "" && rt.Resource == "secrets" {
		return c.listSecretsMetadata(ctx, contextName, namespace, rt)
	}

	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return nil, err
	}

	gvr := schema.GroupVersionResource{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
	}

	var lister dynamic.ResourceInterface
	if rt.Namespaced {
		lister = dynClient.Resource(gvr).Namespace(namespace) // empty string = all namespaces
	} else {
		lister = dynClient.Resource(gvr)
	}

	list, err := lister.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", rt.Resource, err)
	}

	items := make([]model.Item, 0, len(list.Items))
	for _, item := range list.Items {
		ti := c.buildResourceItem(&item, &rt)
		items = append(items, ti)
	}
	// Sort events by most recent observation first (LastSeen, not CreatedAt).
	// CreatedAt holds the firstTimestamp — sorting on it would push recurring
	// incidents to the bottom even when their latest report is the freshest
	// thing in the list. Users expect "what happened most recently" at the top.
	// All other resources sort alphabetically by name.
	if rt.Kind == "Event" {
		sort.Slice(items, func(i, j int) bool { return items[i].LastSeen.After(items[j].LastSeen) })
	} else {
		sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	}
	return items, nil
}

// listSecretsMetadata fetches the Secret list using the metadata-only API,
// returning model.Items with only Name/Namespace/Age/Deletion/OwnerReferences.
func (c *Client) listSecretsMetadata(ctx context.Context, contextName, namespace string, rt model.ResourceTypeEntry) ([]model.Item, error) {
	mc, err := c.metadataForContext(contextName)
	if err != nil {
		return nil, err
	}

	var getter interface {
		List(ctx context.Context, opts metav1.ListOptions) (*metav1.PartialObjectMetadataList, error)
	}
	if rt.Namespaced {
		getter = mc.Resource(secretGVR).Namespace(namespace) // empty string = all namespaces
	} else {
		getter = mc.Resource(secretGVR)
	}

	list, err := getter.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing secrets (metadata): %w", err)
	}

	items := make([]model.Item, 0, len(list.Items))
	for i := range list.Items {
		ti := buildMetadataItem(&list.Items[i], rt.Namespaced)
		items = append(items, ti)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

// buildMetadataItem converts a PartialObjectMetadata into a model.Item.
// Only metadata fields are populated — no status, no kind-specific columns.
func buildMetadataItem(obj *metav1.PartialObjectMetadata, namespaced bool) model.Item {
	ti := model.Item{
		Name: obj.GetName(),
		Kind: obj.Kind,
	}

	if namespaced {
		ti.Namespace = obj.GetNamespace()
	}

	if ts := obj.GetCreationTimestamp(); !ts.IsZero() {
		ti.CreatedAt = ts.Time
		ti.Age = formatAge(time.Since(ts.Time))
	}

	if dt := obj.GetDeletionTimestamp(); dt != nil {
		ti.Deleting = true
		ti.Status = "Terminating"
		ti.Columns = append(ti.Columns, model.KeyValue{
			Key:   "Deletion",
			Value: dt.Format(time.RFC3339),
		})
	}

	// Append owner references for navigation (same logic as populateOwnerReferences
	// but operating on the typed OwnerReferences slice from PartialObjectMetadata).
	for i, ref := range obj.GetOwnerReferences() {
		if ref.Kind != "" && ref.Name != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{
				Key:   fmt.Sprintf("owner:%d", i),
				Value: ref.APIVersion + "||" + ref.Kind + "||" + ref.Name,
			})
		}
	}

	return ti
}

// buildResourceItem converts a single unstructured resource into a model.Item.
func (c *Client) buildResourceItem(item *unstructured.Unstructured, rt *model.ResourceTypeEntry) model.Item {
	ti := model.Item{
		Name:   item.GetName(),
		Kind:   item.GetKind(),
		Status: extractStatus(item.Object),
	}

	// Check if the resource is being deleted.
	if item.GetDeletionTimestamp() != nil {
		ti.Deleting = true
		ti.Columns = append(ti.Columns, model.KeyValue{
			Key:   "Deletion",
			Value: item.GetDeletionTimestamp().Format(time.RFC3339),
		})
	}

	// Always populate namespace for namespaced resources so that actions
	// (logs, exec, etc.) use the item's actual namespace, not the selector.
	if rt.Namespaced {
		ti.Namespace = item.GetNamespace()
	}

	// Populate Age from creationTimestamp.
	creationTS := item.GetCreationTimestamp()
	if !creationTS.IsZero() {
		ti.CreatedAt = creationTS.Time
		ti.Age = formatAge(time.Since(creationTS.Time))
	}

	// Populate Ready and Restarts based on kind.
	populateResourceDetails(&ti, item.Object, rt.Kind)

	// Override status to "Terminating" for resources marked for deletion.
	applyDeletionStatus(&ti)

	// "Used By" (pods referencing the PVC) used to be populated here, but
	// that required a per-PVC pod-list call (N+1). The info is now loaded
	// lazily as the PVC's owned children via GetOwnedResources when the
	// user selects or drills into a PVC — see resources.go's
	// getPodsUsingPVC and view_right.go's kindHasOwnedChildren.

	// Evaluate CRD additionalPrinterColumns if present.
	populatePrinterColumns(&ti, item.Object, rt.PrinterColumns)

	// Extract owner references for navigation.
	populateOwnerReferences(&ti, item.Object)

	// Extract labels, finalizers, and annotation count from metadata.
	populateMetadataFields(&ti, item.Object)

	return ti
}

// populatePrinterColumns evaluates CRD additionalPrinterColumns and appends
// them to the item's columns, skipping duplicates and status-matching values.
func populatePrinterColumns(ti *model.Item, obj map[string]any, printerColumns []model.PrinterColumn) {
	if len(printerColumns) == 0 {
		return
	}
	// Build a set of existing column keys to avoid duplicates.
	existingKeys := make(map[string]bool, len(ti.Columns))
	for _, kv := range ti.Columns {
		existingKeys[kv.Key] = true
	}
	for _, pc := range printerColumns {
		if existingKeys[pc.Name] {
			continue
		}
		val, ok := evaluateSimpleJSONPath(obj, pc.JSONPath)
		if !ok || val == nil {
			continue
		}
		formatted := formatPrinterValue(val, pc.Type)
		if formatted == "" {
			continue
		}
		// Skip printer columns that duplicate the STATUS column
		// (exact match or contained within, e.g., "Healthy" in "Healthy/Synced").
		if formatted == ti.Status || strings.Contains(ti.Status, formatted) {
			continue
		}
		ti.Columns = append(ti.Columns, model.KeyValue{Key: pc.Name, Value: formatted})
	}
}

// populateOwnerReferences extracts owner references from the object metadata
// and appends them as columns for navigation.
func populateOwnerReferences(ti *model.Item, obj map[string]any) {
	metadata, ok := obj["metadata"].(map[string]any)
	if !ok {
		return
	}
	ownerRefs, ok := metadata["ownerReferences"].([]any)
	if !ok {
		return
	}
	for i, ref := range ownerRefs {
		refMap, ok := ref.(map[string]any)
		if !ok {
			continue
		}
		kind, _ := refMap["kind"].(string)
		name, _ := refMap["name"].(string)
		apiVersion, _ := refMap["apiVersion"].(string)
		if kind != "" && name != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{
				Key:   fmt.Sprintf("owner:%d", i),
				Value: apiVersion + "||" + kind + "||" + name,
			})
		}
	}
}
