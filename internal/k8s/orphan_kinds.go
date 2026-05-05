package k8s

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

// rbacIndex tracks which (Cluster)Roles are bound and which Roles
// exist (so binding.roleRef validation can flag dangling references).
// A RoleBinding's roleRef can target either a Role (in the binding's
// own namespace) or a ClusterRole (cluster-wide); a ClusterRoleBinding
// only ever targets a ClusterRole.
type rbacIndex struct {
	// boundRoles tracks Role refs by (binding.namespace, role.Name).
	// RoleBindings that reference a Role pull that role's
	// (binding.namespace, roleRef.Name) into this set since Roles are
	// scoped to the binding's namespace.
	boundRoles map[k8stypes.NamespacedName]struct{}
	// boundClusterRoles tracks ClusterRole names referenced by EITHER
	// (Cluster)RoleBinding kind. Cluster-scoped, so just a name set.
	boundClusterRoles map[string]struct{}
	// existingRoles / existingClusterRoles let RoleBinding /
	// ClusterRoleBinding orphan checks detect dangling roleRefs.
	existingRoles        map[k8stypes.NamespacedName]struct{}
	existingClusterRoles map[string]struct{}
}

func buildRBACIndex(
	roles []rbacv1.Role, clusterRoles []rbacv1.ClusterRole,
	rbs []rbacv1.RoleBinding, crbs []rbacv1.ClusterRoleBinding,
) rbacIndex {
	idx := rbacIndex{
		boundRoles:           make(map[k8stypes.NamespacedName]struct{}),
		boundClusterRoles:    make(map[string]struct{}),
		existingRoles:        make(map[k8stypes.NamespacedName]struct{}),
		existingClusterRoles: make(map[string]struct{}),
	}
	for _, r := range roles {
		idx.existingRoles[k8stypes.NamespacedName{Namespace: r.Namespace, Name: r.Name}] = struct{}{}
	}
	for _, cr := range clusterRoles {
		idx.existingClusterRoles[cr.Name] = struct{}{}
	}
	for _, rb := range rbs {
		switch rb.RoleRef.Kind {
		case "Role":
			// Roles live in the binding's namespace.
			idx.boundRoles[k8stypes.NamespacedName{Namespace: rb.Namespace, Name: rb.RoleRef.Name}] = struct{}{}
		case "ClusterRole":
			idx.boundClusterRoles[rb.RoleRef.Name] = struct{}{}
		}
	}
	for _, crb := range crbs {
		// ClusterRoleBindings only reference ClusterRoles.
		idx.boundClusterRoles[crb.RoleRef.Name] = struct{}{}
	}
	return idx
}

// roleOrphan: Role with no RoleBinding referencing it.
func roleOrphan(r rbacv1.Role, idx rbacIndex) (OrphanItem, bool) {
	if len(r.OwnerReferences) > 0 {
		return OrphanItem{}, false
	}
	if _, bound := idx.boundRoles[k8stypes.NamespacedName{Namespace: r.Namespace, Name: r.Name}]; bound {
		return OrphanItem{}, false
	}
	return OrphanItem{
		Kind: "Role", Namespace: r.Namespace, Name: r.Name,
		Reason: "no binding",
	}, true
}

// clusterRoleOrphan: ClusterRole with no (Cluster)RoleBinding
// referencing it. Skips the well-known system ClusterRoles
// (system:*, edit, admin, view, cluster-admin) and aggregated roles
// (those with .aggregationRule != nil) because those are kube-managed
// regardless of binding state — flagging them would be noise.
func clusterRoleOrphan(cr rbacv1.ClusterRole, idx rbacIndex) (OrphanItem, bool) {
	if len(cr.OwnerReferences) > 0 {
		return OrphanItem{}, false
	}
	if cr.AggregationRule != nil {
		return OrphanItem{}, false
	}
	switch cr.Name {
	case "edit", "admin", "view", "cluster-admin":
		return OrphanItem{}, false
	}
	if strings.HasPrefix(cr.Name, "system:") {
		return OrphanItem{}, false
	}
	if _, bound := idx.boundClusterRoles[cr.Name]; bound {
		return OrphanItem{}, false
	}
	return OrphanItem{
		Kind: "ClusterRole", Name: cr.Name,
		Reason: "no binding",
	}, true
}

