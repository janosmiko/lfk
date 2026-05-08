package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- expandCustomActionTemplate: additional cases ---

func TestExpandCustomActionTemplateMultipleColumns(t *testing.T) {
	actx := actionContext{
		name:      "web",
		namespace: "prod",
		columns: []model.KeyValue{
			{Key: "Node", Value: "worker-1"},
			{Key: "IP", Value: "10.0.0.5"},
		},
	}
	result := expandCustomActionTemplate("curl http://{IP}:8080 on {Node}", actx)
	assert.Equal(t, "curl http://10.0.0.5:8080 on worker-1", result)
}

// --- findCustomAction ---

func TestFindCustomActionNotFound(t *testing.T) {
	_, found := findCustomAction("NonExistentKind", "NonExistentAction")
	assert.False(t, found)
}

// --- copyYAMLToClipboard: nil selected item ---

func TestCopyYAMLToClipboardNilSelected(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level: model.LevelResources,
		},
		middleItems: []model.Item{}, // empty, no selected item
	}
	cmd := m.copyYAMLToClipboard()
	assert.Nil(t, cmd)
}

func TestCopyYAMLToClipboardNilAtOwned(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level: model.LevelOwned,
		},
		middleItems: []model.Item{}, // empty
	}
	cmd := m.copyYAMLToClipboard()
	assert.Nil(t, cmd)
}

func TestCopyYAMLToClipboardNilAtDefault(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level: model.LevelResourceTypes,
		},
	}
	cmd := m.copyYAMLToClipboard()
	assert.Nil(t, cmd)
}

// --- exportResourceToFile: nil cases ---

func TestExportResourceToFileNilSelected(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level: model.LevelResources,
		},
		middleItems: []model.Item{},
	}
	cmd := m.exportResourceToFile()
	assert.Nil(t, cmd)
}

func TestExportResourceToFileNilDefault(t *testing.T) {
	m := Model{
		nav: model.NavigationState{
			Level: model.LevelClusters,
		},
	}
	cmd := m.exportResourceToFile()
	assert.Nil(t, cmd)
}

// --- security view: synthetic kinds have no YAML / no file to export ---

// TestCopyYAMLToClipboardSecurityNoFetch guards against the same warning class
// as TestLoadYAMLLevelOwnedSecurityNoFetch — pressing 'Y' / `:export yaml` on
// a __security_affected_resource__ row at LevelOwned must not dispatch a
// kubectl-shaped fetch.
func TestCopyYAMLToClipboardSecurityNoFetch(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelOwned
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__security_falco__", APIGroup: "_security"}
	m.middleItems = []model.Item{{Name: "pod/affected", Kind: "__security_affected_resource__", Namespace: "default"}}
	m.setCursor(0)
	cmd := m.copyYAMLToClipboard()
	assert.Nil(t, cmd)
}

// TestExportResourceToFileSecurityNoFetch — same regression for save-to-file.
func TestExportResourceToFileSecurityNoFetch(t *testing.T) {
	m := basePush80Model()
	m.nav.Level = model.LevelOwned
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__security_falco__", APIGroup: "_security"}
	m.middleItems = []model.Item{{Name: "pod/affected", Kind: "__security_affected_resource__", Namespace: "default"}}
	m.setCursor(0)
	cmd := m.exportResourceToFile()
	assert.Nil(t, cmd)
}

// --- renderCanIOverlay ---

func TestRenderCanIOverlayEmptySubjectName(t *testing.T) {
	m := baseOverlayModel()
	m.canIGroups = []model.CanIGroup{
		{
			Name: "core",
			Resources: []model.CanIResource{
				{Resource: "pods", Verbs: map[string]bool{"get": true}},
			},
		},
	}
	m.canISubjectName = ""
	bg := "bg\n"
	result := m.renderCanIOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderCanIOverlayWithSubject(t *testing.T) {
	m := baseOverlayModel()
	m.canIGroups = []model.CanIGroup{
		{
			Name: "",
			Resources: []model.CanIResource{
				{Resource: "pods", Verbs: map[string]bool{"get": true}},
			},
		},
	}
	m.canISubjectName = "admin-user"
	bg := "bg\n"
	result := m.renderCanIOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderCanIOverlayAllowedOnly(t *testing.T) {
	m := baseOverlayModel()
	m.canIGroups = []model.CanIGroup{
		{
			Name: "core",
			Resources: []model.CanIResource{
				{Resource: "pods", Verbs: map[string]bool{"get": true}},
				{Resource: "secrets", Verbs: map[string]bool{"get": false}},
			},
		},
	}
	m.canIAllowedOnly = true
	bg := "bg\n"
	result := m.renderCanIOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderCanIOverlaySearchActive(t *testing.T) {
	m := baseOverlayModel()
	m.canIGroups = []model.CanIGroup{
		{
			Name: "core",
			Resources: []model.CanIResource{
				{Resource: "pods", Verbs: map[string]bool{"get": true}},
			},
		},
	}
	m.canISearchActive = true
	m.canISearchInput = TextInput{Value: "pod"}
	bg := "bg\n"
	result := m.renderCanIOverlay(bg)
	assert.NotEmpty(t, result)
}

func TestRenderCanIOverlaySearchQuery(t *testing.T) {
	m := baseOverlayModel()
	m.canIGroups = []model.CanIGroup{
		{
			Name: "core",
			Resources: []model.CanIResource{
				{Resource: "pods", Verbs: map[string]bool{"get": true}},
			},
		},
	}
	m.canISearchActive = false
	m.canISearchQuery = "pods"
	bg := "bg\n"
	result := m.renderCanIOverlay(bg)
	assert.NotEmpty(t, result)
}
