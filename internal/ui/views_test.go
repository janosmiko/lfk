package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseColumnSpec(t *testing.T) {
	cases := []struct {
		input     string
		wantName  string
		wantPath  string
		wantFlags ColumnFlags
		wantOK    bool
	}{
		{input: "NAME", wantName: "NAME", wantOK: true},
		{input: "REV", wantName: "REV", wantOK: true},
		{input: "IMAGE:.spec.template.spec.containers[0].image", wantName: "IMAGE", wantPath: ".spec.template.spec.containers[0].image", wantOK: true},
		{input: "REV:.metadata.resourceVersion", wantName: "REV", wantPath: ".metadata.resourceVersion", wantOK: true},
		{input: "AGE:.metadata.creationTimestamp|T", wantName: "AGE", wantPath: ".metadata.creationTimestamp", wantFlags: FlagTimestamp, wantOK: true},
		{input: "CPU:.usage|R", wantName: "CPU", wantPath: ".usage", wantFlags: FlagRightAlign, wantOK: true},
		{input: "NODE:.spec.nodeName|W", wantName: "NODE", wantPath: ".spec.nodeName", wantFlags: FlagWideOnly, wantOK: true},
		{input: "USAGE:.x|R|W", wantName: "USAGE", wantPath: ".x", wantFlags: FlagRightAlign | FlagWideOnly, wantOK: true},
		{input: "MIXEDCASE:.x|r|w", wantName: "MIXEDCASE", wantPath: ".x", wantFlags: FlagRightAlign | FlagWideOnly, wantOK: true},
		{input: "  PADDED  ", wantName: "PADDED", wantOK: true},
		{input: "", wantOK: false},
		{input: ":.path", wantOK: false},       // empty name
		{input: "NAME:|R", wantOK: false},      // empty path after colon
		{input: "NAME:.path|Q", wantOK: false}, // unknown flag
		{input: "NAME:.path|", wantOK: false},  // empty flag
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			spec, ok := ParseColumnSpec(c.input)
			assert.Equal(t, c.wantOK, ok)
			if !ok {
				return
			}
			assert.Equal(t, c.wantName, spec.Name)
			assert.Equal(t, c.wantPath, spec.JSONPath)
			assert.Equal(t, c.wantFlags, spec.Flags)
		})
	}
}

func TestColumnSpec_IsCustom(t *testing.T) {
	custom, _ := ParseColumnSpec("X:.foo")
	builtin, _ := ParseColumnSpec("Name")
	assert.True(t, custom.IsCustom())
	assert.False(t, builtin.IsCustom())
}

func TestResolveView_GVRPrimary(t *testing.T) {
	origG := ConfigViews
	t.Cleanup(func() { ConfigViews = origG })
	gvr := mustView(t, &ConfigView{
		Columns:    []string{"Name", "REV", "IMAGE:.spec.template.spec.containers[0].image"},
		SortColumn: "REV:desc",
	})
	kindOnly := mustView(t, &ConfigView{
		Columns:    []string{"Name", "Age"},
		SortColumn: "Age",
	})
	ConfigViews = map[string]*View{
		"apps/v1/deployments": gvr,
		"deployment":          kindOnly,
	}
	rt := ResourceRef{Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment"}
	v, ok := ResolveView(rt, "")
	assert.True(t, ok)
	assert.Equal(t, "REV", v.SortColumn)
	assert.False(t, v.SortAsc)
	assert.Len(t, v.Columns, 3)
	assert.Equal(t, "IMAGE", v.Columns[2].Name)
	assert.True(t, v.Columns[2].IsCustom())
	assert.NotNil(t, v.Columns[2].Compiled)
}

func TestResolveView_KindFallback(t *testing.T) {
	orig := ConfigViews
	t.Cleanup(func() { ConfigViews = orig })
	ConfigViews = map[string]*View{
		"pod": mustView(t, &ConfigView{Columns: []string{"Name", "Status"}, SortColumn: "Status:asc"}),
	}
	rt := ResourceRef{Group: "", Version: "v1", Resource: "pods", Kind: "Pod"}
	v, ok := ResolveView(rt, "")
	assert.True(t, ok)
	assert.Equal(t, "Status", v.SortColumn)
	assert.True(t, v.SortAsc)
}

func TestResolveView_ClusterOverride(t *testing.T) {
	origG := ConfigViews
	origC := ConfigClusterViews
	t.Cleanup(func() {
		ConfigViews = origG
		ConfigClusterViews = origC
	})
	ConfigViews = map[string]*View{
		"apps/v1/deployments": mustView(t, &ConfigView{SortColumn: "Name"}),
	}
	ConfigClusterViews = map[string]map[string]*View{
		"prod": {"apps/v1/deployments": mustView(t, &ConfigView{SortColumn: "REV:desc"})},
	}
	rt := ResourceRef{Group: "apps", Version: "v1", Resource: "deployments"}
	v, _ := ResolveView(rt, "prod")
	assert.Equal(t, "REV", v.SortColumn)
	v, _ = ResolveView(rt, "dev")
	assert.Equal(t, "Name", v.SortColumn)
}

func TestResolveView_MissingReturnsFalse(t *testing.T) {
	origG := ConfigViews
	t.Cleanup(func() { ConfigViews = origG })
	ConfigViews = nil
	_, ok := ResolveView(ResourceRef{Kind: "Pod"}, "")
	assert.False(t, ok)
}

func TestBuildView_RejectsInvalidColumn(t *testing.T) {
	_, err := BuildView(&ConfigView{Columns: []string{":.bad"}})
	assert.Error(t, err)
}

func TestBuildView_RejectsInvalidJSONPath(t *testing.T) {
	_, err := BuildView(&ConfigView{Columns: []string{"X:.spec.foo[invalid"}})
	assert.Error(t, err)
}

func TestParseSortSpec_Defaults(t *testing.T) {
	cases := []struct {
		spec string
		col  string
		asc  bool
	}{
		{"REV", "REV", false},
		{"REV:asc", "REV", true},
		{"REV:desc", "REV", false},
		{"Age", "Age", true},
		{"Age:desc", "Age", false},
		{"Name:bogus", "Name", true}, // unknown direction falls back to default for col
		{"", "", true},
	}
	for _, c := range cases {
		t.Run(c.spec, func(t *testing.T) {
			col, asc := parseSortSpec(c.spec)
			assert.Equal(t, c.col, col)
			assert.Equal(t, c.asc, asc)
		})
	}
}

func mustView(t *testing.T, cv *ConfigView) *View {
	t.Helper()
	v, err := BuildView(cv)
	if err != nil {
		t.Fatalf("BuildView: %v", err)
	}
	return v
}
