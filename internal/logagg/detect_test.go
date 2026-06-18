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
	if ParserFor(ProfileIngressNginx).Kind() != ProfileIngressNginx {
		t.Error("ParserFor(ingress-nginx) returned wrong kind")
	}
	if ParserFor(ProfileEnvoy).Kind() != ProfileEnvoy {
		t.Error("ParserFor(envoy) returned wrong kind")
	}
}

func TestDetectKind_IngressNginx(t *testing.T) {
	sample := []string{
		`192.168.1.1 - - [10/Oct/2023:13:55:36 +0000] "GET /api/users?q=1 HTTP/1.1" 200 1234 "-" "curl/7.68" 123 0.045 [default-myapp-80] [] 10.0.0.5:8080 1234 0.044 200 abc123`,
		`192.168.1.2 - - [10/Oct/2023:13:55:37 +0000] "POST /api/items HTTP/1.1" 201 56 "-" "curl/7.68" 88 0.012 [default-api-80] [] 10.0.0.6:8080 56 0.011 201 xyz789`,
	}
	got := DetectKind(sample)
	if got != ProfileIngressNginx {
		t.Errorf("DetectKind(ingress-nginx sample) = %q, want %q", got, ProfileIngressNginx)
	}
}

func TestDetectKind_Envoy(t *testing.T) {
	sample := []string{
		`[2023-10-10T13:55:36.000Z] "GET /api/users?q=1 HTTP/1.1" 200 - 0 1234 45 44 "10.0.0.1" "curl/7.68" "abc-123" "myapp.example.com" "10.0.0.5:8080"`,
		`[2023-10-10T13:55:37.000Z] "POST /api/items HTTP/1.1" 201 - 10 56 12 11 "10.0.0.1" "curl/7.68" "def-456" "myapp.example.com" "10.0.0.6:8080"`,
	}
	got := DetectKind(sample)
	if got != ProfileEnvoy {
		t.Errorf("DetectKind(envoy sample) = %q, want %q", got, ProfileEnvoy)
	}
}

func TestDetectKind_PlainNginxStillNginx(t *testing.T) {
	sample := []string{
		`192.0.2.1 - - [10/Oct/2000:13:55:36 -0700] "GET /index.html HTTP/1.0" 200 2326 "-" "Mozilla/5.0"`,
		`192.0.2.2 - - [10/Oct/2000:13:55:37 -0700] "POST /api HTTP/1.1" 201 100 "-" "curl/7.68"`,
	}
	got := DetectKind(sample)
	if got != ProfileNginx {
		t.Errorf("DetectKind(plain nginx sample) = %q, want %q", got, ProfileNginx)
	}
}
