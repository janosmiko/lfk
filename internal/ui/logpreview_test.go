package ui

import (
	"strings"
	"testing"
)

func TestParseLogLine_PlainText(t *testing.T) {
	p := ParseLogLine("just a plain log line with no structure")
	if p.Kind != LogPreviewText {
		t.Fatalf("kind = %v, want Text", p.Kind)
	}
	if p.Prefix != "" || p.Time != "" {
		t.Fatalf("prefix/time should be empty, got %q / %q", p.Prefix, p.Time)
	}
	if len(p.Fields) != 0 {
		t.Fatalf("plain text should have no fields, got %v", p.Fields)
	}
}

func TestParseLogLine_JSON(t *testing.T) {
	p := ParseLogLine(`{"level":"info","msg":"server started","port":8080}`)
	if p.Kind != LogPreviewJSON {
		t.Fatalf("kind = %v, want JSON", p.Kind)
	}
	if len(p.Fields) != 3 {
		t.Fatalf("want 3 fields, got %d (%v)", len(p.Fields), p.Fields)
	}
	// Common keys are ranked: level (1), msg (2), then alphabetical.
	if p.Fields[0].Key != "level" || p.Fields[1].Key != "msg" || p.Fields[2].Key != "port" {
		t.Fatalf("field order wrong: %v", p.Fields)
	}
	if p.Fields[2].Value != "8080" {
		t.Fatalf("port value = %q, want 8080", p.Fields[2].Value)
	}
}

func TestParseLogLine_JSONNested(t *testing.T) {
	p := ParseLogLine(`{"event":{"name":"x","count":3}}`)
	if p.Kind != LogPreviewJSON {
		t.Fatalf("kind = %v, want JSON", p.Kind)
	}
	if !strings.Contains(p.Fields[0].Value, "name") {
		t.Fatalf("nested value should pretty-print object, got %q", p.Fields[0].Value)
	}
}

func TestParseLogLine_Logfmt(t *testing.T) {
	p := ParseLogLine(`level=info msg="server started" port=8080`)
	if p.Kind != LogPreviewLogfmt {
		t.Fatalf("kind = %v, want Logfmt", p.Kind)
	}
	if len(p.Fields) != 3 {
		t.Fatalf("want 3 fields, got %d", len(p.Fields))
	}
	if p.Fields[1].Key != "msg" || p.Fields[1].Value != "server started" {
		t.Fatalf("quoted field wrong: %+v", p.Fields[1])
	}
}

func TestParseLogLine_LogfmtNeedsTwoPairs(t *testing.T) {
	// A single foo=bar in free-form text must not promote to logfmt.
	p := ParseLogLine("starting service config=prod")
	if p.Kind != LogPreviewText {
		t.Fatalf("kind = %v, want Text (single pair)", p.Kind)
	}
}

func TestParseLogLine_PodPrefix(t *testing.T) {
	p := ParseLogLine(`[pod/api/server] {"level":"info","msg":"hi"}`)
	if p.Prefix != "[pod/api/server]" {
		t.Fatalf("prefix = %q, want [pod/api/server]", p.Prefix)
	}
	if p.Kind != LogPreviewJSON {
		t.Fatalf("kind = %v, want JSON after prefix strip", p.Kind)
	}
}

func TestParseLogLine_Timestamp(t *testing.T) {
	p := ParseLogLine(`2024-01-15T10:30:00.123456789Z hello world`)
	if p.Time == "" {
		t.Fatalf("expected timestamp to be extracted")
	}
	if !strings.HasPrefix(p.Time, "2024-01-15T") {
		t.Fatalf("timestamp = %q", p.Time)
	}
	if p.Body != "hello world" {
		t.Fatalf("body after timestamp = %q", p.Body)
	}
}

func TestParseLogLine_PrefixAndTimestampAndJSON(t *testing.T) {
	p := ParseLogLine(`[pod/api/server] 2024-01-15T10:30:00.000000000Z {"level":"warn","msg":"slow"}`)
	if p.Prefix != "[pod/api/server]" {
		t.Fatalf("prefix = %q", p.Prefix)
	}
	if p.Time == "" {
		t.Fatalf("time empty")
	}
	if p.Kind != LogPreviewJSON {
		t.Fatalf("kind = %v, want JSON", p.Kind)
	}
	if p.Fields[0].Key != "level" {
		t.Fatalf("first field = %q, want level", p.Fields[0].Key)
	}
}

func TestParseLogLine_MalformedJSON(t *testing.T) {
	p := ParseLogLine(`{not valid json}`)
	if p.Kind == LogPreviewJSON {
		t.Fatalf("malformed JSON should not parse as JSON")
	}
}

func TestParseLogLine_EmptyJSON(t *testing.T) {
	p := ParseLogLine(`{}`)
	if p.Kind == LogPreviewJSON {
		t.Fatalf("empty JSON object should fall through to text")
	}
}

