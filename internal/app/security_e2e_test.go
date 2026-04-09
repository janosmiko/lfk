// End-to-end smoke test for the security dashboard. This wires a real
// security.Manager, the real heuristic source, and the real trivy-operator
// source against fake kubernetes and dynamic clients to exercise the
// dispatch path the live application uses.
package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/security"
	"github.com/janosmiko/lfk/internal/security/heuristic"
	"github.com/janosmiko/lfk/internal/security/trivyop"
	"github.com/janosmiko/lfk/internal/ui"
)

func boolPtrE2E(b bool) *bool { return &b }

func TestSecurityFlowEndToEnd(t *testing.T) {
	// Fake kubernetes clientset with a privileged container that the heuristic
	// source will flag as a misconfiguration.
	badPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "bad"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "c",
				Image: "nginx:latest",
				SecurityContext: &corev1.SecurityContext{
					Privileged: boolPtrE2E(true),
				},
			}},
		},
	}
	kubeClient := kubefake.NewSimpleClientset(badPod)

	// Fake dynamic client with one VulnerabilityReport that the trivy-operator
	// source will surface as a critical CVE finding.
	vuln := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "aquasecurity.github.io/v1alpha1",
			"kind":       "VulnerabilityReport",
			"metadata": map[string]interface{}{
				"namespace": "prod",
				"name":      "vr1",
				"labels": map[string]interface{}{
					"trivy-operator.resource.kind":  "Deployment",
					"trivy-operator.resource.name":  "api",
					"trivy-operator.container.name": "app",
				},
			},
			"report": map[string]interface{}{
				"vulnerabilities": []interface{}{
					map[string]interface{}{
						"vulnerabilityID": "CVE-2024-1234",
						"severity":        "CRITICAL",
						"resource":        "openssl",
					},
				},
			},
		},
	}
	scheme := runtime.NewScheme()
	dynClient := dynfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			trivyop.VulnerabilityReportGVR: "VulnerabilityReportList",
			trivyop.ConfigAuditReportGVR:   "ConfigAuditReportList",
		},
		vuln,
	)

	// Build a Manager wired with both real sources against the fake clients.
	mgr := security.NewManager()
	mgr.Register(heuristic.NewWithClient(kubeClient))
	mgr.Register(trivyop.NewWithDynamic(dynClient))

	m := Model{securityManager: mgr}
	m.nav.Context = "kctx"

	// Trigger the fetch via the same path the app uses for the # hotkey.
	cmd := m.loadSecurityDashboard()
	require.NotNil(t, cmd, "loadSecurityDashboard must return a non-nil command")
	msg := cmd()

	loaded, ok := msg.(securityFindingsLoadedMsg)
	require.True(t, ok, "expected securityFindingsLoadedMsg, got %T", msg)
	updated := m.handleSecurityFindingsLoaded(loaded)

	// Both sources should have contributed findings.
	var gotHeuristic, gotTrivy bool
	for _, f := range updated.securityView.Findings {
		switch f.Source {
		case "heuristic":
			gotHeuristic = true
		case "trivy-operator":
			gotTrivy = true
		}
	}
	assert.True(t, gotHeuristic, "heuristic findings should be present")
	assert.True(t, gotTrivy, "trivy-operator findings should be present")

	// Both source categories should now be available for tab cycling.
	assert.Contains(t, updated.securityView.AvailableCategories, security.CategoryVuln)
	assert.Contains(t, updated.securityView.AvailableCategories, security.CategoryMisconfig)

	// Render the dashboard and assert that key elements are present.
	rendered := ui.RenderSecurityDashboard(updated.securityView, 100, 30)
	assert.Contains(t, rendered, "Security Dashboard")
	assert.Contains(t, rendered, "CVE-2024-1234")

	// Press Tab and verify the active category cycles. With the default
	// AvailableCategories order (Vuln, Misconfig), Tab should advance to
	// the next entry.
	originalCategory := updated.securityView.ActiveCategory
	tabbed, _ := updated.handleSecurityKey(tea.KeyMsg{Type: tea.KeyTab})
	assert.NotEqual(t, originalCategory, tabbed.securityView.ActiveCategory,
		"Tab should cycle the active category")

	// The manager's FindingIndex must contain the Deployment that the
	// trivy-operator finding referenced. This proves the SEC column wiring
	// stays in sync with the async fetch path.
	idx := mgr.Index()
	deploymentRef := security.ResourceRef{Namespace: "prod", Kind: "Deployment", Name: "api"}
	counts := idx.For(deploymentRef)
	assert.Greater(t, counts.Total(), 0, "FindingIndex should aggregate the Deployment finding")
}
