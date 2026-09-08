package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestExecuteBuiltinCommand_ExportKYAML(t *testing.T) {
	t.Run("dispatches_and_shows_fetching_status_for_selection", func(t *testing.T) {
		m := basePush80Model()
		m.toggleSelection(m.middleItems[0])
		m.toggleSelection(m.middleItems[1])

		result, cmd := m.executeBuiltinCommand("export kyaml")
		rm := result.(Model)

		assert.Equal(t, "Fetching 2 manifests...", rm.statusMessage)
		assert.NotNil(t, cmd)
	})

	t.Run("is_case_insensitive", func(t *testing.T) {
		m := basePush80Model()
		result, cmd := m.executeBuiltinCommand("export KYAML")
		rm := result.(Model)

		assert.Empty(t, rm.statusMessage, "a known format never reports an unknown-format error")
		assert.NotNil(t, cmd)
	})
}

func TestWrapYAMLCmdAsKYAML(t *testing.T) {
	t.Run("single_doc_becomes_kyaml", func(t *testing.T) {
		inner := func() tea.Msg {
			return yamlClipboardMsg{
				content: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: foo\n",
				count:   1,
			}
		}

		out := wrapYAMLCmdAsKYAML(inner)().(yamlClipboardMsg)
		require.NoError(t, out.err)
		assert.Equal(t, 1, out.count)
		assert.Equal(t, "kyaml", out.format)
		assert.Contains(t, out.content, `apiVersion: "v1",`)
		assert.Contains(t, out.content, "metadata: {")
	})

	t.Run("multi_doc_keeps_every_document", func(t *testing.T) {
		inner := func() tea.Msg {
			return yamlClipboardMsg{
				content: "kind: Pod\nmetadata:\n  name: a\n" +
					"\n---\n" +
					"kind: Pod\nmetadata:\n  name: b\n",
				count: 2,
			}
		}

		out := wrapYAMLCmdAsKYAML(inner)().(yamlClipboardMsg)
		require.NoError(t, out.err)
		assert.Equal(t, 2, out.count)
		assert.Equal(t, "kyaml", out.format)

		docs := strings.Split(strings.TrimPrefix(out.content, "---\n"), "---\n")
		require.Len(t, docs, 2)
		for i, want := range []string{"a", "b"} {
			var obj map[string]any
			require.NoError(t, yaml.Unmarshal([]byte(docs[i]), &obj))
			assert.Equal(t, want, obj["metadata"].(map[string]any)["name"])
		}
	})

	t.Run("inner_error_passes_through_unchanged", func(t *testing.T) {
		inner := func() tea.Msg { return yamlClipboardMsg{err: assert.AnError} }

		out := wrapYAMLCmdAsKYAML(inner)().(yamlClipboardMsg)
		assert.ErrorIs(t, out.err, assert.AnError)
		assert.Empty(t, out.content)
	})

	t.Run("non_yaml_message_passes_through_unchanged", func(t *testing.T) {
		marker := struct{ note string }{note: "not-a-yaml-msg"}
		inner := func() tea.Msg { return marker }

		assert.Equal(t, marker, wrapYAMLCmdAsKYAML(inner)())
	})

	t.Run("malformed_yaml_surfaces_as_error_envelope", func(t *testing.T) {
		inner := func() tea.Msg {
			return yamlClipboardMsg{content: "a: [1, 2\nb: {", count: 1}
		}

		out := wrapYAMLCmdAsKYAML(inner)().(yamlClipboardMsg)
		require.Error(t, out.err)
		assert.Contains(t, out.err.Error(), "converting YAML to KYAML")
	})
}

func TestCopyFormatStatusParts_KYAML(t *testing.T) {
	label, unit := copyFormatStatusParts("kyaml")
	assert.Equal(t, "KYAML", label)
	assert.Equal(t, "manifests", unit)
}

func TestCommandBarCompletions_ExportOffersKYAML(t *testing.T) {
	m := basePush80Model()
	m.commandBarInput.Value = "export "

	suggestions := m.generateCommandBarSuggestions()
	got := make([]string, 0, len(suggestions))
	for _, s := range suggestions {
		got = append(got, s.Text)
	}
	assert.Contains(t, got, "kyaml")
	assert.Contains(t, got, "yaml")
	assert.Contains(t, got, "json")
}

func TestKubectlOutputFormats_IncludeKYAML(t *testing.T) {
	assert.Contains(t, outputFormatsComplete(), "kyaml",
		"kubectl 1.34 and later accept -o kyaml")
}
