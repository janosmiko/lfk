package ui

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// LogPreviewKind identifies how the previewed log line was parsed.
type LogPreviewKind int

const (
	LogPreviewText LogPreviewKind = iota
	LogPreviewJSON
	LogPreviewLogfmt
	// LogPreviewKlog covers the klog/glog format used by kubelet,
	// kube-proxy, leaderelection, and most controller binaries:
	// `LMMDD HH:MM:SS.uuuuuu threadid file:line] msg`.
	LogPreviewKlog
	// LogPreviewZap covers the zap dev/console encoder shape that
	// controller-runtime emits (after a leading RFC3339 timestamp):
	// `LEVEL\t[LOGGER\t]MESSAGE[\t{JSON_FIELDS}]`. The optional trailing
	// JSON object is unpacked into individual fields so the structured
	// context (controller name, namespace, reconcileID, ...) appears
	// flat in the preview pane.
	LogPreviewZap
	// LogPreviewNginx covers the NCSA "Combined Log Format" emitted by
	// nginx, Apache, Traefik, and most ingress controllers:
	// `host - user [DD/Mon/YYYY:HH:MM:SS +ZZZZ] "METHOD path proto" status bytes "referer" "ua"`.
	LogPreviewNginx
	// LogPreviewEnvoy covers the Envoy/Istio default text access log
	// format (15 positional fields after a bracketed RFC3339 timestamp):
	// `[time] "METHOD path proto" status flags rx tx dur upstream_time "xff" "ua" "rid" "auth" "upstream_host"`.
	LogPreviewEnvoy
	// LogPreviewJava covers Spring Boot's default console pattern and
	// the plain Logback BasicConfigurator default. Together these cover
	// the vast majority of JVM workloads in a Kubernetes cluster.
	LogPreviewJava
	// LogPreviewPostgres covers the PostgreSQL server log produced by
	// the default `log_line_prefix='%m [%p] '` setting (the value used
	// by the upstream postgres Docker image).
	LogPreviewPostgres
)

// LogPreviewField is a single key/value pair extracted from a structured log line.
type LogPreviewField struct {
	Key   string
	Value string
}

// ParsedLogPreview holds the result of parsing a single log line for preview.
type ParsedLogPreview struct {
	Kind   LogPreviewKind
	Prefix string // pod/container bracket prefix, e.g. "[pod/foo/bar]" or ""
	Time   string // RFC3339Nano timestamp prefix, or ""
	Body   string // raw line with prefix and timestamp stripped
	Fields []LogPreviewField
}

// ParseLogLine extracts the optional pod prefix and timestamp, then tries
// JSON and logfmt parsing in order. Falls back to plain text when neither
// matches. Parsing is intentionally cheap and never panics.
func ParseLogLine(line string) ParsedLogPreview {
	p := ParsedLogPreview{Kind: LogPreviewText, Body: line}
	rest := line
	// Pod prefixes are kubectl-shape: "[pod/<name>/<container>] ". The
	// "[pod/" gate is required to avoid eating any leading bracketed
	// token — Envoy access logs lead with a bracketed RFC3339 timestamp
	// (`[2024-01-15T10:30:00.000Z] ...`) that would otherwise be
	// misclassified as a pod prefix and starve the structural parsers.
	if strings.HasPrefix(rest, "[pod/") {
		if i := strings.Index(rest, "] "); i > 0 {
			p.Prefix = rest[:i+1]
			rest = rest[i+2:]
		}
	}
	if t, after, ok := splitLeadingTimestamp(rest); ok {
		p.Time = t
		rest = after
	}
	p.Body = rest

	trimmed := strings.TrimSpace(rest)
	if len(trimmed) >= 2 && trimmed[0] == '{' && trimmed[len(trimmed)-1] == '}' {
		if fields, ok := parseJSONFields(trimmed); ok {
			p.Kind = LogPreviewJSON
			p.Fields = fields
			return p
		}
	}
	// klog must run before logfmt: a klog message body that contains
	// `key=value` fragments would otherwise be promoted to logfmt and
	// the structural metadata (level, caller, thread) would be lost.
	if fields, ok := parseKlogFields(rest); ok {
		p.Kind = LogPreviewKlog
		p.Fields = fields
		return p
	}
	// zap (controller-runtime dev/console encoder) must also run before
	// logfmt — its embedded JSON fields contain colon-bearing values that
	// the loose logfmt regex never matches, but the message body alone
	// could carry stray `key=value` fragments and hijack classification.
	if fields, ok := parseZapFields(rest); ok {
		p.Kind = LogPreviewZap
		p.Fields = fields
		return p
	}
	// HTTP access logs (NGINX/Apache Combined and Envoy/Istio default)
	// must precede logfmt: a request URL with a query string carries
	// `key=value` pairs that the loose logfmt detector would otherwise
	// claim, hiding the request method/status/bytes structure.
	if fields, ok := parseNginxFields(rest); ok {
		p.Kind = LogPreviewNginx
		p.Fields = fields
		return p
	}
	if fields, ok := parseEnvoyFields(rest); ok {
		p.Kind = LogPreviewEnvoy
		p.Fields = fields
		return p
	}
	if fields, ok := parsePostgresFields(rest); ok {
		p.Kind = LogPreviewPostgres
		p.Fields = fields
		return p
	}
	if fields, ok := parseJavaFields(rest); ok {
		p.Kind = LogPreviewJava
		p.Fields = fields
		return p
	}
	if fields, ok := parseLogfmtFields(rest); ok {
		p.Kind = LogPreviewLogfmt
		p.Fields = fields
		return p
	}
	return p
}

