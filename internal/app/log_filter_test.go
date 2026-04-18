package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuleKindString(t *testing.T) {
	assert.Equal(t, "INC", RuleInclude.String())
	assert.Equal(t, "EXC", RuleExclude.String())
	assert.Equal(t, "SEV", RuleSeverity.String())
	assert.Equal(t, "GRP", RuleGroup.String())
}

func TestIncludeModeString(t *testing.T) {
	assert.Equal(t, "any", IncludeAny.String())
	assert.Equal(t, "all", IncludeAll.String())
}

func TestPatternRuleMatches(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		mode    PatternMode
		negate  bool
		line    string
		want    bool
	}{
		{"substr include match", "foo", PatternSubstring, false, "x foo y", true},
		{"substr include miss", "foo", PatternSubstring, false, "bar baz", false},
		{"substr exclude match (negate)", "foo", PatternSubstring, true, "x foo y", true},
		{"regex include", `^err`, PatternRegex, false, "err: boom", true},
		{"regex include miss", `^err`, PatternRegex, false, "x err", false},
		{"fuzzy include", "abc", PatternFuzzy, false, "axxbxxc", true},
		{"fuzzy include miss", "abc", PatternFuzzy, false, "axxxxx", false},
		{"case-insensitive substr", "FOO", PatternSubstring, false, "foo bar", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, err := NewPatternRule(c.pattern, c.mode, c.negate)
			assert.NoError(t, err)
			assert.Equal(t, c.want, r.Matches(c.line))
		})
	}
}

func TestPatternRuleInvalidRegex(t *testing.T) {
	_, err := NewPatternRule("[invalid(", PatternRegex, false)
	assert.Error(t, err)
}

func TestSeverityRuleMatches(t *testing.T) {
	r := SeverityRule{Floor: SeverityWarn}
	d, _ := newSeverityDetector(nil)

	assert.True(t, r.Allows(d.Detect("[ERROR] x")), "error >= warn")
	assert.True(t, r.Allows(d.Detect("[WARN] x")), "warn >= warn")
	assert.False(t, r.Allows(d.Detect("[INFO] x")), "info < warn")
	assert.False(t, r.Allows(d.Detect("[DEBUG] x")), "debug < warn")
	assert.True(t, r.Allows(d.Detect("plain text no level")), "Unknown is kept by default")
}

func TestSeverityRuleDisplay(t *testing.T) {
	r := SeverityRule{Floor: SeverityWarn}
	assert.Equal(t, ">= WARN", r.Display())
}

func TestSeverityRuleKind(t *testing.T) {
	r := SeverityRule{Floor: SeverityError}
	assert.Equal(t, RuleSeverity, r.Kind())
}

func TestRuleInterface(t *testing.T) {
	var _ Rule = (*PatternRule)(nil)
	var _ Rule = SeverityRule{}
}

func TestFilterChainAnyMode(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	mkInc := func(p string) *PatternRule {
		r, _ := NewPatternRule(p, PatternSubstring, false)
		return r
	}
	mkExc := func(p string) *PatternRule {
		r, _ := NewPatternRule(p, PatternSubstring, true)
		return r
	}

	cases := []struct {
		name  string
		rules []Rule
		mode  IncludeMode
		line  string
		keep  bool
	}{
		{"no rules → keep all", nil, IncludeAny, "anything", true},
		{"single include match", []Rule{mkInc("foo")}, IncludeAny, "x foo y", true},
		{"single include miss", []Rule{mkInc("foo")}, IncludeAny, "x bar y", false},
		{"two includes ANY: one matches", []Rule{mkInc("foo"), mkInc("bar")}, IncludeAny, "x bar y", true},
		{"two includes ANY: none match", []Rule{mkInc("foo"), mkInc("bar")}, IncludeAny, "x baz y", false},
		{"exclude drops line", []Rule{mkExc("/healthz")}, IncludeAny, "GET /healthz 200", false},
		{"exclude allows other", []Rule{mkExc("/healthz")}, IncludeAny, "GET /api 200", true},
		{"exclude takes priority over include", []Rule{mkInc("api"), mkExc("/healthz")}, IncludeAny, "/healthz api", false},
		{"severity floor keeps error", []Rule{SeverityRule{SeverityWarn}}, IncludeAny, "[ERROR] x", true},
		{"severity floor drops info", []Rule{SeverityRule{SeverityWarn}}, IncludeAny, "[INFO] x", false},
		{"severity + exclude + include", []Rule{
			SeverityRule{SeverityWarn},
			mkExc("/healthz"),
			mkInc("api"),
		}, IncludeAny, "[ERROR] api request", true},
		{"severity + exclude + include: include miss", []Rule{
			SeverityRule{SeverityWarn},
			mkExc("/healthz"),
			mkInc("api"),
		}, IncludeAny, "[ERROR] startup complete", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			chain := NewFilterChain(c.rules, c.mode, d)
			assert.Equal(t, c.keep, chain.Keep(c.line))
		})
	}
}

