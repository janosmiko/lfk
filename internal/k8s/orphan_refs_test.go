package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

func podWithVolumes(ns, name string, secrets, configMaps []string) corev1.Pod {
	p := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name}}
	for _, s := range secrets {
		p.Spec.Volumes = append(p.Spec.Volumes, corev1.Volume{
			Name:         "vol-" + s,
			VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: s}},
		})
	}
	for _, cm := range configMaps {
		p.Spec.Volumes = append(p.Spec.Volumes, corev1.Volume{
			Name: "vol-" + cm,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: cm},
				},
			},
		})
	}
	return p
}

func TestBuildRefSet_PodVolumes(t *testing.T) {
	pods := []corev1.Pod{
		podWithVolumes("ns1", "p1", []string{"tls-cert"}, []string{"app-config"}),
		podWithVolumes("ns2", "p2", []string{"db-creds"}, nil),
	}

	refs := buildRefSet(workloadInputs{pods: pods})

	assert.Contains(t, refs.secrets, k8stypes.NamespacedName{Namespace: "ns1", Name: "tls-cert"})
	assert.Contains(t, refs.secrets, k8stypes.NamespacedName{Namespace: "ns2", Name: "db-creds"})
	assert.Contains(t, refs.configMaps, k8stypes.NamespacedName{Namespace: "ns1", Name: "app-config"})
	assert.Len(t, refs.secrets, 2)
	assert.Len(t, refs.configMaps, 1)
}

func TestBuildRefSet_PodEnvAndEnvFrom(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "app"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "main",
				Env: []corev1.EnvVar{
					{Name: "DB_PASS", ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "db-secret"},
							Key:                  "password",
						},
					}},
					{Name: "FEATURE", ValueFrom: &corev1.EnvVarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "feature-cm"},
							Key:                  "enabled",
						},
					}},
				},
				EnvFrom: []corev1.EnvFromSource{
					{SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "envfrom-secret"},
					}},
					{ConfigMapRef: &corev1.ConfigMapEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "envfrom-cm"},
					}},
				},
			}},
			ImagePullSecrets: []corev1.LocalObjectReference{{Name: "regcred"}},
		},
	}

	refs := buildRefSet(workloadInputs{pods: []corev1.Pod{pod}})

	assert.Contains(t, refs.secrets, k8stypes.NamespacedName{Namespace: "default", Name: "db-secret"})
	assert.Contains(t, refs.secrets, k8stypes.NamespacedName{Namespace: "default", Name: "envfrom-secret"})
	assert.Contains(t, refs.secrets, k8stypes.NamespacedName{Namespace: "default", Name: "regcred"})
	assert.Contains(t, refs.configMaps, k8stypes.NamespacedName{Namespace: "default", Name: "feature-cm"})
	assert.Contains(t, refs.configMaps, k8stypes.NamespacedName{Namespace: "default", Name: "envfrom-cm"})
}

func TestBuildRefSet_InitAndEphemeralContainers(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "app"},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Name: "init",
				Env: []corev1.EnvVar{{
					Name: "INIT_PASS", ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "init-env-secret"},
							Key:                  "password",
						},
					},
				}},
				EnvFrom: []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "init-secret"},
				}}},
			}},
			EphemeralContainers: []corev1.EphemeralContainer{{
				EphemeralContainerCommon: corev1.EphemeralContainerCommon{
					Name: "debug",
					Env: []corev1.EnvVar{{
						Name: "DEBUG_FLAG", ValueFrom: &corev1.EnvVarSource{
							ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "debug-env-cm"},
								Key:                  "level",
							},
						},
					}},
					EnvFrom: []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "debug-cm"},
					}}},
				},
			}},
		},
	}

	refs := buildRefSet(workloadInputs{pods: []corev1.Pod{pod}})

	assert.Contains(t, refs.secrets, k8stypes.NamespacedName{Namespace: "default", Name: "init-secret"})
	assert.Contains(t, refs.configMaps, k8stypes.NamespacedName{Namespace: "default", Name: "debug-cm"})
	assert.Contains(t, refs.secrets, k8stypes.NamespacedName{Namespace: "default", Name: "init-env-secret"})
	assert.Contains(t, refs.configMaps, k8stypes.NamespacedName{Namespace: "default", Name: "debug-env-cm"})
}

func TestBuildRefSet_PodProjectedVolumes(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "app"},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{{
				Name: "kube-api-access",
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						Sources: []corev1.VolumeProjection{
							{Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{Name: "bound-sa-token"},
							}},
							{ConfigMap: &corev1.ConfigMapProjection{
								LocalObjectReference: corev1.LocalObjectReference{Name: "ca-bundle"},
							}},
						},
					},
				},
			}},
		},
	}

	refs := buildRefSet(workloadInputs{pods: []corev1.Pod{pod}})

	assert.Contains(t, refs.secrets, k8stypes.NamespacedName{Namespace: "default", Name: "bound-sa-token"})
	assert.Contains(t, refs.configMaps, k8stypes.NamespacedName{Namespace: "default", Name: "ca-bundle"})
}

