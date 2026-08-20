package logger

import (
	"fmt"
	"regexp"
	"strings"
)

// secretKeyAlt lists the credential-shaped key names recognized by the
// inline key=value/key: value rule and the YAML block-scalar header rule
// below. Shared so both stay in sync.
const secretKeyAlt = `(?:password|passwd|pwd|db[_-]?pass(?:word)?|secret|token|api[_-]?key|access[_-]?key|private[_-]?key|client[_-]?secret|dsn|database[_-]?url|connection[_-]?string)`

// redactPatterns is intentionally conservative: a false positive breaks
// debugging stderr output, a missed novel pattern doesn't. Redact applies
// each one line at a time so `\s*` can't cross into an unrelated next line.
var redactPatterns = []struct {
	re   *regexp.Regexp
	repl string
}{
	// JWT three-segment base64url tokens (used by AWS STS, OIDC, etc.).
	{regexp.MustCompile(`eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`), "[REDACTED-JWT]"},
	// AWS access keys: well-defined 4-letter prefix + 16 alphanumerics.
	{regexp.MustCompile(`(?:AKIA|ASIA|AGPA|AROA|AIDA|ANPA|ANVA|AIPA)[A-Z0-9]{16}`), "[REDACTED-AWS-KEY]"},
	// GCP OAuth access tokens (gke-gcloud-auth-plugin emits these to stderr on failure).
	{regexp.MustCompile(`ya29\.[A-Za-z0-9_-]{20,}`), "[REDACTED-GCP-TOKEN]"},
	// GitHub tokens (PAT, OAuth, server, user).
	{regexp.MustCompile(`gh[opsu]_[A-Za-z0-9]{36,}`), "[REDACTED-GH-TOKEN]"},
	// URLs with embedded credentials: keep scheme/host but redact the user:pass.
	// Any URI scheme (postgres, mysql, mongodb, redis, amqp, ...), not just
	// the well-known ones - connection strings carry these just as often.
	{regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.-]*://)[^:@\s/]*:[^@\s/]+@`), "${1}[REDACTED-CREDS]@"},
	// Bearer tokens in Authorization headers.
	{regexp.MustCompile(`(?i)(Bearer\s+)[A-Za-z0-9._-]{20,}`), "${1}[REDACTED-BEARER]"},
	// RE2 has no lookbehind: the boundary char and value quote are captured
	// and replayed so unquoted (or unterminated-quote) output keeps its shape.
	// Fully quoted values are handled by redactQuotedValues before this runs.
	{regexp.MustCompile(`(?i)(^|[^\w/])([A-Za-z0-9_-]*` + secretKeyAlt + `["']?\s*[=:]\s*)(["']?)[^\s&"',;]+`), "${1}${2}${3}[REDACTED]"},
	// kubectl --from-literal=KEY=VALUE — keep KEY, redact VALUE.
	{regexp.MustCompile(`(--from-literal=[^=\s]+=)[^\s"']+`), "${1}[REDACTED]"},
}

// A header this regex misses leaves its whole block body unredacted, so it
// must accept "|2" / ">4-" indicators and trailing YAML comments too.
var blockScalarHeaderRe = regexp.MustCompile(`(?i)^(\s*)[A-Za-z0-9_-]*` + secretKeyAlt + `["']?\s*:\s*[|>][0-9+-]{0,2}\s*(?:#.*)?$`)

// Quoted bodies are walked by redactQuotedValues, not matched greedily: a
// value whose raw text ends in a backslash would overrun its closing quote,
// swallow the next field's key label, and leak that field's value.
var quotedValueLabelRe = regexp.MustCompile(`(?i)(?:^|[^\w/])[A-Za-z0-9_-]*` + secretKeyAlt + `["']?\s*[=:]\s*["']`)

// The overrun signature: a second secret-key label inside one quoted span.
var innerSecretLabelRe = regexp.MustCompile(`(?i)[A-Za-z0-9_-]*` + secretKeyAlt + `["']?\s*[=:]`)

// Deliberate non-goal: base64 under a generic key like "data:" stays
// unredacted. Entropy checks misfire on kubectl/helm's legitimate base64.
// Scoping to Secret "data:" needs doc-boundary tracking this redactor avoids.

// Redact scrubs likely-sensitive content from s before it reaches the log
// file. Idempotent. Processes one line at a time, plus a stateful pass for
// YAML block scalars ("password: |-" plus indented body) a per-line regex misses.
func Redact(s string) string {
	if !strings.Contains(s, "\n") {
		return redactLine(s)
	}

	lines := strings.Split(s, "\n")
	inBlock := false
	blockIndent := 0
	for i, line := range lines {
		if inBlock {
			if strings.TrimSpace(line) == "" {
				continue
			}
			if indent := leadingIndent(line); indent > blockIndent {
				lines[i] = line[:indent] + "[REDACTED]"
				continue
			}
			inBlock = false
		}
		if m := blockScalarHeaderRe.FindStringSubmatch(line); m != nil {
			blockIndent = len(m[1])
			inBlock = true
		}
		lines[i] = redactLine(line)
	}
	return strings.Join(lines, "\n")
}

func redactLine(line string) string {
	line = redactQuotedValues(line)
	for _, p := range redactPatterns {
		line = p.re.ReplaceAllString(line, p.repl)
	}
	return line
}

func redactQuotedValues(line string) string {
	var b strings.Builder
	rest := line
	for {
		loc := quotedValueLabelRe.FindStringIndex(rest)
		if loc == nil {
			b.WriteString(rest)
			return b.String()
		}
		open := loc[1] - 1
		quote := rest[open]
		end := scanQuoted(rest, open+1, quote)
		if end < 0 {
			// Unterminated: the generic pattern redacts what it can.
			b.WriteString(rest[:loc[1]])
			rest = rest[loc[1]:]
			continue
		}
		if innerSecretLabelRe.MatchString(rest[open+1 : end]) {
			// The escape-aware close swallowed another field's label, so
			// re-terminate at the first quote and let the loop redact
			// that next field on its own.
			if i := strings.IndexByte(rest[open+1:], quote); i >= 0 {
				end = open + 1 + i
			}
		}
		b.WriteString(rest[:open+1])
		b.WriteString("[REDACTED]")
		b.WriteByte(quote)
		rest = rest[end+1:]
	}
}

// scanQuoted returns the index of the closing quote, treating any
// backslash-prefixed byte as escaped, or -1 when the quote never closes.
func scanQuoted(s string, start int, quote byte) int {
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case quote:
			return i
		}
	}
	return -1
}

func leadingIndent(line string) int {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return i
}

// RedactErr wraps err with output, redacted - subprocess output (e.g. helm
// values) can carry secrets and this error reaches the log file and status bar.
func RedactErr(err error, output []byte) error {
	return fmt.Errorf("%w: %s", err, Redact(strings.TrimSpace(string(output))))
}
