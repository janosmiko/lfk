package app

import (
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"
)

// ExtractContainerBlocksYAML returns the YAML for the given container
// names extracted from podYAML. Looks under spec.containers,
// spec.initContainers, and spec.ephemeralContainers. Single name →
// a single YAML document. Multiple names → YAML documents joined
// with "\n---\n". Returns an error listing the first missing name
// when any requested name resolves in none of the lists.
func ExtractContainerBlocksYAML(podYAML string, names []string) (string, error) {
	blocks, err := extractContainerBlocks(podYAML, names)
	if err != nil {
		return "", err
	}
	docs := make([]string, 0, len(blocks))
	for _, b := range blocks {
		out, err := yaml.Marshal(b)
		if err != nil {
			return "", fmt.Errorf("marshaling container %q: %w", b["name"], err)
		}
		docs = append(docs, strings.TrimRight(string(out), "\n"))
	}
	return strings.Join(docs, "\n---\n") + "\n", nil
}

// extractContainerBlocks parses podYAML and returns each requested
// container's spec block (a map[string]any) in the requested order.
// Looks across spec.containers, spec.initContainers, and
// spec.ephemeralContainers. Returns an error on the first name that
// resolves in none of the lists.
func extractContainerBlocks(podYAML string, names []string) ([]map[string]any, error) {
	var pod map[string]any
	if err := yaml.Unmarshal([]byte(podYAML), &pod); err != nil {
		return nil, fmt.Errorf("parsing Pod manifest: %w", err)
	}
	spec, _ := pod["spec"].(map[string]any)
	if spec == nil {
		return nil, fmt.Errorf("pod manifest has no spec")
	}
	index := indexContainers(spec)
	out := make([]map[string]any, 0, len(names))
	for _, name := range names {
		block, ok := index[name]
		if !ok {
			meta, _ := pod["metadata"].(map[string]any)
			podName, _ := meta["name"].(string)
			return nil, fmt.Errorf("container %q not found in Pod %q", name, podName)
		}
		out = append(out, block)
	}
	return out, nil
}

// indexContainers walks spec.containers, spec.initContainers, and
// spec.ephemeralContainers and returns a name → block map. First
// occurrence wins; since K8s validation prevents duplicate names
// across these lists this branch is never taken in practice.
func indexContainers(spec map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	collect := func(key string) {
		list, _ := spec[key].([]any)
		for _, c := range list {
			block, ok := c.(map[string]any)
			if !ok {
				continue
			}
			name, _ := block["name"].(string)
			if name == "" {
				continue
			}
			if _, exists := out[name]; exists {
				continue
			}
			out[name] = block
		}
	}
	collect("containers")
	collect("initContainers")
	collect("ephemeralContainers")
	return out
}