func TestFilterChainAllMode(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	mkInc := func(p string) *PatternRule {
		r, _ := NewPatternRule(p, PatternSubstring, false)
		return r
	}
	rules := []Rule{mkInc("user_id=42"), mkInc("request_id")}
	chain := NewFilterChain(rules, IncludeAll, d)

	assert.True(t, chain.Keep("user_id=42 request_id=xyz"), "both match")
	assert.False(t, chain.Keep("user_id=42 only"), "only one matches")
	assert.False(t, chain.Keep("request_id=xyz only"), "only the other")
	assert.False(t, chain.Keep("neither"), "no match")
}

func TestParseRuleInput(t *testing.T) {
	cases := []struct {
		input    string
		wantKind RuleKind
		wantPat  string
		wantMode PatternMode
		wantSev  Severity // only for severity rules
		wantErr  bool
	}{
		// Include
		{"foo", RuleInclude, "foo", PatternSubstring, 0, false},
		{"~typo", RuleInclude, "typo", PatternFuzzy, 0, false},
		{"^err.*$", RuleInclude, "^err.*$", PatternRegex, 0, false},

		// Exclude
		{"-foo", RuleExclude, "foo", PatternSubstring, 0, false},
		{"-~typo", RuleExclude, "typo", PatternFuzzy, 0, false},
		{"-^foo$", RuleExclude, "^foo$", PatternRegex, 0, false},

		// Severity (exact match only)
		{">error", RuleSeverity, "", 0, SeverityError, false},
		{">warn", RuleSeverity, "", 0, SeverityWarn, false},
		{">info", RuleSeverity, "", 0, SeverityInfo, false},
		{">debug", RuleSeverity, "", 0, SeverityDebug, false},
		{">ERROR", RuleSeverity, "", 0, SeverityError, false}, // case-insensitive

		// Edge cases — these become include patterns
		{">warning", RuleInclude, ">warning", PatternSubstring, 0, false},
		{">", RuleInclude, ">", PatternSubstring, 0, false},
		{`\-foo`, RuleInclude, "-foo", PatternSubstring, 0, false}, // escape

		// Empty input
		{"", 0, "", 0, 0, true},
		{"   ", 0, "", 0, 0, true},

		// Invalid regex
		{"[invalid(", RuleInclude, "[invalid(", PatternRegex, 0, true},
	}

	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			r, err := ParseRuleInput(c.input)
			if c.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, c.wantKind, r.Kind())

			switch rr := r.(type) {
			case *PatternRule:
				assert.Equal(t, c.wantPat, rr.Pattern)
				assert.Equal(t, c.wantMode, rr.Mode)
			case SeverityRule:
				assert.Equal(t, c.wantSev, rr.Floor)
			}
		})
	}
}

func TestBuildVisibleIndices(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	lines := []string{
		"[INFO] starting",
		"GET /healthz 200",
		"[ERROR] db dropped",
		"GET /api 200",
		"[WARN] slow query",
	}

	mkExc := func(p string) *PatternRule {
		r, _ := NewPatternRule(p, PatternSubstring, true)
		return r
	}

	t.Run("no rules → all visible", func(t *testing.T) {
		chain := NewFilterChain(nil, IncludeAny, d)
		idx := BuildVisibleIndices(lines, chain)
		assert.Equal(t, []int{0, 1, 2, 3, 4}, idx)
	})

	t.Run("exclude /healthz", func(t *testing.T) {
		chain := NewFilterChain([]Rule{mkExc("/healthz")}, IncludeAny, d)
		idx := BuildVisibleIndices(lines, chain)
		assert.Equal(t, []int{0, 2, 3, 4}, idx)
	})

	t.Run("severity floor warn", func(t *testing.T) {
		chain := NewFilterChain([]Rule{SeverityRule{SeverityWarn}}, IncludeAny, d)
		idx := BuildVisibleIndices(lines, chain)
		// Lines 1 and 3 are Unknown (kept), line 0 is Info (dropped),
		// lines 2 and 4 are Error/Warn (kept).
		assert.Equal(t, []int{1, 2, 3, 4}, idx)
	})

	t.Run("empty buffer", func(t *testing.T) {
		chain := NewFilterChain(nil, IncludeAny, d)
		idx := BuildVisibleIndices(nil, chain)
		assert.Equal(t, []int{}, idx)
	})
}

