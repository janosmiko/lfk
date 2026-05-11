package ui

import (
	"strings"
	"testing"
)

// FuzzParseLogLine drives arbitrary input through ParseLogLine and its
// downstream format-specific parsers (JSON, klog, zap, nginx, envoy,
// postgres, java, logfmt). The function is documented as "never panics" —
// the fuzzer's job is to keep that promise honest as new formats are added.
func FuzzParseLogLine(f *testing.F) {
	seeds := []string{
		"",
		"plain text log line",
		`{"level":"info","msg":"hello","ts":"2024-01-15T10:30:00Z"}`,
		`I0115 10:30:00.123456    1234 server.go:42] starting controller`,
		"2024-01-15T10:30:00.000Z\tINFO\tcontroller\tReconciling\t{\"name\":\"foo\"}",
		`192.0.2.1 - - [15/Jan/2024:10:30:00 +0000] "GET /healthz HTTP/1.1" 200 2 "-" "kube-probe/1.29"`,
		`[2024-01-15T10:30:00.000Z] "GET /api HTTP/2" 200 - 0 12 5 4 "-" "curl/8.0" "abc-123" "auth" "10.0.0.1:8080"`,
		`2024-01-15 10:30:00.123 UTC [123] LOG:  database system is ready to accept connections`,
		`2024-01-15 10:30:00.123  INFO 1 --- [           main] c.example.App                  : Started App in 3.5 seconds`,
		"[pod/nginx-abc/proxy] some log body",
		`key1=value1 key2="two words" key3=42`,
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, line string) {
		got := ParseLogLine(line)

		if got.Kind < LogPreviewText || got.Kind > LogPreviewPostgres {
			t.Fatalf("ParseLogLine(%q) returned out-of-range Kind %d", line, got.Kind)
		}

		// Stripped pieces must come from the head of the input. We don't
		// reconstruct an exact prefix (the parser consumes either a space
		// or a tab between time and body), but Prefix is always a substring
		// at offset 0 and Time, when present, must appear before Body.
		if got.Prefix != "" && !strings.HasPrefix(line, got.Prefix) {
			t.Fatalf("ParseLogLine(%q): Prefix %q is not a prefix of input", line, got.Prefix)
		}
		if got.Time != "" && !strings.Contains(line, got.Time) {
			t.Fatalf("ParseLogLine(%q): Time %q not found in input", line, got.Time)
		}

		// Fields are extracted verbatim from the source (JSON allows
		// `{"":""}`, logfmt allows weird shapes); we only assert no panics
		// here, not key non-emptiness.
		_ = got.Fields
	})
}
