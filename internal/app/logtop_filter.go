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
// 2-char ops are checked before 1-char to avoid mis-splitting ">=" as ">".
// If no operator is found or the field part is empty, returns ("", "", tok) for free-text.
func splitFilterToken(tok string) (field, op, value string) {
	twoCharOps := []string{">=", "<=", "!="}
	for _, o := range twoCharOps {
		if i := strings.Index(tok, o); i > 0 {
			return tok[:i], o, tok[i+len(o):]
		}
	}
	oneCharOps := []string{"=", ">", "<", "~"}
	for _, o := range oneCharOps {
		if i := strings.Index(tok, o); i > 0 {
			return tok[:i], o, tok[i+len(o):]
		}
	}
	return "", "", tok
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
