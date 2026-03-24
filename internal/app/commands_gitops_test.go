package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

func TestBulkSyncArgoApps(t *testing.T) {
	m := Model{
		bulkItems: []model.Item{
			{Name: "app-1", Namespace: "argocd"},
			{Name: "app-2", Namespace: "argocd"},
		},
		actionCtx: actionContext{
			context:   "test-cluster",
			namespace: "argocd",
		},
	}
	cmd := m.bulkSyncArgoApps(false)
	assert.NotNil(t, cmd, "bulkSyncArgoApps should return a command")
}

func TestBulkSyncArgoAppsApplyOnly(t *testing.T) {
	m := Model{
		bulkItems: []model.Item{
			{Name: "app-1", Namespace: "argocd"},
		},
		actionCtx: actionContext{
			context:   "test-cluster",
			namespace: "argocd",
		},
	}
	cmd := m.bulkSyncArgoApps(true)
	assert.NotNil(t, cmd, "bulkSyncArgoApps(applyOnly=true) should return a command")
}

func TestBulkRefreshArgoApps(t *testing.T) {
	m := Model{
		bulkItems: []model.Item{
			{Name: "app-1", Namespace: "argocd"},
			{Name: "app-2", Namespace: "argocd"},
			{Name: "app-3", Namespace: "argocd"},
		},
		actionCtx: actionContext{
			context:   "test-cluster",
			namespace: "argocd",
		},
	}
	cmd := m.bulkRefreshArgoApps()
	assert.NotNil(t, cmd, "bulkRefreshArgoApps should return a command")
}