// roleBindingOrphan: RoleBinding with empty subjects or a roleRef
// pointing to a missing Role/ClusterRole.
func roleBindingOrphan(rb rbacv1.RoleBinding, idx rbacIndex) (OrphanItem, bool) {
	if len(rb.OwnerReferences) > 0 {
		return OrphanItem{}, false
	}
	if len(rb.Subjects) == 0 {
		return OrphanItem{
			Kind: "RoleBinding", Namespace: rb.Namespace, Name: rb.Name,
			Reason: "empty subjects",
		}, true
	}
	switch rb.RoleRef.Kind {
	case "Role":
		if _, ok := idx.existingRoles[k8stypes.NamespacedName{Namespace: rb.Namespace, Name: rb.RoleRef.Name}]; !ok {
			return OrphanItem{
				Kind: "RoleBinding", Namespace: rb.Namespace, Name: rb.Name,
				Reason: fmt.Sprintf("missing role: Role/%s", rb.RoleRef.Name),
			}, true
		}
	case "ClusterRole":
		if _, ok := idx.existingClusterRoles[rb.RoleRef.Name]; !ok {
			return OrphanItem{
				Kind: "RoleBinding", Namespace: rb.Namespace, Name: rb.Name,
				Reason: fmt.Sprintf("missing role: ClusterRole/%s", rb.RoleRef.Name),
			}, true
		}
	}
	return OrphanItem{}, false
}

// clusterRoleBindingOrphan: ClusterRoleBinding with empty subjects or
// a roleRef pointing to a missing ClusterRole.
func clusterRoleBindingOrphan(crb rbacv1.ClusterRoleBinding, idx rbacIndex) (OrphanItem, bool) {
	if len(crb.OwnerReferences) > 0 {
		return OrphanItem{}, false
	}
	if len(crb.Subjects) == 0 {
		return OrphanItem{
			Kind: "ClusterRoleBinding", Name: crb.Name,
			Reason: "empty subjects",
		}, true
	}
	if _, ok := idx.existingClusterRoles[crb.RoleRef.Name]; !ok {
		return OrphanItem{
			Kind: "ClusterRoleBinding", Name: crb.Name,
			Reason: fmt.Sprintf("missing role: ClusterRole/%s", crb.RoleRef.Name),
		}, true
	}
	return OrphanItem{}, false
}

// workloadIndex resolves whether (kind, namespace, name) names an
// existing workload. Used to validate HPA scaleTargetRefs.
type workloadIndex struct {
	keys map[workloadKey]struct{}
}

type workloadKey struct {
	kind, namespace, name string
}

func buildWorkloadIndex(in workloadInputs) workloadIndex {
	idx := workloadIndex{keys: make(map[workloadKey]struct{})}
	for _, d := range in.deployments {
		idx.keys[workloadKey{"Deployment", d.Namespace, d.Name}] = struct{}{}
	}
	for _, ss := range in.statefulSets {
		idx.keys[workloadKey{"StatefulSet", ss.Namespace, ss.Name}] = struct{}{}
	}
	for _, ds := range in.daemonSets {
		idx.keys[workloadKey{"DaemonSet", ds.Namespace, ds.Name}] = struct{}{}
	}
	// ReplicaSets and Pods can theoretically also be scaleTargetRefs but
	// it's rare; HPA targeting bare Pods isn't even valid. Skip those —
	// users with custom-resource HPA targets will see false positives,
	// which is acceptable until someone actually hits one.
	return idx
}

func (w workloadIndex) exists(kind, namespace, name string) bool {
	_, ok := w.keys[workloadKey{kind, namespace, name}]
	return ok
}

// templatePodLabels is a flat list of (namespace, label-set) pairs
// gathered from every workload's PodTemplate plus every live Pod. Used
// by selector-driven orphans (PDB, NetworkPolicy) so a selector
// targeting a scaled-to-zero Deployment still counts as "matching" —
// otherwise every PDB on a CronJob's pods would flag as orphan between
// firings, recreating the false-positive problem we already solved for
// Secrets.
type templatePodLabels []labelledPod

type labelledPod struct {
	namespace string
	labels    map[string]string
}

func collectTemplatePodLabels(in workloadInputs) templatePodLabels {
	out := make(templatePodLabels, 0,
		len(in.pods)+len(in.deployments)+len(in.statefulSets)+
			len(in.daemonSets)+len(in.jobs)+len(in.cronJobs))
	for _, p := range in.pods {
		out = append(out, labelledPod{p.Namespace, p.Labels})
	}
	for _, d := range in.deployments {
		out = append(out, labelledPod{d.Namespace, d.Spec.Template.Labels})
	}
	for _, ss := range in.statefulSets {
		out = append(out, labelledPod{ss.Namespace, ss.Spec.Template.Labels})
	}
	for _, ds := range in.daemonSets {
		out = append(out, labelledPod{ds.Namespace, ds.Spec.Template.Labels})
	}
	for _, j := range in.jobs {
		out = append(out, labelledPod{j.Namespace, j.Spec.Template.Labels})
	}
	for _, cj := range in.cronJobs {
		out = append(out, labelledPod{cj.Namespace, cj.Spec.JobTemplate.Spec.Template.Labels})
	}
	return out
}

