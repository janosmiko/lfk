package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeverityOrdering(t *testing.T) {
	assert.True(t, SeverityDebug < SeverityInfo)
	assert.True(t, SeverityInfo < SeverityWarn)
	assert.True(t, SeverityWarn < SeverityError)
	assert.True(t, SeverityUnknown < SeverityDebug, "Unknown < Debug for monotonic ordering")
}

func TestSeverityString(t *testing.T) {
	cases := []struct {
		s    Severity
		want string
	}{
		{SeverityUnknown, "UNKNOWN"},
		{SeverityDebug, "DEBUG"},
		{SeverityInfo, "INFO"},
		{SeverityWarn, "WARN"},
		{SeverityError, "ERROR"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, c.s.String(), "severity %d", c.s)
	}
}

func TestDefaultSeverityPatternsCover(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	cases := []struct {
		line string
		want Severity
	}{
		// Error formats
		{"2024-01-15T10:30:00Z [ERROR] something went wrong", SeverityError},
		{"level=error msg=\"db connection lost\"", SeverityError},
		{`{"level":"error","msg":"x"}`, SeverityError},
		{"FATAL: corrupted state", SeverityError},
		{"runtime ERR: stack overflow", SeverityError},

		// Warn
		{"WARNING: deprecated call", SeverityWarn},
		{"[WARN] retry exceeded", SeverityWarn},
		{"level=warn msg=high latency", SeverityWarn},
		{`{"level":"warn"}`, SeverityWarn},

		// Info
		{"INFO starting up", SeverityInfo},
		{"[INFO] ready", SeverityInfo},
		{"level=info connected", SeverityInfo},

		// Debug
		{"DEBUG cache hit", SeverityDebug},
		{"TRACE x=1", SeverityDebug},
		{"level=debug noisy", SeverityDebug},

		// I-1: TRACE pattern must not match the word "trace" inside
		// phrases like "stack trace:" — that would mis-classify
		// stack traces as Debug and hide them under >=ERROR filters.
		{"stack trace: foo", SeverityUnknown},

		// I-2: ERROR/ERR must not classify prose mentions of the word
		// "error" as Error. Both patterns now require structured-log
		// shape (ERR: / ERROR=). An INFO line that mentions an error
		// in prose stays Info because INFO is a present level marker.
		{"INFO: handled error gracefully", SeverityInfo},
		// Without any level marker, prose mentioning "error" stays
		// Unknown — no pattern matches.
		{"no error count today", SeverityUnknown},
		// Same prose-mention tightening for FATAL — bare prose "fatal"
		// no longer matches; structured-log shape (FATAL: / FATAL=) required.
		{"a fatal mistake happened", SeverityUnknown},

		// Order-dependence lock-down: when both error and info markers
		// are present, the first matching pattern wins. ERROR patterns
		// are evaluated before INFO, so this classifies as Error.
		{"level=error and level=info", SeverityError},

		// Unknown / no match
		{"server is running on port 8080", SeverityUnknown},
		{"PANTHER", SeverityUnknown}, // word-boundary: shouldn't match ERROR
		{"", SeverityUnknown},
	}
	for _, c := range cases {
		got := d.Detect(c.line)
		assert.Equal(t, c.want, got, "line=%q", c.line)
	}
}

// TestSeverityCustomPatternInvalidRegexReturnsError verifies that
// newSeverityDetector surfaces compile errors for user-supplied patterns
// instead of silently dropping them. The detector is still created with
// the valid default patterns so the loader can choose how to handle
// partial-failure cases.
func TestSeverityCustomPatternInvalidRegexReturnsError(t *testing.T) {
	extras := map[Severity][]string{
		SeverityError: {"[invalid("},
	}
	d, errs := newSeverityDetector(extras)
	assert.NotNil(t, d, "detector still created with valid defaults")
	assert.Len(t, errs, 1, "one error for the invalid pattern")
	assert.Contains(t, errs[0].Error(), "[invalid(", "error mentions the bad pattern source")
}

