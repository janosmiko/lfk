package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// CheckRBAC checks what verbs the current user can perform on the given resource.
func (c *Client) CheckRBAC(ctx context.Context, contextName, namespace, group, resource string) ([]RBACCheck, error) {
	clientset, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	verbs := []string{"get", "list", "watch", "create", "update", "patch", "delete"}
	results := make([]RBACCheck, 0, len(verbs))

	for _, verb := range verbs {
		sar := &authorizationv1.SelfSubjectAccessReview{
			Spec: authorizationv1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &authorizationv1.ResourceAttributes{
					Namespace: namespace,
					Verb:      verb,
					Group:     group,
					Resource:  resource,
				},
			},
		}

		result, err := clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("RBAC check failed for verb %s: %w", verb, err)
		}
		results = append(results, RBACCheck{
			Verb:    verb,
			Allowed: result.Status.Allowed,
		})
	}

	return results, nil
}

// GetSelfRules returns all access rules for the current user in the given namespace
// using SelfSubjectRulesReview.
func (c *Client) GetSelfRules(ctx context.Context, contextName, namespace string) ([]AccessRule, error) {
	return c.GetSelfRulesAs(ctx, contextName, namespace, "")
}

// GetSelfRulesAs returns all access rules for the specified user/ServiceAccount in the given namespace.
// The asUser parameter should be in the format "system:serviceaccount:<namespace>:<name>" for ServiceAccounts.
// If asUser is empty, it checks the current user's permissions.
func (c *Client) GetSelfRulesAs(ctx context.Context, contextName, namespace, asUser string) ([]AccessRule, error) {
	cfg, err := c.restConfigForContext(contextName)
	if err != nil {
		return nil, err
	}

	if asUser != "" {
		if after, ok := strings.CutPrefix(asUser, "group:"); ok {
			// Group impersonation: set a neutral user and the group.
			groupName := after
			cfg.Impersonate = rest.ImpersonationConfig{
				UserName: "system:anonymous",
				Groups:   []string{groupName},
			}
		} else {
			// User or ServiceAccount impersonation.
			cfg.Impersonate = rest.ImpersonationConfig{
				UserName: asUser,
			}
		}
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating clientset: %w", err)
	}

	review := &authorizationv1.SelfSubjectRulesReview{
		Spec: authorizationv1.SelfSubjectRulesReviewSpec{
			Namespace: namespace,
		},
	}

	result, err := clientset.AuthorizationV1().SelfSubjectRulesReviews().Create(ctx, review, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("SelfSubjectRulesReview failed: %w", err)
	}

	rules := make([]AccessRule, 0, len(result.Status.ResourceRules))
	for _, r := range result.Status.ResourceRules {
		rules = append(rules, AccessRule{
			Verbs:         r.Verbs,
			APIGroups:     r.APIGroups,
			Resources:     r.Resources,
			ResourceNames: r.ResourceNames,
		})
	}

	return rules, nil
}

// maxMultiNSNamespaces caps the number of namespaces queried in GetSelfRulesMultiNS
// to avoid overwhelming the API server.
const maxMultiNSNamespaces = 50

// GetSelfRulesMultiNS discovers all namespaces where the given ServiceAccount has
// RoleBindings and queries SelfSubjectRulesReview for each one (plus the SA's own
// namespace). The results are merged into a union of AccessRules. The second return
// value is the sorted list of namespaces that were queried.
//
// The asUser parameter must be in the format "system:serviceaccount:<namespace>:<name>".
func (c *Client) GetSelfRulesMultiNS(ctx context.Context, contextName, asUser string) ([]AccessRule, []string, error) {
	// Parse the SA identity from the impersonation string.
	parts := strings.Split(asUser, ":")
	if len(parts) != 4 || parts[0] != "system" || parts[1] != "serviceaccount" {
		return nil, nil, fmt.Errorf("invalid service account format: %q (expected system:serviceaccount:<ns>:<name>)", asUser)
	}
	saNamespace := parts[2]
	saName := parts[3]

	clientset, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, nil, err
	}

	// Collect namespaces where this SA has RoleBindings.
	nsSet := map[string]struct{}{
		saNamespace: {}, // always include the SA's own namespace
	}

	// List all RoleBindings across all namespaces and filter for matching subjects.
	rbList, err := clientset.RbacV1().RoleBindings("").List(ctx, metav1.ListOptions{})
	if err != nil {
		// If we can't list RoleBindings (permission denied), fall back to just the SA namespace.
		rbList = &rbacv1.RoleBindingList{}
	}
	for _, rb := range rbList.Items {
		for _, subj := range rb.Subjects {
			if subj.Kind == "ServiceAccount" && subj.Name == saName && subj.Namespace == saNamespace {
				nsSet[rb.Namespace] = struct{}{}
				break
			}
		}
	}

	// Also check ClusterRoleBindings — if the SA is bound at cluster scope, the
	// SelfSubjectRulesReview in any namespace will already reflect those permissions,
	// but we note this for completeness. No extra namespaces to add here.

	// Build the sorted, capped namespace list.
	namespaces := make([]string, 0, len(nsSet))
	for ns := range nsSet {
		namespaces = append(namespaces, ns)
	}
	sort.Strings(namespaces)
	if len(namespaces) > maxMultiNSNamespaces {
		namespaces = namespaces[:maxMultiNSNamespaces]
	}

	// Query SelfSubjectRulesReview for each namespace in parallel.
	type nsResult struct {
		rules []AccessRule
		err   error
	}
	results := make([]nsResult, len(namespaces))
	var wg sync.WaitGroup
	wg.Add(len(namespaces))
	for i, ns := range namespaces {
		go func(idx int, namespace string) {
			defer wg.Done()
			rules, rErr := c.GetSelfRulesAs(ctx, contextName, namespace, asUser)
			results[idx] = nsResult{rules: rules, err: rErr}
		}(i, ns)
	}
	wg.Wait()

	// Merge results: deduplicate using a string key for each rule.
	seen := make(map[string]struct{})
	var merged []AccessRule
	for i, res := range results {
		if res.err != nil {
			// If a single namespace fails, log but continue with others.
			// Return an error only if all namespaces failed.
			if len(namespaces) == 1 {
				return nil, nil, fmt.Errorf("rules review for namespace %q: %w", namespaces[i], res.err)
			}
			continue
		}
		for _, rule := range res.rules {
			key := ruleKey(rule)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, rule)
		}
	}

	return merged, namespaces, nil
}

