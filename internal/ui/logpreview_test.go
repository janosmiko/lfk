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

// --- klog (LMMDD HH:MM:SS.uuuuuu threadid file:line] msg) ---

func TestParseLogLine_Klog_Warning(t *testing.T) {
	in := `W0415 12:40:18.037168       1 mount_helper_common.go:142] Warning: "/var/lib/kubelet/pods/abc/mount" is not a mountpoint, deleting`
	p := ParseLogLine(in)
	if p.Kind != LogPreviewKlog {
		t.Fatalf("kind = %v, want Klog", p.Kind)
	}
	want := map[string]string{
		"level":   "Warning",
		"time":    "0415 12:40:18.037168",
		"thread":  "1",
		"caller":  "mount_helper_common.go:142",
		"message": `Warning: "/var/lib/kubelet/pods/abc/mount" is not a mountpoint, deleting`,
	}
	if len(p.Fields) != len(want) {
		t.Fatalf("got %d fields, want %d (%v)", len(p.Fields), len(want), p.Fields)
	}
	for _, f := range p.Fields {
		if w, ok := want[f.Key]; !ok {
			t.Errorf("unexpected field key %q (value %q)", f.Key, f.Value)
		} else if f.Value != w {
			t.Errorf("field %q = %q, want %q", f.Key, f.Value, w)
		}
	}
}

func TestParseLogLine_Klog_AllLevels(t *testing.T) {
	cases := []struct {
		letter, name string
	}{
		{"I", "Info"},
		{"W", "Warning"},
		{"E", "Error"},
		{"F", "Fatal"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := tc.letter + `0427 16:06:59.211543       1 leaderelection.go:257] hi`
			p := ParseLogLine(in)
			if p.Kind != LogPreviewKlog {
				t.Fatalf("%s: kind = %v, want Klog", tc.letter, p.Kind)
			}
			var got string
			for _, f := range p.Fields {
				if f.Key == "level" {
					got = f.Value
				}
			}
			if got != tc.name {
				t.Errorf("level for %q = %q, want %q", tc.letter, got, tc.name)
			}
		})
	}
}

func TestParseLogLine_Klog_KeyOrderIsTimeLevelMessageCallerThread(t *testing.T) {
	// Display order is dictated by the Fields slice; the parser must put
	// the most useful information first so the user sees it without
	// scrolling. Order: time, level, message, caller, thread.
	p := ParseLogLine(`E0427 16:06:59.212862       1 leaderelection.go:436] error retrieving resource lock`)
	if p.Kind != LogPreviewKlog {
		t.Fatalf("kind = %v, want Klog", p.Kind)
	}
	gotKeys := make([]string, len(p.Fields))
	for i, f := range p.Fields {
		gotKeys[i] = f.Key
	}
	wantKeys := []string{"time", "level", "message", "caller", "thread"}
	if len(gotKeys) != len(wantKeys) {
		t.Fatalf("got %d fields (%v), want %d (%v)", len(gotKeys), gotKeys, len(wantKeys), wantKeys)
	}
	for i := range wantKeys {
		if gotKeys[i] != wantKeys[i] {
			t.Errorf("field[%d] = %q, want %q (full order: %v)", i, gotKeys[i], wantKeys[i], gotKeys)
		}
	}
}

func TestParseLogLine_Klog_WithPodPrefix(t *testing.T) {
	// Pod prefix stripping happens before structured parsing, so a
	// klog line wearing a kubectl bracket prefix still resolves to Klog
	// kind and the prefix lands in p.Prefix for the preview header.
	in := `[pod/kubelet/main] W0415 12:40:18.037168       1 mount_helper_common.go:142] Warning: x`
	p := ParseLogLine(in)
	if p.Prefix != "[pod/kubelet/main]" {
		t.Fatalf("prefix = %q, want [pod/kubelet/main]", p.Prefix)
	}
	if p.Kind != LogPreviewKlog {
		t.Fatalf("kind = %v, want Klog after prefix strip", p.Kind)
	}
}

