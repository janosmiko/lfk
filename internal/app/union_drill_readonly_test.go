package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func TestUnionDrillDown_KeepsReadOnlyForMember(t *testing.T) {
	deployRT := model.ResourceTypeEntry{Kind: "Deployment", APIGroup: "apps", APIVersion: "v1", Resource: "deployments", Namespaced: true}

	drills := map[string]func(m Model) Model{
		"resource": func(m Model) Model {
			m.nav.Level = model.LevelResources
			m.nav.ResourceType = deployRT
			m.middleItems = []model.Item{{Name: "web", Kind: "Deployment", Namespace: "default", ClusterName: "prod"}}
			m.setCursor(0)
			r, _ := m.navigateChild()
			return r.(Model)
		},
		"owned": func(m Model) Model {
			m.nav.Level = model.LevelOwned
			m.nav.ResourceType = deployRT
			m.nav.ResourceName = "web"
			m.middleItems = []model.Item{{Name: "web-1", Kind: "Pod", Namespace: "default", ClusterName: "prod"}}
			m.setCursor(0)
			r, _ := m.navigateChild()
			return r.(Model)
		},
	}
	setups := map[string]func(t *testing.T, m *Model){
		"config": func(t *testing.T, _ *Model) {
			prev := ui.ConfigClusterReadOnly
			t.Cleanup(func() { ui.ConfigClusterReadOnly = prev })
			ui.ConfigClusterReadOnly = map[string]bool{"prod": true}
		},
		"ctrl+r override": func(_ *testing.T, m *Model) {
			m.contextROOverrides = map[string]bool{"prod": true}
		},
	}
	actions := []struct{ kind, label string }{
		{"Pod", "Delete"}, {"Pod", "Exec"}, {"Deployment", "Restart"},
	}

	for drillName, drill := range drills {
		for setupName, setup := range setups {
			t.Run(drillName+"/"+setupName, func(t *testing.T) {
				m := baseModelWithFakeClient()
				m.unionMode = true
				m.unionContexts = []string{"dev", "prod"}
				m.nav.Context = UnionContextSentinel
				setup(t, &m)

				m = drill(m)
				require.Equal(t, "prod", m.nav.Context)

				for _, a := range actions {
					m.actionCtx = actionContext{kind: a.kind, name: "x", namespace: "default", context: m.nav.Context}
					msg, blocked := m.actionBlockedReason(a.kind, a.label)
					assert.True(t, blocked, a.label)
					assert.Equal(t, readOnlyBlockedMessage(a.label), msg)
				}
			})
		}
	}
}
