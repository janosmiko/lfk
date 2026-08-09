package app

import (
	"slices"
	"strings"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/ui"
)

// blameLine is the blame entry for one rendered YAML line. rolled marks a
// manager that was derived rather than recorded: either inherited from an
// owned ancestor, or rolled up from a subtree whose lines share one manager.
type blameLine struct {
	manager string
	owner   k8s.FieldOwner
	rolled  bool
}

// linePath is what one YAML line points at in the object.
type linePath struct {
	segs   []k8s.PathSeg
	header bool // a key with no inline value, so it can inherit from its subtree
	body   bool // a line inside a block scalar, which belongs to the key above it
}

// computeYAMLBlame maps each line of the rendered YAML to the field manager
// that last wrote it. The result has one entry per line, so a line index
// addresses it directly, including blank and comment lines.
func computeYAMLBlame(content string, owners *k8s.FieldOwners) []blameLine {
	if owners.Empty() {
		return nil
	}
	lines := strings.Split(content, "\n")
	paths := buildLinePaths(lines)
	out := make([]blameLine, len(lines))
	for i := range lines {
		if paths[i].segs == nil {
			continue
		}
		out[i] = resolveOwner(owners, paths[i].segs)
	}
	rollUpHeaders(lines, paths, out)
	return out
}

// resolveOwner takes the exact owner of a path, or the nearest owned ancestor.
// A field the manifest never named still belongs to whoever wrote the block
// around it. The manager name is cluster-controlled, so it is stripped of
// control characters here, at the one point every blame entry passes.
func resolveOwner(owners *k8s.FieldOwners, segs []k8s.PathSeg) blameLine {
	if o, ok := owners.At(segs); ok {
		return blameLine{manager: ui.SanitizeTerminalText(o.Manager), owner: o}
	}
	for n := len(segs) - 1; n > 0; n-- {
		if o, ok := owners.At(segs[:n]); ok {
			return blameLine{manager: ui.SanitizeTerminalText(o.Manager), owner: o, rolled: true}
		}
	}
	return blameLine{}
}

// rollUpHeaders gives a section header the manager of its subtree when every
// line below it has the same one. Without this there is no owner on exactly
// the lines that organize the document.
func rollUpHeaders(lines []string, paths []linePath, out []blameLine) {
	for i := range slices.Backward(lines) {
		if !paths[i].header || out[i].manager != "" {
			continue
		}
		if child, ok := uniformChildManager(lines, out, i); ok {
			out[i] = blameLine{manager: child.manager, owner: child.owner, rolled: true}
		}
	}
}

// uniformChildManager returns the single owner of everything below a header,
// or reports false when the lines below disagree. The child entry is carried
// whole, so the header can show the same timestamp as the block it covers.
func uniformChildManager(lines []string, out []blameLine, header int) (blameLine, bool) {
	indent := countIndent(lines[header])
	var child blameLine
	for j := header + 1; j < len(lines); j++ {
		trimmed := strings.TrimSpace(lines[j])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if countIndent(lines[j]) <= indent {
			break
		}
		if out[j].manager == "" {
			return blameLine{}, false
		}
		// The whole owner has to match, not just the name: one manager can
		// write two children in two applies, and the header would then show
		// the write time of whichever child came last.
		if child.manager != "" && (child.manager != out[j].manager || child.owner != out[j].owner) {
			return blameLine{}, false
		}
		child = out[j]
	}
	return child, child.manager != ""
}

type pathFrame struct {
	childIndent int
	seg         k8s.PathSeg
}

// buildLinePaths walks the document once and records the object path each line
// points at. List items are identified by their scalar fields rather than by
// index, because that is how managedFields names them.
//
//nolint:gocyclo // one pass over YAML line shapes: the branches are the shapes, not nesting
func buildLinePaths(lines []string) []linePath {
	paths := make([]linePath, len(lines))
	stack := make([]pathFrame, 0, 8)
	blockIndent := -1
	var blockPath []k8s.PathSeg

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		indent := countIndent(line)

		if blockIndent >= 0 {
			if trimmed == "" || indent >= blockIndent {
				paths[i] = linePath{segs: blockPath, body: true}
				continue
			}
			blockIndent, blockPath = -1, nil
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if trimmed == "---" {
			stack = stack[:0]
			continue
		}

		for len(stack) > 0 && stack[len(stack)-1].childIndent > indent {
			stack = stack[:len(stack)-1]
		}

		if strings.HasPrefix(trimmed, "- ") {
			stack = append(stack, pathFrame{
				childIndent: indent + 2,
				seg:         k8s.PathSeg{Item: collectItemFields(lines, i, indent)},
			})
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			indent += 2
		} else if trimmed == "-" {
			continue
		}

		key, afterColon, ok := splitYAMLPair(trimmed)
		if !ok {
			paths[i] = linePath{segs: segsOf(stack)}
			continue
		}
		segs := append(segsOf(stack), k8s.PathSeg{Key: key})
		paths[i] = linePath{segs: segs, header: afterColon == ""}

		switch {
		case afterColon == "":
			stack = append(stack, pathFrame{childIndent: indent + 2, seg: k8s.PathSeg{Key: key}})
		case isBlockIndicator(afterColon):
			blockIndent, blockPath = indent+1, segs
		}
	}
	return paths
}

// collectItemFields reads the scalar fields of one list item so it can be
// matched against a managedFields selector such as k:{"name":"nginx"}.
func collectItemFields(lines []string, start, itemIndent int) map[string]string {
	fields := make(map[string]string, 4)
	if key, value, ok := splitYAMLPair(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[start]), "- "))); ok &&
		value != "" {
		fields[key] = unquoteYAML(value)
	}
	for j := start + 1; j < len(lines); j++ {
		trimmed := strings.TrimSpace(lines[j])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := countIndent(lines[j])
		if indent <= itemIndent || strings.HasPrefix(trimmed, "- ") {
			break
		}
		if indent != itemIndent+2 {
			continue
		}
		key, value, ok := splitYAMLPair(trimmed)
		if ok && value != "" && !isBlockIndicator(value) {
			fields[key] = unquoteYAML(value)
		}
	}
	return fields
}

// isBlockIndicator reports a real block scalar opener. isBlockScalar in
// yamlfold.go also accepts an empty value, because a fold section treats a
// bare header the same way; here the two must stay apart.
func isBlockIndicator(afterColon string) bool {
	return afterColon != "" && isBlockScalar(afterColon)
}

// splitYAMLPair splits "key: value" at the colon that ends the key. A quoted
// key may contain a colon of its own, so the scan skips quoted spans. The
// value may be empty, which marks a section header.
func splitYAMLPair(trimmed string) (key, value string, ok bool) {
	idx := keyColonIndex(trimmed)
	if idx <= 0 {
		return "", "", false
	}
	return unquoteYAML(trimmed[:idx]), strings.TrimSpace(trimmed[idx+1:]), true
}

// keyColonIndex returns the offset of the colon that separates key from value,
// or -1 when the line is not a pair.
func keyColonIndex(trimmed string) int {
	var quote byte
	for i := range len(trimmed) {
		c := trimmed[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == ':':
			return i
		}
	}
	return -1
}

func unquoteYAML(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func segsOf(stack []pathFrame) []k8s.PathSeg {
	segs := make([]k8s.PathSeg, len(stack))
	for i := range stack {
		segs[i] = stack[i].seg
	}
	return segs
}
