package logagg

import (
	"regexp"
	"strings"
)

// envoyRe matches Envoy's default access log text format:
// [START_TIME] "METHOD PATH PROTO" STATUS FLAGS BYTES_RECV BYTES_SENT DURATION ...
// Group 1=method, 2=path, 3=status, 4=duration (ms).
var envoyRe = regexp.MustCompile(`^\[[^\]]*\] "([A-Z]+) (\S+)[^"]*" (\d{3}) \S+ \d+ \d+ (\d+)\b`)

// envoyQuotedRe captures all double-quoted tokens from a line.
var envoyQuotedRe = regexp.MustCompile(`"([^"]*)"`)

type envoyParser struct{}

// NewEnvoyParser parses Envoy's default access log text format as used by
// Istio, Contour, Emissary, Gloo, and Gateway API.
func NewEnvoyParser() Parser { return envoyParser{} }

func (envoyParser) Kind() ProfileKind { return ProfileEnvoy }

func (envoyParser) Parse(line string) (Fields, bool) {
	m := envoyRe.FindStringSubmatch(line)
	if m == nil {
		return nil, false
	}
	path := m[2]
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	f := Fields{
		FieldMethod:     m[1],
		FieldPath:       path,
		FieldStatus:     m[3],
		FieldDurationMS: m[4],
	}
	// Extract authority (host) and upstream (service) from quoted tokens.
	// Default format quoted tokens: [request-line, XFF, UA, req-id, authority, upstream-host].
	q := envoyQuotedRe.FindAllStringSubmatch(line, -1)
	if len(q) >= 5 {
		authority := q[len(q)-2][1]
		upstream := q[len(q)-1][1]
		if authority != "" && authority != "-" {
			f[FieldHost] = authority
		}
		if upstream != "" && upstream != "-" {
			f[FieldService] = upstream
		}
	}
	return f, true
}
