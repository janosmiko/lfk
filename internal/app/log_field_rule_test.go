package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseFieldJSON is a small helper for tests — returns the parsed JSON
// value by running DetectJSONLine (so we consistently get json.Number
// for numeric fields, matching what the production hot path sees).
func parseFieldJSON(t *testing.T, line string) any {
	t.Helper()
	j := DetectJSONLine(line)
	require.True(t, j.IsJSON, "expected %q to parse as JSON", line)
	return j.Value
}

func TestFieldRuleKindAndDisplay(t *testing.T) {
	r, err := NewFieldRule([]string{"level"}, false, FieldOpEq, "error")
	require.NoError(t, err)
	assert.Equal(t, RuleField, r.Kind())
	assert.Equal(t, ".level = error", r.Display())

	r2, err := NewFieldRule([]string{"user", "id"}, false, FieldOpGt, "42")
	require.NoError(t, err)
	assert.Equal(t, ".user.id > 42", r2.Display())

	r3, err := NewFieldRule([]string{"tags"}, true, FieldOpEq, "api")
	require.NoError(t, err)
	assert.Equal(t, ".tags[] = api", r3.Display())
}

func TestFieldRuleEq(t *testing.T) {
	v := parseFieldJSON(t, `{"level":"error","msg":"boom"}`)

	cases := []struct {
		name     string
		path     []string
		value    string
		wantPass bool
	}{
		{"string match", []string{"level"}, "error", true},
		{"case-insensitive string match", []string{"level"}, "ERROR", true},
		{"string miss", []string{"level"}, "warn", false},
		{"missing field", []string{"missing"}, "x", false},
		{"nested miss on scalar", []string{"level", "x"}, "x", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, err := NewFieldRule(c.path, false, FieldOpEq, c.value)
			require.NoError(t, err)
			assert.Equal(t, c.wantPass, r.MatchesJSON(v))
		})
	}
}

func TestFieldRuleEqNumericAndBool(t *testing.T) {
	v := parseFieldJSON(t, `{"count":42,"big":9007199254740999,"ok":true,"nothing":null}`)

	cases := []struct {
		name string
		path []string
		val  string
		want bool
	}{
		{"number equal", []string{"count"}, "42", true},
		{"number unequal", []string{"count"}, "43", false},
		{"large int preserves precision", []string{"big"}, "9007199254740999", true},
		{"bool true", []string{"ok"}, "true", true},
		{"bool false miss", []string{"ok"}, "false", false},
		{"null matches 'null'", []string{"nothing"}, "null", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, err := NewFieldRule(c.path, false, FieldOpEq, c.val)
			require.NoError(t, err)
			assert.Equal(t, c.want, r.MatchesJSON(v))
		})
	}
}

func TestFieldRuleNeq(t *testing.T) {
	v := parseFieldJSON(t, `{"level":"error"}`)

	r, err := NewFieldRule([]string{"level"}, false, FieldOpNeq, "debug")
	require.NoError(t, err)
	assert.True(t, r.MatchesJSON(v))

	r2, err := NewFieldRule([]string{"level"}, false, FieldOpNeq, "error")
	require.NoError(t, err)
	assert.False(t, r2.MatchesJSON(v))

	// Missing field: neither eq nor neq holds (no value to compare).
	r3, err := NewFieldRule([]string{"missing"}, false, FieldOpNeq, "x")
	require.NoError(t, err)
	assert.False(t, r3.MatchesJSON(v))
}

func TestFieldRuleNumericCmp(t *testing.T) {
	v := parseFieldJSON(t, `{"user_id":42,"big":9007199254740999}`)

	cases := []struct {
		name string
		path []string
		op   FieldOp
		val  string
		want bool
	}{
		{"gt true", []string{"user_id"}, FieldOpGt, "41", true},
		{"gt false (eq boundary)", []string{"user_id"}, FieldOpGt, "42", false},
		{"gte boundary", []string{"user_id"}, FieldOpGte, "42", true},
		{"lt true", []string{"user_id"}, FieldOpLt, "100", true},
		{"lt false", []string{"user_id"}, FieldOpLt, "10", false},
		{"lte boundary", []string{"user_id"}, FieldOpLte, "42", true},
		{"gt non-numeric rhs fails", []string{"user_id"}, FieldOpGt, "abc", false},
		{"gt on json.Number large", []string{"big"}, FieldOpGt, "9007199254740000", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, err := NewFieldRule(c.path, false, c.op, c.val)
			require.NoError(t, err)
			assert.Equal(t, c.want, r.MatchesJSON(v))
		})
	}
}

func TestFieldRuleNumericCmpStringField(t *testing.T) {
	// Numeric comparison should still work when the field value is a
	// string that parses as a number (common when logs stringify ints).
	v := parseFieldJSON(t, `{"user_id":"42"}`)
	r, err := NewFieldRule([]string{"user_id"}, false, FieldOpGt, "41")
	require.NoError(t, err)
	assert.True(t, r.MatchesJSON(v))
}

func TestFieldRuleNumericCmpNonNumericFieldFails(t *testing.T) {
	// A string field that isn't numeric must fail numeric comparisons
	// rather than falling back to lexical order.
	v := parseFieldJSON(t, `{"level":"error"}`)
	r, err := NewFieldRule([]string{"level"}, false, FieldOpGt, "1")
	require.NoError(t, err)
	assert.False(t, r.MatchesJSON(v))
}

