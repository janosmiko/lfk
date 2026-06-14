package heuristic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/security"
)

func refPod(ns, name string, spec corev1.PodSpec) *corev1.Pod {
	if len(spec.Containers) == 0 {
		spec.Containers = []corev1.Container{{Name: "c"}}
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns, Name: name,
			OwnerReferences: []metav1.OwnerReference{{Kind: "ReplicaSet", Name: "rs"}},
		},
		Spec: spec,
	}
}

func missingRefFindings(t *testing.T, s *Source) map[string]security.Finding {
	t.Helper()
	findings, err := s.Fetch(t.Context(), "", "")
	require.NoError(t, err)
	out := map[string]security.Finding{}
	for _, f := range findings {
		if f.Labels["check"] == "missing_config_ref" {
			out[f.Resource.Name] = f
		}
	}
	return out
}

func TestMissingConfigRefs(t *testing.T) {
	existingCM := configMap("prod", "app-config", map[string]string{"k": "v"})
	existingSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "app-creds"}}

	volumeRef := refPod("prod", "vol-missing", corev1.PodSpec{
		Volumes: []corev1.Volume{{Name: "cfg", VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "ghost-cm"}},
		}}},
	})
	envRef := refPod("prod", "env-missing", corev1.PodSpec{
		Containers: []corev1.Container{{Name: "c", Env: []corev1.EnvVar{{
			Name: "TOKEN",
			ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "ghost-secret"}, Key: "k",
			}},
		}}}},
	})
	optionalRef := refPod("prod", "optional-ok", corev1.PodSpec{
		Containers: []corev1.Container{{Name: "c", EnvFrom: []corev1.EnvFromSource{{
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: "ghost-cm"},
				Optional:             new(true),
			},
		}}}},
	})
	healthy := refPod("prod", "healthy", corev1.PodSpec{
		Volumes: []corev1.Volume{{Name: "cfg", VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "app-config"}},
		}}},
		Containers: []corev1.Container{{Name: "c", EnvFrom: []corev1.EnvFromSource{{
			SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "app-creds"}},
		}}}},
	})

	ignoredPod := refPod("ignored-ns", "ignored", corev1.PodSpec{
		Volumes: []corev1.Volume{{Name: "cfg", VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "ghost-cm"}},
		}}},
	})

	s := NewWithClient(fake.NewSimpleClientset(existingCM, existingSecret, volumeRef, envRef, optionalRef, healthy, ignoredPod))
	s.SetScanSecrets(true)
	s.SetIgnoredNamespaces([]string{"ignored-ns"})
	got := missingRefFindings(t, s)

	require.Contains(t, got, "vol-missing")
	assert.Contains(t, got["vol-missing"].Summary, "ConfigMap ghost-cm")
	assert.Equal(t, security.SeverityHigh, got["vol-missing"].Severity)
	assert.Equal(t, security.CategoryReliability, got["vol-missing"].Category,
		"missing refs are reliability findings and stay off the SEC badge")
	require.Contains(t, got, "env-missing")
	assert.Contains(t, got["env-missing"].Summary, "Secret ghost-secret")
	assert.NotContains(t, got, "optional-ok", "optional references never flag")
	assert.NotContains(t, got, "healthy")
	assert.NotContains(t, got, "ignored", "pods in ignored namespaces are never scanned")
}

// TestMissingConfigRefsNeedsCompleteLists: absence can only be asserted
// against a complete list — a disabled secret scan must mute Secret-side
// checking while ConfigMap-side still works.
func TestMissingConfigRefsNeedsCompleteLists(t *testing.T) {
	pod := refPod("prod", "both-missing", corev1.PodSpec{
		Containers: []corev1.Container{{Name: "c", EnvFrom: []corev1.EnvFromSource{
			{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "ghost-cm"}}},
			{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "ghost-secret"}}},
		}}},
	})

	// scan_secrets off: only the ConfigMap side is verified.
	s := NewWithClient(fake.NewSimpleClientset(pod.DeepCopy()))
	got := missingRefFindings(t, s)
	require.Contains(t, got, "both-missing")
	assert.Contains(t, got["both-missing"].Summary, "ConfigMap ghost-cm")
	assert.NotContains(t, got["both-missing"].Summary, "ghost-secret",
		"secret refs cannot be verified without the secret list")

	// configmaps forbidden AND secrets off: no finding at all.
	client := fake.NewSimpleClientset(pod.DeepCopy())
	forbidList(client, "configmaps")
	got = missingRefFindings(t, NewWithClient(client))
	assert.Empty(t, got)
}
