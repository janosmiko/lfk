package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestYAMLVisibleLinesCache_ReusesResultForUnchangedInputs(t *testing.T) {
	sections := parseYAMLSections(testYAML)
	collapsed := map[string]bool{"metadata.labels": true}
	c := &yamlVisibleLinesCache{}

	visLines1, mapping1 := c.resolve(testYAML, sections, collapsed)
	visLines2, mapping2 := c.resolve(testYAML, sections, collapsed)

	require.NotEmpty(t, visLines1)
	require.NotEmpty(t, mapping1)
	assert.Same(t, &visLines1[0], &visLines2[0])
	assert.Same(t, &mapping1[0], &mapping2[0])
}

func TestYAMLVisibleLinesCache_InvalidatesOnCollapsedChange(t *testing.T) {
	sections := parseYAMLSections(testYAML)
	collapsed := map[string]bool{}
	c := &yamlVisibleLinesCache{}

	before, _ := c.resolve(testYAML, sections, collapsed)
	collapsed["metadata.labels"] = true
	after, _ := c.resolve(testYAML, sections, collapsed)

	assert.Less(t, len(after), len(before))
}

func TestYAMLVisibleLinesCache_InvalidatesOnContentChange(t *testing.T) {
	sections := parseYAMLSections(testYAML)
	c := &yamlVisibleLinesCache{}

	c.resolve(testYAML, sections, nil)
	visLines, mapping := c.resolve(testYAML+"\nextra: line", sections, nil)

	want, wantMapping := buildVisibleLines(testYAML+"\nextra: line", sections, nil)
	assert.Equal(t, want, visLines)
	assert.Equal(t, wantMapping, mapping)
}

func TestYAMLVisibleLinesCache_NilReceiverIsSafe(t *testing.T) {
	var c *yamlVisibleLinesCache
	sections := parseYAMLSections(testYAML)

	visLines, mapping := c.resolve(testYAML, sections, nil)

	want, wantMapping := buildVisibleLines(testYAML, sections, nil)
	assert.Equal(t, want, visLines)
	assert.Equal(t, wantMapping, mapping)
}

func TestYAMLStepCol_ReusesRenderCachedVisibleLines(t *testing.T) {
	sections := parseYAMLSections(testYAML)
	m := Model{
		yamlView: yamlViewState{
			content:   testYAML,
			sections:  sections,
			collapsed: map[string]bool{},
			cursor:    3, // "  name: nginx"
			visCache:  &yamlVisibleLinesCache{},
		},
	}

	// Simulates viewYAML building the slice for the current render.
	rendered, _ := m.yamlVisibleLines()
	require.NotEmpty(t, rendered)
	wantCol := max(stepCol(rendered[m.yamlView.cursor], 0, 1), yamlFoldPrefixLen)

	gotCol := m.yamlStepCol(1)
	assert.Equal(t, wantCol, gotCol)

	// The motion must have read the cache, not rebuilt it: the slice handed
	// back is still backed by the same array the render produced.
	stillCached, _ := m.yamlVisibleLines()
	assert.Same(t, &rendered[0], &stillCached[0])
}

func TestHandleYAMLKeyH_MovesColumnUsingCachedVisibleLines(t *testing.T) {
	sections := parseYAMLSections(testYAML)
	m := Model{
		yamlView: yamlViewState{
			content:      testYAML,
			sections:     sections,
			collapsed:    map[string]bool{},
			cursor:       3,
			visualCurCol: 6,
			visCache:     &yamlVisibleLinesCache{},
		},
	}
	m.yamlVisibleLines() // warm the cache, as the previous render would have

	mdl, _ := m.handleYAMLKeyH()
	updated := mdl.(Model)

	assert.Equal(t, m.yamlStepColExpected(-1), updated.yamlView.visualCurCol)
}

// yamlStepColExpected recomputes the expected result of a single h-step
// directly from buildVisibleLines, independent of the cache under test.
func (m Model) yamlStepColExpected(n int) int {
	visLines, _ := buildVisibleLines(m.yamlView.content, m.yamlView.sections, m.yamlView.collapsed)
	return max(stepCol(visLines[m.yamlView.cursor], m.yamlView.visualCurCol, n), yamlFoldPrefixLen)
}
