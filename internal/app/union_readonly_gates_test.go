package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func setClusterReadOnly(t *testing.T, ro map[string]bool) {
	t.Helper()
	prev := ui.ConfigClusterReadOnly
	t.Cleanup(func() { ui.ConfigClusterReadOnly = prev })
	ui.ConfigClusterReadOnly = ro
}

func unionActionModel(t *testing.T, cluster string) Model {
	t.Helper()
	setClusterReadOnly(t, map[string]bool{"prod": true})
	m := baseModelWithFakeClient()
	m.unionMode = true
	m.unionContexts = []string{"dev", "prod"}
	m.nav.Context = UnionContextSentinel
	m.actionCtx = actionContext{kind: "Deployment", name: "x", namespace: "default", context: cluster}
	return m
}

func TestOverlaySaveGates_HonorMemberReadOnly(t *testing.T) {
	saves := map[string]struct {
		label string
		run   func(m Model) Model
	}{
		"secret": {"Secret Editor", func(m Model) Model {
			m.secretData = &model.SecretData{Keys: []string{"k"}, Data: map[string]string{"k": "new"}}
			m.secretDataOriginal = map[string]string{"k": "old"}
			r, _ := m.handleSecretEditorKey(tea.KeyPressMsg{Code: tea.KeyEnter})
			return r.(Model)
		}},
		"configmap": {"ConfigMap Editor", func(m Model) Model {
			m.configMapData = &model.ConfigMapData{Keys: []string{"k"}, Data: map[string]string{"k": "new"}}
			m.configMapDataOriginal = map[string]string{"k": "old"}
			r, _ := m.handleConfigMapEditorKey(tea.KeyPressMsg{Code: tea.KeyEnter})
			return r.(Model)
		}},
		"auto-sync": {"Auto Sync", func(m Model) Model {
			r, _ := m.handleAutoSyncKey(tea.KeyPressMsg{Code: tea.KeyEnter})
			return r.(Model)
		}},
		"labels": {"Labels / Annotations", func(m Model) Model {
			m.labelData = &model.LabelAnnotationData{
				Labels: map[string]string{"a": "new"}, LabelKeys: []string{"a"},
				Annotations: map[string]string{},
			}
			m.labelLabelsOriginal = map[string]string{"a": "old"}
			m.labelAnnotationsOriginal = map[string]string{}
			r, _ := m.handleLabelEditorKey(tea.KeyPressMsg{Code: tea.KeyEnter})
			return r.(Model)
		}},
	}
	for name, s := range saves {
		for cluster, wantBlocked := range map[string]bool{"prod": true, "dev": false} {
			t.Run(name+"/"+cluster, func(t *testing.T) {
				rm := s.run(unionActionModel(t, cluster))
				if wantBlocked {
					assert.Equal(t, readOnlyBlockedMessage(s.label), rm.statusMessage)
				} else {
					assert.NotEqual(t, readOnlyBlockedMessage(s.label), rm.statusMessage)
				}
			})
		}
	}
}

func TestBatchLabelSave_GatesOnBulkSelectionCluster(t *testing.T) {
	cases := []struct {
		name        string
		bulkCluster string
		staleCtx    string
		wantBlocked bool
	}{
		{"read-only member, stale writable actionCtx", "prod", "dev", true},
		{"writable member, stale read-only actionCtx", "dev", "prod", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := unionActionModel(t, tc.staleCtx)
			m.bulkMode = true
			m.bulkItems = []model.Item{{Name: "x", Kind: "Deployment", Namespace: "default", ClusterName: tc.bulkCluster}}
			m.overlay = overlayBatchLabel
			m.batchLabelInput.Insert("a=b")
			r, _ := m.handleBatchLabelOverlayKey(tea.KeyPressMsg{Code: tea.KeyEnter})
			rm := r.(Model)
			if tc.wantBlocked {
				assert.Equal(t, readOnlyBlockedMessage("Labels / Annotations"), rm.statusMessage)
				assert.Equal(t, overlayNone, rm.overlay)
			} else {
				assert.NotEqual(t, readOnlyBlockedMessage("Labels / Annotations"), rm.statusMessage)
			}
		})
	}
}

func TestCustomActionGate_HonorsMemberReadOnly(t *testing.T) {
	prev := ui.ConfigCustomActions
	t.Cleanup(func() { ui.ConfigCustomActions = prev })
	ui.ConfigCustomActions = map[string][]ui.CustomAction{
		"Deployment": {{Label: "Poke", Command: "true"}},
	}
	for cluster, wantBlocked := range map[string]bool{"prod": true, "dev": false} {
		t.Run(cluster, func(t *testing.T) {
			m := unionActionModel(t, cluster)
			r, _ := m.executeActionDefault("Poke")
			rm := r.(Model)
			if wantBlocked {
				assert.Equal(t, readOnlyBlockedMessage("Poke"), rm.statusMessage)
			} else {
				assert.NotEqual(t, readOnlyBlockedMessage("Poke"), rm.statusMessage)
			}
		})
	}
}

func TestSingleContextReadOnly_BlocksCtrlEAndCustomAction(t *testing.T) {
	prev := ui.ConfigCustomActions
	t.Cleanup(func() { ui.ConfigCustomActions = prev })
	ui.ConfigCustomActions = map[string][]ui.CustomAction{"Pod": {{Label: "Poke", Command: "true"}}}

	setups := map[string]func(t *testing.T, m *Model){
		"cli flag": func(_ *testing.T, m *Model) {
			m.cliReadOnly = true
			m.recomputeReadOnly("test-ctx")
		},
		"config": func(t *testing.T, m *Model) {
			setClusterReadOnly(t, map[string]bool{"test-ctx": true})
			m.recomputeReadOnly("test-ctx")
		},
	}
	for name, setup := range setups {
		t.Run(name, func(t *testing.T) {
			m := baseModelWithFakeClient()
			m.nav.Level = model.LevelResources
			m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod", APIVersion: "v1", Resource: "pods", Namespaced: true}
			m.middleItems = []model.Item{{Name: "p", Kind: "Pod", Namespace: "default"}}
			m.setCursor(0)
			setup(t, &m)
			require.True(t, m.readOnly)

			r, _ := m.handleYAMLKeyCtrlE()
			assert.Equal(t, readOnlyBlockedMessage("Edit"), r.(Model).statusMessage)

			m.actionCtx = actionContext{kind: "Pod", name: "p", namespace: "default", context: "test-ctx"}
			r, _ = m.executeActionDefault("Poke")
			assert.Equal(t, readOnlyBlockedMessage("Poke"), r.(Model).statusMessage)
		})
	}
}
