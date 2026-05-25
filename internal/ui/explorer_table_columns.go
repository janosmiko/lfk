package ui

// isBuiltinColumnKey reports whether key is one of the fixed built-in item
// field columns other than the union-only Context column. Context is excluded
// because, when it isn't already requested via hasContext, an extras-defined
// "Context" can still render.
func isBuiltinColumnKey(key string) bool {
	_, ok := builtinColumnsByKey[key]
	return ok && key != "Context"
}

// orderedColumnKeys returns the ordered list of column keys (excluding "Name")
// that RenderTable should emit for a middle-column render.
func orderedColumnKeys(hasContext, hasNs, hasReady, hasRestarts, hasStatus, hasAge bool, extraCols []extraColumn) []string {
	defaults := make([]string, 0, 6+len(extraCols))
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

	if ActiveMiddleScroll < 0 || ActiveColumnOrder == nil {
		return defaults
	}

	visible := make(map[string]bool, len(defaults))
	for _, k := range defaults {
		visible[k] = true
	}

	seen := make(map[string]bool, len(defaults))
	ordered := make([]string, 0, len(defaults))

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
