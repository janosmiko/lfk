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

func TestSourceFetch(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Pod{
			Namespace: "prod", Name: "bad",
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name: "c", Image: "nginx:latest",
					SecurityContext: &corev1.SecurityContext{Privileged: new(true)},
				}},
			},
		},
		&corev1.Pod{
			Namespace: "prod", Name: "clean",
			// A fully clean pod is controller-managed; without an owner
			// the bare_pod check fires.
			OwnerReferences: []metav1.OwnerReference{{Kind: "ReplicaSet", Name: "rs"}},
			Spec: corev1.PodSpec{
				ServiceAccountName: "api-sa",
				Containers: []corev1.Container{{
					Name: "c", Image: "nginx@sha256:abcdef",
					SecurityContext: &corev1.SecurityContext{
						Privileged:               new(false),
						AllowPrivilegeEscalation: new(false),
						ReadOnlyRootFilesystem:   new(true),
						RunAsNonRoot:             new(true),
					},
					Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resourceQuantity("100m"),
						corev1.ResourceMemory: resourceQuantity("128Mi"),
					}},
				}},
			},
		},
	)

	s := NewWithClient(client)
	findings, err := s.Fetch(t.Context(), "", "")
	require.NoError(t, err)

	badCount := 0
	cleanCount := 0
	for _, f := range findings {
		switch f.Resource.Name {
		case "bad":
			badCount++
		case "clean":
			cleanCount++
		}
	}
	assert.Greater(t, badCount, 0)
	assert.Equal(t, 0, cleanCount)

	for _, f := range findings {
		assert.Equal(t, "heuristic", f.Source)
		if f.Labels["check"] == "bare_pod" {
			// bare_pod is the one reliability recommendation the heuristic
			// emits; it must stay off the SEC badge.
			assert.Equal(t, security.CategoryReliability, f.Category)
			continue
		}
		assert.Equal(t, security.CategoryMisconfig, f.Category)
	}
}

func TestSourceFetchNamespaceFilter(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Pod{
			Namespace: "prod", Name: "p1",
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "c", SecurityContext: &corev1.SecurityContext{Privileged: new(true)}}}},
		},
		&corev1.Pod{
			Namespace: "staging", Name: "p2",
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "c", SecurityContext: &corev1.SecurityContext{Privileged: new(true)}}}},
		},
	)

	s := NewWithClient(client)
	findings, err := s.Fetch(t.Context(), "", "prod")
	require.NoError(t, err)
	for _, f := range findings {
		assert.Equal(t, "prod", f.Resource.Namespace)
	}
}

// TestSourceFetchSecretEnvPatterns verifies the secret_env check runs through
// Fetch (it is dispatched directly, not via allChecks) and that configured
// include/exclude patterns reach it.
func TestSourceFetchSecretEnvPatterns(t *testing.T) {
	pod := &corev1.Pod{
		Namespace: "prod", Name: "p",
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "c", Image: "nginx:1.25",
				Env: []corev1.EnvVar{
					{Name: "LEGACY_PASSWORD", Value: "x"},
					{Name: "MY_CONN_STR", Value: "y"},
				},
			}},
		},
	}
	secretEnvNames := func(s *Source) []string {
		findings, err := s.Fetch(t.Context(), "", "")
		require.NoError(t, err)
		var sums []string
		for _, f := range findings {
			if f.Labels["check"] == "secret_env" {
				sums = append(sums, f.Summary)
			}
		}
		return sums
	}

	// Default: keyword name flags, non-keyword name does not.
	defaultSums := secretEnvNames(NewWithClient(fake.NewSimpleClientset(pod.DeepCopy())))
	require.Len(t, defaultSums, 1)
	assert.Contains(t, defaultSums[0], "LEGACY_PASSWORD")
	assert.NotContains(t, defaultSums[0], "MY_CONN_STR")

	// Configured: exclude suppresses the keyword name, include adds the other.
	s := NewWithClient(fake.NewSimpleClientset(pod.DeepCopy()))
	s.SetSecretEnvPatterns([]string{"*_CONN_STR"}, []string{"LEGACY_*"})
	configuredSums := secretEnvNames(s)
	require.Len(t, configuredSums, 1)
	assert.Contains(t, configuredSums[0], "MY_CONN_STR")
	assert.NotContains(t, configuredSums[0], "LEGACY_PASSWORD")
}

func TestSourceFetchNilClient(t *testing.T) {
	s := NewWithClient(nil)
	findings, err := s.Fetch(t.Context(), "", "")
	require.NoError(t, err)
	assert.Empty(t, findings)
}

// TestSourceFetchScansInitAndEphemeralContainers verifies that the
// heuristic checks run against init and ephemeral containers, not just
// the main Spec.Containers — privileged init containers were silently
// invisible to the dashboard before the fix.
func TestSourceFetchScansInitAndEphemeralContainers(t *testing.T) {
	priv := corev1.SecurityContext{Privileged: new(true)}
	pod := &corev1.Pod{
		Namespace: "prod", Name: "pod",
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Name: "init", Image: "busybox", SecurityContext: &priv,
			}},
			Containers: []corev1.Container{{
				Name: "main", Image: "nginx", SecurityContext: &priv,
			}},
			EphemeralContainers: []corev1.EphemeralContainer{{
				Name: "debug", Image: "alpine", SecurityContext: &priv,
			}},
		},
	}
	client := fake.NewSimpleClientset(pod)
	s := NewWithClient(client)
	findings, err := s.Fetch(t.Context(), "", "")
	require.NoError(t, err)

	containers := map[string]bool{}
	for _, f := range findings {
		if f.Labels["check"] == "privileged" {
			containers[f.Resource.Container] = true
		}
	}
	assert.True(t, containers["init"], "init container must be scanned")
	assert.True(t, containers["main"], "main container must be scanned")
	assert.True(t, containers["debug"], "ephemeral container must be scanned")
}