func TestParseLogLine_Klog_RejectsNonKlog(t *testing.T) {
	// These should NOT be misclassified as klog. The parser must be
	// strict about the leading L<digits> shape so plain text starting
	// with a capital letter doesn't get co-opted.
	cases := []string{
		"W is the 23rd letter",                    // letter-then-text, no digits
		"INFO server started",                     // word, not single-letter level
		"X0415 12:40:18.000000 1 a.go:1] x",       // unknown level letter
		"W04 12:40:18.000000 1 a.go:1] x",         // mmdd too short
		"W0415 12:40 1 a.go:1] x",                 // time missing seconds/micros
		"W0415 12:40:18.000000 a.go:1] no thread", // missing thread id
		"W0415 12:40:18.000000 1 a.go:NaN] x",     // non-numeric line
		"W0415 12:40:18.000000 1 a.go:1 missing-bracket",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			p := ParseLogLine(in)
			if p.Kind == LogPreviewKlog {
				t.Errorf("misclassified as Klog: %q -> fields=%v", in, p.Fields)
			}
		})
	}
}

func TestParseLogLine_Klog_BeatsLogfmtForMessageBody(t *testing.T) {
	// A klog body that happens to contain key=value fragments must be
	// classified as klog, not promoted to logfmt by the >=2-pair detector.
	in := `I0427 16:07:22.000000       1 controller.go:99] reconciling foo=bar baz=qux`
	p := ParseLogLine(in)
	if p.Kind != LogPreviewKlog {
		t.Fatalf("kind = %v, want Klog (must beat logfmt)", p.Kind)
	}
	var msg string
	for _, f := range p.Fields {
		if f.Key == "message" {
			msg = f.Value
		}
	}
	if msg != "reconciling foo=bar baz=qux" {
		t.Fatalf("message = %q, want full klog body", msg)
	}
}

func TestRenderLogPreviewPane_KlogTitleIndicator(t *testing.T) {
	in := `W0415 12:40:18.037168       1 mount_helper_common.go:142] Warning: x`
	out := RenderLogPreviewPane(in, 80, 20, 0)
	if !strings.Contains(out, "[KLOG]") {
		t.Fatalf("expected [KLOG] kind indicator in title; got:\n%s", out)
	}
	for _, want := range []string{"level", "Warning", "caller", "mount_helper_common.go:142"} {
		if !strings.Contains(out, want) {
			t.Errorf("klog preview missing %q; got:\n%s", want, out)
		}
	}
}

// --- zap dev/console encoder (TIMESTAMP\tLEVEL\t[LOGGER\t]MESSAGE[\t{JSON}]) ---

func TestParseLogLine_Zap_LevelLoggerMessageJSON(t *testing.T) {
	// Real controller-runtime emission:
	// timestamp + tab + level + tab + logger + tab + message + tab + JSON.
	// Pod prefix and timestamp are stripped first so the parser sees the
	// LEVEL\tLOGGER\tMESSAGE\tJSON tail.
	in := "[pod/dragonfly-operator-x/manager] 2026-04-27T16:06:59Z\tINFO\tcontroller-runtime.metrics\tServing metrics server\t{\"bindAddress\": \"127.0.0.1:8080\", \"secure\": false}"
	p := ParseLogLine(in)
	if p.Kind != LogPreviewZap {
		t.Fatalf("kind = %v, want Zap", p.Kind)
	}
	if p.Prefix != "[pod/dragonfly-operator-x/manager]" {
		t.Errorf("prefix = %q", p.Prefix)
	}
	if p.Time == "" {
		t.Errorf("expected RFC3339 timestamp to be extracted into p.Time, got empty")
	}
	want := []LogPreviewField{
		{Key: "level", Value: "Info"},
		{Key: "logger", Value: "controller-runtime.metrics"},
		{Key: "message", Value: "Serving metrics server"},
		// Embedded JSON fields preserve their natural ordering (jsonKeyRank
		// applies — bindAddress and secure both rank 100, so alpha order).
		{Key: "bindAddress", Value: "127.0.0.1:8080"},
		{Key: "secure", Value: "false"},
	}
	if len(p.Fields) != len(want) {
		t.Fatalf("got %d fields, want %d (%v)", len(p.Fields), len(want), p.Fields)
	}
	for i, w := range want {
		if p.Fields[i].Key != w.Key || p.Fields[i].Value != w.Value {
			t.Errorf("field[%d] = {%q, %q}, want {%q, %q}",
				i, p.Fields[i].Key, p.Fields[i].Value, w.Key, w.Value)
		}
	}
}