// klogLine matches a klog/glog formatted log line:
//
//	LMMDD HH:MM:SS.uuuuuu threadid file:line] msg
//
// L is one of I/W/E/F (Info/Warning/Error/Fatal). The thread id is
// right-aligned with leading whitespace in real klog output, so \s+ is
// used between the time and the thread id. The trailing `]` after the
// file:line is the signature element that distinguishes klog from
// free-form text that happens to start with a capital letter.
var klogLine = regexp.MustCompile(`^([IWEF])(\d{4}) (\d{2}:\d{2}:\d{2}\.\d+)\s+(\d+)\s+([^:\s]+:\d+)\]\s*(.*)$`)

var klogLevelName = map[byte]string{
	'I': "Info",
	'W': "Warning",
	'E': "Error",
	'F': "Fatal",
}

// zapLevelName maps the uppercase zap level tokens emitted by zap's
// CapitalLevelEncoder (the controller-runtime default) to display names.
// DPANIC and PANIC collapse to Fatal because to a log reader they signal
// the same "process is in trouble" severity.
var zapLevelName = map[string]string{
	"DEBUG":   "Debug",
	"INFO":    "Info",
	"WARN":    "Warning",
	"WARNING": "Warning",
	"ERROR":   "Error",
	"DPANIC":  "Fatal",
	"PANIC":   "Fatal",
	"FATAL":   "Fatal",
}

// parseZapFields recognises the zap dev/console encoder shape:
//
//	LEVEL\t[LOGGER\t]MESSAGE[\t{JSON_FIELDS}]
//
// produced by controller-runtime after a leading RFC3339 timestamp.
// LEVEL must be a known token (see zapLevelName); the trailing JSON
// object, when present and parseable, is unpacked so structured context
// (controller, namespace, reconcileID, ...) appears flat in the preview.
//
// The parser refuses anything outside the LEVEL[\tLOGGER]\tMESSAGE[\tJSON]
// shape: too many tab segments, an unknown level token, or an empty
// message all return false rather than risk a sloppy match that loses
// structural information.
func parseZapFields(s string) ([]LogPreviewField, bool) {
	parts := strings.Split(s, "\t")
	if len(parts) < 2 {
		return nil, false
	}
	levelName, ok := zapLevelName[parts[0]]
	if !ok {
		return nil, false
	}
	end := len(parts)
	var jsonFields []LogPreviewField
	last := strings.TrimSpace(parts[end-1])
	if len(last) >= 2 && last[0] == '{' && last[len(last)-1] == '}' {
		if jf, jOk := parseJSONFields(last); jOk {
			jsonFields = jf
			end--
		}
	}
	middle := parts[1:end]
	var logger, message string
	switch len(middle) {
	case 1:
		message = middle[0]
	case 2:
		logger, message = middle[0], middle[1]
	default:
		// 0 or 3+ middle parts: not the zap shape we recognise.
		return nil, false
	}
	if message == "" {
		return nil, false
	}
	out := make([]LogPreviewField, 0, 3+len(jsonFields))
	out = append(out, LogPreviewField{Key: "level", Value: levelName})
	if logger != "" {
		out = append(out, LogPreviewField{Key: "logger", Value: logger})
	}
	out = append(out, LogPreviewField{Key: "message", Value: message})
	out = append(out, jsonFields...)
	return out, true
}

