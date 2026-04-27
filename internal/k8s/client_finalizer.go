package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// FinalizerMatch represents a resource with a matching finalizer.
type FinalizerMatch struct {
	Name       string
	Namespace  string
	Kind       string
	APIGroup   string
	APIVersion string
	Resource   string // plural
	Namespaced bool
	Finalizers []string
	Matched    string // the specific finalizer that matched
	Age        string
}

// FindResourcesWithFinalizer iterates resource types, lists each via the dynamic
// client, and returns resources whose metadata.finalizers contain a case-insensitive
// substring match of the given pattern.
func (c *Client) FindResourcesWithFinalizer(
	ctx context.Context,
	contextName, namespace, pattern string,
	resourceTypes []model.ResourceTypeEntry,
) ([]FinalizerMatch, error) {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return nil, err
	}

	lowerPattern := strings.ToLower(pattern)
	var results []FinalizerMatch

	for _, rt := range resourceTypes {
		// Check for context cancellation between resource types.
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		// Skip virtual resource types.
		if rt.APIGroup == "_helm" || rt.APIGroup == "_portforward" {
			continue
		}

		gvr := schema.GroupVersionResource{
			Group:    rt.APIGroup,
			Version:  rt.APIVersion,
			Resource: rt.Resource,
		}

		var lister dynamic.ResourceInterface
		if rt.Namespaced {
			lister = dynClient.Resource(gvr).Namespace(namespace)
		} else {
			lister = dynClient.Resource(gvr)
		}

		list, err := lister.List(ctx, metav1.ListOptions{})
		if err != nil {
			// Skip resource types we cannot list (RBAC, CRDs not present, etc.).
			continue
		}

		for _, item := range list.Items {
			finalizers := item.GetFinalizers()
			if len(finalizers) == 0 {
				continue
			}

			for _, f := range finalizers {
				if strings.Contains(strings.ToLower(f), lowerPattern) {
					age := ""
					ts := item.GetCreationTimestamp()
					if !ts.IsZero() {
						age = formatAge(time.Since(ts.Time))
					}

					results = append(results, FinalizerMatch{
						Name:       item.GetName(),
						Namespace:  item.GetNamespace(),
						Kind:       rt.Kind,
						APIGroup:   rt.APIGroup,
						APIVersion: rt.APIVersion,
						Resource:   rt.Resource,
						Namespaced: rt.Namespaced,
						Finalizers: finalizers,
						Matched:    f,
						Age:        age,
					})
					break // one match per resource is enough
				}
			}
		}
	}

	return results, nil
}

// RemoveFinalizerFromResource uses a merge patch to remove a specific finalizer
// from a resource while preserving all other finalizers.
func (c *Client) RemoveFinalizerFromResource(
	ctx context.Context,
	contextName string,
	match FinalizerMatch,
) error {
	logger.Info("Removing finalizer",
		"context", contextName,
		"namespace", match.Namespace,
		"name", match.Name,
		"kind", match.Kind,
		"finalizer", match.Matched)
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	gvr := schema.GroupVersionResource{
		Group:    match.APIGroup,
		Version:  match.APIVersion,
		Resource: match.Resource,
	}

	// Get the current resource to read its finalizers.
	var lister dynamic.ResourceInterface
	if match.Namespaced {
		lister = dynClient.Resource(gvr).Namespace(match.Namespace)
	} else {
		lister = dynClient.Resource(gvr)
	}

	obj, err := lister.Get(ctx, match.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting resource %s/%s: %w", match.Kind, match.Name, err)
	}

	// Build new finalizers list excluding the matched one.
	currentFinalizers := obj.GetFinalizers()
	var newFinalizers []string
	for _, f := range currentFinalizers {
		if f != match.Matched {
			newFinalizers = append(newFinalizers, f)
		}
	}

	// Use a merge patch that sets the entire finalizers list.
	// Since finalizers is a simple list of strings, replacing the whole list
	// is the safest approach to avoid partial updates.
	patchData, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"finalizers": newFinalizers,
		},
	})
	if err != nil {
		return fmt.Errorf("marshaling patch: %w", err)
	}

	_, err = lister.Patch(ctx, match.Name, k8stypes.MergePatchType, patchData, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("patching finalizers on %s/%s: %w", match.Kind, match.Name, err)
	}

	return nil
}
