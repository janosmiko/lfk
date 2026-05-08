// Package localcluster manages local Kubernetes clusters created via the
// kind, k3d, and minikube CLIs. Each provider exposes the same small
// interface so the manager overlay can treat them uniformly.
package localcluster

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNotSupported is returned by Provider methods that the underlying
// CLI doesn't implement — for example, kind has no native start/stop.
var ErrNotSupported = errors.New("operation not supported by this provider")

// ErrInvalidConfigFile is returned by Provider.Create when CreateSpec.ConfigFile
// fails the path-safety gate (relative paths, traversal segments,
// non-existent files). Provider implementations enforce this at the
// boundary so a future caller that bypasses the wizard can't smuggle
// arbitrary paths into the underlying CLI's --config flag.
var ErrInvalidConfigFile = errors.New("invalid config file path")

// validateConfigFile gates the optional CreateSpec.ConfigFile field.
// It rejects empty-but-meant-to-be-set, relative paths, paths with
// traversal segments, and non-existent files. Callers must pass an
// absolute path to a regular file.
//
// The wizard never sets ConfigFile in v1, so this is dead in the
// happy path today. It exists to harden the provider boundary against
// future callers (or programmatic API consumers) that could otherwise
// hand a hostile string to kind/k3d's --config flag. The escape hatch
// for a future config-file wizard step lands here unchanged.
func validateConfigFile(path string) error {
	if path == "" {
		return nil
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%w: must be absolute path, got %q", ErrInvalidConfigFile, path)
	}
	clean := filepath.Clean(path)
	if clean != path {
		return fmt.Errorf("%w: path contains traversal segments, got %q", ErrInvalidConfigFile, path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%w: stat: %w", ErrInvalidConfigFile, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: not a regular file: %q", ErrInvalidConfigFile, path)
	}
	return nil
}

// ClusterStatus is the lifecycle state reported by a Provider's List
// implementation. The set is closed: providers must map their CLI
// output to one of these values (anything they can't classify becomes
// ClusterStatusUnknown).
type ClusterStatus string

const (
	ClusterStatusRunning ClusterStatus = "running"
	ClusterStatusStopped ClusterStatus = "stopped"
	ClusterStatusUnknown ClusterStatus = "unknown"
)

// Provider is the minimum contract every local-cluster CLI wrapper
// implements: identity, presence, listing, and creation/destruction.
// Implementations are not required to be goroutine-safe; callers must
// serialize access to a single Provider.
//
// kind cannot start or stop existing clusters in place (it has no
// native verb for it), so kindProvider implements Provider but NOT
// LifecycleProvider. Callers that need start/stop must type-assert
// to LifecycleProvider — the type assertion replaces the runtime
// SupportsStartStop() capability query that earlier revisions used.
type Provider interface {
	Name() string
	// Installed reports whether the underlying CLI is present on $PATH.
	// Implementations should treat any LookPath failure (file not found,
	// permission denied, malformed PATH) as "not installed" and return
	// false — distinguishing transient detection failures from genuine
	// absence is not worth the API complexity here.
	Installed() bool
	List(ctx context.Context) ([]Cluster, error)
	Create(ctx context.Context, spec CreateSpec) error
	Delete(ctx context.Context, name string) error
}

// LifecycleProvider is the optional capability for providers whose
// CLIs support stopping and restarting an existing cluster. k3d and
// minikube implement this; kind does not (creating + deleting are the
// only kind verbs that touch cluster lifecycle).
//
// Use a type assertion to test for it:
//
//	if lp, ok := prov.(LifecycleProvider); ok {
//	    lp.Start(ctx, name)
//	}
type LifecycleProvider interface {
	Provider
	Start(ctx context.Context, name string) error
	Stop(ctx context.Context, name string) error
}

// CreateSpec carries user input from the wizard down to the Provider.
// Empty K8sVersion / ConfigFile mean "use the provider default".
//
// When ConfigFile is non-empty, Nodes is ignored — the config file is
// presumed to declare the cluster topology. The wizard's Nodes input
// only matters in the no-config-file path.
//
// K8sVersion is provider-translated. For k3d specifically, the value is
// embedded into a `rancher/k3s:<K8sVersion>-k3s1` image ref — only
// versions whose `-k3s1` build was published will pull successfully.
// The wizard should constrain the input to a curated list rather than
// accepting free-form versions to avoid silent docker-pull failures
// on uncommon patch revisions.
//
// ConfigFile is validated by Provider.Create at the boundary
// (validateConfigFile): must be empty, or an absolute path to an
// existing regular file with no traversal segments. The v1 wizard
// always leaves it empty; the validation exists so a future
// programmatic caller can't smuggle ../etc/passwd or similar into
// the underlying CLI's --config flag.
type CreateSpec struct {
	Name       string
	K8sVersion string
	Nodes      int    // ignored when ConfigFile is set
	ConfigFile string // optional absolute path to existing regular file; see validateConfigFile
}

// Cluster is the manager's view of one local cluster. Best-effort: any
// field that the underlying CLI doesn't expose is left empty / zero.
type Cluster struct {
	Provider    string
	Name        string
	ContextName string
	Status      ClusterStatus
	K8sVersion  string
	Nodes       int
	Age         string
}

// All returns one Provider per known impl, regardless of installation
// state. Order is stable (kind, k3d, minikube) so the manager overlay
// can render a deterministic provider list.
func All() []Provider {
	r := realRunner{}
	return []Provider{
		newKindProvider(r),
		newK3dProvider(r),
		newMinikubeProvider(r),
	}
}

// Installed returns only providers whose CLIs are present on $PATH.
// Use this when populating UI surfaces that should only show actionable
// providers.
func Installed() []Provider {
	return installedFrom(All())
}

// installedFrom is the test seam: callers can pass providers built with
// a FakeRunner to assert the filter logic without touching $PATH.
func installedFrom(provs []Provider) []Provider {
	out := make([]Provider, 0, len(provs))
	for _, p := range provs {
		if p.Installed() {
			out = append(out, p)
		}
	}
	return out
}

// ByName returns the named provider (kind | k3d | minikube), or nil
// when the name is unknown.
func ByName(name string) Provider {
	for _, p := range All() {
		if p.Name() == name {
			return p
		}
	}
	return nil
}

// --- providers ---
//
// Each provider type has a runner field for the os/exec seam. Real
// lifecycle implementations live in kind.go, k3d.go, and minikube.go;
// k3d and minikube also satisfy LifecycleProvider via Start/Stop
// methods declared in their respective files. The trivial Name and
// Installed methods stay here (provider-uniform shape).

func newKindProvider(r CmdRunner) Provider     { return &kindProvider{runner: r} }
func newK3dProvider(r CmdRunner) Provider      { return &k3dProvider{runner: r} }
func newMinikubeProvider(r CmdRunner) Provider { return &minikubeProvider{runner: r} }

type kindProvider struct{ runner CmdRunner }

func (*kindProvider) Name() string      { return "kind" }
func (p *kindProvider) Installed() bool { _, err := p.runner.LookPath("kind"); return err == nil }

type k3dProvider struct{ runner CmdRunner }

func (*k3dProvider) Name() string      { return "k3d" }
func (p *k3dProvider) Installed() bool { _, err := p.runner.LookPath("k3d"); return err == nil }

type minikubeProvider struct{ runner CmdRunner }

func (*minikubeProvider) Name() string { return "minikube" }
func (p *minikubeProvider) Installed() bool {
	_, err := p.runner.LookPath("minikube")
	return err == nil
}
