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
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
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

// workload is the common shape of a Deployment or StatefulSet the checks
// operate on.
type workload struct {
	kind       string // "Deployment" or "StatefulSet"
	namespace  string
	name       string
	replicas   int32
	podLabels  map[string]string
	containers []corev1.Container
}

// clusterData holds everything Fetch could list. Each OK flag records whether
// the corresponding list succeeded; checks that depend on a failed list are
// skipped entirely (best-effort RBAC) instead of emitting false positives.
type clusterData struct {
	workloads []workload
	pdbs      []policyv1.PodDisruptionBudget
	pdbsOK    bool
	hpas      []autoscalingv2.HorizontalPodAutoscaler
	hpasOK    bool
	// namespaces to run namespace-level checks against.
	namespaces []string
	quotaNS    map[string]bool
	quotasOK   bool
	limitNS    map[string]bool
	limitsOK   bool
}

// collect paginates one list call to completion. Any error (typically
// Forbidden for read-only users) returns ok=false so dependent checks are
// skipped rather than misreporting.
func collect[T any](fn func(opts metav1.ListOptions) ([]T, string, error)) ([]T, bool) {
	var out []T
	opts := metav1.ListOptions{Limit: 200}
	for {
		items, cont, err := fn(opts)
		if err != nil {
			return nil, false
		}
		out = append(out, items...)
		if cont == "" {
			return out, true
		}
		opts.Continue = cont
	}
}

// Fetch lists workloads, PDBs, HPAs, namespaces, quotas, and limit ranges
// (best-effort per type) and runs the reliability checks over them. Empty
// namespace means all namespaces.
func (s *Source) Fetch(ctx context.Context, kubeCtx, namespace string) ([]security.Finding, error) {
	if s.client == nil {
		return nil, nil
	}
	d := &clusterData{}

	deps, depsOK := collect(func(o metav1.ListOptions) ([]appsv1.Deployment, string, error) {
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
				kind:       "Deployment",
				namespace:  dep.Namespace,
				name:       dep.Name,
				replicas:   replicasOrDefault(dep.Spec.Replicas),
				podLabels:  dep.Spec.Template.Labels,
				containers: dep.Spec.Template.Spec.Containers,
			})
		}
	}
	stss, stssOK := collect(func(o metav1.ListOptions) ([]appsv1.StatefulSet, string, error) {
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
				kind:       "StatefulSet",
				namespace:  sts.Namespace,
				name:       sts.Name,
				replicas:   replicasOrDefault(sts.Spec.Replicas),
				podLabels:  sts.Spec.Template.Labels,
				containers: sts.Spec.Template.Spec.Containers,
			})
		}
	}

	d.pdbs, d.pdbsOK = collect(func(o metav1.ListOptions) ([]policyv1.PodDisruptionBudget, string, error) {
		l, err := s.client.PolicyV1().PodDisruptionBudgets(namespace).List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	})
	d.hpas, d.hpasOK = collect(func(o metav1.ListOptions) ([]autoscalingv2.HorizontalPodAutoscaler, string, error) {
		l, err := s.client.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	})

	if namespace != "" {
		d.namespaces = []string{namespace}
	} else if nss, ok := collect(func(o metav1.ListOptions) ([]corev1.Namespace, string, error) {
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

	quotas, quotasOK := collect(func(o metav1.ListOptions) ([]corev1.ResourceQuota, string, error) {
		l, err := s.client.CoreV1().ResourceQuotas(namespace).List(ctx, o)
		if err != nil {
			return nil, "", err
		}
		return l.Items, l.Continue, nil
	})
	d.quotasOK = quotasOK
	d.quotaNS = map[string]bool{}
	for i := range quotas {
		d.quotaNS[quotas[i].Namespace] = true
	}
	limits, limitsOK := collect(func(o metav1.ListOptions) ([]corev1.LimitRange, string, error) {
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
