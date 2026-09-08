package k8s

import (
	"fmt"
	"strings"

	"sigs.k8s.io/yaml/kyaml"
)

// ToKYAML converts block YAML to the KYAML subset kubectl emits for `-o kyaml`.
// The encoder reads a whole multi-document stream, so documents are not split here.
func ToKYAML(yamlText string) (string, error) {
	trimmed := trimTrailingDocSeparators(yamlText)
	if trimmed == "" {
		return "", nil
	}

	var b strings.Builder
	b.Grow(len(trimmed) + len(trimmed)/4)
	enc := kyaml.Encoder{}
	if err := enc.FromYAML(strings.NewReader(trimmed), &b); err != nil {
		return "", fmt.Errorf("converting YAML to KYAML: %w", err)
	}
	return b.String(), nil
}

// trimTrailingDocSeparators drops a dangling "---", which the encoder would
// otherwise render as an empty document. Text is returned untouched unless a
// separator is actually removed, so block-scalar chomping stays intact.
func trimTrailingDocSeparators(yamlText string) string {
	lines := strings.Split(yamlText, "\n")
	end := len(lines)
	for {
		content := end
		for content > 0 && strings.TrimSpace(lines[content-1]) == "" {
			content--
		}
		if content == 0 {
			return ""
		}
		if !isBareDocSeparator(lines[content-1]) {
			return strings.Join(lines[:end], "\n")
		}
		end = content - 1
	}
}

// isBareDocSeparator matches a document separator only at column 0. An indented
// "---" is content inside a block scalar.
func isBareDocSeparator(line string) bool {
	return strings.TrimRight(line, " \t\r") == "---"
}
