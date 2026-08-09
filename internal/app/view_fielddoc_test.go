package app

import (
	"errors"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
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

func TestSplitFieldDocWidth(t *testing.T) {
	tests := []struct {
		name        string
		total       int
		wantPaneNil bool
	}{
		{name: "narrow terminal gets no pane", total: 80, wantPaneNil: true},
		{name: "tiny terminal gets no pane", total: 40, wantPaneNil: true},
		{name: "roomy terminal gets a pane", total: 120},
		{name: "wide terminal gets a pane", total: 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viewW, paneW := splitFieldDocWidth(tt.total)

			assert.Equal(t, tt.total, viewW+paneW, "the split must account for every column")
			if tt.wantPaneNil {
				assert.Zero(t, paneW)
				return
			}
			assert.GreaterOrEqual(t, paneW, fieldDocMinPaneWidth)
			assert.LessOrEqual(t, paneW, fieldDocMaxPaneWidth)
			assert.GreaterOrEqual(t, viewW, fieldDocMinViewWidth, "the viewer must keep room to read")
		})
	}
}

// The pane takes columns, not rows. The YAML viewer keeps every body line; the
// Object Explorer gives up exactly one row, to the hint bar that moves out from
// under its columns to span both panes.
func TestFieldDocPaneCostsNoYAMLRowsAndOneOERow(t *testing.T) {
	m := fieldDocViewModel()
	yamlBefore, oeBefore := m.yamlViewportLines(), m.objectExplorerBodyHeight()

	m.fieldDoc.on = true

	assert.Equal(t, yamlBefore, m.yamlViewportLines())
	assert.Equal(t, oeBefore-1, m.objectExplorerBodyHeight())
}

func TestFieldDocPaneWidthZeroWhenClosed(t *testing.T) {
	m := fieldDocViewModel()

	assert.Zero(t, m.fieldDocPaneWidth())
	assert.Empty(t, m.renderFieldDocPane(m.height, false))
}

// The joined view must stay exactly as wide and as tall as the terminal, or
// lipgloss pads one side and the layout tears.
func TestYAMLViewWithSchemaPaneKeepsTerminalBox(t *testing.T) {
	closed := fieldDocViewModel()
	m := fieldDocViewModel()
	m.fieldDoc.on = true
	m.fieldDoc.key = fieldDocKey{resource: "pods", apiVersion: "v1", path: "spec.dnsPolicy"}
	m.fieldDoc.entry = fieldDocEntry{fieldType: "<string>", desc: strings.Repeat("word ", 200)}

	open := m.viewYAML()

	assert.Equal(t, m.width, lipgloss.Width(open), "the view must fill the terminal width")
	assert.Equal(t, lipgloss.Height(closed.viewYAML()), lipgloss.Height(open),
		"opening the pane must not change the view height")
}

// lastLine returns the hint bar, which both viewers draw as the bottom row.
func hintBarRow(t *testing.T, view string) string {
	t.Helper()
	rows := strings.Split(stripANSI(view), "\n")
	return rows[len(rows)-1]
}

// The hint bar spans the terminal under both panes. Drawing it inside the
// narrowed viewer column cost it entries instead. It must read the same open
// as closed; how many entries fit at a given width is a separate matter.
func TestYAMLViewHintBarUnchangedByPane(t *testing.T) {
	m := fieldDocViewModel()
	m.fieldDoc.on = true
	m.fieldDoc.key = fieldDocKey{resource: "pods", path: "spec.dnsPolicy"}

	assert.Equal(t, hintBarRow(t, fieldDocViewModel().viewYAML()), hintBarRow(t, m.viewYAML()),
		"the pane must not cost the hint bar any entries")
}

func TestObjectExplorerHintBarUnchangedByPane(t *testing.T) {
	closed := fieldDocViewModel()
	closed.mode = modeObjectExplorer
	m := fieldDocViewModel()
	m.mode = modeObjectExplorer
	m.fieldDoc.on = true
	m.fieldDoc.key = fieldDocKey{resource: "pods", path: "spec.dnsPolicy"}

	assert.Equal(t, hintBarRow(t, closed.viewObjectExplorer()), hintBarRow(t, m.viewObjectExplorer()),
		"the pane must not cost the hint bar any entries")
}

