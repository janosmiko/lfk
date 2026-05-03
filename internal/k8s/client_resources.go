package k8s

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/janosmiko/lfk/internal/model"
)

// secretGVR is the GroupVersionResource for Kubernetes Secrets.
var secretGVR = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}

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

	gvr := schema.GroupVersionResource{
		Group:    rt.APIGroup,
		Version:  rt.APIVersion,
		Resource: rt.Resource,
	}

	// Issue #86: route through the informer cache when it makes sense.
	// "always" forces every list through the cache; "auto" promotes a
	// (context, GVR) on the first large list and demotes it back to direct
	// once cached size has stayed below the demote threshold for several
	// calls. Cache miss (sync timeout) falls through to the direct path so
	// the UI always gets a result.
	//
	// listItems memoizes per-item by namespace/name + the object's own
	// resourceVersion: items unchanged since the last call are reused
	// without re-running buildResourceItem or copying anything. On a
	// busy cluster only the few churning pods rebuild; the rest hit the
	// memo. That's what removes the residual ~300ms per-call cost on a
	// 6k-pod list when the cluster has background churn.
	mode, infs := c.informerSnapshot()
	if cacheEnabled(mode, infs) && shouldUseCache(mode, infs, contextName, gvr) {
		build := func(obj *unstructured.Unstructured) model.Item {
			return c.buildResourceItem(obj, &rt)
		}
		items, _, cerr := infs.listItems(ctx, contextName, gvr, namespace, build)
		if cerr == nil {
			items = sortResourceItems(items, rt.Kind)
			infs.observeCachedListSize(contextName, gvr, len(items))
			return items, nil
		}
	}

	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return nil, err
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
	for i := range list.Items {
		ti := c.buildResourceItem(&list.Items[i], &rt)
		items = append(items, ti)
	}

	// Auto-mode promotion fires after the direct list, when we know the
	// observed result size. Crossing the threshold flips the (context, GVR)
	// to cache-backed for the *next* GetResources call, which is what makes
	// subsequent namespace switches on a 7k-pod list feel instant.
	if mode == InformerCacheAuto && infs != nil {
		infs.observeDirectListSize(contextName, gvr, len(items))
	}
	return sortResourceItems(items, rt.Kind), nil
}

// cacheEnabled reports whether the informer cache is wired up at all. False
// in InformerCacheOff or when SetInformerCacheMode has not been called.
// Pure helper on a snapshot — callers must already have taken
// informerSnapshot so the mode + pointer are consistent.
func cacheEnabled(mode InformerCacheMode, infs *informerCache) bool {
	return infs != nil && mode != InformerCacheOff
}

// shouldUseCache decides — for the current call — whether to take the
// informer-backed path or fall through to a direct list. Always-mode
// short-circuits to true; auto-mode consults the per-(context, GVR) state
// machine maintained by observeDirectListSize / observeCachedListSize.
func shouldUseCache(mode InformerCacheMode, infs *informerCache, contextName string, gvr schema.GroupVersionResource) bool {
	if mode == InformerCacheAlways {
		return true
	}
	if mode == InformerCacheAuto {
		return infs.isPromoted(contextName, gvr)
	}
	return false
}

// sortResourceItems applies the canonical row order GetResources uses
// regardless of whether the items came from a fresh apiserver LIST or from
// the informer cache. Events sort by most recent observation (LastSeen, not
// CreatedAt) so a recurring incident's latest report stays on top; everything
// else sorts alphabetically by Name.
func sortResourceItems(items []model.Item, kind string) []model.Item {
	if kind == "Event" {
		sort.Slice(items, func(i, j int) bool { return items[i].LastSeen.After(items[j].LastSeen) })
	} else {
		sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	}
	return items
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
