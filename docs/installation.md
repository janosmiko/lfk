# Installation

`lfk` is distributed as a single static Go binary. Pick whichever method fits your platform.

## Homebrew (macOS / Linux)

```bash
brew install janosmiko/tap/lfk
```

## Linux

### Debian / Ubuntu (Cloudsmith APT)

Add the Cloudsmith repository, then install:

```bash
curl -1sLf 'https://dl.cloudsmith.io/public/janosmiko/lfk/setup.deb.sh' | sudo -E bash
sudo apt update
sudo apt install lfk
```

The setup script imports the Cloudsmith GPG key and writes `/etc/apt/sources.list.d/janosmiko-lfk.list`. Manual setup steps are also documented at https://cloudsmith.io/~janosmiko/repos/lfk/setup/#formats-deb.

### Fedora / RHEL / CentOS (Cloudsmith DNF)

Add the Cloudsmith repository, then install:

```bash
curl -1sLf 'https://dl.cloudsmith.io/public/janosmiko/lfk/setup.rpm.sh' | sudo -E bash
sudo dnf install lfk
```

Manual setup steps are documented at https://cloudsmith.io/~janosmiko/repos/lfk/setup/#formats-rpm.

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

## Windows

### Scoop

```powershell
scoop bucket add janosmiko https://github.com/janosmiko/scoop-bucket
scoop install lfk
```

### Winget

```powershell
winget install janosmiko.lfk
```

> The first release after a tag opens an automatic PR to `microsoft/winget-pkgs`. The package becomes installable once that PR is merged by the Winget maintainers — typically within hours.

### Chocolatey

```powershell
choco install lfk
```

> First-time installs from Chocolatey may show "Pending" while the package goes through chocolatey.org moderation. Subsequent versions usually become available immediately after publish.

### Manual binary

Download `lfk_<version>_windows_<arch>.zip` from [GitHub Releases](https://github.com/janosmiko/lfk/releases), extract `lfk.exe` into a directory in your `PATH`, and verify with `lfk --version`. Each archive is covered by the same cosign Sigstore bundle as Linux/macOS builds.

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

## External Dependencies

**Required:**
- `kubectl` - Kubernetes CLI (must be configured and in PATH)

**Optional** (needed only for specific features):
| Command | Feature |
|---------|---------|
| `helm` | Helm release management (values, diff, upgrade, rollback, uninstall) |
| `trivy` | Container image vulnerability scanning ([install](https://aquasecurity.github.io/trivy)) |

All other features (KEDA, External Secrets, Argo Workflows, cert-manager, ArgoCD, FluxCD, PVC resize, etc.) use the Kubernetes API directly and require no additional CLI tools.
