package logagg

// Fields is a normalized set of attributes extracted from one log line.
type Fields map[string]string

// Normalized field keys shared across all profiles.
const (
	FieldMethod     = "method"
	FieldPath       = "path"
	FieldHost       = "host"
	FieldStatus     = "status"
	FieldDurationMS = "duration_ms"
	FieldLevel      = "level"
	FieldRouter     = "router"
	FieldService    = "service"
	FieldScheme     = "scheme"
)

// ProfileKind identifies a built-in log format parser.
type ProfileKind string

const (
	ProfileTraefikJSON  ProfileKind = "traefik-json"
	ProfileNginx        ProfileKind = "nginx-combined"
	ProfileIngressNginx ProfileKind = "ingress-nginx"
	ProfileEnvoy        ProfileKind = "envoy"
	ProfileJSON         ProfileKind = "json"
	ProfileLogfmt       ProfileKind = "logfmt"
)

// Parser turns a single log line into normalized Fields.
type Parser interface {
	Kind() ProfileKind
	Parse(line string) (Fields, bool)
}