func TestBuildRefSet_IngressTLS(t *testing.T) {
	ing := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Namespace: "web", Name: "site"},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{{SecretName: "site-tls"}},
		},
	}

	refs := buildRefSet(workloadInputs{ingresses: []networkingv1.Ingress{ing}})

	assert.Contains(t, refs.secrets, k8stypes.NamespacedName{Namespace: "web", Name: "site-tls"})
}

func TestBuildRefSet_PodPVCVolume(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "data", Name: "db"},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{{
				Name: "data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "db-data",
					},
				},
			}},
		},
	}

	refs := buildRefSet(workloadInputs{pods: []corev1.Pod{pod}})

	assert.Contains(t, refs.pvcs, k8stypes.NamespacedName{Namespace: "data", Name: "db-data"})
}

func TestBuildRefSet_ServiceAccountSecrets(t *testing.T) {
	sa := corev1.ServiceAccount{
		ObjectMeta:       metav1.ObjectMeta{Namespace: "default", Name: "deployer"},
		Secrets:          []corev1.ObjectReference{{Name: "deployer-token"}},
		ImagePullSecrets: []corev1.LocalObjectReference{{Name: "registry-creds"}},
	}

	refs := buildRefSet(workloadInputs{serviceAccounts: []corev1.ServiceAccount{sa}})

	assert.Contains(t, refs.secrets, k8stypes.NamespacedName{Namespace: "default", Name: "deployer-token"})
	assert.Contains(t, refs.secrets, k8stypes.NamespacedName{Namespace: "default", Name: "registry-creds"})
}

// podSpecWithSecretVolume builds a one-container PodSpec mounting the
// named Secret as a volume — used by the workload-template tests below
// to verify each controller's template gets walked.
func podSpecWithSecretVolume(secretName string) corev1.PodSpec {
	return corev1.PodSpec{
		Volumes: []corev1.Volume{{
			Name:         "creds",
			VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: secretName}},
		}},
	}
}

// TestBuildRefSet_DeploymentTemplate covers the false-positive class
// where a Deployment references a Secret via .spec.template.spec but no
// live Pod exists momentarily (rolling update, all replicas just
// restarted). Without walking templates the Secret would be flagged as
// "unmounted" between the old ReplicaSet's Pod termination and the new
// one's startup.
func TestBuildRefSet_DeploymentTemplate(t *testing.T) {
	d := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "api"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{Spec: podSpecWithSecretVolume("api-creds")},
		},
	}

	refs := buildRefSet(workloadInputs{deployments: []appsv1.Deployment{d}})

	assert.Contains(t, refs.secrets, k8stypes.NamespacedName{Namespace: "default", Name: "api-creds"})
}

func TestBuildRefSet_StatefulSetTemplate(t *testing.T) {
	ss := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "data", Name: "db"},
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{Spec: podSpecWithSecretVolume("db-pass")},
		},
	}

	refs := buildRefSet(workloadInputs{statefulSets: []appsv1.StatefulSet{ss}})

	assert.Contains(t, refs.secrets, k8stypes.NamespacedName{Namespace: "data", Name: "db-pass"})
}

func TestBuildRefSet_DaemonSetTemplate(t *testing.T) {
	ds := appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: "cni"},
		Spec: appsv1.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{Spec: podSpecWithSecretVolume("cni-creds")},
		},
	}

	refs := buildRefSet(workloadInputs{daemonSets: []appsv1.DaemonSet{ds}})

	assert.Contains(t, refs.secrets, k8stypes.NamespacedName{Namespace: "kube-system", Name: "cni-creds"})
}

func TestBuildRefSet_JobTemplate(t *testing.T) {
	j := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "migrate"},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{Spec: podSpecWithSecretVolume("migrate-creds")},
		},
	}

	refs := buildRefSet(workloadInputs{jobs: []batchv1.Job{j}})

	assert.Contains(t, refs.secrets, k8stypes.NamespacedName{Namespace: "default", Name: "migrate-creds"})
}

// TestBuildRefSet_CronJobTemplate is the headline false-positive fix —
// CronJobs have a Secret mounted via .spec.jobTemplate.spec.template.spec
// and between cron firings no live Pod references it. Walking only Pods
// would flag every CronJob's Secret as orphan in the gap, which a real
// user reported. The double-template path (jobTemplate.spec.template) is
// what makes CronJobs special vs Job/Deployment/etc.
func TestBuildRefSet_CronJobTemplate(t *testing.T) {
	cj := batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "backup"},
		Spec: batchv1.CronJobSpec{
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{Spec: podSpecWithSecretVolume("backup-creds")},
				},
			},
		},
	}

	refs := buildRefSet(workloadInputs{cronJobs: []batchv1.CronJob{cj}})

	assert.Contains(t, refs.secrets, k8stypes.NamespacedName{Namespace: "default", Name: "backup-creds"})
}
