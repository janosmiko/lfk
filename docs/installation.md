# Installation

`lfk` is distributed as a single static Go binary. Pick whichever method fits your platform.

## Homebrew (macOS / Linux)

```bash
brew install janosmiko/tap/lfk
```

## Nix (flake)

Requires Nix ≥ 2.4 with flakes enabled (`experimental-features = nix-command flakes` in `~/.config/nix/nix.conf`).

**Try without installing:**

```bash
nix run github:janosmiko/lfk
# pinned to a release:
nix run github:janosmiko/lfk/v0.9.22
```

**Install into your profile:**

```bash
nix profile install github:janosmiko/lfk
# or a specific release:
nix profile install github:janosmiko/lfk/v0.9.22
```

**Use as a flake input** (e.g. in a NixOS / home-manager config):

```nix
{
  inputs.lfk.url = "github:janosmiko/lfk";

  outputs = { self, nixpkgs, lfk, ... }: {
    # ...
    environment.systemPackages = [ lfk.packages.${system}.default ];
  };
}
```

## Binary releases

Download pre-built binaries from the [GitHub Releases](https://github.com/janosmiko/lfk/releases) page.

## From source

```bash
go install github.com/janosmiko/lfk@latest
```

## Build from source

```bash
git clone https://github.com/janosmiko/lfk.git
cd lfk
go build -o lfk .
```

## Docker

```bash
docker run -it --rm \
  -v ~/.kube:/home/lfk/.kube:ro \
  janosmiko/lfk
```

To use a specific kubeconfig:

```bash
docker run -it --rm \
  -v /path/to/kubeconfig:/home/lfk/.kube/config:ro \
  janosmiko/lfk
```

For port forwarding, add `--net=host`:

```bash
docker run -it --rm \
  --net=host \
  -v ~/.kube:/home/lfk/.kube:ro \
  janosmiko/lfk
```

## Nightly Builds

Nightly builds track the latest development work and are published as GitHub
pre-releases with every `v*-nightly*` tag.

**Homebrew:**

```bash
brew install janosmiko/tap/lfk-nightly
```

**Docker:**

```bash
# Latest nightly
docker run -it --rm \
  -v ~/.kube:/home/lfk/.kube:ro \
  janosmiko/lfk:nightly

# Specific nightly date
docker run -it --rm \
  -v ~/.kube:/home/lfk/.kube:ro \
  janosmiko/lfk:nightly-20260414
```

**Binary releases:**

Download from [GitHub Releases](https://github.com/janosmiko/lfk/releases) (look for pre-release tags).

## External Dependencies

**Required:**
- `kubectl` - Kubernetes CLI (must be configured and in PATH)

**Optional** (needed only for specific features):
| Command | Feature |
|---------|---------|
| `helm` | Helm release management (values, diff, upgrade, rollback, uninstall) |
| `trivy` | Container image vulnerability scanning ([install](https://aquasecurity.github.io/trivy)) |

All other features (KEDA, External Secrets, Argo Workflows, cert-manager, ArgoCD, FluxCD, PVC resize, etc.) use the Kubernetes API directly and require no additional CLI tools.
