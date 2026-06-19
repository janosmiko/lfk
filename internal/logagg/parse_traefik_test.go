package logagg

import "testing"

func TestTraefikParser_Parse(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		wantOK bool
		want   map[string]string
	}{
		{
			name:   "basic traefik access log",
			line:   `{"RequestMethod":"GET","RequestPath":"/api","RequestHost":"example.com","DownstreamStatus":200}`,
			wantOK: true,
			want:   map[string]string{"method": "GET", "path": "/api", "host": "example.com", "status": "200"},
		},
		{
			name:   "traefik log with RequestScheme https",
			line:   `{"RequestMethod":"GET","RequestPath":"/api","RequestScheme":"https","DownstreamStatus":200}`,
			wantOK: true,
			want:   map[string]string{"method": "GET", "path": "/api", "scheme": "https", "status": "200"},
		},
		{
			name:   "traefik log with RequestScheme http",
			line:   `{"RequestMethod":"POST","RequestPath":"/submit","RequestScheme":"http","DownstreamStatus":204}`,
			wantOK: true,
			want:   map[string]string{"method": "POST", "path": "/submit", "scheme": "http", "status": "204"},
		},
		{
			name:   "traefik log without RequestScheme",
			line:   `{"RequestMethod":"GET","RequestPath":"/health","DownstreamStatus":200}`,
			wantOK: true,
			want:   map[string]string{"method": "GET", "path": "/health", "status": "200"},
		},
		{
			name:   "not json",
			line:   `not a json line`,
			wantOK: false,
		},
		{
			name:   "json without traefik keys",
			line:   `{"foo":"bar"}`,
			wantOK: false,
		},
	}
	p := NewTraefikJSONParser()
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
		})
	}
}
