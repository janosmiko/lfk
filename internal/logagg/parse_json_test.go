package logagg

import "testing"

func TestJSONParser_Parse(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		wantOK bool
		want   map[string]string
	}{
		{
			name:   "node json access log",
			line:   `{"level":"info","method":"GET","path":"/api/users","status":200,"duration":12.5}`,
			wantOK: true,
			want:   map[string]string{"level": "info", "method": "GET", "path": "/api/users", "status": "200", "duration_ms": "12.5"},
		},
		{
			name:   "not json",
			line:   `192.0.2.1 GET /x 200`,
			wantOK: false,
		},
	}
	p := NewJSONParser()
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

func TestLogfmtParser_Parse(t *testing.T) {
	p := NewLogfmtParser()
	got, ok := p.Parse(`level=info method=POST path=/api/login status=401 duration=88ms`)
	if !ok {
		t.Fatal("expected logfmt parse to succeed")
	}
	if got[FieldMethod] != "POST" || got[FieldStatus] != "401" || got[FieldPath] != "/api/login" {
		t.Errorf("unexpected fields: %#v", got)
	}
	if got[FieldDurationMS] != "88" {
		t.Errorf("duration_ms = %q, want 88", got[FieldDurationMS])
	}
}
