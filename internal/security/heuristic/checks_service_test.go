package heuristic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/janosmiko/lfk/internal/security"
)

func service(ns, name string, externalIPs ...string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Spec:       corev1.ServiceSpec{ExternalIPs: externalIPs},
	}
}

func TestSourceFetchServiceExternalIPs(t *testing.T) {
	client := fake.NewSimpleClientset(
		service("prod", "with-ext", "203.0.113.10"),
		service("prod", "plain"),
		service("ignored-ns", "also-ext", "203.0.113.11"),
	)
	s := NewWithClient(client)
	s.SetIgnoredNamespaces([]string{"ignored-ns"})
	findings, err := s.Fetch(t.Context(), "", "")
	require.NoError(t, err)

	byName := map[string]security.Finding{}
	for _, f := range findings {
		if f.Labels["check"] == "service_external_ips" {
			byName[f.Resource.Name] = f
		}
	}
	require.Contains(t, byName, "with-ext")
	assert.Equal(t, "Service", byName["with-ext"].Resource.Kind)
	assert.Equal(t, security.SeverityHigh, byName["with-ext"].Severity)
	assert.Contains(t, byName["with-ext"].Summary, "203.0.113.10")
	assert.NotContains(t, byName, "plain")
	assert.NotContains(t, byName, "also-ext", "ignored namespaces apply to service checks")
}

// TestSourceFetchServiceListBestEffort: a Forbidden services list must not
// fail the source or hide pod findings — service checks are best-effort,
// matching the advisor's behavior for unlistable types.
func TestSourceFetchServiceListBestEffort(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "p"},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name: "c", SecurityContext: &corev1.SecurityContext{Privileged: new(true)},
			}}},
		},
	)
	client.PrependReactor("list", "services", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "services"}, "", nil)
	})

	s := NewWithClient(client)
	findings, err := s.Fetch(t.Context(), "", "")
	require.NoError(t, err)
	var privileged bool
	for _, f := range findings {
		if f.Labels["check"] == "privileged" {
			privileged = true
		}
	}
	assert.True(t, privileged, "pod findings must survive a forbidden services list")
}
