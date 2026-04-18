package app

import (
	"fmt"
	"regexp"
	"strings"
)

// Severity classifies a log line. Values are ordered for >=-floor comparisons.
type Severity int

const (
	SeverityUnknown Severity = iota
	SeverityDebug
	SeverityInfo
	SeverityWarn
	SeverityError
)

func (s Severity) String() string {
	switch s {
	case SeverityDebug:
		return "DEBUG"
	case SeverityInfo:
		return "INFO"
	case SeverityWarn:
		return "WARN"
	case SeverityError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// defaultSeverityPatterns lists ordered (Severity, regex source) pairs.
// First match wins. Patterns are case-insensitive.
var defaultSeverityPatterns = []struct {
	sev      Severity
	patterns []string
}{
	{SeverityError, []string{
		// ERROR / ERR / FATAL require structured-log shape (KEY: or KEY=)
		// to avoid matching the prose words "error" / "err" / "fatal"
		// inside otherwise-Info lines like "INFO: handled error gracefully"
		// or prose like "a fatal mistake happened".
		`\bERROR\s*[:=]`, `\bERR\s*[:=]`, `\bFATAL\s*[:=]`, `\[ERROR\]`,
		`level=error\b`, `severity=ERROR\b`,
		`"level"\s*:\s*"error"`, `"severity"\s*:\s*"error"`,
		// klog/glog format used by Kubernetes components (kube-controller-manager,
		// cluster-autoscaler, coredns, kube-proxy, etc.):
		//   E0412 12:34:56.789012       1 main.go:123] error message
		//   F0412 12:34:56.789012       1 main.go:123] fatal message
		// The letter sits at start of line or after whitespace (kubectl may
		// prepend an RFC3339 timestamp when --timestamps is active).
		`\b[EF]\d{4}\s`,
	}},
	{SeverityWarn, []string{
		`\bWARN(ING)?\b`, `\[WARN\]`,
		`level=warn(ing)?\b`, `"level"\s*:\s*"warn(ing)?"`,
		`\bW\d{4}\s`, // klog
	}},
	{SeverityInfo, []string{
		`\bINFO\b`, `\[INFO\]`,
		`level=info\b`, `"level"\s*:\s*"info"`,
		`\bI\d{4}\s`, // klog
	}},
	{SeverityDebug, []string{
		// TRACE must be followed by structured-log shape (KEY=VAL)
		// to avoid false positives like "stack trace: foo".
		`\bDEBUG\b`, `\bTRACE\s+\w+\s*=`,
		`level=debug\b`, `"level"\s*:\s*"debug"`,
	}},
}

// severityDetector compiles patterns once and detects the severity of a line.
type severityDetector struct {
	patterns []severityPattern
}

type severityPattern struct {
	sev Severity
	re  *regexp.Regexp
}

// newSeverityDetector returns a detector. extras lets users append additional
// patterns per severity (extends defaults, does not replace). The returned
// detector is always non-nil — defaults always compile via MustCompile. The
// returned []error contains one entry per user-supplied pattern that failed
// to compile, with the original pattern source attached so the caller can
// surface it. Returns nil for the slice when no user patterns failed.
func newSeverityDetector(extras map[Severity][]string) (*severityDetector, []error) {
	d := &severityDetector{}
	var errs []error
	for _, group := range defaultSeverityPatterns {
		for _, src := range group.patterns {
			d.patterns = append(d.patterns, severityPattern{
				sev: group.sev,
				re:  regexp.MustCompile(`(?i)` + src),
			})
		}
		if extra, ok := extras[group.sev]; ok {
			for _, src := range extra {
				re, err := regexp.Compile(`(?i)` + src)
				if err != nil {
					// Compile errors are returned to the caller below, not silently dropped.
					errs = append(errs, fmt.Errorf("invalid pattern %q for severity %s: %w", src, group.sev, err))
					continue
				}
				d.patterns = append(d.patterns, severityPattern{
					sev: group.sev,
					re:  re,
				})
			}
		}
	}
	return d, errs
}

// Detect returns the severity of the first matching pattern, or
// SeverityUnknown if no pattern matches. Strips a leading "[pod/container] "
// prefix so prefixed lines still classify correctly. The prefix must contain
// a "/" to avoid mis-stripping bracketed level markers like "[WARN]".
func (d *severityDetector) Detect(line string) Severity {
	if len(line) > 0 && line[0] == '[' {
		if idx := strings.Index(line, "] "); idx > 0 && strings.Contains(line[:idx], "/") {
			line = line[idx+2:]
		}
	}
	for _, p := range d.patterns {
		if p.re.MatchString(line) {
			return p.sev
		}
	}
	return SeverityUnknown
}
