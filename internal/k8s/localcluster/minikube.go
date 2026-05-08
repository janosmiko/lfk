package localcluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/janosmiko/lfk/internal/logger"
)

const (
	minikubeListTimeout      = 10 * time.Second
	minikubeCreateTimeout    = 10 * time.Minute
	minikubeStartStopTimeout = 2 * time.Minute
	minikubeDeleteTimeout    = 60 * time.Second
)

// minikubeCLIError mirrors kindCLIError / k3dCLIError: handles nil-err
// and empty-stderr edge cases so the wrap text is always well-formed.
func minikubeCLIError(op string, code int, stderr string, err error) error {
	base := fmt.Sprintf("minikube: %s failed (exit %d)", op, code)
	if s := strings.TrimSpace(stderr); s != "" {
		base += ": " + s
	}
	if err != nil {
		return fmt.Errorf("%s: %w", base, err)
	}
	return errors.New(base)
}

// minikubeProfileListJSONShape mirrors the subset of `minikube profile
// list -o json` we care about. minikube's JSON has more fields than
// these; anything we omit is ignored on Unmarshal.
type minikubeProfileListJSONShape struct {
	Valid []minikubeProfile `json:"valid"`
}

type minikubeProfile struct {
	Name   string `json:"Name"`
	Status string `json:"Status"`
	Config struct {
		KubernetesConfig struct {
			KubernetesVersion string `json:"KubernetesVersion"`
		} `json:"KubernetesConfig"`
		Nodes []struct {
			Name string `json:"Name"`
		} `json:"Nodes"`
	} `json:"Config"`
}

func (p *minikubeProvider) List(ctx context.Context) ([]Cluster, error) {
	cctx, cancel := context.WithTimeout(ctx, minikubeListTimeout)
	defer cancel()

	stdout, stderr, code, err := p.runner.Run(cctx, "minikube", "profile", "list", "-o", "json")
	if err != nil || code != 0 {
		return nil, minikubeCLIError("profile list", code, stderr, err)
	}
	var raw minikubeProfileListJSONShape
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		return nil, fmt.Errorf("minikube: parse profile list: %w", err)
	}
	out := make([]Cluster, 0, len(raw.Valid))
	for _, r := range raw.Valid {
		out = append(out, Cluster{
			Provider:    "minikube",
			Name:        r.Name,
			ContextName: r.Name, // minikube context == profile name (no prefix)
			Status:      minikubeStatus(r.Status),
			K8sVersion:  r.Config.KubernetesConfig.KubernetesVersion,
			Nodes:       len(r.Config.Nodes),
		})
	}
	return out, nil
}

// minikubeStatus maps minikube's profile-list Status string to the
// closed ClusterStatus set. minikube emits at least Running / Stopped /
// Paused / Stopping / Starting / Misconfigured / OK (recent versions
// aggregate host+apiserver into "OK" when both are Running) — we
// surface the two healthy states (running/ok → Running, stopped →
// Stopped) and let transient/misconfigured states fall through to
// Unknown. Casting the raw string into ClusterStatus directly would
// smuggle arbitrary values past the type's "closed set" contract.
//
// Unknown raw values get logged so we can spot new minikube version
// behaviours from user reports without having to reproduce locally.
func minikubeStatus(raw string) ClusterStatus {
	switch strings.ToLower(raw) {
	case "running", "ok":
		return ClusterStatusRunning
	case "stopped":
		return ClusterStatusStopped
	default:
		logger.Info("localcluster: unrecognized minikube status",
			"raw", raw,
			"hint", "fell through to Unknown; add to minikubeStatus switch if this is a healthy state",
		)
		return ClusterStatusUnknown
	}
}

func (p *minikubeProvider) Create(ctx context.Context, spec CreateSpec) error {
	cctx, cancel := context.WithTimeout(ctx, minikubeCreateTimeout)
	defer cancel()

	// `--interactive=false` is critical: lfk attaches a TTY to the
	// child process, so minikube's first-run driver picker / sudo
	// prompts would block forever, looking like a silent hang until
	// the 10-minute timeout. Forcing non-interactive mode makes
	// minikube error loud instead — the stderr surfaces in the
	// manager's global err banner.
	args := []string{"start", "--profile", spec.Name, "--interactive=false"}
	if spec.K8sVersion != "" {
		args = append(args, "--kubernetes-version", spec.K8sVersion)
	}
	if spec.Nodes > 1 {
		args = append(args, "--nodes", strconv.Itoa(spec.Nodes))
	}
	if spec.ConfigFile != "" {
		// minikube has no single config-file flag for cluster topology
		// — `--extra-config` is for component knobs like
		// `kubelet.foo=bar`, not a YAML file path. Rather than smuggle
		// the path into the wrong flag, fail loud. The wizard's
		// config-file escape hatch is deferred to v1.1; v1 keeps
		// CreateSpec.ConfigFile as always-empty.
		return fmt.Errorf("minikube: config-file is not supported (deferred to v1.1)")
	}
	_, stderr, code, err := p.runner.Run(cctx, "minikube", args...)
	if err != nil || code != 0 {
		return minikubeCLIError("start", code, stderr, err)
	}
	return nil
}

func (p *minikubeProvider) Delete(ctx context.Context, name string) error {
	cctx, cancel := context.WithTimeout(ctx, minikubeDeleteTimeout)
	defer cancel()

	_, stderr, code, err := p.runner.Run(cctx, "minikube", "delete", "--profile", name)
	if err != nil || code != 0 {
		return minikubeCLIError("delete", code, stderr, err)
	}
	return nil
}

func (p *minikubeProvider) Start(ctx context.Context, name string) error {
	return p.startStop(ctx, "start", name)
}

func (p *minikubeProvider) Stop(ctx context.Context, name string) error {
	return p.startStop(ctx, "stop", name)
}

func (p *minikubeProvider) startStop(ctx context.Context, verb, name string) error {
	cctx, cancel := context.WithTimeout(ctx, minikubeStartStopTimeout)
	defer cancel()

	_, stderr, code, err := p.runner.Run(cctx, "minikube", verb, "--profile", name)
	if err != nil || code != 0 {
		return minikubeCLIError(verb, code, stderr, err)
	}
	return nil
}
