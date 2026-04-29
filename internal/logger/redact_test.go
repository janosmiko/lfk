package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedact(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantContains    []string // substrings expected in the redacted output
		wantNotContains []string // substrings that MUST be absent (the secret)
	}{
		{
			name:            "JWT three-segment token",
			input:           `auth failed: invalid token eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c at provider`,
			wantContains:    []string{"[REDACTED-JWT]", "auth failed", "at provider"},
			wantNotContains: []string{"eyJhbGci", "SflKxwRJ"},
		},
		{
			name:            "AWS access key id",
			input:           "AccessKeyId=AKIAIOSFODNN7EXAMPLE expired",
			wantContains:    []string{"[REDACTED-AWS-KEY]", "expired"},
			wantNotContains: []string{"AKIAIOSFODNN7EXAMPLE"},
		},
		{
			name:            "AWS temporary key",
			input:           "ASIATESTAA3LDPGRBVR4 not authorized",
			wantContains:    []string{"[REDACTED-AWS-KEY]", "not authorized"},
			wantNotContains: []string{"ASIATESTAA3LDPGRBVR4"},
		},
		{
			name:            "GCP OAuth token",
			input:           "Authorization: Bearer ya29.a0AfH6SMBxxxxxxxxxxxxxxxxxxxx_yyyyyyyyyy",
			wantContains:    []string{"[REDACTED"},
			wantNotContains: []string{"ya29.a0AfH6"},
		},
		{
			name:            "GitHub token",
			input:           "remote: Invalid token ghp_AbCdEfGhIjKlMnOpQrStUvWxYz0123456789",
			wantContains:    []string{"[REDACTED-GH-TOKEN]", "Invalid token"},
			wantNotContains: []string{"ghp_AbCdEf", "0123456789"},
		},
		{
			name:            "URL with embedded credentials",
			input:           "could not clone https://alice:hunter2@github.com/foo/bar.git: 403",
			wantContains:    []string{"https://[REDACTED-CREDS]@github.com/foo/bar.git", "403"},
			wantNotContains: []string{"alice:hunter2", "hunter2@"},
		},
		{
			name:            "bearer token in HTTP error",
			input:           "401: Authorization: Bearer abcDEF1234567890_-.tokenABCDEF1234567890",
			wantContains:    []string{"Bearer [REDACTED-BEARER]", "401"},
			wantNotContains: []string{"abcDEF1234567890"},
		},
		{
			name:            "password kv inline",
			input:           "connect: pq: password=s3cret-pass-123 host=db.example.com",
			wantContains:    []string{"password=[REDACTED]", "host=db.example.com"},
			wantNotContains: []string{"s3cret-pass-123"},
		},
		{
			name:            "token kv inline",
			input:           "auth failed: token=tok_live_xyz_abcdefghij and retry pending",
			wantContains:    []string{"token=[REDACTED]", "retry pending"},
			wantNotContains: []string{"tok_live_xyz_abcdefghij"},
		},
		{
			name:            "api_key kv inline",
			input:           "request body: api_key=sk-proj-abcdef0123 and other params",
			wantContains:    []string{"api_key=[REDACTED]"},
			wantNotContains: []string{"sk-proj-abcdef0123"},
		},
		{
			name:            "kubectl --from-literal",
			input:           "$ kubectl create secret generic foo --from-literal=API_KEY=topsecretvalue --from-literal=DB_PASSWORD=alsosecret",
			wantContains:    []string{"--from-literal=API_KEY=[REDACTED]", "--from-literal=DB_PASSWORD=[REDACTED]"},
			wantNotContains: []string{"topsecretvalue", "alsosecret"},
		},
		{
			name:            "no secrets passes through unchanged",
			input:           "kubectl get pods -n default --context prod",
			wantContains:    []string{"kubectl get pods -n default --context prod"},
			wantNotContains: []string{"[REDACTED"},
		},
		{
			name:            "empty input",
			input:           "",
			wantContains:    []string{},
			wantNotContains: []string{"[REDACTED"},
		},
		{
			name:            "preserves the rest of the line",
			input:           "ERROR: connect to db host=db.example.com user=app password=oh-no port=5432",
			wantContains:    []string{"host=db.example.com", "port=5432", "password=[REDACTED]"},
			wantNotContains: []string{"oh-no"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Redact(tt.input)
			for _, want := range tt.wantContains {
				assert.Contains(t, got, want, "redacted output should contain %q", want)
			}
			for _, secret := range tt.wantNotContains {
				assert.NotContains(t, got, secret, "redacted output must NOT contain %q (leaked secret)", secret)
			}
		})
	}
}

func TestRedactIsIdempotent(t *testing.T) {
	// Running Redact twice should be a no-op on already-redacted output.
	input := "password=hunter2 token=xyz123abcdef"
	once := Redact(input)
	twice := Redact(once)
	assert.Equal(t, once, twice)
}
