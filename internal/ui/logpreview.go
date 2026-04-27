package ui

import (
	"encoding/json"
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
	if len(rest) > 0 && rest[0] == '[' {
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
	if fields, ok := parseLogfmtFields(rest); ok {
		p.Kind = LogPreviewLogfmt
		p.Fields = fields
		return p
	}
	return p
}

// splitLeadingTimestamp returns the leading RFC3339Nano timestamp and the
// remainder, or ("", s, false) when s does not start with a timestamp.
// Mirrors the detection in stripTimestampRaw.
func splitLeadingTimestamp(s string) (string, string, bool) {
	if len(s) < 21 || s[4] != '-' || s[10] != 'T' {
		return "", s, false
	}
	spaceIdx := strings.IndexByte(s, ' ')
	if spaceIdx < 20 || spaceIdx > 35 {
		return "", s, false
	}
	return s[:spaceIdx], s[spaceIdx+1:], true
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
// be JoinHorizontal'd next to the main log view.
func RenderLogPreviewPane(line string, width, height int) string {
	if width < 10 {
		width = 10
	}
	if height < 3 {
		height = 3
	}
	parsed := ParseLogLine(line)

	titleText := " PREVIEW "
	switch parsed.Kind {
	case LogPreviewJSON:
		titleText += HelpKeyStyle.Render("[JSON]")
	case LogPreviewLogfmt:
		titleText += HelpKeyStyle.Render("[LOGFMT]")
	default:
		titleText += HelpKeyStyle.Render("[TEXT]")
	}
	titleBar := FillLinesBg(
		TitleStyle.Width(width).MaxWidth(width).MaxHeight(1).Render(titleText),
		width, BarBg)

	contentHeight := max(height-2, 1)
	contentWidth := max(width-4, 10) // border 2 + padding 2

	bodyLines := buildPreviewBody(parsed, contentWidth)

	if len(bodyLines) > contentHeight {
		bodyLines = bodyLines[:contentHeight]
	}
	for len(bodyLines) < contentHeight {
		bodyLines = append(bodyLines, "")
	}

	bodyContent := strings.Join(bodyLines, "\n")
	bodyContent = FillLinesBg(bodyContent, contentWidth, BaseBg)
	body := FullscreenBorderStyle(width, contentHeight).Render(bodyContent)

	// Empty status bar keeps the panel's row count aligned with the log
	// viewer's title + body + footer layout so JoinHorizontal stays clean.
	// The P binding is already shown in the main log hint bar.
	footer := StatusBarBgStyle.Width(width).MaxWidth(width).MaxHeight(1).Render("")

	return lipgloss.JoinVertical(lipgloss.Left, titleBar, body, footer)
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