func TestFieldRuleNestedPath(t *testing.T) {
	v := parseFieldJSON(t, `{"user":{"id":42,"name":"alice"},"req":{"method":"GET"}}`)

	r, err := NewFieldRule([]string{"user", "id"}, false, FieldOpEq, "42")
	require.NoError(t, err)
	assert.True(t, r.MatchesJSON(v))

	r2, err := NewFieldRule([]string{"user", "name"}, false, FieldOpEq, "alice")
	require.NoError(t, err)
	assert.True(t, r2.MatchesJSON(v))

	// Path points to non-object intermediate.
	r3, err := NewFieldRule([]string{"user", "name", "deep"}, false, FieldOpEq, "x")
	require.NoError(t, err)
	assert.False(t, r3.MatchesJSON(v))
}

func TestFieldRuleArrayAny(t *testing.T) {
	v := parseFieldJSON(t, `{"tags":["api","db","cache"]}`)

	r, err := NewFieldRule([]string{"tags"}, true, FieldOpEq, "api")
	require.NoError(t, err)
	assert.True(t, r.MatchesJSON(v))

	r2, err := NewFieldRule([]string{"tags"}, true, FieldOpEq, "web")
	require.NoError(t, err)
	assert.False(t, r2.MatchesJSON(v))

	// Case-insensitive matching for strings.
	r3, err := NewFieldRule([]string{"tags"}, true, FieldOpEq, "API")
	require.NoError(t, err)
	assert.True(t, r3.MatchesJSON(v))

	// Bare scalar as a one-element "array" — handy when a tag field is
	// sometimes a list and sometimes a single value.
	vScalar := parseFieldJSON(t, `{"tags":"api"}`)
	r4, err := NewFieldRule([]string{"tags"}, true, FieldOpEq, "api")
	require.NoError(t, err)
	assert.True(t, r4.MatchesJSON(vScalar))
}

func TestFieldRuleArrayAnyNumericAndNeq(t *testing.T) {
	v := parseFieldJSON(t, `{"ports":[80,443,8080]}`)

	r, err := NewFieldRule([]string{"ports"}, true, FieldOpGt, "1000")
	require.NoError(t, err)
	assert.True(t, r.MatchesJSON(v))

	// Neq in array-any: true when ANY element isn't the value. With
	// [80,443,8080] and value 80, elements 443 and 8080 make it true.
	r2, err := NewFieldRule([]string{"ports"}, true, FieldOpNeq, "80")
	require.NoError(t, err)
	assert.True(t, r2.MatchesJSON(v))
}

func TestFieldRuleMatchRegex(t *testing.T) {
	v := parseFieldJSON(t, `{"msg":"starting up"}`)

	r, err := NewFieldRule([]string{"msg"}, false, FieldOpMatch, "^start")
	require.NoError(t, err)
	assert.True(t, r.MatchesJSON(v))

	r2, err := NewFieldRule([]string{"msg"}, false, FieldOpMatch, "^end")
	require.NoError(t, err)
	assert.False(t, r2.MatchesJSON(v))

	// Plain substring (no regex metachars): case-insensitive contains.
	r3, err := NewFieldRule([]string{"msg"}, false, FieldOpMatch, "UP")
	require.NoError(t, err)
	assert.True(t, r3.MatchesJSON(v))
}

func TestFieldRuleMatchOnNumberField(t *testing.T) {
	// Match/contains should coerce numbers to strings so patterns like
	// "42" work against {"count": 42}.
	v := parseFieldJSON(t, `{"count":42}`)
	r, err := NewFieldRule([]string{"count"}, false, FieldOpMatch, "4")
	require.NoError(t, err)
	assert.True(t, r.MatchesJSON(v))
}

func TestFieldRuleMissingPathNoMatch(t *testing.T) {
	v := parseFieldJSON(t, `{"level":"error"}`)

	ops := []FieldOp{FieldOpEq, FieldOpNeq, FieldOpGt, FieldOpGte, FieldOpLt, FieldOpLte, FieldOpMatch}
	for _, op := range ops {
		t.Run(op.String(), func(t *testing.T) {
			r, err := NewFieldRule([]string{"missing"}, false, op, "x")
			require.NoError(t, err)
			assert.False(t, r.MatchesJSON(v), "missing path should never match (op %s)", op.String())
		})
	}
}

func TestFieldRuleMatchesJSONNil(t *testing.T) {
	r, err := NewFieldRule([]string{"level"}, false, FieldOpEq, "error")
	require.NoError(t, err)
	assert.False(t, r.MatchesJSON(nil))
}

func TestNewFieldRuleValidation(t *testing.T) {
	// Empty path is rejected.
	_, err := NewFieldRule(nil, false, FieldOpEq, "x")
	assert.Error(t, err)

	// Empty segment is rejected (prevents `.a..b` parses slipping through).
	_, err = NewFieldRule([]string{"a", "", "b"}, false, FieldOpEq, "x")
	assert.Error(t, err)

	// Bad regex for FieldOpMatch.
	_, err = NewFieldRule([]string{"msg"}, false, FieldOpMatch, "[unclosed")
	assert.Error(t, err)
}

func TestFieldOpSerialise(t *testing.T) {
	cases := []struct {
		op  FieldOp
		ser string
	}{
		{FieldOpEq, "eq"},
		{FieldOpNeq, "neq"},
		{FieldOpGt, "gt"},
		{FieldOpGte, "gte"},
		{FieldOpLt, "lt"},
		{FieldOpLte, "lte"},
		{FieldOpMatch, "match"},
	}
	for _, c := range cases {
		t.Run(c.ser, func(t *testing.T) {
			assert.Equal(t, c.ser, c.op.serialisedOp())
			parsed, err := parseFieldOp(c.ser)
			assert.NoError(t, err)
			assert.Equal(t, c.op, parsed)
		})
	}
	_, err := parseFieldOp("bogus")
	assert.Error(t, err)
}

func TestRuleFieldKindString(t *testing.T) {
	assert.Equal(t, "FLD", RuleField.String())
}
