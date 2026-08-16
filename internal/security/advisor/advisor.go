// Package advisor implements a zero-dependency security.SecuritySource that
// emits reliability recommendations (security.CategoryReliability): missing
// PDBs, ResourceQuotas, probes, resource requests, and drain-blocking PDBs.
// Findings are dashboard-only — BuildFindingIndex keeps reliability findings
// out of the per-resource SEC badge.
package advisor

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	policyv1 "k8s.io/api/policy/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/janosmiko/lfk/internal/security"
)

// Source is the advisor SecuritySource implementation.
type Source struct {
	client kubernetes.Interface
}

// New returns an advisor source with no client. Fetch returns an empty slice
// and IsAvailable reports false.
func New() *Source { return &Source{} }

// NewWithClient returns an advisor source that lists via the given client.
func NewWithClient(client kubernetes.Interface) *Source {
	return &Source{client: client}
}

// Name returns the stable identifier.
func (s *Source) Name() string { return "advisor" }

// Categories returns the categories this source contributes to.
func (s *Source) Categories() []security.Category {
	return []security.Category{security.CategoryReliability}
}

// IsAvailable returns true only when a kubernetes client has been injected.
func (s *Source) IsAvailable(ctx context.Context, kubeCtx string) (bool, error) {
	return s.client != nil, nil
}

// systemNamespaces are excluded from every advisor check — their workloads
// are not user-fixable and quota/PDB recommendations there are pure noise.
var systemNamespaces = map[string]bool{
	"kube-system":     true,
	"kube-public":     true,
	"kube-node-lease": true,
}

// workload is the common shape of a Deployment, StatefulSet, or DaemonSet
// the checks operate on. DaemonSets carry replicas 0, which keeps them out
// of every replica-based check (single_replica, no_pdb, no_topology_spread)
// while the container-level checks still apply. Excluding DaemonSets from
// no_pdb is deliberate: node drains skip DaemonSet pods by design
// (--ignore-daemonsets), so a PDB recommendation there would be noise.
type workload struct {
	kind       string // "Deployment", "StatefulSet", or "DaemonSet"
	namespace  string
	name       string
	replicas   int32
	podLabels  map[string]string
	containers []corev1.Container
	volumes    []corev1.Volume
	// spreadConfigured is true when the pod template sets either
	// topologySpreadConstraints or pod anti-affinity.
	spreadConfigured bool
	// strategy is set for Deployments only.
	strategy *appsv1.DeploymentStrategy
	// zeroGracePeriod is true when the pod template sets
	// terminationGracePeriodSeconds: 0 (instant SIGKILL).
	zeroGracePeriod bool
	// onDeleteUpdate is true for DaemonSets/StatefulSets with an explicit
	// updateStrategy: OnDelete. An empty type is NOT OnDelete — the API
	// server defaults it to RollingUpdate.
	onDeleteUpdate bool
	// staticReplicasOwner is the field manager that owns .spec.replicas
	// through a whole-object write per managedFields (scale-subresource
	// writers like the HPA are excluded) — empty when no manifest pins
	// the replica count.
	staticReplicasOwner string
	// stsServiceName is the StatefulSet's spec.serviceName (governing
	// headless Service). Empty for other kinds.
	stsServiceName string
	// stsVCTClasses holds the storageClassName of each StatefulSet
	// volumeClaimTemplate ("" = the cluster default class), nil for other
	// kinds.
	stsVCTClasses []string
}

// templateZeroGrace reports whether the pod template requests instant
// SIGKILL on termination.
func templateZeroGrace(tmpl *corev1.PodTemplateSpec) bool {
	g := tmpl.Spec.TerminationGracePeriodSeconds
	return g != nil && *g == 0
}

// templateSpreads reports whether the pod template spreads replicas across
// failure domains by either mechanism.
func templateSpreads(tmpl *corev1.PodTemplateSpec) bool {
	if len(tmpl.Spec.TopologySpreadConstraints) > 0 {
		return true
	}
	return tmpl.Spec.Affinity != nil && tmpl.Spec.Affinity.PodAntiAffinity != nil
}

