# Release Process

Operational reference for the maintainer. The release pipeline is fully automated on tag push (see `.github/workflows/release.yml`); this document covers one-time bootstrap, secret management, and recovery procedures.

## Trigger a Release

Releases are driven by `release-please`:

1. Land conventional commits on `main`.
2. `release-please` opens / updates the long-running Release PR with version bump + changelog.
3. Merging the Release PR creates a `vX.Y.Z` tag.
4. The tag triggers `.github/workflows/release.yml`, which builds and publishes everything.

To cut a release manually (skipping `release-please`): `make release VERSION=X.Y.Z` then `git push && git push --tags`.

## External Accounts (one-time bootstrap)

| Channel | Account / Repo | Setup |
|---|---|---|
| Homebrew | `janosmiko/homebrew-tap` | Existing. PAT in `RELEASE_TAP_TOKEN`. |
| Scoop | `janosmiko/scoop-bucket` | Empty repo with README + LICENSE. Same PAT. |
| Winget | Fork of `microsoft/winget-pkgs` at `janosmiko/winget-pkgs` | PAT in `WINGET_TOKEN` with `contents:write` on the fork. |
| AUR | aur.archlinux.org account `janosmiko` | SSH public key uploaded; `lfk-bin` reserved. Private key in `AUR_SSH_PRIVATE_KEY` (multi-line PEM). |
| Chocolatey | chocolatey.org publisher `janosmiko` | API key in `CHOCOLATEY_API_KEY`. `lfk` package id reserved. |
| Snap | snapcraft.io publisher | Snap name `lfk` registered; classic confinement justification approved. Macaroon in `SNAPCRAFT_STORE_CREDENTIALS` (run `snapcraft export-login --snaps=lfk --acls=package_access,package_push,package_update,package_release - 2>&1 \| tail -n +2`). |
| Cloudsmith | cloudsmith.io account `janosmiko` | Repository `janosmiko/lfk` with DEB + RPM enabled. API key in `CLOUDSMITH_API_KEY`. |

## GitHub Secrets

| Secret | Used by | Rotation |
|---|---|---|
| `GITHUB_TOKEN` | release publish | auto, never rotate manually |
| `RELEASE_PLEASE_TOKEN` | release-please CI | rotate annually |
| `DOCKERHUB_USERNAME` / `DOCKERHUB_TOKEN` | Docker push | rotate annually |
| `RELEASE_TAP_TOKEN` | Homebrew + Scoop | rotate annually |
| `WINGET_TOKEN` | Winget upstream PR | rotate annually |
| `AUR_SSH_PRIVATE_KEY` | AUR push | rotate when key compromised |
| `CHOCOLATEY_API_KEY` | choco push | rotate when key compromised |
| `SNAPCRAFT_STORE_CREDENTIALS` | snap upload | macaroon expires; rotate every 12 months |
| `CLOUDSMITH_API_KEY` | cloudsmith push | rotate annually |

## Recovery: a single channel's publish failed

The release succeeded for most channels but one publish step errored. Re-run only the failing channel:

```bash
# Locally, on the tag commit:
git checkout vX.Y.Z
goreleaser release --clean --config .goreleaser.yaml --skip=<comma-separated channels you do NOT want>

# Example: re-publish only Chocolatey (skip every other publisher; keep build, archive, sign, sbom).
goreleaser release --clean --config .goreleaser.yaml \
  --skip=brews,scoops,winget,aurs,nfpms,dockers,snapcrafts
```

For Cloudsmith specifically, you can manually push:

```bash
cloudsmith push deb janosmiko/lfk/any-distro/any-version dist/lfk_<version>_<arch>.deb
cloudsmith push rpm janosmiko/lfk/any-distro/any-version dist/lfk_<version>_<arch>.rpm
```

## Per-channel post-release validation checklist

Run after each release tag completes. Channels with classic Snap or first-time Chocolatey moderation may legitimately be "pending" — that's not a failure.

- [ ] **Homebrew:** `brew update && brew upgrade lfk` on macOS or Linux; `lfk --version` matches.
- [ ] **Scoop:** on Windows, `scoop update lfk && lfk --version`.
- [ ] **Winget:** check `https://github.com/microsoft/winget-pkgs/pulls?q=author%3Ajanosmiko` for the auto-PR. Once merged: `winget upgrade janosmiko.lfk`.
- [ ] **AUR:** `https://aur.archlinux.org/packages/lfk-bin` shows the new version; `yay -Syu lfk-bin` on Arch.
- [ ] **Chocolatey:** `https://chocolatey.org/packages/lfk` shows the new version (may say "Pending" for first submission). `choco upgrade lfk`.
- [ ] **Snap:** `snap info lfk` shows new version on `stable` track. `snap refresh lfk --classic`.
- [ ] **Cloudsmith:** `https://cloudsmith.io/~janosmiko/repos/lfk/packages/` lists the new `.deb` and `.rpm`.
- [ ] **Docker:** `docker pull janosmiko/lfk:vX.Y.Z` and `docker pull janosmiko/lfk:latest`.
- [ ] **Nix:** `nix run github:janosmiko/lfk/vX.Y.Z` succeeds.

## Adding a new channel

1. Add a `<channel>:` block to `.goreleaser.yaml` per GoReleaser docs.
2. Add any required tooling install step to `.github/workflows/release.yml`.
3. Add the channel's secret to GitHub Secrets and to the `env:` block of the `Run GoReleaser` step.
4. Update this file's tables (External Accounts, GitHub Secrets, validation checklist).
5. Update `docs/installation.md` with the user-facing install instructions.
6. Open a release tag to validate end-to-end.