func TestParseLogLine_Zap_NoLoggerNoJSON(t *testing.T) {
	// Bare zap line: level + message only. No logger, no fields JSON.
	in := "INFO\tstarting manager"
	p := ParseLogLine(in)
	if p.Kind != LogPreviewZap {
		t.Fatalf("kind = %v, want Zap", p.Kind)
	}
	if len(p.Fields) != 2 {
		t.Fatalf("got %d fields, want 2 (%v)", len(p.Fields), p.Fields)
	}
	if p.Fields[0].Key != "level" || p.Fields[0].Value != "Info" {
		t.Errorf("field[0] = %+v, want {level Info}", p.Fields[0])
	}
	if p.Fields[1].Key != "message" || p.Fields[1].Value != "starting manager" {
		t.Errorf("field[1] = %+v, want {message starting manager}", p.Fields[1])
	}
}

func TestParseLogLine_Zap_MessageWithoutLoggerButWithJSON(t *testing.T) {
	// Common controller-runtime pattern: no logger, message + JSON fields.
	// Tab-split gives [LEVEL, MESSAGE, JSON_TEXT]. The detector must treat
	// the middle token as the message because the last token parses as JSON.
	in := `INFO	reconciling dragonfly instance	{"controller": "Dragonfly", "namespace": "m2community-stage", "name": "redis"}`
	p := ParseLogLine(in)
	if p.Kind != LogPreviewZap {
		t.Fatalf("kind = %v, want Zap", p.Kind)
	}
	keys := make([]string, len(p.Fields))
	for i, f := range p.Fields {
		keys[i] = f.Key
	}
	// level first, then message (no logger), then JSON-derived fields.
	if len(keys) < 2 || keys[0] != "level" || keys[1] != "message" {
		t.Fatalf("first two keys = %v, want [level message ...]", keys)
	}
	// JSON-derived field values must be present.
	got := map[string]string{}
	for _, f := range p.Fields {
		got[f.Key] = f.Value
	}
	if got["controller"] != "Dragonfly" {
		t.Errorf("expected controller=Dragonfly from embedded JSON, got %q", got["controller"])
	}
	if got["namespace"] != "m2community-stage" {
		t.Errorf("expected namespace=m2community-stage, got %q", got["namespace"])
	}
}

func TestParseLogLine_Zap_AllLevels(t *testing.T) {
	cases := []struct {
		token, name string
	}{
		{"DEBUG", "Debug"},
		{"INFO", "Info"},
		{"WARN", "Warning"},
		{"WARNING", "Warning"},
		{"ERROR", "Error"},
		{"DPANIC", "Fatal"},
		{"PANIC", "Fatal"},
		{"FATAL", "Fatal"},
	}
	for _, tc := range cases {
		t.Run(tc.token, func(t *testing.T) {
			in := tc.token + "\thello"
			p := ParseLogLine(in)
			if p.Kind != LogPreviewZap {
				t.Fatalf("kind = %v, want Zap for %q", p.Kind, tc.token)
			}
			if p.Fields[0].Value != tc.name {
				t.Errorf("level for %q = %q, want %q", tc.token, p.Fields[0].Value, tc.name)
			}
		})
	}
}

func TestParseLogLine_Zap_RejectsNonZap(t *testing.T) {
	cases := []string{
		"plain text without tabs",
		"INFO no tab follows",   // level token but no tab
		"NOTALEVEL\tsome text",  // first token is not a known level
		"\tleading tab",         // empty level token
		"info\tlowercase level", // real controller-runtime/zap uses uppercase tokens
		// Falls through to existing parsers.
		"key1=v1 key2=v2 key3=v3",
		`{"level":"info","msg":"x"}`,
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			p := ParseLogLine(in)
			if p.Kind == LogPreviewZap {
				t.Errorf("misclassified as Zap: %q -> %v", in, p.Fields)
			}
		})
	}
}

func TestParseLogLine_Zap_TooManyTabSegmentsRejected(t *testing.T) {
	// More than LEVEL + LOGGER + MESSAGE + JSON (= 4 tab-separated parts)
	// is not the controller-runtime/zap shape; refuse to claim the format
	// rather than silently lose data into a single 'message' field.
	in := "INFO\tlogger\tmsg\textra\t{\"k\":\"v\"}"
	p := ParseLogLine(in)
	if p.Kind == LogPreviewZap {
		t.Errorf("unexpected Zap classification for %d tab segments: %v",
			len(strings.Split(in, "\t")), p.Fields)
	}
}

