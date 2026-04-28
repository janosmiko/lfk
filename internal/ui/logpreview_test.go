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

func TestJSONKeyRank(t *testing.T) {
	cases := []struct {
		key  string
		want int
	}{
		// Time bucket.
		{"time", 0},
		{"timestamp", 0},
		{"ts", 0},
		{"@timestamp", 0},
		// Level bucket.
		{"level", 1},
		{"lvl", 1},
		{"severity", 1},
		// Message bucket.
		{"msg", 2},
		{"message", 2},
		// Source bucket.
		{"logger", 3},
		{"caller", 3},
		{"source", 3},
		// Error bucket.
		{"error", 4},
		{"err", 4},
		// Case-insensitive matching for any of the above.
		{"LEVEL", 1},
		{"Msg", 2},
		{"@TimeStamp", 0},
		// Anything else falls into the catch-all bucket.
		{"port", 100},
		{"trace_id", 100},
		{"", 100},
	}
	for _, tc := range cases {
		if got := jsonKeyRank(tc.key); got != tc.want {
			t.Errorf("jsonKeyRank(%q) = %d, want %d", tc.key, got, tc.want)
		}
	}
}

func TestParseLogLine_JSONFieldOrderUsesAliases(t *testing.T) {
	// Mixed aliases must produce the canonical rank order:
	// time(0) < level(1) < msg(2) < logger(3) < error(4) < others(100).
	p := ParseLogLine(`{"trace_id":"x","err":"boom","caller":"main.go:1","lvl":"info","message":"hi","@timestamp":"now"}`)
	if p.Kind != LogPreviewJSON {
		t.Fatalf("kind = %v, want JSON", p.Kind)
	}
	gotKeys := make([]string, len(p.Fields))
	for i, f := range p.Fields {
		gotKeys[i] = f.Key
	}
	wantKeys := []string{"@timestamp", "lvl", "message", "caller", "err", "trace_id"}
	if len(gotKeys) != len(wantKeys) {
		t.Fatalf("got %d fields, want %d", len(gotKeys), len(wantKeys))
	}
	for i := range wantKeys {
		if gotKeys[i] != wantKeys[i] {
			t.Errorf("field[%d] = %q, want %q (full order: %v)", i, gotKeys[i], wantKeys[i], gotKeys)
		}
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
	out := RenderLogPreviewPane(`{"level":"info","msg":"hi"}`, 40, 20, 0)
	lines := strings.Split(out, "\n")
	// Title (1) + border-wrapped body (height-2 + 2 border lines = height) + footer (1) = height + 2.
	want := 22
	if len(lines) != want {
		t.Fatalf("rendered %d lines, want %d", len(lines), want)
	}
}

func TestRenderLogPreviewPane_EmptyLine(t *testing.T) {
	out := RenderLogPreviewPane("", 40, 10, 0)
	if !strings.Contains(out, "no log line selected") {
		t.Fatalf("expected empty-state hint, got: %s", out)
	}
}

func TestRenderLogPreviewPane_KindLabel(t *testing.T) {
	jsonOut := RenderLogPreviewPane(`{"a":"b","c":"d"}`, 40, 10, 0)
	if !strings.Contains(jsonOut, "JSON") {
		t.Fatalf("JSON label missing: %s", jsonOut)
	}
	logfmtOut := RenderLogPreviewPane(`a=b c=d`, 40, 10, 0)
	if !strings.Contains(logfmtOut, "LOGFMT") {
		t.Fatalf("LOGFMT label missing: %s", logfmtOut)
	}
	textOut := RenderLogPreviewPane(`free form`, 40, 10, 0)
	if !strings.Contains(textOut, "TEXT") {
		t.Fatalf("TEXT label missing: %s", textOut)
	}
}

func TestRenderLogPreviewPane_NarrowDoesNotPanic(t *testing.T) {
	// Tiny dimensions must clamp gracefully.
	_ = RenderLogPreviewPane("hello", 1, 1, 0)
}

// scrollOverflowJSON is a JSON line whose pretty-printed body easily
// exceeds a small height so we can drive scroll behavior deterministically.
const scrollOverflowJSON = `{"a":"1","b":"2","c":"3","d":"4","e":"5","f":"6","g":"7","h":"8"}`

func TestLogPreviewMaxScroll(t *testing.T) {
	// Tall enough that everything fits — no scrolling possible.
	if got := LogPreviewMaxScroll(scrollOverflowJSON, 60, 30); got != 0 {
		t.Errorf("max scroll on tall pane = %d, want 0", got)
	}
	// Same content, short pane — must report a positive bound.
	if got := LogPreviewMaxScroll(scrollOverflowJSON, 60, 5); got <= 0 {
		t.Errorf("max scroll on short pane = %d, want > 0", got)
	}
	// Empty line: empty-state placeholder is one row, never overflows.
	if got := LogPreviewMaxScroll("", 40, 10); got != 0 {
		t.Errorf("max scroll on empty line = %d, want 0", got)
	}
	// Defensive: tiny width/height clamps internally without panicking and
	// still returns a non-negative bound.
	if got := LogPreviewMaxScroll(scrollOverflowJSON, 1, 1); got < 0 {
		t.Errorf("max scroll on tiny pane = %d, want >= 0", got)
	}
}

func TestRenderLogPreviewPane_ScrollHidesEarlierRows(t *testing.T) {
	// Render the same content at scroll 0 vs scroll 2. The first body
	// row at scroll 0 must not appear in the scroll-2 output.
	at0 := RenderLogPreviewPane(scrollOverflowJSON, 60, 5, 0)
	at2 := RenderLogPreviewPane(scrollOverflowJSON, 60, 5, 2)
	if at0 == at2 {
		t.Fatal("scrolled and unscrolled output should differ")
	}
	// "a" is the alphabetically-first non-ranked key; with the rank-then-
	// alpha sort it sits on the first body row at scroll 0.
	firstRowToken := " a "
	if !strings.Contains(at0, firstRowToken) {
		t.Fatalf("expected first body row token %q in scroll=0 output:\n%s", firstRowToken, at0)
	}
	if strings.Contains(at2, firstRowToken) {
		t.Fatalf("scroll=2 output should not contain first row token %q:\n%s", firstRowToken, at2)
	}
}

func TestRenderLogPreviewPane_ScrollClampsToMax(t *testing.T) {
	// Scroll past the end clamps; output must equal what render at the
	// computed maxScroll produces.
	maxScroll := LogPreviewMaxScroll(scrollOverflowJSON, 60, 5)
	atMax := RenderLogPreviewPane(scrollOverflowJSON, 60, 5, maxScroll)
	atOverflow := RenderLogPreviewPane(scrollOverflowJSON, 60, 5, maxScroll+10)
	if atMax != atOverflow {
		t.Fatal("scroll beyond max should clamp and produce identical output")
	}
}

func TestRenderLogPreviewPane_NegativeScrollClampsToZero(t *testing.T) {
	at0 := RenderLogPreviewPane(scrollOverflowJSON, 60, 5, 0)
	atNeg := RenderLogPreviewPane(scrollOverflowJSON, 60, 5, -5)
	if at0 != atNeg {
		t.Fatal("negative scroll should clamp to 0 and produce identical output")
	}
}

func TestRenderLogPreviewPane_AtMaxScrollLastBodyRowIsVisible(t *testing.T) {
	// Regression: scrolling to max must actually reveal the last body row.
	// We pick a key (z_last_bucket) that ranks at the catch-all 100 bucket
	// AND sorts alphabetically last, so we know which token must appear.
	json := `{"a":"1","b":"2","c":"3","d":"4","e":"5","f":"6","g":"7","z_last_bucket":"END"}`
	width, height := 60, 5
	maxScroll := LogPreviewMaxScroll(json, width, height)
	if maxScroll == 0 {
		t.Fatal("test setup error: pick dimensions/content that overflow")
	}
	out := RenderLogPreviewPane(json, width, height, maxScroll)
	if !strings.Contains(out, "z_last_bucket") {
		t.Fatalf("at max scroll the last body row (key 'z_last_bucket') must be visible:\n%s", out)
	}
	if !strings.Contains(out, "END") {
		t.Fatalf("at max scroll the value 'END' of the last body row must be visible:\n%s", out)
	}
}

func TestRenderLogPreviewPane_TitleShowsPositionWhenScrollable(t *testing.T) {
	// Short pane → overflow → "N/M" position label appears in the title.
	out := RenderLogPreviewPane(scrollOverflowJSON, 60, 5, 0)
	if !strings.Contains(out, "1/") {
		t.Fatalf("expected position label like '1/N' in scrollable output:\n%s", out)
	}
}

func TestRenderLogPreviewPane_NoPositionLabelWhenContentFits(t *testing.T) {
	// Tall pane → no overflow → no position label.
	out := RenderLogPreviewPane(`{"a":"1","b":"2"}`, 60, 30, 0)
	for _, frag := range []string{"1/", "2/", "3/"} {
		if strings.Contains(out, frag) {
			t.Fatalf("non-scrollable pane should not show position label %q:\n%s", frag, out)
		}
	}
}

func TestRenderLogPreviewPane_FooterHasNoLegend(t *testing.T) {
	// The P binding is shown in the main log hint bar; the preview footer
	// must not duplicate it.
	out := RenderLogPreviewPane(`{"a":"b","c":"d"}`, 40, 10, 0)
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
	out := RenderLogPreviewPane(`{"extremely_long_key_name":"v","b":"c"}`, 40, 10, 0)
	if !strings.Contains(out, "…") {
		t.Fatalf("expected ellipsis on truncated key, got: %s", out)
	}
}

func TestRenderLogPreviewPane_SanitizesControlChars(t *testing.T) {
	// A JSON value containing an embedded ESC + non-SGR CSI must be
	// scrubbed so the panel layout cannot be corrupted by producer output.
	// (ConfigLogRenderAnsi defaults to false during tests.)
	in := "{\"msg\":\"before\x1b[2Jafter\"}"
	out := RenderLogPreviewPane(in, 60, 10, 0)
	if strings.Contains(out, "\x1b[2J") {
		t.Fatalf("preview must scrub raw CSI sequences from values, got: %q", out)
	}
}

func TestRenderLogPreviewPane_SanitizesPlainTextBody(t *testing.T) {
	out := RenderLogPreviewPane("plain\x00body", 40, 10, 0)
	if strings.Contains(out, "\x00") {
		t.Fatalf("preview must scrub NUL bytes from plain-text body")
	}
}

func TestRenderLogPreviewPane_NestedJSONKeepsLineBreaks(t *testing.T) {
	in := `{"event":{"type":"order","actor":{"id":"u1","role":"admin"}}}`
	out := RenderLogPreviewPane(in, 80, 20, 0)
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
