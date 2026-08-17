package k8s

import (
	"context"
	"fmt"
	"maps"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// NetpolsForResource holds the network policies that select a pod or a
// service's backing pods, shown by the "Network Policies" action.
type NetpolsForResource struct {
	Kind        string // "Pod" or "Service"
	Name        string
	Namespace   string
	NoSelector  bool     // Service only: the service defines no pod selector
	BackingPods []string // Service only: pods backing the service
	Policies    []NetpolForResource
}

// NetpolForResource is a matching policy plus, for services, the subset of
// the service's backing pods it selects.
type NetpolForResource struct {
	NetworkPolicyInfo
	MatchedPods []string // Service only: backing pods this policy selects
}

// GetNetworkPoliciesForPod returns the NetworkPolicies in the pod's namespace
// whose podSelector selects the given pod.
func (c *Client) GetNetworkPoliciesForPod(ctx context.Context, kubeCtx, namespace, podName string) (*NetpolsForResource, error) {
	dynClient, err := c.dynamicForContext(kubeCtx)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	pod, err := dynClient.Resource(podGVR).Namespace(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting pod %s: %w", podName, err)
	}
	podLabels := labels.Set(pod.GetLabels())

	policies, pods, err := listNetpolsAndPods(ctx, dynClient, namespace)
	if err != nil {
		return nil, fmt.Errorf("loading policies for pod %s: %w", podName, err)
	}

	result := &NetpolsForResource{Kind: "Pod", Name: podName, Namespace: namespace}
	for i := range policies {
		sel, info := parseNetpolForMatch(&policies[i])
		if sel == nil || !sel.Matches(podLabels) {
			continue
		}
		info.AffectedPods = matchingPodNames(pods, sel)
		result.Policies = append(result.Policies, NetpolForResource{NetworkPolicyInfo: *info})
	}

	clusterPods := lazyClusterPods(ctx, dynClient, pods)
	nsLabels := lazyNamespaceLabels(ctx, dynClient)
	for _, ps := range listCiliumPolicySpecs(ctx, dynClient, namespace) {
		if ps.selector == nil {
			continue
		}
		augmented := ciliumPodLabels(pod.GetLabels(), namespace, nsLabels()[namespace])
		if !ps.selector.Matches(augmented) {
			continue
		}
		info := ps.info
		info.AffectedPods = matchingCiliumPodNames(ciliumAffectedPodPool(ps, pods, clusterPods), ps.selector, nsLabels())
		result.Policies = append(result.Policies, NetpolForResource{NetworkPolicyInfo: info})
	}

	sortNetpolsByName(result.Policies)
	return result, nil
}

// GetNetworkPoliciesForService returns the NetworkPolicies in the service's
// namespace that select at least one of the service's backing pods. Each
// returned policy records which backing pods it covers, since a policy may
// select only a subset of them.
func (c *Client) GetNetworkPoliciesForService(ctx context.Context, kubeCtx, namespace, svcName string) (*NetpolsForResource, error) {
	dynClient, err := c.dynamicForContext(kubeCtx)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	svcGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}
	svc, err := dynClient.Resource(svcGVR).Namespace(namespace).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting service %s: %w", svcName, err)
	}

	result := &NetpolsForResource{Kind: "Service", Name: svcName, Namespace: namespace}

	selector, _, _ := unstructured.NestedStringMap(svc.Object, "spec", "selector")
	if len(selector) == 0 {
		// ExternalName services or manually-managed endpoints: pod-selector
		// based policies cannot be resolved against this service.
		result.NoSelector = true
		return result, nil
	}
	svcSelector := labels.SelectorFromSet(selector)

	policies, pods, err := listNetpolsAndPods(ctx, dynClient, namespace)
	if err != nil {
		return nil, fmt.Errorf("loading policies for service %s: %w", svcName, err)
	}

	result.BackingPods = matchingPodNames(pods, svcSelector)

	// A policy counts if it selects at least one backing pod. This includes
	// namespace-wide policies (empty podSelector): they do apply to the
	// service's pods, and the per-policy coverage line shows the extent.
	for i := range policies {
		sel, info := parseNetpolForMatch(&policies[i])
		if sel == nil {
			continue
		}
		var matched []string
		for _, p := range pods {
			podLabels := labels.Set(p.GetLabels())
			if svcSelector.Matches(podLabels) && sel.Matches(podLabels) {
				matched = append(matched, p.GetName())
			}
		}
		if len(matched) == 0 {
			continue
		}
		sort.Strings(matched)
		info.AffectedPods = matchingPodNames(pods, sel)
		result.Policies = append(result.Policies, NetpolForResource{NetworkPolicyInfo: *info, MatchedPods: matched})
	}

	clusterPods := lazyClusterPods(ctx, dynClient, pods)
	nsLabels := lazyNamespaceLabels(ctx, dynClient)
	for _, ps := range listCiliumPolicySpecs(ctx, dynClient, namespace) {
		if ps.selector == nil {
			continue
		}
		var matched []string
		for _, p := range pods {
			if !svcSelector.Matches(labels.Set(p.GetLabels())) {
				continue
			}
			if ps.selector.Matches(ciliumPodLabels(p.GetLabels(), namespace, nsLabels()[namespace])) {
				matched = append(matched, p.GetName())
			}
		}
		if len(matched) == 0 {
			continue
		}
		sort.Strings(matched)
		info := ps.info
		info.AffectedPods = matchingCiliumPodNames(ciliumAffectedPodPool(ps, pods, clusterPods), ps.selector, nsLabels())
		result.Policies = append(result.Policies, NetpolForResource{NetworkPolicyInfo: info, MatchedPods: matched})
	}

	sortNetpolsByName(result.Policies)
	return result, nil
}