func TestRenderLogPreviewPane_ZapTitleIndicator(t *testing.T) {
	in := "[pod/x/manager] 2026-04-27T16:06:59Z\tINFO\tsetup\tstarting manager"
	out := RenderLogPreviewPane(in, 80, 20, 0)
	if !strings.Contains(out, "[ZAP]") {
		t.Fatalf("expected [ZAP] kind indicator in title; got:\n%s", out)
	}
	for _, want := range []string{"level", "Info", "logger", "setup", "message", "starting manager"} {
		if !strings.Contains(out, want) {
			t.Errorf("zap preview missing %q; got:\n%s", want, out)
		}
	}
}

// --- NGINX/Apache/Traefik Combined Log Format ---
//
//	host - user [DD/Mon/YYYY:HH:MM:SS +ZZZZ] "METHOD path HTTP/v" status bytes "referer" "ua"

func TestParseLogLine_Nginx_Combined(t *testing.T) {
	in := `192.168.1.1 - - [15/Jan/2024:10:30:00 +0000] "GET /api/v1/pods HTTP/1.1" 200 1234 "https://example.com" "Mozilla/5.0 (X11; Linux x86_64)"`
	p := ParseLogLine(in)
	if p.Kind != LogPreviewNginx {
		t.Fatalf("kind = %v, want Nginx", p.Kind)
	}
	want := map[string]string{
		"method":     "GET",
		"path":       "/api/v1/pods",
		"status":     "200",
		"bytes":      "1234",
		"protocol":   "HTTP/1.1",
		"referer":    "https://example.com",
		"user_agent": "Mozilla/5.0 (X11; Linux x86_64)",
		"client":     "192.168.1.1",
		"user":       "-",
		"time":       "15/Jan/2024:10:30:00 +0000",
	}
	got := map[string]string{}
	for _, f := range p.Fields {
		got[f.Key] = f.Value
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("field %q = %q, want %q", k, got[k], v)
		}
	}
	if len(p.Fields) != len(want) {
		t.Errorf("got %d fields (%v), want %d", len(p.Fields), p.Fields, len(want))
	}
}

func TestParseLogLine_Nginx_FieldOrderPutsRequestSummaryFirst(t *testing.T) {
	in := `10.0.0.1 - alice [15/Jan/2024:10:30:00 +0000] "POST /login HTTP/2.0" 401 0 "-" "curl/8.5.0"`
	p := ParseLogLine(in)
	if p.Kind != LogPreviewNginx {
		t.Fatalf("kind = %v, want Nginx", p.Kind)
	}
	keys := make([]string, len(p.Fields))
	for i, f := range p.Fields {
		keys[i] = f.Key
	}
	// "What happened" first: method, path, status. Then bytes, then context.
	want := []string{"method", "path", "status"}
	for i, w := range want {
		if i >= len(keys) || keys[i] != w {
			t.Errorf("key[%d] = %q, want %q (full order: %v)", i, keys[i], w, keys)
		}
	}
}

func TestParseLogLine_Nginx_PodPrefixStripped(t *testing.T) {
	in := `[pod/ingress-nginx-controller-x/controller] 192.168.1.1 - - [15/Jan/2024:10:30:00 +0000] "GET / HTTP/1.1" 200 0 "-" "kube-probe/1.30"`
	p := ParseLogLine(in)
	if p.Prefix != "[pod/ingress-nginx-controller-x/controller]" {
		t.Errorf("prefix = %q", p.Prefix)
	}
	if p.Kind != LogPreviewNginx {
		t.Fatalf("kind = %v, want Nginx after prefix strip", p.Kind)
	}
}

func TestParseLogLine_Nginx_BytesDashAccepted(t *testing.T) {
	// Some configs emit "-" for body_bytes_sent on empty responses.
	in := `1.2.3.4 - - [15/Jan/2024:10:30:00 +0000] "HEAD /healthz HTTP/1.1" 204 - "-" "kube-probe/1.30"`
	p := ParseLogLine(in)
	if p.Kind != LogPreviewNginx {
		t.Fatalf("kind = %v, want Nginx (bytes='-')", p.Kind)
	}
}

func TestParseLogLine_Nginx_RejectsNonNginx(t *testing.T) {
	cases := []string{
		// Missing the bracketed Apache-style timestamp.
		`192.168.1.1 - - GET / HTTP/1.1 200`,
		// Missing the quoted request line.
		`192.168.1.1 - - [15/Jan/2024:10:30:00 +0000] GET / 200`,
		// Wrong timestamp format (RFC3339, not Apache).
		`192.168.1.1 - - [2024-01-15T10:30:00Z] "GET / HTTP/1.1" 200 0 "-" "-"`,
		// Missing the second quoted block (combined needs both referer and ua).
		`192.168.1.1 - - [15/Jan/2024:10:30:00 +0000] "GET / HTTP/1.1" 200 1234 "https://example.com"`,
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			p := ParseLogLine(in)
			if p.Kind == LogPreviewNginx {
				t.Errorf("misclassified as Nginx: %q -> %v", in, p.Fields)
			}
		})
	}
}

