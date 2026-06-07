package app

import (
	"strings"
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

// L964: the YAML viewer title bar shows the attribute path under the cursor.
func TestYamlTitleShowsCursorPath(t *testing.T) {
	m := Model{
		nav:       model.NavigationState{Level: model.LevelResources},
		namespace: "default",
		middleItems: []model.Item{
			{Name: "my-pod"},
		},
		yamlView: yamlViewState{
			content:   "spec:\n  replicas: 3",
			collapsed: map[string]bool{},
			cursor:    1,
		},
	}
	title := m.yamlTitle()
	assert.True(t, strings.HasPrefix(title, "YAML: default/my-pod"),
		"title keeps the resource label, got %q", title)
	assert.Contains(t, title, "spec.replicas",
		"title must show the cursor attribute path, got %q", title)
}

// Locks the title format: resource label, two-space gap, dotted path (no
// leading dot — matching formatObjectPath used by yank/find elsewhere).
func TestYamlTitlePathFormat(t *testing.T) {
	m := Model{
		nav:         model.NavigationState{Level: model.LevelResources},
		namespace:   "default",
		middleItems: []model.Item{{Name: "my-pod"}},
		yamlView: yamlViewState{
			content:   "spec:\n  replicas: 3",
			collapsed: map[string]bool{},
			cursor:    0, // "spec:" — a single root key still yields ["spec"]
		},
	}
	// cursor on "spec:" resolves to the top-level key path.
	assert.Equal(t, "YAML: default/my-pod  spec", m.yamlTitle())
}
