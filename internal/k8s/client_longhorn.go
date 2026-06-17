package k8s

import (
	"context"
	"fmt"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// LonghornReplicaColumn is the header for the computed per-node replica-count
// column added to the Longhorn Nodes list.
const LonghornReplicaColumn = "REPLICAS"

// withLonghornReplicaCounts appends a REPLICAS column to each Longhorn node
// item with the number of longhorn.io/replicas scheduled on that node
// (spec.nodeID == node name). It is a no-op for any other resource type.
//
// One extra LIST per refresh (not per node): replicas are listed once and
// grouped by node. Best-effort — a failure leaves the column unset rather than
// failing the whole node list, so the table still renders.
func (c *Client) withLonghornReplicaCounts(ctx context.Context, contextName, namespace string, rt model.ResourceTypeEntry, items []model.Item) []model.Item {
	if !model.IsLonghornNode(rt) || len(items) == 0 {
		return items
	}
	counts, err := c.longhornReplicaCountsByNode(ctx, contextName, namespace, rt)
	if err != nil {
		logger.Warn("counting Longhorn replicas for node column", "context", contextName, "error", err)
		return items
	}
	for i := range items {
		items[i].Columns = append(items[i].Columns, model.KeyValue{
			Key:   LonghornReplicaColumn,
			Value: strconv.Itoa(counts[items[i].Name]),
		})
	}
	return items
}

// longhornReplicaCountsByNode lists longhorn.io/replicas and returns a map of
// node name -> replica count, keyed by each replica's spec.nodeID. The
// replicas share the node's API group/version, so rt.APIVersion is reused
// rather than hardcoding a Longhorn API version.
func (c *Client) longhornReplicaCountsByNode(ctx context.Context, contextName, namespace string, rt model.ResourceTypeEntry) (map[string]int, error) {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return nil, err
	}
	gvr := schema.GroupVersionResource{Group: rt.APIGroup, Version: rt.APIVersion, Resource: "replicas"}
	list, err := dynClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing Longhorn replicas: %w", err)
	}
	counts := make(map[string]int, len(list.Items))
	for i := range list.Items {
		nodeID, _, _ := unstructured.NestedString(list.Items[i].Object, "spec", "nodeID")
		if nodeID != "" {
			counts[nodeID]++
		}
	}
	return counts, nil
}

// ForceDeleteLonghornNode deletes a longhorn.io Node that the validating
// webhook (validator.longhorn.io) would otherwise refuse to remove.
//
// The webhook rejects deletion of a node that is still schedulable
// (spec.allowScheduling=true), so this disables scheduling first and then
// deletes. It deliberately does NOT remove replicas or engines: the webhook
// keeps rejecting a node that still holds data, and that error is surfaced to
// the caller so data is never silently destroyed. Evict the node's replicas
// first (SetLonghornNodeEviction) when it still holds data.
//
// The exact GroupVersionResource is used via the dynamic client rather than
// shelling out to "kubectl delete nodes" because that resource name is
// ambiguous between core nodes and longhorn.io nodes.
//
// The patch and delete are two separate calls: if the delete fails after the
// patch succeeds, the node is left with allowScheduling=false. The error is
// surfaced to the caller and a retry is safe (the patch is idempotent), but
// re-enable scheduling manually if abandoning the delete.
func (c *Client) ForceDeleteLonghornNode(contextName, namespace string, rt model.ResourceTypeEntry, name string) error {
	logger.Info("Force deleting Longhorn node", "context", contextName, "namespace", namespace, "name", name)
	res, err := c.longhornNodeResource(contextName, namespace, rt)
	if err != nil {
		return err
	}

	patch := []byte(`{"spec":{"allowScheduling":false}}`)
	if _, err := res.Patch(context.Background(), name, k8stypes.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("disabling scheduling on Longhorn node %s: %w", name, err)
	}
	if err := res.Delete(context.Background(), name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("deleting Longhorn node %s: %w", name, err)
	}
	return nil
}

// SetLonghornNodeEviction toggles replica eviction on a longhorn.io Node.
//
// When evict is true it sets spec.evictionRequested=true and
// spec.allowScheduling=false in one patch: Longhorn only evicts a node whose
// scheduling is disabled, and then rebuilds each replica on another node
// before removing it from this one (data-safe). When evict is false it clears
// spec.evictionRequested, leaving scheduling untouched.
func (c *Client) SetLonghornNodeEviction(contextName, namespace string, rt model.ResourceTypeEntry, name string, evict bool) error {
	logger.Info("Setting Longhorn node eviction", "context", contextName, "namespace", namespace, "name", name, "evict", evict)
	res, err := c.longhornNodeResource(contextName, namespace, rt)
	if err != nil {
		return err
	}

	patch := []byte(`{"spec":{"evictionRequested":false}}`)
	if evict {
		patch = []byte(`{"spec":{"allowScheduling":false,"evictionRequested":true}}`)
	}
	if _, err := res.Patch(context.Background(), name, k8stypes.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("setting eviction=%t on Longhorn node %s: %w", evict, name, err)
	}
	return nil
}

// longhornNodeResource returns the namespaced dynamic resource interface for
// the longhorn.io Node CRD in the given context.
func (c *Client) longhornNodeResource(contextName, namespace string, rt model.ResourceTypeEntry) (dynamic.ResourceInterface, error) {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return nil, err
	}
	gvr := schema.GroupVersionResource{Group: rt.APIGroup, Version: rt.APIVersion, Resource: rt.Resource}
	return dynClient.Resource(gvr).Namespace(namespace), nil
}
