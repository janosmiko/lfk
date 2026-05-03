package ui

// isBuiltinColumnKey reports whether key is one of the five mandatory
// built-in column keys.
func isBuiltinColumnKey(key string) bool {
	switch key {
	case "Namespace", "Ready", "Restarts", "Status", "Age":
		return true
	}
	return false
}

// orderedColumnKeys returns the ordered list of column keys (excluding "Name")
// that RenderTable should emit for a middle-column render.
func orderedColumnKeys(hasNs, hasReady, hasRestarts, hasStatus, hasAge bool, extraCols []extraColumn) []string {
	defaults := make([]string, 0, 5+len(extraCols))
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
func widthForColumnKey(key string, nsW, readyW, restartsW, statusW, ageW int, extraCols []extraColumn) int {
	switch key {
	case "Namespace":
		return nsW
	case "Ready":
		return readyW
	case "Restarts":
		return restartsW
	case "Status":
		return statusW
	case "Age":
		return ageW
	}
	for _, ec := range extraCols {
		if ec.key == key {
			return ec.width
		}
	}
	return 0
}

// headerCellForKey returns the pre-styled header cell string for a single
// column key.
func headerCellForKey(key string, extraCols []extraColumn,
	nsHeader, readyHeader, rsHeader, statusHeader, ageHeader string,
) string {
	switch key {
	case "Namespace":
		return nsHeader
	case "Ready":
		return readyHeader
	case "Restarts":
		return rsHeader
	case "Status":
		return statusHeader
	case "Age":
		return ageHeader
	}
	for _, ec := range extraCols {
		if ec.key == key {
			return headerWithIndicator(columnHeaderLabel(ec.key), ec.key, ec.width)
		}
	}
	return ""
}
