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