func TestParseLogLine_Nginx_BeatsLogfmtForQuerystringPairs(t *testing.T) {
	// A request URL with query-string pairs would satisfy logfmt's >=2-pair
	// detector. The nginx parser must run first so the structural
	// classification wins.
	in := `192.168.1.1 - - [15/Jan/2024:10:30:00 +0000] "GET /api?foo=bar&baz=qux HTTP/1.1" 200 1234 "-" "-"`
	p := ParseLogLine(in)
	if p.Kind != LogPreviewNginx {
		t.Fatalf("kind = %v, want Nginx (must beat logfmt)", p.Kind)
	}
}

func TestRenderLogPreviewPane_NginxTitleIndicator(t *testing.T) {
	in := `192.168.1.1 - - [15/Jan/2024:10:30:00 +0000] "GET /api HTTP/1.1" 200 1234 "-" "curl/8"`
	out := RenderLogPreviewPane(in, 100, 20, 0)
	if !strings.Contains(out, "[NGINX]") {
		t.Fatalf("expected [NGINX] kind indicator in title; got:\n%s", out)
	}
	for _, want := range []string{"method", "GET", "path", "/api", "status", "200"} {
		if !strings.Contains(out, want) {
			t.Errorf("nginx preview missing %q; got:\n%s", want, out)
		}
	}
}

// --- Envoy access log (default text format) ---
//
//	[RFC3339] "METHOD path proto" status flags bytes_recv bytes_sent dur ust "xff" "ua" "rid" "auth" "upstream"

func TestParseLogLine_Envoy_DefaultAccessLog(t *testing.T) {
	in := `[2024-01-15T10:30:00.123Z] "GET /api/v1/pods HTTP/1.1" 200 - 0 1234 5 4 "192.168.1.1" "curl/7.81" "abc-123-def" "api.example.com" "10.0.0.5:8080"`
	p := ParseLogLine(in)
	if p.Kind != LogPreviewEnvoy {
		t.Fatalf("kind = %v, want Envoy", p.Kind)
	}
	want := map[string]string{
		"method":          "GET",
		"path":            "/api/v1/pods",
		"protocol":        "HTTP/1.1",
		"status":          "200",
		"response_flags":  "-",
		"bytes_received":  "0",
		"bytes_sent":      "1234",
		"duration_ms":     "5",
		"upstream_time":   "4",
		"x_forwarded_for": "192.168.1.1",
		"user_agent":      "curl/7.81",
		"request_id":      "abc-123-def",
		"authority":       "api.example.com",
		"upstream_host":   "10.0.0.5:8080",
		"time":            "2024-01-15T10:30:00.123Z",
	}
	got := map[string]string{}
	for _, f := range p.Fields {
		got[f.Key] = f.Value
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("field %q = %q, want %q", k, got[k], v)
		}
	}
	if len(p.Fields) != len(want) {
		t.Errorf("got %d fields (%v), want %d", len(p.Fields), p.Fields, len(want))
	}
}

func TestParseLogLine_Envoy_ResponseFlagAndDashUpstream(t *testing.T) {
	// Real Istio output for a request that never reached an upstream:
	// response_flags="UH" (upstream healthy unavailable), upstream time "-",
	// upstream host "-".
	in := `[2024-01-15T10:30:00.000Z] "GET /missing HTTP/1.1" 503 UH 0 91 0 - "10.244.0.1" "kube-probe/1.30" "req-x" "svc.ns.svc.cluster.local" "-"`
	p := ParseLogLine(in)
	if p.Kind != LogPreviewEnvoy {
		t.Fatalf("kind = %v, want Envoy", p.Kind)
	}
	got := map[string]string{}
	for _, f := range p.Fields {
		got[f.Key] = f.Value
	}
	if got["response_flags"] != "UH" {
		t.Errorf("response_flags = %q, want UH", got["response_flags"])
	}
	if got["upstream_time"] != "-" {
		t.Errorf("upstream_time = %q, want -", got["upstream_time"])
	}
	if got["upstream_host"] != "-" {
		t.Errorf("upstream_host = %q, want -", got["upstream_host"])
	}
}

