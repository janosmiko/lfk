package logagg

import "testing"

func TestEnvoyParser_Parse(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		wantOK bool
		want   map[string]string
		absent []string
	}{
		{
			name:   "full envoy default format sample",
			line:   `[2023-10-10T13:55:36.000Z] "GET /api/users?q=1 HTTP/1.1" 200 - 0 1234 45 44 "10.0.0.1" "curl/7.68" "abc-123" "myapp.example.com" "10.0.0.5:8080"`,
			wantOK: true,
			want: map[string]string{
				FieldMethod:     "GET",
				FieldPath:       "/api/users",
				FieldStatus:     "200",
				FieldDurationMS: "45",
				FieldHost:       "myapp.example.com",
				FieldService:    "10.0.0.5:8080",
			},
		},
		{
			name:   "authority is dash gives no host",
			line:   `[2023-10-10T13:55:36.000Z] "GET /ping HTTP/1.1" 200 - 0 5 2 1 "10.0.0.1" "curl/7.68" "req-456" "-" "10.0.0.5:8080"`,
			wantOK: true,
			want: map[string]string{
				FieldMethod:     "GET",
				FieldPath:       "/ping",
				FieldStatus:     "200",
				FieldDurationMS: "2",
			},
			absent: []string{FieldHost},
		},
		{
			name:   "non-envoy CLF line is rejected",
			line:   `192.0.2.1 - - [10/Oct/2000:13:55:36 -0700] "GET /index.html HTTP/1.0" 200 2326 "-" "Mozilla/5.0"`,
			wantOK: false,
		},
		{
			name:   "JSON line is rejected",
			line:   `{"method":"GET","status":200}`,
			wantOK: false,
		},
	}
	p := NewEnvoyParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := p.Parse(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("field %q = %q, want %q", k, got[k], v)
				}
			}
			for _, k := range tt.absent {
				if _, has := got[k]; has {
					t.Errorf("field %q should be absent, got %q", k, got[k])
				}
			}
		})
	}
}

func TestEnvoyParser_Kind(t *testing.T) {
	if NewEnvoyParser().Kind() != ProfileEnvoy {
		t.Error("Kind() != ProfileEnvoy")
	}
}

// TestEnvoyParser_TruncatedLine verifies that a truncated Envoy log line (fewer
// than 5 quoted tokens) does not set host/service even when it parses successfully.
func TestEnvoyParser_TruncatedLine(t *testing.T) {
	p := NewEnvoyParser()

	// Truncated line: only request-line quoted token (1 quoted token) — method/path/status still extracted.
	truncLine := `[2023-10-10T13:55:36.000Z] "GET /api/health HTTP/1.1" 200 - 0 5 3 2`
	got, ok := p.Parse(truncLine)
	if !ok {
		t.Fatal("expected parse ok for truncated envoy line")
	}
	if got[FieldMethod] != "GET" {
		t.Errorf("method = %q, want GET", got[FieldMethod])
	}
	if _, has := got[FieldHost]; has {
		t.Errorf("truncated line must not set host field, got %q", got[FieldHost])
	}
	if _, has := got[FieldService]; has {
		t.Errorf("truncated line must not set service field, got %q", got[FieldService])
	}

	// Full line still sets host/service.
	fullLine := `[2023-10-10T13:55:36.000Z] "GET /api/users HTTP/1.1" 200 - 0 1234 45 44 "10.0.0.1" "curl/7.68" "abc-123" "myapp.example.com" "10.0.0.5:8080"`
	got2, ok2 := p.Parse(fullLine)
	if !ok2 {
		t.Fatal("expected parse ok for full envoy line")
	}
	if got2[FieldHost] != "myapp.example.com" {
		t.Errorf("full line host = %q, want %q", got2[FieldHost], "myapp.example.com")
	}
	if got2[FieldService] != "10.0.0.5:8080" {
		t.Errorf("full line service = %q, want %q", got2[FieldService], "10.0.0.5:8080")
	}
}
