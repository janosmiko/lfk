package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// logPresetsPath returns the absolute path to the sidecar preset file.
// Honors $XDG_CONFIG_HOME (if set) else uses os.UserConfigDir() / "lfk".
func logPresetsPath() (string, error) {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "lfk", "log_presets.yaml"), nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	return filepath.Join(dir, "lfk", "log_presets.yaml"), nil
}

// PresetFile is the on-disk schema for log_presets.yaml.
type PresetFile struct {
	Presets map[string][]LogPreset `yaml:"presets"`
}

// LogPreset is one named preset for a resource Kind.
type LogPreset struct {
	Name        string       `yaml:"name"`
	Default     bool         `yaml:"default"`
	IncludeMode string       `yaml:"include_mode"`
	Rules       []PresetRule `yaml:"rules"`
}

// PresetRule is one rule entry within a preset. A single struct carries
// all shapes (pattern, severity, group, field) so the YAML stays flat-ish
// and old presets keep working — unused fields stay zero and are omitted
// via yaml:",omitempty".
type PresetRule struct {
	Type    string `yaml:"type"`              // "include" | "exclude" | "severity" | "group" | "field"
	Pattern string `yaml:"pattern,omitempty"` // for include/exclude
	Mode    string `yaml:"mode,omitempty"`    // include/exclude: "substring"|"regex"|"fuzzy". group: "any"|"all"
	Floor   string `yaml:"floor,omitempty"`   // for severity

	// Children are the nested rules of a group. Recursively typed so
	// groups can nest arbitrarily. Only set when Type == "group".
	Children []PresetRule `yaml:"children,omitempty"`

	// Field-rule fields. Only set when Type == "field".
	Path     []string `yaml:"path,omitempty"`      // dotted path segments, e.g. ["user", "id"]
	ArrayAny bool     `yaml:"array_any,omitempty"` // true for the "[]" array-any form
	FieldOp  string   `yaml:"field_op,omitempty"`  // "eq"|"neq"|"gt"|"gte"|"lt"|"lte"|"match"
	Value    string   `yaml:"value,omitempty"`     // raw RHS value
}

// readPresetFile loads the preset sidecar file. A missing file is treated as
// an empty PresetFile (not an error).
func readPresetFile(path string) (PresetFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return PresetFile{}, nil
		}
		return PresetFile{}, fmt.Errorf("read preset file: %w", err)
	}
	var f PresetFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return PresetFile{}, fmt.Errorf("parse preset file: %w", err)
	}
	return f, nil
}

// writePresetFile serializes the PresetFile and atomically replaces path.
func writePresetFile(path string, f PresetFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	data, err := yaml.Marshal(f)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("atomic rename: %w", err)
	}
	return nil
}

// presetToRules converts the on-disk preset representation into runtime
// Rule values plus the include-mode flag.
func presetToRules(p LogPreset) ([]Rule, IncludeMode, error) {
	mode := IncludeAny
	if p.IncludeMode == "all" {
		mode = IncludeAll
	}
	out, err := presetRulesToRules(p.Rules)
	if err != nil {
		return nil, mode, err
	}
	return out, mode, nil
}