// A terminal wide enough for every entry must show every entry with the pane
// open, which is the case the report came from.
func TestYAMLViewHintBarCompleteOnAWideTerminal(t *testing.T) {
	m := fieldDocViewModel()
	m.width = 300
	m.fieldDoc.on = true
	m.fieldDoc.key = fieldDocKey{resource: "pods", path: "spec.dnsPolicy"}

	hint := hintBarRow(t, m.viewYAML())

	for _, want := range []string{"scroll", "visual select", "object explorer", "explain", "schema", "back"} {
		assert.Contains(t, hint, want, "hint entry %q must survive the pane", want)
	}
}

// The pane is sized like the log viewer's structured preview, so the same
// terminal gives both the same width.
func TestFieldDocPaneMatchesLogPreviewWidth(t *testing.T) {
	for _, total := range []int{100, 120, 160, 200, 300} {
		_, logPaneW := splitLogPreviewWidth(total)
		_, docPaneW := splitFieldDocWidth(total)

		assert.Equal(t, logPaneW, docPaneW, "width %d: the two side panes must agree", total)
	}
}

func TestYAMLViewRendersSchemaPane(t *testing.T) {
	m := fieldDocViewModel()
	m.fieldDoc.on = true
	m.fieldDoc.key = fieldDocKey{resource: "pods", apiVersion: "v1", path: "spec.dnsPolicy"}
	m.fieldDoc.entry = fieldDocEntry{fieldType: "<string>", desc: "Set DNS policy for the pod."}

	view := stripANSI(m.viewYAML())

	assert.Contains(t, view, "SCHEMA")
	assert.Contains(t, view, "spec.dnsPolicy")
	assert.Contains(t, view, "Set DNS policy for the pod.")
}

func TestYAMLViewOmitsSchemaPaneWhenClosed(t *testing.T) {
	m := fieldDocViewModel()
	m.fieldDoc.entry = fieldDocEntry{desc: "Set DNS policy for the pod."}

	view := stripANSI(m.viewYAML())

	assert.NotContains(t, view, "SCHEMA")
	assert.NotContains(t, view, "Set DNS policy for the pod.")
}

func TestYAMLViewSchemaPaneEmptyState(t *testing.T) {
	m := fieldDocViewModel()
	m.fieldDoc.on = true
	m.fieldDoc.key = fieldDocKey{resource: "pods", path: "spec.opaque"}

	view := stripANSI(m.viewYAML())

	assert.Contains(t, view, "No description")
}

// The reported bug: the raw exit status and the KIND/VERSION preamble reached
// the pane instead of the one line that says what went wrong.
func TestYAMLViewSchemaPaneShowsErrorAlone(t *testing.T) {
	m := fieldDocViewModel()
	m.fieldDoc.on = true
	m.fieldDoc.key = fieldDocKey{resource: "pods", path: "metadata.annotations.checksum/config"}
	m.fieldDoc.err = parseExplainError(
		"KIND:       Pod\nVERSION:    v1\n\nerror: field \"checksum/config\" does not exist\n",
		errors.New("exit status 1"),
	).Error()

	view := stripANSI(m.viewYAML())

	assert.Contains(t, view, `field "checksum/config" does not exist`)
	assert.NotContains(t, view, "exit status")
	assert.NotContains(t, view, "VERSION:")
}

func TestObjectExplorerViewKeepsTerminalBox(t *testing.T) {
	m := fieldDocViewModel()
	m.mode = modeObjectExplorer
	closedH := lipgloss.Height(m.viewObjectExplorer())

	m.fieldDoc.on = true
	m.fieldDoc.key = fieldDocKey{resource: "pods", path: "spec.dnsPolicy"}
	m.fieldDoc.entry = fieldDocEntry{fieldType: "<string>", desc: "Set DNS policy."}
	open := m.viewObjectExplorer()

	require.Contains(t, stripANSI(open), "SCHEMA")
	assert.Equal(t, m.width, lipgloss.Width(open))
	assert.Equal(t, closedH, lipgloss.Height(open))
}

// Opening on a terminal that cannot fit both panes has to say so rather than
// toggling a pane the user never sees.
func TestToggleFieldDocRefusesNarrowTerminal(t *testing.T) {
	m := fieldDocViewModel()
	m.width = 70

	mdl, _ := m.toggleFieldDoc([]string{"spec", "dnsPolicy"})
	got := mdl.(Model)

	assert.False(t, got.fieldDoc.on)
	assert.Contains(t, got.statusMessage, "too narrow")
}
