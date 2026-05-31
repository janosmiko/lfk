package k8s

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/janosmiko/lfk/internal/logger"
)

// taggedHTTPClient builds the HTTP client for a context's rest.Config and, when
// the config uses an exec credential plugin, wraps its transport so credential
// acquisition failures are attributed to the cluster context (see
// credTaggingRoundTripper). Mirrors what kubernetes.NewForConfig does
// internally — rest.HTTPClientFor already installs the exec auth round tripper —
// so the wrapper lands outermost and observes the failing RoundTrip.
func taggedHTTPClient(cfg *rest.Config, contextName string) (*http.Client, error) {
	httpClient, err := rest.HTTPClientFor(cfg)
	if err != nil {
		return nil, fmt.Errorf("building http client: %w", err)
	}
	if cfg.ExecProvider != nil {
		// rest.HTTPClientFor can return a client with a nil Transport (the
		// http.DefaultClient case when no special transport is needed). Fall
		// back to http.DefaultTransport so RoundTrip never dereferences nil.
		inner := httpClient.Transport
		if inner == nil {
			inner = http.DefaultTransport
		}
		httpClient.Transport = &credTaggingRoundTripper{
			inner:   inner,
			context: contextName,
			command: cfg.ExecProvider.Command,
			profile: awsProfileFromExec(cfg.ExecProvider),
		}
	}
	return httpClient, nil
}

// awsProfileFromExec extracts the AWS profile a kubeconfig exec credential
// plugin runs under: the `--profile <name>` / `--profile=<name>` argument,
// falling back to the AWS_PROFILE env entry. Returns "" when no profile is
// configured (or the plugin is not AWS).
func awsProfileFromExec(exec *clientcmdapi.ExecConfig) string {
	if exec == nil {
		return ""
	}
	for i, a := range exec.Args {
		if a == "--profile" {
			if i+1 < len(exec.Args) {
				return exec.Args[i+1]
			}
			return ""
		}
		if v, ok := strings.CutPrefix(a, "--profile="); ok {
			return v
		}
	}
	for _, e := range exec.Env {
		if e.Name == "AWS_PROFILE" {
			return e.Value
		}
	}
	return ""
}

// isExecCredentialError reports whether err is a failure to acquire credentials
// from a kubeconfig exec plugin (expired AWS SSO, missing VPN, gke auth plugin
// not logged in, etc). client-go surfaces these as
// "getting credentials: exec: executable <cmd> failed with exit code N" — the
// underlying plugin's own message (e.g. the AWS SSO-expired text) only reaches
// os.Stderr, never this error, so the cluster context here is the reliable
// signal.
func isExecCredentialError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	switch {
	case strings.Contains(s, "getting credentials"):
		return true
	case strings.Contains(s, "exec plugin"):
		return true
	case strings.Contains(s, "exec:") && strings.Contains(s, "executable") && strings.Contains(s, "exit code"):
		return true
	default:
		return false
	}
}

// isAWSCommand reports whether the exec plugin command is the AWS CLI.
func isAWSCommand(command string) bool {
	return filepath.Base(command) == "aws"
}

// credTaggingRoundTripper wraps a context's REST transport so a credential
// acquisition failure — which client-go otherwise only reports as a contextless
// line on os.Stderr from the exec plugin — is logged once with the cluster
// context and AWS profile that triggered it, plus an actionable hint. It is
// installed outermost (around the exec auth round tripper) so the failing
// RoundTrip is observed on the goroutine that knows the context, making the
// attribution reliable under concurrent multi-cluster loads.
type credTaggingRoundTripper struct {
	inner   http.RoundTripper
	context string // lfk cluster display name
	command string // exec plugin command (e.g. "aws"), "" if unknown
	profile string // AWS profile, "" if unknown / non-AWS
}

func (rt *credTaggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := rt.inner.RoundTrip(req)
	if isExecCredentialError(err) {
		rt.logCredentialFailure(err)
	}
	return resp, err
}

func (rt *credTaggingRoundTripper) logCredentialFailure(err error) {
	msg := "cluster authentication failed: credential plugin error"
	args := []any{"context", rt.context, "error", logger.Redact(err.Error())}
	if rt.command != "" {
		args = append(args, "plugin", rt.command)
	}
	if isAWSCommand(rt.command) {
		msg = "cluster authentication failed: AWS credentials/SSO unavailable"
		// The profile comes from the kubeconfig; strip control characters so a
		// crafted/odd profile name can't corrupt the log line or in-app overlay.
		profile := stripControlChars(rt.profile)
		if profile != "" {
			args = append(args, "profile", profile, "hint", "aws sso login --profile "+profile)
		} else {
			args = append(args, "hint", "aws sso login")
		}
	}
	// Dedup per (plugin, context): a cluster that keeps failing every
	// background tick surfaces once per window, and one cluster's outage
	// never masks another's.
	logger.ErrorOnce("cluster-auth-"+rt.command, rt.context, msg, args...)
}
