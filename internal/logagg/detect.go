package logagg

// AllKinds returns the built-in profile kinds available in this phase.
func AllKinds() []ProfileKind {
	return []ProfileKind{ProfileTraefikJSON, ProfileIngressNginx, ProfileNginx, ProfileEnvoy, ProfileJSON, ProfileLogfmt}
}

// ParserFor returns the parser for a kind, defaulting to the JSON parser.
func ParserFor(kind ProfileKind) Parser {
	switch kind {
	case ProfileTraefikJSON:
		return NewTraefikJSONParser()
	case ProfileIngressNginx:
		return NewIngressNginxParser()
	case ProfileNginx:
		return NewNginxParser()
	case ProfileEnvoy:
		return NewEnvoyParser()
	case ProfileLogfmt:
		return NewLogfmtParser()
	case ProfileJSON:
		return NewJSONParser()
	default:
		return NewJSONParser()
	}
}

// structuredHTTPKinds are the strict HTTP access-log profiles. They are
// structurally demanding (traefik-json requires DownstreamStatus/RequestMethod,
// and envoy/nginx/ingress require a specific line shape), so a non-HTTP app log
// cannot accidentally match them. When one matches even a few sample lines it is
// preferred over the loose logfmt/json matchers — see DetectKind.
var structuredHTTPKinds = []ProfileKind{ProfileTraefikJSON, ProfileIngressNginx, ProfileEnvoy, ProfileNginx}

// structuredDetectMinHits is how many sample lines a structured HTTP profile
// must match to be preferred over a generic matcher that scores higher. It is a
// small absolute count because the structured parsers are strict (no false
// positives). So a handful of real access-log lines reliably identifies the
// format even amid a flood of console/error lines.
const structuredDetectMinHits = 3

// DetectKind picks the log format for a sample. A structured HTTP access-log
// profile (traefik-json / ingress-nginx / envoy / nginx) is preferred whenever
// it matches at least structuredDetectMinHits lines. This holds EVEN IF the
// generic logfmt/json matchers match more lines. This handles mixed deployments (e.g.
// Traefik emitting JSON access logs alongside ANSI console error lines that the
// logfmt matcher loosely catches): the access logs are the data the Top view
// exists for, so they win over the noise. When no structured profile is present,
// it falls back to whichever profile matches the most lines.
func DetectKind(sample []string) ProfileKind {
	order := []ProfileKind{ProfileTraefikJSON, ProfileIngressNginx, ProfileEnvoy, ProfileNginx, ProfileLogfmt, ProfileJSON}
	hits := make(map[ProfileKind]int, len(order))
	for _, kind := range order {
		p := ParserFor(kind)
		for _, line := range sample {
			if _, ok := p.Parse(line); ok {
				hits[kind]++
			}
		}
	}

	// Prefer the best structured HTTP profile when it has a meaningful count.
	bestStruct := ProfileKind("")
	bestStructHits := 0
	for _, kind := range structuredHTTPKinds {
		if hits[kind] > bestStructHits {
			bestStructHits, bestStruct = hits[kind], kind
		}
	}
	if bestStruct != "" && bestStructHits >= structuredDetectMinHits {
		return bestStruct
	}

	// Otherwise pick whatever matches the most lines (first in order wins ties).
	best := ProfileJSON
	bestHits := 0
	for _, kind := range order {
		if hits[kind] > bestHits {
			bestHits, best = hits[kind], kind
		}
	}
	return best
}
