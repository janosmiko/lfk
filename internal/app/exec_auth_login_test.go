package app

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/ui"
)

func TestResolveLoginCommand(t *testing.T) {
	awsInfo := k8s.ExecPluginInfo{
		Command:    "/opt/homebrew/bin/aws",
		Env:        []string{"AWS_CONFIG_FILE=/x/config"},
		AWSProfile: "dev",
	}
	noProfile := awsInfo
	noProfile.AWSProfile = ""
	evil := awsInfo
	evil.AWSProfile = "dev; touch /tmp/pwned"
	other := k8s.ExecPluginInfo{Command: "kubelogin"}
	yes := func(string, string, []string) bool { return true }
	no := func(string, string, []string) bool { return false }

	tests := []struct {
		name        string
		override    string
		info        k8s.ExecPluginInfo
		hasInfo     bool
		isSSO       func(string, string, []string) bool
		wantArgv    []string
		wantDisplay string
		wantEnv     []string
		wantKubectl bool
	}{
		{
			name: "override wins over SSO", override: "my-login", info: awsInfo, hasInfo: true, isSSO: yes,
			wantArgv: []string{"sh", "-c", "my-login"}, wantDisplay: "my-login",
		},
		{
			name: "aws sso with profile", info: awsInfo, hasInfo: true, isSSO: yes,
			wantArgv:    []string{"/opt/homebrew/bin/aws", "sso", "login", "--profile", "dev"},
			wantDisplay: "aws sso login --profile dev", wantEnv: awsInfo.Env,
		},
		{
			name: "aws sso without profile", info: noProfile, hasInfo: true, isSSO: yes,
			wantArgv:    []string{"/opt/homebrew/bin/aws", "sso", "login"},
			wantDisplay: "aws sso login", wantEnv: awsInfo.Env,
		},
		{
			name: "aws not sso falls back", info: awsInfo, hasInfo: true, isSSO: no,
			wantArgv:    []string{"get", "--raw", "/version", "--context", "kctx", "--request-timeout=5m"},
			wantDisplay: "kubectl get --raw /version --context kctx", wantKubectl: true,
		},
		{
			name: "non-aws falls back", info: other, hasInfo: true, isSSO: yes,
			wantArgv:    []string{"get", "--raw", "/version", "--context", "kctx", "--request-timeout=5m"},
			wantDisplay: "kubectl get --raw /version --context kctx", wantKubectl: true,
		},
		{
			name: "no info falls back", isSSO: yes,
			wantArgv:    []string{"get", "--raw", "/version", "--context", "kctx", "--request-timeout=5m"},
			wantDisplay: "kubectl get --raw /version --context kctx", wantKubectl: true,
		},
		{
			name: "hostile profile stays one argv entry", info: evil, hasInfo: true, isSSO: yes,
			wantArgv:    []string{"/opt/homebrew/bin/aws", "sso", "login", "--profile", "dev; touch /tmp/pwned"},
			wantDisplay: "aws sso login --profile " + shellQuote("dev; touch /tmp/pwned"), wantEnv: awsInfo.Env,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := ui.ConfigClusterLoginCommand
			t.Cleanup(func() { ui.ConfigClusterLoginCommand = old })
			ui.ConfigClusterLoginCommand = map[string]string{}
			if tt.override != "" {
				ui.ConfigClusterLoginCommand["ctx"] = tt.override
			}
			got := resolveLoginCommand("ctx", "kctx", tt.info, tt.hasInfo, tt.isSSO)
			if !reflect.DeepEqual(got.argv, tt.wantArgv) {
				t.Errorf("argv = %q, want %q", got.argv, tt.wantArgv)
			}
			if got.display != tt.wantDisplay {
				t.Errorf("display = %q, want %q", got.display, tt.wantDisplay)
			}
			if !reflect.DeepEqual(got.env, tt.wantEnv) {
				t.Errorf("env = %q, want %q", got.env, tt.wantEnv)
			}
			if got.kubectl != tt.wantKubectl {
				t.Errorf("kubectl = %v, want %v", got.kubectl, tt.wantKubectl)
			}
		})
	}
}

func TestAWSProfileIsSSO(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "aws")
	script := `#!/bin/sh
for a in "$@"; do
  if [ "$a" = "sso_start_url" ]; then echo "https://x.awsapps.com/start"; fi
  if [ "$a" = "sso_session" ] && [ -n "$FAKE_SESSION" ]; then echo "$FAKE_SESSION"; fi
done
`
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		profile string
		env     []string
		want    bool
	}{
		{"start url only", "dev", nil, true},
		{"no profile", "", nil, true},
		{"session via plugin env", "dev", []string{"FAKE_SESSION=s"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := awsProfileIsSSO(fake, tt.profile, tt.env); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}

	empty := filepath.Join(dir, "aws-empty")
	if err := os.WriteFile(empty, []byte("#!/bin/sh\nexit 255\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if awsProfileIsSSO(empty, "dev", nil) {
		t.Error("expected false when aws prints nothing")
	}
	if awsProfileIsSSO(filepath.Join(dir, "missing"), "dev", nil) {
		t.Error("expected false for missing binary")
	}
}

func TestResolveLoginCommandDisplayHasNoControlChars(t *testing.T) {
	hasCtl := func(s string) bool {
		for _, r := range s {
			if r < 0x20 || r == 0x7f {
				return true
			}
		}
		return false
	}
	yes := func(string, string, []string) bool { return true }
	profile := "x\x1b[2Jfake\nline\r"
	ctxName := "kc\x1b[2Jtx"
	tests := []struct {
		name     string
		override string
		info     k8s.ExecPluginInfo
		hasInfo  bool
		kctx     string
		wantArg  string
		argIdx   int
	}{
		{"profile with ESC and newline", "", k8s.ExecPluginInfo{Command: "aws", AWSProfile: profile}, true, "k", profile, 4},
		{"kubectl context with ESC", "", k8s.ExecPluginInfo{}, false, ctxName, ctxName, 4},
		{"override with ESC", "echo \x1b[2J", k8s.ExecPluginInfo{}, false, "k", "echo \x1b[2J", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := ui.ConfigClusterLoginCommand
			t.Cleanup(func() { ui.ConfigClusterLoginCommand = old })
			ui.ConfigClusterLoginCommand = map[string]string{}
			if tt.override != "" {
				ui.ConfigClusterLoginCommand["ctx"] = tt.override
			}
			got := resolveLoginCommand("ctx", tt.kctx, tt.info, tt.hasInfo, yes)
			if got.argv[tt.argIdx] != tt.wantArg {
				t.Errorf("argv[%d] = %q, want raw %q", tt.argIdx, got.argv[tt.argIdx], tt.wantArg)
			}
			if hasCtl(got.display) {
				t.Errorf("display has control bytes: %q", got.display)
			}
		})
	}
}
