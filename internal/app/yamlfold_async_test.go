package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeLargeYAML synthesizes a realistic "really long CRD" YAML body with
// deeply nested status and spec sections, plus a large managedFields-style
// tree that resembles what kubectl prints for popular CRDs. Used by the
// regression tests for the async YAML processing pipeline.
func makeLargeYAML(sectionCount int) string {
	var b strings.Builder
	b.WriteString("apiVersion: example.com/v1\nkind: LargeCR\nmetadata:\n  name: foo\n  namespace: default\n  managedFields:\n")
	for i := range sectionCount {
		fmt.Fprintf(&b, "  - apiVersion: example.com/v1\n    fieldsType: FieldsV1\n    fieldsV1:\n      f:spec:\n        f:replicas: {}\n        f:template:\n          f:metadata:\n            f:labels:\n              f:app: {}\n              f:version: {}\n            f:annotations:\n              f:key%d: {}\n          f:spec:\n            f:containers:\n              k:{\"name\":\"c%d\"}:\n                .: {}\n                f:image: {}\n                f:env:\n                  k:{\"name\":\"ENV_%d\"}:\n                    .: {}\n                    f:value: {}\n    manager: kubectl\n    operation: Update\n    time: \"2025-01-01T00:00:00Z\"\n", i, i, i)
	}
	b.WriteString("spec:\n  replicas: 3\nstatus:\n  conditions:\n")
	for i := range sectionCount / 4 {
		fmt.Fprintf(&b, "  - type: Condition%d\n    status: \"True\"\n    reason: Ready\n    message: ready\n", i)
	}
	return b.String()
}

// TestBuildYAMLLoadedMsg verifies that buildYAMLLoadedMsg performs the same
// content transformations that used to happen in the main-thread message
// handler (indentYAMLListItems + parseYAMLSections). This guards the async
// move: if anyone adds new work to the handler, this test starts diverging.
func TestBuildYAMLLoadedMsg(t *testing.T) {
	raw := makeLargeYAML(20)

	msg := buildYAMLLoadedMsg(raw, nil)

	require.NoError(t, msg.err)
	expectedContent := indentYAMLListItems(raw)
	assert.Equal(t, expectedContent, msg.content,
		"builder must pre-indent the content exactly like the old handler did")
	expectedSections := parseYAMLSections(expectedContent)
	assert.Equal(t, expectedSections, msg.sections,
		"builder must pre-parse sections exactly like the old handler did")
	assert.NotEmpty(t, msg.sections, "large fixture should parse to at least one section")
}

// TestBuildYAMLLoadedMsgError ensures the error path short-circuits the heavy
// work — we must not accidentally call parseYAMLSections on an empty string.
func TestBuildYAMLLoadedMsgError(t *testing.T) {
	err := errors.New("boom")
	msg := buildYAMLLoadedMsg("apiVersion: v1\n", err)
	assert.Equal(t, err, msg.err)
	assert.Empty(t, msg.content, "error messages must not carry content")
	assert.Nil(t, msg.sections, "error messages must not carry pre-parsed sections")
}

// TestBuildPreviewYAMLLoadedMsg mirrors TestBuildYAMLLoadedMsg for the
// preview variant used by the split/full YAML preview in the right column.
func TestBuildPreviewYAMLLoadedMsg(t *testing.T) {
	raw := makeLargeYAML(10)

	msg := buildPreviewYAMLLoadedMsg(raw, nil, 42)

	require.NoError(t, msg.err)
	assert.Equal(t, uint64(42), msg.gen)
	assert.Equal(t, indentYAMLListItems(raw), msg.content,
		"preview builder must pre-indent the content")
}

func TestBuildPreviewYAMLLoadedMsgError(t *testing.T) {
	err := errors.New("nope")
	msg := buildPreviewYAMLLoadedMsg("kind: Pod\n", err, 7)
	assert.Equal(t, err, msg.err)
	assert.Equal(t, uint64(7), msg.gen,
		"gen must be preserved even on error so the handler can detect stale responses")
	assert.Empty(t, msg.content)
}

// TestUpdateYamlLoadedDoesNotReprocess is the core regression guard for the
// freeze fix: the message handler must NOT call indentYAMLListItems or
// parseYAMLSections. It only assigns pre-computed fields from the message.
//
// We send a large YAML whose content is intentionally NOT pre-processed, so
// the handler's stored yamlContent should be bit-identical to what we sent,
// not the indented version a producer would have made. Any future refactor
// that puts heavy work back on the main thread fails this test loudly.
func TestUpdateYamlLoadedDoesNotReprocess(t *testing.T) {
	raw := makeLargeYAML(500)
	msg := yamlLoadedMsg{content: raw, sections: nil}

	m := Model{mode: modeYAML}
	result, _ := m.updateYamlLoaded(msg)
	rm := result.(Model)

	assert.Equal(t, raw, rm.yamlContent,
		"handler must store the content verbatim — no re-indenting")
	assert.Nil(t, rm.yamlSections,
		"handler must store sections verbatim — no re-parsing")
}

// TestUpdatePreviewYAMLLoadedDoesNotReprocess is the preview counterpart to
// TestUpdateYamlLoadedDoesNotReprocess.
func TestUpdatePreviewYAMLLoadedDoesNotReprocess(t *testing.T) {
	raw := makeLargeYAML(500)
	msg := previewYAMLLoadedMsg{content: raw, gen: 1}

	m := Model{requestGen: 1}
	rm := m.updatePreviewYAMLLoaded(msg)

	assert.Equal(t, raw, rm.previewYAML,
		"handler must store the preview content verbatim — no re-indenting")
}

// BenchmarkBuildYAMLLoadedMsg keeps the async pipeline honest: regressions in
// indentYAMLListItems or parseYAMLSections show up here first. Kept small so
// it runs fast in CI; expand the section count locally to profile bigger
// manifests.
func BenchmarkBuildYAMLLoadedMsg(b *testing.B) {
	content := makeLargeYAML(100)
	b.ReportAllocs()
	for b.Loop() {
		_ = buildYAMLLoadedMsg(content, nil)
	}
}