// nginxCombined matches the NCSA "Combined Log Format" used by nginx,
// Apache, Traefik, and most ingress controllers as their default access
// log line:
//
//	host - user [DD/Mon/YYYY:HH:MM:SS +ZZZZ] "METHOD path proto" status bytes "referer" "ua"
//
// host is permissive (\S+) so IPv4, IPv6, and DNS names all match. The
// quoted referer/user_agent blocks are required (Combined, not the
// shorter Common Log Format) so 7-field "common" lines are rejected.
var nginxCombined = regexp.MustCompile(`^(\S+) \S+ (\S+) \[(\d{2}/[A-Z][a-z]{2}/\d{4}:\d{2}:\d{2}:\d{2} [+-]\d{4})\] "([A-Z]+) (\S+) (HTTP/[\d.]+)" (\d+) (\d+|-) "([^"]*)" "([^"]*)"$`)

// parseNginxFields extracts request method, path, status, bytes, and
// the surrounding metadata from an NCSA Combined log line. Field order
// puts the request summary (method, path, status) first so the most
// useful information sits at the top of the preview pane.
func parseNginxFields(s string) ([]LogPreviewField, bool) {
	m := nginxCombined.FindStringSubmatch(s)
	if m == nil {
		return nil, false
	}
	return []LogPreviewField{
		{Key: "method", Value: m[4]},
		{Key: "path", Value: m[5]},
		{Key: "status", Value: m[7]},
		{Key: "bytes", Value: m[8]},
		{Key: "protocol", Value: m[6]},
		{Key: "referer", Value: m[9]},
		{Key: "user_agent", Value: m[10]},
		{Key: "client", Value: m[1]},
		{Key: "user", Value: m[2]},
		{Key: "time", Value: m[3]},
	}, true
}

// envoyAccessLog matches Envoy's default text-format access log (also
// emitted by Istio sidecars):
//
//	[RFC3339] "METHOD path proto" status flags rx tx duration upstream_time "xff" "ua" "rid" "auth" "upstream_host"
//
// response_flags is a short alphabetic token (e.g. UH, NR, DI, FI) or
// "-". upstream_time is a number or "-" when no upstream was contacted.
// The five trailing quoted blocks are required, so partial Envoy
// configurations with fewer columns are rejected rather than misparsed.
var envoyAccessLog = regexp.MustCompile(`^\[(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+Z)\] "([A-Z]+) (\S+) (HTTP/[\d.]+)" (\d+) (\S+) (\d+) (\d+) (\d+|-) (\d+|-) "([^"]*)" "([^"]*)" "([^"]*)" "([^"]*)" "([^"]*)"$`)

// parseEnvoyFields extracts the 15-positional Envoy default access log.
// Field order surfaces the request summary first, then the
// quantitative fields (flags, bytes, durations), then the upstream and
// identity context.
func parseEnvoyFields(s string) ([]LogPreviewField, bool) {
	m := envoyAccessLog.FindStringSubmatch(s)
	if m == nil {
		return nil, false
	}
	return []LogPreviewField{
		{Key: "method", Value: m[2]},
		{Key: "path", Value: m[3]},
		{Key: "status", Value: m[5]},
		{Key: "response_flags", Value: m[6]},
		{Key: "bytes_received", Value: m[7]},
		{Key: "bytes_sent", Value: m[8]},
		{Key: "duration_ms", Value: m[9]},
		{Key: "upstream_time", Value: m[10]},
		{Key: "protocol", Value: m[4]},
		{Key: "user_agent", Value: m[12]},
		{Key: "x_forwarded_for", Value: m[11]},
		{Key: "request_id", Value: m[13]},
		{Key: "authority", Value: m[14]},
		{Key: "upstream_host", Value: m[15]},
		{Key: "time", Value: m[1]},
	}, true
}

