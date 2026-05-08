package localcluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	k3dListTimeout      = 10 * time.Second
	k3dCreateTimeout    = 10 * time.Minute
	k3dStartStopTimeout = 2 * time.Minute
	k3dDeleteTimeout    = 60 * time.Second
)

// k3dCLIError mirrors kindCLIError: handles nil-err and empty-stderr
// edge cases so the wrap text is always well-formed.
func k3dCLIError(op string, code int, stderr string, err error) error {
	base := fmt.Sprintf("k3d: %s failed (exit %d)", op, code)
	if s := strings.TrimSpace(stderr); s != "" {
		base += ": " + s
	}
	if err != nil {
		return fmt.Errorf("%s: %w", base, err)
	}
	return errors.New(base)
}

// k3dClusterJSON mirrors the subset of `k3d cluster list -o json` we
// care about. Field names are exactly k3d's JSON output.
type k3dClusterJSON struct {
	Name           string `json:"name"`
	ServersRunning int    `json:"serversRunning"`
	ServersCount   int    `json:"serversCount"`
	AgentsRunning  int    `json:"agentsRunning"`
	AgentsCount    int    `json:"agentsCount"`
}

func (p *k3dProvider) List(ctx context.Context) ([]Cluster, error) {
	cctx, cancel := context.WithTimeout(ctx, k3dListTimeout)
	defer cancel()

	stdout, stderr, code, err := p.runner.Run(cctx, "k3d", "cluster", "list", "-o", "json")
	if err != nil || code != 0 {
		return nil, k3dCLIError("cluster list", code, stderr, err)
	}
	var raw []k3dClusterJSON
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		return nil, fmt.Errorf("k3d: parse cluster list: %w", err)
	}
	out := make([]Cluster, 0, len(raw))
	for _, r := range raw {
		// Running iff every server is up. We treat partial-up as
		// stopped so the manager UI offers Start as the corrective
		// action (Start is idempotent on already-running nodes).
		status := ClusterStatusStopped
		if r.ServersCount > 0 && r.ServersRunning >= r.ServersCount {
			status = ClusterStatusRunning
		}
		// Nodes counts only servers + agents. k3d also adds a default
		// load-balancer container per cluster, but that's implementation
		// detail, not topology — the manager UI's "node count" should
		// reflect what the user asked for, not container count.
		out = append(out, Cluster{
			Provider:    "k3d",
			Name:        r.Name,
			ContextName: "k3d-" + r.Name,
			Status:      status,
			Nodes:       r.ServersCount + r.AgentsCount,
		})
	}
	return out, nil
}

func (p *k3dProvider) Create(ctx context.Context, spec CreateSpec) error {
	if err := validateConfigFile(spec.ConfigFile); err != nil {
		return fmt.Errorf("k3d: %w", err)
	}
	cctx, cancel := context.WithTimeout(ctx, k3dCreateTimeout)
	defer cancel()

	args := []string{"cluster", "create", spec.Name}
	if spec.K8sVersion != "" {
		args = append(args, "--image", "rancher/k3s:"+spec.K8sVersion+"-k3s1")
	}
	if spec.Nodes > 1 {
		// k3d models the topology as 1 server + (N-1) agents.
		args = append(args, "--agents", strconv.Itoa(spec.Nodes-1))
	}
	if spec.ConfigFile != "" {
		args = append(args, "--config", spec.ConfigFile)
	}

	_, stderr, code, err := p.runner.Run(cctx, "k3d", args...)
	if err != nil || code != 0 {
		return k3dCLIError("create", code, stderr, err)
	}
	return nil
}

func (p *k3dProvider) Delete(ctx context.Context, name string) error {
	cctx, cancel := context.WithTimeout(ctx, k3dDeleteTimeout)
	defer cancel()

	_, stderr, code, err := p.runner.Run(cctx, "k3d", "cluster", "delete", name)
	if err != nil || code != 0 {
		return k3dCLIError("delete", code, stderr, err)
	}
	return nil
}

func (p *k3dProvider) Start(ctx context.Context, name string) error {
	return p.startStop(ctx, "start", name)
}

func (p *k3dProvider) Stop(ctx context.Context, name string) error {
	return p.startStop(ctx, "stop", name)
}

func (p *k3dProvider) startStop(ctx context.Context, verb, name string) error {
	cctx, cancel := context.WithTimeout(ctx, k3dStartStopTimeout)
	defer cancel()

	_, stderr, code, err := p.runner.Run(cctx, "k3d", "cluster", verb, name)
	if err != nil || code != 0 {
		return k3dCLIError(verb, code, stderr, err)
	}
	return nil
}