// TestFilterChainFieldRuleDropsNonJSON exercises the hard-gate
// semantics: the moment any FieldRule lands in the chain, every
// non-JSON line is dropped regardless of whether the line would
// otherwise match any include / exclude rule. This is the documented
// trade-off for structured filtering.
func TestFilterChainFieldRuleDropsNonJSON(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	lines := []string{
		`{"level":"error","msg":"db dropped"}`,
		`GET /api 200`, // not JSON — must be dropped with a field rule active
		`{"level":"info","msg":"ok"}`,
		`plain text log`, // also dropped
	}

	fr, err := NewFieldRule([]string{"level"}, false, FieldOpEq, "error")
	require.NoError(t, err)
	chain := NewFilterChain([]Rule{fr}, IncludeAny, d)

	indices := BuildVisibleIndices(lines, chain)
	// Only the JSON line whose level is "error" should survive.
	assert.Equal(t, []int{0}, indices)
}

// TestFilterChainFieldRuleMatchesOnlyStructured validates the positive
// case: a field rule correctly routes through includes and accepts the
// matching JSON line.
func TestFilterChainFieldRuleMatchesOnlyStructured(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	lines := []string{
		`{"level":"error","user_id":42}`,
		`{"level":"warn","user_id":10}`,
		`{"level":"error","user_id":100}`,
		`plain text`, // dropped by the field-rule gate
	}

	fr, err := NewFieldRule([]string{"user_id"}, false, FieldOpGt, "50")
	require.NoError(t, err)
	chain := NewFilterChain([]Rule{fr}, IncludeAny, d)

	indices := BuildVisibleIndices(lines, chain)
	assert.Equal(t, []int{2}, indices)
}

// TestFilterChainFieldRuleWithGroupAndIncludes validates that a
// FieldRule combines with a PatternRule inside a group, and that the
// field-rule gate still drops non-JSON lines even when the group
// would otherwise have matched through a plain-text predicate.
func TestFilterChainFieldRuleWithGroupAndIncludes(t *testing.T) {
	d, _ := newSeverityDetector(nil)

	// Rules: (.level=error AND "db") — an ALL-group with a field rule
	// and a substring pattern. The substring must be present AND the
	// field must match. Non-JSON lines are dropped by the top-level
	// field-rule gate before the group evaluates.
	fr, err := NewFieldRule([]string{"level"}, false, FieldOpEq, "error")
	require.NoError(t, err)
	pr, err := NewPatternRule("db", PatternSubstring, false)
	require.NoError(t, err)
	group := &GroupRule{
		Mode:     IncludeAll,
		Children: []Rule{fr, pr},
	}
	chain := NewFilterChain([]Rule{group}, IncludeAny, d)

	cases := []struct {
		name string
		line string
		keep bool
	}{
		{
			name: "JSON error with db mention",
			line: `{"level":"error","msg":"db dropped"}`,
			keep: true,
		},
		{
			name: "JSON error without db mention",
			line: `{"level":"error","msg":"cache cold"}`,
			keep: false,
		},
		{
			name: "JSON warn with db mention fails level predicate",
			line: `{"level":"warn","msg":"db slow"}`,
			keep: false,
		},
		{
			name: "plain text with db mention dropped by field-rule gate",
			line: `ERROR db dropped`,
			keep: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.keep, chain.Keep(c.line))
		})
	}
}

// TestFilterChainFieldRuleCombinesWithSeverity checks that a severity
// floor and a field rule cooperate without interfering: severity is
// evaluated before the field-rule gate, so a JSON line that fails the
// severity predicate is dropped without parsing JSON twice.
//
// Severity is detected from the JSON payload itself — the default
// severity patterns recognise `"level":"error"` inside a JSON line, so
// we can combine severity and field predicates on the same line.
func TestFilterChainFieldRuleCombinesWithSeverity(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	fr, err := NewFieldRule([]string{"user_id"}, false, FieldOpGt, "0")
	require.NoError(t, err)
	chain := NewFilterChain([]Rule{
		SeverityRule{Floor: SeverityError},
		fr,
	}, IncludeAny, d)

	// JSON error line with user_id > 0 → keep.
	assert.True(t, chain.Keep(`{"level":"error","user_id":42,"msg":"db dropped"}`))
	// JSON info line → dropped by severity floor.
	assert.False(t, chain.Keep(`{"level":"info","user_id":42,"msg":"ok"}`))
	// Non-JSON line with ERROR severity → dropped by field-rule gate.
	assert.False(t, chain.Keep(`[ERROR] plain text`))
}
