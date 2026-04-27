package logger

import "regexp"

// redactPatterns is a curated set of regexes for likely-sensitive content
// that may end up in stderr from auth helpers, exec credential plugins,
// kubectl invocations, or upstream library messages. The list is intentionally
// conservative — false positives in stderr logs are worse than missing a
// novel pattern, since users investigating issues need readable context.
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
	{regexp.MustCompile(`((?:https?|git|ssh)://)[^:@\s/]+:[^@\s/]+@`), "${1}[REDACTED-CREDS]@"},
	// Bearer tokens in Authorization headers.
	{regexp.MustCompile(`(?i)(Bearer\s+)[A-Za-z0-9._-]{20,}`), "${1}[REDACTED-BEARER]"},
	// Generic key=value pairs for password/token/secret/api_key/etc.
	// Keep the key visible (useful for diagnosis), redact only the value.
	{regexp.MustCompile(`(?i)((?:password|passwd|pwd|secret|token|api[_-]?key|access[_-]?key|private[_-]?key|client[_-]?secret)\s*[=:]\s*)[^\s&"',;]+`), "${1}[REDACTED]"},
	// kubectl --from-literal=KEY=VALUE — keep KEY, redact VALUE.
	{regexp.MustCompile(`(--from-literal=[^=\s]+=)[^\s"']+`), "${1}[REDACTED]"},
}

// Redact scrubs likely-sensitive content (JWTs, cloud-provider keys, embedded
// URL credentials, common key=value secrets) from s before it lands in the
// log file. Idempotent: running it twice on the same input produces the same
// output. Designed for stderr capture and other places where untrusted text
// flows into the structured logger.
func Redact(s string) string {
	for _, p := range redactPatterns {
		s = p.re.ReplaceAllString(s, p.repl)
	}
	return s
}
