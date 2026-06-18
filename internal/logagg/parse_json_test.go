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
			name:   "empty string scalar preserved",
			line:   `{"message":"","status":200}`,
			wantOK: true,
			want:   map[string]string{"message": "", "status": "200"},
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
	tests := []struct {
		name   string
		line   string
		wantOK bool
		checks func(t *testing.T, got Fields)
	}{
		{
			name:   "basic logfmt",
			line:   `level=info method=POST path=/api/login status=401 duration=88ms`,
			wantOK: true,
			checks: func(t *testing.T, got Fields) {
				if got[FieldMethod] != "POST" {
					t.Errorf("method = %q, want POST", got[FieldMethod])
				}
				if got[FieldStatus] != "401" {
					t.Errorf("status = %q, want 401", got[FieldStatus])
				}
				if got[FieldPath] != "/api/login" {
					t.Errorf("path = %q, want /api/login", got[FieldPath])
				}
				if got[FieldDurationMS] != "88" {
					t.Errorf("duration_ms = %q, want 88", got[FieldDurationMS])
				}
			},
		},
		{
			name:   "quoted value with space",
			line:   `msg="hello world" status=200`,
			wantOK: true,
			checks: func(t *testing.T, got Fields) {
				if got["msg"] != "hello world" {
					t.Errorf("msg = %q, want 'hello world'", got["msg"])
				}
				if got[FieldStatus] != "200" {
					t.Errorf("status = %q, want 200", got[FieldStatus])
				}
			},
		},
		{
			name:   "unquoted value",
			line:   `method=GET status=404`,
			wantOK: true,
			checks: func(t *testing.T, got Fields) {
				if got[FieldMethod] != "GET" {
					t.Errorf("method = %q, want GET", got[FieldMethod])
				}
				if got[FieldStatus] != "404" {
					t.Errorf("status = %q, want 404", got[FieldStatus])
				}
			},
		},
	}
	p := NewLogfmtParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := p.Parse(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			tt.checks(t, got)
		})
	}
}