// javaLevelName normalises the SLF4J / Logback / Spring Boot level
// tokens (the only frameworks worth recognising here cover all of
// these). DEBUG and INFO are common; FATAL is rare in pure Logback but
// emitted by some shims.
var javaLevelName = map[string]string{
	"TRACE":   "Trace",
	"DEBUG":   "Debug",
	"INFO":    "Info",
	"WARN":    "Warning",
	"WARNING": "Warning",
	"ERROR":   "Error",
	"FATAL":   "Fatal",
}

// javaSpringBoot matches Spring Boot's default ConsoleAppender pattern
// after the leading RFC3339 timestamp has been peeled off by the shared
// splitLeadingTimestamp helper. The full pattern is:
//
//	%d{yyyy-MM-dd'T'HH:mm:ss.SSSXXX} %5p ${PID:- } --- [%t] %-40.40c{1.} : %m
//
// produced as e.g.
//
//	2024-01-15T10:30:00.123+00:00  INFO 12345 --- [main] c.e.MyService : Server started
//
// After timestamp strip the parser sees `<spaces>INFO 12345 --- [main]
// c.e.MyService : Server started`. The "--- [thread]" sentinel is the
// strongest distinguishing marker for Spring Boot output and rarely
// collides with anything else.
var javaSpringBoot = regexp.MustCompile(`^\s*(TRACE|DEBUG|INFO|WARN|WARNING|ERROR|FATAL)\s+(\d+)\s+---\s+\[([^\]]+)\]\s+([\w.$]+)\s*:\s*(.*)$`)

// javaLogback matches Logback's BasicConfigurator default pattern:
//
//	%d{HH:mm:ss.SSS} [%thread] %-5level %logger{36} - %msg
//
// produced as e.g.
//
//	10:30:00.123 [main] INFO com.example.MyService - Connecting to database
//
// The " - " separator before the message is part of the canonical
// pattern; lines without it are rejected to avoid claiming free-form
// text that happens to contain a level word.
var javaLogback = regexp.MustCompile(`^(\d{2}:\d{2}:\d{2}[.,]\d{3,})\s+\[([^\]]+)\]\s+(TRACE|DEBUG|INFO|WARN|WARNING|ERROR|FATAL)\s+([\w.$]+)\s+-\s+(.*)$`)

// parseJavaFields tries the Spring Boot pattern first, then the plain
// Logback pattern. Field order leads with the level, logger, and
// message because those are what a developer scans for; thread, pid,
// and time follow. For Spring Boot the leading RFC3339 timestamp is
// extracted earlier by splitLeadingTimestamp and lives in p.Time, so
// time does not appear in the returned Fields. For Logback the pattern
// emits only `HH:mm:ss.SSS` (no date), which the timestamp helper
// rejects, so the time stays in the parsed body and the parser
// includes it in Fields.
func parseJavaFields(s string) ([]LogPreviewField, bool) {
	if m := javaSpringBoot.FindStringSubmatch(s); m != nil {
		return []LogPreviewField{
			{Key: "level", Value: javaLevelName[m[1]]},
			{Key: "logger", Value: m[4]},
			{Key: "message", Value: m[5]},
			{Key: "thread", Value: m[3]},
			{Key: "pid", Value: m[2]},
		}, true
	}
	if m := javaLogback.FindStringSubmatch(s); m != nil {
		return []LogPreviewField{
			{Key: "level", Value: javaLevelName[m[3]]},
			{Key: "logger", Value: m[4]},
			{Key: "message", Value: m[5]},
			{Key: "thread", Value: m[2]},
			{Key: "time", Value: m[1]},
		}, true
	}
	return nil, false
}

