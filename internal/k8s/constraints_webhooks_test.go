package k8s

import (
	"testing"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestRulesMatchTarget_Scope(t *testing.T) {
	clusterScope := admissionregistrationv1.ClusterScope
	namespacedScope := admissionregistrationv1.NamespacedScope
	allScopes := admissionregistrationv1.AllScopes
	deployments := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

	rule := func(scope *admissionregistrationv1.ScopeType, resources ...string) []admissionregistrationv1.RuleWithOperations {
		return []admissionregistrationv1.RuleWithOperations{{
			Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.OperationAll},
			APIGroups:  []string{"*"}, APIVersions: []string{"*"}, Resources: resources, Scope: scope,
		}}
	}

	tests := []struct {
		name         string
		rules        []admissionregistrationv1.RuleWithOperations
		targetScope  admissionregistrationv1.ScopeType
		wantMatch    bool
		wantWildcard bool
	}{
		{"cluster rule against a namespaced target", rule(&clusterScope, "deployments"), namespacedScope, false, false},
		{"namespaced rule against a cluster target", rule(&namespacedScope, "deployments"), clusterScope, false, false},
		{"namespaced rule against a namespaced target", rule(&namespacedScope, "deployments"), namespacedScope, true, false},
		{"unset scope places no restriction", rule(nil, "deployments"), clusterScope, true, false},
		{"explicit all scopes", rule(&allScopes, "deployments"), clusterScope, true, false},
		{"*/* matches every resource", rule(nil, "*/*"), namespacedScope, true, true},
		{"* matches the bare resource", rule(nil, "*"), namespacedScope, true, true},
		{"resource/* does not match the bare resource", rule(nil, "deployments/*"), namespacedScope, false, false},
		{"exact resource matches", rule(nil, "deployments"), namespacedScope, true, false},
		{"other resource does not match", rule(nil, "statefulsets"), namespacedScope, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, wildcard := rulesMatchTarget(tt.rules, deployments, tt.targetScope)
			if matched != tt.wantMatch || wildcard != tt.wantWildcard {
				t.Errorf("rulesMatchTarget = (%v, %v), want (%v, %v)", matched, wildcard, tt.wantMatch, tt.wantWildcard)
			}
		})
	}
}

func TestWebhookRows_NamespaceSelectorForClusterScopedTargets(t *testing.T) {
	sideEffects := admissionregistrationv1.SideEffectClassNone
	cfg := &admissionregistrationv1.ValidatingWebhookConfiguration{
		Name: "ns-scoped",
		Webhooks: []admissionregistrationv1.ValidatingWebhook{{
			Name:        "ns-scoped.example.com",
			SideEffects: &sideEffects,
			Rules: []admissionregistrationv1.RuleWithOperations{{
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.OperationAll},
				APIGroups:  []string{"*"}, APIVersions: []string{"*"}, Resources: []string{"*"},
			}},
			NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
		}},
	}
	client := newFakeClient(k8sfake.NewClientset(cfg), nil)

	tests := []struct {
		name     string
		target   ConstraintTarget
		wantRows int
	}{
		{
			"Namespace object matches on its own labels",
			ConstraintTarget{
				Kind:   "Namespace",
				Labels: map[string]string{"env": "prod"},
				GVR:    schema.GroupVersionResource{Version: "v1", Resource: "namespaces"},
			},
			1,
		},
		{
			"Namespace object with other labels is excluded",
			ConstraintTarget{
				Kind:   "Namespace",
				Labels: map[string]string{"env": "staging"},
				GVR:    schema.GroupVersionResource{Version: "v1", Resource: "namespaces"},
			},
			0,
		},
		{
			"Node ignores namespaceSelector entirely",
			ConstraintTarget{
				Kind: "Node",
				GVR:  schema.GroupVersionResource{Version: "v1", Resource: "nodes"},
			},
			1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := client.validatingWebhookConstraintRows(t.Context(), "", tt.target)
			if err != nil {
				t.Fatalf("validatingWebhookConstraintRows: %v", err)
			}
			if len(rows) != tt.wantRows {
				t.Errorf("rows = %d, want %d: %+v", len(rows), tt.wantRows, rows)
			}
		})
	}
}

func TestWebhookRows_OnlyMatchingConfigurations(t *testing.T) {
	sideEffects := admissionregistrationv1.SideEffectClassNone
	ns := &corev1.Namespace{
		Name: "default", Labels: map[string]string{"env": "prod"},
	}

	ruleMismatch := &admissionregistrationv1.ValidatingWebhookConfiguration{
		Name: "rule-mismatch",
		Webhooks: []admissionregistrationv1.ValidatingWebhook{{
			Name:        "rule-mismatch.example.com",
			SideEffects: &sideEffects,
			Rules: []admissionregistrationv1.RuleWithOperations{{
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
				APIGroups:  []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"},
			}},
		}},
	}
	nsSelectorMismatch := &admissionregistrationv1.ValidatingWebhookConfiguration{
		Name: "ns-mismatch",
		Webhooks: []admissionregistrationv1.ValidatingWebhook{{
			Name:        "ns-mismatch.example.com",
			SideEffects: &sideEffects,
			Rules: []admissionregistrationv1.RuleWithOperations{{
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update},
				APIGroups:  []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"},
			}},
			NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "staging"}},
		}},
	}
	fullMatch := &admissionregistrationv1.ValidatingWebhookConfiguration{
		Name: "full-match",
		Webhooks: []admissionregistrationv1.ValidatingWebhook{{
			Name:        "full-match.example.com",
			SideEffects: &sideEffects,
			Rules: []admissionregistrationv1.RuleWithOperations{{
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update, admissionregistrationv1.Delete},
				APIGroups:  []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"},
			}},
			NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
		}},
	}

	client := newFakeClient(k8sfake.NewClientset(ns, ruleMismatch, nsSelectorMismatch, fullMatch), nil)

	target := ConstraintTarget{
		Namespace: "default",
		GVR:       schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
	}

	rows, err := client.validatingWebhookConstraintRows(t.Context(), "", target)
	if err != nil {
		t.Fatalf("validatingWebhookConstraintRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 row, got %d: %+v", len(rows), rows)
	}
	if rows[0].Name != "full-match" {
		t.Errorf("Name = %q, want full-match", rows[0].Name)
	}
}
