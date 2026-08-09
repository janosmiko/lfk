package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func pdbWithSelector(sel *metav1.LabelSelector) *policyv1.PodDisruptionBudget {
	return &policyv1.PodDisruptionBudget{Spec: policyv1.PodDisruptionBudgetSpec{Selector: sel}}
}

func TestPDBSelectorMatches(t *testing.T) {
	tests := []struct {
		name      string
		selector  *metav1.LabelSelector
		podLabels map[string]string
		want      bool
	}{
		{
			name:      "nil selector selects nothing",
			selector:  nil,
			podLabels: map[string]string{"app": "web"},
			want:      false,
		},
		{
			name:      "empty selector selects every pod in the namespace",
			selector:  &metav1.LabelSelector{},
			podLabels: map[string]string{"app": "web"},
			want:      true,
		},
		{
			name:      "matchLabels hit",
			selector:  &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
			podLabels: map[string]string{"app": "web", "tier": "front"},
			want:      true,
		},
		{
			name:      "matchLabels miss",
			selector:  &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
			podLabels: map[string]string{"app": "api"},
			want:      false,
		},
		{
			name: "matchExpressions In",
			selector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
				{Key: "tier", Operator: metav1.LabelSelectorOpIn, Values: []string{"front", "back"}},
			}},
			podLabels: map[string]string{"tier": "back"},
			want:      true,
		},
		{
			name: "an operator the API rejects selects nothing",
			selector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
				{Key: "tier", Operator: "NotAnOperator", Values: []string{"front"}},
			}},
			podLabels: map[string]string{"tier": "front"},
			want:      false,
		},
		{
			name:      "a pod with no labels misses a real selector",
			selector:  &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
			podLabels: nil,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, PDBSelectorMatches(pdbWithSelector(tt.selector), tt.podLabels))
		})
	}
}
