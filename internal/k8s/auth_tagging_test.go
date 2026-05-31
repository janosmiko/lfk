package k8s

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/janosmiko/lfk/internal/logger"
)

func TestAWSProfileFromExec(t *testing.T) {
	tests := []struct {
		name string
		exec *clientcmdapi.ExecConfig
		want string
	}{
		{"nil", nil, ""},
		{"profile flag separate arg", &clientcmdapi.ExecConfig{Args: []string{"eks", "get-token", "--profile", "prod"}}, "prod"},
		{"profile flag equals form", &clientcmdapi.ExecConfig{Args: []string{"eks", "get-token", "--profile=staging"}}, "staging"},
		{"profile from env", &clientcmdapi.ExecConfig{Env: []clientcmdapi.ExecEnvVar{{Name: "AWS_PROFILE", Value: "dev"}}}, "dev"},
		{"flag wins over env", &clientcmdapi.ExecConfig{Args: []string{"--profile", "flagprof"}, Env: []clientcmdapi.ExecEnvVar{{Name: "AWS_PROFILE", Value: "envprof"}}}, "flagprof"},
		{"no profile", &clientcmdapi.ExecConfig{Args: []string{"eks", "get-token"}}, ""},
		{"trailing profile flag without value", &clientcmdapi.ExecConfig{Args: []string{"eks", "--profile"}}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, awsProfileFromExec(tc.exec))
		})
	}
}

func TestIsExecCredentialError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"aws sso exit", errors.New("getting credentials: exec: executable aws failed with exit code 255"), true},
		{"executable not found", errors.New("getting credentials: exec: executable aws not found"), true},
		{"generic exec exit", errors.New("exec: executable gke-gcloud-auth-plugin failed with exit code 1"), true},
		{"exec plugin generic", errors.New("exec plugin: invalid apiVersion"), true},
		{"unrelated api error", errors.New("pods \"foo\" not found"), false},
		{"timeout", errors.New("context deadline exceeded"), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isExecCredentialError(tc.err))
		})
	}
}

type fakeRoundTripper struct {
	resp   *http.Response
	err    error
	called bool
}

func (f *fakeRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	f.called = true
	return f.resp, f.err
}

func readUI(t *testing.T) (logger.UIEntry, bool) {
	t.Helper()
	select {
	case e := <-logger.UIChan():
		return e, true
	case <-time.After(100 * time.Millisecond):
		return logger.UIEntry{}, false
	}
}

func argsToMap(args []any) map[string]string {
	m := map[string]string{}
	for i := 0; i+1 < len(args); i += 2 {
		k, _ := args[i].(string)
		v, _ := args[i+1].(string)
		m[k] = v
	}
	return m
}

func TestCredTaggingRoundTripper(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://example.test/api", nil)

	t.Run("aws credential error emits contextual line with profile and hint", func(t *testing.T) {
		logger.ResetDedupForTest()
		inner := &fakeRoundTripper{err: errors.New("getting credentials: exec: executable aws failed with exit code 255")}
		rt := &credTaggingRoundTripper{inner: inner, context: "dev-envs", command: "aws", profile: "myprofile"}

		resp, err := rt.RoundTrip(req)
		closeResp(resp)
		assert.Error(t, err)
		assert.True(t, inner.called)

		e, ok := readUI(t)
		assert.True(t, ok, "expected a UI overlay entry for the credential failure")
		assert.Equal(t, "ERR", e.Level)
		m := argsToMap(e.Args)
		assert.Equal(t, "dev-envs", m["context"])
		assert.Equal(t, "myprofile", m["profile"])
		assert.Contains(t, m["hint"], "aws sso login --profile myprofile")
	})

	t.Run("non-credential error passes through without emitting", func(t *testing.T) {
		logger.ResetDedupForTest()
		inner := &fakeRoundTripper{err: errors.New("pods \"foo\" not found")}
		rt := &credTaggingRoundTripper{inner: inner, context: "dev-envs", command: "aws", profile: "myprofile"}

		resp, err := rt.RoundTrip(req)
		closeResp(resp)
		assert.Error(t, err)
		_, ok := readUI(t)
		assert.False(t, ok, "should not surface an overlay entry for a normal API error")
	})

	t.Run("success passes response through", func(t *testing.T) {
		logger.ResetDedupForTest()
		want := &http.Response{StatusCode: 200}
		inner := &fakeRoundTripper{resp: want}
		rt := &credTaggingRoundTripper{inner: inner, context: "dev-envs", command: "aws", profile: "p"}

		got, err := rt.RoundTrip(req)
		assert.NoError(t, err)
		assert.Same(t, want, got)
		closeResp(got)
		_, ok := readUI(t)
		assert.False(t, ok)
	})
}

// closeResp closes a response body when present, satisfying the bodyclose
// linter for the fake round tripper (whose responses carry no real body).
func closeResp(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
}
