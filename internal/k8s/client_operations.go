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
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"

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
	return c.GetResourceYAML(ctx, contextName, namespace, model.ResourceTypeEntry{
		APIGroup: "", APIVersion: "v1", Resource: "pods",
	}, podName)
}

// DeleteResource deletes a Kubernetes resource by type and name.
func (c *Client) DeleteResource(contextName, namespace string, rt model.ResourceTypeEntry, name string) error {
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

// RestartResource performs a rolling restart by patching the pod template annotation.
func (c *Client) RestartResource(contextName, namespace, name, kind string) error {
	cs, err := c.clientsetForContext(contextName)
	if err != nil {
		return err
	}

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
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
			patchData, err := json.Marshal(map[string]interface{}{
				"spec": map[string]interface{}{
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

func (c *Client) restConfigForContext(contextName string) (*rest.Config, error) {
	overrides := &clientcmd.ConfigOverrides{CurrentContext: contextName}
	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(c.loadingRules, overrides)
	cfg, err := cc.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("building rest config for context %q: %w", contextName, err)
	}
	return cfg, nil
}

func (c *Client) clientsetForContext(contextName string) (*kubernetes.Clientset, error) {
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
