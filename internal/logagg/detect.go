package logagg

// AllKinds returns the built-in profile kinds available in this phase.
func AllKinds() []ProfileKind {
	return []ProfileKind{ProfileTraefikJSON, ProfileNginx, ProfileJSON, ProfileLogfmt}
}

// ParserFor returns the parser for a kind, defaulting to the JSON parser.
func ParserFor(kind ProfileKind) Parser {
	switch kind {
	case ProfileTraefikJSON:
		return NewTraefikJSONParser()
	case ProfileNginx:
		return NewNginxParser()
	case ProfileLogfmt:
		return NewLogfmtParser()
	case ProfileJSON:
		return NewJSONParser()
	default:
		return NewJSONParser()
	}
}

// DetectKind picks the profile that successfully parses the most sample lines.
// More specific profiles are tried first: Traefik/nginx win ties over the
// generic JSON/logfmt matchers. nginx is tried before logfmt so CLF lines whose
// query strings contain "key=value" are not misclaimed by the logfmt matcher.
func DetectKind(sample []string) ProfileKind {
	order := []ProfileKind{ProfileTraefikJSON, ProfileNginx, ProfileLogfmt, ProfileJSON}
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
