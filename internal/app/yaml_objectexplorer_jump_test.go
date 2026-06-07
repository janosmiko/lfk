package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// yamlOEJumpModel returns a YAML viewer that was opened from the Object
// Explorer, so the preserved tree (root) is reused on jump-back.
func yamlOEJumpModel() Model {
	m := Model{
		width: 80, height: 30, mode: modeYAML,
		yamlReturnMode: modeObjectExplorer,
		yamlView: yamlViewState{
			content:   "spec:\n  replicas: 3",
			collapsed: map[string]bool{},
			cursor:    1,
		},
		tabs: []TabState{{}},
	}
	m.objectExplorerView.root = map[string]any{
		"spec": map[string]any{"replicas": int64(3)},
	}
	m.objectExplorerView.level = model.ObjectFieldsAt(m.objectExplorerView.root, nil)
	return m
}

// L963: O is the Object Explorer key everywhere, so it must trigger the
// transitive jump from the YAML viewer (previously bound to P).
func TestYAMLKeyOJumpsToObjectExplorer(t *testing.T) {
	m := yamlOEJumpModel()
	ret, _ := m.handleYAMLNormalKey(keyMsg("O"))
	rm := ret.(Model)
	assert.Equal(t, modeObjectExplorer, rm.mode, "O must switch to the Object Explorer")
}

// P no longer performs the jump (replaced by O).
func TestYAMLKeyPNoLongerJumpsToObjectExplorer(t *testing.T) {
	m := yamlOEJumpModel()
	ret, _ := m.handleYAMLNormalKey(keyMsg("P"))
	rm := ret.(Model)
	assert.Equal(t, modeYAML, rm.mode, "P must no longer switch to the Object Explorer")
}

// L964: the top breadcrumb shows the resource name + the cursor's attribute
// path while in the YAML viewer, mirroring the Object Explorer location. The
// path lives in the breadcrumb (top title bar), not the viewer's sub-title.
func TestYAMLBreadcrumbDrillPathShowsNameAndPath(t *testing.T) {
	m := Model{
		mode:      modeYAML,
		nav:       model.NavigationState{Level: model.LevelResources},
		namespace: "default",
		middleItems: []model.Item{
			{Name: "my-pod", Kind: "Pod"},
		},
		yamlView: yamlViewState{
			content:   "spec:\n  replicas: 3",
			collapsed: map[string]bool{},
			cursor:    1,
		},
	}
	// At LevelResources the nav breadcrumb stops at the type, so the YAML
	// drill path must contribute both the resource name and the attribute path.
	assert.Equal(t, []string{"my-pod", "spec.replicas"}, m.explorerDrillPath())
}

// The viewer sub-title keeps only the resource label (no attribute path — that
// now lives in the breadcrumb).
func TestYamlTitleHasNoPath(t *testing.T) {
	m := Model{
		nav:         model.NavigationState{Level: model.LevelResources},
		namespace:   "default",
		middleItems: []model.Item{{Name: "my-pod"}},
		yamlView: yamlViewState{
			content:   "spec:\n  replicas: 3",
			collapsed: map[string]bool{},
			cursor:    1,
		},
	}
	assert.Equal(t, "YAML: default/my-pod", m.yamlTitle())
}

// The resource name appears in the breadcrumb even when the cursor is on the
// root line (no attribute path yet) — fixing "open YAML and the resource name
// isn't shown".
func TestYAMLBreadcrumbDrillPathShowsNameWithoutPath(t *testing.T) {
	m := Model{
		mode:        modeYAML,
		nav:         model.NavigationState{Level: model.LevelResources},
		namespace:   "default",
		middleItems: []model.Item{{Name: "my-pod", Kind: "Pod"}},
		yamlView: yamlViewState{
			content:   "spec:\n  replicas: 3",
			collapsed: map[string]bool{},
			cursor:    0, // "spec:" resolves to ["spec"]
		},
	}
	assert.Equal(t, []string{"my-pod", "spec"}, m.explorerDrillPath())
}
