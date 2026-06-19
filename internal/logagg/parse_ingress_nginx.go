package logagg

import (
	"regexp"
	"strconv"
	"strings"
)

// ingressTailRe matches the ingress-nginx-specific trailing fields after the
// user-agent closing quote: <request_length> <request_time> [<upstream_name>].
// Group 1 = request_time (seconds, float), group 2 = proxy_upstream_name.
var ingressTailRe = regexp.MustCompile(`"\s+\d+\s+([\d.]+)\s+\[([^\]]*)\]`)

type ingressNginxParser struct{}

// NewIngressNginxParser parses the NGINX Ingress Controller default
// log-format-upstream, which is CLF-core plus ingress-specific trailing fields.
func NewIngressNginxParser() Parser { return ingressNginxParser{} }

func (ingressNginxParser) Kind() ProfileKind { return ProfileIngressNginx }

func (ingressNginxParser) Parse(line string) (Fields, bool) {
	m := nginxRe.FindStringSubmatch(line)
	if m == nil {
		return nil, false
	}
	tail := ingressTailRe.FindStringSubmatch(line)
	if tail == nil {
		return nil, false
	}
	path := m[2]
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	f := Fields{
		FieldMethod: m[1],
		FieldPath:   path,
		FieldStatus: m[3],
	}
	// Convert request_time from seconds to milliseconds.
	if secs, err := strconv.ParseFloat(tail[1], 64); err == nil {
		f[FieldDurationMS] = strconv.FormatFloat(secs*1000, 'f', -1, 64)
	}
	if tail[2] != "" {
		f[FieldService] = tail[2]
	}
	return f, true
}
