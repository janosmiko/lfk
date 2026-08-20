package advisor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func svc(ns, name, clusterIP string, selector map[string]string) *corev1.Service {
	return &corev1.Service{
		Namespace: ns, Name: name,
		Spec: corev1.ServiceSpec{ClusterIP: clusterIP, Selector: selector},
	}
}

func endpointSlice(ns, svcName string, readiness ...*bool) *discoveryv1.EndpointSlice {
	eps := &discoveryv1.EndpointSlice{
		Namespace: ns, Name: svcName + "-abc",
		Labels: map[string]string{discoveryv1.LabelServiceName: svcName},
	}
	for _, r := range readiness {
		eps.Endpoints = append(eps.Endpoints, discoveryv1.Endpoint{
			Conditions: discoveryv1.EndpointConditions{Ready: r},
		})
	}
	return eps
}

func TestStatefulSetServiceMissing(t *testing.T) {
	missing := statefulSet("prod", "no-svc", 2, map[string]string{"app": "a"}, hardened("db"))
	missing.Spec.ServiceName = "ghost"
	notHeadless := statefulSet("prod", "cluster-ip-svc", 2, map[string]string{"app": "b"}, hardened("db"))
	notHeadless.Spec.ServiceName = "regular"
	good := statefulSet("prod", "ok-svc", 2, map[string]string{"app": "c"}, hardened("db"))
	good.Spec.ServiceName = "headless"

	checks := fetchChecks(t,
		missing, notHeadless, good,
		svc("prod", "regular", "10.0.0.1", map[string]string{"app": "b"}),
		svc("prod", "headless", corev1.ClusterIPNone, map[string]string{"app": "c"}),
	)
	assert.True(t, checks["prod/StatefulSet/no-svc"]["statefulset_service_missing"])
	assert.True(t, checks["prod/StatefulSet/cluster-ip-svc"]["statefulset_service_not_headless"],
		"a non-headless governing service breaks per-pod DNS the same way")
	assert.False(t, checks["prod/StatefulSet/ok-svc"]["statefulset_service_missing"])
	assert.False(t, checks["prod/StatefulSet/ok-svc"]["statefulset_service_not_headless"])
}

func TestServiceEndpointChecks(t *testing.T) {
	checks := fetchChecks(t,
		svc("prod", "dead", "10.0.0.1", map[string]string{"app": "dead"}),
		svc("prod", "fragile", "10.0.0.2", map[string]string{"app": "fragile"}),
		endpointSlice("prod", "fragile", new(true)),
		svc("apps", "healthy", "10.0.0.3", map[string]string{"app": "healthy"}),
		endpointSlice("apps", "healthy", new(true), new(true)),
		// nil readiness counts as ready per the EndpointSlice contract.
		svc("prod", "unknown-ready", "10.0.0.4", map[string]string{"app": "u"}),
		endpointSlice("prod", "unknown-ready", nil, nil),
		// not-ready endpoints do not count.
		svc("prod", "all-down", "10.0.0.5", map[string]string{"app": "d"}),
		endpointSlice("prod", "all-down", new(false), new(false)),
		// selector-less services manage endpoints manually: skipped.
		svc("prod", "manual", "10.0.0.6", nil),
	)
	assert.True(t, checks["prod/Service/dead"]["service_no_endpoints"])
	assert.True(t, checks["prod/Service/fragile"]["single_endpoint_service"])
	assert.False(t, checks["prod/Service/fragile"]["service_no_endpoints"])
	assert.Empty(t, checks["apps/Service/healthy"], "multi-endpoint services in any namespace are clean")
	assert.Empty(t, checks["prod/Service/unknown-ready"])
	assert.True(t, checks["prod/Service/all-down"]["service_no_endpoints"])
	assert.Empty(t, checks["prod/Service/manual"])
}

// TestServiceChecksSkippedWhenListsForbidden: endpoint checks need both the
// Service and EndpointSlice lists; the STS check needs Services.
func TestServiceChecksSkippedWhenListsForbidden(t *testing.T) {
	sts := statefulSet("prod", "s", 2, map[string]string{"app": "a"}, hardened("db"))
	sts.Spec.ServiceName = "ghost"

	client := fake.NewSimpleClientset(
		sts,
		svc("prod", "dead", "10.0.0.1", map[string]string{"app": "dead"}),
	)
	forbidList(client, "endpointslices")
	s := NewWithClient(client)
	findings, err := s.Fetch(t.Context(), "", "")
	assert.NoError(t, err)
	got := checksFor(t, findings)
	assert.False(t, got["prod/Service/dead"]["service_no_endpoints"],
		"endpoint checks must be skipped when EndpointSlices are unlistable")
	assert.True(t, got["prod/StatefulSet/s"]["statefulset_service_missing"],
		"the STS governing-service check only needs the Service list")
}
