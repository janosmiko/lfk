package logagg

import "testing"

func TestDetectKind(t *testing.T) {
	traefik := []string{
		`{"DownstreamStatus":200,"RequestMethod":"GET","RequestPath":"/api/users","Duration":4200000}`,
		`{"DownstreamStatus":404,"RequestMethod":"GET","RequestPath":"/missing","Duration":1200000}`,
	}
	if got := DetectKind(traefik); got != ProfileTraefikJSON {
		t.Errorf("DetectKind(traefik) = %q, want %q", got, ProfileTraefikJSON)
	}

	logfmt := []string{`level=info method=GET path=/x status=200`, `level=warn method=POST path=/y status=500`}
	if got := DetectKind(logfmt); got != ProfileLogfmt {
		t.Errorf("DetectKind(logfmt) = %q, want %q", got, ProfileLogfmt)
	}
}

func TestParserFor(t *testing.T) {
	if ParserFor(ProfileTraefikJSON).Kind() != ProfileTraefikJSON {
		t.Error("ParserFor(traefik) returned wrong kind")
	}
	if ParserFor("nonsense").Kind() != ProfileJSON {
		t.Error("ParserFor(unknown) should fall back to JSON")
	}
}
