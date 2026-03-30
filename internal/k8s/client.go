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
// 3. All files in ~/.kube/config.d/ (recursively)
func NewClient() (*Client, error) {
	kubeconfigPaths := buildKubeconfigPaths()

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

		// config.d directory - recursively find all files.
		configDir := filepath.Join(home, ".kube", "config.d")
		_ = filepath.WalkDir(configDir, func(path string, d os.DirEntry, err error) error {
			// skip errors (dir might not exist)
			if err == nil && !d.IsDir() {
				paths = append(paths, path)
			}
			return nil
		})
	}

	return paths
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

		// Add "Used By" column for PVCs showing which pods reference the claim.
		if rt.Kind == "PersistentVolumeClaim" {
			if pods, err := c.GetPodsUsingPVC(ctx, contextName, ti.Namespace, ti.Name); err == nil && len(pods) > 0 {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Used By", Value: strings.Join(pods, ", ")})
			}
		}

		// Evaluate CRD additionalPrinterColumns if present.
		if len(rt.PrinterColumns) > 0 {
			// Build a set of existing column keys to avoid duplicates.
			existingKeys := make(map[string]bool, len(ti.Columns))
			for _, kv := range ti.Columns {
				existingKeys[kv.Key] = true
			}
			for _, pc := range rt.PrinterColumns {
				if existingKeys[pc.Name] {
					continue
				}
				val, ok := evaluateSimpleJSONPath(item.Object, pc.JSONPath)
				if !ok || val == nil {
					continue
				}
				formatted := formatPrinterValue(val, pc.Type)
				if formatted == "" {
					continue
				}
				// Skip printer columns that duplicate the STATUS column.
				if formatted == ti.Status {
					continue
				}
				ti.Columns = append(ti.Columns, model.KeyValue{Key: pc.Name, Value: formatted})
			}
		}

		// Extract owner references for navigation.
		if metadata, ok := item.Object["metadata"].(map[string]interface{}); ok {
			if ownerRefs, ok := metadata["ownerReferences"].([]interface{}); ok {
				for i, ref := range ownerRefs {
					if refMap, ok := ref.(map[string]interface{}); ok {
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
			}
		}

		// Extract labels, finalizers, and annotation count from metadata.
		populateMetadataFields(&ti, item.Object)

		items = append(items, ti)
	}
	// Sort events by time (newest first); all other resources alphabetically by name.
	if rt.Kind == "Event" {
		sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	} else {
		sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	}
	return items, nil
}
