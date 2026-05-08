package localcluster

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	kindListTimeout   = 10 * time.Second
	kindCreateTimeout = 10 * time.Minute
	kindDeleteTimeout = 60 * time.Second
)

// kindCLIError builds the wrapped error for a non-zero kind invocation.
// Handles two edge cases the inline format cannot:
//   - err == nil with non-zero exit code: emits a plain error (no %!w)
//   - empty stderr: omits the double-colon ": :"
//
// Both are theoretically reachable through FakeRunner; the production
// realRunner always pairs a non-zero exit with a non-nil ExitError.
func kindCLIError(op string, code int, stderr string, err error) error {
	base := fmt.Sprintf("kind: %s failed (exit %d)", op, code)
	if s := strings.TrimSpace(stderr); s != "" {
		base += ": " + s
	}
	if err != nil {
		return fmt.Errorf("%s: %w", base, err)
	}
	return errors.New(base)
}

func (p *kindProvider) List(ctx context.Context) ([]Cluster, error) {
	cctx, cancel := context.WithTimeout(ctx, kindListTimeout)
	defer cancel()

	stdout, stderr, code, err := p.runner.Run(cctx, "kind", "get", "clusters")
	if err != nil || code != 0 {
		return nil, kindCLIError("get clusters", code, stderr, err)
	}
	out := make([]Cluster, 0)
	for line := range strings.SplitSeq(strings.TrimSpace(stdout), "\n") {
		name := strings.TrimSpace(line)
		if name == "" || name == "No kind clusters found." {
			continue
		}
		c := Cluster{
			Provider:    "kind",
			Name:        name,
			ContextName: "kind-" + name,
			Status:      kindContainerStatus(cctx, p.runner, name),
		}
		out = append(out, c)
	}
	return out, nil
}

func (p *kindProvider) Create(ctx context.Context, spec CreateSpec) error {
	if err := validateConfigFile(spec.ConfigFile); err != nil {
		return fmt.Errorf("kind: %w", err)
	}
	cctx, cancel := context.WithTimeout(ctx, kindCreateTimeout)
	defer cancel()

	args := []string{"create", "cluster", "--name", spec.Name}
	if spec.K8sVersion != "" {
		args = append(args, "--image", "kindest/node:"+spec.K8sVersion)
	}
	if spec.ConfigFile != "" {
		args = append(args, "--config", spec.ConfigFile)
	} else if spec.Nodes > 1 {
		path, err := writeKindMultiNodeConfig(spec.Nodes)
		if err != nil {
			return fmt.Errorf("kind: write config: %w", err)
		}
		defer removeFile(path)
		args = append(args, "--config", path)
	}

	_, stderr, code, err := p.runner.Run(cctx, "kind", args...)
	if err != nil || code != 0 {
		return kindCLIError("create", code, stderr, err)
	}
	return nil
}

func writeKindMultiNodeConfig(nodes int) (string, error) {
	var b strings.Builder
	b.WriteString("kind: Cluster\napiVersion: kind.x-k8s.io/v1alpha4\nnodes:\n")
	b.WriteString("  - role: control-plane\n")
	for i := 1; i < nodes; i++ {
		b.WriteString("  - role: worker\n")
	}
	f, err := os.CreateTemp("", "lfk-kind-*.yaml")
	if err != nil {
		return "", err
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(f.Name())
		}
	}()
	if _, err := f.WriteString(b.String()); err != nil {
		_ = f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	ok = true
	return f.Name(), nil
}

func removeFile(path string) { _ = os.Remove(path) }

func (p *kindProvider) Delete(ctx context.Context, name string) error {
	cctx, cancel := context.WithTimeout(ctx, kindDeleteTimeout)
	defer cancel()

	_, stderr, code, err := p.runner.Run(cctx, "kind", "delete", "cluster", "--name", name)
	if err != nil || code != 0 {
		return kindCLIError("delete", code, stderr, err)
	}
	return nil
}

// kindProvider intentionally does NOT implement LifecycleProvider.
// kind has no native verb for stopping or restarting an existing
// cluster — handlers must type-assert prov.(LifecycleProvider) and
// silently fall through when the provider doesn't satisfy it.

// kindContainerStatus asks Docker for the control-plane container's
// state. Multi-node kind clusters have additional worker containers
// whose state is NOT polled here; if a partial-stop leaves only
// workers down, this function still reports ClusterStatusRunning.
// That's a deliberate simplification: the manager UI's "is this cluster
// roughly up" probe doesn't justify N docker-inspect calls per cluster.
//
// `name` here is a string from kind's own stdout (`kind get clusters`),
// not from wizard-validated user input. It reaches `docker inspect` as
// a discrete argv element via exec.CommandContext (NOT via `sh -c`),
// so shell metacharacters cannot escape — argv injection is contained
// at the os/exec layer regardless of what kind reports. If a future
// refactor switches to a shell-interpreted path, this is the call site
// that needs explicit validation.
//
// Returns ClusterStatusRunning, ClusterStatusStopped, or
// ClusterStatusUnknown (when docker isn't available or the container
// isn't found). The manager UI treats Unknown as informational, not
// an error — kind on Podman or with a non-default container runtime
// might land here.
func kindContainerStatus(ctx context.Context, r CmdRunner, name string) ClusterStatus {
	stdout, _, code, err := r.Run(ctx, "docker", "inspect", name+"-control-plane", "--format", "{{.State.Status}}")
	if err != nil || code != 0 {
		return ClusterStatusUnknown
	}
	switch strings.TrimSpace(stdout) {
	case "running":
		return ClusterStatusRunning
	case "exited", "paused", "dead":
		return ClusterStatusStopped
	default:
		return ClusterStatusUnknown
	}
}
