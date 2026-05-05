package k8s

import (
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

// refSet collects every (namespace, name) of a Secret, ConfigMap, or
// PVC referenced by something in the cluster. The detector treats
// anything not in the relevant set as a candidate orphan (subject to
// system-managed exclusions).
type refSet struct {
	secrets    map[k8stypes.NamespacedName]struct{}
	configMaps map[k8stypes.NamespacedName]struct{}
	pvcs       map[k8stypes.NamespacedName]struct{}
}

func newRefSet() refSet {
	return refSet{
		secrets:    make(map[k8stypes.NamespacedName]struct{}),
		configMaps: make(map[k8stypes.NamespacedName]struct{}),
		pvcs:       make(map[k8stypes.NamespacedName]struct{}),
	}
}

// workloadInputs bundles the workload-controller slices `buildRefSet`
// scans so the function signature doesn't grow every time we add a new
// kind. Each slice is optional — pass nil when the caller's RBAC denied
// listing that kind, and the safe* helpers in orphan_detector.go will
// have already accumulated the error so the user sees a partial-result
// banner.
type workloadInputs struct {
	pods            []corev1.Pod
	ingresses       []networkingv1.Ingress
	serviceAccounts []corev1.ServiceAccount
	deployments     []appsv1.Deployment
	statefulSets    []appsv1.StatefulSet
	daemonSets      []appsv1.DaemonSet
	jobs            []batchv1.Job
	cronJobs        []batchv1.CronJob
}

// buildRefSet walks every workload kind that owns a PodSpec — directly
// (Pod) or transitively via a PodTemplate (Deployment, StatefulSet,
// DaemonSet, Job, CronJob). Walking only live Pods would falsely flag
// every Secret/ConfigMap referenced by a CronJob between runs (no Pod
// exists in the gap) or by a Deployment whose Pods just got bounced.
// Walking the templates makes the orphan check stable across the
// workload's life cycle.
//
// Ingress.tls and ServiceAccount references catch the remaining sources
// that aren't expressed via a PodSpec at all.
func buildRefSet(in workloadInputs) refSet {
	rs := newRefSet()
	for _, pod := range in.pods {
		collectPodSpecRefs(&rs, pod.Namespace, pod.Spec)
	}
	for _, d := range in.deployments {
		collectPodSpecRefs(&rs, d.Namespace, d.Spec.Template.Spec)
	}
	for _, ss := range in.statefulSets {
		collectPodSpecRefs(&rs, ss.Namespace, ss.Spec.Template.Spec)
	}
	for _, ds := range in.daemonSets {
		collectPodSpecRefs(&rs, ds.Namespace, ds.Spec.Template.Spec)
	}
	for _, j := range in.jobs {
		collectPodSpecRefs(&rs, j.Namespace, j.Spec.Template.Spec)
	}
	for _, cj := range in.cronJobs {
		collectPodSpecRefs(&rs, cj.Namespace, cj.Spec.JobTemplate.Spec.Template.Spec)
	}
	for _, ing := range in.ingresses {
		collectIngressRefs(&rs, ing)
	}
	for _, sa := range in.serviceAccounts {
		collectServiceAccountRefs(&rs, sa)
	}
	return rs
}

func collectIngressRefs(rs *refSet, ing networkingv1.Ingress) {
	for _, t := range ing.Spec.TLS {
		if t.SecretName != "" {
			rs.secrets[k8stypes.NamespacedName{Namespace: ing.Namespace, Name: t.SecretName}] = struct{}{}
		}
	}
}

func collectServiceAccountRefs(rs *refSet, sa corev1.ServiceAccount) {
	for _, ref := range sa.Secrets {
		if ref.Name != "" {
			rs.secrets[k8stypes.NamespacedName{Namespace: sa.Namespace, Name: ref.Name}] = struct{}{}
		}
	}
	for _, ref := range sa.ImagePullSecrets {
		if ref.Name != "" {
			rs.secrets[k8stypes.NamespacedName{Namespace: sa.Namespace, Name: ref.Name}] = struct{}{}
		}
	}
}

// collectPodSpecRefs walks a PodSpec for Secret/ConfigMap references in
// volumes (incl. projected sources), env, envFrom, imagePullSecrets,
// and the init/ephemeral container variants. Decoupled from a `Pod`
// object so the same logic can walk Deployment/StatefulSet/DaemonSet/
// Job/CronJob `.spec.template.spec` (and CronJob's deeper
// `.spec.jobTemplate.spec.template.spec`) — which is required to keep
// the orphan check stable across workload life cycles where no live Pod
// momentarily references the resource.
func collectPodSpecRefs(rs *refSet, ns string, spec corev1.PodSpec) {
	collectPodVolumeRefs(rs, ns, spec.Volumes)
	collectContainerEnvRefs(rs, ns, spec.Containers)
	collectContainerEnvRefs(rs, ns, spec.InitContainers)
	for _, ec := range spec.EphemeralContainers {
		collectEnvRefs(rs, ns, ec.Env)
		collectEnvFromRefs(rs, ns, ec.EnvFrom)
	}
	for _, ips := range spec.ImagePullSecrets {
		if ips.Name != "" {
			rs.secrets[k8stypes.NamespacedName{Namespace: ns, Name: ips.Name}] = struct{}{}
		}
	}
}

func collectPodVolumeRefs(rs *refSet, ns string, volumes []corev1.Volume) {
	for _, v := range volumes {
		if v.Secret != nil && v.Secret.SecretName != "" {
			rs.secrets[k8stypes.NamespacedName{Namespace: ns, Name: v.Secret.SecretName}] = struct{}{}
		}
		if v.ConfigMap != nil && v.ConfigMap.Name != "" {
			rs.configMaps[k8stypes.NamespacedName{Namespace: ns, Name: v.ConfigMap.Name}] = struct{}{}
		}
		if v.PersistentVolumeClaim != nil && v.PersistentVolumeClaim.ClaimName != "" {
			rs.pvcs[k8stypes.NamespacedName{Namespace: ns, Name: v.PersistentVolumeClaim.ClaimName}] = struct{}{}
		}
		if v.Projected != nil {
			for _, src := range v.Projected.Sources {
				if src.Secret != nil && src.Secret.Name != "" {
					rs.secrets[k8stypes.NamespacedName{Namespace: ns, Name: src.Secret.Name}] = struct{}{}
				}
				if src.ConfigMap != nil && src.ConfigMap.Name != "" {
					rs.configMaps[k8stypes.NamespacedName{Namespace: ns, Name: src.ConfigMap.Name}] = struct{}{}
				}
			}
		}
	}
}

func collectContainerEnvRefs(rs *refSet, ns string, containers []corev1.Container) {
	for _, c := range containers {
		collectEnvRefs(rs, ns, c.Env)
		collectEnvFromRefs(rs, ns, c.EnvFrom)
	}
}

func collectEnvRefs(rs *refSet, ns string, env []corev1.EnvVar) {
	for _, e := range env {
		if e.ValueFrom == nil {
			continue
		}
		if ref := e.ValueFrom.SecretKeyRef; ref != nil && ref.Name != "" {
			rs.secrets[k8stypes.NamespacedName{Namespace: ns, Name: ref.Name}] = struct{}{}
		}
		if ref := e.ValueFrom.ConfigMapKeyRef; ref != nil && ref.Name != "" {
			rs.configMaps[k8stypes.NamespacedName{Namespace: ns, Name: ref.Name}] = struct{}{}
		}
	}
}

func collectEnvFromRefs(rs *refSet, ns string, envFrom []corev1.EnvFromSource) {
	for _, e := range envFrom {
		if e.SecretRef != nil && e.SecretRef.Name != "" {
			rs.secrets[k8stypes.NamespacedName{Namespace: ns, Name: e.SecretRef.Name}] = struct{}{}
		}
		if e.ConfigMapRef != nil && e.ConfigMapRef.Name != "" {
			rs.configMaps[k8stypes.NamespacedName{Namespace: ns, Name: e.ConfigMapRef.Name}] = struct{}{}
		}
	}
}