// matchesAny returns true when at least one pod-or-template in `pool`
// shares the given namespace and matches the provided selector. Empty
// `selectorSpec` (nil pointer) matches all pods in the namespace —
// which is the API contract for both PDB and NetworkPolicy.
func (t templatePodLabels) matchesAny(namespace string, sel labels.Selector) bool {
	for _, lp := range t {
		if lp.namespace != namespace {
			continue
		}
		if sel.Matches(labels.Set(lp.labels)) {
			return true
		}
	}
	return false
}

// secretOrphan classifies a Secret using both refsets:
//
//   - mounted by a live Pod / Ingress / SA (lenient ref) → not orphan
//   - mounted only by a workload template (strict ref) → lenient-only
//     orphan: "no live consumer", LenientOnly=true. Visible in lenient
//     mode only.
//   - mounted by nothing → strict orphan: "unmounted", LenientOnly=false.
//     Always visible.
func secretOrphan(s corev1.Secret, lenient, strict refSet) (OrphanItem, bool) {
	switch string(s.Type) {
	case "helm.sh/release.v1", string(corev1.SecretTypeServiceAccountToken):
		return OrphanItem{}, false
	}
	if len(s.OwnerReferences) > 0 {
		return OrphanItem{}, false
	}
	nn := k8stypes.NamespacedName{Namespace: s.Namespace, Name: s.Name}
	if _, mounted := lenient.secrets[nn]; mounted {
		return OrphanItem{}, false
	}
	item := OrphanItem{Kind: "Secret", Namespace: s.Namespace, Name: s.Name}
	if _, byTemplate := strict.secrets[nn]; byTemplate {
		item.Reason = "no live consumer"
		item.LenientOnly = true
	} else {
		item.Reason = "unmounted"
	}
	return item, true
}

func configMapOrphan(cm corev1.ConfigMap, lenient, strict refSet) (OrphanItem, bool) {
	if cm.Name == "kube-root-ca.crt" {
		return OrphanItem{}, false
	}
	if len(cm.OwnerReferences) > 0 {
		return OrphanItem{}, false
	}
	nn := k8stypes.NamespacedName{Namespace: cm.Namespace, Name: cm.Name}
	if _, mounted := lenient.configMaps[nn]; mounted {
		return OrphanItem{}, false
	}
	item := OrphanItem{Kind: "ConfigMap", Namespace: cm.Namespace, Name: cm.Name}
	if _, byTemplate := strict.configMaps[nn]; byTemplate {
		item.Reason = "no live consumer"
		item.LenientOnly = true
	} else {
		item.Reason = "unmounted"
	}
	return item, true
}

func podOrphan(pod corev1.Pod) (OrphanItem, bool) {
	if len(pod.OwnerReferences) > 0 {
		return OrphanItem{}, false
	}
	if _, mirror := pod.Annotations[mirrorPodAnnotation]; mirror {
		return OrphanItem{}, false
	}
	reason := "no owner"
	if (pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed) &&
		!pod.CreationTimestamp.IsZero() &&
		time.Since(pod.CreationTimestamp.Time) > time.Hour {
		reason = "no owner (terminal)"
	}
	return OrphanItem{
		Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name, Reason: reason,
	}, true
}

func sortOrphans(items []OrphanItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Namespace != items[j].Namespace {
			return items[i].Namespace < items[j].Namespace
		}
		return items[i].Name < items[j].Name
	})
}

// pvcOrphan classifies a PersistentVolumeClaim using the same dual-mode
// rules as Secret/ConfigMap. Owner-managed PVCs (e.g. those created by
// a StatefulSet's volumeClaimTemplates) are excluded since the parent
// controller manages them. PVCs in `Pending` state are flagged with
// reason "unbound" instead of "unmounted" so the user can distinguish
// provisioning failures from genuinely orphaned claims at a glance.
func pvcOrphan(p corev1.PersistentVolumeClaim, lenient, strict refSet) (OrphanItem, bool) {
	if len(p.OwnerReferences) > 0 {
		return OrphanItem{}, false
	}
	nn := k8stypes.NamespacedName{Namespace: p.Namespace, Name: p.Name}
	if _, mounted := lenient.pvcs[nn]; mounted {
		return OrphanItem{}, false
	}
	item := OrphanItem{Kind: "PersistentVolumeClaim", Namespace: p.Namespace, Name: p.Name}
	if p.Status.Phase == corev1.ClaimPending {
		// "unbound" overrides the strict/lenient distinction — a
		// pending PVC is a real problem regardless of who would mount
		// it once it bound.
		item.Reason = "unbound"
		return item, true
	}
	if _, byTemplate := strict.pvcs[nn]; byTemplate {
		item.Reason = "no live consumer"
		item.LenientOnly = true
	} else {
		item.Reason = "unmounted"
	}
	return item, true
}

