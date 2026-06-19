package app

import (
	"strconv"
	"strings"

	"github.com/janosmiko/lfk/internal/logagg"
)

type logTopFilterTerm struct {
	field string // "" for a bare free-text term
	op    string // "=", "!=", "~", ">=", "<=", ">", "<"; "" for free-text
	value string
}

// parseLogTopFilter splits the query into space-separated ANDed terms.
func parseLogTopFilter(q string) []logTopFilterTerm {
	var terms []logTopFilterTerm
	for tok := range strings.FieldsSeq(q) {
		field, op, value := splitFilterToken(tok)
		terms = append(terms, logTopFilterTerm{field: field, op: op, value: value})
	}
	return terms
}

// splitFilterToken finds the first operator in tok and returns (field, op, value).
// 2-char ops are checked before 1-char so ">=" is not mis-split as ">". The part
// before the operator must be a valid field name (letters/digits/_.:-); otherwise
// the token is free-text. This keeps a bare ">=500" (no field) as free-text rather
// than a phantom field term that matches nothing.
func splitFilterToken(tok string) (field, op, value string) {
	for _, o := range []string{">=", "<=", "!=", "=", ">", "<", "~"} {
		if i := strings.Index(tok, o); i > 0 && isFilterFieldName(tok[:i]) {
			return tok[:i], o, tok[i+len(o):]
		}
	}
	return "", "", tok
}

// isFilterFieldName reports whether s is a plausible field name: non-empty and
// composed only of letters, digits, and _ . : - (no operator characters).
func isFilterFieldName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '_' || r == '.' || r == ':' || r == '-':
		default:
			return false
		}
	}
	return true
}

// logTopFilterMatch reports whether a parsed line satisfies all terms.
// When terms is empty, it always returns true.
func logTopFilterMatch(f logagg.Fields, terms []logTopFilterTerm) bool {
	for _, t := range terms {
		if !t.matches(f) {
			return false
		}
	}
	return true
}

func (t logTopFilterTerm) matches(f logagg.Fields) bool {
	if t.op == "" {
		// bare free-text: any field contains value (case-insensitive)
		low := strings.ToLower(t.value)
		for _, v := range f {
			if strings.Contains(strings.ToLower(v), low) {
				return true
			}
		}
		return false
	}
	fv := f[t.field]
	switch t.op {
	case "=":
		return strings.EqualFold(fv, t.value)
	case "!=":
		return !strings.EqualFold(fv, t.value)
	case "~":
		return strings.Contains(strings.ToLower(fv), strings.ToLower(t.value))
	case ">=", "<=", ">", "<":
		a, errA := strconv.ParseFloat(fv, 64)
		b, errB := strconv.ParseFloat(t.value, 64)
		if errA != nil || errB != nil {
			return false
		}
		switch t.op {
		case ">=":
			return a >= b
		case "<=":
			return a <= b
		case ">":
			return a > b
		case "<":
			return a < b
		}
	}
	return false
}