// postgresLine matches the PostgreSQL server log produced by the
// default `log_line_prefix='%m [%p] '`:
//
//	YYYY-MM-DD HH:MM:SS.SSS TZ [pid] SEVERITY:  message
//
// Severity covers the standard production levels plus the
// continuation tags (STATEMENT, DETAIL, HINT, CONTEXT, QUERY,
// LOCATION) that postgres emits as separate lines after a primary log
// entry.
var postgresLine = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d+ \w+) \[(\d+)\] (DEBUG[1-5]|INFO|NOTICE|WARNING|ERROR|LOG|FATAL|PANIC|STATEMENT|DETAIL|HINT|CONTEXT|QUERY|LOCATION):\s+(.*)$`)

// parsePostgresFields extracts severity, message, pid, and time from a
// PostgreSQL server log line. Severity is preserved verbatim (LOG,
// ERROR, WARNING, ...) rather than normalised because the labels are
// already the conventional names a postgres operator scans for.
func parsePostgresFields(s string) ([]LogPreviewField, bool) {
	m := postgresLine.FindStringSubmatch(s)
	if m == nil {
		return nil, false
	}
	return []LogPreviewField{
		{Key: "severity", Value: m[3]},
		{Key: "message", Value: m[4]},
		{Key: "pid", Value: m[2]},
		{Key: "time", Value: m[1]},
	}, true
}

// parseKlogFields extracts level, time, message, caller, and thread from
// a klog-formatted line. Field order is fixed (time, level, message,
// caller, thread) so the preview pane shows the most useful information
// at the top — RenderLogPreviewPane preserves the slice order.
func parseKlogFields(s string) ([]LogPreviewField, bool) {
	m := klogLine.FindStringSubmatch(s)
	if m == nil {
		return nil, false
	}
	return []LogPreviewField{
		{Key: "time", Value: m[2] + " " + m[3]},
		{Key: "level", Value: klogLevelName[m[1][0]]},
		{Key: "message", Value: m[6]},
		{Key: "caller", Value: m[5]},
		{Key: "thread", Value: m[4]},
	}, true
}

// splitLeadingTimestamp returns the leading RFC3339Nano timestamp and the
// remainder, or ("", s, false) when s does not start with a timestamp.
// This is the canonical primitive; stripTimestampRaw delegates to it.
// Minimum length: "2024-01-15T10:30:00Z " = 21 chars.
//
// The separator after the timestamp may be either a space (kubectl logs)
// or a tab (controller-runtime/zap dev encoder). Both producers are
// common in a Kubernetes context.
func splitLeadingTimestamp(s string) (string, string, bool) {
	if len(s) < 21 || s[4] != '-' || s[10] != 'T' {
		return "", s, false
	}
	sepIdx := strings.IndexFunc(s, func(r rune) bool { return r == ' ' || r == '\t' })
	if sepIdx < 20 || sepIdx > 35 {
		return "", s, false
	}
	return s[:sepIdx], s[sepIdx+1:], true
}

func parseJSONFields(s string) ([]LogPreviewField, bool) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return nil, false
	}
	if len(raw) == 0 {
		return nil, false
	}
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		ri, rj := jsonKeyRank(keys[i]), jsonKeyRank(keys[j])
		if ri != rj {
			return ri < rj
		}
		return keys[i] < keys[j]
	})
	fields := make([]LogPreviewField, 0, len(keys))
	for _, k := range keys {
		fields = append(fields, LogPreviewField{Key: k, Value: jsonValueString(raw[k])})
	}
	return fields, true
}

// jsonKeyRank orders well-known structured-logging keys ahead of arbitrary
// fields so the most useful information sits at the top of the panel.
func jsonKeyRank(k string) int {
	switch strings.ToLower(k) {
	case "time", "timestamp", "ts", "@timestamp":
		return 0
	case "level", "lvl", "severity":
		return 1
	case "msg", "message":
		return 2
	case "logger", "caller", "source":
		return 3
	case "error", "err":
		return 4
	}
	return 100
}

func jsonValueString(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var v any
	if err := json.Unmarshal(raw, &v); err == nil {
		b, err := json.MarshalIndent(v, "", "  ")
		if err == nil {
			return string(b)
		}
	}
	return string(raw)
}

// logfmtPair matches `key=bareword` and `key="quoted value"` pairs. Keys are
// alphanumeric plus _ . - to cover the common ecosystem (logfmt, klog, slog).
var logfmtPair = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_.\-]*)=(?:"((?:[^"\\]|\\.)*)"|([^\s"]+))`)

