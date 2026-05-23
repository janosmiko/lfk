package ui

import (
	"fmt"
	"strings"

	"k8s.io/client-go/util/jsonpath"
)

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

// View is a compiled view configuration. Constructed from configView at
// config-load time; consumed by both the column renderer and the sort-default
// seeder.
type View struct {
	Columns    []ResolvedColumn
	SortColumn string
	SortAsc    bool
}

// ResolvedColumn is a ColumnSpec with the JSONPath (if any) pre-compiled.
type ResolvedColumn struct {
	ColumnSpec
	Compiled *jsonpath.JSONPath // non-nil iff IsCustom()
}

// ResourceRef is enough of model.ResourceTypeEntry to resolve a view without
// pulling the model import (which would create a cycle: ui -> model -> ui).
type ResourceRef struct {
	Group    string
	Version  string
	Resource string
	Kind     string
}

// GVRKey returns the canonical lookup key "<group>/<version>/<resource>",
// lowercased. Core group renders with an empty leading segment: "/v1/pods".
func (r ResourceRef) GVRKey() string {
	return strings.ToLower(r.Group + "/" + r.Version + "/" + r.Resource)
}

// KindKey returns the case-insensitive Kind fallback key.
func (r ResourceRef) KindKey() string {
	return strings.ToLower(r.Kind)
}

// configView is the on-disk schema (will be referenced by config_load.go in
// the next task). Exported as ConfigView for cross-package test use; the
// lowercase alias remains internal for clarity at package use sites.
type configView struct {
	Columns    []string `json:"columns" yaml:"columns"`
	SortColumn string   `json:"sort_column" yaml:"sort_column"`
}

// ConfigView is the exported alias for configView, intended for use in
// cross-package tests that need to build a View without going through
// YAML parsing.
type ConfigView = configView

// ConfigViews holds the global, compiled views keyed by GVR or lowercase Kind.
var ConfigViews map[string]*View

// ConfigClusterViews holds per-cluster compiled views: context name -> key -> view.
var ConfigClusterViews map[string]map[string]*View

// ResolveView returns the compiled view for the given resource type and
// cluster context. Resolution order: per-cluster GVR > per-cluster Kind >
// global GVR > global Kind. Returns ok=false when no view is configured.
func ResolveView(rt ResourceRef, context string) (*View, bool) {
	if context != "" && len(ConfigClusterViews) > 0 {
		if perCluster, hit := ConfigClusterViews[context]; hit {
			if v, ok := perCluster[rt.GVRKey()]; ok {
				return v, true
			}
			if rt.Kind != "" {
				if v, ok := perCluster[rt.KindKey()]; ok {
					return v, true
				}
			}
		}
	}
	if len(ConfigViews) == 0 {
		return nil, false
	}
	if v, ok := ConfigViews[rt.GVRKey()]; ok {
		return v, true
	}
	if rt.Kind != "" {
		if v, ok := ConfigViews[rt.KindKey()]; ok {
			return v, true
		}
	}
	return nil, false
}

// BuildView compiles a configView into a ready-to-use View. JSONPath
// compilation errors are surfaced to the caller (config applier) so the
// user gets a warning at startup rather than silent missing columns.
// Exported (instead of lowercase) so cross-package tests in internal/app
// can build a View directly without going through YAML.
func BuildView(cv *configView) (*View, error) {
	if cv == nil {
		return nil, nil
	}
	v := &View{SortAsc: true}
	for _, raw := range cv.Columns {
		spec, ok := ParseColumnSpec(raw)
		if !ok {
			return nil, fmt.Errorf("invalid column spec %q", raw)
		}
		rc := ResolvedColumn{ColumnSpec: spec}
		if spec.IsCustom() {
			jp, err := CompileJSONPath(spec.JSONPath)
			if err != nil {
				return nil, fmt.Errorf("compile %q: %w", spec.Name, err)
			}
			rc.Compiled = jp
		}
		v.Columns = append(v.Columns, rc)
	}
	if cv.SortColumn != "" {
		col, asc := parseSortSpec(cv.SortColumn)
		v.SortColumn = col
		v.SortAsc = asc
	}
	return v, nil
}

// parseSortSpec splits "COL[:asc|:desc]" into (col, ascending). Direction
// defaults to desc for REV and asc otherwise when omitted. Invalid direction
// tokens fall through to that default.
func parseSortSpec(spec string) (string, bool) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", true
	}
	col, dir, hasDir := strings.Cut(spec, ":")
	col = strings.TrimSpace(col)
	asc := col != "REV"
	if hasDir {
		switch strings.ToLower(strings.TrimSpace(dir)) {
		case "asc":
			asc = true
		case "desc":
			asc = false
		}
	}
	return col, asc
}