// TestSeverityWordBoundaryEdgeCases locks in the actual current behavior
// of the tightened severity patterns introduced during A.2 review feedback.
// The goal is regression coverage of how the patterns interact with word
// boundaries and structured-log shape requirements (KEY: / KEY=), not to
// enforce a specific contract — adjust expectations if patterns change.
func TestSeverityWordBoundaryEdgeCases(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	cases := []struct {
		line string
		want Severity
	}{
		// `\bERROR\s*[:=]` requires `:` or `=` immediately (with optional
		// whitespace) after ERROR. `_LOG_LEVEL=5` puts `_` (letter-class)
		// after ERROR, breaking the structured-log shape, so no match.
		{"ERROR_LOG_LEVEL=5", SeverityUnknown},
		// "ERROR" appears as a substring inside PANTHER but there is no
		// word boundary before it, so `\bERROR` cannot match.
		{"PANTHER", SeverityUnknown},
		// "ERROR" is preceded by "NO" (letters), so `\bERROR` cannot
		// match — no word boundary before ERROR.
		{"NOERROR", SeverityUnknown},
		// `err:` — `\bERR\s*[:=]` matches: word boundary before ERR,
		// then `:` directly after. Tightened ERR pattern still fires
		// because the structured-log shape is satisfied.
		{"err: something", SeverityError},
		// Tightened ERROR pattern requires `:` or `=` immediately after
		// ERROR. "eRrOr count" has a space, then "count", so the
		// structured-log shape is missing — no match despite case-
		// insensitive boundary match on the word itself.
		{"my eRrOr count", SeverityUnknown},
		// `\bWARN(ING)?\b` matches WARNING (the first token); first
		// match wins, so WARN classification holds.
		{"WARNING WARN", SeverityWarn},
		{"no severity here", SeverityUnknown},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, d.Detect(c.line), "line=%q", c.line)
	}
}

// TestSeverityCustomPatternsExtendDefaults locks in the contract that
// user-supplied patterns extend (do not replace) the built-in defaults:
// custom patterns classify lines that defaults wouldn't, and default
// patterns continue to classify lines untouched by the user config.
func TestSeverityCustomPatternsExtendDefaults(t *testing.T) {
	extras := map[Severity][]string{
		SeverityError: {`my-app: failed`, `opcode=0x80`},
		SeverityWarn:  {`legacy-warn-marker`},
	}
	d, _ := newSeverityDetector(extras)

	// Custom patterns matched
	assert.Equal(t, SeverityError, d.Detect("my-app: failed to bind"))
	assert.Equal(t, SeverityError, d.Detect("opcode=0x80"))
	assert.Equal(t, SeverityWarn, d.Detect("legacy-warn-marker triggered"))

	// Defaults still active
	assert.Equal(t, SeverityError, d.Detect("[ERROR] still works"))
	assert.Equal(t, SeverityInfo, d.Detect("level=info"))
}

func TestSeverityDetectsPrefixedLines(t *testing.T) {
	d, _ := newSeverityDetector(nil)
	assert.Equal(t, SeverityError, d.Detect("[pod/api-7f4/server] [ERROR] boom"))
	assert.Equal(t, SeverityInfo, d.Detect("[pod/web-2x9/nginx] INFO ready"))
	// Prefix only, no level marker
	assert.Equal(t, SeverityUnknown, d.Detect("[pod/api/server] hello"))
}

// TestSeverityDetectsKlogFormat verifies detection of the klog/glog format
// used by Kubernetes core components (cluster-autoscaler, coredns,
// kube-controller-manager, kube-proxy, etc.) — `L<MMDD> HH:MM:SS.µs ...`
// where L is I/W/E/F.
func TestSeverityDetectsKlogFormat(t *testing.T) {
	d, _ := newSeverityDetector(nil)

	cases := []struct {
		name string
		line string
		want Severity
	}{
		{"klog info at start", "I0412 12:34:56.789012       1 main.go:123] Starting", SeverityInfo},
		{"klog warn at start", "W0412 12:34:56.789012       1 main.go:123] degraded", SeverityWarn},
		{"klog error at start", "E0412 12:34:56.789012       1 main.go:123] failed", SeverityError},
		{"klog fatal at start", "F0412 12:34:56.789012       1 main.go:123] bye", SeverityError},

		// kubectl --timestamps prepends RFC3339; klog marker sits after a space.
		{"klog info after ts", "2026-04-16T10:20:30.123456789Z I0412 12:34:56.789012       1 s.go:1] info", SeverityInfo},
		{"klog error after ts", "2026-04-16T10:20:30.123456789Z E0412 12:34:56.789012       1 s.go:1] err", SeverityError},

		// With pod-prefix (multi-container --prefix) → stripped, then klog matched.
		{"klog warn pod-prefixed", "[kube-system/cluster-autoscaler-abc/cluster-autoscaler] W0412 10:00:00.000001       1 main.go:1] careful", SeverityWarn},

		// Avoid false positives — word boundary required.
		{"letters inside word are not klog", "problemW0412inside", SeverityUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, d.Detect(c.line))
		})
	}
}
