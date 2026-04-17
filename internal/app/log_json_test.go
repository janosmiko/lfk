package app

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectJSONLine(t *testing.T) {
	cases := []struct {
		name       string
		line       string
		wantIsJSON bool
		wantPay    string // expected payload (only meaningful when wantIsJSON)
	}{
		{
			name:       "plain object",
			line:       `{"level":"info","msg":"hello"}`,
			wantIsJSON: true,
			wantPay:    `{"level":"info","msg":"hello"}`,
		},
		{
			name:       "plain array",
			line:       `[1,2,3]`,
			wantIsJSON: true,
			wantPay:    `[1,2,3]`,
		},
		{
			name:       "nested object",
			line:       `{"a":{"b":{"c":[1,2,{"d":true}]}}}`,
			wantIsJSON: true,
			wantPay:    `{"a":{"b":{"c":[1,2,{"d":true}]}}}`,
		},
		{
			name:       "pod-prefixed object",
			line:       `[mypod/mycontainer] {"level":"info","msg":"hi"}`,
			wantIsJSON: true,
			wantPay:    `{"level":"info","msg":"hi"}`,
		},
		{
			name:       "rfc3339 prefixed",
			line:       `2024-01-15T10:30:00.123456789Z {"a":1}`,
			wantIsJSON: true,
			wantPay:    `{"a":1}`,
		},
		{
			name:       "rfc3339 no nanos",
			line:       `2024-01-15T10:30:00Z {"a":1}`,
			wantIsJSON: true,
			wantPay:    `{"a":1}`,
		},
		{
			name:       "rfc3339 offset",
			line:       `2024-01-15T10:30:00+02:00 {"a":1}`,
			wantIsJSON: true,
			wantPay:    `{"a":1}`,
		},
		{
			name:       "pod prefix then rfc3339",
			line:       `[pod-a/web] 2024-01-15T10:30:00.000Z {"x":1}`,
			wantIsJSON: true,
			wantPay:    `{"x":1}`,
		},
		{
			name:       "klog prefixed",
			line:       `I0412 12:34:56.789012       1 main.go:123] {"msg":"hi"}`,
			wantIsJSON: true,
			wantPay:    `{"msg":"hi"}`,
		},
		{
			name:       "rfc3339 then klog then json",
			line:       `2024-01-15T10:30:00Z I0412 12:34:56.789012       1 main.go:123] {"msg":"hi"}`,
			wantIsJSON: true,
			wantPay:    `{"msg":"hi"}`,
		},
		{
			name:       "trailing whitespace",
			line:       `{"a":1}   `,
			wantIsJSON: true,
			wantPay:    `{"a":1}   `,
		},
		{
			name:       "empty string",
			line:       ``,
			wantIsJSON: false,
		},
		{
			name:       "plain text",
			line:       `hello world`,
			wantIsJSON: false,
		},
		{
			name:       "warn bracketed level marker not json",
			line:       `[WARN] hello world`,
			wantIsJSON: false,
		},
		{
			name:       "half json",
			line:       `{"foo":`,
			wantIsJSON: false,
		},
		{
			name:       "json then trailing text",
			line:       `{"a":1} extra`,
			wantIsJSON: false,
		},
		{
			name:       "unmatched brackets",
			line:       `{}bracket text}`,
			wantIsJSON: false,
		},
		{
			name:       "bare number rejected",
			line:       `42`,
			wantIsJSON: false,
		},
		{
			name:       "bare string rejected",
			line:       `"hello"`,
			wantIsJSON: false,
		},
		{
			name:       "bare null rejected",
			line:       `null`,
			wantIsJSON: false,
		},
		{
			name:       "single brace too short",
			line:       `{`,
			wantIsJSON: false,
		},
		{
			name:       "object missing close",
			line:       `{"a":1`,
			wantIsJSON: false,
		},
		{
			name:       "array missing close",
			line:       `[1,2`,
			wantIsJSON: false,
		},
		{
			name:       "object then array",
			line:       `{"a":1}[1]`,
			wantIsJSON: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DetectJSONLine(c.line)
			assert.Equal(t, c.wantIsJSON, got.IsJSON, "IsJSON for %q", c.line)
			if c.wantIsJSON {
				assert.Equal(t, c.wantPay, got.Payload, "Payload for %q", c.line)
				require.NotNil(t, got.Value, "Value for %q", c.line)
				switch got.Value.(type) {
				case map[string]any, []any:
					// ok
				default:
					t.Fatalf("unexpected Value type %T for %q", got.Value, c.line)
				}
			} else {
				assert.Nil(t, got.Value, "Value for non-JSON line %q", c.line)
				assert.Empty(t, got.Payload, "Payload for non-JSON line %q", c.line)
			}
		})
	}
}

