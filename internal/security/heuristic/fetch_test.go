package heuristic

import (
	"context"
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
			ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "bad"},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name: "c", Image: "nginx:latest",
					SecurityContext: &corev1.SecurityContext{Privileged: boolPtr(true)},
				}},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "clean"},
			Spec: corev1.PodSpec{
				ServiceAccountName: "api-sa",
				Containers: []corev1.Container{{
					Name: "c", Image: "nginx@sha256:abcdef",
					SecurityContext: &corev1.SecurityContext{
						Privileged:               boolPtr(false),
						AllowPrivilegeEscalation: boolPtr(false),
						ReadOnlyRootFilesystem:   boolPtr(true),
						RunAsNonRoot:             boolPtr(true),
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
	findings, err := s.Fetch(context.Background(), "", "")
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
		assert.Equal(t, security.CategoryMisconfig, f.Category)
	}
}

func TestSourceFetchNamespaceFilter(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p1"},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", SecurityContext: &corev1.SecurityContext{Privileged: boolPtr(true)}}}},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "staging", Name: "p2"},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", SecurityContext: &corev1.SecurityContext{Privileged: boolPtr(true)}}}},
		},
	)

	s := NewWithClient(client)
	findings, err := s.Fetch(context.Background(), "", "prod")
	require.NoError(t, err)
	for _, f := range findings {
		assert.Equal(t, "prod", f.Resource.Namespace)
	}
}

func TestSourceFetchNilClient(t *testing.T) {
	s := New()
	findings, err := s.Fetch(context.Background(), "", "")
	require.NoError(t, err)
	assert.Empty(t, findings)
}
