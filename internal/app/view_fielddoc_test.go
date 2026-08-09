package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func fieldDocViewModel() Model {
	m := basePush80Model()
	m.mode = modeYAML
	m.nav.ResourceType = model.ResourceTypeEntry{
		Kind: "Pod", APIGroup: "", APIVersion: "v1", Resource: "pods", Namespaced: true,
	}
	m.fieldDoc.cache = newFieldDocCache()
	m.yamlView.content = "spec:\n  dnsPolicy: ClusterFirst\n  restartPolicy: Always\n"
	return m
}

// The pane takes its lines out of the body, not off the bottom of the terminal.
// If Update and View disagree the cursor scrolls in behind the pane.
func TestFieldDocPaneShrinksYAMLViewport(t *testing.T) {
	m := fieldDocViewModel()
	before := m.yamlViewportLines()

	m.fieldDoc.on = true
	after := m.yamlViewportLines()

	assert.Equal(t, before-ui.FieldDocPaneHeight, after,
		"the viewport must lose exactly the pane's lines")
}

func TestFieldDocPaneShrinksObjectExplorerBody(t *testing.T) {
	m := fieldDocViewModel()
	before := m.objectExplorerBodyHeight()

	m.fieldDoc.on = true
	after := m.objectExplorerBodyHeight()

	assert.Equal(t, before-ui.FieldDocPaneHeight, after)
}

func TestFieldDocPaneHeightZeroWhenClosed(t *testing.T) {
	m := fieldDocViewModel()

	assert.Equal(t, 0, m.fieldDocPaneHeight())
	assert.Empty(t, m.renderFieldDocPane())
}

func TestYAMLViewRendersFieldDocPane(t *testing.T) {
	m := fieldDocViewModel()
	m.fieldDoc.on = true
	m.fieldDoc.key = fieldDocKey{resource: "pods", apiVersion: "v1", path: "spec.dnsPolicy"}
	m.fieldDoc.entry = fieldDocEntry{fieldType: "<string>", desc: "Set DNS policy for the pod."}

	view := stripANSI(m.viewYAML())

	assert.Contains(t, view, "spec.dnsPolicy")
	assert.Contains(t, view, "Set DNS policy for the pod.")
}

func TestYAMLViewOmitsFieldDocPaneWhenClosed(t *testing.T) {
	m := fieldDocViewModel()
	m.fieldDoc.entry = fieldDocEntry{desc: "Set DNS policy for the pod."}

	view := stripANSI(m.viewYAML())

	assert.NotContains(t, view, "Set DNS policy for the pod.")
}

// The whole view must keep fitting the terminal, or the pane pushes the hint
// bar off the bottom of the screen.
func TestYAMLViewWithFieldDocPaneFitsTerminalHeight(t *testing.T) {
	m := fieldDocViewModel()
	m.fieldDoc.on = true
	m.fieldDoc.entry = fieldDocEntry{fieldType: "<string>", desc: strings.Repeat("long ", 200)}

	closed := len(strings.Split(stripANSI(fieldDocViewModel().viewYAML()), "\n"))
	open := len(strings.Split(stripANSI(m.viewYAML()), "\n"))

	require.Positive(t, closed)
	assert.Equal(t, closed, open, "opening the pane must not make the view taller")
}

func TestYAMLViewFieldDocEmptyState(t *testing.T) {
	m := fieldDocViewModel()
	m.fieldDoc.on = true
	m.fieldDoc.key = fieldDocKey{resource: "pods", path: "spec.opaque"}

	view := stripANSI(m.viewYAML())

	assert.Contains(t, view, "No description")
}