// TestDetectJSONLineUsesNumber verifies large integers are preserved as
// json.Number so field filters comparing big ints don't drop precision
// through a float64 round-trip.
func TestDetectJSONLineUsesNumber(t *testing.T) {
	const raw = `{"n":9007199254740999}`
	j := DetectJSONLine(raw)
	require.True(t, j.IsJSON)
	obj, ok := j.Value.(map[string]any)
	require.True(t, ok, "expected object, got %T", j.Value)

	num, ok := obj["n"].(json.Number)
	require.True(t, ok, "expected json.Number, got %T", obj["n"])
	assert.Equal(t, "9007199254740999", num.String(),
		"json.Number must preserve the exact integer representation")
}

// TestDetectJSONLineBenchmarkFastPath ensures the cheap gates bail out
// before decode is even attempted. There is no hard assertion to make
// without microbenchmarking here, but we at least exercise the path and
// confirm correctness on a realistic payload length.
func TestDetectJSONLineNestedLarge(t *testing.T) {
	line := `{"level":"info","ts":"2024-01-15T10:30:00Z","msg":"request served","attrs":{"status":200,"latency_ms":42,"paths":["/a","/b","/c"]}}`
	j := DetectJSONLine(line)
	require.True(t, j.IsJSON)
	obj, ok := j.Value.(map[string]any)
	require.True(t, ok)
	attrs, ok := obj["attrs"].(map[string]any)
	require.True(t, ok)
	paths, ok := attrs["paths"].([]any)
	require.True(t, ok)
	assert.Len(t, paths, 3)
}

func TestStripRFC3339Prefix(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"2024-01-15T10:30:00Z hello", "hello"},
		{"2024-01-15T10:30:00.000000000Z hello", "hello"},
		{"2024-01-15T10:30:00+02:00 hello", "hello"},
		{"2024-01-15T10:30:00-07:30 hello", "hello"},
		{"bogus prefix", "bogus prefix"},
		{"2024-01-15T10:30:00", "2024-01-15T10:30:00"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, stripRFC3339Prefix(c.in), "input %q", c.in)
	}
}

func TestStripKlogPrefix(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{
			in:   `I0412 12:34:56.789012       1 main.go:123] payload`,
			want: `payload`,
		},
		{
			in:   `W0412 12:34:56.789012 1 main.go:1] {"k":1}`,
			want: `{"k":1}`,
		},
		{
			in:   `E0412 12:34:56.789012       1 main.go:123] oops`,
			want: `oops`,
		},
		{
			// Not klog: wrong first letter.
			in:   `X0412 12:34:56.789012       1 main.go:123] payload`,
			want: `X0412 12:34:56.789012       1 main.go:123] payload`,
		},
		{
			// Not klog: non-digits in MMDD.
			in:   `IABCD 12:34:56.789012       1 main.go:123] payload`,
			want: `IABCD 12:34:56.789012       1 main.go:123] payload`,
		},
		{
			in:   `regular text with ] no klog`,
			want: `regular text with ] no klog`,
		},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, stripKlogPrefix(c.in), "input %q", c.in)
	}
}

func TestStripPodContainerPrefix(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{`[pod/web] hello`, `hello`},
		{`[my-pod/my-container] {"a":1}`, `{"a":1}`},
		{`[WARN] bracketed level keeps`, `[WARN] bracketed level keeps`}, // no slash → don't strip
		{`no bracket`, `no bracket`},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, stripPodContainerPrefix(c.in), "input %q", c.in)
	}
}
