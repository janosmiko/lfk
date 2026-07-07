package model

import (
	"sort"
	"strconv"
	"strings"
)

// ObjectField is one navigable entry at the current level of a resource-object
// tree browser: a map key or an array index, with its inferred type, a short
// value/summary preview, and whether it can be drilled into.
type ObjectField struct {
	Key         string // map key, or "[i]" for an array element
	Type        string // "<Object>", "<[]Object>", "<string>", etc.
	Preview     string // scalar value, or a summary for objects/arrays
	HasChildren bool   // true for non-empty maps and arrays
}

// objectNameKeys are tried, in order, to label an array element (a map) by a
// human-friendly field instead of just its index.
var objectNameKeys = []string{"name", "id", "key", "type", "step"}

// ObjectFieldsAt returns the navigable fields of the value at segs within root:
// sorted keys for a map, or "[i]" entries for an array. Scalars and leaves
// (including empty containers) return nil. segs uses plain map keys and "[i]"
// for array indices.
func ObjectFieldsAt(root any, segs []string) []ObjectField {
	v, ok := ResolveObjectPath(root, segs)
	if !ok {
		return nil
	}
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make([]ObjectField, 0, len(keys))
		for _, k := range keys {
			out = append(out, objectFieldFor(k, t[k]))
		}
		return out
	case []any:
		out := make([]ObjectField, 0, len(t))
		for i, e := range t {
			f := objectFieldFor("["+strconv.Itoa(i)+"]", e)
			// Prefer a friendly element label over a bare index preview.
			if name := elementName(e); name != "" {
				f.Preview = name
			}
			out = append(out, f)
		}
		return out
	default:
		return nil
	}
}

// objectFieldFor builds an ObjectField for a single key/value.
func objectFieldFor(key string, v any) ObjectField {
	return ObjectField{
		Key:         key,
		Type:        ObjectFieldType(v),
		Preview:     ObjectValuePreview(v),
		HasChildren: ObjectFieldHasChildren(v),
	}
}

// ObjectElementLabel returns the discriminator key/value labeling an
// array element that is a map — the first of objectNameKeys resolving
// to a non-empty scalar (e.g. "type"/"ExternalIP" for a node address).
// Both returns are empty when no discriminator applies.
func ObjectElementLabel(v any) (key, val string) {
	m, ok := v.(map[string]any)
	if !ok {
		return "", ""
	}
	for _, k := range objectNameKeys {
		if s := scalarString(m[k]); s != "" {
			return k, s
		}
	}
	return "", ""
}

// elementName returns a friendly label for an array element that is a map,
// using the first of objectNameKeys that resolves to a non-empty scalar.
func elementName(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	for _, k := range objectNameKeys {
		if s := scalarString(m[k]); s != "" {
			return s
		}
	}
	return ""
}

// ObjectFieldHasChildren reports whether v is a non-empty map or array (i.e.
// can be drilled into).
func ObjectFieldHasChildren(v any) bool {
	switch t := v.(type) {
	case map[string]any:
		return len(t) > 0
	case []any:
		return len(t) > 0
	default:
		return false
	}
}

// ObjectFieldType classifies v for the TYPE column, mirroring kubectl-explain
// notation (<Object>, <[]Object>, <string>, <integer>, <number>, <boolean>).
func ObjectFieldType(v any) string {
	switch t := v.(type) {
	case map[string]any:
		return "<Object>"
	case []any:
		if len(t) == 0 {
			return "<[]>"
		}
		return "<[]" + bareType(t[0]) + ">"
	default:
		return "<" + bareType(v) + ">"
	}
}

// bareType returns the type label without angle brackets.
func bareType(v any) string {
	switch v.(type) {
	case map[string]any:
		return "Object"
	case []any:
		return "[]"
	case bool:
		return "boolean"
	case int, int32, int64, uint, uint32, uint64:
		return "integer"
	case float32, float64:
		return "number"
	case nil:
		return "null"
	default:
		return "string"
	}
}

