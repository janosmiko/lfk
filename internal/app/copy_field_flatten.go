package app

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/model"
)

// maxCopyFieldEntries caps the flattened field list so a pathological
// object cannot produce an unbounded row list (mirrors
// model.DefaultObjectTreeRowLimit).
const maxCopyFieldEntries = 10000

// copyFieldSeg is one segment of a field path. Map segments carry the
// key; array segments carry "[i]" plus — when the element is a map
// with a unique discriminator (name/id/key/type/step, per
// model.ObjectElementLabel) — the selector that identifies the element
// semantically, so extraction can resolve it per manifest instead of
// by position.
type copyFieldSeg struct {
	key    string // map key, or "[i]" for array elements
	selKey string // discriminator key ("type", "name", ...) when uniquely labeled
	selVal string // discriminator value ("ExternalIP", ...) when uniquely labeled
}

// copyFieldEntry is one selectable row of the ctrl+y picker: either a
// visible table column (column != "") or a leaf field of the fetched
// manifest, with its display path (array elements labeled
// semantically: status.addresses[ExternalIP]) and current value.
type copyFieldEntry struct {
	column  string // table column key — set only for columns-mode rows
	path    []copyFieldSeg
	display string
	value   string
}

// flattenCopyFields lists every leaf field of a parsed manifest in
// pre-order (sorted map keys, array indices in order). Non-leaf nodes
// are omitted — the picker copies values, not subtrees — and the
// metadata.managedFields block is dropped as noise. Returns nil for a
// scalar or nil root.
func flattenCopyFields(root any) []copyFieldEntry {
	if _, ok := root.(map[string]any); !ok {
		return nil
	}
	var out []copyFieldEntry
	var walk func(v any, path []copyFieldSeg)
	walk = func(v any, path []copyFieldSeg) {
		if len(out) >= maxCopyFieldEntries {
			return
		}
		switch t := v.(type) {
		case map[string]any:
			if len(t) == 0 {
				out = append(out, newCopyFieldEntry(path, v))
				return
			}
			keys := make([]string, 0, len(t))
			for k := range t {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if k == "managedFields" && len(path) == 1 && path[0].key == "metadata" {
					continue
				}
				walk(t[k], append(path, copyFieldSeg{key: k}))
			}
		case []any:
			if len(t) == 0 {
				out = append(out, newCopyFieldEntry(path, v))
				return
			}
			labels := uniqueElementSelectors(t)
			for i, e := range t {
				seg := copyFieldSeg{key: "[" + strconv.Itoa(i) + "]"}
				if sel, ok := labels[i]; ok {
					seg.selKey, seg.selVal = sel.key, sel.val
				}
				walk(e, append(path, seg))
			}
		default:
			out = append(out, newCopyFieldEntry(path, v))
		}
	}
	walk(root, nil)
	return out
}

// newCopyFieldEntry builds an entry for the leaf at path, copying the
// shared path slice (walk reuses its backing array between siblings).
// The whole display path is sanitized — map keys and selector labels
// both come from the manifest and could carry control bytes.
func newCopyFieldEntry(path []copyFieldSeg, v any) copyFieldEntry {
	p := make([]copyFieldSeg, len(path))
	copy(p, path)
	return copyFieldEntry{
		path:    p,
		display: copyFieldDisplayValue(formatCopyFieldPath(p)),
		value:   copyFieldDisplayValue(copyFieldValueString(v)),
	}
}

type elementSelector struct{ key, val string }

// uniqueElementSelectors returns, per array index, the discriminator
// selector labeling that element — only for labels that are unique
// within the array. Ambiguous labels (two InternalIP addresses) keep
// their numeric index so a selection can never silently pick the
// wrong element.
func uniqueElementSelectors(arr []any) map[int]elementSelector {
	counts := make(map[string]int, len(arr))
	sels := make(map[int]elementSelector, len(arr))
	for i, e := range arr {
		k, v := model.ObjectElementLabel(e)
		if v == "" {
			continue
		}
		counts[k+"\x00"+v]++
		sels[i] = elementSelector{key: k, val: v}
	}
	for i, s := range sels {
		if counts[s.key+"\x00"+s.val] != 1 {
			delete(sels, i)
		}
	}
	return sels
}

