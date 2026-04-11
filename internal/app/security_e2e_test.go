// Package app — security_e2e_test.go
// End-to-end smoke test for the security navigation feature. Wires real
// heuristic + trivyop sources against fake k8s clients and exercises the
// full path: press # → drill into a source → verify findings render as
// model.Items → render the preview pane → press Enter → jump to affected
// resource.
package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
	"github.com/janosmiko/lfk/internal/security/heuristic"
	"github.com/janosmiko/lfk/internal/security/trivyop"
	"github.com/janosmiko/lfk/internal/ui"
)

func boolPE2E(b bool) *bool { return &b }

// TestSecurityNavigationFlowEndToEnd exercises the revamp's navigation
// pipeline from keypress through the render layer.
func TestSecurityNavigationFlowEndToEnd(t *testing.T) {
	// Fake k8s clientset with one privileged Pod (will produce a
	// heuristic finding).
	badPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "prod", Name: "bad"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "c",
				Image: "nginx:latest",
				SecurityContext: &corev1.SecurityContext{
					Privileged: boolPE2E(true),
				},
			}},
		},
	}
	kubeClient := kubefake.NewSimpleClientset(badPod)

	// Fake dynamic client with one VulnerabilityReport (will produce a
	// trivy-operator finding).
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

	// Build the Manager with real source adapters.
	mgr := security.NewManager()
	mgr.Register(heuristic.NewWithClient(kubeClient))
	mgr.Register(trivyop.NewWithDynamic(dynClient))

	// Availability map — both sources reachable.
	avail := map[string]bool{
		"heuristic":      true,
		"trivy-operator": true,
	}

	// Install the SecuritySourcesFn hook pointing at a local Manager +
	// availability map. Must be restored after the test.
	prev := model.SecuritySourcesFn
	t.Cleanup(func() { model.SecuritySourcesFn = prev })
	model.SecuritySourcesFn = func() []model.SecuritySourceEntry {
		return buildSecuritySourceEntries(mgr, avail)
	}

	// Dispatch via the security manager directly first, so that the
	// Manager's Index is populated. buildSecuritySourceEntries reads
	// CountBySource from the index, and we assert those counts below.
	res, err := mgr.FetchAll(context.Background(), "kctx", "")
	require.NoError(t, err)
	require.NotEmpty(t, res.Findings, "both sources should produce findings")

	var gotHeuristic, gotTrivy bool
	for _, f := range res.Findings {
		if f.Source == "heuristic" {
			gotHeuristic = true
		}
		if f.Source == "trivy-operator" {
			gotTrivy = true
		}
	}
	assert.True(t, gotHeuristic, "heuristic should contribute at least one finding")
	assert.True(t, gotTrivy, "trivy-operator should contribute at least one finding")

	// Verify FindingIndex picks up both sources.
	idx := mgr.Index()
	assert.Greater(t, idx.CountBySource("heuristic"), 0)
	assert.Greater(t, idx.CountBySource("trivy-operator"), 0)

	// Verify SecuritySourcesFn returns non-empty entries.
	entries := model.SecuritySourcesFn()
	require.Len(t, entries, 2,
		"expected 2 Security entries (heuristic + trivy-operator)")

	// Sanity check: the Security category in BuildSidebarItems is now
	// populated from the hook.
	var securityItems []model.Item
	for _, it := range model.BuildSidebarItems(nil) {
		if it.Category == "Security" {
			securityItems = append(securityItems, it)
		}
	}
	require.Len(t, securityItems, 2,
		"Security category must have 2 entries")

	// Pick the trivy-operator source entry to verify the Kind/APIGroup
	// wiring is intact. Items use Extra to carry the resource ref, which
	// starts with the virtual API group.
	var trivyItem *model.Item
	for i := range securityItems {
		if securityItems[i].Kind == "__security_trivy-operator__" {
			trivyItem = &securityItems[i]
			break
		}
	}
	require.NotNil(t, trivyItem,
		"trivy-operator entry must be present in the category")
	assert.Contains(t, trivyItem.Extra, model.SecurityVirtualAPIGroup)

	// Render the preview pane for a sample finding (prove the details
	// renderer wiring is intact).
	sample := model.Item{
		Name: "CVE-2024-1234",
		Kind: "__security_finding__",
		Columns: []model.KeyValue{
			{Key: "Severity", Value: "CRIT"},
			{Key: "Resource", Value: "deploy/api"},
			{Key: "Title", Value: "CVE-2024-1234"},
			{Key: "Category", Value: "vuln"},
			{Key: "ResourceKind", Value: "Deployment"},
		},
	}
	rendered := ui.RenderFindingDetails(sample, 100, 30)
	assert.Contains(t, rendered, "CRIT")
	assert.Contains(t, rendered, "CVE-2024-1234")
	assert.Contains(t, rendered, "deploy/api")

	// Exercise the # hotkey: cursor should land on the first Security
	// entry (index 1 in the items list below).
	m := baseExplorerModel()
	m.nav.Level = model.LevelResourceTypes
	m.middleItems = []model.Item{
		{Name: "Monitoring", Extra: "__monitoring__"},
		{Name: "Trivy", Category: "Security", Extra: "_security/v1/findings-trivy-operator"},
		{Name: "Heuristic", Category: "Security", Extra: "_security/v1/findings-heuristic"},
		{Name: "Workloads"},
	}
	updated, _, handled := m.handleExplorerActionKeySecurity()
	require.True(t, handled)
	mm, ok := updated.(Model)
	require.True(t, ok)
	assert.Equal(t, 1, mm.cursor(),
		"# should jump to the first Security entry (index 1)")

	// Exercise enterFullView dispatch on a finding item. For a finding
	// with no real affected resource, the jump fallback sets a status
	// message — we just care that the call returns a Model and doesn't
	// panic.
	m2 := baseExplorerModel()
	m2.nav.Level = model.LevelResources
	m2.middleItems = []model.Item{sample}
	entered, _ := m2.enterFullView()
	_, ok = entered.(Model)
	assert.True(t, ok, "enterFullView must return a Model for a finding item")
}