func parseLogfmtFields(s string) ([]LogPreviewField, bool) {
	matches := logfmtPair.FindAllStringSubmatch(s, -1)
	// Require at least 2 distinct pairs to count as logfmt — a single
	// `foo=bar` token is too easy to hit on free-form text.
	if len(matches) < 2 {
		return nil, false
	}
	fields := make([]LogPreviewField, 0, len(matches))
	for _, m := range matches {
		v := m[2]
		if v == "" {
			v = m[3]
		}
		fields = append(fields, LogPreviewField{Key: m[1], Value: v})
	}
	return fields, true
}

// RenderLogPreviewPane renders a side panel showing the parsed view of a
// single log line. Output is exactly width columns wide and height+2 rows
// tall (matching RenderLogViewer's title + body + footer layout) so it can
// be JoinHorizontal'd next to the main log view. scroll is the number of
// body rows to skip from the top; it is clamped to [0, max] internally so
// callers can pass an unclamped value and rely on LogPreviewMaxScroll for
// the upper bound when they need it (e.g. to gate key handlers).
//
// omitFooter, when true, suppresses the empty status-bar padding row at the
// bottom so the caller can JoinVertical a full-width footer below the
// JoinHorizontal'd panes (see RenderLogFooter and issue #71). The output is
// one row shorter than the default rendering when omitFooter=true.
func RenderLogPreviewPane(line string, width, height, scroll int, omitFooter bool) string {
	if width < 10 {
		width = 10
	}
	if height < 3 {
		height = 3
	}
	parsed := ParseLogLine(line)

	contentHeight := max(height-2, 1)
	contentWidth := max(width-4, 10) // border 2 + padding 2

	bodyLines := buildPreviewBody(parsed, contentWidth)
	totalLines := len(bodyLines)
	scroll = clampPreviewScroll(scroll, totalLines, contentHeight)
	if scroll > 0 {
		bodyLines = bodyLines[scroll:]
	}

	titleText := " PREVIEW "
	switch parsed.Kind {
	case LogPreviewJSON:
		titleText += HelpKeyStyle.Render("[JSON]")
	case LogPreviewLogfmt:
		titleText += HelpKeyStyle.Render("[LOGFMT]")
	case LogPreviewKlog:
		titleText += HelpKeyStyle.Render("[KLOG]")
	case LogPreviewZap:
		titleText += HelpKeyStyle.Render("[ZAP]")
	case LogPreviewNginx:
		titleText += HelpKeyStyle.Render("[NGINX]")
	case LogPreviewEnvoy:
		titleText += HelpKeyStyle.Render("[ENVOY]")
	case LogPreviewJava:
		titleText += HelpKeyStyle.Render("[JAVA]")
	case LogPreviewPostgres:
		titleText += HelpKeyStyle.Render("[POSTGRES]")
	default:
		titleText += HelpKeyStyle.Render("[TEXT]")
	}
	if totalLines > contentHeight {
		// Show "topVisibleRow/totalRows" so the user can see how far they
		// have scrolled and how much body content is still hidden.
		titleText += "  " + DimStyle.Render(scrollPositionLabel(scroll, totalLines))
	}
	titleBar := FillLinesBg(
		TitleStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(titleText),
		width, BarBg)

	if len(bodyLines) > contentHeight {
		bodyLines = bodyLines[:contentHeight]
	}
	for len(bodyLines) < contentHeight {
		bodyLines = append(bodyLines, "")
	}

	bodyContent := strings.Join(bodyLines, "\n")
	bodyContent = FillLinesBg(bodyContent, contentWidth, BaseBg)
	body := FullscreenBorderStyle(width, contentHeight).Render(bodyContent)

	if omitFooter {
		return lipgloss.JoinVertical(lipgloss.Left, titleBar, body)
	}

	// Empty status bar keeps the panel's row count aligned with the log
	// viewer's title + body + footer layout so JoinHorizontal stays clean.
	// The P / J / K bindings are advertised in the main log hint bar.
	footer := StatusBarBgStyle.Width(width).MaxWidth(width).MaxHeight(1).Render("")

	return lipgloss.JoinVertical(lipgloss.Left, titleBar, body, footer)
}

