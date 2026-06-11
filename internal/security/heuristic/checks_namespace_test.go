package heuristic

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/janosmiko/lfk/internal/security"
)

func namespaceObj(name string, labels map[string]string) *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
}

func forbidList(client *fake.Clientset, res string) {
	client.PrependReactor("list", res, func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: res}, "", nil)
	})
}

func minimalPod(ns, name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns, Name: name,
			OwnerReferences: []metav1.OwnerReference{{Kind: "ReplicaSet", Name: "rs"}},
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "c"}}},
	}
}

func netpol(ns, name string) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name}}
}

func checksByResource(t *testing.T, s *Source) map[string]map[string]bool {
	t.Helper()
	findings, err := s.Fetch(context.Background(), "", "")
	require.NoError(t, err)
	out := map[string]map[string]bool{}
	for _, f := range findings {
		key := f.Resource.Namespace + "/" + f.Resource.Kind + "/" + f.Resource.Name
		if out[key] == nil {
			out[key] = map[string]bool{}
		}
		out[key][f.Labels["check"]] = true
	}
	return out
}

func TestNamespaceChecks(t *testing.T) {
	objs := []runtime.Object{
		namespaceObj("unlabeled", nil),
		namespaceObj("enforced", map[string]string{"pod-security.kubernetes.io/enforce": "restricted"}),
		namespaceObj("kube-system", nil),
		namespaceObj("ignored-ns", nil),
		// unlabeled has pods and no netpol; enforced has pods and a netpol.
		minimalPod("unlabeled", "p1"),
		minimalPod("enforced", "p2"),
		minimalPod("kube-system", "p3"),
		netpol("enforced", "default-deny"),
		// empty-ns has no pods: PSA still applies, netpol does not.
		namespaceObj("empty-ns", nil),
	}
	s := NewWithClient(fake.NewSimpleClientset(objs...))
	s.SetIgnoredNamespaces([]string{"ignored-ns"})
	checks := checksByResource(t, s)

	assert.True(t, checks["unlabeled/Namespace/unlabeled"]["psa_labels_missing"])
	assert.True(t, checks["unlabeled/Namespace/unlabeled"]["namespace_no_netpol"])
	assert.False(t, checks["enforced/Namespace/enforced"]["psa_labels_missing"])
	assert.False(t, checks["enforced/Namespace/enforced"]["namespace_no_netpol"])
	assert.Empty(t, checks["kube-system/Namespace/kube-system"], "system namespaces are skipped")
	assert.Empty(t, checks["ignored-ns/Namespace/ignored-ns"], "ignored namespaces are skipped")
	assert.True(t, checks["empty-ns/Namespace/empty-ns"]["psa_labels_missing"])
	assert.False(t, checks["empty-ns/Namespace/empty-ns"]["namespace_no_netpol"],
		"a namespace without pods needs no NetworkPolicy finding")
}

// TestNamespaceChecksBestEffort: forbidden namespace/netpol lists skip their
// checks without failing the source.
func TestNamespaceChecksBestEffort(t *testing.T) {
	client := fake.NewSimpleClientset(namespaceObj("prod", nil), minimalPod("prod", "p"))
	forbidList(client, "namespaces")
	forbidList(client, "networkpolicies")
	s := NewWithClient(client)
	findings, err := s.Fetch(context.Background(), "", "")
	require.NoError(t, err)
	for _, f := range findings {
		assert.NotEqual(t, "psa_labels_missing", f.Labels["check"])
		assert.NotEqual(t, "namespace_no_netpol", f.Labels["check"])
	}
}

func TestNamespaceFindingCategory(t *testing.T) {
	s := NewWithClient(fake.NewSimpleClientset(namespaceObj("prod", nil)))
	findings, err := s.Fetch(context.Background(), "", "")
	require.NoError(t, err)
	require.NotEmpty(t, findings)
	for _, f := range findings {
		assert.Equal(t, security.CategoryMisconfig, f.Category)
		assert.Equal(t, security.SeverityMedium, f.Severity)
	}
}
