package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExprSummaryPatternRule covers the leaf PatternRule rendering:
// substrings appear verbatim, fuzzy prefixed with "~", regex verbatim,
// and negated leaves prefixed with "!".
func TestExprSummaryPatternRule(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		mode    PatternMode
		negate  bool
		want    string
	}{
		{"substring plain", "foo", PatternSubstring, false, "foo"},
		{"substring negated", "foo", PatternSubstring, true, "!foo"},
		{"fuzzy plain", "typo", PatternFuzzy, false, "~typo"},
		{"fuzzy negated", "typo", PatternFuzzy, true, "!~typo"},
		{"regex plain", "^err.*$", PatternRegex, false, "^err.*$"},
		{"regex negated", "^err.*$", PatternRegex, true, "!^err.*$"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, err := NewPatternRule(c.pattern, c.mode, c.negate)
			assert.NoError(t, err)
			assert.Equal(t, c.want, exprSummary(r))
		})
	}
}

// TestExprSummarySeverityRule uses the existing Severity.String() via
// SeverityRule.Display() — severity is rendered as ">= LEVEL".
func TestExprSummarySeverityRule(t *testing.T) {
	assert.Equal(t, ">= ERROR", exprSummary(SeverityRule{Floor: SeverityError}))
	assert.Equal(t, ">= WARN", exprSummary(SeverityRule{Floor: SeverityWarn}))
}

// TestExprSummaryGroupFlat verifies the OP joiner: AND for ALL, OR for ANY.
func TestExprSummaryGroupFlat(t *testing.T) {
	any := &GroupRule{Mode: IncludeAny, Children: []Rule{inc("foo"), inc("bar")}}
	assert.Equal(t, "(foo OR bar)", exprSummary(any))

	all := &GroupRule{Mode: IncludeAll, Children: []Rule{inc("foo"), inc("bar")}}
	assert.Equal(t, "(foo AND bar)", exprSummary(all))
}

// TestExprSummaryGroupNegatedChildren verifies that negated PatternRule
// children inside a group render with the "!" prefix (the in-group
// negation semantics).
func TestExprSummaryGroupNegatedChildren(t *testing.T) {
	g := &GroupRule{Mode: IncludeAll, Children: []Rule{inc("foo"), exc("bar")}}
	assert.Equal(t, "(foo AND !bar)", exprSummary(g))
}

// TestExprSummaryGroupNested verifies nested parens: a group inside a
// group produces another pair of parens.
func TestExprSummaryGroupNested(t *testing.T) {
	// foo AND (bar OR baz)
	g := &GroupRule{Mode: IncludeAll, Children: []Rule{
		inc("foo"),
		&GroupRule{Mode: IncludeAny, Children: []Rule{inc("bar"), inc("baz")}},
	}}
	assert.Equal(t, "(foo AND (bar OR baz))", exprSummary(g))
}

// TestExprSummaryGroupEmpty ensures degenerate empty groups render as
// empty parens — unusual but not a crash.
func TestExprSummaryGroupEmpty(t *testing.T) {
	assert.Equal(t, "()", exprSummary(&GroupRule{Mode: IncludeAny}))
}

// TestExprSummaryGroupSingleChild covers the one-child case: the joiner
// never appears because there is only one child.
func TestExprSummaryGroupSingleChild(t *testing.T) {
	g := &GroupRule{Mode: IncludeAll, Children: []Rule{inc("foo")}}
	assert.Equal(t, "(foo)", exprSummary(g))
}

// TestRuleToRowStateGroup verifies the overlay projection for a group rule:
// Kind="GRP", Mode="any"/"all", Pattern=group summary.
func TestRuleToRowStateGroup(t *testing.T) {
	g := &GroupRule{Mode: IncludeAny, Children: []Rule{inc("foo"), inc("bar")}}
	row := ruleToRowState(g)
	assert.Equal(t, "GRP", row.Kind)
	assert.Equal(t, "any", row.Mode)
	assert.Equal(t, "(foo OR bar)", row.Pattern)

	g2 := &GroupRule{Mode: IncludeAll, Children: []Rule{inc("foo"), inc("bar")}}
	row2 := ruleToRowState(g2)
	assert.Equal(t, "GRP", row2.Kind)
	assert.Equal(t, "all", row2.Mode)
	assert.Equal(t, "(foo AND bar)", row2.Pattern)
}

// TestRuleToRowStatePatternPreserved ensures the existing PatternRule and
// SeverityRule projections remain unchanged.
func TestRuleToRowStatePatternPreserved(t *testing.T) {
	pr, err := NewPatternRule("hello", PatternSubstring, false)
	assert.NoError(t, err)
	row := ruleToRowState(pr)
	assert.Equal(t, "INC", row.Kind)
	assert.Equal(t, "substr", row.Mode)
	assert.Equal(t, "hello", row.Pattern)

	sev := SeverityRule{Floor: SeverityError}
	row2 := ruleToRowState(sev)
	assert.Equal(t, "SEV", row2.Kind)
	assert.Equal(t, "", row2.Mode)
	assert.Equal(t, ">= ERROR", row2.Pattern)
}
