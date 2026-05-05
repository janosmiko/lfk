package k8s

import (
	"context"
	"errors"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const mirrorPodAnnotation = "kubernetes.io/config.mirror"

// OrphanReport groups orphan resources by kind. Each slice is sorted by
// (namespace, name) so renders are stable across fetches.
type OrphanReport struct {
	Pods                []OrphanItem
	Secrets             []OrphanItem
	ConfigMaps          []OrphanItem
	Services            []OrphanItem
	PVCs                []OrphanItem
	HPAs                []OrphanItem // HorizontalPodAutoscaler
	PDBs                []OrphanItem // PodDisruptionBudget
	NetworkPolicies     []OrphanItem
	Roles               []OrphanItem // not bound by any (Cluster)RoleBinding
	ClusterRoles        []OrphanItem
	RoleBindings        []OrphanItem // empty subjects or missing role
	ClusterRoleBindings []OrphanItem
}

// OrphanItem is a single orphan row. Reason is a stable lowercased
// descriptor (e.g. "no owner", "unmounted", "0/0 endpoints") so external
// tooling and tests can match on it without dealing with localization.
//
// LenientOnly distinguishes two flavours of "unmounted" Secret/ConfigMap:
//
//   - LenientOnly=false: the resource is referenced by NOTHING — no live
//     Pod, no Ingress, no ServiceAccount, AND no workload-controller
//     PodTemplate. Reason is "unmounted". Always shown.
//   - LenientOnly=true: the resource has no live Pod / Ingress / SA ref
//     right now, but a workload template (Deployment/Job/CronJob/...)
//     still references it — typical CronJob between firings or a
//     scaled-to-zero Deployment. Reason is "no live consumer". Hidden by
//     default; the strict-mode toggle in the overlay flips them on so
//     users can audit "what's idle right now" instead of just "what's
//     truly unused".
//
// Pods/Services always have LenientOnly=false — the distinction only
// applies to refset-driven kinds (Secret/ConfigMap).
type OrphanItem struct {
	Namespace   string
	Name        string
	Kind        string // "Pod" | "Secret" | "ConfigMap" | "Service"
	Reason      string
	LenientOnly bool
}

// DetectOrphans scans the cluster (or a single namespace when ns != "")
// and returns every resource that meets the orphan criteria documented in
// docs/superpowers/specs/2026-05-04-orphan-detector-design.md.
//
// Partial RBAC denial is non-fatal: the returned report holds whatever
// kinds the caller's credentials could read, and the error wraps every
// underlying list failure so the UI can render a banner.
// orphanLists bundles every API list response DetectOrphans needs.
// Pulled into its own type so DetectOrphans can stay below the gocyclo
// threshold — the per-list error-handling branches were dominating its
// complexity score.
type orphanLists struct {
	pods                *corev1.PodList
	ingresses           *networkingv1.IngressList
	serviceAccounts     *corev1.ServiceAccountList
	secrets             *corev1.SecretList
	configMaps          *corev1.ConfigMapList
	services            *corev1.ServiceList
	pvcs                *corev1.PersistentVolumeClaimList
	hpas                *autoscalingv2.HorizontalPodAutoscalerList
	pdbs                *policyv1.PodDisruptionBudgetList
	netpols             *networkingv1.NetworkPolicyList
	deployments         *appsv1.DeploymentList
	statefulSets        *appsv1.StatefulSetList
	daemonSets          *appsv1.DaemonSetList
	jobs                *batchv1.JobList
	cronJobs            *batchv1.CronJobList
	roles               *rbacv1.RoleList
	clusterRoles        *rbacv1.ClusterRoleList
	roleBindings        *rbacv1.RoleBindingList
	clusterRoleBindings *rbacv1.ClusterRoleBindingList
}

// fetchOrphanLists pulls every input the detector needs in one place.
// Per-list errors accumulate; the corresponding pointer stays nil and
// the safe*Items helpers turn that into an empty slice so the rest of
// the pipeline runs unaffected.
func fetchOrphanLists(ctx context.Context, cs kubernetes.Interface, namespace string) (orphanLists, []error) {
	var errs []error
	collect := func(name string, err error) {
		if err != nil {
			errs = append(errs, fmt.Errorf("listing %s: %w", name, err))
		}
	}
	opts := metav1.ListOptions{}
	out := orphanLists{}
	var err error
	out.pods, err = cs.CoreV1().Pods(namespace).List(ctx, opts)
	collect("pods", err)
	out.ingresses, err = cs.NetworkingV1().Ingresses(namespace).List(ctx, opts)
	collect("ingresses", err)
	out.serviceAccounts, err = cs.CoreV1().ServiceAccounts(namespace).List(ctx, opts)
	collect("service accounts", err)
	out.secrets, err = cs.CoreV1().Secrets(namespace).List(ctx, opts)
	collect("secrets", err)
	out.configMaps, err = cs.CoreV1().ConfigMaps(namespace).List(ctx, opts)
	collect("configmaps", err)
	out.services, err = cs.CoreV1().Services(namespace).List(ctx, opts)
	collect("services", err)
	out.pvcs, err = cs.CoreV1().PersistentVolumeClaims(namespace).List(ctx, opts)
	collect("pvcs", err)
	out.hpas, err = cs.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(ctx, opts)
	collect("hpas", err)
	out.pdbs, err = cs.PolicyV1().PodDisruptionBudgets(namespace).List(ctx, opts)
	collect("pdbs", err)
	out.netpols, err = cs.NetworkingV1().NetworkPolicies(namespace).List(ctx, opts)
	collect("networkpolicies", err)
	out.deployments, err = cs.AppsV1().Deployments(namespace).List(ctx, opts)
	collect("deployments", err)
	out.statefulSets, err = cs.AppsV1().StatefulSets(namespace).List(ctx, opts)
	collect("statefulsets", err)
	out.daemonSets, err = cs.AppsV1().DaemonSets(namespace).List(ctx, opts)
	collect("daemonsets", err)
	out.jobs, err = cs.BatchV1().Jobs(namespace).List(ctx, opts)
	collect("jobs", err)
	out.cronJobs, err = cs.BatchV1().CronJobs(namespace).List(ctx, opts)
	collect("cronjobs", err)
	out.roles, err = cs.RbacV1().Roles(namespace).List(ctx, opts)
	collect("roles", err)
	out.roleBindings, err = cs.RbacV1().RoleBindings(namespace).List(ctx, opts)
	collect("rolebindings", err)
	// ClusterRole(Binding)s are cluster-scoped — list cluster-wide
	// regardless of the namespace parameter so a namespace-scoped run
	// can still validate that a RoleBinding's roleRef pointing at a
	// ClusterRole resolves.
	out.clusterRoles, err = cs.RbacV1().ClusterRoles().List(ctx, opts)
	collect("clusterroles", err)
	out.clusterRoleBindings, err = cs.RbacV1().ClusterRoleBindings().List(ctx, opts)
	collect("clusterrolebindings", err)
	return out, errs
}

func (c *Client) DetectOrphans(ctx context.Context, kubeContext, namespace string) (OrphanReport, error) {
	cs, err := c.clientsetForContext(kubeContext)
	if err != nil {
		return OrphanReport{}, err
	}

	report := OrphanReport{}
	lists, errs := fetchOrphanLists(ctx, cs, namespace)
	pods := lists.pods
	ingresses := lists.ingresses
	serviceAccounts := lists.serviceAccounts
	secrets := lists.secrets
	configMaps := lists.configMaps
	services := lists.services
	pvcs := lists.pvcs
	hpas := lists.hpas
	pdbs := lists.pdbs
	netpols := lists.netpols
	deployments := lists.deployments
	statefulSets := lists.statefulSets
	daemonSets := lists.daemonSets
	jobs := lists.jobs
	cronJobs := lists.cronJobs
	roles := lists.roles
	clusterRoles := lists.clusterRoles
	roleBindings := lists.roleBindings
	clusterRoleBindings := lists.clusterRoleBindings

	// Two refsets so the UI can switch between strict and lenient
	// definitions without re-fetching. Lenient walks only the
	// "persistent" reference sources — live Pods, Ingresses, and
	// ServiceAccounts — that exist independently of any workload's
	// life cycle. Strict additionally walks PodTemplates so a Secret
	// referenced by a CronJob between firings or a Deployment in the
	// middle of a rollout doesn't get flagged.
	lenientInputs := workloadInputs{
		pods:            safePodItems(pods),
		ingresses:       safeIngressItems(ingresses),
		serviceAccounts: safeSAItems(serviceAccounts),
	}
	strictInputs := lenientInputs
	strictInputs.deployments = safeDeploymentItems(deployments)
	strictInputs.statefulSets = safeStatefulSetItems(statefulSets)
	strictInputs.daemonSets = safeDaemonSetItems(daemonSets)
	strictInputs.jobs = safeJobItems(jobs)
	strictInputs.cronJobs = safeCronJobItems(cronJobs)
	lenientRefs := buildRefSet(lenientInputs)
	strictRefs := buildRefSet(strictInputs)

	for _, pod := range safePodItems(pods) {
		if item, ok := podOrphan(pod); ok {
			report.Pods = append(report.Pods, item)
		}
	}
	for _, s := range safeSecretItems(secrets) {
		if item, ok := secretOrphan(s, lenientRefs, strictRefs); ok {
			report.Secrets = append(report.Secrets, item)
		}
	}
	for _, cm := range safeCMItems(configMaps) {
		if item, ok := configMapOrphan(cm, lenientRefs, strictRefs); ok {
			report.ConfigMaps = append(report.ConfigMaps, item)
		}
	}
	for _, svc := range safeSvcItems(services) {
		item, ok, svcErr := c.serviceOrphan(ctx, kubeContext, svc)
		if svcErr != nil {
			errs = append(errs, svcErr)
			continue
		}
		if ok {
			report.Services = append(report.Services, item)
		}
	}
	for _, p := range safePVCItems(pvcs) {
		if item, ok := pvcOrphan(p, lenientRefs, strictRefs); ok {
			report.PVCs = append(report.PVCs, item)
		}
	}

	// HPAs / PDBs / NetworkPolicies share a "live Pods + workload
	// templates" target index. HPAs check that scaleTargetRef resolves
	// to an existing workload (Deployment/StatefulSet/DaemonSet/...);
	// PDBs and NetworkPolicies check that their podSelector matches at
	// least one Pod referenced by the strict refset (live Pod or
	// PodTemplate label set) so the same dual-mode logic that protects
	// CronJob secrets between firings also protects PDBs targeting a
	// scaled-to-zero Deployment.
	wlIdx := buildWorkloadIndex(strictInputs)
	livePods := safePodItems(pods)
	templatePodLabels := collectTemplatePodLabels(strictInputs)

	for _, h := range safeHPAItems(hpas) {
		if item, ok := hpaOrphan(h, wlIdx); ok {
			report.HPAs = append(report.HPAs, item)
		}
	}
	for _, b := range safePDBItems(pdbs) {
		if item, ok := pdbOrphan(b, livePods, templatePodLabels); ok {
			report.PDBs = append(report.PDBs, item)
		}
	}
	for _, n := range safeNetPolItems(netpols) {
		if item, ok := netpolOrphan(n, livePods, templatePodLabels); ok {
			report.NetworkPolicies = append(report.NetworkPolicies, item)
		}
	}

	// RBAC orphans — Roles/ClusterRoles unbound by any binding;
	// (Cluster)RoleBindings with empty subjects or a missing roleRef.
	rbacIdx := buildRBACIndex(
		safeRoleItems(roles), safeClusterRoleItems(clusterRoles),
		safeRoleBindingItems(roleBindings), safeClusterRoleBindingItems(clusterRoleBindings),
	)
	for _, r := range safeRoleItems(roles) {
		if item, ok := roleOrphan(r, rbacIdx); ok {
			report.Roles = append(report.Roles, item)
		}
	}
	for _, cr := range safeClusterRoleItems(clusterRoles) {
		if item, ok := clusterRoleOrphan(cr, rbacIdx); ok {
			report.ClusterRoles = append(report.ClusterRoles, item)
		}
	}
	for _, rb := range safeRoleBindingItems(roleBindings) {
		if item, ok := roleBindingOrphan(rb, rbacIdx); ok {
			report.RoleBindings = append(report.RoleBindings, item)
		}
	}
	for _, crb := range safeClusterRoleBindingItems(clusterRoleBindings) {
		if item, ok := clusterRoleBindingOrphan(crb, rbacIdx); ok {
			report.ClusterRoleBindings = append(report.ClusterRoleBindings, item)
		}
	}

	sortOrphans(report.Pods)
	sortOrphans(report.Secrets)
	sortOrphans(report.ConfigMaps)
	sortOrphans(report.Services)
	sortOrphans(report.PVCs)
	sortOrphans(report.HPAs)
	sortOrphans(report.PDBs)
	sortOrphans(report.NetworkPolicies)
	sortOrphans(report.Roles)
	sortOrphans(report.ClusterRoles)
	sortOrphans(report.RoleBindings)
	sortOrphans(report.ClusterRoleBindings)

	return report, errors.Join(errs...)
}

func safePodItems(l *corev1.PodList) []corev1.Pod {
	if l == nil {
		return nil
	}
	return l.Items
}

func safeSecretItems(l *corev1.SecretList) []corev1.Secret {
	if l == nil {
		return nil
	}
	return l.Items
}

func safeCMItems(l *corev1.ConfigMapList) []corev1.ConfigMap {
	if l == nil {
		return nil
	}
	return l.Items
}

func safeSvcItems(l *corev1.ServiceList) []corev1.Service {
	if l == nil {
		return nil
	}
	return l.Items
}

func safeIngressItems(l *networkingv1.IngressList) []networkingv1.Ingress {
	if l == nil {
		return nil
	}
	return l.Items
}

func safeSAItems(l *corev1.ServiceAccountList) []corev1.ServiceAccount {
	if l == nil {
		return nil
	}
	return l.Items
}

func safeDeploymentItems(l *appsv1.DeploymentList) []appsv1.Deployment {
	if l == nil {
		return nil
	}
	return l.Items
}

func safeStatefulSetItems(l *appsv1.StatefulSetList) []appsv1.StatefulSet {
	if l == nil {
		return nil
	}
	return l.Items
}

func safeDaemonSetItems(l *appsv1.DaemonSetList) []appsv1.DaemonSet {
	if l == nil {
		return nil
	}
	return l.Items
}

func safeJobItems(l *batchv1.JobList) []batchv1.Job {
	if l == nil {
		return nil
	}
	return l.Items
}

func safeCronJobItems(l *batchv1.CronJobList) []batchv1.CronJob {
	if l == nil {
		return nil
	}
	return l.Items
}

func safePVCItems(l *corev1.PersistentVolumeClaimList) []corev1.PersistentVolumeClaim {
	if l == nil {
		return nil
	}
	return l.Items
}

func safeHPAItems(l *autoscalingv2.HorizontalPodAutoscalerList) []autoscalingv2.HorizontalPodAutoscaler {
	if l == nil {
		return nil
	}
	return l.Items
}

func safePDBItems(l *policyv1.PodDisruptionBudgetList) []policyv1.PodDisruptionBudget {
	if l == nil {
		return nil
	}
	return l.Items
}

func safeNetPolItems(l *networkingv1.NetworkPolicyList) []networkingv1.NetworkPolicy {
	if l == nil {
		return nil
	}
	return l.Items
}

func safeRoleItems(l *rbacv1.RoleList) []rbacv1.Role {
	if l == nil {
		return nil
	}
	return l.Items
}

func safeClusterRoleItems(l *rbacv1.ClusterRoleList) []rbacv1.ClusterRole {
	if l == nil {
		return nil
	}
	return l.Items
}

func safeRoleBindingItems(l *rbacv1.RoleBindingList) []rbacv1.RoleBinding {
	if l == nil {
		return nil
	}
	return l.Items
}

func safeClusterRoleBindingItems(l *rbacv1.ClusterRoleBindingList) []rbacv1.ClusterRoleBinding {
	if l == nil {
		return nil
	}
	return l.Items
}
