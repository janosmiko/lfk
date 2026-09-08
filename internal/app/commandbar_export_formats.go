package app

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/k8s"
)

// wrapYAMLCmdAsJSON converts the YAML payload of an inner yamlClipboardMsg
// into JSON. A single-document payload becomes a JSON object. A multi-document
// payload (separated by `\n---\n` per copyYAMLToClipboard's joiner) becomes a
// JSON array. The bulk-fetch wiring, status messages, and error envelope are
// reused unchanged.
func wrapYAMLCmdAsJSON(cmd tea.Cmd) tea.Cmd {
	return func() tea.Msg {
		msg := cmd()
		yc, ok := msg.(yamlClipboardMsg)
		if !ok || yc.err != nil {
			return msg
		}
		if yc.count <= 1 {
			jsonBytes, err := yaml.YAMLToJSON([]byte(yc.content))
			if err != nil {
				yc.err = fmt.Errorf("converting YAML to JSON: %w", err)
				return yc
			}
			yc.content = string(jsonBytes) + "\n"
			yc.format = "json"
			return yc
		}
		docs := strings.Split(strings.TrimRight(yc.content, "\n"), "\n---\n")
		objects := make([]json.RawMessage, 0, len(docs))
		for _, doc := range docs {
			jsonBytes, err := yaml.YAMLToJSON([]byte(doc))
			if err != nil {
				yc.err = fmt.Errorf("converting YAML to JSON: %w", err)
				return yc
			}
			objects = append(objects, jsonBytes)
		}
		arrayBytes, err := json.Marshal(objects)
		if err != nil {
			yc.err = fmt.Errorf("marshaling JSON array: %w", err)
			return yc
		}
		yc.content = string(arrayBytes) + "\n"
		yc.format = "json"
		return yc
	}
}

// wrapYAMLCmdAsKYAML converts the YAML payload of an inner yamlClipboardMsg
// into KYAML. Multi-document payloads stay multi-document, because KYAML is
// still YAML and needs no array wrapper the way JSON does.
func wrapYAMLCmdAsKYAML(cmd tea.Cmd) tea.Cmd {
	return func() tea.Msg {
		msg := cmd()
		yc, ok := msg.(yamlClipboardMsg)
		if !ok || yc.err != nil {
			return msg
		}
		converted, err := k8s.ToKYAML(yc.content)
		if err != nil {
			yc.err = err
			return yc
		}
		yc.content = converted
		yc.format = "kyaml"
		return yc
	}
}
