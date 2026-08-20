package heuristic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/janosmiko/lfk/internal/security"
)

func TestCheckEnvFromSecret(t *testing.T) {
	pod := &corev1.Pod{Namespace: "prod", Name: "p"}
	cases := []struct {
		name    string
		envFrom []corev1.EnvFromSource
		want    int
	}{
		{"no envFrom", nil, 0},
		{"configMapRef only", []corev1.EnvFromSource{
			{ConfigMapRef: &corev1.ConfigMapEnvSource{Name: "cm"}},
		}, 0},
		{"secretRef", []corev1.EnvFromSource{
			{SecretRef: &corev1.SecretEnvSource{Name: "db-creds"}},
		}, 1},
		{"two secretRefs produce one finding", []corev1.EnvFromSource{
			{SecretRef: &corev1.SecretEnvSource{Name: "a"}},
			{SecretRef: &corev1.SecretEnvSource{Name: "b"}},
		}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := corev1.Container{Name: "c", EnvFrom: tc.envFrom}
			findings := checkEnvFromSecret(pod, c)
			assert.Len(t, findings, tc.want)
			if tc.want == 1 {
				assert.Equal(t, security.SeverityLow, findings[0].Severity)
				assert.Equal(t, "envfrom_secret", findings[0].Labels["check"])
				for _, ef := range tc.envFrom {
					if ef.SecretRef != nil {
						assert.Contains(t, findings[0].Summary, ef.SecretRef.Name)
					}
				}
			}
		})
	}
}

func TestCheckEphemeralContainers(t *testing.T) {
	makePod := func(eph ...corev1.EphemeralContainer) *corev1.Pod {
		return &corev1.Pod{
			Namespace: "prod", Name: "p",
			Spec: corev1.PodSpec{
				Containers:          []corev1.Container{{Name: "c1"}, {Name: "c2"}},
				EphemeralContainers: eph,
			},
		}
	}
	t.Run("none is clean", func(t *testing.T) {
		pod := makePod()
		assert.Nil(t, checkEphemeralContainers(pod, pod.Spec.Containers[0]))
	})
	t.Run("debug container flagged once on first container", func(t *testing.T) {
		pod := makePod(corev1.EphemeralContainer{
			Name: "debugger",
		})
		findings := checkEphemeralContainers(pod, pod.Spec.Containers[0])
		assert.Len(t, findings, 1)
		assert.Equal(t, security.SeverityLow, findings[0].Severity)
		assert.Equal(t, "ephemeral_containers", findings[0].Labels["check"])
		assert.Contains(t, findings[0].Summary, "debugger")
		assert.Nil(t, checkEphemeralContainers(pod, pod.Spec.Containers[1]),
			"pod-level check must emit only for the first container")
	})
}

func TestCheckBarePod(t *testing.T) {
	makePod := func(owners ...metav1.OwnerReference) *corev1.Pod {
		return &corev1.Pod{
			Namespace: "prod", Name: "p", OwnerReferences: owners,
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "c1"}, {Name: "c2"}}},
		}
	}
	t.Run("bare pod flagged once as reliability", func(t *testing.T) {
		pod := makePod()
		findings := checkBarePod(pod, pod.Spec.Containers[0])
		assert.Len(t, findings, 1)
		assert.Equal(t, security.SeverityLow, findings[0].Severity)
		assert.Equal(t, security.CategoryReliability, findings[0].Category,
			"bare_pod is a reliability recommendation and must not color the SEC badge")
		assert.Equal(t, "bare_pod", findings[0].Labels["check"])
		assert.Nil(t, checkBarePod(pod, pod.Spec.Containers[1]),
			"pod-level check must emit only for the first container")
	})
	t.Run("controller-owned pod is clean", func(t *testing.T) {
		pod := makePod(metav1.OwnerReference{Kind: "ReplicaSet", Name: "rs"})
		assert.Nil(t, checkBarePod(pod, pod.Spec.Containers[0]))
	})
}

func TestCheckHostProcess(t *testing.T) {
	winOpts := func(hostProcess bool) *corev1.WindowsSecurityContextOptions {
		return &corev1.WindowsSecurityContextOptions{HostProcess: &hostProcess}
	}
	cases := []struct {
		name   string
		podWin *corev1.WindowsSecurityContextOptions
		cWin   *corev1.WindowsSecurityContextOptions
		want   int
	}{
		{"no windows options", nil, nil, 0},
		{"pod-level hostProcess", winOpts(true), nil, 1},
		{"container-level hostProcess", nil, winOpts(true), 1},
		{"container overrides pod to false", winOpts(true), winOpts(false), 0},
		{"explicit false everywhere", winOpts(false), winOpts(false), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pod := &corev1.Pod{
				Namespace: "prod", Name: "p",
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "c"}}},
			}
			if tc.podWin != nil {
				pod.Spec.SecurityContext = &corev1.PodSecurityContext{WindowsOptions: tc.podWin}
			}
			c := pod.Spec.Containers[0]
			if tc.cWin != nil {
				c.SecurityContext = &corev1.SecurityContext{WindowsOptions: tc.cWin}
			}
			findings := checkHostProcess(pod, c)
			assert.Len(t, findings, tc.want)
			if tc.want == 1 {
				assert.Equal(t, security.SeverityCritical, findings[0].Severity)
				assert.Equal(t, "host_process", findings[0].Labels["check"])
			}
		})
	}
}
