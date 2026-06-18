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

// DetectKind picks the profile that successfully parses the most sample lines.
// More specific profiles are tried first: Traefik/ingress-nginx/envoy win over
// the plain nginx/logfmt/json matchers. ingress-nginx is before nginx (requires
// the trailing ingress fields); envoy is before nginx (bracket timestamp format).
func DetectKind(sample []string) ProfileKind {
	order := []ProfileKind{ProfileTraefikJSON, ProfileIngressNginx, ProfileEnvoy, ProfileNginx, ProfileLogfmt, ProfileJSON}
	best := ProfileJSON
	bestHits := 0
	for _, kind := range order {
		p := ParserFor(kind)
		hits := 0
		for _, line := range sample {
			if _, ok := p.Parse(line); ok {
				hits++
			}
		}
		if hits > bestHits {
			bestHits, best = hits, kind
		}
	}
	return best
}
