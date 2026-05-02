package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCountPrefix(t *testing.T) {
	cases := []struct {
		name string
		buf  string
		want int
	}{
		{"empty buffer falls back to 1", "", 1},
		{"single digit", "5", 5},
		{"multi digit", "42", 42},
		{"non-numeric falls back to 1", "abc", 1},
		{"zero falls back to 1", "0", 1},
		{"negative-looking string falls back to 1", "-3", 1},
		{"value at cap is preserved", "1000000", maxCountPrefix},
		{"value above cap saturates", "9999999999", maxCountPrefix},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, parseCountPrefix(tc.buf))
		})
	}
}

// consumeCountPrefix must both parse the buffer and clear it in one step —
// every caller relies on the buffer being empty after the command so the
// digits don't leak into the next one.
func TestConsumeCountPrefix(t *testing.T) {
	cases := []struct {
		name      string
		buf       string
		wantCount int
	}{
		{"empty buffer falls back to 1", "", 1},
		{"valid count is parsed", "5", 5},
		{"multi digit", "42", 42},
		{"non-numeric falls back to 1", "abc", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := tc.buf
			n := consumeCountPrefix(&buf)
			assert.Equal(t, tc.wantCount, n)
			assert.Empty(t, buf, "buffer must be cleared after consume")
		})
	}
}
