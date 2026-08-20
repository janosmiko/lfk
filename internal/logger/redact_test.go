package logger

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			name:            "postgres URL with embedded credentials",
			input:           "dial tcp: could not connect to postgres://dbuser:hunter2-PGMARKER@db.internal:5432/app: connection refused",
			wantContains:    []string{"postgres://[REDACTED-CREDS]@db.internal:5432/app", "connection refused"},
			wantNotContains: []string{"dbuser:hunter2-PGMARKER", "hunter2-PGMARKER@"},
		},
		{
			name:            "mongodb URL with embedded credentials",
			input:           "mongodb://root:s3cret-MONGOMARKER@cluster0.example.net/admin timed out",
			wantContains:    []string{"mongodb://[REDACTED-CREDS]@cluster0.example.net/admin", "timed out"},
			wantNotContains: []string{"root:s3cret-MONGOMARKER"},
		},
		{
			name:            "DB_PASS kv inline",
			input:           "connect: DB_PASS=hunter2-DBPASSMARKER host=db.example.com",
			wantContains:    []string{"DB_PASS=[REDACTED]", "host=db.example.com"},
			wantNotContains: []string{"hunter2-DBPASSMARKER"},
		},
		{
			name:            "DATABASE_URL kv inline",
			input:           "config error: DATABASE_URL=postgres://u:hunter2-DATABASEURLMARKER@db/app is invalid",
			wantContains:    []string{"DATABASE_URL=[REDACTED]", "is invalid"},
			wantNotContains: []string{"hunter2-DATABASEURLMARKER"},
		},
		{
			name:            "connectionString kv inline",
			input:           "startup failed: connectionString=hunter2-CONNSTRMARKER retrying",
			wantContains:    []string{"connectionString=[REDACTED]", "retrying"},
			wantNotContains: []string{"hunter2-CONNSTRMARKER"},
		},
		{
			name:            "dsn kv inline",
			input:           "sqlx: dsn=hunter2-DSNMARKER open failed",
			wantContains:    []string{"dsn=[REDACTED]", "open failed"},
			wantNotContains: []string{"hunter2-DSNMARKER"},
		},
		{
			name:            "MYSQL_PASSWORD env-style key",
			input:           "startup: MYSQL_PASSWORD=hunter2marker refused",
			wantContains:    []string{"MYSQL_PASSWORD=[REDACTED]", "refused"},
			wantNotContains: []string{"hunter2marker"},
		},
		{
			name:            "POSTGRES_PASSWORD env-style key with colon",
			input:           "config: POSTGRES_PASSWORD: hunter2marker invalid",
			wantContains:    []string{"POSTGRES_PASSWORD: [REDACTED]", "invalid"},
			wantNotContains: []string{"hunter2marker"},
		},
		{
			name:            "ADMIN_TOKEN env-style key",
			input:           "auth: ADMIN_TOKEN=hunter2marker expired",
			wantContains:    []string{"ADMIN_TOKEN=[REDACTED]", "expired"},
			wantNotContains: []string{"hunter2marker"},
		},
		{
			name:            "AWS_SECRET_ACCESS_KEY env-style key",
			input:           "creds: AWS_SECRET_ACCESS_KEY=hunter2marker denied",
			wantContains:    []string{"AWS_SECRET_ACCESS_KEY=[REDACTED]", "denied"},
			wantNotContains: []string{"hunter2marker"},
		},
		{
			name:            "MY_API_KEY env-style key",
			input:           "request: MY_API_KEY=hunter2marker rejected",
			wantContains:    []string{"MY_API_KEY=[REDACTED]", "rejected"},
			wantNotContains: []string{"hunter2marker"},
		},
		{
			name:            "user_password lowercase env-style key",
			input:           "login: user_password=hunter2marker failed",
			wantContains:    []string{"user_password=[REDACTED]", "failed"},
			wantNotContains: []string{"hunter2marker"},
		},
		{
			name:            "password as a path component is not mangled",
			input:           "cat /etc/passwd: permission denied",
			wantContains:    []string{"cat /etc/passwd: permission denied"},
			wantNotContains: []string{"[REDACTED"},
		},
		{
			name:            "redis URL with password only (no username)",
			input:           "redis://:hunter2SECRET@host:6379/0 connection refused",
			wantContains:    []string{"redis://[REDACTED-CREDS]@host:6379/0", "connection refused"},
			wantNotContains: []string{"hunter2SECRET"},
		},
		{
			name:            "redis URL with no credentials passes through unchanged",
			input:           "connecting to redis://host:6379/0",
			wantContains:    []string{"connecting to redis://host:6379/0"},
			wantNotContains: []string{"[REDACTED"},
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
		{
			name:            "YAML block scalar with one indented secret line",
			input:           "password: |-\n  s3cret-block-line-1",
			wantContains:    []string{"password:", "[REDACTED]"},
			wantNotContains: []string{"s3cret-block-line-1"},
		},
		{
			name:            "YAML block scalar with three indented secret lines",
			input:           "token: |-\n  line-one-secret\n  line-two-secret\n  line-three-secret",
			wantContains:    []string{"[REDACTED]"},
			wantNotContains: []string{"line-one-secret", "line-two-secret", "line-three-secret"},
		},
		{
			name:            "YAML block scalar under a non-secret key is left untouched",
			input:           "description: |-\n  this is just a normal\n  multi-line description",
			wantContains:    []string{"this is just a normal", "multi-line description"},
			wantNotContains: []string{"[REDACTED"},
		},
		{
			name:            "YAML block scalar ends when indentation returns to the key level",
			input:           "password: |-\n  s3cret-in-block\nnext_key: plain-value",
			wantContains:    []string{"[REDACTED]", "next_key: plain-value"},
			wantNotContains: []string{"s3cret-in-block"},
		},
		{
			name:            "double-quoted JSON-ish key",
			input:           `payload: {"password": "s3cret-quoted-double"} received`,
			wantContains:    []string{`"password": "[REDACTED]"`, "received"},
			wantNotContains: []string{"s3cret-quoted-double"},
		},
		{
			name:            "single-quoted JSON-ish key",
			input:           `payload: {'password': 's3cret-quoted-single'} received`,
			wantContains:    []string{`'password': '[REDACTED]'`, "received"},
			wantNotContains: []string{"s3cret-quoted-single"},
		},
		{
			name:            "base64 blob under a generic data key is not redacted (documented stance)",
			input:           "data: dGVzdC1zM2NyZXQtYmFzZTY0LWJsb2I=",
			wantContains:    []string{"data: dGVzdC1zM2NyZXQtYmFzZTY0LWJsb2I="},
			wantNotContains: []string{"[REDACTED"},
		},
		{
			name:            "ordinary YAML list is not mangled",
			input:           "items:\n  - foo\n  - bar",
			wantContains:    []string{"items:", "- foo", "- bar"},
			wantNotContains: []string{"[REDACTED"},
		},
		{
			name:            "diff-style indented context lines under a bare password key (no block marker) pass through",
			input:           "--- a/values.yaml\n+++ b/values.yaml\n password:\n-  old-secret-context-line\n+  new-secret-context-line",
			wantContains:    []string{"old-secret-context-line", "new-secret-context-line"},
			wantNotContains: []string{"[REDACTED"},
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

func TestRedactErr(t *testing.T) {
	baseErr := errors.New("exit status 1")
	output := []byte("Error: UPGRADE FAILED: could not connect, password: hunter2-MARKER host=db.example.com")

	got := RedactErr(baseErr, output)

	require.Error(t, got)
	assert.NotContains(t, got.Error(), "hunter2-MARKER", "redacted error must NOT leak the secret")
	assert.Contains(t, got.Error(), "[REDACTED]", "redacted error must contain a redaction placeholder")
	assert.Contains(t, got.Error(), "exit status 1", "redacted error must preserve the underlying error reason")
	assert.Contains(t, got.Error(), "could not connect", "redacted error must preserve the non-secret output")
	assert.ErrorIs(t, got, baseErr, "redacted error must still wrap the original error for errors.Is")
}

func BenchmarkRedact(b *testing.B) {
	var sb strings.Builder
	sb.WriteString("password: |-\n")
	for range 500 {
		sb.WriteString("  some-non-secret-configuration-line-that-is-reasonably-long-for-benchmarking\n")
	}
	input := sb.String()

	b.ResetTimer()
	for range b.N {
		_ = Redact(input)
	}
}

func TestRedactIsIdempotent(t *testing.T) {
	// Running Redact twice should be a no-op on already-redacted output.
	input := "password=hunter2 token=xyz123abcdef"
	once := Redact(input)
	twice := Redact(once)
	assert.Equal(t, once, twice)
}
