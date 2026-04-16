package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseRuleInputGroupFlat covers the base case: `(a OP b)` with both
// OR and AND operators, including the case-insensitive variants.
func TestParseRuleInputGroupFlat(t *testing.T) {
	cases := []struct {
		input    string
		wantMode IncludeMode
	}{
		{"(foo OR bar)", IncludeAny},
		{"(foo AND bar)", IncludeAll},
		{"(foo or bar)", IncludeAny},
		{"(foo and bar)", IncludeAll},
		{"(foo Or bar)", IncludeAny},
		{"(foo aNd bar)", IncludeAll},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			r, err := ParseRuleInput(c.input)
			require.NoError(t, err)
			g, ok := r.(*GroupRule)
			require.True(t, ok, "expected *GroupRule, got %T", r)
			assert.Equal(t, c.wantMode, g.Mode)
			require.Len(t, g.Children, 2)

			// Each child should be an include PatternRule with the verbatim
			// substring pattern.
			first, ok := g.Children[0].(*PatternRule)
			require.True(t, ok)
			assert.Equal(t, "foo", first.Pattern)
			assert.Equal(t, PatternSubstring, first.Mode)
			assert.False(t, first.Negate)

			second, ok := g.Children[1].(*PatternRule)
			require.True(t, ok)
			assert.Equal(t, "bar", second.Pattern)
		})
	}
}

// TestParseRuleInputImplicitOuterParens verifies that the user can type
// `foo AND bar` without the outer parens and get the same group semantics
// as `(foo AND bar)`. This was reported as a UX bug — people naturally
// write expressions without outer wrapping.
func TestParseRuleInputImplicitOuterParens(t *testing.T) {
	cases := []struct {
		input    string
		wantMode IncludeMode
		wantKids []string // raw pattern text expected for each child
	}{
		{"server AND Leaving", IncludeAll, []string{"server", "Leaving"}},
		{"foo OR bar", IncludeAny, []string{"foo", "bar"}},
		{"a AND b AND c", IncludeAll, []string{"a", "b", "c"}},
		{"foo and bar", IncludeAll, []string{"foo", "bar"}}, // case-insensitive
		// Mixed: top-level without parens, nested group with parens.
		{"foo AND (bar OR baz)", IncludeAll, []string{"foo", "(bar OR baz)"}},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			r, err := ParseRuleInput(c.input)
			require.NoError(t, err)
			g, ok := r.(*GroupRule)
			require.True(t, ok, "expected *GroupRule, got %T", r)
			assert.Equal(t, c.wantMode, g.Mode)
			require.Len(t, g.Children, len(c.wantKids))
		})
	}
}

// TestContainsTopLevelBoolOp covers the helper that gates the
// implicit-outer-parens behavior. Needs whole-word detection so that
// patterns like "ORACLE" and "ANDROID" are NOT misinterpreted as OR / AND.
func TestContainsTopLevelBoolOp(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"foo AND bar", true},
		{"foo OR bar", true},
		{"foo and bar", true},
		{"foo or bar", true},
		{"FOO AND BAR", true},

		// Whole-word: these identifiers merely contain the letters.
		{"ORACLE database", false},
		{"ANDROID debug", false},
		{"foo LAND bar", false}, // "LAND" is a whole word, but not " AND "
		{"SAND castle", false},

		// No operators at all.
		{"plain substring", false},
		{"", false},

		// Inside parens — depth > 0, should NOT trigger implicit outer parens
		// (the group parser handles the inner AND/OR).
		{"(foo AND bar)", false},
		{"(foo OR bar)", false},

		// Mix: inner group, outer AND.
		{"(foo OR bar) AND baz", true},

		// Trailing operator (incomplete) — ignore.
		{"foo AND", false},
		{"foo OR", false},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			assert.Equal(t, c.want, containsTopLevelBoolOp(c.input))
		})
	}
}

// TestParseRuleInputGroupManyChildren verifies that three or more children
// are supported: `(a OR b OR c)` produces a 3-child ANY group.
func TestParseRuleInputGroupManyChildren(t *testing.T) {
	r, err := ParseRuleInput("(a OR b OR c)")
	require.NoError(t, err)
	g, ok := r.(*GroupRule)
	require.True(t, ok)
	assert.Equal(t, IncludeAny, g.Mode)
	require.Len(t, g.Children, 3)

	all, err2 := ParseRuleInput("(a AND b AND c AND d)")
	require.NoError(t, err2)
	gAll, ok := all.(*GroupRule)
	require.True(t, ok)
	assert.Equal(t, IncludeAll, gAll.Mode)
	require.Len(t, gAll.Children, 4)
}

// TestParseRuleInputGroupNested covers a single-level nested group.
func TestParseRuleInputGroupNested(t *testing.T) {
	r, err := ParseRuleInput("(foo AND (bar OR baz))")
	require.NoError(t, err)
	g, ok := r.(*GroupRule)
	require.True(t, ok)
	assert.Equal(t, IncludeAll, g.Mode)
	require.Len(t, g.Children, 2)

	first, ok := g.Children[0].(*PatternRule)
	require.True(t, ok)
	assert.Equal(t, "foo", first.Pattern)

	inner, ok := g.Children[1].(*GroupRule)
	require.True(t, ok)
	assert.Equal(t, IncludeAny, inner.Mode)
	require.Len(t, inner.Children, 2)

	bar, ok := inner.Children[0].(*PatternRule)
	require.True(t, ok)
	assert.Equal(t, "bar", bar.Pattern)

	baz, ok := inner.Children[1].(*PatternRule)
	require.True(t, ok)
	assert.Equal(t, "baz", baz.Pattern)
}

