package k8s

import (
	"testing"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

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

	rows, err := client.webhookConstraintRows(t.Context(), "", target)
	if err != nil {
		t.Fatalf("webhookConstraintRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 row, got %d: %+v", len(rows), rows)
	}
	if rows[0].Name != "full-match" {
		t.Errorf("Name = %q, want full-match", rows[0].Name)
	}
}
