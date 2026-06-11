package model

// ObjectTreeRow is one row of the Object Explorer's expanded tree view: a node
// of the subtree flattened in pre-order, with its path relative to the subtree
// root and its nesting depth (0 for direct children of the root).
type ObjectTreeRow struct {
	Field ObjectField
	Segs  []string // path from the subtree root to this node
	Depth int
}

// DefaultObjectTreeRowLimit caps the flattened tree so a pathological object
// (e.g. huge managedFields) cannot produce an unbounded row list.
const DefaultObjectTreeRowLimit = 10000

// ObjectTreeRowsAt flattens the subtree at segs within root into pre-order
// rows, in the same per-level order as ObjectFieldsAt (sorted map keys, array
// indices in order). Scalars and missing paths return nil. limit <= 0 uses
// DefaultObjectTreeRowLimit.
func ObjectTreeRowsAt(root any, segs []string, limit int) []ObjectTreeRow {
	if limit <= 0 {
		limit = DefaultObjectTreeRowLimit
	}
	node, ok := ResolveObjectPath(root, segs)
	if !ok {
		return nil
	}
	var out []ObjectTreeRow
	var walk func(v any, rel []string, depth int)
	walk = func(v any, rel []string, depth int) {
		for _, f := range ObjectFieldsAt(v, nil) {
			if len(out) >= limit {
				return
			}
			childSegs := append(append([]string{}, rel...), f.Key)
			out = append(out, ObjectTreeRow{Field: f, Segs: childSegs, Depth: depth})
			if f.HasChildren {
				child, ok := ResolveObjectPath(v, []string{f.Key})
				if ok {
					walk(child, childSegs, depth+1)
				}
			}
		}
	}
	walk(node, nil, 0)
	return out
}
