package ui

import "strings"

// Severity ranks, ascending. SevUnknown (0) means no level was detected.
const (
	SevUnknown = iota
	SevTrace
	SevDebug
	SevInfo
	SevWarn
	SevError
	SevFatal
)

// severityByName maps level/severity field values — both the normalized
// display names emitted by ParseLogLine ("Warning", "Error", ...) and common
// raw tokens — to a rank.
var severityByName = map[string]int{
	"trace": SevTrace,
	"debug": SevDebug, "dbg": SevDebug,
	"info": SevInfo, "information": SevInfo, "notice": SevInfo, "log": SevInfo,
	"warn": SevWarn, "warning": SevWarn,
	"error": SevError, "err": SevError,
	"fatal": SevFatal, "panic": SevFatal, "dpanic": SevFatal,
	"crit": SevFatal, "critical": SevFatal, "emerg": SevFatal,
	"emergency": SevFatal, "alert": SevFatal,
}

// SeverityRank maps a single level/severity token to a rank, or SevUnknown.
// A trailing digit (Postgres DEBUG1..DEBUG5) is stripped before lookup.
func SeverityRank(name string) int {
	s := strings.ToLower(strings.TrimSpace(name))
	if r, ok := severityByName[s]; ok {
		return r
	}
	s = strings.TrimRight(s, "0123456789")
	return severityByName[s]
}

// LineSeverity parses a log line and returns its severity rank, or SevUnknown
// when no level/severity field is detected. Reuses the preview-pane parser.
func LineSeverity(line string) int {
	p := ParseLogLine(line)
	for _, f := range p.Fields {
		switch strings.ToLower(f.Key) {
		case "level", "lvl", "severity":
			if r := SeverityRank(f.Value); r != SevUnknown {
				return r
			}
		}
	}
	return SevUnknown
}

// SeverityName returns the short uppercase display name for a rank, or "".
func SeverityName(rank int) string {
	switch rank {
	case SevTrace:
		return "TRACE"
	case SevDebug:
		return "DEBUG"
	case SevInfo:
		return "INFO"
	case SevWarn:
		return "WARN"
	case SevError:
		return "ERROR"
	case SevFatal:
		return "FATAL"
	default:
		return ""
	}
}
