package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper: build a non-negated substring include PatternRule.
func inc(pattern string) *PatternRule {
	r, err := NewPatternRule(pattern, PatternSubstring, false)
	if err != nil {
		panic(err)
	}
	return r
}

// Helper: build a negated substring PatternRule. Behaves as unconditional
// exclude when placed at the top of FilterChain; behaves as "NOT matches"
// when placed inside a GroupRule.
func exc(pattern string) *PatternRule {
	r, err := NewPatternRule(pattern, PatternSubstring, true)
	if err != nil {
		panic(err)
	}
	return r
}

// TestGroupRuleKindAndDisplay verifies the Rule interface hookup for
// GroupRule — so the overlay / state builder can render it without
// special-casing.
func TestGroupRuleKindAndDisplay(t *testing.T) {
	var r Rule = &GroupRule{Mode: IncludeAny}
	assert.Equal(t, RuleGroup, r.Kind())
	assert.Contains(t, r.Display(), "Group")
	assert.Contains(t, r.Display(), "any")
}

// TestGroupRuleMatches covers the direct Matches API on GroupRule:
// empty group identity, ANY mode (disjunction), ALL mode (conjunction),
// negation inside a group, and nesting.
func TestGroupRuleMatches(t *testing.T) {
	d, errs := newSeverityDetector(nil)
	require.Empty(t, errs)

	cases := []struct {
		name string
		g    *GroupRule
		line string
		want bool
	}{
		// Empty group identity.
		{"empty ANY is false", &GroupRule{Mode: IncludeAny}, "foo", false},
		{"empty ALL is true", &GroupRule{Mode: IncludeAll}, "foo", true},

		// ANY mode: disjunction.
		{
			"any: first child matches",
			&GroupRule{Mode: IncludeAny, Children: []Rule{inc("foo"), inc("bar")}},
			"x foo", true,
		},
		{
			"any: second child matches",
			&GroupRule{Mode: IncludeAny, Children: []Rule{inc("foo"), inc("bar")}},
			"x bar", true,
		},
		{
			"any: no child matches",
			&GroupRule{Mode: IncludeAny, Children: []Rule{inc("foo"), inc("bar")}},
			"x baz", false,
		},

		// ALL mode: conjunction.
		{
			"all: every child matches",
			&GroupRule{Mode: IncludeAll, Children: []Rule{inc("foo"), inc("bar")}},
			"x foo bar baz", true,
		},
		{
			"all: one child misses",
			&GroupRule{Mode: IncludeAll, Children: []Rule{inc("foo"), inc("bar")}},
			"x foo baz", false,
		},

		// Negation inside a group: "match iff line contains foo AND NOT bar".
		{
			"all + negate: matches when pattern absent",
			&GroupRule{Mode: IncludeAll, Children: []Rule{inc("foo"), exc("bar")}},
			"x foo", true,
		},
		{
			"all + negate: fails when pattern present",
			&GroupRule{Mode: IncludeAll, Children: []Rule{inc("foo"), exc("bar")}},
			"x foo bar", false,
		},

		// Nested group: foo AND (bar OR baz).
		{
			"nested: foo AND (bar OR baz) — bar present",
			&GroupRule{Mode: IncludeAll, Children: []Rule{
				inc("foo"),
				&GroupRule{Mode: IncludeAny, Children: []Rule{inc("bar"), inc("baz")}},
			}},
			"x foo bar", true,
		},
		{
			"nested: foo AND (bar OR baz) — baz present",
			&GroupRule{Mode: IncludeAll, Children: []Rule{
				inc("foo"),
				&GroupRule{Mode: IncludeAny, Children: []Rule{inc("bar"), inc("baz")}},
			}},
			"x foo baz", true,
		},
		{
			"nested: foo AND (bar OR baz) — only foo",
			&GroupRule{Mode: IncludeAll, Children: []Rule{
				inc("foo"),
				&GroupRule{Mode: IncludeAny, Children: []Rule{inc("bar"), inc("baz")}},
			}},
			"x foo qux", false,
		},
		{
			"nested: foo AND (bar OR baz) — no foo",
			&GroupRule{Mode: IncludeAll, Children: []Rule{
				inc("foo"),
				&GroupRule{Mode: IncludeAny, Children: []Rule{inc("bar"), inc("baz")}},
			}},
			"x bar baz", false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, c.g.Matches(c.line, d))
		})
	}
}

