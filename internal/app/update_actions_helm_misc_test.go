package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
)

// TestHandleExplorerActionKeyOpenBrowser covers the global ctrl+o keybinding
// gate: it must open a browser for Ingress resources and port-forward entries,
// and reject everything else.
func TestHandleExplorerActionKeyOpenBrowser(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(m *Model)
		wantErr bool
		wantMsg string
	}{
		{
			name: "ingress opens its precomputed URL",
			setup: func(m *Model) {
				m.nav.Level = model.LevelResources
				m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Ingress"}
				m.middleItems = []model.Item{{
					Name:    "my-ingress",
					Columns: []model.KeyValue{{Key: "__ingress_url", Value: "https://example.com"}},
				}}
			},
			wantErr: false,
			wantMsg: "Opening https://example.com",
		},
		{
			name: "port forward opens its localhost URL",
			setup: func(m *Model) {
				m.nav.Level = model.LevelResources
				m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__port_forwards__"}
				m.middleItems = []model.Item{{
					Name:    "Pod/my-pod  8080:80",
					Kind:    "__port_forward_entry__",
					Columns: []model.KeyValue{{Key: "Local", Value: "8080"}},
				}}
			},
			wantErr: false,
			wantMsg: "Opening http://localhost:8080",
		},
		{
			name: "unsupported kind is rejected",
			setup: func(m *Model) {
				m.nav.Level = model.LevelResources
				m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}
				m.middleItems = []model.Item{{Name: "my-pod", Kind: "Pod"}}
			},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := baseModelUpdate()
			tc.setup(&m)

			ret, cmd, handled := m.handleExplorerActionKeyOpenBrowser()
			result := ret.(Model)

			assert.True(t, handled)
			assert.NotNil(t, cmd)
			assert.True(t, result.hasStatusMessage())
			assert.Equal(t, tc.wantErr, result.statusMessageErr)
			if tc.wantMsg != "" {
				assert.Equal(t, tc.wantMsg, result.statusMessage)
			}
		})
	}
}

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
