package ui

import (
	"regexp"
	"strings"
)

var yamlNumberRe = regexp.MustCompile(
	`^[+-]?(\d[\d_]*(\.\d[\d_]*)?(e[+-]?\d+)?` +
		`|0x[\da-fA-F_]+` +
		`|0o[0-7_]+` +
		`|\.inf|\.Inf|\.INF` +
		`|\.nan|\.NaN|\.NAN)$`,
)

var yamlBoolValues = map[string]bool{
	"true": true, "false": true,
	"True": true, "False": true,
	"TRUE": true, "FALSE": true,
	"yes": true, "no": true,
	"Yes": true, "No": true,
	"YES": true, "NO": true,
	"on": true, "off": true,
	"On": true, "Off": true,
	"ON": true, "OFF": true,
}

var yamlNullValues = map[string]bool{
	"null": true, "~": true, "Null": true, "NULL": true,
}

var yamlBlockScalarIndicators = map[string]bool{
	"|": true, ">": true, "|-": true, ">-": true, "|+": true, ">+": true,
}

// RenderYAMLContent renders arbitrary YAML content with syntax highlighting, truncated to fit.
func RenderYAMLContent(content string, width, height int) string {
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	var b strings.Builder
	for i, line := range lines {
		b.WriteString(HighlightYAMLLine(Truncate(line, width)))
		if i < len(lines)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// styleYAMLValue applies type-aware styling to a YAML value string.
func styleYAMLValue(val string) string {
	v := strings.TrimSpace(val)
	if v == "" {
		return YamlValueStyle.Render(val)
	}

	lead := val[:len(val)-len(strings.TrimLeft(val, " "))]

	switch {
	case yamlNullValues[v]:
		return lead + YamlNullStyle.Render(v)
	case yamlBoolValues[v]:
		return lead + YamlBoolStyle.Render(v)
	case isYAMLQuotedString(v):
		return lead + YamlStringStyle.Render(v)
	case strings.HasPrefix(v, "&") || strings.HasPrefix(v, "*"):
		return lead + YamlAnchorStyle.Render(v)
	case strings.HasPrefix(v, "!!") || strings.HasPrefix(v, "!"):
		return lead + YamlTagStyle.Render(v)
	case yamlBlockScalarIndicators[v]:
		return lead + YamlBlockScalarStyle.Render(v)
	case yamlNumberRe.MatchString(v):
		return lead + YamlNumberStyle.Render(v)
	}

	return lead + YamlStringStyle.Render(v)
}

// isYAMLQuotedString returns true if v is a single- or double-quoted string.
func isYAMLQuotedString(v string) bool {
	return (strings.HasPrefix(v, `"`) && strings.HasSuffix(v, `"`)) ||
		(strings.HasPrefix(v, "'") && strings.HasSuffix(v, "'"))
}

// isYAMLKey reports whether s looks like a valid YAML mapping key.
func isYAMLKey(s string) bool {
	if s == "" {
		return false
	}
	if (s[0] == '"' && s[len(s)-1] == '"') ||
		(s[0] == '\'' && s[len(s)-1] == '\'') {
		return true
	}
	return !strings.Contains(s, " ")
}

// renderKeyValue renders a YAML key: value pair with syntax highlighting.
func renderKeyValue(indent, key, rest string) string {
	styledKey := YamlKeyStyle.Render(key)
	if len(rest) <= 1 {
		return indent + styledKey + YamlPunctuationStyle.Render(":")
	}

	colon := YamlPunctuationStyle.Render(":")
	valPart := rest[1:]

	if ci := findInlineComment(valPart); ci >= 0 {
		return indent + styledKey + colon +
			styleYAMLValue(valPart[:ci]) +
			YamlCommentStyle.Render(valPart[ci:])
	}

	return indent + styledKey + colon + styleYAMLValue(valPart)
}

// HighlightYAMLLine applies syntax highlighting to a single YAML line.
func HighlightYAMLLine(line string) string {
	var foldPrefix string
	cleaned := line
	for _, r := range line {
		if r == '▾' || r == '▸' {
			runes := []rune(line)
			for i, cr := range runes {
				if cr == '▾' || cr == '▸' {
					foldPrefix = string(runes[:i+1])
					cleaned = string(runes[i+1:])
					break
				}
			}
			break
		}
		if r != ' ' {
			break
		}
	}
	if foldPrefix != "" {
		return foldPrefix + highlightYAMLContent(cleaned)
	}
	return highlightYAMLContent(line)
}

// highlightYAMLContent applies syntax highlighting to YAML content (without
// fold indicators).
func highlightYAMLContent(line string) string {
	trimmed := strings.TrimLeft(line, " ")
	indent := line[:len(line)-len(trimmed)]

	if strings.HasPrefix(trimmed, "#") {
		return YamlCommentStyle.Render(line)
	}

	if strings.HasPrefix(trimmed, "- ") {
		marker := YamlPunctuationStyle.Render("- ")
		content := trimmed[2:]

		if colonIdx := findYAMLColon(content); colonIdx > 0 {
			key := content[:colonIdx]
			rest := content[colonIdx:]
			if isYAMLKey(key) {
				return indent + marker + renderKeyValue("", key, rest)
			}
		}

		return indent + marker + styleYAMLValue(content)
	}

	if colonIdx := findYAMLColon(trimmed); colonIdx > 0 {
		key := trimmed[:colonIdx]
		rest := trimmed[colonIdx:]
		if isYAMLKey(key) {
			return renderKeyValue(indent, key, rest)
		}
	}

	return YamlValueStyle.Render(line)
}

// findYAMLColon finds the index of the first colon that looks like a YAML
// key-value separator.
func findYAMLColon(s string) int {
	inSingle := false
	inDouble := false
	for i := range len(s) {
		switch s[i] {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case ':':
			if !inSingle && !inDouble {
				if i == len(s)-1 || s[i+1] == ' ' {
					return i
				}
			}
		}
	}
	return -1
}

// findInlineComment returns the index of an inline comment (# preceded by
// whitespace) in a YAML value, or -1 if none is found.
func findInlineComment(s string) int {
	inSingle := false
	inDouble := false
	for i := range len(s) {
		switch s[i] {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble && i > 0 && s[i-1] == ' ' {
				return i - 1
			}
		}
	}
	return -1
}
