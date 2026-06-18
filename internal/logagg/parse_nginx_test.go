package logagg

import "testing"

func TestNginxParser_Parse(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		wantOK bool
		want   map[string]string
	}{
		{
			name:   "traefik common log format",
			line:   `10.42.2.19 - - [18/Jun/2026:15:18:46 +0000] "POST /api/v4/jobs/request HTTP/1.1" 204 0 "-" "-" 8086 "websecure-gitlab@kubernetes" "http://10.42.13.162:8181" 2ms`,
			wantOK: true,
			want:   map[string]string{"method": "POST", "path": "/api/v4/jobs/request", "status": "204", "duration_ms": "2"},
		},
		{
			name:   "query string stripped from path",
			line:   `10.255.0.12 - - [18/Jun/2026:15:18:46 +0000] "GET /rest/executions?filter=%7B%22a%22%3A1%7D&limit=10 HTTP/2.0" 304 0 "-" "-" 8087 "websecure-n8n@kubernetes" "http://10.42.2.57:5678" 20ms`,
			wantOK: true,
			want:   map[string]string{"method": "GET", "path": "/rest/executions", "status": "304", "duration_ms": "20"},
		},
		{
			name:   "combined log without trailing duration",
			line:   `192.0.2.1 - - [10/Oct/2000:13:55:36 -0700] "GET /index.html HTTP/1.0" 200 2326 "-" "Mozilla/5.0"`,
			wantOK: true,
			want:   map[string]string{"method": "GET", "path": "/index.html", "status": "200"},
		},
		{
			name:   "not a CLF line",
			line:   `{"method":"GET","status":200}`,
			wantOK: false,
		},
		{
			name:   "logfmt line is not CLF",
			line:   `level=info method=GET path=/x status=200`,
			wantOK: false,
		},
	}
	p := NewNginxParser()
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
			if tt.want["duration_ms"] == "" {
				if _, has := got[FieldDurationMS]; has {
					t.Errorf("expected no duration_ms, got %q", got[FieldDurationMS])
				}
			}
		})
	}
}