func TestParseLogLine_Envoy_FieldOrderPutsRequestSummaryFirst(t *testing.T) {
	in := `[2024-01-15T10:30:00.000Z] "POST /login HTTP/2" 401 - 12 0 1 - "-" "-" "-" "-" "-"`
	p := ParseLogLine(in)
	if p.Kind != LogPreviewEnvoy {
		t.Fatalf("kind = %v, want Envoy", p.Kind)
	}
	keys := make([]string, len(p.Fields))
	for i, f := range p.Fields {
		keys[i] = f.Key
	}
	want := []string{"method", "path", "status"}
	for i, w := range want {
		if i >= len(keys) || keys[i] != w {
			t.Errorf("key[%d] = %q, want %q (full order: %v)", i, keys[i], w, keys)
		}
	}
}

func TestParseLogLine_Envoy_PodPrefixStripped(t *testing.T) {
	in := `[pod/istio-proxy-x/istio-proxy] [2024-01-15T10:30:00.000Z] "GET / HTTP/1.1" 200 - 0 0 0 - "-" "-" "-" "-" "-"`
	p := ParseLogLine(in)
	if p.Prefix != "[pod/istio-proxy-x/istio-proxy]" {
		t.Errorf("prefix = %q", p.Prefix)
	}
	if p.Kind != LogPreviewEnvoy {
		t.Fatalf("kind = %v, want Envoy after prefix strip", p.Kind)
	}
}

func TestParseLogLine_Envoy_RejectsNonEnvoy(t *testing.T) {
	cases := []string{
		// Missing the leading bracketed RFC3339.
		`"GET / HTTP/1.1" 200 - 0 0 0 - "-" "-" "-" "-" "-"`,
		// Missing the trailing quoted blocks (only 3 of 5).
		`[2024-01-15T10:30:00.000Z] "GET / HTTP/1.1" 200 - 0 0 0 - "-" "-" "-"`,
		// NGINX-style timestamp inside brackets.
		`[15/Jan/2024:10:30:00 +0000] "GET / HTTP/1.1" 200 - 0 0 0 - "-" "-" "-" "-" "-"`,
		// Random bracketed prefix that isn't a timestamp.
		`[some-tag] "GET / HTTP/1.1" 200 - 0 0 0 - "-" "-" "-" "-" "-"`,
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			p := ParseLogLine(in)
			if p.Kind == LogPreviewEnvoy {
				t.Errorf("misclassified as Envoy: %q -> %v", in, p.Fields)
			}
		})
	}
}

func TestParseLogLine_Envoy_BeatsLogfmtForQuerystringPairs(t *testing.T) {
	in := `[2024-01-15T10:30:00.000Z] "GET /api?foo=bar&baz=qux HTTP/1.1" 200 - 0 1234 5 4 "-" "-" "-" "-" "-"`
	p := ParseLogLine(in)
	if p.Kind != LogPreviewEnvoy {
		t.Fatalf("kind = %v, want Envoy (must beat logfmt)", p.Kind)
	}
}

func TestRenderLogPreviewPane_EnvoyTitleIndicator(t *testing.T) {
	in := `[2024-01-15T10:30:00.000Z] "GET /api HTTP/1.1" 200 - 0 1234 5 4 "192.168.1.1" "curl/8" "rid" "host" "10.0.0.5:8080"`
	out := RenderLogPreviewPane(in, 120, 25, 0)
	if !strings.Contains(out, "[ENVOY]") {
		t.Fatalf("expected [ENVOY] kind indicator in title; got:\n%s", out)
	}
	for _, want := range []string{"method", "GET", "status", "200", "duration_ms", "upstream_host"} {
		if !strings.Contains(out, want) {
			t.Errorf("envoy preview missing %q; got:\n%s", want, out)
		}
	}
}

// --- Java: Spring Boot default and plain Logback ---