// listNetpolsAndPods lists the namespace's network policies and pods in two
// calls, so per-policy matching happens locally instead of one API list per
// policy.
func listNetpolsAndPods(ctx context.Context, dynClient dynamic.Interface, namespace string) ([]unstructured.Unstructured, []unstructured.Unstructured, error) {
	netpolGVR := schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}
	polList, err := dynClient.Resource(netpolGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("listing network policies: %w", err)
	}

	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	podList, err := dynClient.Resource(podGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("listing pods: %w", err)
	}

	return polList.Items, podList.Items, nil
}

// parseNetpolForMatch parses a NetworkPolicy object into a label selector and
// a display info struct. The selector honors both matchLabels and
// matchExpressions. An empty podSelector selects all pods in the namespace.
// Returns a nil selector if the podSelector cannot be parsed.
func parseNetpolForMatch(obj *unstructured.Unstructured) (labels.Selector, *NetworkPolicyInfo) {
	info := &NetworkPolicyInfo{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}

	// An absent or empty podSelector selects all pods in the namespace, so
	// the zero LabelSelector (-> labels.Everything) is the correct default.
	var ls metav1.LabelSelector
	spec, _ := obj.Object["spec"].(map[string]any)
	if spec != nil {
		populateNetpolSpec(info, spec)
		if podSel, found, _ := unstructured.NestedMap(spec, "podSelector"); found {
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(podSel, &ls); err != nil {
				return nil, info
			}
		}
	}
	sel, err := metav1.LabelSelectorAsSelector(&ls)
	if err != nil {
		return nil, info
	}
	return sel, info
}

// matchingPodNames returns the sorted names of pods whose labels match the
// selector.
func matchingPodNames(pods []unstructured.Unstructured, sel labels.Selector) []string {
	var names []string
	for _, p := range pods {
		if sel.Matches(labels.Set(p.GetLabels())) {
			names = append(names, p.GetName())
		}
	}
	sort.Strings(names)
	return names
}

// ciliumGVRs for the two Cilium policy CRDs.
var (
	ciliumCNPGVR  = schema.GroupVersionResource{Group: "cilium.io", Version: "v2", Resource: "ciliumnetworkpolicies"}
	ciliumCCNPGVR = schema.GroupVersionResource{Group: "cilium.io", Version: "v2", Resource: "ciliumclusterwidenetworkpolicies"}
)

// listCiliumPolicySpecs lists CiliumNetworkPolicies in the namespace plus
// all CiliumClusterwideNetworkPolicies, parsed into per-spec entries.
// Errors are swallowed deliberately: clusters without the Cilium CRDs (or
// without list permission) simply contribute nothing.
func listCiliumPolicySpecs(ctx context.Context, dynClient dynamic.Interface, namespace string) []ciliumParsedSpec {
	var out []ciliumParsedSpec
	if list, err := dynClient.Resource(ciliumCNPGVR).Namespace(namespace).List(ctx, metav1.ListOptions{}); err == nil {
		for i := range list.Items {
			out = append(out, parseCiliumPolicy(&list.Items[i], "CiliumNetworkPolicy")...)
		}
	}
	if list, err := dynClient.Resource(ciliumCCNPGVR).List(ctx, metav1.ListOptions{}); err == nil {
		for i := range list.Items {
			out = append(out, parseCiliumPolicy(&list.Items[i], "CiliumClusterwideNetworkPolicy")...)
		}
	}
	return out
}

