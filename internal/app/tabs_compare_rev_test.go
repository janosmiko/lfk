package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/model"
)

func TestComparePrimaryColumn_REV(t *testing.T) {
	mkItem := func(rev string) model.Item {
		return model.Item{Columns: []model.KeyValue{{Key: "REV", Value: rev}}}
	}
	tests := []struct {
		name string
		a, b string
		want int // sign only
	}{
		{name: "a less", a: "10", b: "100", want: -1},
		{name: "discriminates from lex", a: "9", b: "10", want: -1}, // lex says +1
		{name: "a greater", a: "65535", b: "16", want: 1},
		{name: "equal", a: "42", b: "42", want: 0},
		{name: "non-numeric falls back to lex", a: "abc", b: "abd", want: -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := comparePrimaryColumn(mkItem(tt.a), mkItem(tt.b), "REV")
			if (got < 0) != (tt.want < 0) || (got > 0) != (tt.want > 0) {
				t.Fatalf("comparePrimaryColumn(%q,%q,REV) = %d, want sign %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
