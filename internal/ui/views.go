package ui

import "strings"

// ColumnFlags is a bitmask of optional column rendering hints parsed from
// the |X|Y suffix of a column spec.
type ColumnFlags uint8

const (
	FlagRightAlign ColumnFlags = 1 << iota
	FlagTimestamp
	FlagWideOnly
)

// ColumnSpec is a parsed column entry from a views.<key>.columns list. A
// non-empty JSONPath means the column is custom; an empty path means the
// Name refers to a built-in column resolved by the existing renderer.
type ColumnSpec struct {
	Name     string
	JSONPath string
	Flags    ColumnFlags
}

// IsCustom reports whether the column carries a JSONPath expression.
func (c ColumnSpec) IsCustom() bool { return c.JSONPath != "" }

// ParseColumnSpec parses one entry from a views.<key>.columns list. Format:
//
//	NAME[:.jsonpath][|flag]*
//
// Returns ok=false for empty input, empty name, empty path after a colon,
// or an unrecognised/empty flag.
func ParseColumnSpec(s string) (ColumnSpec, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return ColumnSpec{}, false
	}
	parts := strings.Split(s, "|")
	head := strings.TrimSpace(parts[0])
	var spec ColumnSpec
	if name, path, hasPath := strings.Cut(head, ":"); hasPath {
		name = strings.TrimSpace(name)
		path = strings.TrimSpace(path)
		if name == "" || path == "" {
			return ColumnSpec{}, false
		}
		spec.Name = name
		spec.JSONPath = path
	} else {
		if head == "" {
			return ColumnSpec{}, false
		}
		spec.Name = head
	}
	for _, f := range parts[1:] {
		switch strings.ToUpper(strings.TrimSpace(f)) {
		case "R":
			spec.Flags |= FlagRightAlign
		case "T":
			spec.Flags |= FlagTimestamp
		case "W":
			spec.Flags |= FlagWideOnly
		default:
			return ColumnSpec{}, false
		}
	}
	return spec, true
}
