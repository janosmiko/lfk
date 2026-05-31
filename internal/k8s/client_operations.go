package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// GetResourceYAML returns the YAML representation of a specific resource.
// For cluster-scoped resources (rt.Namespaced == false), the namespace is ignored.
// The resourceNamespace parameter is used when in all-namespaces mode to target
// the specific namespace of the resource.
func (c *Client) GetResourceYAML(ctx context.Context, contextName, namespace string, rt model.ResourceTypeEntry, name string) (string, error) {
	// Special handling for Helm virtual resource type.
	if rt.APIGroup == "_helm" && rt.Resource == "releases" {
		return c.GetHelmReleaseYAML(ctx, contextName, namespace, name)
	}
	if rt.APIGroup == "_portforward" {
		return "", nil // port forwards are managed locally, not via K8s API
	}

	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return "", err
	}

	gvr := schema.GroupVersionResource{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
	}

	var getter dynamic.ResourceInterface
	if rt.Namespaced {
		getter = dynClient.Resource(gvr).Namespace(namespace)
	} else {
		getter = dynClient.Resource(gvr)
	}

	obj, err := getter.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("getting resource: %w", err)
	}

	obj.SetManagedFields(nil)

	data, err := yaml.Marshal(obj.Object)
	if err != nil {
		return "", fmt.Errorf("marshalling to YAML: %w", err)
	}
	return reorderYAMLFields(string(data)), nil
}

// GetPodYAML returns the YAML for a pod.
func (c *Client) GetPodYAML(ctx context.Context, contextName, namespace, podName string) (string, error) {
	// Pods are namespaced; without Namespaced: true, GetResourceYAML
	// falls through to the cluster-scoped branch (dynClient.Resource(gvr)
	// with no .Namespace(...)), which hits /api/v1/pods/<name> — an
	// endpoint that does not exist on the API server. The API replies
	// with "the server could not find the requested resource", which
	// the caller surfaces as the YAML-load error.
	return c.GetResourceYAML(ctx, contextName, namespace, model.ResourceTypeEntry{
		APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true,
	}, podName)
}

// DeleteResource deletes a Kubernetes resource by type and name.
func (c *Client) DeleteResource(contextName, namespace string, rt model.ResourceTypeEntry, name string) error {
	logger.Info("Deleting resource", "context", contextName, "namespace", namespace, "kind", rt.Kind, "name", name)
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	gvr := schema.GroupVersionResource{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
	}

	var deleter dynamic.ResourceInterface
	if rt.Namespaced {
		deleter = dynClient.Resource(gvr).Namespace(namespace)
	} else {
		deleter = dynClient.Resource(gvr)
	}

	err = deleter.Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("deleting %s/%s: %w", rt.Resource, name, err)
	}
	return nil
}

// ScaleResource scales a Deployment, StatefulSet, or ReplicaSet to the specified replica count.
func (c *Client) ScaleResource(contextName, namespace, name, kind string, replicas int32) error {
	logger.Info("Scaling resource", "context", contextName, "namespace", namespace, "kind", kind, "name", name, "replicas", replicas)
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return err
	}

	ctx := context.Background()
	appsV1 := cs.AppsV1()

	switch kind {
	case "Deployment":
		scale, err := appsV1.Deployments(namespace).GetScale(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("getting scale for %s: %w", name, err)
		}
		scale.Spec.Replicas = replicas
		_, err = appsV1.Deployments(namespace).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("scaling %s to %d: %w", name, replicas, err)
		}
	case "StatefulSet":
		scale, err := appsV1.StatefulSets(namespace).GetScale(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("getting scale for %s: %w", name, err)
		}
		scale.Spec.Replicas = replicas
		_, err = appsV1.StatefulSets(namespace).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("scaling %s to %d: %w", name, replicas, err)
		}
	case "ReplicaSet":
		scale, err := appsV1.ReplicaSets(namespace).GetScale(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("getting scale for %s: %w", name, err)
		}
		scale.Spec.Replicas = replicas
		_, err = appsV1.ReplicaSets(namespace).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("scaling %s to %d: %w", name, replicas, err)
		}
	default:
		return fmt.Errorf("unsupported kind for scaling: %s", kind)
	}
	return nil
}

