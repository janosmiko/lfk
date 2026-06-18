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