// ruleKey produces a deterministic string key for deduplicating AccessRules.
func ruleKey(r AccessRule) string {
	return strings.Join(r.Verbs, ",") + "|" +
		strings.Join(r.APIGroups, ",") + "|" +
		strings.Join(r.Resources, ",") + "|" +
		strings.Join(r.ResourceNames, ",")
}

// ListServiceAccounts returns the names of all ServiceAccounts in the given namespace.
func (c *Client) ListServiceAccounts(ctx context.Context, contextName, namespace string) ([]string, error) {
	clientset, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	saList, err := clientset.CoreV1().ServiceAccounts(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing service accounts: %w", err)
	}

	names := make([]string, 0, len(saList.Items))
	for _, sa := range saList.Items {
		if namespace == "" {
			names = append(names, sa.Namespace+"/"+sa.Name)
		} else {
			names = append(names, sa.Name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// ListRBACSubjects lists all unique subjects (User, Group, ServiceAccount) from
// ClusterRoleBindings and RoleBindings across all namespaces. Results are
// deduplicated and sorted by Kind (User, Group, ServiceAccount) then Name.
func (c *Client) ListRBACSubjects(ctx context.Context, contextName string) ([]RBACSubject, error) {
	clientset, err := c.clientsetForContext(contextName)
	if err != nil {
		return nil, err
	}

	type subjectKey struct {
		Kind      string
		Name      string
		Namespace string
	}
	seen := make(map[subjectKey]struct{})

	// Collect subjects from ClusterRoleBindings.
	crbList, err := clientset.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	if err != nil {
		// Permission denied is non-fatal; continue with RoleBindings.
		crbList = &rbacv1.ClusterRoleBindingList{}
	}
	for _, crb := range crbList.Items {
		for _, subj := range crb.Subjects {
			switch subj.Kind {
			case "User", "Group", "ServiceAccount":
				seen[subjectKey{Kind: subj.Kind, Name: subj.Name, Namespace: subj.Namespace}] = struct{}{}
			}
		}
	}

	// Collect subjects from RoleBindings across all namespaces.
	rbList, err := clientset.RbacV1().RoleBindings("").List(ctx, metav1.ListOptions{})
	if err != nil {
		// Permission denied is non-fatal.
		rbList = &rbacv1.RoleBindingList{}
	}
	for _, rb := range rbList.Items {
		for _, subj := range rb.Subjects {
			switch subj.Kind {
			case "User", "Group", "ServiceAccount":
				ns := subj.Namespace
				if subj.Kind == "ServiceAccount" && ns == "" {
					ns = rb.Namespace
				}
				seen[subjectKey{Kind: subj.Kind, Name: subj.Name, Namespace: ns}] = struct{}{}
			}
		}
	}

	subjects := make([]RBACSubject, 0, len(seen))
	for key := range seen {
		subjects = append(subjects, RBACSubject(key))
	}

	// Sort: Users first, then Groups, then ServiceAccounts; within each kind by name.
	kindOrder := map[string]int{"User": 0, "Group": 1, "ServiceAccount": 2}
	sort.Slice(subjects, func(i, j int) bool {
		if kindOrder[subjects[i].Kind] != kindOrder[subjects[j].Kind] {
			return kindOrder[subjects[i].Kind] < kindOrder[subjects[j].Kind]
		}
		if subjects[i].Name != subjects[j].Name {
			return subjects[i].Name < subjects[j].Name
		}
		return subjects[i].Namespace < subjects[j].Namespace
	})

	return subjects, nil
}

// GetNamespaceQuotas lists ResourceQuota objects in the given namespace
// and computes per-resource usage percentages.
func (c *Client) GetNamespaceQuotas(ctx context.Context, kubeCtx, namespace string) ([]QuotaInfo, error) {
	dynClient, err := c.dynamicForContext(kubeCtx)
	if err != nil {
		return nil, err
	}

	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "resourcequotas"}

	var lister dynamic.ResourceInterface
	if namespace != "" {
		lister = dynClient.Resource(gvr).Namespace(namespace)
	} else {
		lister = dynClient.Resource(gvr)
	}

	list, err := lister.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing resourcequotas: %w", err)
	}

	quotas := make([]QuotaInfo, 0, len(list.Items))
	for _, item := range list.Items {
		qi := QuotaInfo{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
		}

		spec, _ := item.Object["spec"].(map[string]any)
		status, _ := item.Object["status"].(map[string]any)

		hardMap, _ := spec["hard"].(map[string]any)
		usedMap := map[string]any{}
		if status != nil {
			usedMap, _ = status["used"].(map[string]any)
		}

		// Collect resource names from hard limits.
		for resName, hardVal := range hardMap {
			hardStr := fmt.Sprintf("%v", hardVal)
			usedStr := "0"
			if uv, ok := usedMap[resName]; ok {
				usedStr = fmt.Sprintf("%v", uv)
			}

			pct := computeQuotaPercent(resName, hardStr, usedStr)

			qi.Resources = append(qi.Resources, QuotaResource{
				Name:    resName,
				Hard:    hardStr,
				Used:    usedStr,
				Percent: pct,
			})
		}

		// Sort resources by name for stable display order.
		sort.Slice(qi.Resources, func(i, j int) bool {
			return qi.Resources[i].Name < qi.Resources[j].Name
		})

		quotas = append(quotas, qi)
	}

	return quotas, nil
}

// computeQuotaPercent computes the usage percentage for a quota resource.
// For resources like cpu and memory, it parses Kubernetes quantity strings.
// For simple numeric resources (pods, services, etc.), it parses as integers.
func computeQuotaPercent(_, hardStr, usedStr string) float64 {
	// Try to parse as Kubernetes quantities (handles cpu, memory, storage, etc.)
	hardQty, errH := resource.ParseQuantity(hardStr)
	usedQty, errU := resource.ParseQuantity(usedStr)
	if errH == nil && errU == nil {
		hardVal := hardQty.AsApproximateFloat64()
		usedVal := usedQty.AsApproximateFloat64()
		if hardVal > 0 {
			pct := (usedVal / hardVal) * 100
			if pct > 100 {
				pct = 100
			}
			return pct
		}
		return 0
	}

	// Fallback: shouldn't normally be reached since resource.ParseQuantity
	// handles both quantities and plain integers, but guard against it.
	return 0
}

// EventInfo holds a single Kubernetes event with its key fields.
type EventInfo struct {
	Timestamp    time.Time
	Type         string // "Normal" or "Warning"
	Reason       string
	Message      string
	Source       string // e.g. "kubelet", "scheduler"
	Count        int32
	InvolvedName string
	InvolvedKind string
}

// GetResourceEvents fetches events related to the named resource and its owned
// resources (using a name-prefix heuristic). Events are returned sorted by
// timestamp descending (most recent first).
func (c *Client) GetResourceEvents(ctx context.Context, kubeCtx, namespace, name, kind string) ([]EventInfo, error) {
	dynClient, err := c.dynamicForContext(kubeCtx)
	if err != nil {
		return nil, err
	}

	eventGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "events",
	}

	list, err := dynClient.Resource(eventGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing events: %w", err)
	}

	// Kinds that create child resources with name prefixes (e.g. Deployment "foo"
	// creates ReplicaSet "foo-abc", which creates Pod "foo-abc-xyz").
	prefixKinds := map[string]bool{
		"Deployment":  true,
		"StatefulSet": true,
		"DaemonSet":   true,
		"ReplicaSet":  true,
		"Job":         true,
		"CronJob":     true,
	}
	allowPrefix := prefixKinds[kind]
	namePrefix := name + "-"

	events := make([]EventInfo, 0, len(list.Items))
	for _, item := range list.Items {
		involved, _, _ := unstructured.NestedMap(item.Object, "involvedObject")
		if involved == nil {
			continue
		}
		involvedName, _ := involved["name"].(string)
		involvedKind, _ := involved["kind"].(string)

		// Exact name match requires the kind to match as well.
		// Prefix match (for owned child resources) is only allowed for kinds
		// that create children with the parent name as a prefix.
		exactMatch := involvedName == name && involvedKind == kind
		prefixMatch := allowPrefix && involvedName != name && strings.HasPrefix(involvedName, namePrefix)
		if !exactMatch && !prefixMatch {
			continue
		}

		eventType, _, _ := unstructured.NestedString(item.Object, "type")
		reason, _, _ := unstructured.NestedString(item.Object, "reason")
		message, _, _ := unstructured.NestedString(item.Object, "message")

		// Extract source component.
		source, _, _ := unstructured.NestedString(item.Object, "source", "component")
		if source == "" {
			source, _, _ = unstructured.NestedString(item.Object, "reportingComponent")
		}

		// Extract count.
		countVal, _, _ := unstructured.NestedInt64(item.Object, "count")

		// Extract timestamp: prefer lastTimestamp, fall back to eventTime, then metadata.creationTimestamp.
		var ts time.Time
		if lastTS, _, _ := unstructured.NestedString(item.Object, "lastTimestamp"); lastTS != "" {
			ts, _ = time.Parse(time.RFC3339, lastTS)
		}
		if ts.IsZero() {
			if eventTime, _, _ := unstructured.NestedString(item.Object, "eventTime"); eventTime != "" {
				ts, _ = time.Parse(time.RFC3339, eventTime)
			}
		}
		if ts.IsZero() {
			if ct, _, _ := unstructured.NestedString(item.Object, "metadata", "creationTimestamp"); ct != "" {
				ts, _ = time.Parse(time.RFC3339, ct)
			}
		}

		events = append(events, EventInfo{
			Timestamp:    ts,
			Type:         eventType,
			Reason:       reason,
			Message:      message,
			Source:       source,
			Count:        int32(countVal),
			InvolvedName: involvedName,
			InvolvedKind: involvedKind,
		})
	}

	// Sort by timestamp descending (most recent first).
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.After(events[j].Timestamp)
	})

	return events, nil
}

