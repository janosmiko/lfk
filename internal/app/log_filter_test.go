package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
