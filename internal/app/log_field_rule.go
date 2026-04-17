package app

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// FieldOp is the comparison operator for a FieldRule.
type FieldOp int

const (
	// FieldOpEq tests whether the field equals Value. String comparison
	// is case-insensitive; numeric comparison uses json.Number when both
	// sides parse as numbers.
	FieldOpEq FieldOp = iota
	// FieldOpNeq is the negation of FieldOpEq.
	FieldOpNeq
	// FieldOpGt / Gte / Lt / Lte are numeric comparisons. They only
	// succeed when both the field value and Value parse as numbers; a
	// non-numeric field fails the comparison rather than falling back
	// to lexical order (which surprises users more often than it helps).
	FieldOpGt
	FieldOpGte
	FieldOpLt
	FieldOpLte
	// FieldOpMatch is a regex-or-substring match against the field's
	// stringified value, reusing the same heuristic as NewPatternRule.
	FieldOpMatch
)

// String returns the symbolic representation of the op (as it would
// appear in the input syntax).
func (o FieldOp) String() string {
	switch o {
	case FieldOpEq:
		return "="
	case FieldOpNeq:
		return "!="
	case FieldOpGt:
		return ">"
	case FieldOpGte:
		return ">="
	case FieldOpLt:
		return "<"
	case FieldOpLte:
		return "<="
	case FieldOpMatch:
		return "~"
	}
	return "?"
}

// serialisedOp returns the serialised form used in preset YAML.
func (o FieldOp) serialisedOp() string {
	switch o {
	case FieldOpEq:
		return "eq"
	case FieldOpNeq:
		return "neq"
	case FieldOpGt:
		return "gt"
	case FieldOpGte:
		return "gte"
	case FieldOpLt:
		return "lt"
	case FieldOpLte:
		return "lte"
	case FieldOpMatch:
		return "match"
	}
	return ""
}

// parseFieldOp maps the preset-YAML string form back to a FieldOp.
func parseFieldOp(s string) (FieldOp, error) {
	switch s {
	case "eq":
		return FieldOpEq, nil
	case "neq":
		return FieldOpNeq, nil
	case "gt":
		return FieldOpGt, nil
	case "gte":
		return FieldOpGte, nil
	case "lt":
		return FieldOpLt, nil
	case "lte":
		return FieldOpLte, nil
	case "match":
		return FieldOpMatch, nil
	}
	return FieldOpEq, fmt.Errorf("unknown field op %q", s)
}

// FieldRule matches JSON log lines whose nested field (dotted path)
// compares with Value per Op.
//
// Semantics:
//   - On non-JSON lines, MatchesJSON is never called (FilterChain drops
//     such lines as a hard gate when any FieldRule is active).
//   - Missing paths evaluate to false for every op — the field isn't
//     there, so no comparison can hold.
//   - Equality on strings is case-insensitive; numeric comparisons use
//     json.Number so large integers don't lose precision.
//   - ArrayAny makes the field a collection walker: the comparison holds
//     when ANY element of the resolved array matches. Resolving a scalar
//     in ArrayAny mode treats it as a one-element array, so `.tags[]=x`
//     also matches when tags is the bare string "x".
type FieldRule struct {
	// Path is the dotted path to the field, split on '.'. For example
	// "user.id" is stored as ["user", "id"]. A single-element path
	// targets a top-level key.
	Path []string
	// ArrayAny is true when the input syntax ended the field segment
	// with "[]" — e.g. `.tags[]=api`. The comparison becomes "any
	// element of this array matches".
	ArrayAny bool
	// Op is the comparison operator.
	Op FieldOp
	// Value is the raw right-hand side from the input syntax. Numeric
	// ops parse this as a json.Number on demand.
	Value string

	// re is the compiled regex for Op=~ with metacharacters.
	re *regexp.Regexp
	// lowerValue caches strings.ToLower(Value) for case-insensitive eq/neq.
	lowerValue string
}

// NewFieldRule constructs a FieldRule, validating the inputs and pre-
// compiling the regex for Op=FieldOpMatch when the value looks like
// a regex (same auto-detection as NewPatternRule).
func NewFieldRule(path []string, arrayAny bool, op FieldOp, value string) (*FieldRule, error) {
	if len(path) == 0 {
		return nil, fmt.Errorf("field rule requires a non-empty path")
	}
	for _, seg := range path {
		if seg == "" {
			return nil, fmt.Errorf("field rule path contains an empty segment")
		}
	}
	r := &FieldRule{
		Path:       append([]string(nil), path...),
		ArrayAny:   arrayAny,
		Op:         op,
		Value:      value,
		lowerValue: strings.ToLower(value),
	}
	if op == FieldOpMatch && looksLikeRegex(value) {
		re, err := regexp.Compile("(?i)" + value)
		if err != nil {
			return nil, fmt.Errorf("invalid regex %q: %w", value, err)
		}
		r.re = re
	}
	return r, nil
}

// Kind returns RuleField.
func (r *FieldRule) Kind() RuleKind { return RuleField }