// clusterData holds everything Fetch could list. Each OK flag records whether
// the corresponding list succeeded. Checks that depend on a failed list are
// skipped entirely (best-effort RBAC) instead of emitting false positives.
type clusterData struct {
	workloads []workload
	// workloadsOK is true only when every workload list (Deployments,
	// StatefulSets, DaemonSets) succeeded — required by checks that reason
	// about the absence of a matching workload (orphan_pdb).
	workloadsOK bool
	pdbs        []policyv1.PodDisruptionBudget
	pdbsOK      bool
	hpas        []autoscalingv2.HorizontalPodAutoscaler
	hpasOK      bool
	// namespaces to run namespace-level checks against.
	namespaces     []string
	quotas         []corev1.ResourceQuota
	quotaNS        map[string]bool
	quotasOK       bool
	limitNS        map[string]bool
	limitsOK       bool
	services       []corev1.Service
	servicesOK     bool
	endpointSlices []discoveryv1.EndpointSlice
	epsOK          bool
	cronJobs       []batchv1.CronJob
	cronJobsOK     bool
	jobs           []batchv1.Job
	jobsOK         bool
	storageClasses []storagev1.StorageClass
	scOK           bool
}

// vctClasses extracts the storageClassName of each volumeClaimTemplate
// ("" when unset, meaning the cluster default class).
func vctClasses(vcts []corev1.PersistentVolumeClaim) []string {
	if len(vcts) == 0 {
		return nil
	}
	classes := make([]string, 0, len(vcts))
	for i := range vcts {
		name := ""
		if vcts[i].Spec.StorageClassName != nil {
			name = *vcts[i].Spec.StorageClassName
		}
		classes = append(classes, name)
	}
	return classes
}

