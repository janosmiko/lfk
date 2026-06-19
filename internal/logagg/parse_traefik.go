package logagg

import (
	"encoding/json"
	"strconv"
	"strings"
)

type traefikParser struct{}

// NewTraefikJSONParser parses Traefik access logs in JSON format. Duration is
// reported in nanoseconds and converted to milliseconds.
func NewTraefikJSONParser() Parser { return traefikParser{} }

func (traefikParser) Kind() ProfileKind { return ProfileTraefikJSON }

func (traefikParser) Parse(line string) (Fields, bool) {
	line = strings.TrimSpace(line)
	if len(line) == 0 || line[0] != '{' {
		return nil, false
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return nil, false
	}
	// Require at least one Traefik-specific access-log key to claim the line.
	if _, ok := raw["DownstreamStatus"]; !ok {
		if _, ok := raw["RequestMethod"]; !ok {
			return nil, false
		}
	}
	f := make(Fields, 7)
	if v, ok := scalarString(raw["RequestMethod"]); ok {
		f[FieldMethod] = v
	}
	if v, ok := scalarString(raw["RequestPath"]); ok {
		f[FieldPath] = v
	}
	if v, ok := scalarString(raw["RequestHost"]); ok {
		f[FieldHost] = v
	}
	if v, ok := scalarString(raw["DownstreamStatus"]); ok {
		f[FieldStatus] = v
	}
	if ns, ok := raw["Duration"].(float64); ok {
		f[FieldDurationMS] = strconv.FormatFloat(ns/1_000_000, 'f', -1, 64)
	}
	if v, ok := scalarString(raw["RequestScheme"]); ok {
		f[FieldScheme] = v
	}
	return f, len(f) > 0
}