// hpaOrphan flags an HPA whose `spec.scaleTargetRef` points at a
// workload that doesn't exist (deleted, typo, or migrated to a
// different kind). Owner-managed HPAs (rare — usually a HelmRelease or
// custom controller) are excluded since the owner manages the
// reference. Custom-resource HPA targets aren't validated here — we'd
// need to walk every CRD; until someone hits a real false positive
// from that, skip the lookup if the target kind isn't one of the
// well-known workload kinds we list.
func hpaOrphan(h autoscalingv2.HorizontalPodAutoscaler, idx workloadIndex) (OrphanItem, bool) {
	if len(h.OwnerReferences) > 0 {
		return OrphanItem{}, false
	}
	target := h.Spec.ScaleTargetRef
	switch target.Kind {
	case "Deployment", "StatefulSet", "DaemonSet":
		// validated below
	default:
		return OrphanItem{}, false
	}
	if idx.exists(target.Kind, h.Namespace, target.Name) {
		return OrphanItem{}, false
	}
	return OrphanItem{
		Kind:      "HorizontalPodAutoscaler",
		Namespace: h.Namespace,
		Name:      h.Name,
		Reason:    fmt.Sprintf("target not found: %s/%s", target.Kind, target.Name),
	}, true
}

// pdbOrphan flags a PodDisruptionBudget whose selector doesn't match
// any live Pod or workload-template Pod label. Including templates is
// what protects a PDB targeting a CronJob from being flagged between
// firings or a Deployment scaled to zero — same false-positive class
// the workload-template walker already fixed for Secrets/ConfigMaps.
//
// Empty selector matches all pods in the namespace per API contract,
// so it's never an orphan. nil selector is invalid; treat as orphan
// with a distinct reason so the user knows what's wrong.
func pdbOrphan(b policyv1.PodDisruptionBudget, _ []corev1.Pod, allLabels templatePodLabels) (OrphanItem, bool) {
	if len(b.OwnerReferences) > 0 {
		return OrphanItem{}, false
	}
	if b.Spec.Selector == nil {
		return OrphanItem{
			Kind: "PodDisruptionBudget", Namespace: b.Namespace, Name: b.Name,
			Reason: "no selector",
		}, true
	}
	sel, err := metav1.LabelSelectorAsSelector(b.Spec.Selector)
	if err != nil {
		// Invalid selector — surface as a distinct reason so the user
		// can fix the manifest rather than seeing a generic "no match".
		return OrphanItem{
			Kind: "PodDisruptionBudget", Namespace: b.Namespace, Name: b.Name,
			Reason: "invalid selector",
		}, true
	}
	if allLabels.matchesAny(b.Namespace, sel) {
		return OrphanItem{}, false
	}
	return OrphanItem{
		Kind:      "PodDisruptionBudget",
		Namespace: b.Namespace,
		Name:      b.Name,
		Reason:    "selects no pods",
	}, true
}

// netpolOrphan flags a NetworkPolicy whose podSelector matches no Pod.
// Same logic as PDB. NetworkPolicy's empty podSelector matches all
// pods in the namespace per API contract, so an empty (but non-nil)
// selector is never orphan.
func netpolOrphan(n networkingv1.NetworkPolicy, _ []corev1.Pod, allLabels templatePodLabels) (OrphanItem, bool) {
	if len(n.OwnerReferences) > 0 {
		return OrphanItem{}, false
	}
	sel, err := metav1.LabelSelectorAsSelector(&n.Spec.PodSelector)
	if err != nil {
		return OrphanItem{
			Kind: "NetworkPolicy", Namespace: n.Namespace, Name: n.Name,
			Reason: "invalid selector",
		}, true
	}
	if allLabels.matchesAny(n.Namespace, sel) {
		return OrphanItem{}, false
	}
	return OrphanItem{
		Kind:      "NetworkPolicy",
		Namespace: n.Namespace,
		Name:      n.Name,
		Reason:    "selects no pods",
	}, true
}

func (c *Client) serviceOrphan(ctx context.Context, kubeContext string, svc corev1.Service) (OrphanItem, bool, error) {
	if svc.Spec.ClusterIP == "None" || svc.Spec.Type == corev1.ServiceTypeExternalName {
		return OrphanItem{}, false, nil
	}
	endpoints, err := c.GetServiceEndpoints(ctx, kubeContext, svc.Namespace, svc.Name)
	if err != nil {
		return OrphanItem{}, false, err
	}
	if endpoints.Ready+endpoints.NotReady > 0 {
		return OrphanItem{}, false, nil
	}
	return OrphanItem{
		Kind: "Service", Namespace: svc.Namespace, Name: svc.Name, Reason: "0/0 endpoints",
	}, true, nil
}
