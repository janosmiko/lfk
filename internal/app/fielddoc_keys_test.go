package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func TestFieldDocKeybindingDefault(t *testing.T) {
	assert.Equal(t, "ctrl+k", ui.DefaultKeybindings().FieldDoc)
}

// ctrl+k has to stay clear of every other default, or the viewer that owns the
// other binding never sees the footnote key.
func TestFieldDocKeybindingDoesNotCollide(t *testing.T) {
	kb := ui.DefaultKeybindings()

	assert.NotEqual(t, kb.FieldDoc, kb.PreviewUp, "K stays with preview scroll")
	assert.NotEqual(t, kb.FieldDoc, kb.PreviewDown)
	assert.NotEqual(t, kb.FieldDoc, kb.APIExplorer)
	assert.NotEqual(t, kb.FieldDoc, kb.ObjectExplorer)
	assert.NotEqual(t, kb.FieldDoc, kb.WhichKeyLeader)
}

func yamlFieldDocModel(t *testing.T) Model {
	t.Helper()
	m := basePush80Model()
	m.mode = modeYAML
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true,
	}
	m.fieldDoc.cache = newFieldDocCache()
	m.yamlView.content = "spec:\n  dnsPolicy: ClusterFirst\n  restartPolicy: Always\n"
	m.yamlView.cursor = 1
	return m
}

func TestYAMLViewerCtrlKOpensFieldDoc(t *testing.T) {
	m := yamlFieldDocModel(t)
	require.False(t, m.fieldDoc.on)

	mdl, _ := m.handleYAMLKey(keyMsg("ctrl+k"))
	got := mdl.(Model)

	assert.True(t, got.fieldDoc.on, "ctrl+k must open the footnote pane")
}

func TestYAMLViewerCtrlKTogglesOff(t *testing.T) {
	m := yamlFieldDocModel(t)
	m.fieldDoc.on = true
	m.fieldDoc.entry = fieldDocEntry{desc: "text"}

	mdl, _ := m.handleYAMLKey(keyMsg("ctrl+k"))
	got := mdl.(Model)

	assert.False(t, got.fieldDoc.on)
	assert.Empty(t, got.fieldDoc.entry.desc)
}

// With the pane open, walking the document has to re-target it, or the footnote
// keeps describing the field the user already left.
func TestYAMLViewerCursorMoveRetargetsFieldDoc(t *testing.T) {
	m := yamlFieldDocModel(t)
	m.fieldDoc.on = true
	m.fieldDoc.key = fieldDocKey{
		context: m.effectiveContext(), apiVersion: "v1", resource: "pods", path: "spec.dnsPolicy",
	}
	m.fieldDoc.entry = fieldDocEntry{desc: "dns text"}

	mdl, _ := m.handleYAMLKey(keyMsg("j"))
	got := mdl.(Model)

	require.True(t, got.fieldDoc.on)
	assert.Equal(t, "spec.restartPolicy", got.fieldDoc.key.path,
		"the pane must follow the cursor to the next field")
}

// A closed pane must stay closed and must not fetch when the cursor moves.
func TestYAMLViewerCursorMoveWithPaneClosedDoesNothing(t *testing.T) {
	m := yamlFieldDocModel(t)

	mdl, _ := m.handleYAMLKey(keyMsg("j"))
	got := mdl.(Model)

	assert.False(t, got.fieldDoc.on)
	assert.Empty(t, got.fieldDoc.key.path)
}

func objectExplorerFieldDocModel(t *testing.T) Model {
	t.Helper()
	m := basePush80Model()
	m.mode = modeObjectExplorer
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true,
	}
	m.fieldDoc.cache = newFieldDocCache()
	return m
}

func TestObjectExplorerCtrlKOpensFieldDoc(t *testing.T) {
	m := objectExplorerFieldDocModel(t)

	mdl, _ := m.handleObjectExplorerKey(keyMsg("ctrl+k"))
	got := mdl.(Model)

	assert.True(t, got.fieldDoc.on, "ctrl+k must open the footnote pane in the Object Explorer")
}

// The whole reason ctrl+k was chosen over K: the Object Explorer keeps J and K
// on preview scrolling.
func TestObjectExplorerKStillScrollsPreview(t *testing.T) {
	m := objectExplorerFieldDocModel(t)
	m.objectExplorerView.previewScroll = 3

	mdl, _ := m.handleObjectExplorerKey(keyMsg("K"))
	got := mdl.(Model)

	assert.False(t, got.fieldDoc.on, "K must not open the footnote pane")
	// The empty test model clamps the scroll to 0; what matters is that K
	// reached the preview handler at all instead of the footnote toggle.
	assert.NotEqual(t, 3, got.objectExplorerView.previewScroll, "K must still reach preview scrolling")
}
