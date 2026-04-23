// Package k8s provides Kubernetes API access for the TUI application.
package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/janosmiko/lfk/internal/model"
)

// Client wraps Kubernetes API access.
type Client struct {
	rawConfig    api.Config
	loadingRules *clientcmd.ClientConfigLoadingRules

	// testClientset and testDynClient allow tests to inject fake clients.
	// When set, clientsetForContext and dynamicForContext return these
	// instead of building real clients from the kubeconfig.
	testClientset interface{} // kubernetes.Interface (avoid import cycle in non-test code)
	testDynClient interface{} // dynamic.Interface
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

	return &Client{
		rawConfig:    rawConfig,
		loadingRules: loadingRules,
	}, nil
}

// KubeconfigPaths returns the colon-separated kubeconfig paths used by this client.
func (c *Client) KubeconfigPaths() string {
	return strings.Join(c.loadingRules.Precedence, ":")
}

// KubeconfigPathForContext returns the kubeconfig file path that defines the
// given context. If the context's origin file cannot be determined, it falls
// back to the first path in the precedence list.
func (c *Client) KubeconfigPathForContext(contextName string) string {
	// Check if the context has a location extension that tracks its source file.
	if ctx, ok := c.rawConfig.Contexts[contextName]; ok && ctx != nil {
		for _, loc := range ctx.Extensions {
			// clientcmd doesn't store source file in extensions by default,
			// so we try a different approach below.
			_ = loc
		}
	}

	// Walk each kubeconfig file and check if it defines this context.
	for _, path := range c.loadingRules.Precedence {
		cfg, err := clientcmd.LoadFromFile(path)
		if err != nil {
			continue
		}
		if _, ok := cfg.Contexts[contextName]; ok {
			return path
		}
	}

	// Fallback to the first file.
	if len(c.loadingRules.Precedence) > 0 {
		return c.loadingRules.Precedence[0]
	}
	return ""
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

	return paths
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
	for _, p := range paths {
		if p == target {
			return true
		}
	}
	return false
}

// GetContexts returns all available kube contexts.
func (c *Client) GetContexts() ([]model.Item, error) {
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

// CurrentContext returns the current context name from the kubeconfig.
func (c *Client) CurrentContext() string {
	return c.rawConfig.CurrentContext
}

// ContextExists returns true if the named context is defined in the loaded kubeconfig.
func (c *Client) ContextExists(name string) bool {
	_, ok := c.rawConfig.Contexts[name]
	return ok
}

// DefaultNamespace returns the namespace configured for the given context,
// falling back to "default" if none is set.
func (c *Client) DefaultNamespace(contextName string) string {
	if ctx, ok := c.rawConfig.Contexts[contextName]; ok && ctx.Namespace != "" {
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
func (c *Client) GetResources(ctx context.Context, contextName, namespace string, rt model.ResourceTypeEntry) ([]model.Item, error) {
	// Special handling for virtual resource types.
	if rt.APIGroup == "_helm" && rt.Resource == "releases" {
		return c.GetHelmReleases(ctx, contextName, namespace)
	}
	if rt.APIGroup == "_portforward" {
		return nil, nil // port forwards are managed locally, not via K8s API
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
func populatePrinterColumns(ti *model.Item, obj map[string]interface{}, printerColumns []model.PrinterColumn) {
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
func populateOwnerReferences(ti *model.Item, obj map[string]interface{}) {
	metadata, ok := obj["metadata"].(map[string]interface{})
	if !ok {
		return
	}
	ownerRefs, ok := metadata["ownerReferences"].([]interface{})
	if !ok {
		return
	}
	for i, ref := range ownerRefs {
		refMap, ok := ref.(map[string]interface{})
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