// ResizePVC patches a PersistentVolumeClaim's spec.resources.requests.storage to the given size.
func (c *Client) ResizePVC(contextName, namespace, name, newSize string) error {
	logger.Info("Resizing PVC", "context", contextName, "namespace", namespace, "name", name, "size", newSize)
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumeclaims"}
	patch := fmt.Appendf(nil, `{"spec":{"resources":{"requests":{"storage":"%s"}}}}`, newSize)
	_, err = dynClient.Resource(gvr).Namespace(namespace).Patch(
		context.Background(), name, k8stypes.MergePatchType, patch, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("resizing PVC %s to %s: %w", name, newSize, err)
	}
	return nil
}

// RestartResource performs a rolling restart by patching the pod template annotation.
func (c *Client) RestartResource(contextName, namespace, name, kind string) error {
	logger.Info("Restarting resource", "context", contextName, "namespace", namespace, "kind", kind, "name", name)
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return err
	}

	patch := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						"kubectl.kubernetes.io/restartedAt": time.Now().Format(time.RFC3339),
					},
				},
			},
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshalling patch: %w", err)
	}

	ctx := context.Background()
	appsV1 := cs.AppsV1()

	switch kind {
	case "Deployment":
		_, err = appsV1.Deployments(namespace).Patch(ctx, name, k8stypes.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	case "StatefulSet":
		_, err = appsV1.StatefulSets(namespace).Patch(ctx, name, k8stypes.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	case "DaemonSet":
		_, err = appsV1.DaemonSets(namespace).Patch(ctx, name, k8stypes.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	default:
		return fmt.Errorf("unsupported kind for restart: %s", kind)
	}
	if err != nil {
		return fmt.Errorf("restarting %s %s: %w", kind, name, err)
	}
	return nil
}

// RollbackDeployment rolls back a deployment to a specific revision.
func (c *Client) RollbackDeployment(ctx context.Context, contextName, namespace, name string, revision int64) error {
	logger.Info("Rolling back Deployment", "context", contextName, "namespace", namespace, "name", name, "revision", revision)
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return err
	}

	// List ReplicaSets owned by this deployment.
	deploy, err := cs.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting deployment: %w", err)
	}

	rsList, err := cs.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing replicasets: %w", err)
	}

	// Find the RS with the target revision.
	for _, rs := range rsList.Items {
		// Check ownership.
		owned := false
		for _, ref := range rs.OwnerReferences {
			if ref.UID == deploy.UID {
				owned = true
				break
			}
		}
		if !owned {
			continue
		}

		rev, _ := strconv.ParseInt(rs.Annotations["deployment.kubernetes.io/revision"], 10, 64)
		if rev == revision {
			// Patch the deployment with this RS's pod template.
			patchData, err := json.Marshal(map[string]any{
				"spec": map[string]any{
					"template": rs.Spec.Template,
				},
			})
			if err != nil {
				return fmt.Errorf("marshaling patch: %w", err)
			}

			_, err = cs.AppsV1().Deployments(namespace).Patch(ctx, name, k8stypes.MergePatchType, patchData, metav1.PatchOptions{})
			if err != nil {
				return fmt.Errorf("patching deployment: %w", err)
			}
			return nil
		}
	}

	return fmt.Errorf("revision %d not found", revision)
}

// GetDeploymentRevisions returns the list of revisions for a deployment.
func (c *Client) GetDeploymentRevisions(ctx context.Context, contextName, namespace, name string) ([]DeploymentRevision, error) {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	deploy, err := cs.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting deployment: %w", err)
	}

	rsList, err := cs.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing replicasets: %w", err)
	}

	revisions := make([]DeploymentRevision, 0, len(rsList.Items))
	for _, rs := range rsList.Items {
		owned := false
		for _, ref := range rs.OwnerReferences {
			if ref.UID == deploy.UID {
				owned = true
				break
			}
		}
		if !owned {
			continue
		}

		rev, _ := strconv.ParseInt(rs.Annotations["deployment.kubernetes.io/revision"], 10, 64)

		// Extract images from the template.
		var images []string
		for _, container := range rs.Spec.Template.Spec.Containers {
			images = append(images, container.Image)
		}

		revisions = append(revisions, DeploymentRevision{
			Revision:  rev,
			Name:      rs.Name,
			Replicas:  rs.Status.Replicas,
			Images:    images,
			CreatedAt: rs.CreationTimestamp.Time,
		})
	}

	// Sort by revision descending.
	sort.Slice(revisions, func(i, j int) bool {
		return revisions[i].Revision > revisions[j].Revision
	})

	return revisions, nil
}

