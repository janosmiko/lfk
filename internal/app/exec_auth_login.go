package app

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/ui"
)

// loginCommand is the command lfk offers to run after an exec plugin fails.
type loginCommand struct {
	display string
	argv    []string
	env     []string
	// kubectl means argv holds only kubectl args, the caller picks the binary.
	kubectl bool
}

var safeShellWord = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

// resolveLoginCommand picks the override, then AWS SSO login, then the kubectl
// fallback. It spawns nothing itself, only isSSO may.
func resolveLoginCommand(ctx, kubectlCtx string, info k8s.ExecPluginInfo, hasInfo bool,
	isSSO func(awsBin, profile string, env []string) bool,
) loginCommand {
	if cmd := ui.ConfigClusterLoginCommand[ctx]; cmd != "" {
		return loginCommand{display: cmd, argv: []string{"sh", "-c", cmd}}
	}
	if hasInfo && k8s.IsAWSCommand(info.Command) && isSSO(info.Command, info.AWSProfile, info.Env) {
		argv := []string{info.Command, "sso", "login"}
		display := "aws sso login"
		if info.AWSProfile != "" {
			argv = append(argv, "--profile", info.AWSProfile)
			p := info.AWSProfile
			if !safeShellWord.MatchString(p) {
				p = shellQuote(p)
			}
			display += " --profile " + p
		}
		return loginCommand{display: display, argv: argv, env: info.Env}
	}
	return loginCommand{
		display: "kubectl get --raw /version --context " + kubectlCtx,
		argv: k8s.DemoKubectlArgs([]string{
			"get", "--raw", "/version", "--context", kubectlCtx, "--request-timeout=5m",
		}),
		kubectl: true,
	}
}

// awsProfileIsSSO reports whether the profile has an sso_session or sso_start_url.
// env carries the plugin env so AWS_CONFIG_FILE from the kubeconfig applies.
func awsProfileIsSSO(awsBin, profile string, env []string) bool {
	for _, key := range []string{"sso_session", "sso_start_url"} {
		args := []string{"configure", "get", key}
		if profile != "" {
			args = append(args, "--profile", profile)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(ctx, awsBin, args...)
		cmd.Env = append(os.Environ(), env...)
		out, err := cmd.Output()
		cancel()
		if err == nil && strings.TrimSpace(string(out)) != "" {
			return true
		}
	}
	return false
}
