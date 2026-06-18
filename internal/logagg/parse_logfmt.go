package logagg

import (
	"regexp"
	"strings"
)

var logfmtPairRe = regexp.MustCompile(`(\w[\w.-]*)=("([^"]*)"|\S+)`)

type logfmtParser struct{}

// NewLogfmtParser parses key=value (logfmt) lines.
func NewLogfmtParser() Parser { return logfmtParser{} }

func (logfmtParser) Kind() ProfileKind { return ProfileLogfmt }

func (logfmtParser) Parse(line string) (Fields, bool) {
	matches := logfmtPairRe.FindAllStringSubmatch(line, -1)
	if len(matches) == 0 {
		return nil, false
	}
	f := make(Fields, len(matches))
	for _, m := range matches {
		key := m[1]
		val := m[3]
		if val == "" {
			val = strings.Trim(m[2], `"`)
		}
		if norm, ok := jsonKeyAliases[key]; ok {
			if norm == FieldDurationMS {
				if ms := durationToMS(val); ms != "" {
					f[norm] = ms
				}
				continue
			}
			f[norm] = val
		} else {
			f[key] = val
		}
	}
	return f, len(f) > 0
}