// TestParseRuleInputGroupDeepNested covers three-level nesting.
func TestParseRuleInputGroupDeepNested(t *testing.T) {
	r, err := ParseRuleInput("(foo OR (bar AND (baz OR qux)))")
	require.NoError(t, err)
	g, ok := r.(*GroupRule)
	require.True(t, ok)
	assert.Equal(t, IncludeAny, g.Mode)
	require.Len(t, g.Children, 2)

	middle, ok := g.Children[1].(*GroupRule)
	require.True(t, ok)
	assert.Equal(t, IncludeAll, middle.Mode)
	require.Len(t, middle.Children, 2)

	inner, ok := middle.Children[1].(*GroupRule)
	require.True(t, ok)
	assert.Equal(t, IncludeAny, inner.Mode)
	require.Len(t, inner.Children, 2)
}

// TestParseRuleInputGroupLeafModes covers the full leaf grammar inside a
// group: substring, fuzzy (~), negated (-), and regex.
func TestParseRuleInputGroupLeafModes(t *testing.T) {
	r, err := ParseRuleInput("(~typo OR -bar OR ^err.*$)")
	require.NoError(t, err)
	g, ok := r.(*GroupRule)
	require.True(t, ok)
	require.Len(t, g.Children, 3)

	fuzzy, ok := g.Children[0].(*PatternRule)
	require.True(t, ok)
	assert.Equal(t, "typo", fuzzy.Pattern)
	assert.Equal(t, PatternFuzzy, fuzzy.Mode)
	assert.False(t, fuzzy.Negate)

	neg, ok := g.Children[1].(*PatternRule)
	require.True(t, ok)
	assert.Equal(t, "bar", neg.Pattern)
	assert.Equal(t, PatternSubstring, neg.Mode)
	assert.True(t, neg.Negate)

	rx, ok := g.Children[2].(*PatternRule)
	require.True(t, ok)
	assert.Equal(t, "^err.*$", rx.Pattern)
	assert.Equal(t, PatternRegex, rx.Mode)
}

// TestParseRuleInputGroupWhitespaceTolerant verifies that extra whitespace
// around tokens, operators, and brackets is tolerated.
func TestParseRuleInputGroupWhitespaceTolerant(t *testing.T) {
	inputs := []string{
		"(  foo   OR   bar  )",
		"( foo OR bar )",
		"(foo  OR  bar)",
		"\t(foo OR bar)\t",
	}
	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			r, err := ParseRuleInput(in)
			require.NoError(t, err)
			g, ok := r.(*GroupRule)
			require.True(t, ok)
			assert.Equal(t, IncludeAny, g.Mode)
			require.Len(t, g.Children, 2)
		})
	}
}

// TestParseRuleInputGroupErrors enumerates the specific error cases the
// parser must reject with user-facing messages.
func TestParseRuleInputGroupErrors(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantSub string // substring the error must contain
	}{
		{
			"mixed operators in one level",
			"(foo OR bar AND baz)",
			"mix",
		},
		{
			"unclosed group",
			"(foo OR bar",
			"unclosed",
		},
		{
			"empty group",
			"()",
			"empty",
		},
		{
			"missing operator",
			"(foo bar)",
			"operator",
		},
		{
			"operator without right operand",
			"(foo OR)",
			"operand",
		},
		{
			"operator without left operand",
			"(OR bar)",
			"operand",
		},
		{
			"severity inside group",
			"(>error OR foo)",
			"severity",
		},
		{
			"trailing garbage after closing paren",
			"(foo OR bar) extra",
			"trailing",
		},
		{
			"unexpected closing paren",
			"(foo OR bar))",
			"trailing",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseRuleInput(c.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.wantSub)
		})
	}
}

// TestParseRuleInputNonGroupUnchanged pins the pre-existing scalar-input
// behavior: anything that doesn't start with '(' must fall through to
// the existing leaf parser (covered by TestParseRuleInput).
func TestParseRuleInputNonGroupUnchanged(t *testing.T) {
	r, err := ParseRuleInput("foo")
	require.NoError(t, err)
	_, ok := r.(*PatternRule)
	assert.True(t, ok)

	r2, err := ParseRuleInput(">error")
	require.NoError(t, err)
	_, ok = r2.(SeverityRule)
	assert.True(t, ok)
}

// TestParseRuleInputGroupEndToEnd builds a complete filter scenario from
// a typed group expression and verifies FilterChain evaluates correctly.
func TestParseRuleInputGroupEndToEnd(t *testing.T) {
	r, err := ParseRuleInput("(foo AND (bar OR baz))")
	require.NoError(t, err)

	d, _ := newSeverityDetector(nil)
	chain := NewFilterChain([]Rule{r}, IncludeAll, d)

	assert.True(t, chain.Keep("foo bar"), "foo AND bar → keep")
	assert.True(t, chain.Keep("foo baz"), "foo AND baz → keep")
	assert.False(t, chain.Keep("foo qux"), "foo without (bar|baz) → drop")
	assert.False(t, chain.Keep("bar baz"), "no foo → drop")
}