// Display returns the rule rendered in its input syntax — handy both
// for the rules table and for edit-mode round-trips. For ArrayAny rules
// the "[]" suffix is preserved (`.tags[]=api`).
func (r *FieldRule) Display() string {
	path := strings.Join(r.Path, ".")
	suffix := ""
	if r.ArrayAny {
		suffix = "[]"
	}
	return fmt.Sprintf(".%s%s %s %s", path, suffix, r.Op.String(), r.Value)
}

// MatchesJSON takes the parsed top-level JSON value (as returned by
// DetectJSONLine) and returns true when the rule's field comparison
// holds. Returns false on non-JSON values (nil or primitives), on
// missing paths, and on type mismatches (e.g. numeric ops against
// a string field).
func (r *FieldRule) MatchesJSON(v any) bool {
	if v == nil {
		return false
	}
	resolved, ok := resolveFieldPath(v, r.Path)
	if !ok {
		return false
	}
	if r.ArrayAny {
		return r.matchAny(resolved)
	}
	return r.matchScalar(resolved)
}

// matchAny is the ArrayAny evaluator: the rule holds when any element
// of resolved satisfies matchScalar. Scalars are treated as one-element
// arrays for ergonomic `tags[]` syntax against either an array or a
// bare scalar field.
func (r *FieldRule) matchAny(resolved any) bool {
	arr, ok := resolved.([]any)
	if !ok {
		return r.matchScalar(resolved)
	}
	for _, elem := range arr {
		if r.matchScalar(elem) {
			return true
		}
	}
	return false
}

// matchScalar compares a single resolved value against Value using Op.
// Non-matching types for a given op return false rather than erroring.
func (r *FieldRule) matchScalar(v any) bool {
	switch r.Op {
	case FieldOpEq:
		return equalsFieldValue(v, r.Value, r.lowerValue)
	case FieldOpNeq:
		return !equalsFieldValue(v, r.Value, r.lowerValue)
	case FieldOpGt, FieldOpGte, FieldOpLt, FieldOpLte:
		return compareFieldNumeric(v, r.Value, r.Op)
	case FieldOpMatch:
		return matchFieldValue(v, r)
	}
	return false
}

// resolveFieldPath walks v down the dotted path; each segment must be a
// key lookup in a JSON object. Returns (value, true) on success and
// (nil, false) when any segment is missing or the intermediate value
// isn't an object.
func resolveFieldPath(v any, path []string) (any, bool) {
	cur := v
	for _, seg := range path {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		nxt, present := obj[seg]
		if !present {
			return nil, false
		}
		cur = nxt
	}
	return cur, true
}

// equalsFieldValue compares the JSON value v against the raw string s
// (and its lowercase form lower). Strings compare case-insensitively;
// numbers round-trip through json.Number so large integers stay exact;
// booleans match the canonical "true"/"false" spellings; null matches
// "null".
func equalsFieldValue(v any, s, lower string) bool {
	switch x := v.(type) {
	case string:
		return strings.EqualFold(x, s)
	case json.Number:
		// Numeric equality: try parsing s as a number first, fall back
		// to exact string comparison when s isn't numeric.
		if sNum, ok := asFieldNumber(s); ok {
			if xNum, ok2 := asFieldNumber(x.String()); ok2 {
				return xNum == sNum
			}
		}
		return x.String() == s
	case bool:
		return (x && lower == "true") || (!x && lower == "false")
	case nil:
		return lower == "null"
	}
	return false
}

// compareFieldNumeric performs a numeric comparison v OP s where OP is
// one of gt/gte/lt/lte. Returns false when either side isn't numeric.
func compareFieldNumeric(v any, s string, op FieldOp) bool {
	sNum, ok := asFieldNumber(s)
	if !ok {
		return false
	}
	var xStr string
	switch x := v.(type) {
	case json.Number:
		xStr = x.String()
	case string:
		xStr = x
	default:
		return false
	}
	xNum, ok := asFieldNumber(xStr)
	if !ok {
		return false
	}
	switch op {
	case FieldOpGt:
		return xNum > sNum
	case FieldOpGte:
		return xNum >= sNum
	case FieldOpLt:
		return xNum < sNum
	case FieldOpLte:
		return xNum <= sNum
	}
	return false
}

// matchFieldValue runs the FieldOpMatch comparison against v, coercing v
// to a string first (numbers, booleans, and nil each have a natural
// textual form).
func matchFieldValue(v any, r *FieldRule) bool {
	s := fieldValueToString(v)
	if r.re != nil {
		return r.re.MatchString(s)
	}
	return strings.Contains(strings.ToLower(s), r.lowerValue)
}

// fieldValueToString coerces a JSON value to its canonical string form
// for the match/contains operators. Object/array values render through
// json.Marshal so regex like ".*foo.*" can still meaningfully apply.
func fieldValueToString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return x.String()
	case bool:
		if x {
			return "true"
		}
		return "false"
	case nil:
		return "null"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

// asFieldNumber parses s as a float64 using json.Number's conventions.
// Empty strings and non-numeric values return (0, false).
func asFieldNumber(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	n := json.Number(s)
	f, err := n.Float64()
	if err != nil {
		return 0, false
	}
	return f, true
}
