package ui

import (
	"regexp"
	"strings"
)

// Log filter buckets, ascending. These are the only levels the severity
// filter distinguishes; finer-grained source levels are merged into them
// (trace/debug -> LogDebug, fatal/panic/critical/failure -> LogError). The
// selectable thresholds are LogInfo/LogWarn/LogError (plus off); LogDebug is
// never used as a threshold, only as a line bucket so trace/debug lines drop
// out at INFO+.
const (
	LogDebug = iota // trace, debug, verbose
	LogInfo         // info, notice, log, and the plain-text default
	LogWarn         // warn, warning
	LogError        // error, fatal, panic, critical, failure
)

// bucketByName maps a structured level/severity token (the normalized display
// names emitted by ParseLogLine, e.g. "Warning"/"Error", and common raw
// tokens) to a filter bucket.
var bucketByName = map[string]int{
	"trace": LogDebug, "debug": LogDebug, "dbg": LogDebug, "verbose": LogDebug,
	"fine": LogDebug, "finer": LogDebug, "finest": LogDebug,
	"info": LogInfo, "information": LogInfo, "notice": LogInfo, "log": LogInfo, "status": LogInfo,
	"warn": LogWarn, "warning": LogWarn,
	"error": LogError, "err": LogError, "fatal": LogError, "panic": LogError,
	"dpanic": LogError, "crit": LogError, "critical": LogError, "alert": LogError,
	"emerg": LogError, "emergency": LogError, "severe": LogError,
	"fail": LogError, "failure": LogError,
}

// levelBucket returns the bucket for a structured level token, or (0, false)
// when the token is not a recognized level. A trailing digit (Postgres
// DEBUG1..DEBUG5) is stripped before the second lookup.
func levelBucket(name string) (int, bool) {
	s := strings.ToLower(strings.TrimSpace(name))
	if b, ok := bucketByName[s]; ok {
		return b, true
	}
	s = strings.TrimRight(s, "0123456789")
	b, ok := bucketByName[s]
	return b, ok
}

// Keyword matchers for plain-text logs that carry no structured level. Word
// boundaries keep "err" from matching inside "preferred" and similar.
var (
	kwError = regexp.MustCompile(`(?i)\b(error|errors|err|fail|failed|failing|failure|failures|fatal|panic|exception|critical|severe)\b`)
	kwWarn  = regexp.MustCompile(`(?i)\b(warn|warning|warnings)\b`)
	kwDebug = regexp.MustCompile(`(?i)\b(debug|trace|verbose)\b`)
)

// LineLogLevel returns the filter bucket (LogDebug..LogError) for a log line.
// Structured logs (recognized by ParseLogLine) use the parsed level field,
// which is authoritative. Plain-text logs fall back to a case-insensitive
// keyword scan, defaulting to LogInfo when no severity keyword is present.
func LineLogLevel(line string) int {
	p := ParseLogLine(line)
	for _, f := range p.Fields {
		switch strings.ToLower(f.Key) {
		case "level", "lvl", "severity":
			if b, ok := levelBucket(f.Value); ok {
				return b
			}
		}
	}
	return keywordLevel(line)
}

// keywordLevel classifies a plain-text line by the highest-severity keyword it
// contains, defaulting to LogInfo. Error keywords win over warn over debug.
func keywordLevel(line string) int {
	switch {
	case kwError.MatchString(line):
		return LogError
	case kwWarn.MatchString(line):
		return LogWarn
	case kwDebug.MatchString(line):
		return LogDebug
	default:
		return LogInfo
	}
}

// LogLevelName returns the display name for a selectable threshold:
// INFO/WARN/ERROR for LogInfo/LogWarn/LogError, "" otherwise (off and the
// debug bucket are never rendered as a threshold indicator).
func LogLevelName(level int) string {
	switch level {
	case LogInfo:
		return "INFO"
	case LogWarn:
		return "WARN"
	case LogError:
		return "ERROR"
	default:
		return ""
	}
}
