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
	// and replayed so quoted output ("password": "x") keeps its quoting.
	{regexp.MustCompile(`(?i)(^|[^\w/])([A-Za-z0-9_-]*` + secretKeyAlt + `["']?\s*[=:]\s*)(["']?)[^\s&"',;]+`), "${1}${2}${3}[REDACTED]"},
	// kubectl --from-literal=KEY=VALUE — keep KEY, redact VALUE.
	{regexp.MustCompile(`(--from-literal=[^=\s]+=)[^\s"']+`), "${1}[REDACTED]"},
}

// blockScalarHeaderRe matches a YAML block-scalar header ("password: |-",
// "token: >") whose key looks like a secret. Group 1 is the header's leading
// indentation, used by Redact to find where the block scalar body ends.
var blockScalarHeaderRe = regexp.MustCompile(`(?i)^(\s*)[A-Za-z0-9_-]*` + secretKeyAlt + `["']?\s*:\s*[|>][+-]?\s*$`)

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
	for _, p := range redactPatterns {
		line = p.re.ReplaceAllString(line, p.repl)
	}
	return line
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
