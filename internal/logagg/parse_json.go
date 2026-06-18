package logagg

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// durationToMS converts a duration token like "88ms", "1.2s", "12.5" (assumed
// ms) to a millisecond string. Returns "" if it cannot be interpreted.
var durRe = regexp.MustCompile(`^([0-9]*\.?[0-9]+)(ms|s|us|µs|ns)?$`)

func durationToMS(v string) string {
	m := durRe.FindStringSubmatch(strings.TrimSpace(v))
	if m == nil {
		return ""
	}
	f, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return ""
	}
	switch m[2] {
	case "s":
		f *= 1000
	case "us", "µs":
		f /= 1000
	case "ns":
		f /= 1_000_000
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// jsonKeyAliases maps common source keys to normalized field names.
var jsonKeyAliases = map[string]string{
	"method": FieldMethod, "RequestMethod": FieldMethod, "http_method": FieldMethod,
	"path": FieldPath, "RequestPath": FieldPath, "route": FieldPath, "url": FieldPath, "uri": FieldPath,
	"host": FieldHost, "RequestHost": FieldHost, "vhost": FieldHost,
	"status": FieldStatus, "DownstreamStatus": FieldStatus, "status_code": FieldStatus, "response_status": FieldStatus,
	"duration": FieldDurationMS, "Duration": FieldDurationMS, "response_time": FieldDurationMS, "latency": FieldDurationMS,
	"level": FieldLevel, "lvl": FieldLevel, "severity": FieldLevel,
}

type jsonParser struct{}

// NewJSONParser parses single-line JSON objects, mapping common keys to
// normalized fields and keeping all other scalar keys verbatim.
func NewJSONParser() Parser { return jsonParser{} }

func (jsonParser) Kind() ProfileKind { return ProfileJSON }

func (jsonParser) Parse(line string) (Fields, bool) {
	line = strings.TrimSpace(line)
	if len(line) == 0 || line[0] != '{' {
		return nil, false
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return nil, false
	}
	f := make(Fields, len(raw))
	for k, v := range raw {
		s, ok := scalarString(v)
		if !ok {
			continue
		}
		if norm, ok := jsonKeyAliases[k]; ok {
			if norm == FieldDurationMS {
				if ms := durationToMS(s); ms != "" {
					f[norm] = ms
				}
				continue
			}
			f[norm] = s
		} else {
			f[k] = s
		}
	}
	return f, len(f) > 0
}

func scalarString(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), true
	case bool:
		return strconv.FormatBool(t), true
	default:
		return "", false
	}
}