// TestFilterChainWithGroupInIncludes verifies that GroupRule can sit
// alongside PatternRule in the top-level rules list and be evaluated
// correctly under both ANY and ALL top-level modes.
func TestFilterChainWithGroupInIncludes(t *testing.T) {
	d, _ := newSeverityDetector(nil)

	// Top-level ALL: [foo, (bar OR baz)] → "foo AND (bar OR baz)"
	t.Run("top ALL with group", func(t *testing.T) {
		group := &GroupRule{Mode: IncludeAny, Children: []Rule{inc("bar"), inc("baz")}}
		chain := NewFilterChain([]Rule{inc("foo"), group}, IncludeAll, d)

		assert.True(t, chain.Keep("foo bar"), "foo AND bar")
		assert.True(t, chain.Keep("foo baz"), "foo AND baz")
		assert.False(t, chain.Keep("foo qux"), "foo but neither bar nor baz")
		assert.False(t, chain.Keep("bar baz"), "bar and baz but no foo")
	})

	// Top-level ANY: [foo, (bar AND baz)] → "foo OR (bar AND baz)"
	t.Run("top ANY with group", func(t *testing.T) {
		group := &GroupRule{Mode: IncludeAll, Children: []Rule{inc("bar"), inc("baz")}}
		chain := NewFilterChain([]Rule{inc("foo"), group}, IncludeAny, d)

		assert.True(t, chain.Keep("foo alone"), "foo satisfies OR")
		assert.True(t, chain.Keep("bar baz qux"), "bar AND baz satisfies OR")
		assert.False(t, chain.Keep("bar alone"), "only one half of the AND group")
		assert.False(t, chain.Keep("baz alone"), "only the other half")
		assert.False(t, chain.Keep("nothing"), "neither branch")
	})

	// Excludes at top level still unconditionally drop lines, even when
	// groups are also present. This preserves the pre-groups semantic.
	t.Run("top-level exclude drops before group evaluated", func(t *testing.T) {
		group := &GroupRule{Mode: IncludeAny, Children: []Rule{inc("foo")}}
		chain := NewFilterChain([]Rule{exc("/healthz"), group}, IncludeAny, d)

		assert.False(t, chain.Keep("GET /healthz foo"), "healthz exclude drops")
		assert.True(t, chain.Keep("real request foo"), "foo group matches")
	})

	// Backward compat: no groups present → behavior identical to pre-groups.
	t.Run("no groups → original semantics preserved", func(t *testing.T) {
		chain := NewFilterChain([]Rule{inc("foo"), inc("bar")}, IncludeAll, d)
		assert.True(t, chain.Keep("foo bar"))
		assert.False(t, chain.Keep("only foo"))
	})
}

// TestEvalRuleAsMatchSeverityInGroup covers the less-common case of a
// SeverityRule placed inside a GroupRule. In that position it acts as a
// pure predicate (severity >= floor) without the Unknown-kept escape
// hatch that top-level severity enjoys.
func TestEvalRuleAsMatchSeverityInGroup(t *testing.T) {
	d, _ := newSeverityDetector(nil)

	group := &GroupRule{Mode: IncludeAll, Children: []Rule{
		SeverityRule{Floor: SeverityError},
		inc("api"),
	}}

	assert.True(t, group.Matches("[ERROR] api request failed", d),
		"ERROR + api satisfies both")
	assert.False(t, group.Matches("[INFO] api request", d),
		"INFO < ERROR floor fails the severity predicate")
	assert.False(t, group.Matches("plain message with no level", d),
		"Unknown severity fails the in-group severity predicate (no Unknown-kept escape)")
}

// TestPresetGroupRoundtrip verifies that a preset containing a nested
// group survives YAML round-trip (runtime rules → preset → runtime rules)
// with its structure and semantics intact.
func TestPresetGroupRoundtrip(t *testing.T) {
	// Build an expression: foo AND (bar OR baz)
	original := []Rule{
		inc("foo"),
		&GroupRule{Mode: IncludeAny, Children: []Rule{
			inc("bar"),
			inc("baz"),
		}},
	}

	preset := rulesToPreset("nested-test", false, IncludeAll, original)

	// The on-disk shape should contain a group entry with children.
	require.Len(t, preset.Rules, 2)
	assert.Equal(t, "include", preset.Rules[0].Type)
	assert.Equal(t, "group", preset.Rules[1].Type)
	assert.Equal(t, "any", preset.Rules[1].Mode)
	require.Len(t, preset.Rules[1].Children, 2)
	assert.Equal(t, "bar", preset.Rules[1].Children[0].Pattern)
	assert.Equal(t, "baz", preset.Rules[1].Children[1].Pattern)

	// Convert back to runtime rules; semantics must be preserved.
	restored, mode, err := presetToRules(preset)
	require.NoError(t, err)
	assert.Equal(t, IncludeAll, mode)
	require.Len(t, restored, 2)

	d, _ := newSeverityDetector(nil)
	chain := NewFilterChain(restored, mode, d)

	assert.True(t, chain.Keep("foo bar"), "foo AND bar → keep")
	assert.True(t, chain.Keep("foo baz"), "foo AND baz → keep")
	assert.False(t, chain.Keep("foo qux"), "foo without group match → drop")
	assert.False(t, chain.Keep("bar baz"), "group match without foo → drop")
}

// TestPresetGroupDeepNesting covers 3-level nesting:
//
//	(foo OR (bar AND (baz OR qux)))
func TestPresetGroupDeepNesting(t *testing.T) {
	deep := []Rule{
		&GroupRule{Mode: IncludeAny, Children: []Rule{
			inc("foo"),
			&GroupRule{Mode: IncludeAll, Children: []Rule{
				inc("bar"),
				&GroupRule{Mode: IncludeAny, Children: []Rule{
					inc("baz"),
					inc("qux"),
				}},
			}},
		}},
	}

	preset := rulesToPreset("deep", false, IncludeAny, deep)
	restored, _, err := presetToRules(preset)
	require.NoError(t, err)

	d, _ := newSeverityDetector(nil)
	chain := NewFilterChain(restored, IncludeAny, d)

	assert.True(t, chain.Keep("xx foo"), "foo alone satisfies outer ANY")
	assert.True(t, chain.Keep("xx bar baz"), "bar AND baz satisfies inner branch")
	assert.True(t, chain.Keep("xx bar qux"), "bar AND qux satisfies inner branch")
	assert.False(t, chain.Keep("xx bar"), "bar alone fails the inner ALL")
	assert.False(t, chain.Keep("xx baz qux"), "baz+qux without bar fails the inner ALL")
}
