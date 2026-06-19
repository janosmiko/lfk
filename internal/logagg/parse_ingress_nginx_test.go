package logagg

import "testing"

func TestIngressNginxParser_Parse(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		wantOK bool
		want   map[string]string
		absent []string
	}{
		{
			name:   "full ingress-nginx sample",
			line:   `192.168.1.1 - - [10/Oct/2023:13:55:36 +0000] "GET /api/users?q=1 HTTP/1.1" 200 1234 "-" "curl/7.68" 123 0.045 [default-myapp-80] [] 10.0.0.5:8080 1234 0.044 200 abc123`,
			wantOK: true,
			want: map[string]string{
				FieldMethod:     "GET",
				FieldPath:       "/api/users",
				FieldStatus:     "200",
				FieldDurationMS: "45",
				FieldService:    "default-myapp-80",
			},
		},
		{
			name:   "empty upstream name gives no service",
			line:   `192.168.1.1 - - [10/Oct/2023:13:55:36 +0000] "POST /healthz HTTP/1.1" 200 0 "-" "kube-probe/1.27" 10 0.001 [] [] 10.0.0.5:8080 0 0.001 200 def456`,
			wantOK: true,
			want: map[string]string{
				FieldMethod:     "POST",
				FieldPath:       "/healthz",
				FieldStatus:     "200",
				FieldDurationMS: "1",
			},
			absent: []string{FieldService},
		},
		{
			name:   "plain combined log line is rejected",
			line:   `192.0.2.1 - - [10/Oct/2000:13:55:36 -0700] "GET /index.html HTTP/1.0" 200 2326 "-" "Mozilla/5.0"`,
			wantOK: false,
		},
	}
	p := NewIngressNginxParser()
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

func TestIngressNginxParser_Kind(t *testing.T) {
	if NewIngressNginxParser().Kind() != ProfileIngressNginx {
		t.Error("Kind() != ProfileIngressNginx")
	}
}