// --- internal helpers ---

func (c *Client) restConfigForContext(displayName string) (*rest.Config, error) {
	// Load only the kubeconfig file that defines the requested context. When
	// multiple files are merged via clientcmd, clusters and users sharing a
	// name across files collapse into a single entry — issue #23: every
	// ~/.kube/config.d/*.yaml declared cluster "dev" and user "dev", so
	// every context routed to the same cluster after the merge. Loading the
	// origin file in isolation keeps each context's clusters/users intact.
	//
	// displayName is the lfk-side identifier (potentially disambiguated to
	// "name (basename)"). The override below uses the *original* name from
	// the source kubeconfig because that's what's recorded inside the file.
	rules := c.loadingRules
	overrideName := displayName
	if info, ok := c.contexts[displayName]; ok {
		rules = &clientcmd.ClientConfigLoadingRules{Precedence: []string{info.sourcePath}}
		overrideName = info.original
	} else if path := c.KubeconfigPathForContext(displayName); path != "" {
		rules = &clientcmd.ClientConfigLoadingRules{Precedence: []string{path}}
	}
	overrides := &clientcmd.ConfigOverrides{CurrentContext: overrideName}
	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
	cfg, err := cc.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("building rest config for context %q: %w", displayName, err)
	}
	return cfg, nil
}

func (c *Client) clientsetForContext(contextName string) (kubernetes.Interface, error) {
	// Allow tests to inject a fake clientset.
	if c.testClientset != nil {
		if cs, ok := c.testClientset.(kubernetes.Interface); ok {
			return cs, nil
		}
	}
	cfg, err := c.restConfigForContext(contextName)
	if err != nil {
		return nil, err
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating clientset: %w", err)
	}
	return cs, nil
}

func (c *Client) dynamicForContext(contextName string) (dynamic.Interface, error) {
	// Allow tests to inject a fake dynamic client.
	if c.testDynClient != nil {
		if dc, ok := c.testDynClient.(dynamic.Interface); ok {
			return dc, nil
		}
	}
	cfg, err := c.restConfigForContext(contextName)
	if err != nil {
		return nil, err
	}
	dynClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}
	return dynClient, nil
}

// RawClientset returns the kubernetes clientset for the currently selected
// context, or nil if none is available. Used by security sources that need
// a raw kubernetes.Interface (e.g., the heuristic source walking Pod specs).
func (c *Client) RawClientset() kubernetes.Interface {
	cs, err := c.clientsetForContext(c.CurrentContext())
	if err != nil || cs == nil {
		return nil
	}
	return cs
}

// RawDynamic returns the dynamic client for the currently selected context,
// or nil if none is available. Used by security sources that read CRDs
// (e.g., the trivy-operator source reading VulnerabilityReport CRs).
func (c *Client) RawDynamic() dynamic.Interface {
	dc, err := c.dynamicForContext(c.CurrentContext())
	if err != nil || dc == nil {
		return nil
	}
	return dc
}

// RawClientsetForContext returns the kubernetes clientset for the given
// context, or nil if it cannot be built. Exposed for callers that need to
// construct per-context sources (e.g., security source re-registration on
// context switch).
func (c *Client) RawClientsetForContext(contextName string) kubernetes.Interface {
	if contextName == "" {
		return nil
	}
	cs, err := c.clientsetForContext(contextName)
	if err != nil || cs == nil {
		return nil
	}
	return cs
}

// RawDynamicForContext returns the dynamic client for the given context, or
// nil if it cannot be built.
func (c *Client) RawDynamicForContext(contextName string) dynamic.Interface {
	if contextName == "" {
		return nil
	}
	dc, err := c.dynamicForContext(contextName)
	if err != nil || dc == nil {
		return nil
	}
	return dc
}

// metadataForContext returns a metadata-only client for the given context.
// When testMetaClient is set (tests), it is returned directly.
func (c *Client) metadataForContext(contextName string) (metadata.Interface, error) {
	// Allow tests to inject a fake metadata client.
	if c.testMetaClient != nil {
		if mc, ok := c.testMetaClient.(metadata.Interface); ok {
			return mc, nil
		}
	}
	cfg, err := c.restConfigForContext(contextName)
	if err != nil {
		return nil, err
	}
	mc, err := metadata.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating metadata client: %w", err)
	}
	return mc, nil
}
