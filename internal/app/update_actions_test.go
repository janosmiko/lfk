package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

func TestDirectActionDelete_DeletingPod_ForceDeletes(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true},
		},
		middleItems: []model.Item{
			{Name: "stuck-pod", Kind: "Pod", Namespace: "default", Deleting: true},
		},
		tabs:  []TabState{{}},
		width: 80, height: 40,
	}
	ret, _ := m.directActionDelete()
	result := ret.(Model)
	assert.Equal(t, overlayConfirmType, result.overlay)
	assert.Equal(t, "Force Delete", result.pendingAction)
	assert.Contains(t, result.confirmTitle, "Force Delete")
	assert.Contains(t, result.confirmQuestion, "--force --grace-period=0")
}

func TestDirectActionDelete_DeletingDeployment_ForceFinalize(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "Deployment", Resource: "deployments", Namespaced: true},
		},
		middleItems: []model.Item{
			{Name: "stuck-deploy", Kind: "Deployment", Namespace: "default", Deleting: true},
		},
		tabs:  []TabState{{}},
		width: 80, height: 40,
	}
	ret, _ := m.directActionDelete()
	result := ret.(Model)
	assert.Equal(t, overlayConfirmType, result.overlay)
	assert.Equal(t, "Force Finalize", result.pendingAction)
	assert.Contains(t, result.confirmTitle, "Force Finalize")
}

func TestDirectActionDelete_DeletingJob_ForceDeletes(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "Job", Resource: "jobs", Namespaced: true},
		},
		middleItems: []model.Item{
			{Name: "stuck-job", Kind: "Job", Namespace: "default", Deleting: true},
		},
		tabs:  []TabState{{}},
		width: 80, height: 40,
	}
	ret, _ := m.directActionDelete()
	result := ret.(Model)
	assert.Equal(t, overlayConfirmType, result.overlay)
	assert.Equal(t, "Force Delete", result.pendingAction)
}

func TestDirectActionDelete_NormalPod_NormalDelete(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true},
		},
		middleItems: []model.Item{
			{Name: "healthy-pod", Kind: "Pod", Namespace: "default", Deleting: false},
		},
		tabs:  []TabState{{}},
		width: 80, height: 40,
	}
	ret, _ := m.directActionDelete()
	result := ret.(Model)
	// Normal delete goes through executeAction which opens overlayConfirm.
	assert.Equal(t, overlayConfirm, result.overlay)
	assert.Equal(t, "Delete", result.pendingAction)
}

func TestOpenActionMenu_DeletingPod_ShowsForceDelete(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true},
		},
		middleItems: []model.Item{
			{Name: "stuck-pod", Kind: "Pod", Namespace: "default", Deleting: true},
		},
		tabs:  []TabState{{}},
		width: 80, height: 40,
	}
	ret, _ := m.openActionMenu()
	result := ret.(Model)
	assert.Equal(t, overlayAction, result.overlay)

	// Find the Force Delete item in the menu.
	found := false
	for _, item := range result.overlayItems {
		if item.Name == "Force Delete" {
			found = true
			assert.Contains(t, item.Extra, "--force --grace-period=0")
			break
		}
	}
	assert.True(t, found, "expected Force Delete in menu for deleting Pod")

	// Should NOT have a "Force Finalize" or plain "Delete" entry.
	for _, item := range result.overlayItems {
		assert.NotEqual(t, "Force Finalize", item.Name, "should not show Force Finalize for Pod")
	}
}

func TestOpenActionMenu_DeletingDeployment_ShowsForceFinalize(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			ResourceType: model.ResourceTypeEntry{Kind: "Deployment", Resource: "deployments", Namespaced: true},
		},
		middleItems: []model.Item{
			{Name: "stuck-deploy", Kind: "Deployment", Namespace: "default", Deleting: true},
		},
		tabs:  []TabState{{}},
		width: 80, height: 40,
	}
	ret, _ := m.openActionMenu()
	result := ret.(Model)
	assert.Equal(t, overlayAction, result.overlay)

	found := false
	for _, item := range result.overlayItems {
		if item.Name == "Force Finalize" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected Force Finalize in menu for deleting Deployment")
}
