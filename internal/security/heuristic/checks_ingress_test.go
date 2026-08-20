package heuristic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func ingress(ns, name string, tls bool, hosts ...string) *networkingv1.Ingress {
	ing := &networkingv1.Ingress{Namespace: ns, Name: name}
	if tls {
		ing.Spec.TLS = []networkingv1.IngressTLS{{Hosts: hosts}}
	}
	for _, h := range hosts {
		ing.Spec.Rules = append(ing.Spec.Rules, networkingv1.IngressRule{Host: h})
	}
	return ing
}

func TestIngressChecks(t *testing.T) {
	s := NewWithClient(fake.NewSimpleClientset(
		ingress("prod", "plain", false, "app.example.com"),
		ingress("prod", "secure", true, "app.example.com"),
		ingress("prod", "catch-all", true, ""),
		// Two empty-host rules produce one finding.
		ingress("prod", "multi-empty", true, "", ""),
		ingress("ignored-ns", "skipped", false, "h"),
	))
	s.SetIgnoredNamespaces([]string{"ignored-ns"})
	checks := checksByResource(t, s)

	assert.True(t, checks["prod/Ingress/plain"]["ingress_no_tls"])
	assert.False(t, checks["prod/Ingress/plain"]["ingress_empty_host"])
	assert.Empty(t, checks["prod/Ingress/secure"])
	assert.True(t, checks["prod/Ingress/catch-all"]["ingress_empty_host"])
	assert.False(t, checks["prod/Ingress/catch-all"]["ingress_no_tls"])
	assert.True(t, checks["prod/Ingress/multi-empty"]["ingress_empty_host"])
	assert.Empty(t, checks["ignored-ns/Ingress/skipped"], "ignored namespaces are skipped")
}

func TestIngressListBestEffort(t *testing.T) {
	client := fake.NewSimpleClientset(ingress("prod", "plain", false, "h"))
	forbidList(client, "ingresses")
	s := NewWithClient(client)
	checks := checksByResource(t, s)
	assert.Empty(t, checks["prod/Ingress/plain"])
}
