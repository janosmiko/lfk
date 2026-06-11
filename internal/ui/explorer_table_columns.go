package ui

import "slices"

// isBuiltinColumnKey reports whether key is one of the fixed built-in item
// field columns other than the union-only Context column. Context is excluded
// because, when it isn't already requested via hasContext, an extras-defined
// "Context" can still render.
func isBuiltinColumnKey(key string) bool {
	_, ok := builtinColumnsByKey[key]
	return ok && key != "Context"
}

// orderedColumnKeys returns the ordered list of column keys that RenderTable
// should emit. ActiveColumnOrder applies to the middle column and to
// cursor-less right-pane previews alike — preview call sites swap in the
// rendered kind's config via withSessionColumnsForKind (issue #408). "Name"
// is a first-class member of the list (leading by default) so it can be
// reordered or hidden through the same column-toggle machinery as every
// other column; hasName=false omits it.
func orderedColumnKeys(hasName, hasContext, hasNs, hasReady, hasRestarts, hasStatus, hasAge bool, extraCols []extraColumn) []string {
	defaults := make([]string, 0, 7+len(extraCols))
	if hasName {
		defaults = append(defaults, "Name")
	}
	if hasContext {
		defaults = append(defaults, "Context")
	}
	if hasNs {
		defaults = append(defaults, "Namespace")
	}
	if hasReady {
		defaults = append(defaults, "Ready")
	}
	if hasRestarts {
		defaults = append(defaults, "Restarts")
	}
	if hasStatus {
		defaults = append(defaults, "Status")
	}
	for _, ec := range extraCols {
		defaults = append(defaults, ec.key)
	}
	if hasAge {
		defaults = append(defaults, "Age")
	}

	if ActiveColumnOrder == nil {
		return defaults
	}

	visible := make(map[string]bool, len(defaults))
	for _, k := range defaults {
		visible[k] = true
	}

	seen := make(map[string]bool, len(defaults))
	ordered := make([]string, 0, len(defaults))

	// Backward compatibility: saved orders that predate configurable Name omit
	// the "Name" key. Without an explicit entry, Name keeps its historical
	// pinned-first position instead of falling to the tail-append cleanup
	// below. An order that DOES list "Name" (saved by the newer overlay)
	// honours that position like any other column.
	if visible["Name"] && !slices.Contains(ActiveColumnOrder, "Name") {
		ordered = append(ordered, "Name")
		seen["Name"] = true
	}

	for _, k := range ActiveColumnOrder {
		if !visible[k] || seen[k] {
			continue
		}
		ordered = append(ordered, k)
		seen[k] = true
	}
	for _, k := range defaults {
		if !seen[k] {
			ordered = append(ordered, k)
			seen[k] = true
		}
	}
	return ordered
}

// widthForColumnKey returns the precomputed width for a given column key.
// Builtin keys come from the column registry; extras are looked up by key.
func widthForColumnKey(key string, widths builtinColWidths, extraCols []extraColumn) int {
	if col := renderableBuiltin(key, widths); col != nil {
		return col.width(widths)
	}
	for _, ec := range extraCols {
		if ec.key == key {
			return ec.width
		}
	}
	return 0
}

// headerCellForKey returns the pre-styled header cell string for a single
// column key. Builtin keys read from the precomputed headers struct; extras
// build their header on the fly using stored width + label.
func headerCellForKey(key string, widths builtinColWidths, headers builtinColHeaders, extraCols []extraColumn) string {
	if col := renderableBuiltin(key, widths); col != nil {
		return col.header(headers)
	}
	for _, ec := range extraCols {
		if ec.key == key {
			return headerWithIndicator(ColumnHeaderLabel(ec.key), ec.key, ec.width)
		}
	}
	return ""
}