// ObjectValuePreview renders a short preview for the description column: the
// scalar value, or a count summary for objects and arrays.
func ObjectValuePreview(v any) string {
	switch t := v.(type) {
	case map[string]any:
		return "object (" + strconv.Itoa(len(t)) + " fields)"
	case []any:
		return "array (" + strconv.Itoa(len(t)) + " items)"
	default:
		return scalarString(v)
	}
}

// scalarString stringifies a scalar value; non-scalars and nil yield "".
func scalarString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case int:
		return strconv.Itoa(t)
	case int32:
		return strconv.FormatInt(int64(t), 10)
	case int64:
		return strconv.FormatInt(t, 10)
	case uint:
		return strconv.FormatUint(uint64(t), 10)
	case uint32:
		return strconv.FormatUint(uint64(t), 10)
	case uint64:
		return strconv.FormatUint(t, 10)
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return ""
	}
}

// ResolveObjectPath walks root along segs and returns the value at the end.
// Plain segments index into a map; "[i]" segments index into an array. Returns
// ok=false if any segment is missing or out of range.
func ResolveObjectPath(root any, segs []string) (any, bool) {
	cur := root
	for _, seg := range segs {
		if idx, isIdx := arrayIndex(seg); isIdx {
			arr, ok := cur.([]any)
			if !ok || idx < 0 || idx >= len(arr) {
				return nil, false
			}
			cur = arr[idx]
			continue
		}
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[seg]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// ObjectMatch is one result of a recursive key search: the full path to a
// matching node and a friendly preview of its value.
type ObjectMatch struct {
	Segs    []string // full path from the root to the matched node
	Preview string   // value/summary preview
}

// FindObjectPaths walks the whole object and returns every map-key node whose
// key (the last path segment) contains query, case-insensitively. Results are
// capped at limit (<=0 means a sane default) and returned in a stable
// pre-order.
func FindObjectPaths(root any, query string, limit int) []ObjectMatch {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	return collectObjectPaths(root, limit, func(key string) bool {
		return strings.Contains(strings.ToLower(key), q)
	})
}

// AllObjectPaths returns every map-key node in the object, in pre-order, capped
// at limit. Used to seed the recursive find overlay before any filter is typed.
func AllObjectPaths(root any, limit int) []ObjectMatch {
	return collectObjectPaths(root, limit, func(string) bool { return true })
}

// collectObjectPaths walks the object in pre-order and collects every map-key
// node whose key satisfies match. Array elements are traversed (so nested keys
// carry "[i]" segments) but are never themselves emitted as rows.
func collectObjectPaths(root any, limit int, match func(key string) bool) []ObjectMatch {
	if limit <= 0 {
		limit = 500
	}
	var out []ObjectMatch
	var walk func(v any, segs []string)
	walk = func(v any, segs []string) {
		if len(out) >= limit {
			return
		}
		switch t := v.(type) {
		case map[string]any:
			for _, f := range ObjectFieldsAt(v, nil) {
				child := append(append([]string{}, segs...), f.Key)
				if match(f.Key) {
					out = append(out, ObjectMatch{Segs: child, Preview: f.Preview})
					if len(out) >= limit {
						return
					}
				}
				walk(t[f.Key], child)
			}
		case []any:
			for i, e := range t {
				child := append(append([]string{}, segs...), "["+strconv.Itoa(i)+"]")
				walk(e, child)
			}
		}
	}
	walk(root, nil)
	return out
}

// arrayIndex parses an "[i]" segment into its integer index.
func arrayIndex(seg string) (int, bool) {
	if !strings.HasPrefix(seg, "[") || !strings.HasSuffix(seg, "]") {
		return 0, false
	}
	n, err := strconv.Atoi(seg[1 : len(seg)-1])
	if err != nil {
		return 0, false
	}
	return n, true
}
