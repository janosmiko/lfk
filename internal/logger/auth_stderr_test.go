package logger

import (
	"testing"
	"time"
)

// A credential-plugin stderr line must be demoted: kept out of MsgChan (so it
// never reaches the in-app overlay) since the context-tagged log carries the
// actionable signal. A normal stderr line still flows to MsgChan.
func TestStderrCaptureDemotesCredentialLines(t *testing.T) {
	ResetDedupForTest()
	sc := NewStderrCapture()
	defer sc.Close()
	w := sc.Writer()

	if _, err := w.Write([]byte("aws: [ERROR]: The SSO session associated with this profile has expired")); err != nil {
		t.Fatalf("write: %v", err)
	}
	select {
	case msg := <-sc.MsgChan:
		t.Fatalf("credential stderr line should not reach MsgChan, got %q", msg)
	case <-time.After(200 * time.Millisecond):
		// expected: demoted, no overlay message
	}

	if _, err := w.Write([]byte("regular stderr diagnostic")); err != nil {
		t.Fatalf("write: %v", err)
	}
	select {
	case msg := <-sc.MsgChan:
		if msg != "regular stderr diagnostic" {
			t.Fatalf("unexpected msg: %q", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("non-credential stderr line should reach MsgChan")
	}
}

func TestLooksLikeExecCredentialStderr(t *testing.T) {
	tests := []struct {
		name string
		line string
		want bool
	}{
		{
			"aws sso expired",
			"aws: [ERROR]: The SSO session associated with this profile has expired or is otherwise invalid. To refresh this SSO session run aws sso login with the corresponding profile.",
			true,
		},
		{"aws sso login hint", "Error loading SSO Token: run aws sso login", true},
		{"expired token", "ExpiredToken: The security token included in the request is expired", true},
		{"unable to locate credentials", "Unable to locate credentials. You can configure credentials by running...", true},
		{"gke auth plugin", "gke-gcloud-auth-plugin: error getting credentials", true},
		{"unrelated noise", "some unrelated library warning about deprecation", false},
		{"empty", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := looksLikeExecCredentialStderr(tc.line); got != tc.want {
				t.Errorf("looksLikeExecCredentialStderr(%q) = %v, want %v", tc.line, got, tc.want)
			}
		})
	}
}