// GetPodsUsingPVC returns the names of pods that reference the given PVC in the specified namespace.
func (c *Client) GetPodsUsingPVC(ctx context.Context, kubeCtx, namespace, pvcName string) ([]string, error) {
	dynClient, err := c.dynamicForContext(kubeCtx)
	if err != nil {
		return nil, err
	}

	podGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}

	list, err := dynClient.Resource(podGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}

	var podNames []string
	for _, item := range list.Items {
		spec, ok := item.Object["spec"].(map[string]any)
		if !ok {
			continue
		}
		volumes, ok := spec["volumes"].([]any)
		if !ok {
			continue
		}
		for _, v := range volumes {
			vol, ok := v.(map[string]any)
			if !ok {
				continue
			}
			pvc, ok := vol["persistentVolumeClaim"].(map[string]any)
			if !ok {
				continue
			}
			if claimName, _ := pvc["claimName"].(string); claimName == pvcName {
				podNames = append(podNames, item.GetName())
				break
			}
		}
	}

	sort.Strings(podNames)
	return podNames, nil
}

// PatchLabels patches the labels on a resource using a merge patch.
func (c *Client) PatchLabels(ctx context.Context, contextName, namespace, name string, gvr schema.GroupVersionResource, labels map[string]any) error {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	patch := map[string]any{
		"metadata": map[string]any{
			"labels": labels,
		},
	}

	data, err := json.Marshal(patch)
	if err != nil {
		return err
	}

	if namespace != "" {
		_, err = dynClient.Resource(gvr).Namespace(namespace).Patch(ctx, name, k8stypes.MergePatchType, data, metav1.PatchOptions{})
	} else {
		_, err = dynClient.Resource(gvr).Patch(ctx, name, k8stypes.MergePatchType, data, metav1.PatchOptions{})
	}
	return err
}

// PatchAnnotations patches the annotations on a resource using a merge patch.
func (c *Client) PatchAnnotations(ctx context.Context, contextName, namespace, name string, gvr schema.GroupVersionResource, annotations map[string]any) error {
	dynClient, err := c.dynamicForContext(contextName)
	if err != nil {
		return err
	}

	patch := map[string]any{
		"metadata": map[string]any{
			"annotations": annotations,
		},
	}

	data, err := json.Marshal(patch)
	if err != nil {
		return err
	}

	if namespace != "" {
		_, err = dynClient.Resource(gvr).Namespace(namespace).Patch(ctx, name, k8stypes.MergePatchType, data, metav1.PatchOptions{})
	} else {
		_, err = dynClient.Resource(gvr).Patch(ctx, name, k8stypes.MergePatchType, data, metav1.PatchOptions{})
	}
	return err
}