// Fetch lists workloads, PDBs, HPAs, namespaces, quotas, and limit ranges
// (best-effort per type) and runs the reliability checks over them. Empty
// namespace means all namespaces.
func (s *Source) Fetch(ctx context.Context, kubeCtx, namespace string) ([]security.Finding, error) {
	if s.client == nil {
		return nil, nil
	}
	d := &clusterData{}

	deps, depsOK := security.Collect(func(o metav1.ListOptions) ([]appsv1.Deployment, string, error) {
		l, err := s.client.AppsV1().Deployments(namespace).List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	})
	if depsOK {
		for i := range deps {
			dep := &deps[i]
			d.workloads = append(d.workloads, workload{
				kind:                "Deployment",
				namespace:           dep.Namespace,
				name:                dep.Name,
				replicas:            replicasOrDefault(dep.Spec.Replicas),
				podLabels:           dep.Spec.Template.Labels,
				containers:          dep.Spec.Template.Spec.Containers,
				volumes:             dep.Spec.Template.Spec.Volumes,
				spreadConfigured:    templateSpreads(&dep.Spec.Template),
				strategy:            &dep.Spec.Strategy,
				zeroGracePeriod:     templateZeroGrace(&dep.Spec.Template),
				staticReplicasOwner: staticReplicasOwner(dep.ManagedFields),
			})
		}
	}
	stss, stssOK := security.Collect(func(o metav1.ListOptions) ([]appsv1.StatefulSet, string, error) {
		l, err := s.client.AppsV1().StatefulSets(namespace).List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	})
	if stssOK {
		for i := range stss {
			sts := &stss[i]
			d.workloads = append(d.workloads, workload{
				kind:                "StatefulSet",
				namespace:           sts.Namespace,
				name:                sts.Name,
				replicas:            replicasOrDefault(sts.Spec.Replicas),
				podLabels:           sts.Spec.Template.Labels,
				containers:          sts.Spec.Template.Spec.Containers,
				volumes:             sts.Spec.Template.Spec.Volumes,
				spreadConfigured:    templateSpreads(&sts.Spec.Template),
				zeroGracePeriod:     templateZeroGrace(&sts.Spec.Template),
				onDeleteUpdate:      sts.Spec.UpdateStrategy.Type == appsv1.OnDeleteStatefulSetStrategyType,
				staticReplicasOwner: staticReplicasOwner(sts.ManagedFields),
				stsServiceName:      sts.Spec.ServiceName,
				stsVCTClasses:       vctClasses(sts.Spec.VolumeClaimTemplates),
			})
		}
	}
	dss, dssOK := security.Collect(func(o metav1.ListOptions) ([]appsv1.DaemonSet, string, error) {
		l, err := s.client.AppsV1().DaemonSets(namespace).List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	})
	if dssOK {
		for i := range dss {
			ds := &dss[i]
			d.workloads = append(d.workloads, workload{
				kind:            "DaemonSet",
				namespace:       ds.Namespace,
				name:            ds.Name,
				replicas:        0, // keeps DaemonSets out of replica-based checks
				podLabels:       ds.Spec.Template.Labels,
				containers:      ds.Spec.Template.Spec.Containers,
				volumes:         ds.Spec.Template.Spec.Volumes,
				zeroGracePeriod: templateZeroGrace(&ds.Spec.Template),
				onDeleteUpdate:  ds.Spec.UpdateStrategy.Type == appsv1.OnDeleteDaemonSetStrategyType,
			})
		}
	}
	d.workloadsOK = depsOK && stssOK && dssOK

	d.pdbs, d.pdbsOK = security.Collect(func(o metav1.ListOptions) ([]policyv1.PodDisruptionBudget, string, error) {
		l, err := s.client.PolicyV1().PodDisruptionBudgets(namespace).List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	})
	d.hpas, d.hpasOK = security.Collect(func(o metav1.ListOptions) ([]autoscalingv2.HorizontalPodAutoscaler, string, error) {
		l, err := s.client.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	})

	if namespace != "" {
		d.namespaces = []string{namespace}
	} else if nss, ok := security.Collect(func(o metav1.ListOptions) ([]corev1.Namespace, string, error) {
		l, err := s.client.CoreV1().Namespaces().List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	}); ok {
		for i := range nss {
			d.namespaces = append(d.namespaces, nss[i].Name)
		}
	}

	quotas, quotasOK := security.Collect(func(o metav1.ListOptions) ([]corev1.ResourceQuota, string, error) {
		l, err := s.client.CoreV1().ResourceQuotas(namespace).List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	})
	d.quotas = quotas
	d.quotasOK = quotasOK
	d.quotaNS = map[string]bool{}
	for i := range quotas {
		d.quotaNS[quotas[i].Namespace] = true
	}
	d.services, d.servicesOK = security.Collect(func(o metav1.ListOptions) ([]corev1.Service, string, error) {
		l, err := s.client.CoreV1().Services(namespace).List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	})
	d.endpointSlices, d.epsOK = security.Collect(func(o metav1.ListOptions) ([]discoveryv1.EndpointSlice, string, error) {
		l, err := s.client.DiscoveryV1().EndpointSlices(namespace).List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	})
	d.cronJobs, d.cronJobsOK = security.Collect(func(o metav1.ListOptions) ([]batchv1.CronJob, string, error) {
		l, err := s.client.BatchV1().CronJobs(namespace).List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	})
	d.jobs, d.jobsOK = security.Collect(func(o metav1.ListOptions) ([]batchv1.Job, string, error) {
		l, err := s.client.BatchV1().Jobs(namespace).List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	})
	d.storageClasses, d.scOK = security.Collect(func(o metav1.ListOptions) ([]storagev1.StorageClass, string, error) {
		l, err := s.client.StorageV1().StorageClasses().List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	})

	limits, limitsOK := security.Collect(func(o metav1.ListOptions) ([]corev1.LimitRange, string, error) {
		l, err := s.client.CoreV1().LimitRanges(namespace).List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	})
	d.limitsOK = limitsOK
	d.limitNS = map[string]bool{}
	for i := range limits {
		d.limitNS[limits[i].Namespace] = true
	}

	return d.findings(), nil
}

func replicasOrDefault(r *int32) int32 {
	if r == nil {
		return 1
	}
	return *r
}
