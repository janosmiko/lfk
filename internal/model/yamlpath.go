package model

import (
	"strconv"
	"strings"
)

// yamlLinePaths returns, for each physical line of the displayed YAML, the
// object path at that line (nil for blank/comment/structural lines). The input
// must be in the viewer's displayed form, where list items are indented under
// their parent key ("  - name: x"). Map keys and array indices ("[i]") form the
// path; an inline "- key: val" contributes both the array index and the key.
func yamlLinePaths(content string) [][]string {
	lines := strings.Split(content, "\n")
	out := make([][]string, len(lines))
	segByIndent := map[int]string{}
	arrCount := map[int]int{}

	clearDeeper := func(indent int) {
		for k := range segByIndent {
			if k > indent {
				delete(segByIndent, k)
			}
		}
		for k := range arrCount {
			if k > indent {
				delete(arrCount, k)
			}
		}
	}

	for i, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || trimmed == "---" {
			continue
		}
		indent := yamlIndent(raw)
		clearDeeper(indent)

		if trimmed == "-" || strings.HasPrefix(trimmed, "- ") {
			idx := arrCount[indent]
			arrCount[indent] = idx + 1
			segByIndent[indent] = "[" + strconv.Itoa(idx) + "]"
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			if key, ok := yamlKeyName(rest); ok {
				clearDeeper(indent + 2)
				segByIndent[indent+2] = key
			}
		} else if key, ok := yamlKeyName(trimmed); ok {
			segByIndent[indent] = key
		} else {
			continue
		}

		out[i] = pathFromSegments(segByIndent)
	}
	return out
}

// pathFromSegments collects the contiguous even-indent segments starting at 0.
func pathFromSegments(segByIndent map[int]string) []string {
	var path []string
	for lvl := 0; ; lvl += 2 {
		seg, ok := segByIndent[lvl]
		if !ok {
			break
		}
		path = append(path, seg)
	}
	return path
}

// yamlIndent counts leading spaces.
func yamlIndent(s string) int {
	return len(s) - len(strings.TrimLeft(s, " "))
}

// yamlKeyName extracts the mapping key from "key: value" or "key:". Returns
// ok=false when the text is not a mapping entry (no colon, or empty key).
func yamlKeyName(s string) (string, bool) {
	colon := strings.Index(s, ":")
	if colon <= 0 {
		return "", false
	}
	key := strings.TrimSpace(s[:colon])
	if key == "" || strings.ContainsAny(key, " ") {
		// Keys with spaces are almost certainly a scalar list item, not a map.
		return "", false
	}
	key = strings.Trim(key, `"'`)
	return key, key != ""
}

// PathForYAMLLine returns the object path at the given physical line of the
// displayed YAML. When the line itself carries no path (blank/continuation), it
// returns the nearest preceding line's path. Returns nil when nothing matches.
func PathForYAMLLine(content string, line int) []string {
	paths := yamlLinePaths(content)
	if line < 0 || line >= len(paths) {
		return nil
	}
	for i := line; i >= 0; i-- {
		if paths[i] != nil {
			return paths[i]
		}
	}
	return nil
}

// YAMLLineForPath returns the first physical line of the displayed YAML whose
// path has segs as a prefix (so both a container and its leaves resolve), or -1.
func YAMLLineForPath(content string, segs []string) int {
	if len(segs) == 0 {
		return -1
	}
	paths := yamlLinePaths(content)
	for i, p := range paths {
		if hasPathPrefix(p, segs) {
			return i
		}
	}
	return -1
}

// hasPathPrefix reports whether prefix is a prefix of path.
func hasPathPrefix(path, prefix []string) bool {
	if len(prefix) > len(path) {
		return false
	}
	for i, seg := range prefix {
		if path[i] != seg {
			return false
		}
	}
	return true
}