func TestParseLogLine_Java_SpringBootDefault(t *testing.T) {
	// Spring Boot's default ConsoleAppender pattern (since 2.x):
	// %d{yyyy-MM-dd'T'HH:mm:ss.SSSXXX}  %5p ${PID:- } --- [%t] %-40.40c{1.} : %m%n
	in := `2024-01-15T10:30:00.123+00:00  INFO 12345 --- [main] c.e.MyService : Server started on port 8080`
	p := ParseLogLine(in)
	if p.Kind != LogPreviewJava {
		t.Fatalf("kind = %v, want Java", p.Kind)
	}
	// Leading RFC3339 timestamp is extracted to p.Time by the shared
	// splitLeadingTimestamp helper (same as zap), so the parser sees
	// only the level-onwards tail and Fields does not duplicate time.
	if p.Time != "2024-01-15T10:30:00.123+00:00" {
		t.Errorf("p.Time = %q, want the RFC3339 prefix", p.Time)
	}
	got := map[string]string{}
	for _, f := range p.Fields {
		got[f.Key] = f.Value
	}
	want := map[string]string{
		"level":   "Info",
		"logger":  "c.e.MyService",
		"message": "Server started on port 8080",
		"thread":  "main",
		"pid":     "12345",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("field %q = %q, want %q", k, got[k], v)
		}
	}
}

func TestParseLogLine_Java_PlainLogbackDefault(t *testing.T) {
	// Logback BasicConfigurator default pattern:
	// HH:mm:ss.SSS [thread] LEVEL logger - msg
	in := `10:30:00.123 [main] INFO com.example.MyService - Connecting to database`
	p := ParseLogLine(in)
	if p.Kind != LogPreviewJava {
		t.Fatalf("kind = %v, want Java", p.Kind)
	}
	got := map[string]string{}
	for _, f := range p.Fields {
		got[f.Key] = f.Value
	}
	if got["level"] != "Info" || got["logger"] != "com.example.MyService" ||
		got["message"] != "Connecting to database" || got["thread"] != "main" ||
		got["time"] != "10:30:00.123" {
		t.Errorf("logback fields wrong: %v", got)
	}
}

func TestParseLogLine_Java_AllLevels(t *testing.T) {
	cases := []struct {
		token, name string
	}{
		{"TRACE", "Trace"},
		{"DEBUG", "Debug"},
		{"INFO", "Info"},
		{"WARN", "Warning"},
		{"WARNING", "Warning"},
		{"ERROR", "Error"},
		{"FATAL", "Fatal"},
	}
	for _, tc := range cases {
		t.Run(tc.token, func(t *testing.T) {
			in := `10:30:00.123 [main] ` + tc.token + ` com.x.Y - hello`
			p := ParseLogLine(in)
			if p.Kind != LogPreviewJava {
				t.Fatalf("%s: kind = %v, want Java", tc.token, p.Kind)
			}
			var got string
			for _, f := range p.Fields {
				if f.Key == "level" {
					got = f.Value
				}
			}
			if got != tc.name {
				t.Errorf("level for %q = %q, want %q", tc.token, got, tc.name)
			}
		})
	}
}

func TestParseLogLine_Java_PodPrefixStripped(t *testing.T) {
	in := `[pod/api-server-x/api] 2024-01-15T10:30:00.123+00:00  WARN 12345 --- [http-nio-8080-exec-1] o.s.web.servlet : ` +
		`Resolved [org.springframework.web.HttpRequestMethodNotSupportedException]`
	p := ParseLogLine(in)
	if p.Prefix != "[pod/api-server-x/api]" {
		t.Errorf("prefix = %q", p.Prefix)
	}
	if p.Kind != LogPreviewJava {
		t.Fatalf("kind = %v, want Java after prefix strip", p.Kind)
	}
}

func TestParseLogLine_Java_RejectsNonJava(t *testing.T) {
	cases := []string{
		`plain text without any structure`,
		`10:30:00 [main] INFO Foo - bar`,         // missing milliseconds (Logback always emits .SSS)
		`10:30:00.123 main INFO Foo - bar`,       // thread not bracketed
		`10:30:00.123 [main] NOTLEVEL Foo - bar`, // unknown level token
		`10:30:00.123 [main] INFO Foo bar`,       // missing " - " separator
		`{"level":"info","msg":"x"}`,             // JSON wins
		`I0427 16:06:59 1 file.go:1] klog wins`,  // klog wins
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			p := ParseLogLine(in)
			if p.Kind == LogPreviewJava {
				t.Errorf("misclassified as Java: %q -> %v", in, p.Fields)
			}
		})
	}
}

