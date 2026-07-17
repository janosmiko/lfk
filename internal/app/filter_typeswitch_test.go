package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/security"
)

// These tests pin TASK-839's rule across every path that lands the user on a
// different resource list: the committed quick filter of the origin list must
// never leak into the destination.

// Regular navigation: jobs (filtered) -> h to types -> descend into deployments.
func TestNavigateChild_TypeSwitchClearsFilter(t *testing.T) {
	m := gotoTestModel()
	m.discoveredResources["ctx"] = append(m.discoveredResources["ctx"],
		model.ResourceTypeEntry{Kind: "Job", APIGroup: "batch", APIVersion: "v1", Resource: "jobs", Namespaced: true})
	typesItems := model.BuildSidebarItems(m.discoveredResources["ctx"])
	m.setMiddleItems(typesItems)
	m.pushLeft()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Job", APIGroup: "batch", APIVersion: "v1", Resource: "jobs", Namespaced: true}
	m.filterText = "my-job"
	m.filterInput.Set("my-job")

	out, _ := m.navigateParent()
	m = out.(Model)
	if m.filterText != "" {
		t.Fatalf("after back-nav to types, filterText=%q", m.filterText)
	}

	dep := model.ResourceTypeEntry{Kind: "Deployment", APIGroup: "apps", APIVersion: "v1", Resource: "deployments", Namespaced: true}
	for i, item := range m.visibleMiddleItems() {
		if item.Extra == dep.ResourceRef() {
			m.setCursor(i)
			break
		}
	}
	out, _ = m.navigateChild()
	m = out.(Model)
	if m.filterText != "" || m.filterInput.Value != "" {
		t.Fatalf("regular navigation leaked the filter: filterText=%q input=%q", m.filterText, m.filterInput.Value)
	}
}

// Security-finding teleport: Enter on an affected resource must not carry the
// finding view's filter into the real resource list.
func TestJumpToFindingResource_ClearsQuickFilter(t *testing.T) {
	m := baseModelBoost2()
	m.discoveredResources["test-ctx"] = []model.ResourceTypeEntry{
		{Kind: "Deployment", APIGroup: "apps", APIVersion: "v1", Resource: "deployments", Namespaced: true},
	}
	m.nav.Level = model.LevelOwned
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__security_falco__", APIGroup: "_security"}
	m.nav.ResourceName = "privileged"
	m.securityActiveGroup = "privileged"
	m.filterText = "api"
	m.filterInput.Set("api")
	sel := &model.Item{
		Kind:      "__security_affected_resource__",
		Name:      "deploy/api",
		Namespace: "prod",
		Columns: []model.KeyValue{
			{Key: "__resource_key__", Value: "prod/Deployment/api"},
		},
	}
	m.middleItems = []model.Item{*sel}
	m.setCursor(0)

	out, _ := m.jumpToFindingResource(sel)
	rm := out.(Model)
	if rm.nav.Level != model.LevelResources {
		t.Fatalf("expected teleport to LevelResources, got %v", rm.nav.Level)
	}
	if rm.filterText != "" || rm.filterInput.Value != "" {
		t.Fatalf("finding teleport leaked the filter: filterText=%q input=%q", rm.filterText, rm.filterInput.Value)
	}
}

// Security-findings teleport: opening a resource's findings pseudo-list must
// not keep the origin list's filter or preset state.
func TestOpenSecurityFindingsForResource_ClearsQuickFilter(t *testing.T) {
	m := baseModelBoost2()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", APIVersion: "v1", Resource: "pods", Namespaced: true}
	m.filterText = "nginx"
	m.filterInput.Set("nginx")
	m.filterBroadMode = true
	m.activeFilterPreset = &FilterPreset{Name: "p"}
	m.unfilteredMiddleItems = []model.Item{{Name: "pod-a"}}

	rm, _ := m.openSecurityFindingsForResource(
		[]security.ResourceRef{{Namespace: "prod", Kind: "Pod", Name: "api"}}, "Pod", "api")
	if rm.nav.ResourceType.Kind != model.SecurityResourceFindingsKind {
		t.Fatalf("expected findings pseudo type, got %q", rm.nav.ResourceType.Kind)
	}
	if rm.filterText != "" || rm.filterInput.Value != "" || rm.filterActive || rm.filterBroadMode {
		t.Fatalf("findings teleport leaked filter state: text=%q input=%q active=%v broad=%v",
			rm.filterText, rm.filterInput.Value, rm.filterActive, rm.filterBroadMode)
	}
	if rm.activeFilterPreset != nil || rm.unfilteredMiddleItems != nil {
		t.Fatal("findings teleport must clear filter preset state")
	}
}

// Port-forwards teleport: opening the Port Forwards pseudo-list must not keep
// the origin list's filter (it would hide the forwards).
func TestNavigateToPortForwards_ClearsQuickFilter(t *testing.T) {
	m := baseModelWithFakeClient()
	m.portForwardMgr = k8s.NewPortForwardManager()
	m.nav.Level = model.LevelResources
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", APIVersion: "v1", Resource: "pods", Namespaced: true}
	m.filterText = "nginx"
	m.filterInput.Set("nginx")

	m.navigateToPortForwards()
	if m.nav.ResourceType.Kind != "__port_forwards__" {
		t.Fatalf("expected port-forwards pseudo type, got %q", m.nav.ResourceType.Kind)
	}
	if m.filterText != "" || m.filterInput.Value != "" {
		t.Fatalf("port-forwards teleport leaked the filter: filterText=%q input=%q", m.filterText, m.filterInput.Value)
	}
}