// ciliumNamespaceLabelPrefix is the prefix Cilium adds to each namespace label
// when projecting it onto endpoints in that namespace.
const ciliumNamespaceLabelPrefix = "io.cilium.k8s.namespace.labels."

// ciliumPodLabels returns the pod's labels augmented with the pseudo-labels
// Cilium injects on endpoints: the namespace name, plus every label of the
// pod's namespace under the io.cilium.k8s.namespace.labels.<key> prefix. This
// lets selectors that scope by namespace or by namespace label match
// correctly. Standard NetworkPolicy matching must NOT use this: pods do not
// actually carry the pseudo-labels. nsOwnLabels is the label map of the pod's
// namespace (nil is fine).
func ciliumPodLabels(lbls map[string]string, namespace string, nsOwnLabels map[string]string) labels.Set {
	set := make(labels.Set, len(lbls)+len(nsOwnLabels)+1)
	maps.Copy(set, lbls)
	set["io.kubernetes.pod.namespace"] = namespace
	for k, v := range nsOwnLabels {
		set[ciliumNamespaceLabelPrefix+k] = v
	}
	return set
}

// lazyNamespaceLabels returns a function that lists all namespaces once, on
// first use, into a namespace-name -> labels map. Cilium namespace-derived
// selectors need this to resolve which namespaces carry a given label. On list
// failure (e.g. no permission) it yields an empty map, disabling only the
// namespace-derived matching.
func lazyNamespaceLabels(ctx context.Context, dynClient dynamic.Interface) func() map[string]map[string]string {
	var cached map[string]map[string]string
	nsGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	return func() map[string]map[string]string {
		if cached != nil {
			return cached
		}
		cached = map[string]map[string]string{}
		if list, err := dynClient.Resource(nsGVR).List(ctx, metav1.ListOptions{}); err == nil {
			for i := range list.Items {
				cached[list.Items[i].GetName()] = list.Items[i].GetLabels()
			}
		}
		return cached
	}
}

// lazyClusterPods returns a function that lists pods across all namespaces
// once, on first use. Clusterwide Cilium policies need the full list to
// report affected pods accurately. On list failure it falls back to the
// namespace-scoped pods.
func lazyClusterPods(ctx context.Context, dynClient dynamic.Interface, fallback []unstructured.Unstructured) func() []unstructured.Unstructured {
	var cached []unstructured.Unstructured
	loaded := false
	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	return func() []unstructured.Unstructured {
		if !loaded {
			loaded = true
			if list, err := dynClient.Resource(podGVR).List(ctx, metav1.ListOptions{}); err == nil {
				cached = list.Items
			} else {
				cached = fallback
			}
		}
		return cached
	}
}

// ciliumAffectedPodPool picks the pod pool a policy's affected-pods list is
// computed from: namespace pods for a namespaced CNP, all cluster pods for a
// clusterwide policy.
func ciliumAffectedPodPool(ps ciliumParsedSpec, nsPods []unstructured.Unstructured, clusterPods func() []unstructured.Unstructured) []unstructured.Unstructured {
	if ps.info.Kind == "CiliumClusterwideNetworkPolicy" {
		return clusterPods()
	}
	return nsPods
}

// matchingCiliumPodNames is matchingPodNames with Cilium-augmented labels;
// each pod is augmented with its own namespace pseudo-labels, resolved from
// nsLabels (namespace name -> namespace labels).
func matchingCiliumPodNames(pods []unstructured.Unstructured, sel labels.Selector, nsLabels map[string]map[string]string) []string {
	var names []string
	for _, p := range pods {
		ns := p.GetNamespace()
		if sel.Matches(ciliumPodLabels(p.GetLabels(), ns, nsLabels[ns])) {
			names = append(names, p.GetName())
		}
	}
	sort.Strings(names)
	return names
}

// sortNetpolsByName sorts policies alphabetically for deterministic display.
func sortNetpolsByName(policies []NetpolForResource) {
	sort.Slice(policies, func(i, j int) bool {
		return policies[i].Name < policies[j].Name
	})
}
