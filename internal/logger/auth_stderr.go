package logger

import "strings"

// execCredentialStderrMarkers are case-insensitive substrings that identify a
// kubeconfig exec credential plugin (AWS SSO, gke-gcloud-auth-plugin, generic
// AWS credential providers) reporting a credential failure on os.Stderr. These
// lines carry no cluster context; the failing API call is attributed to its
// context separately (see internal/k8s credTaggingRoundTripper), so the raw
// line is demoted out of the in-app overlay to avoid a contextless duplicate.
var execCredentialStderrMarkers = []string{
	"sso session",
	"aws sso login",
	"loading sso token",
	"expiredtoken",
	"the security token included",
	"unable to locate credentials",
	"gke-gcloud-auth-plugin",
}

// looksLikeExecCredentialStderr reports whether a captured stderr line is a
// credential-plugin failure that should be demoted (logged at Debug, kept out
// of the in-app overlay) in favor of the context-tagged log emitted at the
// failing API call.
func looksLikeExecCredentialStderr(line string) bool {
	l := strings.ToLower(line)
	for _, marker := range execCredentialStderrMarkers {
		if strings.Contains(l, marker) {
			return true
		}
	}
	return false
}
