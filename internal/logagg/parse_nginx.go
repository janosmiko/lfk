package logagg

import (
	"regexp"
	"strings"
)

// nginxRe matches the NCSA Common/Combined Log Format request line and status:
//
//	<host> <ident> <user> [<time>] "<METHOD> <PATH> <PROTO>" <status> <bytes> ...
//
// It also covers Traefik's "common" access-log format, which is CLF with extra
// trailing fields (request count, router name, service URL, duration).
var nginxRe = regexp.MustCompile(`^\S+ \S+ \S+ \[[^\]]*\] "([A-Z]+) (\S+)[^"]*" (\d{3})\b`)

// nginxDurRe captures a trailing "<n>ms" duration that Traefik's CommonLog
// format appends after the standard CLF fields.
var nginxDurRe = regexp.MustCompile(`(\d+)ms\s*$`)

// nginxRouterRe captures a Traefik router name from the Traefik CLF extension.
// It matches a quoted "@"-containing token (e.g. "websecure-gitlab@kubernetes")
// that is followed by another quoted token and an optional "Nms" duration at the
// end of the line. This tighter anchor prevents false positives from "@"-signs in
// user-agent strings, which appear at a different position and are not followed
// by the expected downstream-url and duration fields.
var nginxRouterRe = regexp.MustCompile(`\d+ "([^"]*@[^"]*)" "[^"]*"(?:\s+\d+ms)?\s*$`)

type nginxParser struct{}

// NewNginxParser parses NCSA Common/Combined Log Format access logs (nginx,
// Apache, and Traefik's "common" access-log format).
func NewNginxParser() Parser { return nginxParser{} }

func (nginxParser) Kind() ProfileKind { return ProfileNginx }

func (nginxParser) Parse(line string) (Fields, bool) {
	m := nginxRe.FindStringSubmatch(line)
	if m == nil {
		return nil, false
	}
	path := m[2]
	// Drop the query string so "top paths" groups by endpoint, not by every
	// distinct query (full path normalization is a later phase).
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	f := Fields{
		FieldMethod: m[1],
		FieldPath:   path,
		FieldStatus: m[3],
	}
	if d := nginxDurRe.FindStringSubmatch(line); d != nil {
		f[FieldDurationMS] = d[1]
	}
	if r := nginxRouterRe.FindStringSubmatch(line); r != nil {
		f[FieldRouter] = r[1]
	}
	return f, true
}