// formatCopyFieldPath renders a path as a readable dotted string,
// labeling selector-bearing array segments semantically:
// status.addresses[ExternalIP].address. Follows the
// model.FormatObjectPath joining convention (no dot before "[").
// Sanitization of the whole path happens in newCopyFieldEntry.
func formatCopyFieldPath(path []copyFieldSeg) string {
	var b strings.Builder
	for _, s := range path {
		seg := s.key
		if s.selVal != "" {
			seg = "[" + s.selVal + "]"
		}
		if strings.HasPrefix(seg, "[") {
			b.WriteString(seg)
			continue
		}
		if b.Len() > 0 {
			b.WriteString(".")
		}
		b.WriteString(seg)
	}
	return b.String()
}

// resolveCopyFieldPath walks doc along path. Selector-bearing array
// segments resolve by discriminator match when exactly one element in
// this doc carries the label — so the same semantic field is found
// even when array order differs between manifests — falling back to
// the numeric index otherwise.
func resolveCopyFieldPath(doc any, path []copyFieldSeg) (any, bool) {
	cur := doc
	for _, seg := range path {
		switch t := cur.(type) {
		case map[string]any:
			v, ok := t[seg.key]
			if !ok {
				return nil, false
			}
			cur = v
		case []any:
			idx, ok := resolveArrayIndex(t, seg)
			if !ok {
				return nil, false
			}
			cur = t[idx]
		default:
			return nil, false
		}
	}
	return cur, true
}

// resolveArrayIndex picks the element a path segment refers to within
// arr: unique selector match first, numeric index fallback.
func resolveArrayIndex(arr []any, seg copyFieldSeg) (int, bool) {
	if seg.selKey != "" {
		match, n := -1, 0
		for i, e := range arr {
			if m, ok := e.(map[string]any); ok && copyFieldValueString(m[seg.selKey]) == seg.selVal {
				match, n = i, n+1
			}
		}
		if n == 1 {
			return match, true
		}
	}
	if !strings.HasPrefix(seg.key, "[") || !strings.HasSuffix(seg.key, "]") {
		return 0, false
	}
	idx, err := strconv.Atoi(seg.key[1 : len(seg.key)-1])
	if err != nil || idx < 0 || idx >= len(arr) {
		return 0, false
	}
	return idx, true
}

// copyFieldValueString renders a leaf value for the clipboard: strings
// verbatim, everything else (numbers, bools, empty containers) via
// YAML scalar notation so what lands on the clipboard matches what the
// manifest shows.
func copyFieldValueString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return "null"
	default:
		b, err := yaml.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return strings.TrimRight(string(b), "\n")
	}
}

// copyFieldDisplayValue flattens a value for the picker's one-line
// description column: control bytes (newlines in certs, tabs, ANSI
// escapes in hostile manifest data) become spaces so they cannot break
// the overlay layout or escape-inject the terminal. Only the display
// is sanitized — the clipboard payload keeps the raw value.
func copyFieldDisplayValue(s string) string {
	clean := func(r rune) bool { return r >= 0x20 && r != 0x7f }
	if strings.IndexFunc(s, func(r rune) bool { return !clean(r) }) == -1 {
		return s
	}
	return strings.Map(func(r rune) rune {
		if clean(r) {
			return r
		}
		return ' '
	}, s)
}

// buildCopyFieldPayload extracts the field at path from every doc and
// newline-joins the values in doc order. Docs missing the path are
// skipped (counted in missing) so one divergent manifest doesn't turn
// a bulk copy into an error.
func buildCopyFieldPayload(docs []any, path []copyFieldSeg) (payload string, found, missing int) {
	values := make([]string, 0, len(docs))
	for _, doc := range docs {
		v, ok := resolveCopyFieldPath(doc, path)
		if !ok {
			missing++
			continue
		}
		values = append(values, copyFieldValueString(v))
	}
	return strings.Join(values, "\n"), len(values), missing
}

// parseManifestDocs splits a multi-document YAML payload (joined with
// "\n---\n" by the bulk fetch paths) and parses each document.
// Unparsable documents are dropped — the picker degrades to the docs
// that did parse, mirroring the bulk fetch's partial-failure behavior.
// maxDocs > 0 stops parsing after that many documents; the fetch never
// legitimately returns more docs than were requested, so the cap
// bounds the work a pathological payload full of "---" lines can cause.
func parseManifestDocs(content string, maxDocs int) []any {
	parts := strings.Split(content, "\n---\n")
	docs := make([]any, 0, len(parts))
	for _, p := range parts {
		if maxDocs > 0 && len(docs) >= maxDocs {
			break
		}
		if strings.TrimSpace(p) == "" {
			continue
		}
		var doc any
		if err := yaml.Unmarshal([]byte(p), &doc); err != nil {
			continue
		}
		if doc == nil {
			continue
		}
		docs = append(docs, doc)
	}
	return docs
}