func TestRenderLogPreviewPane_Dimensions(t *testing.T) {
	out := RenderLogPreviewPane(`{"level":"info","msg":"hi"}`, 40, 20)
	lines := strings.Split(out, "\n")
	// Title (1) + border-wrapped body (height-2 + 2 border lines = height) + footer (1) = height + 2.
	want := 22
	if len(lines) != want {
		t.Fatalf("rendered %d lines, want %d", len(lines), want)
	}
}

func TestRenderLogPreviewPane_EmptyLine(t *testing.T) {
	out := RenderLogPreviewPane("", 40, 10)
	if !strings.Contains(out, "no log line selected") {
		t.Fatalf("expected empty-state hint, got: %s", out)
	}
}

func TestRenderLogPreviewPane_KindLabel(t *testing.T) {
	jsonOut := RenderLogPreviewPane(`{"a":"b","c":"d"}`, 40, 10)
	if !strings.Contains(jsonOut, "JSON") {
		t.Fatalf("JSON label missing: %s", jsonOut)
	}
	logfmtOut := RenderLogPreviewPane(`a=b c=d`, 40, 10)
	if !strings.Contains(logfmtOut, "LOGFMT") {
		t.Fatalf("LOGFMT label missing: %s", logfmtOut)
	}
	textOut := RenderLogPreviewPane(`free form`, 40, 10)
	if !strings.Contains(textOut, "TEXT") {
		t.Fatalf("TEXT label missing: %s", textOut)
	}
}

func TestRenderLogPreviewPane_NarrowDoesNotPanic(t *testing.T) {
	// Tiny dimensions must clamp gracefully.
	_ = RenderLogPreviewPane("hello", 1, 1)
}

func TestRenderLogPreviewPane_FooterHasNoLegend(t *testing.T) {
	// The P binding is shown in the main log hint bar; the preview footer
	// must not duplicate it.
	out := RenderLogPreviewPane(`{"a":"b","c":"d"}`, 40, 10)
	if strings.Contains(out, "close preview") {
		t.Fatalf("preview footer should not repeat the P legend: %s", out)
	}
}

func TestTruncateKeyForPreview(t *testing.T) {
	cases := []struct {
		name      string
		in        string
		width     int
		wantKey   string
		wantWidth int
	}{
		{"short ascii fits", "level", 10, "level", 5},
		{"exact fit", "abcde", 5, "abcde", 5},
		{"truncates with ellipsis", "extremely_long_key_name", 18, "extremely_long_ke…", 18},
		{"single-cell width degrades to ellipsis only", "abc", 1, "…", 1},
		{"multibyte counted in runes", "αβγδε", 4, "αβγ…", 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotKey, gotWidth := truncateKeyForPreview(tc.in, tc.width)
			if gotKey != tc.wantKey {
				t.Fatalf("key = %q, want %q", gotKey, tc.wantKey)
			}
			if gotWidth != tc.wantWidth {
				t.Fatalf("width = %d, want %d", gotWidth, tc.wantWidth)
			}
		})
	}
}

func TestRenderLogPreviewPane_TruncatedKeyShowsEllipsis(t *testing.T) {
	out := RenderLogPreviewPane(`{"extremely_long_key_name":"v","b":"c"}`, 40, 10)
	if !strings.Contains(out, "…") {
		t.Fatalf("expected ellipsis on truncated key, got: %s", out)
	}
}

func TestRenderLogPreviewPane_SanitizesControlChars(t *testing.T) {
	// A JSON value containing an embedded ESC + non-SGR CSI must be
	// scrubbed so the panel layout cannot be corrupted by producer output.
	// (ConfigLogRenderAnsi defaults to false during tests.)
	in := "{\"msg\":\"before\x1b[2Jafter\"}"
	out := RenderLogPreviewPane(in, 60, 10)
	if strings.Contains(out, "\x1b[2J") {
		t.Fatalf("preview must scrub raw CSI sequences from values, got: %q", out)
	}
}

func TestRenderLogPreviewPane_SanitizesPlainTextBody(t *testing.T) {
	out := RenderLogPreviewPane("plain\x00body", 40, 10)
	if strings.Contains(out, "\x00") {
		t.Fatalf("preview must scrub NUL bytes from plain-text body")
	}
}

func TestRenderLogPreviewPane_NestedJSONKeepsLineBreaks(t *testing.T) {
	in := `{"event":{"type":"order","actor":{"id":"u1","role":"admin"}}}`
	out := RenderLogPreviewPane(in, 80, 20)
	if strings.Contains(out, "�") {
		t.Fatalf("nested JSON value must not contain replacement chars; got:\n%s", out)
	}
	// Pretty-printed nested keys should be visible on their own lines.
	for _, want := range []string{`"type"`, `"actor"`, `"id"`, `"role"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("nested key %s missing from preview output:\n%s", want, out)
		}
	}
}
