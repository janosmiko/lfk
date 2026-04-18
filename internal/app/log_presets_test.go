package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogPresetsPath(t *testing.T) {
	p, err := logPresetsPath()
	assert.NoError(t, err)
	assert.NotEmpty(t, p)
	assert.Contains(t, p, "lfk")
	assert.Contains(t, p, "log_presets.yaml")
}

func TestPresetRoundTrip(t *testing.T) {
	src := PresetFile{
		Presets: map[string][]LogPreset{
			"Pod": {
				{
					Name:        "noise",
					Default:     false,
					IncludeMode: "any",
					Rules: []PresetRule{
						{Type: "exclude", Pattern: "/healthz", Mode: "substring"},
						{Type: "severity", Floor: "warn"},
					},
				},
			},
		},
	}

	tmp := t.TempDir() + "/log_presets.yaml"
	err := writePresetFile(tmp, src)
	assert.NoError(t, err)

	loaded, err := readPresetFile(tmp)
	assert.NoError(t, err)
	assert.Equal(t, src, loaded)
}

func TestReadMissingFileReturnsEmpty(t *testing.T) {
	loaded, err := readPresetFile(t.TempDir() + "/nonexistent.yaml")
	assert.NoError(t, err)
	assert.Empty(t, loaded.Presets)
}

func TestPresetToRules(t *testing.T) {
	p := LogPreset{
		IncludeMode: "all",
		Rules: []PresetRule{
			{Type: "include", Pattern: "foo", Mode: "substring"},
			{Type: "exclude", Pattern: "bar", Mode: "regex"},
			{Type: "severity", Floor: "warn"},
		},
	}
	rules, mode, err := presetToRules(p)
	assert.NoError(t, err)
	assert.Equal(t, IncludeAll, mode)
	assert.Len(t, rules, 3)
	assert.Equal(t, RuleInclude, rules[0].Kind())
	assert.Equal(t, RuleExclude, rules[1].Kind())
	assert.Equal(t, RuleSeverity, rules[2].Kind())
}

func TestRulesToPreset(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	_ = d
	r1, _ := NewPatternRule("foo", PatternSubstring, false)
	r2, _ := NewPatternRule("bar", PatternRegex, true)
	rules := []Rule{r1, r2, SeverityRule{Floor: SeverityError}}
	preset := rulesToPreset("test", true, IncludeAll, rules)
	assert.Equal(t, "test", preset.Name)
	assert.True(t, preset.Default)
	assert.Equal(t, "all", preset.IncludeMode)
	assert.Len(t, preset.Rules, 3)
	assert.Equal(t, "include", preset.Rules[0].Type)
	assert.Equal(t, "exclude", preset.Rules[1].Type)
	assert.Equal(t, "severity", preset.Rules[2].Type)
	assert.Equal(t, "error", preset.Rules[2].Floor)
}

func TestSinglefyDefaults(t *testing.T) {
	presets := []LogPreset{
		{Name: "a", Default: true},
		{Name: "b", Default: true}, // conflict
		{Name: "c", Default: false},
	}
	out := singlefyDefaults(presets)
	assert.True(t, out[0].Default, "first default wins")
	assert.False(t, out[1].Default)
	assert.False(t, out[2].Default)
}

func TestApplyDefaultPresetForKind(t *testing.T) {
	f := PresetFile{
		Presets: map[string][]LogPreset{
			"Pod": {
				{Name: "x", Default: false, Rules: []PresetRule{{Type: "include", Pattern: "x"}}},
				{Name: "default", Default: true, Rules: []PresetRule{{Type: "include", Pattern: "y"}}},
			},
		},
	}
	rules, mode, ok := defaultPresetForKind(f, "Pod")
	assert.True(t, ok)
	assert.Equal(t, IncludeAny, mode)
	assert.Len(t, rules, 1)

	_, _, ok = defaultPresetForKind(f, "Deployment")
	assert.False(t, ok)
}

func TestFieldRulePresetRoundTrip(t *testing.T) {
	// Build a representative runtime FieldRule, serialise it, reload it
	// from the serialised form, and verify equality on all surfaces a
	// user would notice (path, op, value, array-any flag).
	fr, err := NewFieldRule([]string{"user", "id"}, false, FieldOpGte, "42")
	if err != nil {
		t.Fatalf("NewFieldRule: %v", err)
	}
	farr, err := NewFieldRule([]string{"tags"}, true, FieldOpEq, "api")
	if err != nil {
		t.Fatalf("NewFieldRule: %v", err)
	}

	preset := rulesToPreset("api-errors", false, IncludeAll, []Rule{fr, farr})
	assert.Equal(t, "all", preset.IncludeMode)
	assert.Len(t, preset.Rules, 2)

	// Inspect the serialised form directly — these are the fields the
	// user's YAML file will contain.
	assert.Equal(t, "field", preset.Rules[0].Type)
	assert.Equal(t, []string{"user", "id"}, preset.Rules[0].Path)
	assert.False(t, preset.Rules[0].ArrayAny)
	assert.Equal(t, "gte", preset.Rules[0].FieldOp)
	assert.Equal(t, "42", preset.Rules[0].Value)

	assert.Equal(t, "field", preset.Rules[1].Type)
	assert.True(t, preset.Rules[1].ArrayAny)
	assert.Equal(t, "eq", preset.Rules[1].FieldOp)

	// Reload the preset and verify the runtime rules match.
	rules, mode, err := presetToRules(preset)
	if err != nil {
		t.Fatalf("presetToRules: %v", err)
	}
	assert.Equal(t, IncludeAll, mode)
	assert.Len(t, rules, 2)

	loaded1, ok := rules[0].(*FieldRule)
	assert.True(t, ok, "expected *FieldRule, got %T", rules[0])
	assert.Equal(t, fr.Path, loaded1.Path)
	assert.Equal(t, fr.Op, loaded1.Op)
	assert.Equal(t, fr.Value, loaded1.Value)
	assert.Equal(t, fr.ArrayAny, loaded1.ArrayAny)

	loaded2, ok := rules[1].(*FieldRule)
	assert.True(t, ok)
	assert.Equal(t, farr.Path, loaded2.Path)
	assert.True(t, loaded2.ArrayAny)
}

// TestFieldRulePresetInvalidOp validates the reverse direction: a
// preset entry with a garbage field_op surfaces a user-facing error
// rather than silently defaulting to eq.
func TestFieldRulePresetInvalidOp(t *testing.T) {
	preset := LogPreset{
		IncludeMode: "any",
		Rules: []PresetRule{
			{Type: "field", Path: []string{"level"}, FieldOp: "bogus", Value: "error"},
		},
	}
	_, _, err := presetToRules(preset)
	assert.Error(t, err)
}
