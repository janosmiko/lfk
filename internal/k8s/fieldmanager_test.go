package k8s

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"

	"github.com/janosmiko/lfk/internal/version"
)

func TestBuildFieldManager(t *testing.T) {
	tests := []struct {
		name     string
		override string
		user     string
		want     string
	}{
		{name: "user becomes the suffix", user: "jmiko", want: "lfk:jmiko"},
		{name: "empty user falls back", user: "", want: "lfk:unknown"},
		{name: "override replaces everything", override: "lfk", user: "jmiko", want: "lfk"},
		{name: "override is trimmed", override: "  ci-runner  ", user: "jmiko", want: "ci-runner"},
		{name: "windows domain prefix drops", user: `CORP\jmiko`, want: "lfk:jmiko"},
		{name: "at sign survives", user: "jmiko@corp.example", want: "lfk:jmiko@corp.example"},
		{name: "spaces become dashes", user: "Jane Doe", want: "lfk:Jane-Doe"},
		{name: "control characters become dashes", user: "j\nmiko", want: "lfk:j-miko"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, buildFieldManager(tc.override, tc.user))
		})
	}
}

func TestBuildFieldManager_StaysWithinTheAPILimit(t *testing.T) {
	got := buildFieldManager("", strings.Repeat("x", 500))

	assert.LessOrEqual(t, len(got), metav1validation.FieldManagerMaxLength)
	assert.True(t, strings.HasPrefix(got, "lfk:x"), "prefix survives the cut: %q", got)
}

func TestBuildFieldManager_LongOverrideIsCut(t *testing.T) {
	got := buildFieldManager(strings.Repeat("y", 500), "jmiko")

	assert.LessOrEqual(t, len(got), metav1validation.FieldManagerMaxLength)
}

func TestBuildFieldManager_CarriesNoVersion(t *testing.T) {
	// managedFields entries are keyed by manager name. A version inside the
	// name would add a dead entry on every release.
	got := buildFieldManager("", "jmiko")

	assert.NotContains(t, got, "dev")
	assert.Equal(t, "lfk:jmiko", got)
}

func TestBuildUserAgent(t *testing.T) {
	tests := []struct {
		name    string
		version string
		user    string
		host    string
		want    string
	}{
		{
			name:    "full identity",
			version: "v1.2.3",
			user:    "jmiko",
			host:    "laptop",
			want:    "lfk/v1.2.3 (darwin/arm64) jmiko@laptop",
		},
		{
			name:    "missing host drops the suffix",
			version: "dev",
			user:    "jmiko",
			want:    "lfk/dev (darwin/arm64) jmiko",
		},
		{
			name:    "missing user drops the identity",
			version: "dev",
			want:    "lfk/dev (darwin/arm64)",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildUserAgent(tc.version, "darwin", "arm64", tc.user, tc.host)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestBuildUserAgent_StripsControlCharacters(t *testing.T) {
	// A newline in the agent string would split the HTTP header.
	got := buildUserAgent("v1\r\n0", "darwin", "arm64", "j\nmiko", "host\r")

	assert.NotContains(t, got, "\n")
	assert.NotContains(t, got, "\r")
	assert.Equal(t, "lfk/v10 (darwin/arm64) jmiko@host", got)
}

func TestFieldManager_UsesTheConfiguredOverride(t *testing.T) {
	orig := FieldManagerOverride
	t.Cleanup(func() { FieldManagerOverride = orig })

	FieldManagerOverride = "lfk"

	assert.Equal(t, "lfk", FieldManager())
}

func TestFieldManager_DefaultsToTheOSUser(t *testing.T) {
	orig := FieldManagerOverride
	t.Cleanup(func() { FieldManagerOverride = orig })

	FieldManagerOverride = ""
	got := FieldManager()

	assert.True(t, strings.HasPrefix(got, "lfk:"), "got %q", got)
	assert.LessOrEqual(t, len(got), metav1validation.FieldManagerMaxLength)
}

func TestUserAgent_NamesTheToolAndVersion(t *testing.T) {
	got := UserAgent()

	// Packagers inject the version with -ldflags, so "dev" only holds for a
	// plain `go build`. Assert against the version compiled in.
	assert.True(t, strings.HasPrefix(got, "lfk/"+version.Short()+" "), "got %q", got)
}