// LogPreviewMaxScroll returns the largest valid scroll offset for
// RenderLogPreviewPane given the same line and dimensions. Callers use this
// to clamp scroll-down handlers without re-rendering.
func LogPreviewMaxScroll(line string, width, height int) int {
	if width < 10 {
		width = 10
	}
	if height < 3 {
		height = 3
	}
	parsed := ParseLogLine(line)
	contentHeight := max(height-2, 1)
	contentWidth := max(width-4, 10)
	bodyLines := buildPreviewBody(parsed, contentWidth)
	return max(0, len(bodyLines)-contentHeight)
}

func clampPreviewScroll(scroll, total, contentHeight int) int {
	if scroll < 0 {
		return 0
	}
	maxScroll := max(0, total-contentHeight)
	if scroll > maxScroll {
		return maxScroll
	}
	return scroll
}

func scrollPositionLabel(scroll, total int) string {
	// scroll is 0-based row index of the top visible row. Show 1-based to
	// match how editors present line counters.
	return fmt.Sprintf("%d/%d", scroll+1, total)
}

func buildPreviewBody(parsed ParsedLogPreview, contentWidth int) []string {
	if parsed.Prefix == "" && parsed.Time == "" && parsed.Body == "" && len(parsed.Fields) == 0 {
		return []string{DimStyle.Render("(no log line selected)")}
	}
	var lines []string
	if parsed.Prefix != "" {
		lines = append(lines, DimStyle.Render("source ")+parsed.Prefix)
	}
	if parsed.Time != "" {
		lines = append(lines, DimStyle.Render("time   ")+parsed.Time)
	}
	if (parsed.Prefix != "" || parsed.Time != "") && (len(parsed.Fields) > 0 || parsed.Body != "") {
		lines = append(lines, "")
	}
	if len(parsed.Fields) > 0 {
		lines = append(lines, renderPreviewFields(parsed.Fields, contentWidth)...)
		return lines
	}
	body := strings.TrimSpace(parsed.Body)
	if body == "" {
		lines = append(lines, DimStyle.Render("(empty)"))
		return lines
	}
	for src := range strings.SplitSeq(body, "\n") {
		lines = append(lines, wrapLine(sanitizeLogLine(src, ConfigLogRenderAnsi), contentWidth)...)
	}
	return lines
}

func renderPreviewFields(fields []LogPreviewField, width int) []string {
	keyWidth := 0
	for _, f := range fields {
		if l := len([]rune(f.Key)); l > keyWidth {
			keyWidth = l
		}
	}
	if keyWidth > 18 {
		keyWidth = 18
	}
	keyStyle := lipgloss.NewStyle().Foreground(ThemeColor(ColorPrimary)).Bold(true)
	sepStyle := DimStyle
	prefixCells := keyWidth + 3 // " : " separator
	availForValue := max(width-prefixCells, 10)

	out := make([]string, 0, len(fields))
	for _, f := range fields {
		key, keyDisplayLen := truncateKeyForPreview(f.Key, keyWidth)
		prefix := keyStyle.Render(key) + strings.Repeat(" ", keyWidth-keyDisplayLen) + sepStyle.Render(" : ")
		first := true
		for line := range strings.SplitSeq(f.Value, "\n") {
			chunks := wrapLine(sanitizeLogLine(line, ConfigLogRenderAnsi), availForValue)
			if len(chunks) == 0 {
				chunks = []string{""}
			}
			for _, chunk := range chunks {
				if first {
					out = append(out, prefix+chunk)
					first = false
				} else {
					out = append(out, strings.Repeat(" ", prefixCells)+chunk)
				}
			}
		}
	}
	return out
}

// truncateKeyForPreview clamps a key to at most keyWidth runes, appending an
// ellipsis when truncation occurs so the user can tell the displayed key has
// been shortened. Returns the rendered key and its rune count for padding.
func truncateKeyForPreview(key string, keyWidth int) (string, int) {
	runes := []rune(key)
	if len(runes) <= keyWidth {
		return key, len(runes)
	}
	if keyWidth <= 1 {
		return "…", 1
	}
	return string(runes[:keyWidth-1]) + "…", keyWidth
}
