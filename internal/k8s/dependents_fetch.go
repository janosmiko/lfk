package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// DependentKind is one child kind the owner walk lists, paired with the Kind
// name the summary prints. The list carries the Kind because a dynamic list
// item reports the kind only when the server fills it in.
//
// Selector narrows the list server-side. It is set only where the API
// guarantees the children carry the label, so a namespace holding thousands of
// unrelated objects costs the same as an empty one. Empty means the whole
// namespace, which is what a bulk selection needs: label selectors cannot
// express "any of these fifty workloads".
type DependentKind struct {
	GVR      schema.GroupVersionResource
	Kind     string
	Selector string
}

// serviceNameLabel is the label every EndpointSlice carries naming its
// Service, which is how a Service's slices are found without listing them all.
const serviceNameLabel = "kubernetes.io/service-name"

var (
	podsGVR               = schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	replicaSetsGVR        = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}
	controllerRevsGVR     = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "controllerrevisions"}
	jobsGVR               = schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}
	endpointSlicesGVR     = schema.GroupVersionResource{Group: "discovery.k8s.io", Version: "v1", Resource: "endpointslices"}
	persistentVolClaimGVR = schema.GroupVersionResource{Version: "v1", Resource: "persistentvolumeclaims"}
)

// dependentKindsByOwner says which child kinds are worth listing for each
// owner kind. The set is fixed on purpose: a walk over every kind in the
// cluster would need one list call per API resource, which no confirm dialog
// can afford. A kind that is not listed here yields no count at all, which the
// dialog reports as unknown rather than as zero.
var dependentKindsByOwner = map[string][]DependentKind{
	"Deployment": {
		{GVR: replicaSetsGVR, Kind: "ReplicaSet"},
		{GVR: podsGVR, Kind: "Pod"},
	},
	"StatefulSet": {
		{GVR: controllerRevsGVR, Kind: "ControllerRevision"},
		{GVR: podsGVR, Kind: "Pod"},
		{GVR: persistentVolClaimGVR, Kind: "PersistentVolumeClaim"},
	},
	"DaemonSet": {
		{GVR: controllerRevsGVR, Kind: "ControllerRevision"},
		{GVR: podsGVR, Kind: "Pod"},
	},
	"ReplicaSet":            {{GVR: podsGVR, Kind: "Pod"}},
	"Job":                   {{GVR: podsGVR, Kind: "Pod"}},
	"CronJob":               {{GVR: jobsGVR, Kind: "Job"}, {GVR: podsGVR, Kind: "Pod"}},
	"Service":               {{GVR: endpointSlicesGVR, Kind: "EndpointSlice"}},
	"ReplicationController": {{GVR: podsGVR, Kind: "Pod"}},
}

// selectorScopedGVRs are the child kinds a workload's own spec.selector is
// guaranteed to match, so the list can be narrowed server-side. A Deployment's
// ReplicaSets and Pods must match its selector, and the StatefulSet and
// DaemonSet controllers find their ControllerRevisions by that same selector.
//
// Jobs under a CronJob and PersistentVolumeClaims under a StatefulSet are
// deliberately absent: neither owner has a selector that the API promises its
// children carry, and a narrowed list there would undercount.
var selectorScopedGVRs = map[schema.GroupVersionResource]bool{
	podsGVR:           true,
	replicaSetsGVR:    true,
	controllerRevsGVR: true,
}

// DependentKindsFor returns the child kinds to list for one target, and whether
// its kind is one the walk knows how to follow.
//
// ownerSelector is the target's own spec.selector as a label-selector string,
// empty where it has none. Where the API guarantees the children carry it, the
// list is narrowed to it, so counting one small Deployment in a namespace of
// ten thousand pods costs one small list rather than the whole namespace.
func DependentKindsFor(ownerKind, ownerName, ownerSelector string) ([]DependentKind, bool) {
	kinds, ok := dependentKindsByOwner[ownerKind]
	if !ok {
		return nil, false
	}
	out := make([]DependentKind, 0, len(kinds))
	for _, k := range kinds {
		switch {
		case k.GVR == endpointSlicesGVR && ownerName != "":
			k.Selector = serviceNameLabel + "=" + ownerName
		case ownerSelector != "" && selectorScopedGVRs[k.GVR]:
			k.Selector = ownerSelector
		}
		out = append(out, k)
	}
	return out, true
}

// HasDependentKinds reports whether the walk knows how to follow this kind's
// children, without building the list.
func HasDependentKinds(ownerKind string) bool {
	_, ok := dependentKindsByOwner[ownerKind]
	return ok
}

// MergeDependentKinds folds several owner kinds into one list of child kinds
// to fetch, so a bulk selection lists each kind once per namespace however
// many rows picked it. The lists stay unscoped: a label selector cannot
// express "owned by any of these targets", and one namespace list is cheaper
// than one narrowed list per selected row.
func MergeDependentKinds(ownerKinds []string) []DependentKind {
	seen := make(map[schema.GroupVersionResource]bool)
	out := make([]DependentKind, 0, len(ownerKinds))
	for _, owner := range ownerKinds {
		for _, k := range dependentKindsByOwner[owner] {
			if seen[k.GVR] {
				continue
			}
			seen[k.GVR] = true
			out = append(out, k)
		}
	}
	return out
}

// DependentRefsInNamespace lists the candidate child objects in one namespace,
// reduced to the identity and owners the walk needs.
//
// Cost is one list call per child kind, whatever the number of targets, so a
// bulk delete across one namespace costs the same as a single delete there.
func (c *Client) DependentRefsInNamespace(
	ctx context.Context, contextName, namespace string, kinds []DependentKind,
) ([]DependentRef, error) {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return nil, err
	}
	return dependentRefsFrom(ctx, dynClient, namespace, kinds)
}

func dependentRefsFrom(
	ctx context.Context, dynClient dynamic.Interface, namespace string, kinds []DependentKind,
) ([]DependentRef, error) {
	if namespace == "" {
		// An empty namespace lists cluster-wide, which needs cluster-scoped
		// RBAC a user holding one namespaced Role does not have. Both callers
		// already filter these out. This keeps a future owner kind from
		// reintroducing the cluster-wide call by accident.
		return nil, fmt.Errorf("the dependent walk needs a namespace")
	}
	var out []DependentRef
	for _, k := range kinds {
		list, err := dynClient.Resource(k.GVR).Namespace(namespace).
			List(ctx, metav1.ListOptions{LabelSelector: k.Selector})
		if err != nil {
			// One kind failing takes the whole row down rather than yielding a
			// partial figure. A user who cannot list ControllerRevisions would
			// otherwise read a confident undercount in a dialog that is about
			// to delete something.
			return nil, fmt.Errorf("listing %s in %s: %w", k.GVR.Resource, namespace, err)
		}
		for i := range list.Items {
			item := &list.Items[i]
			refs := item.GetOwnerReferences()
			if len(refs) == 0 {
				// Nothing owns it, so no delete can cascade to it. Kept out
				// of the graph rather than carried as an unreachable node.
				continue
			}
			owners := make([]string, 0, len(refs))
			for _, ref := range refs {
				owners = append(owners, string(ref.UID))
			}
			out = append(out, DependentRef{
				Kind:      k.Kind,
				UID:       string(item.GetUID()),
				OwnerUIDs: owners,
			})
		}
	}
	return out, nil
}
