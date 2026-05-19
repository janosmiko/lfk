package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
)

// TestRemoveSelectedPortForward covers the D / delete key path in the
// __port_forwards__ browser. The action-menu "Remove" shares the same
// manager call.
func TestRemoveSelectedPortForward(t *testing.T) {
	tests := []struct {
		name      string
		items     []model.Item
		wantCmd   bool // removal path ran (non-nil cmd)
		wantEmpty bool // middleItems rebuilt from the (empty) manager
	}{
		{
			name:    "nil selection is a no-op",
			items:   nil,
			wantCmd: false,
		},
		{
			name: "row without a valid port-forward ID is a no-op",
			items: []model.Item{{
				Name:    "Pod/my-pod  8080:80",
				Kind:    "__port_forward_entry__",
				Columns: []model.KeyValue{{Key: "ID", Value: "0"}},
			}},
			wantCmd: false,
		},
		{
			name: "row with a valid ID runs the removal path",
			items: []model.Item{{
				Name:    "Pod/my-pod  8080:80",
				Kind:    "__port_forward_entry__",
				Columns: []model.KeyValue{{Key: "ID", Value: "1"}},
			}},
			wantCmd:   true,
			wantEmpty: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := baseModelUpdate()
			m.nav.Level = model.LevelResources
			m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__port_forwards__"}
			m.portForwardMgr = k8s.NewPortForwardManager()
			m.middleItems = tc.items

			ret, cmd := m.removeSelectedPortForward()
			result := ret.(Model)
			assert.Equal(t, model.LevelResources, result.nav.Level)
			assert.Equal(t, tc.wantCmd, cmd != nil)
			if tc.wantEmpty {
				assert.Empty(t, result.middleItems)
			}
		})
	}
}