func TestRenderLogPreviewPane_JavaTitleIndicator(t *testing.T) {
	in := `2024-01-15T10:30:00.123+00:00  INFO 12345 --- [main] c.e.MyService : Server started on port 8080`
	out := RenderLogPreviewPane(in, 100, 20, 0)
	if !strings.Contains(out, "[JAVA]") {
		t.Fatalf("expected [JAVA] kind indicator in title; got:\n%s", out)
	}
	for _, want := range []string{"level", "Info", "logger", "c.e.MyService", "message", "Server started"} {
		if !strings.Contains(out, want) {
			t.Errorf("java preview missing %q; got:\n%s", want, out)
		}
	}
}

// --- PostgreSQL: default log_line_prefix='%m [%p] ' ---

func TestParseLogLine_Postgres_DefaultLine(t *testing.T) {
	in := `2024-01-15 10:30:00.123 UTC [1234] LOG:  database system is ready to accept connections`
	p := ParseLogLine(in)
	if p.Kind != LogPreviewPostgres {
		t.Fatalf("kind = %v, want Postgres", p.Kind)
	}
	got := map[string]string{}
	for _, f := range p.Fields {
		got[f.Key] = f.Value
	}
	want := map[string]string{
		"severity": "LOG",
		"message":  "database system is ready to accept connections",
		"pid":      "1234",
		"time":     "2024-01-15 10:30:00.123 UTC",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("field %q = %q, want %q", k, got[k], v)
		}
	}
}

func TestParseLogLine_Postgres_AllSeverities(t *testing.T) {
	cases := []string{
		"DEBUG1", "DEBUG2", "DEBUG3", "DEBUG4", "DEBUG5",
		"INFO", "NOTICE", "WARNING", "ERROR", "LOG", "FATAL", "PANIC",
		"STATEMENT", "DETAIL", "HINT", "CONTEXT", "QUERY", "LOCATION",
	}
	for _, sev := range cases {
		t.Run(sev, func(t *testing.T) {
			in := `2024-01-15 10:30:00.123 UTC [1234] ` + sev + `:  some message`
			p := ParseLogLine(in)
			if p.Kind != LogPreviewPostgres {
				t.Fatalf("%s: kind = %v, want Postgres", sev, p.Kind)
			}
			var got string
			for _, f := range p.Fields {
				if f.Key == "severity" {
					got = f.Value
				}
			}
			if got != sev {
				t.Errorf("severity for %q = %q, want %q", sev, got, sev)
			}
		})
	}
}

func TestParseLogLine_Postgres_PodPrefixStripped(t *testing.T) {
	in := `[pod/postgres-0/postgres] 2024-01-15 10:30:00.123 UTC [1234] ERROR:  relation "foo" does not exist`
	p := ParseLogLine(in)
	if p.Prefix != "[pod/postgres-0/postgres]" {
		t.Errorf("prefix = %q", p.Prefix)
	}
	if p.Kind != LogPreviewPostgres {
		t.Fatalf("kind = %v, want Postgres after prefix strip", p.Kind)
	}
}

func TestParseLogLine_Postgres_RejectsNonPostgres(t *testing.T) {
	cases := []string{
		`plain text`,
		`2024-01-15 10:30:00 [1234] LOG:  missing milliseconds`,            // need .SSS
		`2024-01-15 10:30:00.123 UTC LOG:  no pid bracket`,                 // missing [pid]
		`2024-01-15 10:30:00.123 UTC [abc] LOG:  pid not numeric`,          // pid must be digits
		`2024-01-15 10:30:00.123 UTC [1234] BOGUS:  unknown severity`,      // not a postgres severity
		`2024-01-15 10:30:00.123 UTC [1234] LOG without colon and message`, // no colon after severity
		// klog wins for klog-shaped lines.
		`I0427 16:06:59 1 file.go:1] klog wins`,
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			p := ParseLogLine(in)
			if p.Kind == LogPreviewPostgres {
				t.Errorf("misclassified as Postgres: %q -> %v", in, p.Fields)
			}
		})
	}
}

func TestRenderLogPreviewPane_PostgresTitleIndicator(t *testing.T) {
	in := `2024-01-15 10:30:00.123 UTC [1234] ERROR:  relation "foo" does not exist`
	out := RenderLogPreviewPane(in, 100, 20, 0)
	if !strings.Contains(out, "[POSTGRES]") {
		t.Fatalf("expected [POSTGRES] kind indicator in title; got:\n%s", out)
	}
	for _, want := range []string{"severity", "ERROR", "message", "relation"} {
		if !strings.Contains(out, want) {
			t.Errorf("postgres preview missing %q; got:\n%s", want, out)
		}
	}
}