// presetRulesToRules converts a slice of on-disk PresetRule entries into
// runtime Rule values. Groups recurse through this function so nesting
// works at arbitrary depth.
func presetRulesToRules(prs []PresetRule) ([]Rule, error) {
	out := make([]Rule, 0, len(prs))
	for _, pr := range prs {
		r, err := presetRuleToRule(pr)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// presetRuleToRule converts one on-disk PresetRule to a runtime Rule.
func presetRuleToRule(pr PresetRule) (Rule, error) {
	switch pr.Type {
	case "include", "exclude":
		pm := PatternSubstring
		switch pr.Mode {
		case "regex":
			pm = PatternRegex
		case "fuzzy":
			pm = PatternFuzzy
		}
		r, err := NewPatternRule(pr.Pattern, pm, pr.Type == "exclude")
		if err != nil {
			return nil, fmt.Errorf("rule %q: %w", pr.Pattern, err)
		}
		return r, nil
	case "severity":
		var floor Severity
		switch pr.Floor {
		case "error":
			floor = SeverityError
		case "warn":
			floor = SeverityWarn
		case "info":
			floor = SeverityInfo
		case "debug":
			floor = SeverityDebug
		default:
			return nil, fmt.Errorf("unknown severity floor %q", pr.Floor)
		}
		return SeverityRule{Floor: floor}, nil
	case "group":
		gmode := IncludeAny
		if pr.Mode == "all" {
			gmode = IncludeAll
		}
		children, err := presetRulesToRules(pr.Children)
		if err != nil {
			return nil, fmt.Errorf("group: %w", err)
		}
		return &GroupRule{Mode: gmode, Children: children}, nil
	case "field":
		op, err := parseFieldOp(pr.FieldOp)
		if err != nil {
			return nil, fmt.Errorf("field rule: %w", err)
		}
		r, err := NewFieldRule(pr.Path, pr.ArrayAny, op, pr.Value)
		if err != nil {
			return nil, fmt.Errorf("field rule: %w", err)
		}
		return r, nil
	default:
		return nil, fmt.Errorf("unknown rule type %q", pr.Type)
	}
}

// defaultPresetForKind returns the rules + mode of the default preset for kind,
// or false if there isn't one.
func defaultPresetForKind(f PresetFile, kind string) ([]Rule, IncludeMode, bool) {
	for _, p := range f.Presets[kind] {
		if p.Default {
			rules, mode, err := presetToRules(p)
			if err != nil {
				return nil, IncludeAny, false
			}
			return rules, mode, true
		}
	}
	return nil, IncludeAny, false
}

// singlefyDefaults ensures at most one preset has Default=true (first wins).
func singlefyDefaults(ps []LogPreset) []LogPreset {
	out := make([]LogPreset, len(ps))
	copy(out, ps)
	seen := false
	for i := range out {
		if out[i].Default {
			if seen {
				out[i].Default = false
			}
			seen = true
		}
	}
	return out
}

// rulesToPreset converts a runtime Rule slice back to a serializable
// LogPreset so it can be written to the sidecar file.
func rulesToPreset(name string, isDefault bool, mode IncludeMode, rules []Rule) LogPreset {
	return LogPreset{
		Name:        name,
		Default:     isDefault,
		IncludeMode: mode.String(),
		Rules:       rulesToPresetRules(rules),
	}
}

// rulesToPresetRules recursively serialises a runtime Rule slice to the
// on-disk PresetRule form. Groups nest naturally.
func rulesToPresetRules(rules []Rule) []PresetRule {
	out := make([]PresetRule, 0, len(rules))
	for _, r := range rules {
		out = append(out, ruleToPresetRule(r))
	}
	return out
}

func ruleToPresetRule(r Rule) PresetRule {
	switch v := r.(type) {
	case *PatternRule:
		pm := "substring"
		switch v.Mode {
		case PatternRegex:
			pm = "regex"
		case PatternFuzzy:
			pm = "fuzzy"
		}
		t := "include"
		if v.Negate {
			t = "exclude"
		}
		return PresetRule{Type: t, Pattern: v.Pattern, Mode: pm}
	case SeverityRule:
		return PresetRule{Type: "severity", Floor: strings.ToLower(v.Floor.String())}
	case *GroupRule:
		return PresetRule{
			Type:     "group",
			Mode:     v.Mode.String(),
			Children: rulesToPresetRules(v.Children),
		}
	case *FieldRule:
		return PresetRule{
			Type:     "field",
			Path:     append([]string(nil), v.Path...),
			ArrayAny: v.ArrayAny,
			FieldOp:  v.Op.serialisedOp(),
			Value:    v.Value,
		}
	}
	return PresetRule{}
}
